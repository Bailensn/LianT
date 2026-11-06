// Package botrun 管理 Bot 的运行生命周期：按需启动平台适配器、失败退避重试、
// 把入站消息交给核心落库，并把状态写回 store。
//
// 注意：v1 先在同进程内跑适配器（goroutine）。DESIGN D5 最终要求一 bot 一进程，
// 那需要一条子进程 ↔ 父进程的事件通道；等客户端闭环跑通后再迁移，
// 这层接口（Start/Stop/Reconcile）不需要改。
package botrun

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Bailensn/LianT/service/internal/core"
	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/store"
)

// 退避参数（修 A7：固定 3s → 指数退避，上限 30s）。
const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
	// 稳定运行超过这个时长后，重试计数清零。
	stableRun = time.Minute
)

// Ingestor 把入站消息交给核心（core.Core.Ingest 满足）。
type Ingestor interface {
	Ingest(ctx context.Context, botID string, in platform.Inbound) (bool, error)
}

// Options 是 Manager 的依赖。
type Options struct {
	Store    *store.Store
	Ingest   Ingestor
	Adapter  func(platformName string) (platform.Adapter, bool)
	Decode   func(store.Bot) (platform.Credentials, error)
	OnStatus func(botID string, st platform.Status)
	Now      func() time.Time
	Backoff  func(attempt int) time.Duration
}

// Manager 跑着当前进程里的全部 Bot。
type Manager struct {
	opt Options

	mu   sync.Mutex
	runs map[string]*run
}

type run struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// New 校验依赖并构造 Manager。
func New(opt Options) (*Manager, error) {
	if opt.Store == nil {
		return nil, errors.New("botrun: 缺少存储")
	}
	if opt.Ingest == nil {
		return nil, errors.New("botrun: 缺少入站处理")
	}
	if opt.Adapter == nil {
		opt.Adapter = platform.Get
	}
	if opt.Decode == nil {
		opt.Decode = core.DecodeCredentials
	}
	if opt.OnStatus == nil {
		opt.OnStatus = func(string, platform.Status) {}
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.Backoff == nil {
		opt.Backoff = defaultBackoff
	}
	return &Manager{opt: opt, runs: map[string]*run{}}, nil
}

func defaultBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := minBackoff << (attempt - 1)
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// Start 启动一个 Bot；已在运行则幂等返回。
func (m *Manager) Start(ctx context.Context, botID string) error {
	b, err := m.opt.Store.Bot(ctx, botID)
	if err != nil {
		return err
	}
	if _, ok := m.opt.Adapter(b.Platform); !ok {
		return fmt.Errorf("botrun: 平台 %q 没有注册适配器", b.Platform)
	}

	m.mu.Lock()
	if _, ok := m.runs[botID]; ok {
		m.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(context.Background())
	r := &run{cancel: cancel, done: make(chan struct{})}
	m.runs[botID] = r
	m.mu.Unlock()

	go m.loop(runCtx, b, r)
	return nil
}

// Stop 停止一个 Bot 并等待它退出；未运行则返回 nil。
func (m *Manager) Stop(botID string) error {
	m.mu.Lock()
	r, ok := m.runs[botID]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("botrun: 停止 %s 超时", botID)
	}
	return nil
}

// Reconcile 在启动时认领 store 里状态为 running/starting 的 Bot。
func (m *Manager) Reconcile(ctx context.Context) (int, error) {
	bots, err := m.opt.Store.Bots(ctx)
	if err != nil {
		return 0, err
	}
	started := 0
	for _, b := range bots {
		switch b.Status.State {
		case platform.StateRunning, platform.StateStarting:
			if err := m.Start(ctx, b.ID); err != nil {
				m.opt.OnStatus(b.ID, platform.Status{State: platform.StateError, Reason: err.Error()})
				continue
			}
			started++
		}
	}
	return started, nil
}

// Shutdown 停止全部 Bot。
func (m *Manager) Shutdown() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.runs))
	for id := range m.runs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *Manager) loop(ctx context.Context, b store.Bot, r *run) {
	defer close(r.done)
	defer func() {
		m.mu.Lock()
		delete(m.runs, b.ID)
		m.mu.Unlock()
	}()

	attempt := 0
	for {
		adapter, ok := m.opt.Adapter(b.Platform)
		if !ok {
			m.setStatus(ctx, b.ID, platform.Status{State: platform.StateError, Reason: "平台适配器未注册"})
			return
		}
		creds, err := m.opt.Decode(b)
		if err != nil {
			m.setStatus(ctx, b.ID, platform.Status{State: platform.StateError, Reason: "凭据不可用"})
			return
		}
		acc := accountOf(b)
		startedAt := m.opt.Now()
		m.setStatus(ctx, b.ID, platform.Status{State: platform.StateStarting})

		hooks := platform.Hooks{
			OnMessage: func(ctx context.Context, msg platform.Inbound) error {
				_, err := m.opt.Ingest.Ingest(ctx, b.ID, msg)
				return err
			},
			OnStatus: func(st platform.Status) { m.setStatus(ctx, b.ID, st) },
		}
		err = adapter.Run(ctx, acc, creds, hooks)
		if ctx.Err() != nil {
			m.setStatus(ctx, b.ID, platform.Status{State: platform.StateStopped})
			return
		}

		reason := ""
		if err != nil {
			reason = err.Error()
		}
		m.setStatus(ctx, b.ID, platform.Status{State: platform.StateError, Reason: reason})

		if m.opt.Now().Sub(startedAt) > stableRun {
			attempt = 0
		}
		attempt++
		select {
		case <-time.After(m.opt.Backoff(attempt)):
		case <-ctx.Done():
			m.setStatus(ctx, b.ID, platform.Status{State: platform.StateStopped})
			return
		}

		// 重启前重读一次：凭据与元数据可能被更新。
		next, err := m.opt.Store.Bot(ctx, b.ID)
		if err != nil {
			return
		}
		b = next
	}
}

// setStatus 写库并通知外部；ctx 可能已取消，所以写库用独立超时。
func (m *Manager) setStatus(_ context.Context, botID string, st platform.Status) {
	st.Since = m.opt.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = m.opt.Store.SetBotStatus(ctx, botID, st)
	m.opt.OnStatus(botID, st)
}

func accountOf(b store.Bot) platform.Account {
	_, external, _ := platform.SplitID(b.ID)
	return platform.Account{
		ID:          external,
		Username:    b.Username,
		DisplayName: b.DisplayName,
		Caps:        b.Caps,
	}
}

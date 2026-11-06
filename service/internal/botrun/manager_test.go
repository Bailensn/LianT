package botrun_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Bailensn/LianT/service/internal/botrun"
	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/store"
)

const testBotID = "qqbot:1234567890"

type fakeAdapter struct {
	name  string
	run   func(ctx context.Context, h platform.Hooks) error
	calls atomic.Int32
}

func (a *fakeAdapter) Name() string { return a.name }

func (a *fakeAdapter) Validate(context.Context, platform.Credentials) (platform.Account, error) {
	return platform.Account{ID: "1"}, nil
}

func (a *fakeAdapter) Run(ctx context.Context, _ platform.Account, _ platform.Credentials, h platform.Hooks) error {
	a.calls.Add(1)
	return a.run(ctx, h)
}

func (a *fakeAdapter) SendText(context.Context, platform.Account, platform.Credentials, platform.SendRequest) (platform.MessageID, error) {
	return "", platform.ErrUnsupported
}

func (a *fakeAdapter) Format() string { return "qq-plain" }

type fakeIngestor struct {
	n atomic.Int32
}

func (i *fakeIngestor) Ingest(context.Context, string, platform.Inbound) (bool, error) {
	i.n.Add(1)
	return true, nil
}

func seedBot(t *testing.T, db *store.Store) {
	t.Helper()
	err := db.UpsertBot(context.Background(), store.Bot{
		ID:          testBotID,
		Platform:    "qqbot",
		Credentials: []byte("opaque"),
		DisplayName: "Example",
	})
	if err != nil {
		t.Fatalf("写入 Bot: %v", err)
	}
	if err := db.SetBotStatus(context.Background(), testBotID, platform.Status{State: platform.StateRunning}); err != nil {
		t.Fatalf("设置状态: %v", err)
	}
}

func newManager(t *testing.T, ad platform.Adapter, ing botrun.Ingestor, backoff func(int) time.Duration) (*botrun.Manager, *store.Store) {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/db.sqlite")
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	m, err := botrun.New(botrun.Options{
		Store:  db,
		Ingest: ing,
		Adapter: func(name string) (platform.Adapter, bool) {
			if name == ad.Name() {
				return ad, true
			}
			return nil, false
		},
		Decode:  func(store.Bot) (platform.Credentials, error) { return platform.Credentials{}, nil },
		Backoff: backoff,
	})
	if err != nil {
		t.Fatalf("构造 Manager: %v", err)
	}
	return m, db
}

func waitStatus(t *testing.T, db *store.Store, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := db.Bot(context.Background(), testBotID)
		if err == nil && b.Status.State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, _ := db.Bot(context.Background(), testBotID)
	t.Fatalf("等待状态 %s 超时，当前 %+v", want, b.Status)
}

func TestStartRunsAndIngests(t *testing.T) {
	ing := &fakeIngestor{}
	ad := &fakeAdapter{
		name: "qqbot",
		run: func(ctx context.Context, h platform.Hooks) error {
			h.OnStatus(platform.Status{State: platform.StateRunning})
			if err := h.OnMessage(ctx, platform.Inbound{
				MessageID: "m-1",
				Peer:      platform.Peer{ID: "qqbot:peer-1", Kind: platform.PeerGroup},
				Kind:      "text",
				Text:      "hi",
			}); err != nil {
				return err
			}
			<-ctx.Done()
			return nil
		},
	}
	m, db := newManager(t, ad, ing, nil)
	seedBot(t, db)

	if err := m.Start(context.Background(), testBotID); err != nil {
		t.Fatalf("启动: %v", err)
	}
	waitStatus(t, db, platform.StateRunning)
	if got := ing.n.Load(); got != 1 {
		t.Fatalf("入站处理次数不符: %d", got)
	}

	if err := m.Stop(testBotID); err != nil {
		t.Fatalf("停止: %v", err)
	}
	waitStatus(t, db, platform.StateStopped)
}

func TestRestartWithBackoff(t *testing.T) {
	ing := &fakeIngestor{}
	ad := &fakeAdapter{
		name: "qqbot",
		run: func(context.Context, platform.Hooks) error {
			return errors.New("upstream down")
		},
	}
	m, db := newManager(t, ad, ing, func(int) time.Duration { return 5 * time.Millisecond })
	seedBot(t, db)

	if err := m.Start(context.Background(), testBotID); err != nil {
		t.Fatalf("启动: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ad.calls.Load() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ad.calls.Load() < 2 {
		t.Fatalf("应至少重试一次，实际 %d", ad.calls.Load())
	}
	if err := m.Stop(testBotID); err != nil {
		t.Fatalf("停止: %v", err)
	}
}

func TestReconcileStartsRunningBots(t *testing.T) {
	ing := &fakeIngestor{}
	ad := &fakeAdapter{
		name: "qqbot",
		run: func(ctx context.Context, _ platform.Hooks) error {
			<-ctx.Done()
			return nil
		},
	}
	m, db := newManager(t, ad, ing, nil)
	seedBot(t, db)

	n, err := m.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("认领: %v", err)
	}
	if n != 1 {
		t.Fatalf("应认领 1 个 Bot，得到 %d", n)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ad.calls.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if ad.calls.Load() == 0 {
		t.Fatal("适配器未被启动")
	}
	m.Shutdown()
	waitStatus(t, db, platform.StateStopped)
}

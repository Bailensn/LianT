package manager

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"LianT/internal/bot"
	"LianT/internal/config"
	"LianT/internal/platform"
	qqapi "LianT/internal/platform/qq"
	qqstore "LianT/internal/platform/qq/store"
	_ "LianT/internal/platform/telegram"
	"LianT/internal/qqbot"
	"LianT/internal/rules"
	"LianT/internal/store"
)

// ============================================================================
// 多 Bot 管理器
//
//   * 注册表：<数据目录>/LianT.db 的 bots 表（name/token/mode/webhook/enabled）
//   * 业务库：<数据目录>/bots/<id>.db，每个 Bot 独立一份（规则/违禁词/群/禁言/审计）
//   * 一个任务进程里同时跑多个 Bot；每个 Bot 独立长轮询或 webhook
//
// 这样从 .env 里只有两份凭据，变成 SQLite 里无限个 Bot，每个 Bot 的数据互不污染。
// ============================================================================

type Runtime struct {
	Rec   store.BotRec
	Store *store.Store
	Bot   *bot.Bot   // platform=telegram
	QQ    *qqbot.Bot // platform=qq
}

type Manager struct {
	mu    sync.RWMutex
	reg   *store.Store
	items map[int64]*Runtime
}

func New(reg *store.Store) *Manager {
	return &Manager{reg: reg, items: map[int64]*Runtime{}}
}

// BotDBPath 返回某个 Bot 的业务库路径。
func BotDBPath(id int64) string {
	return filepath.Join(config.DataDir, "bots", fmt.Sprintf("%d.db", id))
}

// StartAll 启动所有 enabled 的 Bot（任务进程启动时调用一次）。
func (m *Manager) StartAll() {
	recs, err := m.reg.Bots()
	if err != nil {
		log.Printf("[!] 读取 Bot 注册表失败: %v", err)
		return
	}
	for _, rec := range recs {
		if !rec.Enabled {
			continue
		}
		if err := m.start(rec); err != nil {
			log.Printf("[!] 启动 Bot #%d(%s) 失败: %v", rec.ID, rec.Name, err)
			continue
		}
		log.Printf("[OK] Bot #%d @%s 已启动（%s）", rec.ID, rec.Username, BotDBPath(rec.ID))
	}
	if len(m.items) == 0 {
		log.Printf("[!] 还没有启用中的 Bot：在网页后台「机器人」里添加 token，或 ./LianT bot add")
	}
}

// Add 校验 token 后写入注册表并启动。
func (m *Manager) Add(rec store.BotRec) (int64, error) {
	rec.Token = trim(rec.Token)
	if rec.Token == "" {
		return 0, fmt.Errorf("token 不能为空")
	}
	if rec.Platform == "" {
		rec.Platform = "telegram"
	}
	ad := platform.Get(rec.Platform)
	if ad == nil {
		return 0, platform.ErrUnsupported(rec.Platform)
	}
	existing, err := m.reg.Bots()
	if err != nil {
		return 0, fmt.Errorf("读取现有 Bot 失败: %w", err)
	}
	for _, old := range existing {
		if old.Token == rec.Token {
			return 0, fmt.Errorf("这个 token 已经添加过（Bot #%d %s）", old.ID, old.Name)
		}
	}
	ident, err := ad.ValidateToken(rec.Token)
	if err != nil {
		return 0, fmt.Errorf("%s 凭据校验失败（检查凭据、网络或代理）: %w", ad.Name(), err)
	}
	if rec.Name == "" {
		rec.Name = ident.Name
		if strings.TrimSpace(rec.Name) == "" {
			rec.Name = "@" + ident.Username
		}
	}
	rec.Username = ident.Username
	rec.PlatformID = ident.ID
	if n, err := strconv.ParseInt(ident.ID, 10, 64); err == nil {
		rec.TelegramID = n
	}
	if rec.Mode == "" {
		rec.Mode = "polling"
	}
	if rec.WebhookPath == "" {
		rec.WebhookPath = "/telegram/webhook"
	}
	id, err := m.reg.AddBot(rec)
	if err != nil {
		return 0, fmt.Errorf("写入 Bot 注册表失败: %w", err)
	}
	rec.ID = id
	if rec.WebhookListen == "" {
		rec.WebhookListen = fmt.Sprintf("127.0.0.1:%d", 28743+int(id)-1)
		_ = m.reg.UpdateBot(rec)
	}

	if rec.Enabled {
		if err := m.start(rec); err != nil {
			_ = m.reg.DeleteBot(id)
			return 0, fmt.Errorf("Bot 已写入但启动失败，已回滚注册：%w", err)
		}
	}
	return id, nil
}

// Update 更新配置并重启该 Bot（token 为空表示保留原 token）。
func (m *Manager) Update(rec store.BotRec) error {
	old, ok := m.reg.Bot(rec.ID)
	if !ok {
		return fmt.Errorf("Bot #%d 不存在", rec.ID)
	}
	token := trim(rec.Token)
	if token == "" {
		token = old.Token
	}
	if rec.Platform == "" {
		rec.Platform = old.Platform
	}
	if rec.Platform == "" {
		rec.Platform = "telegram"
	}
	rec.Token = token
	if rec.Name == "" {
		rec.Name = old.Name
	}
	if rec.Mode == "" {
		rec.Mode = "polling"
	}

	// token / 平台没变就不打网络：允许离线改名、改 webhook、停用。
	if token != old.Token || rec.Platform != old.Platform {
		existing, err := m.reg.Bots()
		if err != nil {
			return fmt.Errorf("读取现有 Bot 失败: %w", err)
		}
		for _, other := range existing {
			if other.ID != rec.ID && other.Token == token {
				return fmt.Errorf("这个 token 已经属于 Bot #%d %s", other.ID, other.Name)
			}
		}
		ad := platform.Get(rec.Platform)
		if ad == nil {
			return platform.ErrUnsupported(rec.Platform)
		}
		ident, err := ad.ValidateToken(token)
		if err != nil {
			return fmt.Errorf("%s 凭据校验失败（检查凭据、网络或代理）: %w", ad.Name(), err)
		}
		rec.Username = ident.Username
		rec.PlatformID = ident.ID
		if n, err := strconv.ParseInt(ident.ID, 10, 64); err == nil {
			rec.TelegramID = n
		}
	} else {
		rec.Username = old.Username
		rec.PlatformID = old.PlatformID
		rec.TelegramID = old.TelegramID
	}

	if err := m.reg.UpdateBot(rec); err != nil {
		return err
	}
	m.stop(rec.ID)
	if rec.Enabled {
		if err := m.start(rec); err != nil {
			return err
		}
	}
	return nil
}

// Delete 停止并移除注册表；业务库文件保留，避免误删数据。
func (m *Manager) Delete(id int64) error {
	m.stop(id)
	return m.reg.DeleteBot(id)
}

// Restart 重启指定 Bot。
func (m *Manager) Restart(id int64) error {
	rec, ok := m.reg.Bot(id)
	if !ok {
		return fmt.Errorf("Bot #%d 不存在", id)
	}
	m.stop(id)
	if !rec.Enabled {
		return fmt.Errorf("Bot #%d 已停用", id)
	}
	return m.start(rec)
}

// Reconcile 让运行中的实例与注册表对齐：启动新启用的、停止已删除/停用的。
// CLI 改了 bots 表后调用它即可，不用重启任务进程。
func (m *Manager) Reconcile() []string {
	recs, err := m.reg.Bots()
	if err != nil {
		return []string{"读取 Bot 注册表失败: " + err.Error()}
	}
	want := map[int64]store.BotRec{}
	for _, rec := range recs {
		if rec.Enabled {
			want[rec.ID] = rec
		}
	}
	m.mu.RLock()
	var stopIDs []int64
	for id := range m.items {
		if _, ok := want[id]; !ok {
			stopIDs = append(stopIDs, id)
		}
	}
	m.mu.RUnlock()
	for _, id := range stopIDs {
		m.stop(id)
	}
	var errs []string
	for id, rec := range want {
		if _, ok := m.Get(id); ok {
			continue
		}
		if err := m.start(rec); err != nil {
			msg := fmt.Sprintf("启动 Bot #%d(%s) 失败: %v", id, rec.Name, err)
			log.Printf("[!] %s", msg)
			errs = append(errs, msg)
		}
	}
	return errs
}

// ReloadAll 让所有运行中的 Bot 尽快重连（不重启进程）。
func (m *Manager) ReloadAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, rt := range m.items {
		if rt.Bot != nil {
			_ = rt.Bot.ReloadAll()
		}
		if rt.QQ != nil {
			_ = rt.QQ.ReloadAll()
		}
	}
}

// StopAll 退出任务进程时调用。
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rt := range m.items {
		if rt.Bot != nil {
			rt.Bot.Stop(10 * time.Second)
		}
		if rt.QQ != nil {
			rt.QQ.Stop(10 * time.Second)
		}
		if rt.Store != nil {
			_ = rt.Store.Close()
		}
		delete(m.items, id)
	}
}

// Get 取一个正在运行的 Bot（停用/未启动返回 false）。
func (m *Manager) Get(id int64) (*Runtime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.items[id]
	return rt, ok
}

// First 取第一个运行中的 Bot（旧的单 Bot 调用方兜底）。
func (m *Manager) First() (*Runtime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]int64, 0, len(m.items))
	for id := range m.items {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) == 0 {
		return nil, false
	}
	rt := m.items[ids[0]]
	return rt, true
}

// List 返回注册表里全部 Bot（含停用的），并标记是否运行中。
func (m *Manager) List() ([]View, error) {
	recs, err := m.reg.Bots()
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]View, 0, len(recs))
	for _, rec := range recs {
		v := View{BotRec: rec, Running: false}
		if rt, ok := m.items[rec.ID]; ok {
			v.Running = true
			v.State = runtimeState(rt)
			v.Online = isOnlineState(v.State)
		}
		v.TokenSet = rec.Token != ""
		if ad := platform.Get(rec.Platform); ad != nil {
			v.PlatformName = ad.Name()
			v.Capabilities = ad.Capabilities()
		}
		out = append(out, v)
	}
	return out, nil
}

// View 是管理端用的 Bot 视图（不返回 token 明文）。
type View struct {
	store.BotRec
	Running      bool                `json:"running"` // 进程里有这个运行时（不代表已经连上）
	Online       bool                `json:"online"`  // 平台连接真的建立了（在线/已连接）
	State        string              `json:"state"`   // 运行时自报的状态文本
	TokenSet     bool                `json:"token_set"`
	PlatformName string              `json:"platform_name"`
	Capabilities platform.Capabilities `json:"capabilities"`
}

// runtimeState 取运行时自报的状态文本（bot.Status()/q.Status() 的 bot_state）。
func runtimeState(rt *Runtime) string {
	if rt == nil {
		return ""
	}
	if rt.Bot != nil {
		if st := rt.Bot.Status(); st != nil {
			return st["bot_state"]
		}
	}
	if rt.QQ != nil {
		if st := rt.QQ.Status(); st != nil {
			return st["bot_state"]
		}
	}
	return ""
}

// isOnlineState 把各平台的「真连上了」统一成一个判断：
// Telegram 用「在线」，QQ 用「已连接」。
func isOnlineState(state string) bool {
	switch strings.TrimSpace(state) {
	case "在线", "已连接":
		return true
	}
	return false
}

// migrateLegacyTo 第一次启动最低 ID 的 Bot 时，把旧单 Bot 注册表库里的业务数据复制过来。
func (m *Manager) migrateLegacyTo(id int64) {
	if m.reg.Setting("multi_bot_migrated", "") != "" || !m.reg.HasBusinessData() {
		return
	}
	recs, err := m.reg.Bots()
	if err != nil || len(recs) == 0 || recs[0].ID != id {
		return
	}
	path := BotDBPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if tmp, err := store.Open(path); err == nil {
		_ = tmp.Close()
		if err := m.reg.CopyBusinessTo(path); err != nil {
			log.Printf("[!] 旧单 Bot 数据迁移失败（Bot 会从空库启动）: %v", err)
		} else {
			log.Printf("[OK] 已把旧单 Bot 数据迁移到 Bot #%d 的独立库: %s", id, path)
		}
	}
}

func (m *Manager) start(rec store.BotRec) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[rec.ID]; ok {
		return nil
	}
	path := BotDBPath(rec.ID)
	m.migrateLegacyTo(rec.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 Bot 数据目录失败: %w", err)
	}
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("打开业务库失败: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	seedDefaults(st)
	rt := &Runtime{Rec: rec, Store: st}
	switch rec.Platform {
	case "", "telegram":
		b := bot.New(bot.Config{
			Store:         st,
			ID:            rec.ID,
			Name:          rec.Name,
			Platform:      "telegram",
			DBFile:        path,
			Token:         rec.Token,
			Mode:          rec.Mode,
			WebhookURL:    rec.WebhookURL,
			WebhookSecret: rec.WebhookSecret,
			WebhookListen: rec.WebhookListen,
			WebhookPath:   rec.WebhookPath,
		})
		b.Start()
		rt.Bot = b
	case "qq":
		appID, secret, ok := qqapi.SplitToken(rec.Token)
		if !ok {
			_ = st.Close()
			return fmt.Errorf("QQ 凭据格式应为 AppID:AppSecret")
		}
		q := qqbot.New(qqbot.Config{
			AppID: appID, AppSecret: secret, Intents: (1 << 25) | (1 << 30),
			Name: rec.Name, DBFile: path, Store: qqstore.New(st),
		})
		q.Start()
		rt.QQ = q
	default:
		_ = st.Close()
		return platform.ErrUnsupported(rec.Platform)
	}
	m.items[rec.ID] = rt
	return nil
}

func (m *Manager) stop(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt, ok := m.items[id]
	if !ok {
		return
	}
	if rt.Bot != nil {
		rt.Bot.Stop(10 * time.Second)
	}
	if rt.QQ != nil {
		rt.QQ.Stop(10 * time.Second)
	}
	if rt.Store != nil {
		_ = rt.Store.Close()
	}
	delete(m.items, id)
}

func seedDefaults(st *store.Store) {
	recs, err := st.Rules()
	if err != nil || len(recs) > 0 {
		return
	}
	dc := rules.DefaultConfig()
	_ = st.SetSetting("reply_all", store.BoolStr(dc.IsReplyAll()))
	_ = st.SetSetting("reply_default", dc.DefaultText())
	for i, r := range dc.Rules {
		reply := ""
		if r.Reply != nil {
			reply = *r.Reply
		}
		_, _ = st.SaveRule(store.RuleRec{
			Priority: 100 + i, Match: r.Match, Pattern: r.Pattern, Reply: reply,
			Fold: r.Fold, Always: r.Always, Enabled: true, Note: "初始规则",
		})
	}
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"LianT/internal/config"
	"LianT/internal/store"
	"LianT/internal/telegram"
)

// ============================================================================
// LianT 的 Telegram Bot 本体
//
//   daemon.Run -> New(...) -> admin.Start() -> Start() -> Supervise() -> run()
//
// 用 getUpdates 长轮询（不需要公网回调/域名/证书）：
//   * 断线自动退避重连
//   * 收到 update 先落 offset，再处理消息（重启不会重复处理）
//   * 规则/违禁词走 SQLite 缓存，网页一改下一条消息就生效
// ============================================================================

type Config struct {
	Store *store.Store

	// 多 Bot：每个运行时一份自己的 token / webhook 配置（来自 SQLite bots 表）。
	ID       int64
	Name     string
	Platform string
	DBFile   string
	Token    string
	Mode string // polling | webhook
	WebhookURL    string
	WebhookSecret string
	WebhookListen string
	WebhookPath   string
}

type Bot struct {
	cfg       Config
	st        *store.Store
	startedAt time.Time

	mu      sync.RWMutex
	client  *telegram.Client
	me      telegram.User
	state   string
	lastErr string

	lastMsgAt  time.Time
	msgCount   int64
	reconnects int64

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}

	// reloadFlag 由 ReloadAll 置位，轮询循环看到后主动退出重连（拿新 token/规则）。
	reloadFlag atomic.Bool

	// recent 是最近见过的消息，用于「撤回 上一条/某人最后一条/批量撤回」。
	recentMu sync.Mutex
	recent   map[string][]RecentMsg

	// roleCache 缓存群成员角色，避免每条消息都调 getChatMember。
	roleMu    sync.Mutex
	roleCache map[string]roleEntry

	rc RulesCache
}

type roleEntry struct {
	role string
	at   time.Time
}

// New 创建 Bot（不连接）。
func New(c Config) *Bot {
	return &Bot{
		cfg:       c,
		st:        c.Store,
		startedAt: time.Now(),
		state:     "启动中",
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		recent:    map[string][]RecentMsg{},
		roleCache: map[string]roleEntry{},
	}
}

// ---------------- 最近消息 ----------------

// RecentMsg 一条见过的消息。
type RecentMsg struct {
	MessageID int64
	UserID    string
	Username  string
	Content   string
	Kind      string
	IsBot     bool
	At        time.Time
}

const RecentKeep = 200

// Classify 把 Telegram 消息归类（只用于日志/审计/撤回提示）。
func Classify(m *telegram.Message) string {
	if m == nil {
		return "other"
	}
	switch {
	case len(m.Photo) > 0:
		return "photo"
	case m.Video != nil:
		return "video"
	case m.Voice != nil:
		return "voice"
	case m.Audio != nil:
		return "audio"
	case m.Document != nil:
		return "document"
	case m.Sticker != nil:
		return "sticker"
	case m.Animation != nil:
		return "animation"
	case m.VideoNote != nil:
		return "video_note"
	case m.Poll != nil:
		return "poll"
	case m.Location != nil:
		return "location"
	case m.Contact != nil:
		return "contact"
	case m.Text != "":
		return "text"
	case m.Caption != "":
		return "caption"
	}
	return "other"
}

// KindLabel 给日志/审计看的短标签。
func KindLabel(kind string) string {
	switch kind {
	case "photo":
		return "[图片]"
	case "video":
		return "[视频]"
	case "voice":
		return "[语音]"
	case "audio":
		return "[音频]"
	case "document":
		return "[文件]"
	case "sticker":
		return "[贴纸]"
	case "animation":
		return "[动图]"
	case "video_note":
		return "[圆头视频]"
	case "poll":
		return "[投票]"
	case "location":
		return "[位置]"
	case "contact":
		return "[联系人]"
	case "bot":
		return "[机器人]"
	}
	return ""
}

func (b *Bot) remember(m *telegram.Message) {
	if m == nil || m.MessageID == 0 {
		return
	}
	chatID := strconv.FormatInt(m.Chat.ID, 10)
	userID, username, isBot := "", "", false
	if m.From != nil {
		userID, username, isBot = strconv.FormatInt(m.From.ID, 10), m.From.Display(), m.From.IsBot
	} else if m.SenderChat != nil {
		userID, username = strconv.FormatInt(m.SenderChat.ID, 10), m.SenderChat.Display()
	}
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := append(b.recent[chatID], RecentMsg{
		MessageID: m.MessageID, UserID: userID, Username: username,
		Content: m.Content(), Kind: Classify(m), IsBot: isBot, At: time.Now(),
	})
	if len(list) > RecentKeep {
		list = list[len(list)-RecentKeep:]
	}
	b.recent[chatID] = list
}

// RememberOutgoing 让「撤回机器人」能找到机器人自己刚发的消息。
func (b *Bot) RememberOutgoing(chatID string, messageID int64, content string) {
	if chatID == "" || messageID == 0 {
		return
	}
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := append(b.recent[chatID], RecentMsg{
		MessageID: messageID, Username: "机器人", Content: content,
		Kind: "bot", IsBot: true, At: time.Now(),
	})
	if len(list) > RecentKeep {
		list = list[len(list)-RecentKeep:]
	}
	b.recent[chatID] = list
}

// RecentList 取最近 n 条（时间顺序），onlyWithin>0 时只看该时间窗内的。
func (b *Bot) RecentList(chatID string, n int, onlyWithin time.Duration) []RecentMsg {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recent[chatID]
	var out []RecentMsg
	for i := len(list) - 1; i >= 0 && len(out) < n; i-- {
		if list[i].MessageID == 0 {
			continue
		}
		if onlyWithin > 0 && time.Since(list[i].At) > onlyWithin {
			continue
		}
		out = append(out, list[i])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// LookupRecent 按消息 ID 查最近消息。
func (b *Bot) LookupRecent(chatID string, messageID int64) (RecentMsg, bool) {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recent[chatID]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].MessageID == messageID {
			return list[i], true
		}
	}
	return RecentMsg{}, false
}

// LookupRecentByMember 找某人的最后一条消息。
func (b *Bot) LookupRecentByMember(chatID, userID string) (RecentMsg, bool) {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recent[chatID]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].UserID == userID {
			return list[i], true
		}
	}
	return RecentMsg{}, false
}

// LookupBotRecent 机器人自己最近发的一条消息。
func (b *Bot) LookupBotRecent(chatID string, onlyWithin time.Duration) (RecentMsg, bool) {
	b.recentMu.Lock()
	defer b.recentMu.Unlock()
	list := b.recent[chatID]
	for i := len(list) - 1; i >= 0; i-- {
		if !list[i].IsBot || list[i].MessageID == 0 {
			continue
		}
		if onlyWithin > 0 && time.Since(list[i].At) > onlyWithin {
			return RecentMsg{}, false
		}
		return list[i], true
	}
	return RecentMsg{}, false
}

// ---------------- 生命周期 ----------------

// Start 起一个后台协程跑业务，自身立即返回。
func (b *Bot) Start() { go b.Supervise() }

// Stop 优雅停止（最多等 timeout）。
func (b *Bot) Stop(timeout time.Duration) {
	b.stopOnce.Do(func() { close(b.stopCh) })
	select {
	case <-b.doneCh:
	case <-time.After(timeout):
		log.Printf("[!] 等待机器人停止超时(%s)，继续退出", timeout)
	}
}

func (b *Bot) Stopped() bool {
	select {
	case <-b.stopCh:
		return true
	default:
		return false
	}
}

// Supervise 断线自动重连，直到 Stop。
func (b *Bot) Supervise() {
	defer close(b.doneCh)
	log.Printf("[bot] LianT 已启动（Telegram 长轮询）")
	backoff := time.Second
	for {
		if b.Stopped() {
			b.setState("已停止")
			log.Printf("[bot] 已停止")
			return
		}
		b.setState("连接中")
		started := time.Now()
		err := b.run()
		if b.Stopped() {
			b.setState("已停止")
			return
		}
		if err != nil {
			b.setLastErr(err.Error())
			log.Printf("[!] %v", err)
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second
		}
		log.Printf("[i] %s 后重连", backoff.Round(time.Second))
		select {
		case <-b.stopCh:
			b.setState("已停止")
			return
		case <-time.After(backoff):
		}
		atomic.AddInt64(&b.reconnects, 1)
		if backoff < 60*time.Second {
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
		}
	}
}

// run 一次完整的连接 + 轮询会话，返回即表示断开，需要重连。
func (b *Bot) run() error {
	token := strings.TrimSpace(b.cfg.Token)
	if token == "" {
		name := b.cfg.Name
		if name == "" {
			name = "Bot"
		}
		return fmt.Errorf("%s 未配置 Token：请在网页后台「机器人」里填写", name)
	}
	c, err := telegram.New(token, config.APIBase(), config.Proxy())
	if err != nil {
		return err
	}
	me, err := c.GetMe()
	if err != nil {
		return fmt.Errorf("校验 Bot Token 失败: %w", err)
	}

	b.mu.Lock()
	b.client, b.me = c, me
	b.mu.Unlock()

	log.Printf("[OK] 已连接 @%s（ID %d，API %s）", me.Username, me.ID, config.APIBase())
	b.registerCommands(c)
	b.setState("在线")
	b.setLastErr("")
	go b.ensureAllGroupsAdmin()

	// 配了公网 webhook 就走回调；否则长轮询。
	if strings.EqualFold(strings.TrimSpace(b.cfg.Mode), "webhook") || strings.TrimSpace(b.cfg.WebhookURL) != "" {
		if strings.TrimSpace(b.cfg.WebhookURL) == "" {
			return fmt.Errorf("Bot %s 配置为 webhook，但没有 Webhook URL", b.cfg.Name)
		}
		return b.runWebhook(c)
	}
	if err := c.DeleteWebhook(false); err != nil {
		log.Printf("[!] 关闭 webhook 失败（如果设置了 webhook，轮询会 409）: %v", err)
	}

	offset := b.loadOffset()
	savedAt := time.Now()
	defer func() {
		if offset > 0 {
			b.saveOffset(offset)
		}
	}()
	for {
		if b.Stopped() {
			return nil
		}
		updates, err := c.GetUpdates(offset, 30)
		if err != nil {
			if b.Stopped() {
				return nil
			}
			if ae, ok := err.(*telegram.APIError); ok && ae.Code == 409 {
				log.Printf("[!] getUpdates 409：存在 webhook，自动删除后重试")
				_ = c.DeleteWebhook(false)
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			b.handleUpdate(c, u)
		}
		// offset 不要每条消息都写库（会 bump revision、让规则缓存反复失效）：
		// 最多 5 秒落一次；重启时最多重放几秒的消息。
		if time.Since(savedAt) > 5*time.Second {
			b.saveOffset(offset)
			savedAt = time.Now()
		}
		// 改了 .env / 收到 reload：主动退出，让 Supervise 立即重建连接。
		if b.reloadFlag.Swap(false) || strings.TrimSpace(b.cfg.Token) != token {
			log.Printf("[i] 配置已变化，重连 Telegram…")
			return nil
		}
	}
}

func (b *Bot) registerCommands(c *telegram.Client) {
	cmds := []map[string]string{
		{"command": "start", "description": "启动 / 查看机器人状态"},
		{"command": "help", "description": "查看可用指令"},
		{"command": "status", "description": "本群状态（管理员）"},
		{"command": "mute", "description": "禁言：/mute 10m（回复某人）"},
		{"command": "unmute", "description": "解除禁言（回复某人）"},
		{"command": "ban", "description": "封禁/踢出（回复某人）"},
		{"command": "unban", "description": "解除封禁"},
		{"command": "del", "description": "撤回消息（回复它）"},
		{"command": "pin", "description": "置顶消息（回复它）"},
		{"command": "unpin", "description": "取消置顶"},
		{"command": "invite", "description": "创建群邀请链接"},
		{"command": "topic", "description": "创建论坛话题"},
		{"command": "settitle", "description": "修改群名"},
		{"command": "setdesc", "description": "修改群简介"},
		{"command": "rules", "description": "查看规则数量"},
		{"command": "me", "description": "查看机器人信息"},
	}
	if err := c.SetMyCommands(cmds); err != nil {
		log.Printf("[!] 设置命令菜单失败: %v", err)
	}
}

// ---------------- 事件分发 ----------------

func (b *Bot) handleUpdate(c *telegram.Client, u telegram.Update) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[X] 处理更新 panic: %v", r)
		}
	}()

	switch {
	case u.Message != nil:
		b.handleMessage(c, u.Message)
	case u.ChannelPost != nil:
		b.handleMessage(c, u.ChannelPost)
	case u.MyChatMember != nil:
		b.handleMyChatMember(u.MyChatMember)
	case u.ChatMember != nil:
		b.handleChatMember(u.ChatMember)
	case u.ChatJoinRequest != nil:
		b.handleJoinRequest(c, u.ChatJoinRequest)
	case u.CallbackQuery != nil:
		b.handleCallbackQuery(c, u.CallbackQuery)
	case u.InlineQuery != nil:
		b.handleInlineQuery(c, u.InlineQuery)
	case u.ChosenInlineResult != nil:
		log.Printf("[inline] 用户 %s 选择了结果 %s", u.ChosenInlineResult.From.Display(), u.ChosenInlineResult.ResultID)
	case u.PollAnswer != nil:
		log.Printf("[poll] 投票 %s 有新回答", u.PollAnswer.PollID)
	case u.PreCheckoutQuery != nil:
		autoPay := b.st != nil && strings.EqualFold(b.st.Setting("payment_auto_approve", "false"), "true")
		p := map[string]interface{}{
			"pre_checkout_query_id": u.PreCheckoutQuery.ID,
			"ok":                    autoPay,
		}
		if !autoPay {
			p["error_message"] = "LianT 未开启自动确认支付（settings.payment_auto_approve）"
		}
		_ = c.Call("answerPreCheckoutQuery", p, nil)
	case u.ShippingQuery != nil:
		autoPay2 := b.st != nil && strings.EqualFold(b.st.Setting("payment_auto_approve", "false"), "true")
		sp := map[string]interface{}{
			"shipping_query_id": u.ShippingQuery.ID,
			"ok":                autoPay2,
			"shipping_options":  []interface{}{},
		}
		if !autoPay2 {
			sp["error_message"] = "LianT 未开启自动确认配送"
		}
		_ = c.Call("answerShippingQuery", sp, nil)
	case u.BusinessConnection != nil:
		log.Printf("[business] 连接 %s 用户 %s enabled=%v", u.BusinessConnection.ID, u.BusinessConnection.User.Display(), u.BusinessConnection.IsEnabled)
	case u.BusinessMessage != nil:
		b.handleMessage(c, u.BusinessMessage)
	case u.DeletedBusinessMessages != nil:
		log.Printf("[business] 消息被删除 chat=%d count=%d", u.DeletedBusinessMessages.Chat.ID, len(u.DeletedBusinessMessages.MessageIDs))
	case u.EditedMessage != nil:
		log.Printf("[edit] 忽略编辑消息 chat=%s msg=%d", chatIDStr(u.EditedMessage.Chat.ID), u.EditedMessage.MessageID)
	}
}

// handleCallbackQuery 处理内联按钮回调。约定 data 格式：
//   noop / ack
//   del:<message_id>
//   mute:<user_id>:<seconds>
//   unmute:<user_id> / ban:<user_id> / unban:<user_id>
func (b *Bot) handleCallbackQuery(c *telegram.Client, q *telegram.CallbackQuery) {
	if q == nil {
		return
	}
	answer := "已收到"
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[X] 处理按钮回调 panic: %v", r)
			answer = "处理失败"
		}
		if len([]rune(answer)) > 180 {
			answer = string([]rune(answer)[:180])
		}
		_ = c.AnswerCallbackQuery(q.ID, answer)
	}()

	if q.Message == nil {
		answer = "这个按钮没有绑定会话"
		return
	}
	chatID := chatIDStr(q.Message.Chat.ID)
	userID := chatIDStr(q.From.ID)
	data := strings.TrimSpace(q.Data)
	if data == "" || data == "noop" || data == "ack" {
		return
	}
	parts := strings.Split(data, ":")
	action := parts[0]

	// 群管理类按钮必须是管理员/后台管理员。
	role := b.memberRole(c, chatID, userID)
	_, backendAdmin := b.st.AdminRole(chatID, userID)
	if !(backendAdmin || role == "admin" || role == "owner") {
		answer = "只有管理员能操作"
		return
	}

	switch action {
	case "del":
		if len(parts) < 2 {
			answer = "缺少消息 ID"
			return
		}
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			answer = "消息 ID 不合法"
			return
		}
		if err := c.DeleteMessage(chatID, id); err != nil {
			answer = errText(err)
			return
		}
		answer = "已撤回"
	case "mute":
		if len(parts) < 3 {
			answer = "缺少参数"
			return
		}
		sec, _ := strconv.Atoi(parts[2])
		if sec <= 0 {
			sec = 600
		}
		res := b.MuteMember(ActionCtx{
			ChatID: chatID, UserID: parts[1], Kind: "callback", Hit: "按钮禁言", Operator: q.From.Display(),
		}, time.Duration(sec)*time.Second, "按钮禁言")
		answer = JoinResults(res, "已禁言")
	case "unmute":
		if len(parts) < 2 {
			answer = "缺少用户 ID"
			return
		}
		res := b.UnmuteMember(ActionCtx{
			ChatID: chatID, UserID: parts[1], Kind: "callback", Hit: "按钮解禁", Operator: q.From.Display(),
		})
		answer = JoinResults(res, "已解除禁言")
	case "ban":
		if len(parts) < 2 {
			answer = "缺少用户 ID"
			return
		}
		res := b.BanMember(ActionCtx{
			ChatID: chatID, UserID: parts[1], Kind: "callback", Hit: "按钮封禁", Operator: q.From.Display(),
		}, true)
		answer = JoinResults(res, "已封禁")
	case "unban":
		if len(parts) < 2 {
			answer = "缺少用户 ID"
			return
		}
		_ = b.st.RemoveBlacklist(chatID, parts[1])
		res := b.UnbanMember(ActionCtx{
			ChatID: chatID, UserID: parts[1], Kind: "callback", Hit: "按钮解封", Operator: q.From.Display(),
		})
		answer = JoinResults(res, "已解封")
	default:
		answer = "未知按钮操作"
	}
}

// handleInlineQuery 处理 @机器人 的内联查询：把 settings.inline_results 的每行作为一条结果。
func (b *Bot) handleInlineQuery(c *telegram.Client, q *telegram.InlineQuery) {
	if q == nil {
		return
	}
	raw := ""
	if b.st != nil {
		raw = b.st.Setting("inline_results", "")
	}
	var results []map[string]interface{}
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		title, content := line, line
		if idx := strings.Index(line, "|"); idx > 0 {
			title = strings.TrimSpace(line[:idx])
			content = strings.TrimSpace(line[idx+1:])
		}
		if len([]rune(title)) > 60 {
			title = string([]rune(title)[:60])
		}
		results = append(results, map[string]interface{}{
			"type":        "article",
			"id":          fmt.Sprintf("LianT-%d", i),
			"title":       title,
			"description": truncateLog(content, 80),
			"input_message_content": map[string]interface{}{
				"message_text": content,
			},
		})
	}
	if len(results) == 0 {
		results = append(results, map[string]interface{}{
			"type":        "article",
			"id":          "LianT-help",
			"title":       "LianT · Telegram 群管理机器人",
			"description": "发送 /help 查看指令",
			"input_message_content": map[string]interface{}{
				"message_text": HelpText(),
			},
		})
	}
	if err := c.AnswerInlineQuery(q.ID, results, 60); err != nil {
		log.Printf("[X] 回复内联查询失败: %v", err)
	}
}

// allowedUpdates 是长轮询/webhook 共用的更新类型。
func allowedUpdates() []string {
	return []string{
		"message", "edited_message", "channel_post",
		"my_chat_member", "chat_member", "chat_join_request", "callback_query",
		"inline_query", "chosen_inline_result", "poll_answer",
		"pre_checkout_query", "shipping_query",
		"business_connection", "business_message", "edited_business_message",
		"deleted_business_messages",
	}
}

// runWebhook 走 webhook 模式（需要公网 HTTPS），收到更新后交给 handleUpdate。
func (b *Bot) runWebhook(c *telegram.Client) error {
	if err := c.SetWebhook(b.cfg.WebhookURL, b.cfg.WebhookSecret, allowedUpdates()); err != nil {
		return fmt.Errorf("设置 webhook 失败: %w", err)
	}
	path := b.webhookPath()
	secret := b.cfg.WebhookSecret
	if strings.TrimSpace(secret) == "" {
		log.Printf("[!] Webhook 模式没有设置 secret_token：能访问 %s%s 的人都能伪造更新，建议在「机器人」里填 Webhook Secret", b.webhookListen(), path)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != secret {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var u telegram.Update
		if err := json.Unmarshal(body, &u); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		go b.handleUpdate(c, u)
	})
	srv := &http.Server{Addr: b.webhookListen(), Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	log.Printf("[OK] Webhook 模式：%s -> %s%s", b.cfg.WebhookURL, b.webhookListen(), path)
	select {
	case <-b.stopCh:
		_ = srv.Close()
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("webhook 服务退出: %w", err)
	}
}

func (b *Bot) handleMessage(c *telegram.Client, m *telegram.Message) {
	if m == nil || m.From == nil {
		return
	}
	if m.From.IsBot {
		return
	}
	atomic.AddInt64(&b.msgCount, 1)
	b.mu.Lock()
	b.lastMsgAt = time.Now()
	b.mu.Unlock()

	chatID := chatIDStr(m.Chat.ID)
	userID := chatIDStr(m.From.ID)
	content := strings.TrimSpace(m.Content())
	log.Printf("[消息] %s | %s(%s): %s", chatLabel(m.Chat), m.From.Display(), userID, truncateLog(content, 120))

	b.remember(m)

	if m.Chat.IsGroup() {
		b.handleGroupMessage(c, m, chatID, userID)
		return
	}
	b.handlePrivateMessage(c, m, chatID, userID)
}

func (b *Bot) handlePrivateMessage(c *telegram.Client, m *telegram.Message, chatID, userID string) {
	if b.st != nil && b.st.IsBlacklisted("", userID) {
		log.Printf("[blacklist] 私聊用户 %s 在黑名单，忽略", userID)
		return
	}
	text := strings.TrimSpace(m.Content())
	if isStart(text) {
		b.reply(c, m, "LianT 在线 ✅\n发送 /help 查看指令；把机器人拉进群并设为管理员即可管理群聊。", 0)
		return
	}
	if isHelp(text) {
		b.reply(c, m, HelpText(), 0)
		return
	}
	_, c2c, _ := b.Rules()
	res := c2c.Evaluate(text, "私聊", "", userID)
	if !res.Send {
		return
	}
	b.reply(c, m, res.Text, 0)
}

func (b *Bot) handleGroupMessage(c *telegram.Client, m *telegram.Message, chatID, userID string) {
	if b.st == nil {
		return
	}
	_ = b.st.EnsureGroup(chatID, m.Chat.Display())
	role := b.memberRole(c, chatID, userID)
	_ = b.st.TouchMember(chatID, userID, m.From.Display(), role, false)

	g, ok := b.st.Group(chatID)
	if ok && !g.Enabled {
		log.Printf("[skip] 群 %s 已停用，忽略消息", chatID)
		return
	}

	if len(m.NewChatMembers) > 0 {
		b.handleNewMembers(c, m, chatID)
		return
	}
	if m.LeftChatMember != nil {
		log.Printf("[成员] 离开 %s: %s", chatID, m.LeftChatMember.Display())
		return
	}

	if b.st.IsBlacklisted(chatID, userID) {
		_ = c.DeleteMessage(chatID, m.MessageID)
		log.Printf("[blacklist] %s(%s) 被拦截并撤回", m.From.Display(), userID)
		return
	}

	if handled, reply := b.handleCommand(c, m, chatID, userID, role); handled {
		if reply != "" {
			b.reply(c, m, reply, 0)
		}
		return
	}

	group, _, words := b.Rules()

	if w, word, ok := words.Match(chatID, m.Content()); ok {
		plan := ActionPlan{
			Reply:       w.Reply,
			Recall:      w.Recall,
			MuteSeconds: w.MuteSeconds,
			Kick:        w.Kick,
			Blacklist:   w.Blacklist,
			Reason:      "违禁词:" + word,
		}
		log.Printf("[words] 命中违禁词 %q（severity=%d）", word, w.Severity)
		results := b.ApplyPlan(ActionCtx{
			ChatID: chatID, UserID: userID, Username: m.From.Display(),
			MessageID: m.MessageID, Content: m.Content(), Kind: "word",
			Hit: "违禁词:" + word, Operator: "system",
		}, plan)
		if plan.Reply != "" {
			b.reply(c, m, plan.Reply, 0)
		}
		log.Printf("[words] 处置结果: %s", strings.Join(results, "；"))
		return
	}

	mentioned := m.IsMentioned(b.me.Username, b.me.ID)
	// 群里的 / 命令即使没 @ 机器人也可能是发给它的（privacy mode 关闭时），照常走规则。
	if !mentioned && !group.IsReplyAll() && !strings.HasPrefix(strings.TrimSpace(m.Content()), "/") {
		return
	}
	content := m.Content()
	if mentioned {
		content = m.StripBotMention(b.me.Username)
	}
	res := group.Evaluate(content, "群消息", chatID, userID)
	if !res.Send {
		log.Printf("[skip] 群消息不回（%s）", res.Rule)
		return
	}
	b.reply(c, m, res.Text, 0)
	_ = b.st.AddAudit(store.AuditRec{
		Kind: "reply", ChatID: chatID, UserID: userID, Username: m.From.Display(),
		Content: truncateLog(content, 500), Hit: res.Rule, Actions: `["reply"]`,
		Result: "ok", Detail: truncateLog(res.Text, 200),
	})
}

func (b *Bot) handleNewMembers(c *telegram.Client, m *telegram.Message, chatID string) {
	for _, u := range m.NewChatMembers {
		role := "member"
		if u.ID == b.me.ID {
			role = "bot"
		}
		_ = b.st.TouchMember(chatID, chatIDStr(u.ID), u.Display(), role, u.IsBot)
	}
	welcome := strings.TrimSpace(b.st.Setting("welcome_text", ""))
	if welcome == "" {
		return
	}
	for _, u := range m.NewChatMembers {
		if u.IsBot {
			continue
		}
		text := strings.NewReplacer(
			"{name}", u.Display(), "{username}", u.At(), "{chat}", m.Chat.Display(),
		).Replace(welcome)
		b.reply(c, m, text, 0)
	}
}

func (b *Bot) handleMyChatMember(ev *telegram.ChatMemberUpdated) {
	chatID := chatIDStr(ev.Chat.ID)
	status := ev.NewChatMember.Status
	log.Printf("[bot] 机器人状态变化 chat=%s(%s): %s → %s", chatID, ev.Chat.Display(), ev.OldChatMember.Status, status)
	if b.st == nil {
		return
	}
	switch status {
	case "member", "administrator", "creator":
		_ = b.st.EnsureGroup(chatID, ev.Chat.Display())
		isAdmin := ev.NewChatMember.IsAdmin()
		_ = b.st.UpdateGroup(chatID, strPtr(ev.Chat.Display()), nil, nil, nil, nil, &isAdmin, nil)
		_ = b.st.AddAudit(store.AuditRec{
			Kind: "join", ChatID: chatID, UserID: chatIDStr(ev.From.ID), Username: ev.From.Display(),
			Hit: "机器人入群", Actions: `["join"]`, Result: "ok",
		})
	case "left", "kicked":
		_ = b.st.AddAudit(store.AuditRec{
			Kind: "leave", ChatID: chatID, UserID: chatIDStr(ev.From.ID), Username: ev.From.Display(),
			Hit: "机器人离群", Actions: `["leave"]`, Result: status,
		})
	}
}

func (b *Bot) handleChatMember(ev *telegram.ChatMemberUpdated) {
	if b.st == nil || !ev.Chat.IsGroup() {
		return
	}
	chatID := chatIDStr(ev.Chat.ID)
	uid := chatIDStr(ev.NewChatMember.User.ID)
	role := ev.NewChatMember.Status
	if role == "creator" {
		role = "owner"
	}
	if role == "administrator" {
		role = "admin"
	}
	log.Printf("[成员] %s %s 状态: %s → %s", chatID, ev.NewChatMember.User.Display(), ev.OldChatMember.Status, ev.NewChatMember.Status)
	_ = b.st.TouchMember(chatID, uid, ev.NewChatMember.User.Display(), role, ev.NewChatMember.User.IsBot)
	if role == "member" && ev.OldChatMember.Status == "left" {
		_ = b.st.AddAudit(store.AuditRec{
			Kind: "join", ChatID: chatID, UserID: uid, Username: ev.NewChatMember.User.Display(),
			Hit: "成员入群", Actions: `["join"]`, Result: "ok",
		})
	}
}

func (b *Bot) handleJoinRequest(c *telegram.Client, req *telegram.ChatJoinRequest) {
	chatID := chatIDStr(req.Chat.ID)
	userID := chatIDStr(req.From.ID)
	log.Printf("[入群申请] 群=%s 用户=%s(%s) 验证=%q", chatID, req.From.Display(), userID, req.Bio)
	if b.st == nil {
		return
	}
	_ = b.st.EnsureGroup(chatID, req.Chat.Display())
	_ = b.st.TouchMember(chatID, userID, req.From.Display(), "member", false)
	_ = b.st.UpsertJoinRequest(chatID, userID, req.From.Display(), req.Bio)

	auto := strings.EqualFold(b.st.Setting("join_auto_approve", "false"), "true")
	result, detail := "待审批", "等待管理员在网页后台处理"
	if auto {
		if err := c.ApproveChatJoinRequest(chatID, userID); err == nil {
			result, detail = "ok", "已自动通过"
			_ = b.st.SetJoinRequestStatus(chatID, userID, "approved")
		} else {
			result, detail = "fail", "自动通过失败："+errText(err)
		}
	}
	_ = b.st.AddAudit(store.AuditRec{
		Kind: "join", ChatID: chatID, UserID: userID, Username: req.From.Display(),
		Content: req.Bio, Hit: "入群申请", Actions: `["join"]`, Result: result, Detail: detail,
	})
}

// ---------------- 发送 ----------------

func (b *Bot) clientOrNil() *telegram.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
}

// reply 异步回复某条消息（失败只写日志）。
func (b *Bot) reply(c *telegram.Client, m *telegram.Message, text string, replyTo int64) {
	if strings.TrimSpace(text) == "" || m == nil {
		return
	}
	chatID := chatIDStr(m.Chat.ID)
	if replyTo == 0 {
		replyTo = m.MessageID
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[X] 回复 panic: %v", r)
			}
		}()
		out, err := c.SendMessage(chatID, text, telegram.SendOptions{ReplyTo: replyTo})
		if err != nil {
			log.Printf("[X] 回复失败: %v", err)
			return
		}
		if out.MessageID > 0 {
			b.RememberOutgoing(chatID, out.MessageID, text)
		}
	}()
}

// SendNow 同步发消息（网页后台/指令结果需要确切的成功失败）。
func (b *Bot) SendNow(chatID, text string, replyTo int64) (int64, error) {
	c := b.clientOrNil()
	if c == nil {
		return 0, errors.New("机器人尚未连接")
	}
	out, err := c.SendMessage(chatID, text, telegram.SendOptions{ReplyTo: replyTo})
	if err != nil {
		return 0, err
	}
	if out.MessageID > 0 {
		b.RememberOutgoing(chatID, out.MessageID, text)
	}
	return out.MessageID, nil
}

// SendMessageOpt 发文本并支持回复/话题/按钮/解析模式（网页后台用）。
func (b *Bot) SendMessageOpt(chatID, text string, opt telegram.SendOptions) (int64, error) {
	c := b.clientOrNil()
	if c == nil {
		return 0, errors.New("机器人尚未连接")
	}
	out, err := c.SendMessage(chatID, text, opt)
	if err != nil {
		return 0, err
	}
	if out.MessageID > 0 {
		b.RememberOutgoing(chatID, out.MessageID, text)
	}
	return out.MessageID, nil
}

// SendCardNow 发一张 Telegram HTML“卡片”，最近消息里保存为 qbot 卡片文本。
func (b *Bot) SendCardNow(chatID, title, content string) (int64, error) {
	c := b.clientOrNil()
	if c == nil {
		return 0, errors.New("机器人尚未连接")
	}
	out, err := c.SendMessage(chatID, BuildCardHTML(title, content), telegram.SendOptions{
		ParseMode:      "HTML",
		DisablePreview: true,
	})
	if err != nil {
		return 0, err
	}
	if out.MessageID > 0 {
		b.RememberOutgoing(chatID, out.MessageID, CardDisplay(title, content))
	}
	return out.MessageID, nil
}

// SendMediaNow 发媒体消息（网页后台用）。
func (b *Bot) SendMediaNow(chatID, kind, media, caption string, opt telegram.SendOptions) (int64, error) {
	c := b.clientOrNil()
	if c == nil {
		return 0, errors.New("机器人尚未连接")
	}
	out, err := c.SendMedia(chatID, kind, media, caption, opt)
	if err != nil {
		return 0, err
	}
	if out.MessageID > 0 {
		b.RememberOutgoing(chatID, out.MessageID, caption)
	}
	return out.MessageID, nil
}

// SendText 兼容旧调用：异步发文本。
func (b *Bot) SendText(scope, chatID, userID string, msgID int64, text string) {
	c := b.clientOrNil()
	if c == nil || strings.TrimSpace(chatID) == "" {
		return
	}
	go func() {
		out, err := c.SendMessage(chatID, text, telegram.SendOptions{ReplyTo: msgID})
		if err != nil {
			log.Printf("[X] 发送失败: %v", err)
			return
		}
		if out.MessageID > 0 {
			b.RememberOutgoing(chatID, out.MessageID, text)
		}
	}()
}

// ---------------- 状态/配置 ----------------

func (b *Bot) Status() map[string]string {
	b.mu.RLock()
	state, lastErr, lastMsg, me := b.state, b.lastErr, b.lastMsgAt, b.me
	b.mu.RUnlock()

	mode := "long-polling"
	if strings.EqualFold(strings.TrimSpace(b.cfg.Mode), "webhook") || strings.TrimSpace(b.cfg.WebhookURL) != "" {
		mode = "webhook"
	}
	d := map[string]string{
		"bot_state":  state,
		"messages":   strconv.FormatInt(atomic.LoadInt64(&b.msgCount), 10),
		"reconnects": strconv.FormatInt(atomic.LoadInt64(&b.reconnects), 10),
		"uptime":     time.Since(b.startedAt).Round(time.Second).String(),
		"platform":   platformOr(b.cfg.Platform),
		"api_base":   config.APIBase(),
		"mode":       mode,
		"bot_name":   b.cfg.Name,
	}
	if me.ID > 0 {
		d["bot_id"] = strconv.FormatInt(me.ID, 10)
		d["bot_username"] = "@" + me.Username
	}
	if !lastMsg.IsZero() {
		d["last_message_at"] = lastMsg.Format("2006-01-02 15:04:05")
	}
	if lastErr != "" {
		d["last_error"] = lastErr
	}
	return d
}

func (b *Bot) Me() telegram.User {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.me
}

func (b *Bot) setState(s string) {
	b.mu.Lock()
	b.state = s
	b.mu.Unlock()
}

func (b *Bot) setLastErr(s string) {
	b.mu.Lock()
	b.lastErr = s
	b.mu.Unlock()
}

// ReloadAll 热重载：重读 .env、让规则缓存失效，并让轮询循环尽快退出重连。
func (b *Bot) ReloadAll() error {
	b.InvalidateRules()
	b.reloadFlag.Store(true)
	b.setState("重连中")
	log.Printf("[i] 已热重载：规则缓存与 .env；轮询返回后立即重连")
	return nil
}

// ---------------- offset 持久化 ----------------

func (b *Bot) loadOffset() int64 {
	if b.st == nil {
		return 0
	}
	n, _ := strconv.ParseInt(b.st.Setting("tg_offset", "0"), 10, 64)
	return n
}

func (b *Bot) saveOffset(n int64) {
	if b.st == nil || n <= 0 {
		return
	}
	_ = b.st.SetSetting("tg_offset", strconv.FormatInt(n, 10))
}

// ---------------- 角色缓存 ----------------

// memberRole 取群成员角色（admin/owner/member），带 5 分钟缓存。
func (b *Bot) memberRole(c *telegram.Client, chatID, userID string) string {
	key := chatID + ":" + userID
	b.roleMu.Lock()
	if e, ok := b.roleCache[key]; ok && time.Since(e.at) < 5*time.Minute {
		b.roleMu.Unlock()
		return e.role
	}
	b.roleMu.Unlock()

	m, err := c.GetChatMember(chatID, userID)
	if err != nil {
		return "member"
	}
	role := m.Status
	switch role {
	case "creator":
		role = "owner"
	case "administrator":
		role = "admin"
	}
	b.roleMu.Lock()
	b.roleCache[key] = roleEntry{role: role, at: time.Now()}
	b.roleMu.Unlock()
	return role
}

// RefreshGroupAdmin 查询机器人在群里的管理员状态并回写 groups 表。
func (b *Bot) RefreshGroupAdmin(c *telegram.Client, chatID string) (bool, error) {
	if b.st == nil || c == nil || b.me.ID == 0 {
		return false, errors.New("未连接")
	}
	m, err := c.GetChatMember(chatID, strconv.FormatInt(b.me.ID, 10))
	if err != nil {
		return false, err
	}
	ok := m.IsAdmin()
	if err := b.st.UpdateGroup(chatID, nil, nil, nil, nil, nil, &ok, nil); err != nil {
		return ok, err
	}
	return ok, nil
}

func (b *Bot) ensureAllGroupsAdmin() {
	if b.st == nil {
		return
	}
	groups, err := b.st.Groups()
	if err != nil {
		return
	}
	c := b.clientOrNil()
	for _, g := range groups {
		if c == nil {
			return
		}
		if _, err := b.RefreshGroupAdmin(c, g.ChatID); err != nil {
			continue
		}
	}
}

func platformOr(p string) string {
	if p == "" {
		return "telegram"
	}
	return p
}

func (b *Bot) webhookListen() string {
	if v := strings.TrimSpace(b.cfg.WebhookListen); v != "" {
		return v
	}
	return "127.0.0.1:28743"
}

func (b *Bot) webhookPath() string {
	p := strings.TrimSpace(b.cfg.WebhookPath)
	if p == "" {
		return "/telegram/webhook"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// ---------------- helpers ----------------

func chatIDStr(id int64) string { return strconv.FormatInt(id, 10) }

func chatLabel(c telegram.Chat) string {
	switch c.Type {
	case "private":
		return "私聊"
	case "group", "supergroup":
		return "群:" + c.Display()
	case "channel":
		return "频道:" + c.Display()
	}
	return c.Display()
}

func truncateLog(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return strings.TrimSpace(s)
	}
	return string(r[:n]) + "…"
}

func strPtr(s string) *string { return &s }

func errText(err error) string {
	if err == nil {
		return ""
	}
	var ae *telegram.APIError
	if errors.As(err, &ae) {
		if h := ae.Hint(); h != "" {
			return ae.Description + "（" + h + "）"
		}
	}
	return err.Error()
}

func isStart(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "/start" || strings.HasPrefix(s, "/start@")
}

func isHelp(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "/help" || strings.HasPrefix(s, "/help@")
}

// RefreshAllGroups 启动后逐个查询机器人在群里的管理员状态（间隔 pause，避免打满限流）。
func (b *Bot) RefreshAllGroups(pause time.Duration) {
	if b.st == nil {
		return
	}
	groups, err := b.st.Groups()
	if err != nil {
		return
	}
	for _, g := range groups {
		c := b.clientOrNil()
		if c == nil {
			return
		}
		if _, err := b.RefreshGroupAdmin(c, g.ChatID); err != nil {
			continue
		}
		if pause > 0 {
			time.Sleep(pause)
		}
	}
}

// Client 返回当前 Telegram 客户端（未连接时为 nil），供管理端查询群信息用。
func (b *Bot) Client() *telegram.Client { return b.clientOrNil() }

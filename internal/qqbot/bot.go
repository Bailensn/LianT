package qqbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"LianT/internal/media"
	"LianT/internal/platform/qq"
	qqstore "LianT/internal/platform/qq/store"
	"LianT/internal/rules"
	"LianT/internal/store"

	"github.com/gorilla/websocket"
)

// ============================================================================
// 业务代码：QQ 机器人本体
//
// 只被任务进程（go 里的 daemon.Run）加载和使用：
//     daemon.Run -> New(...) -> admin.Start() -> Supervise() -> Session() -> HandleEvent()
// 主进程（main.go）不会碰到这里任何东西，它只往控制端口发指令。
//
// 断线不再需要"重启进程"：Supervise 会带退避自动重连，token 过期会自动续。
// ============================================================================

type Config struct {
	AppID     string
	AppSecret string
	Intents   int
	Name      string
	DBFile    string
	// Store 是 LianT 的 SQLite 业务库，经 qqstore 适配成 qbot 的 OpenID 视图。
	Store *qqstore.Store
}

type Bot struct {
	cfg       Config
	startedAt time.Time

	// st 是 SQLite 存储（规则/违禁词/管理员/审计都在里面）
	st *qqstore.Store
	// rc 是规则与违禁词的内存缓存，按 revision() 失效（网页一改下条消息就生效）
	rc RulesCache

	mu        sync.RWMutex
	token     string
	tokenExp  time.Time
	gateway   string
	conn      *websocket.Conn
	state     string
	lastErr   string
	lastMsgAt time.Time

	writeMu sync.Mutex

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}

	// reconnectCh 用于"手动/服务端要求"的立即重连（不等待退避）
	reconnectCh chan struct{}

	msgCount   int64
	reconnects int64
}

func New(bc Config) *Bot {
	return &Bot{
		cfg:         bc,
		startedAt:   time.Now(),
		st:          bc.Store,
		state:       "启动中",
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
	}
}

// RecentMsg 一条见过的群消息（从本地业务库读出）。
type RecentMsg struct {
	MsgIdx       string
	ID           string
	MemberOpenID string
	Username     string
	Content      string
	Kind         string // text/image/voice/video/audio/file/sticker/animation/...
	IsBot        bool   // 机器人自己发的
	State        string // received / sent / recalled
	At           time.Time
	// Media 是这条消息的主要附件引用（含签名直链，随消息持久化）。
	Media *media.Ref
}

// ClassifyMessage 先看附件，再看 message_type/正文。
// 附件类型比正文 URL 可靠：图片消息的 content 经常是空串。
func ClassifyMessage(d qq.MessageData) string {
	if kind := attachmentKind(d.Attachments); kind != "" {
		return kind
	}
	return ClassifyMessageKind(d.MessageType, d.Content)
}

func attachmentKind(list []qq.Attachment) string {
	for _, a := range list {
		if strings.TrimSpace(a.URL) == "" {
			continue
		}
		return attachmentKindOf(a)
	}
	return ""
}

func attachmentKindOf(a qq.Attachment) string {
	ct := strings.ToLower(strings.TrimSpace(a.ContentType))
	name := strings.ToLower(a.Filename)
	switch {
	case strings.HasPrefix(ct, "image/"), hasExt(name, ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"):
		return media.KindImage
	case strings.HasPrefix(ct, "video/"), hasExt(name, ".mp4", ".mov", ".m4v", ".webm"):
		return media.KindVideo
	case strings.HasPrefix(ct, "audio/"), strings.Contains(ct, "silk"), strings.Contains(ct, "amr"),
		hasExt(name, ".silk", ".amr", ".mp3", ".m4a", ".ogg"):
		if hasExt(name, ".silk", ".amr") || strings.Contains(ct, "silk") || strings.Contains(ct, "amr") {
			return media.KindVoice
		}
		return media.KindAudio
	default:
		return media.KindFile
	}
}

func hasExt(name string, exts ...string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// AttachmentRef 取消息里第一个带 URL 的主要附件。
// 一条消息只投影一个媒体；多图消息先取第一张，够用且不伪造聚合。
func AttachmentRef(d qq.MessageData) *media.Ref {
	for _, a := range d.Attachments {
		if strings.TrimSpace(a.URL) == "" {
			continue
		}
		kind := attachmentKindOf(a)
		mimeType := strings.TrimSpace(a.ContentType)
		if mimeType == "" {
			mimeType = mime.TypeByExtension(strings.ToLower(filepath.Ext(a.Filename)))
		}
		return &media.Ref{
			Kind: kind, Name: strings.TrimSpace(a.Filename), MIME: mimeType,
			Size: a.Size, Width: a.Width, Height: a.Height,
			URL: a.URL,
			// QQ 的附件直链是签名 URL，取回之前无法确认是否还有效。
			State: media.StateUnknown,
		}
	}
	return nil
}

// ClassifyMessageKind 把消息归类（QQ 的 message_type + 内容特征）。
// 撤回本身和类型无关，这个只用于日志/审计显示和"撤回机器人"这类指令判断。
func ClassifyMessageKind(messageType int, content string) string {
	switch messageType {
	case 1:
		return "markdown"
	case 2:
		return "ark"
	case 3:
		return "embed"
	case 4, 7:
		return "media"
	}
	c := strings.TrimSpace(content)
	if c == "" {
		return "other" // 纯图片/语音/文件/表情包等，content 可能是空或只有附件
	}
	low := strings.ToLower(c)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		for ext, kind := range map[string]string{
			".jpg": "image", ".jpeg": "image", ".png": "image", ".gif": "image", ".webp": "image",
			".amr": "voice", ".silk": "voice", ".mp3": "voice", ".m4a": "voice",
			".mp4": "video", ".mov": "video",
			".pdf": "file", ".doc": "file", ".docx": "file", ".zip": "file", ".txt": "file",
		} {
			if strings.Contains(low, ext) {
				return kind
			}
		}
		return "media"
	}
	if strings.HasPrefix(c, "<") {
		return "markup" // <emoji:..> 这类表情
	}
	return "text"
}

// KindLabel 给日志/审计看的短标签。
func KindLabel(kind string) string {
	switch kind {
	case media.KindImage:
		return "[图片]"
	case media.KindVoice:
		return "[语音]"
	case media.KindAudio:
		return "[音频]"
	case media.KindVideo:
		return "[视频]"
	case media.KindFile:
		return "[文件]"
	case media.KindSticker:
		return "[表情]"
	case media.KindAnimation:
		return "[动图]"
	case media.KindVideoNote:
		return "[视频留言]"
	case "media":
		return "[富媒体]"
	case "markdown":
		return "[markdown]"
	case "ark":
		return "[ark卡片]"
	case "embed":
		return "[embed卡片]"
	case "markup":
		return "[表情]"
	case "bot":
		return "[机器人]"
	}
	return ""
}

// RememberMessage 把群消息写入本地业务库（每条群消息都调用）。
// 没有 message_id 的事件（极少数系统事件）不落库：撤回/媒体都靠它定位。
func (b *Bot) RememberMessage(d qq.MessageData) {
	if b.st == nil || b.st.L == nil || strings.TrimSpace(d.ID) == "" {
		return
	}
	title := "QQ 群"
	if g, ok := b.st.Group(d.GroupOpenID); ok && strings.TrimSpace(g.Name) != "" {
		title = g.Name
	}
	rec := store.MessageRec{
		ChatID: d.GroupOpenID, MessageID: d.ID, MsgIdx: d.MessageScene.Get("msg_idx"),
		ConvKey: store.ConversationKey(d.GroupOpenID, ""), ChatKind: "group", ChatTitle: title,
		UserID: d.Author.MemberOpenID, Username: d.Author.Username,
		Content: d.Content, Kind: ClassifyMessage(d), State: "received", At: time.Now(),
	}
	if ref := AttachmentRef(d); ref != nil {
		if raw, err := json.Marshal(ref); err == nil {
			rec.MediaJSON = string(raw)
		}
	}
	if err := b.st.L.SaveMessage(rec); err != nil {
		log.Printf("[!] 保存群消息失败 group=%s msg=%s: %v", d.GroupOpenID, d.ID, err)
	}
}

// RememberDirectMessage 把 QQ 单聊消息写入本地库（与群聊分开上下文）。
func (b *Bot) RememberDirectMessage(d qq.MessageData) {
	if b.st == nil || b.st.L == nil || strings.TrimSpace(d.ID) == "" {
		return
	}
	chatID := strings.TrimSpace(d.Author.UserOpenID)
	if chatID == "" {
		chatID = strings.TrimSpace(d.Author.MemberOpenID)
	}
	if chatID == "" {
		return
	}
	title := strings.TrimSpace(d.Author.Username)
	if title == "" {
		title = "私聊"
		if len(chatID) > 6 {
			title += " · " + chatID[len(chatID)-6:]
		}
	}
	rec := store.MessageRec{
		ChatID: chatID, MessageID: d.ID,
		ConvKey: store.ConversationKey(chatID, ""), ChatKind: "private", ChatTitle: title,
		UserID: chatID, Username: d.Author.Username,
		Content: d.Content, Kind: ClassifyMessage(d), State: "received", At: time.Now(),
	}
	if ref := AttachmentRef(d); ref != nil {
		if raw, err := json.Marshal(ref); err == nil {
			rec.MediaJSON = string(raw)
		}
	}
	if err := b.st.L.SaveMessage(rec); err != nil {
		log.Printf("[!] 保存单聊消息失败 user=%s msg=%s: %v", chatID, d.ID, err)
	}
}

// RememberOutgoing 记下机器人自己发出去的消息（这样"撤回你/撤回机器人"能撤自己的回复）。
func (b *Bot) RememberOutgoing(groupOpenID, messageID, content string) {
	if b.st == nil || b.st.L == nil || groupOpenID == "" || messageID == "" {
		return
	}
	kind, title := "", ""
	if conv, ok := b.st.L.Conversation(groupOpenID); ok {
		kind, title = conv.Kind, conv.Title
	}
	b.RememberOutgoingIn(groupOpenID, kind, title, messageID, content)
}

// RememberOutgoingIn 记下机器人发到指定会话（群/私聊）的消息。
func (b *Bot) RememberOutgoingIn(chatID, kind, title, messageID, content string) {
	if b.st == nil || b.st.L == nil || chatID == "" || messageID == "" {
		return
	}
	if kind == "" {
		kind = "group"
	}
	rec := store.MessageRec{
		ChatID: chatID, MessageID: messageID,
		ConvKey: store.ConversationKey(chatID, ""), ChatKind: kind, ChatTitle: title,
		Username: "机器人", Content: content, Kind: "bot", IsBot: true, State: "sent", At: time.Now(),
	}
	if err := b.st.L.SaveMessage(rec); err != nil {
		log.Printf("[!] 保存机器人消息失败 chat=%s msg=%s: %v", chatID, messageID, err)
	}
}

// RecentList 取最近 n 条消息（时间顺序），n<=0 表示全部；onlyWithin 内才算（0 表示不限制）。
func (b *Bot) RecentList(groupOpenID string, n int, onlyWithin time.Duration) []RecentMsg {
	if b.st == nil || b.st.L == nil {
		return nil
	}
	var since time.Time
	if onlyWithin > 0 {
		since = time.Now().Add(-onlyWithin)
	}
	recs, err := b.st.L.Messages(groupOpenID, n, since)
	if err != nil {
		log.Printf("[!] 读取群消息失败 group=%s: %v", groupOpenID, err)
		return nil
	}
	out := make([]RecentMsg, 0, len(recs))
	for _, rec := range recs {
		out = append(out, recentFromRec(rec))
	}
	return out
}

// RecentListByConversation 按本地会话键取消息（群/私聊各自独立上下文）。
func (b *Bot) RecentListByConversation(convKey string, n int, onlyWithin time.Duration) []RecentMsg {
	if b.st == nil || b.st.L == nil {
		return nil
	}
	var since time.Time
	if onlyWithin > 0 {
		since = time.Now().Add(-onlyWithin)
	}
	recs, err := b.st.L.MessagesByConversation(convKey, n, since)
	if err != nil {
		log.Printf("[!] 读取会话消息失败 conv=%s: %v", convKey, err)
		return nil
	}
	out := make([]RecentMsg, 0, len(recs))
	for _, rec := range recs {
		out = append(out, recentFromRec(rec))
	}
	return out
}

// MarkRecalled 把已撤回的消息标记成墓碑：保留时间与发送者，清掉正文与媒体引用。
// 撤回是消息的一个状态，不是「从没发生过」。
func (b *Bot) MarkRecalled(groupOpenID string, ids ...string) {
	if b.st == nil || b.st.L == nil {
		return
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		if err := b.st.L.RecallMessage(groupOpenID, id); err != nil {
			log.Printf("[!] 标记撤回失败 group=%s msg=%s: %v", groupOpenID, id, err)
		}
	}
}

// recentFromRec 把存储记录还原成 runtime 视图。
func recentFromRec(rec store.MessageRec) RecentMsg {
	m := RecentMsg{
		MsgIdx: rec.MsgIdx, ID: rec.MessageID, MemberOpenID: rec.UserID,
		Username: rec.Username, Content: rec.Content, Kind: rec.Kind, IsBot: rec.IsBot,
		State: rec.State, At: rec.At,
	}
	if strings.TrimSpace(rec.MediaJSON) != "" {
		var ref media.Ref
		if err := json.Unmarshal([]byte(rec.MediaJSON), &ref); err == nil && ref.Kind != "" {
			m.Media = &ref
		}
	}
	return m
}

// LookupBotRecent 机器人自己最近发的一条消息。
func (b *Bot) LookupBotRecent(groupOpenID string, onlyWithin time.Duration) (RecentMsg, bool) {
	if b.st == nil || b.st.L == nil {
		return RecentMsg{}, false
	}
	var since time.Time
	if onlyWithin > 0 {
		since = time.Now().Add(-onlyWithin)
	}
	rec, ok := b.st.L.LastBotMessage(groupOpenID, since)
	if !ok {
		return RecentMsg{}, false
	}
	return recentFromRec(rec), true
}

// LookupRecent 按 msg_idx 或消息 ID 找历史消息。
func (b *Bot) LookupRecent(groupOpenID, msgIdx, messageID string) (RecentMsg, bool) {
	if b.st == nil || b.st.L == nil {
		return RecentMsg{}, false
	}
	if msgIdx != "" {
		if rec, ok := b.st.L.MessageByMsgIdx(groupOpenID, msgIdx); ok {
			return recentFromRec(rec), true
		}
	}
	if messageID != "" {
		if rec, ok := b.st.L.Message(groupOpenID, messageID); ok {
			return recentFromRec(rec), true
		}
	}
	return RecentMsg{}, false
}

// LookupRecentByMember 找某人最近一条消息（仍限制在 2 分钟撤回窗口内）。
func (b *Bot) LookupRecentByMember(groupOpenID, memberOpenID string) (RecentMsg, bool) {
	if b.st == nil || b.st.L == nil {
		return RecentMsg{}, false
	}
	rec, ok := b.st.L.LastMessageByMember(groupOpenID, memberOpenID, time.Now().Add(-2*time.Minute))
	if !ok {
		return RecentMsg{}, false
	}
	return recentFromRec(rec), true
}

// LookupMedia 按「群 + 消息 ID」查附件引用。
// 只能查本地消息库里存在的消息，管理端没法拿任意 URL 让服务端代理。
func (b *Bot) LookupMedia(groupOpenID, msgID string) (media.Ref, bool) {
	if b.st == nil || b.st.L == nil {
		return media.Ref{}, false
	}
	rec, ok := b.st.L.Message(groupOpenID, strings.TrimSpace(msgID))
	if !ok || rec.State == "recalled" || strings.TrimSpace(rec.MediaJSON) == "" {
		return media.Ref{}, false
	}
	var ref media.Ref
	if err := json.Unmarshal([]byte(rec.MediaJSON), &ref); err != nil || ref.Kind == "" {
		return media.Ref{}, false
	}
	return ref, true
}

// MarkMediaState 记下取回附件时确认到的状态，下次投影会带上。
func (b *Bot) MarkMediaState(groupOpenID, msgID string, st media.State) {
	if b.st == nil || b.st.L == nil {
		return
	}
	rec, ok := b.st.L.Message(groupOpenID, strings.TrimSpace(msgID))
	if !ok || strings.TrimSpace(rec.MediaJSON) == "" {
		return
	}
	var ref media.Ref
	if err := json.Unmarshal([]byte(rec.MediaJSON), &ref); err != nil {
		return
	}
	next := media.NormalizeState(st)
	if ref.State == next {
		return
	}
	ref.State = next
	raw, err := json.Marshal(ref)
	if err != nil {
		return
	}
	if err := b.st.L.SetMessageMedia(groupOpenID, rec.MessageID, string(raw)); err != nil {
		log.Printf("[!] 更新附件状态失败 group=%s msg=%s: %v", groupOpenID, msgID, err)
	}
}

// MessagesSignatureByConversation 是具体会话的 SSE 签名。
func (b *Bot) MessagesSignatureByConversation(convKey string) string {
	if b.st == nil || b.st.L == nil {
		return ""
	}
	return b.st.L.ConversationSignature(convKey)
}

// MessagesSignature 是 SSE 用的会话快照签名，不必把整段历史读出来。
func (b *Bot) MessagesSignature(groupOpenID string) string {
	if b.st == nil || b.st.L == nil {
		return ""
	}
	return b.st.L.MessageSignature(groupOpenID)
}

// OpenMedia 代理 QQ 附件下载。
// 附件直链由平台签名，可能随时失效；失效时返回 ErrGone，让界面明说而不是留白。
func (b *Bot) OpenMedia(ref media.Ref) (io.ReadCloser, int64, error) {
	if strings.TrimSpace(ref.URL) == "" {
		return nil, 0, media.ErrGone
	}
	if ref.Size > media.MaxBytes {
		return nil, 0, media.ErrTooLarge
	}
	return media.OpenHTTP(ref.URL, media.MaxBytes)
}

func credentialsError() string {
	return "QQ 凭据为空：请检查 Bot 的 AppID:AppSecret"
}

// Creds 当前凭据（每次调用都会顺手看看 .env 有没有被改过）。
func (b *Bot) Creds() (string, string) {
	return b.cfg.AppID, b.cfg.AppSecret
}

// ReloadAll 热重载：重读 .env 凭据、让规则缓存失效、强制换 token 并重建网关连接。
// （规则本身改数据库即时生效，这里主要是给"改了 .env 想立刻用新凭据"用）
func (b *Bot) ReloadAll() error {
	b.mu.Lock()
	b.token = ""
	b.tokenExp = time.Time{}
	b.mu.Unlock()

	b.InvalidateRules()
	log.Printf("[i] 已热重载：.env 凭据 + 规则缓存，并重建网关连接")
	b.Reconnect()
	return nil
}

// InvalidateRules 让规则/违禁词缓存下次访问时重载。
func (b *Bot) InvalidateRules() {
	b.rc.mu.Lock()
	b.rc.rev = -1
	b.rc.mu.Unlock()
}

// ReplyInfo 状态快照（规则/违禁词/数据库）。
func (b *Bot) ReplyInfo() map[string]string {
	group, _, words := b.Rules()
	d := map[string]string{
		"db_file":     b.cfg.DBFile,
		"reply_rules": qqstore.Itoa(len(group.Rules)),
		"reply_all":   qqstore.BoolStr(group.IsReplyAll()),
		"reply_ver":   qqstore.Itoa(int(b.st.Revision())),
		"words":       qqstore.Itoa(words.Count()),
	}
	if b.st != nil {
		d["reply_loaded"] = qqstore.NowStr()
		d["reply_file"] = b.cfg.DBFile
	}
	return d
}

// TestReply 干跑一条消息，看规则会怎么回（CLI: qbot reply "xx"）。
func (b *Bot) TestReply(content, scope string) rules.Result {
	if scope == "" {
		scope = "群@(测试)"
	}
	group, c2c, _ := b.Rules()
	if scope == "私聊" {
		return c2c.Evaluate(content, scope, "", "测试用户ID")
	}
	return group.Evaluate(content, scope, "测试群ID", "测试用户ID")
}

// Start 起一个后台协程跑业务，自身立即返回。
func (b *Bot) Start() {
	go b.Supervise()
}

// Stop 优雅停止：关闭连接并等业务协程退出（最多 timeout）。
func (b *Bot) Stop(timeout time.Duration) {
	b.stopOnce.Do(func() { close(b.stopCh) })
	b.CloseConn()
	select {
	case <-b.doneCh:
	case <-time.After(timeout):
		log.Printf("[!] 等待机器人停止超时(%s)，继续退出", timeout)
	}
}

// Reconnect 软重连：只断开当前网关连接，业务协程原地重连，任务进程不退出。
func (b *Bot) Reconnect() {
	select {
	case b.reconnectCh <- struct{}{}:
	default:
	}
	b.CloseConn()
}

// Status 给控制端口用的状态快照。
func (b *Bot) Status() map[string]string {
	b.mu.RLock()
	state, lastErr, lastMsg, gw := b.state, b.lastErr, b.lastMsgAt, b.gateway
	tokenExp, hasToken := b.tokenExp, b.token != ""
	b.mu.RUnlock()

	d := map[string]string{
		"bot_state":  state,
		"messages":   strconv.FormatInt(atomic.LoadInt64(&b.msgCount), 10),
		"reconnects": strconv.FormatInt(atomic.LoadInt64(&b.reconnects), 10),
		"uptime":     time.Since(b.startedAt).Round(time.Second).String(),
		"platform":   "qq",
		"bot_name":   b.cfg.Name,
		"mode":       "websocket",
		"db_file":    b.cfg.DBFile,
	}
	if hasToken && !tokenExp.IsZero() {
		d["token_expires_at"] = tokenExp.Format("2006-01-02 15:04:05")
	}
	if !lastMsg.IsZero() {
		d["last_message_at"] = lastMsg.Format("2006-01-02 15:04:05")
	}
	if gw != "" {
		d["gateway"] = gw
	}
	if lastErr != "" {
		d["last_error"] = lastErr
	}
	return d
}

func (b *Bot) Stopped() bool {
	select {
	case <-b.stopCh:
		return true
	default:
		return false
	}
}

// Supervise 业务主循环：断线自动重连，直到 Stop。
func (b *Bot) Supervise() {
	defer close(b.doneCh)
	log.Printf("[Bot] 机器人业务已在任务进程内启动 (intents=%d)", b.cfg.Intents)

	backoff := time.Second
	for {
		if b.Stopped() {
			b.SetState("已停止")
			log.Printf("[Bot] 业务已停止")
			return
		}
		b.SetState("连接中")
		started := time.Now()
		err := b.Session()
		if b.Stopped() {
			b.SetState("已停止")
			log.Printf("[Bot] 业务已停止")
			return
		}
		if err != nil {
			b.SetLastErr(err.Error())
			log.Printf("[!] %v", err)
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second // 稳定跑过一段，下次快速重连
		}
		log.Printf("[i] %s 后重连(第 %d 次)", backoff, atomic.LoadInt64(&b.reconnects)+1)
		select {
		case <-b.stopCh:
			b.SetState("已停止")
			log.Printf("[Bot] 业务已停止")
			return
		case <-b.reconnectCh:
			backoff = time.Second
			log.Printf("[i] 收到手动重连指令，立即重连")
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

// Session 一次完整的网关会话，返回即表示连接结束（需要重连）。
func (b *Bot) Session() error {
	token, err := b.EnsureToken(false)
	if err != nil {
		return err
	}

	gw, err := qq.FetchGateway(token)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.gateway = gw
	b.mu.Unlock()

	header := http.Header{}
	header.Set("Authorization", "QQBot "+token)
	header.Set("X-Union-Appid", b.cfg.AppID)

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 15 * time.Second
	conn, _, err := dialer.Dial(gw, header)
	if err != nil {
		return fmt.Errorf("连接网关失败: %w", err)
	}
	b.SetConn(conn)
	log.Printf("[OK] WebSocket 已连接: %s", gw)
	defer func() {
		b.ClearConn(conn)
		_ = conn.Close()
	}()

	sessionStop := make(chan struct{})
	defer close(sessionStop)

	writeJSON := func(v interface{}) error {
		b.writeMu.Lock()
		defer b.writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	var lastSeq int64
	helloCh := make(chan int, 1)
	go func() {
		for {
			select {
			case <-sessionStop:
				return
			case interval := <-helloCh:
				t := time.NewTicker(time.Duration(interval) * time.Millisecond)
				last := time.Now()
				beat := func() bool {
					seq := atomic.LoadInt64(&lastSeq)
					if err := writeJSON(qq.Payload{Op: 1, D: qq.MustJSON(seq)}); err != nil {
						log.Printf("[X] 心跳发送失败，断开重连: %v", err)
						_ = conn.Close() // 让读循环立刻退出
						return false
					}
					now := time.Now()
					gap := now.Sub(last)
					last = now
					if HeartbeatLate(gap, interval) {
						// 定时器都停了：典型的 Android 后台冻结/Doze（进程被挂起）。
						// 这时上一拍早就错过，服务端大概率已经踢了我们，直接重连最干净。
						log.Printf("[!] 心跳断档 %s（应每 %dms 一次）：进程刚被系统挂起过 —— Android 后台冻结/Doze 特征，立即重连", gap.Round(time.Second), interval)
						_ = conn.Close()
						return false
					}
					log.Printf("[hb] 心跳已发送 seq=%d", seq)
					return true
				}
				log.Printf("[hb] 心跳协程启动：间隔 %dms（先立刻发一拍，避免解冻后空窗）", interval)
				if !beat() {
					t.Stop()
					return
				}
				for {
					select {
					case <-sessionStop:
						t.Stop()
						return
					case <-t.C:
						if !beat() {
							t.Stop()
							return
						}
					}
				}
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if b.Stopped() {
				return nil
			}
			return fmt.Errorf("连接断开: %w", err)
		}
		var p qq.Payload
		if err := json.Unmarshal(raw, &p); err != nil {
			log.Printf("[!] 解析失败: %v", err)
			continue
		}
		if p.S > 0 {
			atomic.StoreInt64(&lastSeq, int64(p.S))
		}
		switch p.Op {
		case 10: // Hello -> 鉴权 + 起心跳
			var h qq.HelloData
			_ = json.Unmarshal(p.D, &h)
			interval := h.HeartbeatInterval
			if interval <= 0 {
				interval = 45000
			}
			log.Printf("[OK] Hello，心跳间隔 %dms", interval)
			if err := writeJSON(qq.Payload{
				Op: 2,
				D: qq.MustJSON(qq.IdentifyData{
					Token:   "QQBot " + token,
					Intents: b.cfg.Intents,
					Shard:   []int{0, 1},
				}),
			}); err != nil {
				return fmt.Errorf("鉴权发送失败: %w", err)
			}
			select {
			case helloCh <- interval:
			default:
			}
		case 0: // 事件
			b.HandleEvent(p)
		case 11:
			log.Printf("[hb] 心跳已确认(ACK)")
		case 7:
			return errors.New("服务端要求重连(Op7)")
		case 9:
			return errors.New("会话失效(Op9)")
		}
	}
}

func (b *Bot) HandleEvent(p qq.Payload) {
	switch p.T {
	case "READY":
		b.SetState("在线")
		b.SetLastErr("")
		log.Printf("[OK] 机器人已上线！")
		log.Printf(">>> 回复规则/违禁词都在 SQLite: %s（用网页后台改，改完下条消息即生效）", b.cfg.DBFile)
	case "RESUMED":
		log.Printf("[OK] 会话已恢复")
	case "GROUP_AT_MESSAGE_CREATE":
		var d qq.MessageData
		_ = json.Unmarshal(p.D, &d)
		b.CountMessage()
		log.Printf("[群@] 群ID=%s | %s: %s", d.GroupOpenID, d.Author.Username, d.Content)
		b.HandleGroupMessage(d, "群@")
	case "GROUP_MESSAGE_CREATE":
		var d qq.MessageData
		_ = json.Unmarshal(p.D, &d)
		b.CountMessage()
		log.Printf("[群-全量] 群ID=%s | %s: %s", d.GroupOpenID, d.Author.Username, d.Content)
		b.HandleGroupMessage(d, "群消息")
	case "C2C_MESSAGE_CREATE":
		var d qq.MessageData
		_ = json.Unmarshal(p.D, &d)
		b.CountMessage()
		log.Printf("[私聊] 用户ID=%s | %s", d.Author.UserOpenID, d.Content)
		b.HandleC2CMessage(d)
	case "GROUP_ADD_ROBOT":
		var d struct {
			GroupOpenID    string `json:"group_openid"`
			OpMemberOpenid string `json:"op_member_openid"`
		}
		_ = json.Unmarshal(p.D, &d)
		log.Printf("[OK] 机器人被拉进群: %s", d.GroupOpenID)
		if b.st != nil && d.GroupOpenID != "" {
			_ = b.st.EnsureGroup(d.GroupOpenID, "")
			_ = b.st.AddAudit(qqstore.AuditRec{Kind: "join", GroupOpenID: d.GroupOpenID,
				MemberOpenID: d.OpMemberOpenid, Hit: "机器人入群", Actions: `["join"]`, Result: "ok"})
		}
	case "GROUP_DEL_ROBOT":
		var d struct {
			GroupOpenID string `json:"group_openid"`
		}
		_ = json.Unmarshal(p.D, &d)
		log.Printf("[!] 机器人被移出群: %s", d.GroupOpenID)
		if b.st != nil && d.GroupOpenID != "" {
			_ = b.st.AddAudit(qqstore.AuditRec{Kind: "leave", GroupOpenID: d.GroupOpenID,
				Hit: "机器人退群", Actions: `["leave"]`, Result: "ok"})
		}
	case "GROUP_JOIN_REQUEST":
		b.HandleGroupJoinRequest(p.D)
	case "GROUP_MEMBER_ADD", "GROUP_MEMBER_REMOVE":
		var d struct {
			GroupOpenID  string `json:"group_openid"`
			MemberOpenid string `json:"member_openid"`
			Username     string `json:"username"`
		}
		_ = json.Unmarshal(p.D, &d)
		log.Printf("[事件] %s 群=%s 成员=%s(%s)", p.T, d.GroupOpenID, d.Username, d.MemberOpenid)
		if b.st != nil && d.GroupOpenID != "" && d.MemberOpenid != "" {
			role := "member"
			_ = b.st.TouchMember(d.GroupOpenID, d.MemberOpenid, d.Username, role, false)
		}
	case "AT_MESSAGE_CREATE":
		log.Printf("[频道@] %s", string(p.D))
	default:
		if p.T != "" {
			log.Printf("[事件] %s", p.T)
		}
	}
}

func RecoverLog() {
	if r := recover(); r != nil {
		log.Printf("[X] 回复时发生 panic: %v", r)
	}
}

// HeartbeatLate 判断这一拍是否明显迟到（进程刚被系统挂起过）。
// 依据：心跳由本进程的定时器驱动，进程被冻结时定时器不触发，
// 解冻后这一拍的间隔会明显大于 heartbeat_interval。
// 容差 = max(间隔/4, 5s)，正常抖动不会误报。
// 例：间隔 41250ms 时阈值 51.6s —— 用户日志里那种"冻 65 秒"必然命中。
func HeartbeatLate(gap time.Duration, intervalMs int) bool {
	if intervalMs <= 0 {
		intervalMs = 45000
	}
	interval := time.Duration(intervalMs) * time.Millisecond
	tol := interval / 4
	if tol < 5*time.Second {
		tol = 5 * time.Second
	}
	return gap > interval+tol
}

// EnsureToken 保证有一个可用 token：过期前 5 分钟自动续。
// 每次真正取 token 时才去读凭据，所以 .env 改完能自动生效。
func (b *Bot) EnsureToken(force bool) (string, error) {
	if !force {
		b.mu.RLock()
		tok, exp := b.token, b.tokenExp
		b.mu.RUnlock()
		if tok != "" && time.Now().Before(exp.Add(-5*time.Minute)) {
			return tok, nil
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if !force && b.token != "" && time.Now().Before(b.tokenExp.Add(-5*time.Minute)) {
		return b.token, nil
	}
	id, sec := b.Creds()
	if strings.TrimSpace(id) == "" || strings.TrimSpace(sec) == "" {
		msg := credentialsError()
		if strings.TrimSpace(msg) == "" {
			msg = "机器人凭据为空：请检查 .env / 环境变量里的 QBOT_APP_ID 与 QBOT_APP_SECRET"
		}
		b.lastErr = msg
		return "", errors.New(msg)
	}
	nt, ttl, err := qq.FetchAccessToken(id, sec)
	if err != nil {
		b.lastErr = err.Error()
		return "", err
	}
	if ttl <= 0 {
		ttl = 100 * time.Minute
	}
	b.token = nt
	b.tokenExp = time.Now().Add(ttl)
	log.Printf("[OK] access_token 获取成功(有效期 %s)", ttl)
	return nt, nil
}

func (b *Bot) SetState(s string) {
	b.mu.Lock()
	b.state = s
	b.mu.Unlock()
}

func (b *Bot) SetLastErr(s string) {
	b.mu.Lock()
	b.lastErr = s
	b.mu.Unlock()
}

func (b *Bot) CountMessage() {
	atomic.AddInt64(&b.msgCount, 1)
	b.mu.Lock()
	b.lastMsgAt = time.Now()
	b.mu.Unlock()
}

func (b *Bot) SetConn(c *websocket.Conn) {
	b.mu.Lock()
	b.conn = c
	b.state = "已连接"
	b.mu.Unlock()
}

func (b *Bot) ClearConn(c *websocket.Conn) {
	b.mu.Lock()
	if b.conn == c {
		b.conn = nil
	}
	b.mu.Unlock()
	if b.Stopped() {
		b.SetState("已停止")
		return
	}
	b.SetState("重连中")
}

func (b *Bot) CloseConn() {
	b.mu.Lock()
	c := b.conn
	b.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}

// ConversationsSignature 让 SSE 能感知其他会话的变化。
func (b *Bot) ConversationsSignature() string {
	if b.st == nil || b.st.L == nil {
		return ""
	}
	return b.st.L.ConversationsSignature()
}

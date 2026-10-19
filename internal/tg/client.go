package tg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// ============================================================================
// Telegram Bot API 客户端
//
// 只依赖标准库：
//   * Call 用 JSON body 调 /bot<token>/<method>
//   * getUpdates 长轮询（不需要公网回调，手机/家里机器都能跑）
//   * 429 自动按 retry_after 等待重试，5xx 退避重试一次
//   * 错误保留 Telegram 原始 description，管理端和群内回复都能看到原因
// ============================================================================

const DefaultAPIBase = "https://api.telegram.org"

type Client struct {
	token string
	base  string
	hc    *http.Client

	mu sync.RWMutex
	me User
}

// New 创建客户端。baseURL 为空时用官方地址；proxyURL 支持 http/socks5。
func New(token, baseURL, proxyURL string) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("Telegram Bot Token 为空")
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultAPIBase
	}
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          20,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 40 * time.Second,
	}
	if p := strings.TrimSpace(proxyURL); p != "" {
		pu, err := url.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("代理地址不合法: %w", err)
		}
		tr.Proxy = http.ProxyURL(pu)
	}
	return &Client{
		token: token,
		base:  base,
		hc:    &http.Client{Timeout: 60 * time.Second, Transport: tr},
	}, nil
}

// Base 返回 API 根地址。
func (c *Client) Base() string { return c.base }

// Me 返回 getMe 缓存（启动后由 GetMe 填充）。
func (c *Client) Me() User {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.me
}

// Username 返回机器人用户名（不带 @）。
func (c *Client) Username() string { return c.Me().Username }

// Call 调一个 Bot API 方法；out 非空时解析 result。
func (c *Client) Call(method string, params map[string]interface{}, out interface{}) error {
	if params == nil {
		params = map[string]interface{}{}
	}
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("编码请求失败: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("POST", c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("构造请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("请求 Telegram 失败: %w", err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return lastErr
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()

		var r Response
		if err := json.Unmarshal(raw, &r); err != nil {
			return fmt.Errorf("Telegram 响应无法解析(HTTP %d): %s", resp.StatusCode, truncate(string(raw), 300))
		}
		if r.OK {
			if out != nil && len(r.Result) > 0 {
				if err := json.Unmarshal(r.Result, out); err != nil {
					return fmt.Errorf("解析 %s 结果失败: %w", method, err)
				}
			}
			return nil
		}

		apiErr := &APIError{
			Method: method, Code: r.ErrorCode, Description: r.Description,
			RetryAfter: r.Parameters.RetryAfter, MigrateTo: r.Parameters.MigrateToChatID,
		}
		if apiErr.Code == 0 {
			apiErr.Code = resp.StatusCode
		}
		if apiErr.Description == "" {
			apiErr.Description = truncate(string(raw), 300)
		}
		lastErr = apiErr

		if apiErr.Code == 429 {
			wait := apiErr.RetryAfter
			if wait <= 0 {
				wait = 3
			}
			if wait > 60 {
				wait = 60
			}
			log.Printf("[tg] %s 触发限流，等待 %ds 后重试", method, wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if apiErr.Code >= 500 && attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		return apiErr
	}
	return lastErr
}

// GetMe 校验 token 并缓存机器人信息。
func (c *Client) GetMe() (User, error) {
	var u User
	if err := c.Call("getMe", nil, &u); err != nil {
		return User{}, err
	}
	c.mu.Lock()
	c.me = u
	c.mu.Unlock()
	return u, nil
}

// DeleteWebhook 关掉 webhook（否则 getUpdates 会 409）。失败不致命。
func (c *Client) DeleteWebhook(dropPending bool) error {
	return c.Call("deleteWebhook", map[string]interface{}{"drop_pending_updates": dropPending}, nil)
}

// GetUpdates 长轮询拿更新。timeout 单位秒（0-50）。
func (c *Client) GetUpdates(offset int64, timeout int) ([]Update, error) {
	p := map[string]interface{}{
		"timeout": timeout,
		"limit":   100,
		"allowed_updates": []string{
			"message", "edited_message", "channel_post",
			"my_chat_member", "chat_member", "chat_join_request", "callback_query",
		},
	}
	if offset > 0 {
		p["offset"] = offset
	}
	var out []Update
	if err := c.Call("getUpdates", p, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SendOptions 是发消息的可选参数。
type SendOptions struct {
	ReplyTo             int64
	ThreadID            int64
	DisablePreview      bool
	ParseMode           string // 空 = 纯文本
	DisableNotification bool
}

// SendMessage 发一条消息，返回落库后的消息（含 message_id）。
func (c *Client) SendMessage(chatID, text string, opt SendOptions) (Message, error) {
	text = strings.TrimSpace(text)
	if chatID == "" {
		return Message{}, errors.New("缺少 chat_id")
	}
	if text == "" {
		return Message{}, errors.New("消息内容为空")
	}
	// Telegram 单条上限 4096 个 UTF-16 code unit；超长切分（调用方只拿最后一条 ID）。
	for utf16Len(text) > 4000 {
		part, rest := splitUTF16(text, 3900)
		if part == "" {
			break
		}
		if _, err := c.sendOne(chatID, part, opt); err != nil {
			return Message{}, err
		}
		text = rest
		if strings.TrimSpace(text) == "" {
			return Message{}, nil
		}
	}
	return c.sendOne(chatID, text, opt)
}

func (c *Client) sendOne(chatID, text string, opt SendOptions) (Message, error) {
	p := map[string]interface{}{
		"chat_id":              chatID,
		"text":                 text,
		"link_preview_options": map[string]interface{}{"is_disabled": opt.DisablePreview},
		"disable_notification": opt.DisableNotification,
	}
	if opt.ParseMode != "" {
		p["parse_mode"] = opt.ParseMode
	}
	if opt.ReplyTo > 0 {
		p["reply_parameters"] = map[string]interface{}{
			"message_id":                  opt.ReplyTo,
			"allow_sending_without_reply": true,
		}
	}
	if opt.ThreadID > 0 {
		p["message_thread_id"] = opt.ThreadID
	}
	var m Message
	if err := c.Call("sendMessage", p, &m); err != nil {
		return Message{}, err
	}
	return m, nil
}

// DeleteMessage 撤回/删除消息。
func (c *Client) DeleteMessage(chatID string, messageID int64) error {
	if messageID <= 0 {
		return errors.New("缺少 message_id")
	}
	return c.Call("deleteMessage", map[string]interface{}{
		"chat_id": chatID, "message_id": messageID,
	}, nil)
}

// RestrictChatMember 禁言/解除禁言。until 为 0 表示永久（直到手动解除）。
func (c *Client) RestrictChatMember(chatID, userID string, until time.Time, allow bool) error {
	uid, err := parseID(userID)
	if err != nil {
		return fmt.Errorf("用户 ID 不合法: %w", err)
	}
	// 只调整"发言"相关权限；改群信息/邀请/置顶这些管理员权限一律 false，
	// 免得"解禁"顺手把普通成员提权。
	perms := map[string]interface{}{
		"can_send_messages":         allow,
		"can_send_audios":           allow,
		"can_send_documents":        allow,
		"can_send_photos":           allow,
		"can_send_videos":           allow,
		"can_send_video_notes":      allow,
		"can_send_voice_notes":      allow,
		"can_send_polls":            allow,
		"can_send_other_messages":   allow,
		"can_add_web_page_previews": allow,
		"can_change_info":           false,
		"can_invite_users":          false,
		"can_pin_messages":          false,
		"can_manage_topics":         false,
	}
	p := map[string]interface{}{
		"chat_id": chatID, "user_id": uid, "permissions": perms,
	}
	if !until.IsZero() {
		p["until_date"] = until.Unix()
	}
	return c.Call("restrictChatMember", p, nil)
}

// BanChatMember 封禁（踢出并禁止再进）。
func (c *Client) BanChatMember(chatID, userID string, revokeMessages bool) error {
	uid, err := parseID(userID)
	if err != nil {
		return fmt.Errorf("用户 ID 不合法: %w", err)
	}
	return c.Call("banChatMember", map[string]interface{}{
		"chat_id": chatID, "user_id": uid, "revoke_messages": revokeMessages,
	}, nil)
}

// UnbanChatMember 解除封禁。onlyIfBanned=true 时未封禁会报错，调用方按需忽略。
func (c *Client) UnbanChatMember(chatID, userID string, onlyIfBanned bool) error {
	uid, err := parseID(userID)
	if err != nil {
		return fmt.Errorf("用户 ID 不合法: %w", err)
	}
	return c.Call("unbanChatMember", map[string]interface{}{
		"chat_id": chatID, "user_id": uid, "only_if_banned": onlyIfBanned,
	}, nil)
}

// GetChat 取群/频道信息。
func (c *Client) GetChat(chatID string) (ChatFullInfo, error) {
	var out ChatFullInfo
	err := c.Call("getChat", map[string]interface{}{"chat_id": chatID}, &out)
	return out, err
}

// GetChatMember 取成员状态。
func (c *Client) GetChatMember(chatID, userID string) (ChatMember, error) {
	uid, err := parseID(userID)
	if err != nil {
		return ChatMember{}, fmt.Errorf("用户 ID 不合法: %w", err)
	}
	var out ChatMember
	err = c.Call("getChatMember", map[string]interface{}{"chat_id": chatID, "user_id": uid}, &out)
	return out, err
}

// GetChatAdministrators 取管理员列表（Telegram 不提供普通成员列表）。
func (c *Client) GetChatAdministrators(chatID string) ([]ChatMember, error) {
	var out []ChatMember
	err := c.Call("getChatAdministrators", map[string]interface{}{"chat_id": chatID}, &out)
	return out, err
}

// GetChatMemberCount 取成员数。
func (c *Client) GetChatMemberCount(chatID string) (int, error) {
	var n int
	err := c.Call("getChatMemberCount", map[string]interface{}{"chat_id": chatID}, &n)
	return n, err
}

// ApproveChatJoinRequest 通过入群申请。
func (c *Client) ApproveChatJoinRequest(chatID, userID string) error {
	uid, err := parseID(userID)
	if err != nil {
		return fmt.Errorf("用户 ID 不合法: %w", err)
	}
	return c.Call("approveChatJoinRequest", map[string]interface{}{"chat_id": chatID, "user_id": uid}, nil)
}

// DeclineChatJoinRequest 拒绝入群申请。
func (c *Client) DeclineChatJoinRequest(chatID, userID string) error {
	uid, err := parseID(userID)
	if err != nil {
		return fmt.Errorf("用户 ID 不合法: %w", err)
	}
	return c.Call("declineChatJoinRequest", map[string]interface{}{"chat_id": chatID, "user_id": uid}, nil)
}

// AnswerCallbackQuery 回掉按钮事件（管理端内联键盘用）。
func (c *Client) AnswerCallbackQuery(id, text string) error {
	p := map[string]interface{}{"callback_query_id": id}
	if text != "" {
		p["text"] = text
	}
	return c.Call("answerCallbackQuery", p, nil)
}

// SetMyCommands 设置命令菜单（/start /help /status ...）。
func (c *Client) SetMyCommands(commands []map[string]string) error {
	return c.Call("setMyCommands", map[string]interface{}{"commands": commands}, nil)
}

// SendChatAction 发送"正在输入"等状态。
func (c *Client) SendChatAction(chatID, action string) error {
	return c.Call("sendChatAction", map[string]interface{}{"chat_id": chatID, "action": action}, nil)
}

// LeaveChat 让机器人退群。
func (c *Client) LeaveChat(chatID string) error {
	return c.Call("leaveChat", map[string]interface{}{"chat_id": chatID}, nil)
}

// ---------------- helpers ----------------

func parseID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("空 ID")
	}
	return strconv.ParseInt(s, 10, 64)
}

func strconvI64(n int64) string { return strconv.FormatInt(n, 10) }

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// utf16Len 返回 Telegram 计长度用的 UTF-16 code unit 数。
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// splitUTF16 按 UTF-16 code unit 切分，尽量在换行/句号处断开，且不劈开 emoji 代理对。
func splitUTF16(s string, limit int) (string, string) {
	u := utf16.Encode([]rune(s))
	if len(u) <= limit {
		return s, ""
	}
	cut := limit
	// 尽量往前找自然断点（最多回退 10%），避免把一句话拆开。
	for i := limit; i > limit*9/10; i-- {
		if u[i-1] == '\n' || u[i-1] == '。' || u[i-1] == '.' || u[i-1] == ' ' {
			cut = i
			break
		}
	}
	// 别切在代理对中间。
	if cut > 0 && cut < len(u) && u[cut-1] >= 0xD800 && u[cut-1] <= 0xDBFF {
		cut--
	}
	return string(utf16.Decode(u[:cut])), string(utf16.Decode(u[cut:]))
}

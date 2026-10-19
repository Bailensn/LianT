package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
			"inline_query", "chosen_inline_result", "poll_answer",
			"pre_checkout_query", "shipping_query",
			"business_connection", "business_message", "edited_business_message",
			"deleted_business_messages",
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
	ParseMode           string      // 空 = 纯文本；HTML / MarkdownV2
	DisableNotification bool
	ReplyMarkup         interface{} // inline_keyboard 等
	ProtectContent      bool
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
	if opt.ReplyMarkup != nil {
		p["reply_markup"] = opt.ReplyMarkup
	}
	if opt.ProtectContent {
		p["protect_content"] = true
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

// SendMedia 发送媒体消息。kind 支持：
// photo / document / video / audio / voice / sticker / animation / video_note /
// location（media="纬度,经度"）/ poll（media=问题，caption 里用 | 分隔选项）/ dice。
func (c *Client) SendMedia(chatID, kind, media, caption string, opt SendOptions) (Message, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	media = strings.TrimSpace(media)
	if media == "" {
		return Message{}, errors.New("缺少媒体 file_id / URL / 坐标")
	}
	p := baseSendParams(chatID, opt)
	var method string
	switch kind {
	case "photo":
		method = "sendPhoto"
		p["photo"] = media
	case "document":
		method = "sendDocument"
		p["document"] = media
	case "video":
		method = "sendVideo"
		p["video"] = media
	case "audio":
		method = "sendAudio"
		p["audio"] = media
	case "voice":
		method = "sendVoice"
		p["voice"] = media
	case "sticker":
		method = "sendSticker"
		p["sticker"] = media
	case "animation":
		method = "sendAnimation"
		p["animation"] = media
	case "video_note":
		method = "sendVideoNote"
		p["video_note"] = media
	case "dice":
		method = "sendDice"
		if media != "" && media != "0" {
			p["emoji"] = media
		}
	case "location":
		parts := strings.SplitN(media, ",", 2)
		if len(parts) != 2 {
			return Message{}, errors.New("位置格式应为 纬度,经度")
		}
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil {
			return Message{}, errors.New("位置格式应为 纬度,经度")
		}
		method = "sendLocation"
		p["latitude"] = lat
		p["longitude"] = lon
	case "poll":
		opts := splitPollOptions(caption)
		if len(opts) < 2 {
			return Message{}, errors.New("投票需要至少 2 个选项（在说明里用 | 分隔）")
		}
		method = "sendPoll"
		p["question"] = media
		p["options"] = pollOptionsJSON(opts)
		caption = ""
	default:
		return Message{}, errors.New("不支持的媒体类型: " + kind)
	}
	// 这些类型 Telegram 不接受 caption，避免管理员误填导致 400。
	if kind == "voice" || kind == "sticker" || kind == "video_note" ||
		kind == "location" || kind == "dice" || kind == "poll" {
		caption = ""
	}
	if caption != "" {
		p["caption"] = caption
	}
	var m Message
	if err := c.Call(method, p, &m); err != nil {
		return Message{}, err
	}
	return m, nil
}

// SendMediaGroup 相册（2-10 个媒体）。items 里每项形如
// {"type":"photo","media":"file_id 或 URL","caption":"..."}。
func (c *Client) SendMediaGroup(chatID string, items []map[string]interface{}, opt SendOptions) ([]Message, error) {
	if len(items) < 2 || len(items) > 10 {
		return nil, errors.New("相册需要 2-10 个媒体")
	}
	p := baseSendParams(chatID, opt)
	p["media"] = items
	var out []Message
	if err := c.Call("sendMediaGroup", p, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func baseSendParams(chatID string, opt SendOptions) map[string]interface{} {
	p := map[string]interface{}{
		"chat_id":              chatID,
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
	if opt.ReplyMarkup != nil {
		p["reply_markup"] = opt.ReplyMarkup
	}
	if opt.ProtectContent {
		p["protect_content"] = true
	}
	return p
}

func splitPollOptions(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "|") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func pollOptionsJSON(opts []string) []map[string]string {
	out := make([]map[string]string, 0, len(opts))
	for _, o := range opts {
		out = append(out, map[string]string{"text": o})
	}
	return out
}

// SendLocation 发送位置。
func (c *Client) SendLocation(chatID string, lat, lon float64, opt SendOptions) (Message, error) {
	p := baseSendParams(chatID, opt)
	p["latitude"], p["longitude"] = lat, lon
	var m Message
	err := c.Call("sendLocation", p, &m)
	return m, err
}

// SendPoll 发送投票。options 至少 2 个。
func (c *Client) SendPoll(chatID, question string, options []string, opt SendOptions) (Message, error) {
	if len(options) < 2 {
		return Message{}, errors.New("投票需要至少 2 个选项")
	}
	p := baseSendParams(chatID, opt)
	p["question"], p["options"] = question, pollOptionsJSON(options)
	var m Message
	err := c.Call("sendPoll", p, &m)
	return m, err
}

// SendDice 发送骰子/飞镖/篮球等。
func (c *Client) SendDice(chatID, emoji string, opt SendOptions) (Message, error) {
	p := baseSendParams(chatID, opt)
	if emoji != "" {
		p["emoji"] = emoji
	}
	var m Message
	err := c.Call("sendDice", p, &m)
	return m, err
}

// EditMessageText 编辑文本消息。
func (c *Client) EditMessageText(chatID string, messageID int64, text string, opt SendOptions) error {
	p := map[string]interface{}{"chat_id": chatID, "message_id": messageID, "text": text}
	if opt.ParseMode != "" {
		p["parse_mode"] = opt.ParseMode
	}
	if opt.ReplyMarkup != nil {
		p["reply_markup"] = opt.ReplyMarkup
	}
	if opt.DisablePreview {
		p["link_preview_options"] = map[string]interface{}{"is_disabled": true}
	}
	return c.Call("editMessageText", p, nil)
}

// EditMessageCaption 编辑媒体说明。
func (c *Client) EditMessageCaption(chatID string, messageID int64, caption string, opt SendOptions) error {
	p := map[string]interface{}{"chat_id": chatID, "message_id": messageID, "caption": caption}
	if opt.ParseMode != "" {
		p["parse_mode"] = opt.ParseMode
	}
	if opt.ReplyMarkup != nil {
		p["reply_markup"] = opt.ReplyMarkup
	}
	return c.Call("editMessageCaption", p, nil)
}

// EditMessageReplyMarkup 只改按钮。
func (c *Client) EditMessageReplyMarkup(chatID string, messageID int64, markup interface{}) error {
	return c.Call("editMessageReplyMarkup", map[string]interface{}{
		"chat_id": chatID, "message_id": messageID, "reply_markup": markup,
	}, nil)
}

// ForwardMessage 转发消息。
func (c *Client) ForwardMessage(chatID, fromChatID string, messageID, threadID int64) (Message, error) {
	p := map[string]interface{}{"chat_id": chatID, "from_chat_id": fromChatID, "message_id": messageID}
	if threadID > 0 {
		p["message_thread_id"] = threadID
	}
	var m Message
	err := c.Call("forwardMessage", p, &m)
	return m, err
}

// CopyMessage 复制消息（不带"转发自"）。
func (c *Client) CopyMessage(chatID, fromChatID string, messageID, threadID int64) (int64, error) {
	p := map[string]interface{}{"chat_id": chatID, "from_chat_id": fromChatID, "message_id": messageID}
	if threadID > 0 {
		p["message_thread_id"] = threadID
	}
	var out struct {
		MessageID int64 `json:"message_id"`
	}
	err := c.Call("copyMessage", p, &out)
	return out.MessageID, err
}

// PinChatMessage / UnpinChatMessage / UnpinAllChatMessages。
func (c *Client) PinChatMessage(chatID string, messageID int64, disableNotify bool) error {
	p := map[string]interface{}{"chat_id": chatID, "message_id": messageID, "disable_notification": disableNotify}
	return c.Call("pinChatMessage", p, nil)
}

func (c *Client) UnpinChatMessage(chatID string, messageID int64) error {
	p := map[string]interface{}{"chat_id": chatID}
	if messageID > 0 {
		p["message_id"] = messageID
	}
	return c.Call("unpinChatMessage", p, nil)
}

func (c *Client) UnpinAllChatMessages(chatID string) error {
	return c.Call("unpinAllChatMessages", map[string]interface{}{"chat_id": chatID}, nil)
}

// SetChatTitle / SetChatDescription / SetChatPermissions / DeleteChatPhoto。
func (c *Client) SetChatTitle(chatID, title string) error {
	return c.Call("setChatTitle", map[string]interface{}{"chat_id": chatID, "title": title}, nil)
}

func (c *Client) SetChatDescription(chatID, desc string) error {
	return c.Call("setChatDescription", map[string]interface{}{"chat_id": chatID, "description": desc}, nil)
}

func (c *Client) SetChatPermissions(chatID string, permissions map[string]interface{}) error {
	return c.Call("setChatPermissions", map[string]interface{}{"chat_id": chatID, "permissions": permissions}, nil)
}

func (c *Client) DeleteChatPhoto(chatID string) error {
	return c.Call("deleteChatPhoto", map[string]interface{}{"chat_id": chatID}, nil)
}

// CreateChatInviteLink 创建邀请链接，返回 invite_link。
func (c *Client) CreateChatInviteLink(chatID, name string, expireDate int64, memberLimit int) (string, error) {
	p := map[string]interface{}{"chat_id": chatID}
	if name != "" {
		p["name"] = name
	}
	if expireDate > 0 {
		p["expire_date"] = expireDate
	}
	if memberLimit > 0 {
		p["member_limit"] = memberLimit
	}
	var out struct {
		InviteLink string `json:"invite_link"`
	}
	err := c.Call("createChatInviteLink", p, &out)
	return out.InviteLink, err
}

// RevokeChatInviteLink 撤销邀请链接。
func (c *Client) RevokeChatInviteLink(chatID, link string) error {
	return c.Call("revokeChatInviteLink", map[string]interface{}{"chat_id": chatID, "invite_link": link}, nil)
}

// CreateForumTopic 创建论坛话题，返回 message_thread_id。
func (c *Client) CreateForumTopic(chatID, name string, iconColor int) (int64, error) {
	p := map[string]interface{}{"chat_id": chatID, "name": name}
	if iconColor > 0 {
		p["icon_color"] = iconColor
	}
	var out struct {
		MessageThreadID int64 `json:"message_thread_id"`
	}
	err := c.Call("createForumTopic", p, &out)
	return out.MessageThreadID, err
}

func (c *Client) EditForumTopic(chatID string, threadID int64, name string) error {
	p := map[string]interface{}{"chat_id": chatID, "message_thread_id": threadID}
	if name != "" {
		p["name"] = name
	}
	return c.Call("editForumTopic", p, nil)
}

func (c *Client) CloseForumTopic(chatID string, threadID int64) error {
	return c.Call("closeForumTopic", map[string]interface{}{"chat_id": chatID, "message_thread_id": threadID}, nil)
}

func (c *Client) ReopenForumTopic(chatID string, threadID int64) error {
	return c.Call("reopenForumTopic", map[string]interface{}{"chat_id": chatID, "message_thread_id": threadID}, nil)
}

func (c *Client) DeleteForumTopic(chatID string, threadID int64) error {
	return c.Call("deleteForumTopic", map[string]interface{}{"chat_id": chatID, "message_thread_id": threadID}, nil)
}

func (c *Client) UnpinAllForumTopicMessages(chatID string, threadID int64) error {
	return c.Call("unpinAllForumTopicMessages", map[string]interface{}{"chat_id": chatID, "message_thread_id": threadID}, nil)
}

// AnswerInlineQuery 回复内联查询。results 直接是 Bot API 的 InlineQueryResult 数组。
func (c *Client) AnswerInlineQuery(id string, results []map[string]interface{}, cacheTime int) error {
	p := map[string]interface{}{"inline_query_id": id, "results": results}
	if cacheTime > 0 {
		p["cache_time"] = cacheTime
	}
	return c.Call("answerInlineQuery", p, nil)
}

// SetWebhook 设置 webhook。
func (c *Client) SetWebhook(webhookURL, secret string, allowed []string) error {
	p := map[string]interface{}{"url": webhookURL}
	if secret != "" {
		p["secret_token"] = secret
	}
	if len(allowed) > 0 {
		p["allowed_updates"] = allowed
	}
	return c.Call("setWebhook", p, nil)
}

// GetWebhookInfo 查询 webhook 状态。
func (c *Client) GetWebhookInfo() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.Call("getWebhookInfo", nil, &out)
	return out, err
}

// SetMyDescription / SetMyShortDescription / DeleteMyCommands。
func (c *Client) SetMyDescription(desc string) error {
	return c.Call("setMyDescription", map[string]interface{}{"description": desc}, nil)
}

func (c *Client) SetMyShortDescription(desc string) error {
	return c.Call("setMyShortDescription", map[string]interface{}{"short_description": desc}, nil)
}

func (c *Client) DeleteMyCommands() error {
	return c.Call("deleteMyCommands", nil, nil)
}

// CallMultipart 用 multipart/form-data 调一个需要上传本地文件的方法。
// params 里的非字符串值会转成 JSON 文本；fileField 是 Telegram 要的字段名
// （sendPhoto=photo、sendDocument=document、createNewStickerSet=png_sticker 等）。
func (c *Client) CallMultipart(method string, params map[string]interface{}, fileField, filePath string) (json.RawMessage, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range params {
		s, ok := v.(string)
		if !ok {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("参数 %s 编码失败: %w", k, err)
			}
			s = string(b)
		}
		if err := mw.WriteField(k, s); err != nil {
			return nil, err
		}
	}
	if fileField == "" {
		fileField = "file"
	}
	part, err := mw.CreateFormFile(fileField, filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.base+"/bot"+c.token+"/"+method, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var r Response
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("Telegram 响应无法解析(HTTP %d)", resp.StatusCode)
	}
	if !r.OK {
		ae := &APIError{Method: method, Code: r.ErrorCode, Description: r.Description, RetryAfter: r.Parameters.RetryAfter}
		if ae.Code == 0 {
			ae.Code = resp.StatusCode
		}
		return nil, ae
	}
	return r.Result, nil
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

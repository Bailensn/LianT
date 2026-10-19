package tg

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

// ============================================================================
// Telegram Bot API 数据结构（只保留 LianT 用得到的字段）
// 文档：https://core.telegram.org/bots/api
// ============================================================================

// User 是 Telegram 用户（私聊用户 / 群成员 / 机器人自己）。
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Display 返回适合展示/日志的名字。
func (u User) Display() string {
	if u.Username != "" {
		return u.Username
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	return fmt.Sprintf("%d", u.ID)
}

// At 返回 @username（没有用户名时返回名字）。
func (u User) At() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return u.Display()
}

// String 让日志里直接打印 %s。
func (u User) String() string { return u.Display() }

// Chat 是会话：私聊、群、超级群、频道。
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // private | group | supergroup | channel
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsForum   bool   `json:"is_forum"`
}

// Display 返回会话名（群名 / 私聊用户昵称）。
func (c Chat) Display() string {
	if c.Title != "" {
		return c.Title
	}
	if c.Username != "" {
		return "@" + c.Username
	}
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name != "" {
		return name
	}
	return fmt.Sprintf("%d", c.ID)
}

// IsGroup 判断是否群组（含超级群）。
func (c Chat) IsGroup() bool { return c.Type == "group" || c.Type == "supergroup" }

// ChatID 把数字 ID 转成字符串（存储层统一用字符串，便于和旧库/配置互通）。
func (c Chat) ChatID() string { return strconvI64(c.ID) }

// MessageEntity 是消息里的实体（@提及、命令、链接、加粗等）。
type MessageEntity struct {
	Type   string `json:"type"` // mention | text_mention | bot_command | url | ...
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	User   *User  `json:"user,omitempty"`
}

// Message 是一条消息。
type Message struct {
	MessageID          int64           `json:"message_id"`
	MessageThreadID    int64           `json:"message_thread_id"`
	From               *User           `json:"from"`
	SenderChat         *Chat           `json:"sender_chat"`
	Date               int64           `json:"date"`
	Chat               Chat            `json:"chat"`
	Text               string          `json:"text"`
	Entities           []MessageEntity `json:"entities"`
	Caption            string          `json:"caption"`
	CaptionEntities    []MessageEntity `json:"caption_entities"`
	ReplyToMessage     *Message        `json:"reply_to_message"`
	NewChatMembers     []User          `json:"new_chat_members"`
	LeftChatMember     *User           `json:"left_chat_member"`
	IsAutomaticForward bool            `json:"is_automatic_forward"`

	Photo     []PhotoSize `json:"photo"`
	Video     *Video      `json:"video"`
	Voice     *Voice      `json:"voice"`
	Audio     *Audio      `json:"audio"`
	Document  *Document   `json:"document"`
	Sticker   *Sticker    `json:"sticker"`
	Animation *Animation  `json:"animation"`
	VideoNote *VideoNote  `json:"video_note"`
	Poll      *Poll       `json:"poll"`
	Location  *Location   `json:"location"`
	Contact   *Contact    `json:"contact"`
}

// 下面是最小化的媒体类型（只用于分类/展示，不处理下载）。
type PhotoSize struct {
	FileID string `json:"file_id"`
}

type Video struct {
	FileID string `json:"file_id"`
}

type Voice struct {
	FileID string `json:"file_id"`
}

type Audio struct {
	FileID string `json:"file_id"`
}

type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

type Sticker struct {
	FileID string `json:"file_id"`
}

type Animation struct {
	FileID string `json:"file_id"`
}

type VideoNote struct {
	FileID string `json:"file_id"`
}

type Poll struct {
	ID       string `json:"id"`
	Question string `json:"question"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
}

// Content 返回消息正文（文本或媒体说明）。
func (m *Message) Content() string {
	if m == nil {
		return ""
	}
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
}

// EntityText 取出实体对应的原文片段。
// Telegram 的 offset/length 按 UTF-16 code unit 计（emoji 占 2），不能用 rune 下标。
func (m *Message) EntityText(e MessageEntity) string {
	units := utf16.Encode([]rune(m.Content()))
	if e.Offset < 0 || e.Offset > len(units) {
		return ""
	}
	end := e.Offset + e.Length
	if end > len(units) {
		end = len(units)
	}
	return string(utf16.Decode(units[e.Offset:end]))
}

// IsMentioned 判断消息是否 @ 了指定用户名（不带 @）或用户 ID。
func (m *Message) IsMentioned(username string, userID int64) bool {
	if m == nil {
		return false
	}
	entities := m.Entities
	if len(entities) == 0 && len(m.CaptionEntities) > 0 {
		entities = m.CaptionEntities
	}
	for _, e := range entities {
		switch e.Type {
		case "mention":
			if username != "" && strings.EqualFold(m.EntityText(e), "@"+username) {
				return true
			}
		case "text_mention":
			if e.User != nil && e.User.ID == userID {
				return true
			}
		}
	}
	// 回复机器人也算 @（Telegram 的 reply 没有实体）。
	if m.ReplyToMessage != nil && m.ReplyToMessage.From != nil && m.ReplyToMessage.From.ID == userID {
		return true
	}
	return false
}

// IsCommand 判断消息是否以 @bot 的命令形式发给机器人。
func (m *Message) IsCommand(username string) bool {
	if m == nil {
		return false
	}
	for _, e := range m.Entities {
		if e.Type != "bot_command" {
			continue
		}
		cmd := m.EntityText(e)
		if i := strings.Index(cmd, "@"); i >= 0 {
			return strings.EqualFold(cmd[i+1:], username)
		}
		return true
	}
	return false
}

// StripBotMention 删掉消息里的 @botusername（保留其它内容）。
func (m *Message) StripBotMention(username string) string {
	text := m.Content()
	if username == "" || text == "" {
		return strings.TrimSpace(text)
	}
	mention := "@" + username
	out := strings.ReplaceAll(text, mention, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(out), " "))
}

// Update 是 getUpdates 返回的一次更新。
type Update struct {
	UpdateID          int64              `json:"update_id"`
	Message           *Message           `json:"message"`
	EditedMessage     *Message           `json:"edited_message"`
	ChannelPost       *Message           `json:"channel_post"`
	EditedChannelPost *Message           `json:"edited_channel_post"`
	MyChatMember      *ChatMemberUpdated `json:"my_chat_member"`
	ChatMember        *ChatMemberUpdated `json:"chat_member"`
	ChatJoinRequest   *ChatJoinRequest   `json:"chat_join_request"`
	CallbackQuery     *CallbackQuery     `json:"callback_query"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// ChatMemberUpdated 是机器人自身/成员状态变化事件。
type ChatMemberUpdated struct {
	Chat          Chat            `json:"chat"`
	From          User            `json:"from"`
	Date          int64           `json:"date"`
	OldChatMember ChatMember      `json:"old_chat_member"`
	NewChatMember ChatMember      `json:"new_chat_member"`
	InviteLink    *ChatInviteLink `json:"invite_link"`
}

type ChatInviteLink struct {
	InviteLink string `json:"invite_link"`
	Name       string `json:"name"`
}

// ChatJoinRequest 是入群申请（群链接需要管理员批准时）。
type ChatJoinRequest struct {
	Chat       Chat            `json:"chat"`
	From       User            `json:"from"`
	UserChatID int64           `json:"user_chat_id"`
	Date       int64           `json:"date"`
	Bio        string          `json:"bio"`
	InviteLink *ChatInviteLink `json:"invite_link"`
}

// ChatMember 是群成员状态。
// status: creator | administrator | member | restricted | left | kicked
type ChatMember struct {
	Status             string `json:"status"`
	User               User   `json:"user"`
	CustomTitle        string `json:"custom_title"`
	IsAnonymous        bool   `json:"is_anonymous"`
	UntilDate          int64  `json:"until_date"`
	CanBeEdited        bool   `json:"can_be_edited"`
	CanManageChat      bool   `json:"can_manage_chat"`
	CanDeleteMessages  bool   `json:"can_delete_messages"`
	CanRestrictMembers bool   `json:"can_restrict_members"`
	CanPromoteMembers  bool   `json:"can_promote_members"`
	CanInviteUsers     bool   `json:"can_invite_users"`
	CanPinMessages     bool   `json:"can_pin_messages"`
	CanSendMessages    *bool  `json:"can_send_messages,omitempty"`
}

// IsAdmin 判断成员是否管理员/群主。
func (m ChatMember) IsAdmin() bool {
	return m.Status == "creator" || m.Status == "administrator"
}

// IsRestricted 判断是否被限制（禁言在 Telegram 里表现为 restricted）。
func (m ChatMember) IsRestricted() bool { return m.Status == "restricted" }

// Muted 判断当前是否发不了言。
func (m ChatMember) Muted() bool {
	return m.IsRestricted() && m.CanSendMessages != nil && !*m.CanSendMessages
}

// ChatFullInfo 是 getChat 的返回（比 Chat 多一些字段）。
type ChatFullInfo struct {
	Chat
	Description   string           `json:"description"`
	InviteLink    string           `json:"invite_link"`
	PinnedMessage *Message         `json:"pinned_message"`
	Permissions   *ChatPermissions `json:"permissions"`
}

// ChatPermissions 是群默认/成员权限。
type ChatPermissions struct {
	CanSendMessages       bool `json:"can_send_messages"`
	CanSendAudios         bool `json:"can_send_audios"`
	CanSendDocuments      bool `json:"can_send_documents"`
	CanSendPhotos         bool `json:"can_send_photos"`
	CanSendVideos         bool `json:"can_send_videos"`
	CanSendVideoNotes     bool `json:"can_send_video_notes"`
	CanSendVoiceNotes     bool `json:"can_send_voice_notes"`
	CanSendPolls          bool `json:"can_send_polls"`
	CanSendOtherMessages  bool `json:"can_send_other_messages"`
	CanAddWebPagePreviews bool `json:"can_add_web_page_previews"`
	CanChangeInfo         bool `json:"can_change_info"`
	CanInviteUsers        bool `json:"can_invite_users"`
	CanPinMessages        bool `json:"can_pin_messages"`
	CanManageTopics       bool `json:"can_manage_topics"`
}

// APIError 是 Telegram 返回的业务错误。
type APIError struct {
	Method      string
	Code        int
	Description string
	RetryAfter  int
	MigrateTo   int64
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("Telegram %s 失败(%d)：%s", e.Method, e.Code, e.Description)
}

// Hint 把常见错误翻成中文排查建议。
func (e *APIError) Hint() string {
	if e == nil {
		return ""
	}
	d := strings.ToLower(e.Description)
	switch {
	case strings.Contains(d, "not enough rights"):
		return "机器人权限不足：请在群里把机器人设为管理员，并勾选删除消息/封禁用户等权限"
	case strings.Contains(d, "chat not found"):
		return "找不到会话：确认机器人已在群里，且 chat_id 正确（群 ID 通常是负数）"
	case strings.Contains(d, "bot is not a member"):
		return "机器人不在该群里：先把它拉进群"
	case strings.Contains(d, "message to delete not found"):
		return "这条消息已经不存在（可能已被删除，或超过 48 小时）"
	case strings.Contains(d, "message can't be deleted"):
		return "这条消息删不了：机器人需要管理员权限，且消息不能超过 48 小时"
	case strings.Contains(d, "user is an administrator"):
		return "对方是管理员，机器人不能对其禁言/封禁"
	case strings.Contains(d, "can't restrict self"):
		return "不能限制机器人自己"
	case strings.Contains(d, "not enough rights to send"):
		return "机器人没有发言权限：检查群权限或机器人是否被禁言"
	case strings.Contains(d, "too many requests"):
		return fmt.Sprintf("触发 Telegram 限流，%d 秒后重试", e.RetryAfter)
	case strings.Contains(d, "chat_write_forbidden"):
		return "群里禁止发言"
	}
	return ""
}

// Response 是 Bot API 的统一响应信封。
type Response struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  struct {
		MigrateToChatID int64 `json:"migrate_to_chat_id"`
		RetryAfter      int   `json:"retry_after"`
	} `json:"parameters"`
}

package protocol

import (
	"fmt"

	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/platform"
)

// 消息类型，与 docs/PROTOCOL.md §6 对应。
const (
	TypePairRequest = "pair.request"
	TypePairOK      = "pair.ok"
	TypePairDenied  = "pair.denied"

	TypeAuthChallenge = "auth.challenge"
	TypeAuthResponse  = "auth.response"
	TypeAuthOK        = "auth.ok"

	TypeBotList   = "bot.list"
	TypeBotAdd    = "bot.add"
	TypeBotRemove = "bot.remove"
	TypeBotStart  = "bot.start"
	TypeBotStop   = "bot.stop"
	TypeBotStatus = "bot.status"

	TypeMsgList    = "msg.list"
	TypeMsgSend    = "msg.send"
	TypeMsgInbound = "msg.inbound"
	TypeMsgAck     = "msg.ack"
	TypeMediaFetch = "media.fetch"

	TypeEventBot = "event.bot"
	TypeEventMsg = "event.msg"

	TypeError = "error"
	TypePing  = "ping"
	TypePong  = "pong"
)

// Cur 是分页游标 {ts, msg_id}。
type Cur struct {
	TS    int64  `json:"ts"`
	MsgID string `json:"msg_id"`
}

// Body 是一条消息的内容：plain 必填，doc 可选（D8）。
type Body struct {
	Plain string      `json:"plain"`
	Doc   *format.Doc `json:"doc,omitempty"`
}

// Validate 检查 body 是否可下发：plain 与 doc 不能都为空，doc 必须合法。
func (b Body) Validate() error {
	if b.Plain == "" && b.Doc == nil {
		return fmt.Errorf("%w: body 既没有 plain 也没有 doc", ErrInvalidEnvelope)
	}
	if b.Doc != nil {
		if err := format.Validate(b.Doc); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEnvelope, err)
		}
	}
	return nil
}

// ==========================
// 配对与认证
// ==========================

// PairRequest 是 Manager 扫码后的第一次请求。
//
// Ticket 是扫码得到的配对码；Sig 是对二维码里 nonce 的签名，
// 证明请求方确实持有待绑定私钥。
type PairRequest struct {
	Ticket      string `json:"ticket"`
	ManagerName string `json:"manager_name"`
	ManagerPub  string `json:"manager_pub"`
	Sig         string `json:"sig"`
}

// PairOK 表示配对成功。
type PairOK struct {
	ServiceID    string `json:"service_id"`
	ServiceName  string `json:"service_name"`
	DevicesCount int    `json:"devices_count"`
	// DeviceID 是本次授权得到的设备标识，之后 auth.response 用它。
	// 协议 v1 只加字段不升版本。
	DeviceID string `json:"device_id"`
}

// PairDenied 表示配对被拒绝。
type PairDenied struct {
	Reason string `json:"reason"`
}

// AuthChallenge 是每次连接的挑战。
type AuthChallenge struct {
	Nonce string `json:"nonce"`
	TS    int64  `json:"ts"`
}

// AuthResponse 是 Manager 对挑战的应答。
type AuthResponse struct {
	ManagerID string `json:"manager_id"`
	Sig       string `json:"sig"`
}

// Retention 是服务端保留策略的对外声明。
type Retention struct {
	Days             int   `json:"days"`
	TextMaxBytes     int64 `json:"text_max_bytes"`
	MediaMaxBytes    int64 `json:"media_max_bytes"`
	CleanupIntervalS int   `json:"cleanup_interval_s"`
	Evicted          bool  `json:"evicted"`
}

// AuthOK 表示认证通过并下发服务端能力。
type AuthOK struct {
	ServiceTime int64     `json:"service_time"`
	Caps        []string  `json:"caps"`
	Retention   Retention `json:"retention"`
}

// ==========================
// Bot
// ==========================

// Bot 是 bot.list 返回的单个 Bot。
type Bot struct {
	BotID       string         `json:"bot_id"`
	Platform    string         `json:"platform"`
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name"`
	Status      string         `json:"status"`
	Caps        []string       `json:"caps"`
	Limits      map[string]any `json:"limits"`
	Constraints map[string]any `json:"constraints"`
	Since       int64          `json:"since,omitempty"`
}

// BotList 是 bot.list 的响应。
type BotList struct {
	Bots []Bot `json:"bots"`
}

// BotAdd 是登记一个 Bot 的请求；凭据形状由平台定义。
type BotAdd struct {
	Platform    string            `json:"platform"`
	Credentials map[string]string `json:"credentials"`
	Label       string            `json:"label,omitempty"`
}

// Validate 做平台无关的形状检查；凭据语义由适配器校验。
func (r BotAdd) Validate() error {
	if r.Platform == "" {
		return fmt.Errorf("缺少 platform")
	}
	if len(r.Credentials) == 0 {
		return fmt.Errorf("缺少 credentials")
	}
	return nil
}

// BotRef 是只带 bot_id 的请求/响应。
type BotRef struct {
	BotID string `json:"bot_id"`
}

// BotStatusOK 是 bot.status 的响应。
type BotStatusOK struct {
	Status    string `json:"status"`
	PID       int    `json:"pid,omitempty"`
	Since     int64  `json:"since,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

// ==========================
// 消息
// ==========================

// MsgSend 是 Manager 发起的回复。
//
// InReplyTo 是触发本次回复的入站 msg_id（QQ 被动回复必须带）；
// OperatorID 只用于审计，永不下发平台。
type MsgSend struct {
	BotID      string `json:"bot_id"`
	PeerID     string `json:"peer_id"`
	Body       Body   `json:"body"`
	InReplyTo  string `json:"in_reply_to,omitempty"`
	OperatorID string `json:"operator_id,omitempty"`
}

// Validate 检查必填字段与 ID 形状。
func (r MsgSend) Validate() error {
	if !platform.IsValidID(r.BotID) {
		return fmt.Errorf("非法 bot_id %q", r.BotID)
	}
	if !platform.IsValidID(r.PeerID) {
		return fmt.Errorf("非法 peer_id %q", r.PeerID)
	}
	if err := r.Body.Validate(); err != nil {
		return err
	}
	return nil
}

// MsgSendOK 是 msg.send 的成功响应。
type MsgSendOK struct {
	MsgID string `json:"msg_id"`
	TS    int64  `json:"ts"`
}

// MsgInbound 是一条入站消息，也用于 msg.list 与 event.msg。
type MsgInbound struct {
	BotID      string `json:"bot_id"`
	PeerID     string `json:"peer_id"`
	PeerKind   string `json:"peer_kind"`
	SenderID   string `json:"sender_id,omitempty"`
	SenderName string `json:"sender_name,omitempty"`
	MsgType    string `json:"msg_type"`
	Body       Body   `json:"body"`
	MsgID      string `json:"msg_id"`
	TS         int64  `json:"ts"`
	Direction  string `json:"direction,omitempty"`
}

// MsgList 是会话内分页请求。
type MsgList struct {
	BotID  string `json:"bot_id"`
	PeerID string `json:"peer_id"`
	Before *Cur   `json:"before,omitempty"`
	After  *Cur   `json:"after,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// Validate 检查 ID 与唯一游标。
func (r MsgList) Validate() error {
	if !platform.IsValidID(r.BotID) {
		return fmt.Errorf("非法 bot_id %q", r.BotID)
	}
	if !platform.IsValidID(r.PeerID) {
		return fmt.Errorf("非法 peer_id %q", r.PeerID)
	}
	if r.Before != nil && r.After != nil {
		return fmt.Errorf("before 与 after 不能同时出现")
	}
	if r.Limit < 0 || r.Limit > 100 {
		return fmt.Errorf("limit 必须在 0–100 之间")
	}
	return nil
}

// MsgListOK 是 msg.list 的响应。
type MsgListOK struct {
	Messages []MsgInbound `json:"messages"`
	HasMore  bool         `json:"has_more"`
}

// MsgAck 表示客户端已把消息落库。
type MsgAck struct {
	MsgID string `json:"msg_id"`
}

// MediaFetch 请求一条消息的媒体下载地址。
type MediaFetch struct {
	BotID string `json:"bot_id"`
	MsgID string `json:"msg_id"`
}

// MediaFetchOK 返回本地缓存的下载信息。
type MediaFetchOK struct {
	URL  string `json:"url"`
	Exp  int64  `json:"exp"`
	Size int64  `json:"size"`
	MIME string `json:"mime"`
}

// EventBot 是单 Bot 粒度状态变化。
type EventBot struct {
	BotID  string `json:"bot_id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Package protocol 定义 LianT 线上协议 v1 的单层 JSON 信封、错误码与方法载荷。
//
// 契约以 docs/PROTOCOL.md 为准；这里的类型只是它的 Go 表示，
// 不包含任何传输、鉴权或业务逻辑。未知字段一律忽略，未知 type 由上层回
// error.unknown_type。
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Version 是线上协议版本。只加字段不升版本。
const Version = 1

// MaxFrame 是单帧上限：8 MiB，给 5 MiB 媒体的 base64 留出余量。
const MaxFrame = 8 << 20

// Envelope 是唯一的帧形状：单层 JSON，不嵌套 base64。
type Envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	TS      int64           `json:"ts"`
	ReplyTo string          `json:"reply_to,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// New 构造一个请求信封；ts 由调用方注入，便于测试。
func New(id, typ string, ts int64, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("protocol: 序列化 payload: %w", err)
	}
	return Envelope{V: Version, ID: id, Type: typ, TS: ts, Payload: raw}, nil
}

// NewReply 构造一个带 reply_to 的响应信封。
func NewReply(req Envelope, id string, payload any) (Envelope, error) {
	env, err := New(id, req.Type, time.Now().UnixMilli(), payload)
	if err != nil {
		return Envelope{}, err
	}
	env.ReplyTo = req.ID
	return env, nil
}

// DecodePayload 把 payload 解到 v；payload 为空时按零值处理。
func (e Envelope) DecodePayload(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(e.Payload, v); err != nil {
		return fmt.Errorf("protocol: 解析 %s 的 payload: %w", e.Type, err)
	}
	return nil
}

// Validate 检查信封本身是否合法（不看 payload 语义）。
func (e Envelope) Validate() error {
	switch {
	case e.V != Version:
		return fmt.Errorf("%w: 版本 %d", ErrUnsupportedVersion, e.V)
	case e.ID == "":
		return fmt.Errorf("%w: 缺少 id", ErrInvalidEnvelope)
	case len(e.ID) > 64:
		return fmt.Errorf("%w: id 过长", ErrInvalidEnvelope)
	case e.Type == "":
		return fmt.Errorf("%w: 缺少 type", ErrInvalidEnvelope)
	case len(e.Type) > 64:
		return fmt.Errorf("%w: type 过长", ErrInvalidEnvelope)
	case len(e.Payload) > MaxFrame:
		return fmt.Errorf("%w: payload 超过 %d 字节", ErrFrameTooLarge, MaxFrame)
	}
	return nil
}

// 错误码，与 docs/PROTOCOL.md §7 一一对应。
const (
	CodeUnauthorized          = "unauthorized"
	CodeForbidden             = "forbidden"
	CodeNotFound              = "not_found"
	CodeInvalidPayload        = "invalid_payload"
	CodeInvalidDoc            = "invalid_doc"
	CodeContentTooLong        = "content_too_long"
	CodeUnsupportedCapability = "unsupported_capability"
	CodeReplyWindowClosed     = "reply_window_closed"
	CodeMentionRequired       = "mention_required"
	CodeUnsupportedVersion    = "unsupported_version"
	CodeRateLimited           = "rate_limited"
	CodeClockSkew             = "clock_skew"
	CodeBotStartFailed        = "bot_start_failed"
	CodeUpstreamError         = "upstream_error"
	CodeInternal              = "internal"
	CodeUnknownType           = "unknown_type"
	CodeFrameTooLarge         = "frame_too_large"
	CodeMediaExpired          = "media_expired"
	CodeMediaEvicted          = "media_evicted"
)

// Error 是 error 帧的 payload。
//
// Details 承载各错误码自己的附加字段（retry_after_ms、platform_code 等），
// 避免为每个错误码定义一个结构体。
type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// 协议层自身的哨兵错误，供上层用 errors.Is 判断。
var (
	ErrUnsupportedVersion = errors.New("protocol: 不支持的版本")
	ErrInvalidEnvelope    = errors.New("protocol: 非法信封")
	ErrFrameTooLarge      = errors.New("protocol: 帧过大")
)

// 可重试性表：与文档 §7 保持同步。
func Retryable(code string) bool {
	switch code {
	case CodeRateLimited, CodeClockSkew, CodeBotStartFailed, CodeInternal:
		return true
	case CodeUpstreamError:
		return true
	default:
		return false
	}
}

// 服务端能力，auth.ok.caps。
const (
	CapMsgList   = "msg.list"
	CapBotCtrl   = "bot.control"
	CapMediaRecv = "media.recv"
	CapRichText  = "rich_text"
	CapMultiBot  = "multi_bot"
)

// ServerCaps 是 v1 必须提供的能力集合。
func ServerCaps() []string {
	return []string{CapMsgList, CapBotCtrl, CapMediaRecv, CapRichText, CapMultiBot}
}

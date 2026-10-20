// Package chat 是 Service 与 Manager 之间共享的消息投影模型。
//
// 目标不是「把平台消息抹平成一种」，而是给出一条消息在协议层可被确认的语义：
// 只声明客户端能确认的事实，确认不了就写 unknown，不猜。
package chat

import (
	"time"

	"LianT/internal/media"
)

// State 是消息生命周期里客户端能确认的状态。
type State string

const (
	// StateReceived 群内收到该消息（本地观测）。
	StateReceived State = "received"
	// StateSent 平台已受理发送请求；不等于送达，也不等于已读。
	StateSent State = "sent"
	// StateRecalled 已在平台侧撤回；正文不再保留。
	StateRecalled State = "recalled"
	// StateUnknown 无法确认（超时、平台不提供回执、状态丢失）。
	StateUnknown State = "unknown"
)

// Normalize 把空值或未知值归到 unknown，避免下游把空字符串当成功。
func Normalize(s State) State {
	switch s {
	case StateReceived, StateSent, StateRecalled:
		return s
	case "":
		return StateUnknown
	default:
		return s
	}
}

// Message 是对外投影的一条消息。
type Message struct {
	ID       string    `json:"id"`
	ChatID   string    `json:"chat_id"`
	UserID   string    `json:"user_id,omitempty"`
	Username string    `json:"username,omitempty"`
	Content  string    `json:"content,omitempty"`
	Kind     string    `json:"kind,omitempty"`
	Outgoing bool      `json:"outgoing"`
	State    State     `json:"state"`
	At       time.Time `json:"at"`
	// Media 只给事实：类型、名称、大小、能否取回。
	// 平台 file_id / 附件直链不在这里，取回走管理端 /api/media。
	Media *media.Descriptor `json:"media,omitempty"`
}

// Snapshot 是一次消息查询的结果。
type Snapshot struct {
	BotID    int64     `json:"bot_id"`
	ChatID   string    `json:"chat_id,omitempty"`
	Messages []Message `json:"messages"`
}

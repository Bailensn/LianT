// Package platform 定义每种聊天平台接入核心时需要实现的契约。
//
// 核心（supervisor、store、协议层）只依赖这里的类型，不认识任何具体平台；
// 平台差异（凭据形状、传输方式、能力集合）全部留在各自适配器里。
package platform

import (
	"context"
	"errors"
	"time"

	"github.com/Bailensn/LianT/service/internal/format"
)

// ErrUnsupported 表示当前适配器或账号不具备该能力。
var ErrUnsupported = errors.New("platform: 不支持的能力")

// ErrInvalidCredentials 表示平台明确拒绝了凭据，
// 与「网络不通 / 上游 5xx」区分开，便于 CLI 给出不同提示。
var ErrInvalidCredentials = errors.New("platform: 凭据无效")

// CredentialField 描述一个平台凭据字段，供 CLI 生成参数与提示。
type CredentialField struct {
	Key    string
	Label  string
	Secret bool
}

// CredentialSpec 是可选能力：适配器声明自己需要哪些凭据字段。
// CLI 据此生成 --app-id / --token 之类的参数，核心仍然只认 Credentials map。
type CredentialSpec interface {
	CredentialFields() []CredentialField
}

// Credentials 是平台自定义的凭据集合。
//
// 不同平台的字段完全不同：QQ 是 app_id/app_secret，Telegram 只有 token。
// 核心只负责加密存储与透传，不解释字段含义。
type Credentials map[string]string

// Account 是 Validate 的产物，描述一个已校验的平台账号。
//
// ID 是平台内部的原始标识（如 QQ openid 或 Telegram 数字 id），
// 不含命名空间；核心用 MakeID(Name(), ID) 拼成 bot_id。
type Account struct {
	ID          string
	Username    string
	DisplayName string
	Caps        []string
	MaxTextLen  int
	RichText    bool

	// Limits / Constraints 由适配器声明，核心与 Manager 原样消费（协议 §6）。
	// Constraints 只描述“最严格限制”，不是能力清单。
	Limits      map[string]any
	Constraints map[string]any
}

// Peer 是一段会话的对象，ID 已带命名空间（platform:external）。
type Peer struct {
	ID   string
	Kind string
}

// 会话类型。平台映射到各自的概念：QQ 的群/频道/单聊，Telegram 的群/频道/私聊。
const (
	PeerPrivate = "private"
	PeerGroup   = "group"
	PeerChannel = "channel"
)

// Attachment 是入站消息携带的媒体。
//
// URL 是平台侧地址，只在下载时使用；核心会把它落到本地并换成自己的下载地址，
// 客户端永远看不到平台链接。
type Attachment struct {
	URL  string
	MIME string
	Size int64
}

// Inbound 是适配器归一化后交给核心的一条入站消息。
//
// Text 是纯文本降级；Doc 非空时表示平台能给出行内结构，
// 由 format 适配器解析得到，核心只负责存储与下发。
type Inbound struct {
	MessageID  string
	Peer       Peer
	SenderID   string
	SenderName string
	SenderRole string
	Kind       string
	Text       string
	Doc        *format.Doc
	Media      []Attachment
	At         time.Time
}

// Status 是 Bot 运行状态的一次快照。
type Status struct {
	State  string
	Reason string
	Since  time.Time
}

const (
	StateStarting = "starting"
	StateRunning  = "running"
	StateStopped  = "stopped"
	StateError    = "error"
)

// 能力名，与协议里的 caps 保持一致；适配器按实际支持情况声明。
const (
	CapText      = "text"
	CapPhoto     = "photo"
	CapVideo     = "video"
	CapAudio     = "audio"
	CapVoice     = "voice"
	CapDocument  = "document"
	CapVideoNote = "video_note"
	CapSticker   = "sticker"
	CapAnimation = "animation"
	CapLocation  = "location"
	CapContact   = "contact"
)

// MessageID 是平台侧消息标识。
type MessageID string

// SendRequest 是一条待发送的消息。
//
// InReplyTo 是触发本次回复的平台入站消息 ID（QQ 的被动回复必须带），
// 为空表示主动消息，平台是否允许由适配器决定。
type SendRequest struct {
	Peer      Peer
	Text      string
	InReplyTo string
}

// Hooks 是适配器向核心回调的入口。
//
// OnMessage 返回错误时，适配器应记录日志并继续运行，而不是退出；
// 核心不保证回调线程安全之外的任何时序。
type Hooks struct {
	OnMessage func(ctx context.Context, msg Inbound) error
	OnStatus  func(status Status)
}

// Adapter 是一个平台适配器。
//
// 同一进程内每种平台只注册一个实例，因此实现必须无状态：
// 所有每次调用需要的东西都从 Account / Credentials 传入。
type Adapter interface {
	// Name 返回平台名，必须匹配 ^[a-z][a-z0-9_]{1,15}$。
	Name() string

	// Validate 校验凭据并返回账号信息；凭据非法时返回错误。
	Validate(ctx context.Context, creds Credentials) (Account, error)

	// Run 开始接收消息，直到 ctx 取消或发生不可恢复的错误。
	Run(ctx context.Context, acc Account, creds Credentials, hooks Hooks) error

	// SendText 发送一条已经按 Format() 渲染好的文本。
	SendText(ctx context.Context, acc Account, creds Credentials, req SendRequest) (MessageID, error)

	// Format 返回该平台使用的格式适配器名（见 internal/format）。
	Format() string
}

// MediaResolver 是可选能力：把平台侧媒体换成可下载地址。
// 核心用类型断言探测，没实现就只保留文字说明。
type MediaResolver interface {
	ResolveMedia(ctx context.Context, acc Account, creds Credentials, msg Inbound) (url string, err error)
}

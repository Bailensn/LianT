package platform

import (
	"errors"
	"sort"
	"sync"
)

// ============================================================================
// 平台兼容层
//
// LianT 的网页后台 / 数据库 / 指令引擎不直接依赖 Telegram。平台实现只需要
// 提供身份校验与能力声明，管理端根据 Capabilities 决定显示哪些功能。
//
// 当前实现：telegram。以后接 qbot（QQ）或其他平台时，新增一个 adapter 并
// 在 init 里 Register 即可；注册表 bots.platform 选择具体适配器。
// ============================================================================

// Capabilities 描述一个平台实际能做到什么。管理端据此隐藏/禁用按钮，
// 而不是假设所有平台都有 Telegram 的能力。
type Capabilities struct {
	Text          bool `json:"text"`
	Media         bool `json:"media"`
	Buttons       bool `json:"buttons"`
	Inline        bool `json:"inline"`
	Topics        bool `json:"topics"`
	InviteLinks   bool `json:"invite_links"`
	Webhook       bool `json:"webhook"`
	JoinApproval  bool `json:"join_approval"`
	MemberList    bool `json:"member_list"`
	Roles         bool `json:"roles"`
	Payments      bool `json:"payments"`
	Recall48h     bool `json:"recall_48h"`
	MaxMessageLen int  `json:"max_message_len"`
}

// Identity 是校验 token 后拿到的平台账号信息。
type Identity struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// Adapter 是平台必须实现的兼容契约。
// 事件循环/消息发送仍可由各平台自己的 runtime 实现，但注册、校验、能力声明
// 必须走这一层，保证同一套 Web 管理端可用。
type Adapter interface {
	ID() string
	Name() string
	Capabilities() Capabilities
	ValidateToken(token string) (Identity, error)
}

var (
	mu       sync.RWMutex
	adapters = map[string]Adapter{}
)

func Register(a Adapter) {
	if a == nil || a.ID() == "" {
		return
	}
	mu.Lock()
	adapters[a.ID()] = a
	mu.Unlock()
}

func Get(id string) Adapter {
	mu.RLock()
	defer mu.RUnlock()
	return adapters[id]
}

func List() []Adapter {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Adapter, 0, len(adapters))
	for _, a := range adapters {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

func ErrUnsupported(id string) error {
	return errors.New("不支持的平台: " + id)
}

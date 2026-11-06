package format

import (
	"errors"
	"sort"
	"sync"
)

// ErrNotFound 表示请求的格式没有注册。
var ErrNotFound = errors.New("format: 未注册的格式")

// Entity 是平台侧给出的行内标记（例如 Telegram 的 entities）。
// Offset / Length 一律以 Unicode 码点计，由平台适配器负责从
// UTF-16 code unit 转换过来——中文与 emoji 的错位大多出在这一步。
type Entity struct {
	Type   string
	Offset int
	Length int
	URL    string
}

// 已知的实体类型，与 NDoc 的 span 类型一一对应；pre 归入 code。
const (
	EntityBold   = "bold"
	EntityItalic = "italic"
	EntityStrike = "strike"
	EntityCode   = "code"
	EntityPre    = "pre"
	EntityLink   = "link"
)

// Format 是一个消息格式适配器：负责 NDoc 与平台标记之间的双向转换。
//
// 实现必须满足：
//   - Render 是纯函数，同样的 Doc 得到同样的字符串；
//   - Parse 失败时返回错误，由调用方降级为纯文本；
//   - 渲染时先转义文本、再拼标记。
type Format interface {
	Name() string
	Render(doc *Doc) (string, error)
	Parse(text string, ents []Entity) (*Doc, error)
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Format)
)

// Register 注册一个格式实现。重复注册或空名会 panic——
// 这类错误只应出现在启动期，而不是运行期。
func Register(f Format) {
	if f == nil {
		panic("format: 注册了 nil 格式")
	}
	name := f.Name()
	if name == "" {
		panic("format: 格式名为空")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		panic("format: 重复注册 " + name)
	}
	registry[name] = f
}

// Get 按名字取格式实现。
func Get(name string) (Format, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[name]
	return f, ok
}

// Names 返回已注册的格式名（已排序），用于启动日志与诊断。
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

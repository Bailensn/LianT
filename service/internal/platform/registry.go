package platform

import (
	"errors"
	"sort"
	"sync"
)

// ErrNotFound 表示请求的平台没有注册。
var ErrNotFound = errors.New("platform: 未注册的适配器")

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Adapter)
)

// Register 注册一个适配器。空名或重复注册会 panic——
// 这类错误只应出现在启动期，而不是运行期。
func Register(a Adapter) {
	if a == nil {
		panic("platform: 注册了 nil 适配器")
	}
	name := a.Name()
	if !platformPattern.MatchString(name) {
		panic("platform: 非法平台名 " + name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		panic("platform: 重复注册 " + name)
	}
	registry[name] = a
}

// Get 按平台名取适配器。
func Get(name string) (Adapter, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	a, ok := registry[name]
	return a, ok
}

// Names 返回已注册的平台名（已排序），用于启动日志与诊断。
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

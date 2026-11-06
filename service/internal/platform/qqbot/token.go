package qqbot

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	errMissingCredentials = errors.New("qqbot: 需要 app_id 与 app_secret")
	errEmptyToken         = errors.New("qqbot: 平台返回了空 token")
)

type tokenEntry struct {
	token   string
	expires time.Time
}

type tokenCache struct {
	mu sync.Mutex
	m  map[string]*tokenEntry
}

func (c *tokenCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || e.token == "" {
		return "", false
	}
	// 提前 60 秒视为过期，避免请求途中失效。
	if time.Now().After(e.expires.Add(-60 * time.Second)) {
		return "", false
	}
	return e.token, true
}

func (c *tokenCache) put(key, token string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]*tokenEntry)
	}
	c.m[key] = &tokenEntry{token: token, expires: time.Now().Add(ttl)}
}

// tokenKey 区分沙箱与正式环境，避免串用。
func (a *Adapter) tokenKey(c credentials) string {
	env := "prod"
	if c.sandbox {
		env = "sandbox"
	}
	return fmt.Sprintf("%s/%s", env, c.appID)
}

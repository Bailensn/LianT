package main

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// basicAuth 是公网暴露时的第一道门。
//
// 它挡在 WSS 与媒体下载之前：浏览器会弹出账号密码框，认证通过后
// 同源的 WebSocket 握手会自动带上同一份凭据。
func basicAuth(next http.Handler, user, pass string, onFail func(ip string)) http.Handler {
	if user == "" || pass == "" {
		return next
	}
	lim := newFailLimiter(10, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if lim.blocked(ip) {
			http.Error(w, "尝试次数过多，请稍后再试", http.StatusTooManyRequests)
			return
		}
		u, p, ok := r.BasicAuth()
		// 用户名与密码都做定长比较，避免按位爆破。
		userOK := subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
		if !ok || !userOK || !passOK {
			lim.fail(ip)
			if onFail != nil {
				onFail(ip)
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="LianT", charset="UTF-8"`)
			http.Error(w, "需要认证", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i > 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

// failLimiter 是极简的按 IP 失败计数，窗口过期自动清零。
type failLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*failEntry
}

type failEntry struct {
	count int
	since time.Time
}

func newFailLimiter(limit int, window time.Duration) *failLimiter {
	return &failLimiter{limit: limit, window: window, entries: make(map[string]*failEntry)}
}

func (l *failLimiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[ip]
	if e == nil {
		return false
	}
	if time.Since(e.since) > l.window {
		delete(l.entries, ip)
		return false
	}
	return e.count >= l.limit
}

func (l *failLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[ip]
	if e == nil || time.Since(e.since) > l.window {
		l.entries[ip] = &failEntry{count: 1, since: time.Now()}
		return
	}
	e.count++
	if len(l.entries) > 1024 {
		for k, v := range l.entries {
			if time.Since(v.since) > l.window {
				delete(l.entries, k)
			}
		}
	}
	slog.Debug("基本认证失败", "ip", ip, "count", e.count)
}

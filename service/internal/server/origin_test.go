package server

import (
	"net/http/httptest"
	"testing"
)

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"无 Origin 的命令行客户端", "", "liant.example.com:8443", true},
		{"同源页面", "https://liant.example.com:8443", "liant.example.com:8443", true},
		{"同源但大小写不同", "https://LIANT.example.com:8443", "liant.example.com:8443", true},
		{"别的站点", "https://evil.example.com", "liant.example.com:8443", false},
		{"端口不同", "https://liant.example.com:443", "liant.example.com:8443", false},
		{"origin 不是 URL", "null", "liant.example.com:8443", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+c.host+"/v1/ws", nil)
			r.Host = c.host
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			if got := sameOrigin(r); got != c.want {
				t.Fatalf("sameOrigin(%q, host=%q) = %v, want %v", c.origin, c.host, got, c.want)
			}
		})
	}
}

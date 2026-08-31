package service

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	"github.com/LensnTeam/LianT/service/config"
	"github.com/LensnTeam/LianT/service/wss"
)

// proxyTransport 是 daemon 反向代理下载到 bot 本机 WSS 所用的 HTTP 客户端。
// bot 每个进程各自生成自签名证书，因此对后端跳过证书校验。
var proxyTransport = &http.Transport{
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
}

// startWSSProxy 启动 daemon 的单一安全入口：
// 在 [50000,60000] 随机挑一个可用端口，对外按 /<id>/ws、/<id>/dl 等路由到对应 bot 的本机 WSS。
// 返回公开基地址（https://host:port），并据此给每个 bot 注入 /<id>/ 前缀。
func startWSSProxy() (string, error) {
	cert, err := wss.GenerateCert()
	if err != nil {
		return "", err
	}
	listener, port, err := listenDaemonPort()
	if err != nil {
		return "", err
	}
	srv := &http.Server{
		Handler: &wssProxyHandler{},
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}
	go func() {
		_ = srv.ServeTLS(listener, "", "")
	}()
	return fmt.Sprintf(
		"https://%s:%d",
		advertisedHost(),
		port,
	), nil
}

// advertisedHost 决定对外通告的公网主机：优先配置 connect.address，否则自动探测。
func advertisedHost() string {
	cfg := config.LoadConfig()
	if cfg.Connect.Address != "" {
		return cfg.Connect.Address
	}
	return wss.ResolveAddress()
}

// listenDaemonPort 在 [50000,60000] 范围内随机挑选一个可用端口并监听（所有接口）。
func listenDaemonPort() (net.Listener, int, error) {
	for i := 0; i < 30; i++ {
		b := make([]byte, 2)
		_, _ = rand.Read(b)
		port := 50000 + int(b[0])*39 + int(b[1])%39
		l, err := net.Listen(
			"tcp",
			fmt.Sprintf(":%d", port),
		)
		if err == nil {
			return l, port, nil
		}
	}
	return nil, 0, fmt.Errorf("无法在 50000-60000 范围内分配可用端口")
}

// wssProxyHandler 按路径首段（bot id）把请求反向代理到对应 bot 的本机 WSS。
type wssProxyHandler struct{}

func (h *wssProxyHandler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	parts := strings.SplitN(
		strings.Trim(r.URL.Path, "/"),
		"/",
		2,
	)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	botsMu.RLock()
	bot := bots[id]
	botsMu.RUnlock()
	if bot == nil || bot.WSSAddr == "" {
		http.NotFound(w, r)
		return
	}
	backend := bot.WSSAddr
	rest := "/" + parts[1]

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "https"
			req.URL.Host = backend
			req.URL.Path = rest
			// 保留原始 query（含 token）
		},
		Transport: proxyTransport,
	}
	proxy.ServeHTTP(w, r)
}

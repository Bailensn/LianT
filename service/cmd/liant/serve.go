package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Bailensn/LianT/service/internal/botrun"
	"github.com/Bailensn/LianT/service/internal/config"
	"github.com/Bailensn/LianT/service/internal/core"
	"github.com/Bailensn/LianT/service/internal/node"
	"github.com/Bailensn/LianT/service/internal/pairing"
	"github.com/Bailensn/LianT/service/internal/platform"
	_ "github.com/Bailensn/LianT/service/internal/platform/qqbot"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
	"github.com/skip2/go-qrcode"
)

// nodeDir 放节点私钥与证书；与主密钥分开，轮换节点密钥不应影响数据解密。
const nodeDir = "data/node"

// retention 是 v1 的保留策略声明；实际清理由 store.Purge 执行。
func retention() protocol.Retention {
	r := store.DefaultRetention()
	return protocol.Retention{
		Days:             int(r.Text.Hours() / 24),
		MediaMaxBytes:    r.MediaBytes,
		CleanupIntervalS: 3600,
	}
}

// deviceFactory 允许调用方在 store 之外包一层（例如配对完成通知）。
type deviceFactory func(*store.Store) server.DeviceStore

// buildServer 装配存储、节点身份、配对器与 WSS 服务端。
func buildServer(
	confirm func(context.Context, string, string, *ecdsa.PublicKey) bool,
	pairer *pairing.Pairer,
	factory deviceFactory,
) (*server.Server, *node.Identity, *store.Store, *core.Core, error) {
	cfg := loadConfigOrDie()

	db, err := store.Open(cfg.Storage.Database)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	id, err := node.LoadOrCreate(nodeDir)
	if err != nil {
		db.Close()
		return nil, nil, nil, nil, err
	}
	if pairer == nil {
		pairer = pairing.New()
	}
	// 配对码写进数据库，这样另一个进程（例如 serve）也能兑付。
	pairer.SetStore(db)
	var devices server.DeviceStore = db
	if factory != nil {
		devices = factory(db)
	}
	user, pass := basicAuthConfig(cfg)
	srv, err := server.New(server.Options{
		Identity:  id,
		AutoPair:  user != "" && pass != "",
		Devices:   devices,
		Pairer:    pairer,
		Confirm:    confirm,
		AllowNames: cfg.Pair.Allow,
		Version:    version,
		Audit: func(ctx context.Context, ev server.AuditEvent) {
			if err := db.AddAudit(ctx, store.Audit{
				Kind:     ev.Kind,
				DeviceID: ev.DeviceID,
				Name:     ev.Name,
				IP:       ev.IP,
				Reason:   ev.Reason,
			}); err != nil {
				slog.Warn("写审计事件失败", "err", err)
			}
		},
		Retention: retention(),
		Logf: func(format string, args ...any) {
			slog.Warn(fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		db.Close()
		return nil, nil, nil, nil, err
	}
	coreSvc, err := core.New(core.Options{Store: db, Broadcaster: srv})
	if err != nil {
		db.Close()
		return nil, nil, nil, nil, err
	}
	coreSvc.Register(srv)
	return srv, id, db, coreSvc, nil
}

func loadConfigOrDie() config.Config {
	if _, err := os.Stat(config.ConfigPath()); err != nil {
		fmt.Fprintln(os.Stderr, "还没有配置，先运行：LianT init")
		os.Exit(1)
	}
	return config.LoadConfig()
}

func listenAddr(cfg config.Config) string {
	addr := cfg.Connect.Listen
	if addr == "" {
		addr = ":8443"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	return addr
}

// noTLSRequested 判断是否关闭内置 TLS。
//
// 只用于容器里由反向代理终止 TLS 的部署：代理到本进程走明文，
// 浏览器看到的仍是代理签发的正式证书。
func noTLSRequested(flagValue bool) bool {
	if flagValue {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LIANT_NO_TLS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// servePlain 起明文监听，仅用于反代场景。
func servePlain(srv *http.Server, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	slog.Warn("以明文模式监听，请确保前面有反向代理终止 TLS", "addr", addr)
	return srv.Serve(ln)
}

// serveTLS 用节点证书起 TLS 监听；协议要求 TLS 1.3 起。
func serveTLS(srv *http.Server, id *node.Identity, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", addr, err)
	}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{id.Certificate()},
		MinVersion:   tls.VersionTLS13,
	}
	slog.Info("WSS 监听", "addr", addr, "fingerprint", id.Fingerprint())
	return srv.Serve(tls.NewListener(ln, tlsConf))
}

// serveCommand 前台运行 WSS 服务端。
func serveCommand(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	daemonize := fs.Bool("d", false, "转入后台运行，日志写 data/liant.log，可用 LianT stop 停止")
	noTLS := fs.Bool("no-tls", false, "不启用内置 TLS，仅用于由反向代理终止 TLS 的部署；也可用环境变量 LIANT_NO_TLS=1")
	_ = fs.Parse(args)
	plain := noTLSRequested(*noTLS)

	if *daemonize && !isDaemonChild() {
		if err := startDaemon([]string{"serve"}); err != nil {
			fmt.Fprintln(os.Stderr, "转后台失败：", err)
			os.Exit(1)
		}
		return
	}
	if isDaemonChild() {
		_ = writePidFile(os.Getpid())
		defer removePidFile()
	}

	cfg := loadConfigOrDie()
	srv, id, db, coreSvc, err := buildServer(confirmPolicy(), nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败：", err)
		os.Exit(1)
	}
	defer db.Close()

	manager, err := startBotManager(srv, db, coreSvc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 Bot 管理器失败：", err)
		os.Exit(1)
	}
	defer manager.Shutdown()

	go purgeLoop(db)

	httpSrv := &http.Server{Handler: withBasicAuth(appHandler(srv, coreSvc), db, cfg), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		var err error
		if plain {
			err = servePlain(httpSrv, listenAddr(cfg))
		} else {
			err = serveTLS(httpSrv, id, listenAddr(cfg))
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "服务退出：", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("收到退出信号，正在关闭")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

// startBotManager 构造并认领 Bot；serve 与 pair 共用。
func startBotManager(srv *server.Server, db *store.Store, coreSvc *core.Core) (*botrun.Manager, error) {
	manager, err := botrun.New(botrun.Options{
		Store:  db,
		Ingest: coreSvc,
		OnStatus: func(botID string, st platform.Status) {
			srv.Broadcast(protocol.TypeEventBot, protocol.EventBot{
				BotID:  botID,
				Status: st.State,
				Reason: st.Reason,
			})
		},
	})
	if err != nil {
		return nil, err
	}
	coreSvc.SetController(manager)
	if started, err := manager.Reconcile(context.Background()); err != nil {
		slog.Warn("认领已有 Bot 失败", "err", err)
	} else {
		slog.Info("已认领 Bot", "count", started)
	}
	return manager, nil
}

// pairCommand 打一次配对码；配对成功后继续以服务模式运行。
func pairCommand(args []string) {
	cfg := loadConfigOrDie()

	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	hostFlag := fs.String("host", "", "配对信息里通告的主机名或 IP；留空则用 connect.address、公网 IP 或本机地址")
	portFlag := fs.Int("port", 0, "配对信息里通告的端口；留空用 listen 端口（反代到 443 时填 443）")
	_ = fs.Parse(args)

	pairer := pairing.New()
	watcher := &pairWatcher{done: make(chan struct{})}
	srv, id, db, coreSvc, err := buildServer(confirmPolicy(), pairer, func(db *store.Store) server.DeviceStore {
		watcher.Store = db
		return watcher
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败：", err)
		os.Exit(1)
	}
	defer db.Close()

	// 已经有服务在跑时，配对码交给它兑付，本进程只负责生成和等待。
	delegated := os.Getenv("LIANT_PAIR_DELEGATE") == "1"
	if !delegated {
		if ln, err := net.Listen("tcp", listenAddr(cfg)); err != nil {
			if errors.Is(err, syscall.EADDRINUSE) {
				delegated = true
			} else {
				fmt.Fprintln(os.Stderr, "检测端口失败：", err)
				os.Exit(1)
			}
		} else {
			_ = ln.Close()
		}
	}

	code, err := pairer.Issue()
	if err != nil {
		fmt.Fprintln(os.Stderr, "生成配对码失败：", err)
		os.Exit(1)
	}

	host := pairHost(*hostFlag, cfg)
	if host == "" {
		fmt.Fprintln(os.Stderr, "无法自动确定对外地址，请用：LianT pair --host <手机可达的 IP 或域名>")
		os.Exit(1)
	}
	if cfg.Connect.Address == "" && *hostFlag == "" {
		fmt.Printf("自动检测到对外地址：%s，手机需能访问它；不对请用 --host 覆盖\n", host)
	}
	if isLoopbackHost(host) {
		fmt.Fprintf(os.Stderr, "警告：%s 是回环地址，只有 Service 本机能访问；手机请改用 --host <局域网或公网地址>\n", host)
	} else if isPrivateHost(host) {
		fmt.Fprintf(os.Stderr, "警告：%s 是内网地址，外网手机不可达；跨网请用 --host <公网 IP 或域名>\n", host)
	}

	port := portOf(listenAddr(cfg))
	if *portFlag > 0 {
		if *portFlag > 65535 {
			fmt.Fprintln(os.Stderr, "端口超出范围")
			os.Exit(1)
		}
		port = *portFlag
	}
	fmt.Printf("配对地址：%s:%d\n", host, port)
	if port != portOf(listenAddr(cfg)) {
		fmt.Printf("配对端口使用 %d（监听端口是 %d，确认反代已转发 /v1/ws）\n", port, portOf(listenAddr(cfg)))
	}

	payload, _ := json.Marshal(map[string]any{
		"v":     protocol.Version,
		"host":  host,
		"port":  port,
		"fp":    id.Fingerprint(),
		"ticket": code.Value,
		"nonce": code.Nonce,
		"exp":   code.Expires.Unix(),
	})
	// 二维码直接编码配对信息本身：客户端扫码或粘贴都能解析，不依赖某个网页。
	fmt.Printf("配对码：%s，%d 秒内有效\n", code.Value, int(time.Until(code.Expires).Seconds()))
	fmt.Printf("节点指纹：%s\n", id.Fingerprint())
	fmt.Printf("配对信息：%s\n", payload)
	if qr, err := qrcode.New(string(payload), qrcode.Medium); err == nil {
		fmt.Println(qr.ToSmallString(false))
		if err := os.MkdirAll("data", 0o700); err == nil {
			if err := qr.WriteFile(512, "data/pair-qr.png"); err == nil {
				fmt.Println("二维码图片：data/pair-qr.png")
			}
		}
	}
	fmt.Println("手机打开 LianT 客户端「连接 LianT」页，粘贴上面的配对信息（或扫码）；本终端还会再确认一次。")
	fmt.Println("等待 Manager 连接… 如果长时间没有提示，先确认手机能访问上面的配对地址。")

	if delegated {
		fmt.Println("检测到服务已在运行，这条配对码交给它处理。")
		fmt.Println("配对确认会出现在 serve 的终端；systemd 下通常是自动批准，设备登记后这里会打印成功。")
		before, _ := db.CountDevices(context.Background())
		deadline := time.Now().Add(time.Until(code.Expires) + 2*time.Minute)
		for time.Now().Before(deadline) {
			time.Sleep(500 * time.Millisecond)
			if n, err := db.CountDevices(context.Background()); err == nil && n > before {
				fmt.Println("配对成功，设备已登记。")
				return
			}
		}
		fmt.Fprintln(os.Stderr, "等待配对超时。")
		os.Exit(1)
	}

	httpSrv := &http.Server{Handler: withBasicAuth(appHandler(srv, coreSvc), db, cfg), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		var err error
		if noTLSRequested(false) {
			err = servePlain(httpSrv, listenAddr(cfg))
		} else {
			err = serveTLS(httpSrv, id, listenAddr(cfg))
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "服务退出：", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-watcher.done:
		fmt.Println("配对成功，设备已登记；继续运行服务，Ctrl+C 退出。")
	case <-time.After(time.Until(code.Expires) + 2*time.Minute):
		fmt.Fprintln(os.Stderr, "等待配对超时。")
		os.Exit(1)
	case <-stop:
		fmt.Fprintln(os.Stderr, "已取消。")
		os.Exit(1)
	}
	// 配对完成后不要退出：Manager 会立刻换一条新连接认证，服务得还在。
	manager, err := startBotManager(srv, db, coreSvc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 Bot 管理器失败：", err)
		os.Exit(1)
	}
	defer manager.Shutdown()
	go purgeLoop(db)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

// mustOpenStore 打开配置里的数据库；CLI 命令共用。
// pairHost 决定二维码里写什么主机名：命令行 > 配置 > 本机非回环 IPv4 > 主机名。
func pairHost(flagHost string, cfg config.Config) string {
	if h := strings.TrimSpace(flagHost); h != "" {
		return h
	}
	if h := addressHost(cfg.Connect.Address); h != "" {
		return h
	}
	local := localIPv4()
	// 本机直接有公网地址就不绕 HTTP 探测，避免没网时白等。
	if local != "" && isPublicIP(local) {
		return local
	}
	if ip := publicIPv4(); ip != "" {
		return ip
	}
	if local != "" {
		return local
	}
	// 主机名可能是 localhost，解析出来就是回环，宁可报错让用户显式给 --host。
	if name, err := os.Hostname(); err == nil && !isLoopbackHost(name) {
		return name
	}
	return ""
}

// publicIPv4 依次问几个公共回显服务，拿不到就返回空。
// 只接受公网 IPv4：回环、私网、链路本地、IPv6 一律忽略。
func publicIPv4() string {
	client := &http.Client{Timeout: 4 * time.Second}
	for _, endpoint := range []string{
		"https://api.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://ifconfig.me/ip",
	} {
		resp, err := client.Get(endpoint)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		ip := net.ParseIP(strings.TrimSpace(string(body)))
		if ip == nil || ip.To4() == nil {
			continue
		}
		if isPublicIP(ip.String()) {
			return ip.String()
		}
	}
	return ""
}

func isPublicIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// addressHost 兼容 https://host:port、host:port 与裸 host 三种写法。
func addressHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "://") {
		if u, err := url.Parse(addr); err == nil {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// localIPv4 返回第一个私有网段的非回环 IPv4，供同一局域网内扫码使用。
//
// 先枚举网卡；Android/Termux 上枚举经常只剩 lo，再用 UDP Dial 探出口地址
// （Dial 只做路由查询，不会真的发包）。
func localIPv4() string {
	if ip := interfaceIPv4(); ip != "" {
		return ip
	}
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil {
		if ip := addr.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	return ""
}

func interfaceIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	var fallback string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip.IsPrivate() {
			return ip.String()
		}
		if fallback == "" {
			fallback = ip.String()
		}
	}
	return fallback
}

// basicAuthConfig 取账号密码：环境变量优先，其次配置文件。
func basicAuthConfig(cfg config.Config) (string, string) {
	if v := strings.TrimSpace(os.Getenv("LIANT_BASIC_AUTH")); v != "" {
		if user, pass, ok := strings.Cut(v, ":"); ok {
			return strings.TrimSpace(user), pass
		}
	}
	return cfg.Auth.BasicUser, cfg.Auth.BasicPass
}

// withBasicAuth 在设置了账号密码时挂上 HTTP Basic 认证。
func withBasicAuth(h http.Handler, db *store.Store, cfg config.Config) http.Handler {
	user, pass := basicAuthConfig(cfg)
	if user == "" && pass == "" {
		return h
	}
	if user == "" || pass == "" {
		slog.Error("基本认证只配了一半，已忽略", "user_set", user != "", "pass_set", pass != "")
		return h
	}
	slog.Info("已启用 HTTP 基本认证", "user", user)
	return basicAuth(h, user, pass, func(ip string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.AddAudit(ctx, store.Audit{Kind: "basic.fail", IP: ip}); err != nil {
			slog.Warn("记录认证失败失败", "err", err)
		}
	})
}

// appHandler 同时挂 WSS 与媒体下载；两者共用同一个 TLS 端口。
func appHandler(srv *server.Server, svc *core.Core) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/ws", srv.Handler())
	mux.Handle("/v1/media/", svc.MediaHandler())
	// 服务本身不再内置网页管理端。根路径只回一行说明，
	// 也让浏览器可以在这一层信任自签证书（之后 WebSocket 才能握手）。
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("LianT Service\n\n客户端请连接 /v1/ws（WebSocket），协议见 docs/PROTOCOL.md。\n"))
	})
	return securityHeaders(mux)
}

// securityHeaders 给所有响应加一层浏览器侧防护。
//
// 服务端不再提供网页，但媒体下载仍可能在浏览器里打开，所以保留这些头。
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; media-src 'self'; connect-src 'self'; " +
		"object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		if r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// purgeLoop 每小时清一次过期消息与媒体，并把文件真正删掉。
func purgeLoop(db *store.Store) {
	purge := func() {
		res, err := db.Purge(context.Background(), store.DefaultRetention(), time.Now())
		if err != nil {
			slog.Warn("清理失败", "err", err)
			return
		}
		for _, path := range res.Files {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				slog.Warn("删除媒体文件失败", "path", path, "err", err)
			}
		}
		if err := db.PruneAudit(context.Background(), 2000); err != nil {
			slog.Warn("清理审计事件失败", "err", err)
		}
		if res.Messages > 0 || res.MediaExpired > 0 || res.MediaEvicted > 0 {
			slog.Info("清理完成", "messages", res.Messages, "media_expired", res.MediaExpired, "media_evicted", res.MediaEvicted)
		}
	}
	purge()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		purge()
	}
}

func mustOpenStore() *store.Store {
	cfg := loadConfigOrDie()
	db, err := store.Open(cfg.Storage.Database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开数据库失败：", err)
		os.Exit(1)
	}
	return db
}

func portOf(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 8443
	}
	var p int
	_, _ = fmt.Sscanf(port, "%d", &p)
	return p
}

// pairWatcher 在设备真正落库后才通知配对完成。
type pairWatcher struct {
	*store.Store
	done chan struct{}
	once sync.Once
}

func (p *pairWatcher) PutDevice(ctx context.Context, d store.Device) error {
	if err := p.Store.PutDevice(ctx, d); err != nil {
		return err
	}
	p.once.Do(func() { close(p.done) })
	return nil
}

// confirmTerminal 在终端显示设备名与指纹，要求本地确认。
// isInteractive 判断 stdin 是不是终端。
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	// /dev/null 也是字符设备，但它连不上任何终端。
	return fi.Name() != "null"
}

// confirmPolicy 决定配对确认方式：LIANT_CONFIRM=auto|terminal 显式指定，
// 否则看 stdin 是不是终端。
func confirmPolicy() func(context.Context, string, string, *ecdsa.PublicKey) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LIANT_CONFIRM"))) {
	case "auto", "yes":
		return confirmAuto
	case "terminal", "tty":
		return confirmTerminal
	}
	if isInteractive() {
		return confirmTerminal
	}
	return confirmAuto
}

// confirmAuto 用于无终端场景（systemd/后台运行）。
//
// 配对码只可能由 `LianT pair` 生成并显示在终端，能拿着它来配对本身就意味着
// 操作者已经确认；这里放行并留审计记录。
func confirmAuto(_ context.Context, name, fingerprint string, _ *ecdsa.PublicKey) bool {
	slog.Warn("无终端，自动批准配对", "name", name, "fingerprint", fingerprint)
	return true
}

func confirmTerminal(_ context.Context, name, fingerprint string, _ *ecdsa.PublicKey) bool {
	fmt.Printf("\n配对请求：%s\n设备指纹：%s\n", name, fingerprint)
	if len(fingerprint) >= 9 {
		fmt.Printf("指纹前 8 位：%s\n", fingerprint[:9])
	}
	fmt.Print("确认授权这台设备？[y/N] ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

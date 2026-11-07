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
	"log"
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
	var devices server.DeviceStore = db
	if factory != nil {
		devices = factory(db)
	}
	srv, err := server.New(server.Options{
		Identity:  id,
		Devices:   devices,
		Pairer:    pairer,
		Confirm:   confirm,
		Retention: retention(),
		Logf:      log.Printf,
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
	log.Printf("WSS 监听 %s，指纹 %s", addr, id.Fingerprint())
	return srv.Serve(tls.NewListener(ln, tlsConf))
}

// serveCommand 前台运行 WSS 服务端。
func serveCommand(_ []string) {
	cfg := loadConfigOrDie()
	srv, id, db, coreSvc, err := buildServer(confirmTerminal, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败：", err)
		os.Exit(1)
	}
	defer db.Close()

	manager, err := botrun.New(botrun.Options{
		Store:  db,
		Ingest: coreSvc,
		OnStatus: func(botID string, st platform.Status) {
			reason := st.Reason
			srv.Broadcast(protocol.TypeEventBot, protocol.EventBot{
				BotID:  botID,
				Status: st.State,
				Reason: reason,
			})
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化 Bot 管理器失败：", err)
		os.Exit(1)
	}
	coreSvc.SetController(manager)
	if started, err := manager.Reconcile(context.Background()); err != nil {
		log.Printf("认领已有 Bot 失败: %v", err)
	} else {
		log.Printf("已认领 %d 个 Bot", started)
	}
	defer manager.Shutdown()

	go purgeLoop(db)

	httpSrv := &http.Server{Handler: appHandler(srv, coreSvc), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := serveTLS(httpSrv, id, listenAddr(cfg)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "服务退出：", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("收到退出信号，正在关闭…")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

// pairCommand 打一次配对码，等手机配对成功后退出。
func pairCommand(args []string) {
	cfg := loadConfigOrDie()

	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	hostFlag := fs.String("host", "", "二维码里通告的主机名或 IP；留空则用 connect.address 或自动探测")
	_ = fs.Parse(args)

	pairer := pairing.New()
	code, err := pairer.Issue()
	if err != nil {
		fmt.Fprintln(os.Stderr, "生成配对码失败：", err)
		os.Exit(1)
	}

	watcher := &pairWatcher{done: make(chan struct{})}
	srv, id, db, _, err := buildServer(confirmTerminal, pairer, func(db *store.Store) server.DeviceStore {
		watcher.Store = db
		return watcher
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动失败：", err)
		os.Exit(1)
	}
	defer db.Close()

	host := pairHost(*hostFlag, cfg)
	if host == "" {
		fmt.Fprintln(os.Stderr, "无法自动确定对外地址，请用：LianT pair --host <手机可达的 IP 或域名>")
		os.Exit(1)
	}
	if cfg.Connect.Address == "" && *hostFlag == "" {
		fmt.Printf("自动检测到本机地址：%s（手机需能访问它；跨网请用 --host）\n", host)
	}

	payload, _ := json.Marshal(map[string]any{
		"v":     protocol.Version,
		"host":  host,
		"port":  portOf(listenAddr(cfg)),
		"fp":    id.Fingerprint(),
		"ticket": code.Value,
		"nonce": code.Nonce,
		"exp":   code.Expires.Unix(),
	})
	fmt.Printf("配对码：%s（%d 秒内有效）\n", code.Value, int(time.Until(code.Expires).Seconds()))
	fmt.Printf("节点指纹：%s\n", id.Fingerprint())
	fmt.Printf("二维码内容：%s\n", payload)
	fmt.Println("用 Manager 扫码，或手工输入配对码；本终端会再确认一次。")

	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := serveTLS(httpSrv, id, listenAddr(cfg)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "服务退出：", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-watcher.done:
		fmt.Println("配对成功，设备已登记。")
	case <-time.After(time.Until(code.Expires) + 2*time.Minute):
		fmt.Fprintln(os.Stderr, "等待配对超时。")
		os.Exit(1)
	case <-stop:
		fmt.Fprintln(os.Stderr, "已取消。")
		os.Exit(1)
	}
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
	if ip := localIPv4(); ip != "" {
		return ip
	}
	if name, err := os.Hostname(); err == nil {
		return name
	}
	return ""
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
func localIPv4() string {
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

// appHandler 同时挂 WSS 与媒体下载；两者共用同一个 TLS 端口。
func appHandler(srv *server.Server, svc *core.Core) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/ws", srv.Handler())
	mux.Handle("/v1/media/", svc.MediaHandler())
	return mux
}

// purgeLoop 每小时清一次过期消息与媒体，并把文件真正删掉。
func purgeLoop(db *store.Store) {
	purge := func() {
		res, err := db.Purge(context.Background(), store.DefaultRetention(), time.Now())
		if err != nil {
			log.Printf("清理失败: %v", err)
			return
		}
		for _, path := range res.Files {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("删除媒体文件失败 %s: %v", path, err)
			}
		}
		if res.Messages > 0 || res.MediaExpired > 0 || res.MediaEvicted > 0 {
			log.Printf("清理完成：消息 %d，过期媒体 %d，淘汰媒体 %d", res.Messages, res.MediaExpired, res.MediaEvicted)
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

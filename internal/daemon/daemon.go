package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"LianT/internal/admin"
	"LianT/internal/cfg"
	"LianT/internal/ipc"
	"LianT/internal/logfile"
	"LianT/internal/manager"
	"LianT/internal/proc"
	"LianT/internal/store"
)

// ============================================================================
// 任务进程（daemon / task process）
//
//   * 一个任务进程同时跑多个 Bot（注册表在 SQLite bots 表）
//   * 每个 Bot 独立业务库：<数据目录>/bots/<id>.db
//   * 全局唯一：靠"只有它能绑定控制端口"实现
//   * 与主进程解耦：spawn 时 setsid 独立会话 + 忽略 SIGHUP
// ============================================================================

var DaemonStartedAt = time.Now()

// Run 是任务进程主函数，返回进程退出码。
func Run(args []string) int {
	console := HasFlag(args, "console") || os.Getenv("LIANT_CONSOLE") == "1"

	w, err := logfile.Setup(cfg.LogPath, console)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 打开日志文件失败 %s: %v\n", cfg.LogPath, err)
		return 1
	}
	defer w.Close()

	DaemonStartedAt = time.Now()
	log.Printf("============ LianT 任务进程启动 pid=%d ============", os.Getpid())
	log.Printf("[i] 数据目录: %s", cfg.DataDir)
	log.Printf("[i] 日志文件: %s", cfg.LogPath)

	if !cfg.FileExists(cfg.EnvPath) && cfg.EnsureDotEnvTemplate(cfg.EnvPath) {
		log.Printf("[i] 已生成 .env 模板: %s（只放路径/端口；Bot token 在 SQLite 里）", cfg.EnvPath)
	}
	log.Printf("[i] 配置文件(.env): %s", cfg.EnvPath)
	log.Printf("[i] 注册表数据库: %s", cfg.DBPath)

	// ---- 注册表（bots 表 + admin token 等全局设置） ----
	reg, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Printf("[X] 打开注册表数据库失败: %v", err)
		return 1
	}
	defer reg.Close()

	// ---- 多 Bot 管理器：每个 Bot 独立业务库/连接 ----
	mgr := manager.New(reg)
	mgr.StartAll()

	// 旧的 .env 单 Bot 迁移：有 LIANT_BOT_TOKEN 且注册表为空时，自动导入一次。
	migrateLegacyToken(mgr, reg)

	// ---- 单例锁：控制端口 ----
	addr := cfg.ControlAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[!] 控制端口 %s 已被占用：已有一个任务进程在运行，本次启动放弃。", addr)
		log.Printf("[!] (%v)", err)
		mgr.StopAll()
		return 1
	}
	defer ln.Close()

	info := cfg.RuntimeInfo{
		Version: cfg.ProtocolVersion, PID: os.Getpid(), Port: cfg.ControlPort(),
		DataDir: cfg.DataDir, LogPath: cfg.LogPath,
		StartedAt: DaemonStartedAt.Format(time.RFC3339),
	}
	if exe, err := os.Executable(); err == nil {
		info.Executable = exe
	}
	if err := cfg.WriteRuntimeInfo(info); err != nil {
		log.Printf("[!] 写 runtime 文件失败: %v", err)
	}
	_ = os.WriteFile(cfg.PIDPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)

	proc.IgnoreHangup()
	log.Printf("[OK] 控制端口就绪: %s", addr)
	log.Printf("[i] Telegram API: %s | 代理: %s", cfg.APIBase(), maskProxy(cfg.Proxy()))
	if !cfg.IsLoopbackHost(cfg.ControlHost()) {
		log.Printf("[!] 警告：控制端口绑在 %s（非回环），同网段的人可以控制这些机器人；建议改回 LIANT_BIND=127.0.0.1", addr)
	}

	// ---- 网页管理端 ----
	CurrentAdmin = admin.Start(mgr, reg)

	// ---- 生命周期 ----
	var stopping int32
	done := make(chan struct{})
	var once sync.Once
	shutdown := func(reason string) {
		once.Do(func() {
			atomic.StoreInt32(&stopping, 1)
			log.Printf("[i] 开始退出任务进程(%s)…", reason)
			_ = ln.Close()
			if adm := CurrentAdmin; adm != nil {
				adm.Close()
			}
			mgr.StopAll()
			_ = os.Remove(cfg.RuntimePath)
			_ = os.Remove(cfg.PIDPath)
			log.Printf("[OK] 任务进程已退出")
			close(done)
		})
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		s := <-sig
		log.Printf("[i] 收到信号: %v", s)
		shutdown("signal")
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if atomic.LoadInt32(&stopping) == 1 {
				<-done
				return 0
			}
			log.Printf("[!] 接受控制连接失败: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		go HandleControlConn(conn, mgr, shutdown)
	}
}

// migrateLegacyToken 把旧版 .env 里的单 Bot token 导入注册表（只做一次）。
func migrateLegacyToken(mgr *manager.Manager, reg *store.Store) {
	if reg.Setting("legacy_env_imported", "") != "" || len(mustBots(reg)) > 0 {
		return
	}
	token := strings.TrimSpace(os.Getenv("LIANT_BOT_TOKEN"))
	if token == "" {
		if v, ok := cfg.EnvFile().Lookup("LIANT_BOT_TOKEN"); ok {
			token = strings.TrimSpace(v)
		}
	}
	if token == "" {
		return
	}
	id, err := mgr.Add(store.BotRec{Name: "从 .env 导入", Token: token, Enabled: true, Mode: "polling"})
	if err != nil {
		log.Printf("[!] 旧版 LIANT_BOT_TOKEN 导入失败: %v", err)
		return
	}
	_ = reg.SetSetting("legacy_env_imported", "1")
	log.Printf("[OK] 已把 .env 里的 LIANT_BOT_TOKEN 导入为 Bot #%d（之后请用网页后台管理）", id)
}

func mustBots(reg *store.Store) []store.BotRec {
	recs, _ := reg.Bots()
	return recs
}

// HandleControlConn 处理一条指令（一连接一指令）。
func HandleControlConn(conn net.Conn, mgr *manager.Manager, shutdown func(string)) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	var req ipc.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	resp, bye := DispatchControl(req, mgr)
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		log.Printf("[!] 回复指令失败: %v", err)
	}
	_ = conn.Close()
	if bye {
		shutdown("收到 stop 指令")
	}
}

func DispatchControl(req ipc.Request, mgr *manager.Manager) (ipc.Response, bool) {
	cmd := strings.ToLower(strings.TrimSpace(req.Cmd))
	views, _ := mgr.List()
	running := 0
	for _, v := range views {
		if v.Running {
			running++
		}
	}
	switch cmd {
	case "", "ping":
		return ipc.Response{OK: true, Msg: "pong", Data: map[string]string{
			"pid": strconv.Itoa(os.Getpid()), "port": strconv.Itoa(cfg.ControlPort()),
			"protocol": strconv.Itoa(cfg.ProtocolVersion), "app": cfg.AppVersion,
			"bots": strconv.Itoa(len(views)), "running": strconv.Itoa(running),
		}}, false

	case "status":
		data := map[string]string{
			"pid": strconv.Itoa(os.Getpid()), "port": strconv.Itoa(cfg.ControlPort()),
			"protocol": strconv.Itoa(cfg.ProtocolVersion), "app": cfg.AppVersion,
			"data_dir": cfg.DataDir, "log_path": cfg.LogPath, "env_file": cfg.EnvPath,
			"db_file": cfg.DBPath, "admin_addr": admin.ListenAddr(),
			"admin_prefix": admin.NormalizePrefix(cfg.AdminPrefix()),
			"started_at": DaemonStartedAt.Format("2006-01-02 15:04:05"),
			"bots": strconv.Itoa(len(views)), "running": strconv.Itoa(running),
		}
		if exe, err := os.Executable(); err == nil {
			data["exe"] = exe
		}
		if rt, ok := pickRuntime(mgr, req.Args["bot_id"]); ok {
			for k, v := range rt.Bot.Status() { data[k] = v }
			for k, v := range rt.Bot.ReplyInfo() { data[k] = v }
			data["bot_db"] = manager.BotDBPath(rt.Rec.ID)
			data["bot_id"] = strconv.FormatInt(rt.Rec.ID, 10)
		}
		return ipc.Response{OK: true, Msg: "运行中", Data: data}, false

	case "reload", "reconnect":
		mgr.Reconcile()
		mgr.ReloadAll()
		return ipc.Response{OK: true, Msg: fmt.Sprintf("已同步注册表并请求 %d 个 Bot 重连", running), Data: map[string]string{"bots": strconv.Itoa(len(views))}}, false

	case "test_reply":
		rt, ok := pickRuntime(mgr, req.Args["bot_id"])
		if !ok {
			return ipc.Response{OK: false, Msg: "没有运行中的 Bot"}, false
		}
		res := rt.Bot.TestReply(req.Args["text"], req.Args["scope"])
		data := map[string]string{"matched": res.Rule, "send": "true", "scope": req.Args["scope"]}
		if !res.Send {
			data["send"], data["reply"] = "false", ""
		} else {
			data["reply"] = res.Text
		}
		if res.Silent {
			data["silent"] = "true"
		}
		return ipc.Response{OK: true, Msg: "测试完成（不会真的发消息）", Data: data}, false

	case "stop":
		return ipc.Response{OK: true, Msg: "收到，任务进程正在停止…"}, true
	default:
		return ipc.Response{OK: false, Msg: "未知指令: " + req.Cmd}, false
	}
}

func pickRuntime(mgr *manager.Manager, rawID string) (*manager.Runtime, bool) {
	if id, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64); err == nil && id > 0 {
		return mgr.Get(id)
	}
	return mgr.First()
}

func HasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name || a == "-"+name || a == "--"+name {
			return true
		}
	}
	return false
}

// CurrentAdmin 当前管理端实例（退出时关闭用）。
var CurrentAdmin *admin.Server

func maskProxy(p string) string {
	if strings.TrimSpace(p) == "" {
		return "未启用"
	}
	return p
}

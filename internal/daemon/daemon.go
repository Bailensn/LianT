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
	"LianT/internal/bot"
	"LianT/internal/cfg"
	"LianT/internal/ipc"
	"LianT/internal/logfile"
	"LianT/internal/proc"
	"LianT/internal/rules"
	"LianT/internal/store"
)

// ============================================================================
// 任务进程（daemon / task process）
//
//   * 业务代码（Telegram 长轮询）只在这里运行；
//   * 全局唯一：靠"只有它能绑定控制端口"实现；
//   * 与主进程解耦：spawn 时 setsid 独立会话 + 忽略 SIGHUP；
//   * 输出全部进 .log 文件，不打屏（LianT run 才同时打屏）。
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
		log.Printf("[i] 已生成 .env 模板: %s（填好 Bot Token 即生效）", cfg.EnvPath)
	}
	log.Printf("[i] 凭据文件(.env): %s", cfg.EnvPath)
	log.Printf("[i] 数据库: %s", cfg.DBPath)
	if tok, src := cfg.TokenWithSource(); tok == "" {
		log.Printf("[!] %s", cfg.TokenError())
		log.Printf("[!] 任务进程照常运行：填好 .env 后最长 60 秒自动生效，也可以敲 LianT reload 立即生效")
	} else {
		log.Printf("[OK] Bot Token 来源: %s", src)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Printf("[X] 打开数据库失败: %v", err)
		return 1
	}
	defer st.Close()
	seedDefaultRules(st)

	// ---- 单例锁：控制端口 ----
	addr := cfg.ControlAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[!] 控制端口 %s 已被占用：已有一个任务进程在运行，本次启动放弃。", addr)
		log.Printf("[!] (%v)", err)
		return 1
	}
	defer ln.Close()

	info := cfg.RuntimeInfo{
		Version:   cfg.ProtocolVersion,
		PID:       os.Getpid(),
		Port:      cfg.ControlPort(),
		DataDir:   cfg.DataDir,
		LogPath:   cfg.LogPath,
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
		log.Printf("[!] 警告：控制端口绑在 %s（非回环），同网段的人可以控制这个机器人；建议改回 LIANT_BIND=127.0.0.1", addr)
	}

	// ---- 业务 ----
	b := bot.New(bot.Config{Store: st})
	b.Start()
	log.Printf("[OK] 业务协程已启动（长轮询、断线自动重连、规则改完即生效 %s）", b.DescribeCounts())

	go b.RefreshAllGroups(2 * time.Second)

	// ---- 网页管理端 ----
	CurrentAdmin = admin.Start(b, st)

	// ---- 生命周期 ----
	var stopping int32
	done := make(chan struct{})
	var once sync.Once
	shutdown := func(reason string) {
		once.Do(func() {
			atomic.StoreInt32(&stopping, 1)
			log.Printf("[i] 开始退出任务进程(%s)…", reason)
			_ = ln.Close()
			adm := CurrentAdmin
			adm.Close()
			b.Stop(10 * time.Second)
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
		go HandleControlConn(conn, b, shutdown)
	}
}

// HandleControlConn 处理一条指令（一连接一指令）。
func HandleControlConn(conn net.Conn, b *bot.Bot, shutdown func(string)) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	var req ipc.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	resp, bye := DispatchControl(req, b)
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		log.Printf("[!] 回复指令失败: %v", err)
	}
	_ = conn.Close()
	if bye {
		shutdown("收到 stop 指令")
	}
}

func DispatchControl(req ipc.Request, b *bot.Bot) (ipc.Response, bool) {
	cmd := strings.ToLower(strings.TrimSpace(req.Cmd))
	switch cmd {
	case "", "ping":
		return ipc.Response{
			OK:  true,
			Msg: "pong",
			Data: map[string]string{
				"pid":      strconv.Itoa(os.Getpid()),
				"port":     strconv.Itoa(cfg.ControlPort()),
				"protocol": strconv.Itoa(cfg.ProtocolVersion),
				"app":      cfg.AppVersion,
			},
		}, false

	case "status":
		data := b.Status()
		for k, v := range b.ReplyInfo() {
			data[k] = v
		}
		_, src := cfg.TokenWithSource()
		if src == "" {
			src = "未配置"
		}
		data["pid"] = strconv.Itoa(os.Getpid())
		data["port"] = strconv.Itoa(cfg.ControlPort())
		data["protocol"] = strconv.Itoa(cfg.ProtocolVersion)
		data["app"] = cfg.AppVersion
		data["data_dir"] = cfg.DataDir
		data["log_path"] = cfg.LogPath
		data["env_file"] = cfg.EnvPath
		data["db_file"] = cfg.DBPath
		data["admin_addr"] = admin.ListenAddr()
		data["admin_prefix"] = admin.NormalizePrefix(cfg.AdminPrefix())
		data["token_source"] = src
		data["started_at"] = DaemonStartedAt.Format("2006-01-02 15:04:05")
		if exe, err := os.Executable(); err == nil {
			data["exe"] = exe
		}
		return ipc.Response{OK: true, Msg: "运行中", Data: data}, false

	case "reconnect", "reload":
		err := b.ReloadAll()
		data := b.ReplyInfo()
		_, src := cfg.TokenWithSource()
		if src == "" {
			src = "未配置"
		}
		data["token_source"] = src
		data["env_file"] = cfg.EnvPath
		if err != nil {
			return ipc.Response{OK: false, Msg: "热重载失败: " + err.Error(), Data: data}, false
		}
		return ipc.Response{
			OK:   true,
			Msg:  fmt.Sprintf("热重载完成：规则 %s 条，Token 来源 %s；已请求重连 Telegram", data["reply_rules"], src),
			Data: data,
		}, false

	case "test_reply":
		text := req.Args["text"]
		scope := req.Args["scope"]
		res := b.TestReply(text, scope)
		data := map[string]string{
			"matched": res.Rule,
			"send":    "true",
			"scope":   scope,
		}
		if !res.Send {
			data["send"] = "false"
			data["reply"] = ""
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

// seedDefaultRules 数据库里还没有任何规则时播种一份最标准的规则（/ping）。
func seedDefaultRules(st *store.Store) {
	recs, err := st.Rules()
	if err != nil || len(recs) > 0 {
		return
	}
	dc := rules.DefaultConfig()
	_ = st.SetSetting("reply_all", store.BoolStr(dc.IsReplyAll()))
	_ = st.SetSetting("reply_default", dc.DefaultText())
	n := 0
	for i, r := range dc.Rules {
		reply := ""
		if r.Reply != nil {
			reply = *r.Reply
		}
		if _, err := st.SaveRule(store.RuleRec{
			Priority: 100 + i, Match: r.Match, Pattern: r.Pattern, Reply: reply,
			Fold: r.Fold, Always: r.Always, Enabled: true, Note: "初始规则",
		}); err == nil {
			n++
		}
	}
	log.Printf("[OK] 数据库还没有规则，已播种 %d 条默认规则(/ping)", n)
}

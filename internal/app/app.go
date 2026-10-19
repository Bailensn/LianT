// LianT —— 两层进程模型
//
//	主进程（本文件）：只负责输入指令、转发给任务进程；随时开关都不影响任务进程。
//	任务进程（LianT __run）：真正跑机器人业务，全局唯一，输出写 .log 文件。
//
// 常用指令：
//
//	LianT                  进入交互控制台（输入 start/status/logs/...）
//	LianT start            拉起任务进程（已在运行则什么都不做）
//	LianT stop             停止任务进程
//	LianT restart          重启任务进程
//	LianT status           查看任务进程与机器人状态
//	LianT logs [n] [-f]    看任务进程日志（-f 持续跟随）
//	LianT reconnect        让任务进程重连 Telegram
//	LianT run              前台运行任务进程（调试用；平时用 start）
package app

import (
	"LianT/internal/admin"
	"LianT/internal/config"
	"LianT/internal/daemon"
	"LianT/internal/ipc"
	"LianT/internal/logging"
	"LianT/internal/manager"
	"LianT/internal/platform"
	"LianT/internal/process"
	"LianT/internal/rules"
	"LianT/internal/secret"
	"LianT/internal/store"
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Main 是 CLI 入口；由根目录或 cmd/liant 的薄 main 调用。
func Main() int {
	normalizeArgs()
	args := os.Args[1:]

	// 任务进程入口：业务代码只在 daemon.Run 这条路径上跑。
	if len(args) > 0 {
		switch args[0] {
		case "__run", "daemon", "task":
			return daemon.Run(args[1:])
		case "run":
			// 前台调试：日志同时打到终端，Ctrl-C 即停。
			return daemon.Run(append([]string{"-console"}, args[1:]...))
		}
	}
	return runController(args)
}

// runController 主进程逻辑：只发指令，不碰业务。
func runController(args []string) int {
	if len(args) == 0 {
		return interactive()
	}
	cmd := strings.ToLower(args[0])
	rest := args[1:]

	switch cmd {
	case "help", "-h", "--help", "?":
		printUsage()
		return 0
	case "version", "-v", "--version":
		fmt.Printf("LianT %s (protocol %d)\n", config.AppVersion, config.ProtocolVersion)
		fmt.Printf("  数据目录: %s\n  日志文件: %s\n  控制端口: %d\n", config.DataDir, config.LogPath, config.ControlPort())
		return 0
	case "path", "where":
		fmt.Printf("可执行文件: %s\n数据目录: %s\n日志文件: %s\n凭据文件(.env): %s\n数据库: %s\npid 文件: %s\nruntime : %s\n控制端口: %s\nTelegram API: %s\n管理端: %s\n",
			selfPath(), config.DataDir, config.LogPath, config.EnvPath, config.DBPath, config.PIDPath, config.RuntimePath, config.ControlAddr(), config.APIBase(), config.AdminAddr())
		fmt.Println("（以上全部可用 LIANT_* 环境变量单独覆盖；源码里没有写死的绝对路径）")
		return 0

	case "init":
		return cmdInit()
	case "start", "ensure", "up":
		return cmdStart()
	case "stop", "down":
		return cmdStop()
	case "restart":
		return cmdRestart()
	case "status", "st", "ps":
		return cmdStatus()
	case "logs", "log", "tail":
		return cmdLogs(rest)
	case "admin", "web":
		return cmdAdmin()
	case "bot", "bots":
		return cmdBot(rest)
	case "db":
		return cmdDB()
	case "reply":
		return cmdReply(rest)
	case "reload":
		return cmdReload()
	case "reconnect", "reload-tg":
		return cmdSimple("reconnect")
	case "ping":
		return cmdSimple("ping")
	case "console", "shell", "cli":
		return interactive()
	}

	fmt.Fprintf(os.Stderr, "未知参数: %s\n\n", strings.Join(args, " "))
	printUsage()
	return 2
}

// ---------------- start / stop / restart ----------------

func cmdStart() int {
	if ok, addr, data := ipc.DaemonStatus(); ok {
		fmt.Printf("任务进程已经在运行，本次不启动（全局只会有一个）。\n")
		fmt.Printf("  PID=%s  启动时间=%s  运行时长=%s\n",
			get(data, "pid", "?"), get(data, "started_at", "?"), get(data, "uptime", "?"))
		fmt.Printf("  数据目录=%s\n", get(data, "data_dir", config.DataDir))
		fmt.Printf("  日志文件=%s\n", get(data, "log_path", config.LogPath))
		fmt.Printf("  控制端口=%s\n", addr)
		if msg := otherInstanceWarning(data); msg != "" {
			fmt.Printf("\n[!] %s\n", msg)
			fmt.Println("    要用新构建的那个：先 LianT stop（在任何目录都能停掉它），再在本目录 LianT start。")
			fmt.Println("    想同时跑两个（各用一份配置）：给其中一个加 LIANT_PORT=28732。")
		}
		return 0
	}
	cleanStale()

	// 注意：Android 上 os.Executable() 有时返回 linker64，必须用 selfPath()。
	exe := selfPath()
	if exe == "" || !FileExists(exe) {
		fmt.Fprintln(os.Stderr, "[X] 找不到自己的可执行文件，无法拉起任务进程")
		return 1
	}
	if config.LooksLikeGoRunTempDir(filepath.Dir(exe)) {
		fmt.Fprintf(os.Stderr, "[!] 当前是 go run 的临时二进制（%s，go run 退出后会被清掉）。\n"+
			"    建议先 `go build -trimpath -ldflags \"-s -w\" -o LianT .`，再用 ./LianT start，配置和日志才会落在当前目录。\n", exe)
	}
	pid, err := process.SpawnDetached(exe, []string{"__run"}, config.LogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 拉起任务进程失败: %v\n", err)
		return 1
	}
	fmt.Printf("已拉起任务进程(pid=%d)，等待控制端口就绪…\n", pid)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ok, addr, data := ipc.DaemonStatus(); ok {
			fmt.Printf("[OK] 任务进程就绪\n  pid=%s  控制端口=%s\n  日志文件=%s\n",
				get(data, "pid", strconv.Itoa(pid)), addr, get(data, "log_path", config.LogPath))
			return 0
		}
		time.Sleep(250 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "[X] 10 秒内没等到任务进程就绪，日志尾部如下（完整日志: %s）:\n", config.LogPath)
	if err := logging.Tail(config.LogPath, 20, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "（读日志失败: %v）\n", err)
	}
	return 1
}

func cmdStop() int {
	addr, ok := ipc.FindDaemonAddr()
	if !ok {
		cleanStale()
		fmt.Println("任务进程没有在运行。")
		return 0
	}
	resp, err := ipc.CallAt(addr, "stop", nil, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 发送停止指令失败: %v\n", err)
		return 1
	}
	if resp.Msg != "" {
		fmt.Println(resp.Msg)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, alive := ipc.FindDaemonAddr(); !alive {
			cleanStale()
			fmt.Println("[OK] 任务进程已停止。")
			return 0
		}
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "[!] 等待超时：任务进程可能还在收尾，稍后用 LianT status 确认。")
	return 1
}

func cmdRestart() int {
	if _, ok := ipc.FindDaemonAddr(); ok {
		if code := cmdStop(); code != 0 {
			return code
		}
	}
	return cmdStart()
}

func cmdStatus() int {
	if ok, addr, data := ipc.DaemonStatus(); ok {
		fmt.Println("任务进程: 运行中 (唯一实例)")
		fmt.Printf("  PID       : %s\n", get(data, "pid", "?"))
		fmt.Printf("  控制端口  : %s\n", addr)
		fmt.Printf("  启动时间  : %s\n", get(data, "started_at", "?"))
		fmt.Printf("  运行时长  : %s\n", get(data, "uptime", "?"))
		if v := data["bots"]; v != "" {
			fmt.Printf("  Bot 数量  : %s（运行中 %s）\n", v, get(data, "running", "0"))
		}
		fmt.Printf("  状态      : %s\n", get(data, "bot_state", "?"))
		fmt.Printf("  已收消息  : %s 条\n", get(data, "messages", "0"))
		fmt.Printf("  重连次数  : %s\n", get(data, "reconnects", "0"))
		if v := data["last_message_at"]; v != "" {
			fmt.Printf("  最近消息  : %s\n", v)
		}
		if v := data["bot_username"]; v != "" {
			fmt.Printf("  Bot       : %s（ID %s）\n", v, get(data, "bot_id", "?"))
		}
		if v := data["api_base"]; v != "" {
			fmt.Printf("  API       : %s\n", v)
		}
		if v := data["mode"]; v != "" {
			fmt.Printf("  收消息    : %s\n", v)
		}
		if v := data["last_error"]; v != "" {
			fmt.Printf("  最近错误  : %s\n", v)
		}
		fmt.Printf("  回复规则  : %s 条（版本 v%s, reply_all=%s）\n",
			get(data, "reply_rules", "?"), get(data, "reply_ver", "?"), get(data, "reply_all", "?"))
		if v := data["reply_error"]; v != "" {
			fmt.Printf("  规则错误  : %s（已继续用上一版）\n", v)
		}
		fmt.Printf("  数据库    : %s（规则 %s / 违禁词 %s）\n", get(data, "db_file", config.DBPath),
			get(data, "reply_rules", "?"), get(data, "words", "?"))
		fmt.Printf("  管理端    : http://%s%s/\n", get(data, "admin_addr", admin.ListenAddr()), get(data, "admin_prefix", ""))
		fmt.Printf("  日志文件  : %s\n", get(data, "log_path", config.LogPath))
		if v := data["exe"]; v != "" {
			fmt.Printf("  可执行文件: %s\n", v)
		}
		if msg := otherInstanceWarning(data); msg != "" {
			fmt.Printf("\n[!] %s\n", msg)
			fmt.Println("    （换目录/重新构建后想生效：LianT stop → 再 LianT start）")
		}
		return 0
	}
	fmt.Println("任务进程: 未运行")
	fmt.Printf("  （LianT start 启动；日志文件 %s）\n", config.LogPath)
	return 1
}

// cmdAdmin 打印网页管理端地址与 token（以及反代提示）。
func cmdAdmin() int {
	addr := admin.ListenAddr()
	prefix := admin.NormalizePrefix(config.EnvOr("LIANT_ADMIN_PREFIX", ""))
	// token 可能存在数据库里，所以这里自己开一次库（只读为主，没有就生成并落库），
	// 不然主进程读不到、只会显示"未生成"。
	var st *store.Store
	store.Quiet = true
	if s, err := store.Open(config.DBPath); err == nil {
		st = s
		defer st.Close()
	} else {
		fmt.Fprintf(os.Stderr, "[!] 打开数据库失败(%s)，只能读环境变量/.env 里的 token: %v\n", config.DBPath, err)
	}
	token, src := admin.TokenSource(st)
	if token == "" {
		token = "(没有可用 token)"
	}
	fmt.Printf("网页管理端: http://%s%s/\n", addr, prefix)
	fmt.Printf("token     : %s\n", token)
	fmt.Printf("token 来源: %s\n", src)
	fmt.Println()
	fmt.Println("直连（本机/同设备浏览器）：直接打开上面的地址，token 填进去即可。")
	fmt.Println("反代（推荐，nginx 示例）：")
	fmt.Printf("  location %s/ {\n      proxy_pass http://%s/;\n      proxy_set_header Host $host;\n      proxy_set_header X-Forwarded-Prefix %s;\n  }\n", admin.OrDefault(prefix, "/LianT"), addr, admin.OrDefault(prefix, "/LianT"))
	fmt.Println("Caddy 示例： reverse_proxy /LianT/* 127.0.0.1:" + portOf(addr))
	if ok, _, data := ipc.DaemonStatus(); ok {
		fmt.Printf("\n任务进程: 运行中，数据库 %s\n", get(data, "db_file", config.DBPath))
	} else {
		fmt.Println("\n任务进程: 未运行（LianT start 后再访问）")
	}
	return 0
}

func portOf(addr string) string {
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		return addr[i+1:]
	}
	return addr
}

// cmdDB 查看/初始化数据库情况。
func cmdDB() int {
	st, rec, err := openFirstBotStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] %v\n", err)
		return 1
	}
	defer st.Close()
	Rules, _ := st.Rules()
	words, _ := st.Words()
	groups, _ := st.Groups()
	admins, _ := st.Admins()
	mutes, _ := st.Mutes(true)
	bl, _ := st.Blacklists("")
	recent, _ := st.Audit(1, "", "")
	fmt.Printf("注册表: %s\n", config.DBPath)
	fmt.Printf("Bot: #%d %s（%s）\n", rec.ID, rec.Name, rec.Platform)
	fmt.Printf("业务库: %s\n", manager.BotDBPath(rec.ID))
	fmt.Printf("  回复规则 %d 条 / 违禁词 %d 条 / 群 %d 个 / 管理员 %d 人 / 禁言中 %d 人 / 黑名单 %d 人 / 审计 %d 条\n",
		len(Rules), len(words), len(groups), len(admins), len(mutes), len(bl), len(recent))
	fmt.Println("  改这些请用网页后台（LianT admin 看地址），或直接在群里用中文指令。")
	return 0
}

// openFirstBotStore 打开第一个启用 Bot 的业务库（CLI 单 Bot 视图）。
func openFirstBotStore() (*store.Store, store.BotRec, error) {
	store.Quiet = true
	reg, err := store.Open(registryDBPath())
	if err != nil {
		return nil, store.BotRec{}, fmt.Errorf("打开注册表失败: %w", err)
	}
	recs, err := reg.Bots()
	_ = reg.Close()
	if err != nil {
		return nil, store.BotRec{}, err
	}
	var rec store.BotRec
	for _, b := range recs {
		if b.Enabled {
			rec = b
			break
		}
	}
	if rec.ID == 0 && len(recs) > 0 {
		rec = recs[0]
	}
	if rec.ID == 0 {
		return nil, rec, fmt.Errorf("还没有 Bot：先运行 LianT bot add（按提示输入 token）或打开网页后台「机器人」")
	}
	st, err := store.Open(manager.BotDBPath(rec.ID))
	if err != nil {
		return nil, rec, fmt.Errorf("打开 Bot #%d 业务库失败: %w", rec.ID, err)
	}
	return st, rec, nil
}

func secretKeyLabel(dbPath string) string {
	if strings.TrimSpace(config.EnvOr("LIANT_SECRET", "")) != "" {
		return "LIANT_SECRET"
	}
	return secret.KeyPathForDB(dbPath)
}

// registryDBPath 优先用任务进程当前打开的注册表，避免 CLI 和 daemon
// 因工作目录不同而操作两个 LianT.db。
func registryDBPath() string {
	if ok, _, data := ipc.DaemonStatus(); ok && data != nil {
		if v := strings.TrimSpace(data["registry_db"]); v != "" {
			return v
		}
	}
	return config.DBPath
}

// botAddInput 解析 bot add 参数。推荐交互输入 token，避免 shell history/ps 泄漏。
func botAddInput(args []string) (token, name, platformID string, err error) {
	platformID = "telegram"
	var nameParts []string
	var appID, appSecret string
	var appSecretStdin bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--app-id="):
			appID = strings.TrimSpace(strings.TrimPrefix(a, "--app-id="))
		case a == "--app-id":
			if i+1 >= len(args) {
				return "", "", platformID, fmt.Errorf("--app-id 后面缺少值")
			}
			i++
			appID = strings.TrimSpace(args[i])
		case a == "--app-secret-stdin":
			appSecretStdin = true
		case strings.HasPrefix(a, "--platform="):
			platformID = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(a, "--platform=")))
		case a == "--platform":
			if i+1 >= len(args) {
				return "", "", platformID, fmt.Errorf("--platform 后面缺少值")
			}
			i++
			platformID = strings.ToLower(strings.TrimSpace(args[i]))
		case a == "--token-stdin":
			line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
			if rerr != nil && strings.TrimSpace(line) == "" {
				return "", "", platformID, fmt.Errorf("从 stdin 读取 token 失败: %w", rerr)
			}
			token = strings.TrimSpace(line)
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimSpace(strings.TrimPrefix(a, "--name="))
		case a == "--token" || strings.HasPrefix(a, "--token=") || a == "--app-secret" || strings.HasPrefix(a, "--app-secret="):
			return "", "", platformID, fmt.Errorf("不要把密钥写在命令行（会进 shell history）；直接运行 LianT bot add，或用 --token-stdin / --app-secret-stdin")
		default:
			if token == "" && looksLikeBotToken(a) && len(nameParts) == 0 {
				return "", "", platformID, fmt.Errorf("检测到命令行里的密钥：请删掉它，直接运行 LianT bot add 交互输入")
			}
			nameParts = append(nameParts, a)
		}
	}
	if name == "" {
		name = strings.Join(nameParts, " ")
	}
	if platformID == "" {
		platformID = "telegram"
	}
	if platform.Get(platformID) == nil {
		ids := make([]string, 0, 4)
		for _, ad := range platform.List() {
			ids = append(ids, ad.ID())
		}
		return "", "", platformID, fmt.Errorf("未知平台 %q；已注册: %s", platformID, strings.Join(ids, ", "))
	}
	if appSecretStdin {
		if platformID != "qq" {
			return "", "", platformID, fmt.Errorf("--app-secret-stdin 只用于 QQ")
		}
		if token != "" {
			return "", "", platformID, fmt.Errorf("--token-stdin 和 --app-secret-stdin 不能同时使用")
		}
		line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
		if rerr != nil && strings.TrimSpace(line) == "" {
			return "", "", platformID, fmt.Errorf("从 stdin 读取 AppSecret 失败: %w", rerr)
		}
		appSecret = strings.TrimSpace(line)
	}
	if platformID == "qq" && token == "" {
		if appID == "" {
			appID, err = readSecretLine("QQ AppID: ")
			if err != nil && appID == "" {
				return "", "", platformID, fmt.Errorf("读取 AppID 失败: %w", err)
			}
		}
		if appSecret == "" {
			appSecret, err = readSecretLine("QQ AppSecret: ")
			if err != nil && appSecret == "" {
				return "", "", platformID, fmt.Errorf("读取 AppSecret 失败: %w", err)
			}
		}
		token = strings.TrimSpace(appID) + ":" + strings.TrimSpace(appSecret)
	}
	if token == "" {
		token, err = readSecretLine("Token: ")
		if err != nil && token == "" {
			return "", "", platformID, fmt.Errorf("读取 token 失败: %w", err)
		}
	}
	if token == "" || token == ":" {
		return "", "", platformID, fmt.Errorf("凭据不能为空")
	}
	return token, name, platformID, nil
}

func looksLikeBotToken(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) >= 30 && strings.Contains(s, ":") && !strings.ContainsAny(s, " \t")
}

// cmdBot 管理 SQLite 里的多 Bot 注册表。
func cmdBot(args []string) int {
	store.Quiet = true
	dbPath := registryDBPath()
	reg, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 打开数据库失败 %s: %v\n", dbPath, err)
		return 1
	}
	defer reg.Close()

	if len(args) == 0 {
		fmt.Println("用法: LianT bot platforms | list | add [--platform qq|telegram] [名称] | add --token-stdin [名称] | add --app-id ID --app-secret-stdin [名称] | remove <id> | enable <id> | disable <id>")
		fmt.Printf("注册表: %s\n", dbPath)
		fmt.Printf("加密  : AES-256-GCM（%s）\n", secretKeyLabel(dbPath))
		return 0
	}
	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "platforms", "platform":
		for _, ad := range platform.List() {
			caps := ad.Capabilities()
			modes := []string{}
			if caps.Text { modes = append(modes, "文本") }
			if caps.Media { modes = append(modes, "媒体") }
			if caps.Inline { modes = append(modes, "内联") }
			if caps.JoinApproval { modes = append(modes, "入群审批") }
			fmt.Printf("%-10s %-10s %s\n", ad.ID(), ad.Name(), strings.Join(modes, " / "))
		}
		return 0

	case "list", "ls":
		recs, err := reg.Bots()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] 读取失败: %v\n", err)
			return 1
		}
		if len(recs) == 0 {
			fmt.Println("还没有 Bot。用 `LianT bot add`（按提示输入 token）或网页后台「机器人」添加。")
			return 0
		}
		fmt.Printf("注册表: %s\n", dbPath)
		fmt.Printf("加密  : AES-256-GCM（%s）\n", secretKeyLabel(dbPath))
		fmt.Printf("%-4s %-20s %-10s %-24s %-8s\n", "ID", "名称", "平台", "账号", "状态")
		for _, b := range recs {
			status := "停用"
			if b.Enabled {
				status = "启用"
			}
			fmt.Printf("%-4d %-20s %-10s %-24s %-8s\n", b.ID, truncateText(b.Name, 20), b.Platform, truncateText("@"+b.Username, 24), status)
		}
		return 0

	case "add":
		token, name, plat, err := botAddInput(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] %v\n", err)
			return 2
		}
		ad := platform.Get(plat)
		if ad == nil {
			fmt.Fprintf(os.Stderr, "[X] %s 适配器未注册\n", plat)
			return 1
		}
		existing, err := reg.Bots()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] 读取现有 Bot 失败: %v\n", err)
			return 1
		}
		for _, b := range existing {
			if b.Token == token {
				fmt.Fprintf(os.Stderr, "[X] 这个 token 已经添加过（Bot #%d %s）\n", b.ID, b.Name)
				return 1
			}
		}
		ident, err := ad.ValidateToken(token)
		if err != nil {
			label := "Telegram token"
			if plat == "qq" {
				label = "QQ AppID/AppSecret"
			}
			fmt.Fprintf(os.Stderr, "[X] %s 校验失败（检查凭据、网络或代理）: %v\n", label, err)
			return 1
		}
		if name == "" {
			name = ident.Name
			if strings.TrimSpace(name) == "" {
				name = "@" + ident.Username
			}
		}
		var tgID int64
		if n, err := strconv.ParseInt(ident.ID, 10, 64); err == nil {
			tgID = n
		}
		id, err := reg.AddBot(store.BotRec{
			Name: name, Token: token, Username: ident.Username,
			Platform: plat, PlatformID: ident.ID, TelegramID: tgID,
			Mode: "polling", Enabled: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] 写入失败: %v\n", err)
			return 1
		}
		fmt.Printf("[OK] 已添加 Bot #%d @%s（%s）\n", id, ident.Username, name)
		fmt.Printf("     注册表: %s\n", dbPath)
		fmt.Println("     token 已用 AES-256-GCM 加密存进 SQLite；备份数据目录时别忘了 secret.key")
		notifyBotReload()
		return 0

	case "remove", "rm", "del":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: LianT bot remove <id>")
			return 2
		}
		id, _ := strconv.ParseInt(args[1], 10, 64)
		if id <= 0 {
			fmt.Fprintln(os.Stderr, "[X] id 不合法")
			return 2
		}
		if err := reg.DeleteBot(id); err != nil {
			fmt.Fprintf(os.Stderr, "[X] 删除失败: %v\n", err)
			return 1
		}
		fmt.Printf("[OK] 已删除 Bot #%d（业务库文件保留）\n", id)
		notifyBotReload()
		return 0

	case "enable", "disable":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "用法: LianT bot %s <id>\n", sub)
			return 2
		}
		id, _ := strconv.ParseInt(args[1], 10, 64)
		rec, ok := reg.Bot(id)
		if !ok {
			fmt.Fprintf(os.Stderr, "[X] Bot #%d 不存在\n", id)
			return 1
		}
		rec.Enabled = sub == "enable"
		if err := reg.UpdateBot(rec); err != nil {
			fmt.Fprintf(os.Stderr, "[X] 更新失败: %v\n", err)
			return 1
		}
		fmt.Printf("[OK] Bot #%d 已%s\n", id, map[bool]string{true: "启用", false: "停用"}[rec.Enabled])
		notifyBotReload()
		return 0

	default:
		fmt.Fprintf(os.Stderr, "未知 bot 子命令: %s\n", sub)
		return 2
	}
}

func notifyBotReload() {
	addr, ok := ipc.FindDaemonAddr()
	if !ok {
		fmt.Println("提示：任务进程未运行，LianT start 后会读取 SQLite 里的 Bot。")
		return
	}
	resp, err := ipc.CallAt(addr, "reload", nil, 20*time.Second)
	if err != nil {
		fmt.Printf("[!] 通知任务进程失败: %v（可稍后 LianT reload）\n", err)
		return
	}
	if resp.Msg != "" {
		if resp.OK {
			fmt.Println(resp.Msg)
		} else {
			fmt.Fprintln(os.Stderr, "[!] "+resp.Msg)
		}
	}
}

// localRuleCheck 不开任务进程也能干跑规则（读数据库）。
func localRuleCheck(text string) (rules.Result, error) {
	st, _, err := openFirstBotStore()
	if err != nil {
		return rules.Result{}, err
	}
	defer st.Close()
	recs, err := st.Rules()
	if err != nil {
		return rules.Result{}, err
	}
	replyAll := strings.EqualFold(st.Setting("reply_all", "true"), "true")
	def := st.Setting("reply_default", "")
	rcfg := rules.FromRecords(recs, replyAll, def, "")
	return rcfg.Evaluate(text, "群@(本地测试)", "测试群ID", "测试用户ID"), nil
}

// cmdReload 让任务进程重载规则缓存与 .env Token（不用重启、不影响机器人）。
func cmdReload() int {
	addr, ok := ipc.FindDaemonAddr()
	if !ok {
		fmt.Fprintln(os.Stderr, "任务进程没有在运行（LianT start 启动）。")
		return 1
	}
	resp, err := ipc.CallAt(addr, "reload", nil, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] reload 失败: %v\n", err)
		return 1
	}
	fmt.Println(resp.Msg)
	// 端口/绑定地址/数据目录这类是"进程启动时"才生效的，改了要重启才能用上
	if info, ok := config.ReadRuntimeInfo(); ok {
		if info.Port != config.ControlPort() {
			fmt.Printf("  [!] .env 里端口是 %d，运行中的任务进程在 %d —— 改端口要 LianT restart 才生效\n", config.ControlPort(), info.Port)
		}
		if info.DataDir != "" && info.DataDir != config.DataDir {
			fmt.Printf("  [!] .env 里数据目录是 %s，运行中的是 %s —— 改目录要 LianT restart 才生效\n", config.DataDir, info.DataDir)
		}
	}
	if resp.OK {
		fmt.Printf("  数据库: %s\n", get(resp.Data, "db_file", config.DBPath))
		fmt.Printf("  规则条数: %s   版本: v%s   reply_all: %s\n",
			get(resp.Data, "reply_rules", "?"), get(resp.Data, "reply_ver", "?"), get(resp.Data, "reply_all", "?"))
		return 0
	}
	return 1
}

// cmdReply 干跑回复规则：
//
//	LianT reply            打印当前生效的规则
//	LianT reply /ping      看看这条消息会回什么（不会真的发消息）
func cmdReply(args []string) int {
	text := strings.TrimSpace(strings.Join(args, " "))

	if text == "" {
		st, rec, err := openFirstBotStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] %v\n", err)
			return 1
		}
		defer st.Close()
		recs, _ := st.Rules()
		words, _ := st.Words()
		fmt.Printf("Bot: #%d %s\n业务库: %s\n", rec.ID, rec.Name, manager.BotDBPath(rec.ID))
		fmt.Printf("reply_all=%s  reply_default=%q  规则 %d 条  违禁词 %d 条\n",
			st.Setting("reply_all", "true"), st.Setting("reply_default", ""), len(recs), len(words))
		fmt.Println("回复规则:")
		for _, r := range recs {
			fmt.Printf("  #%d pri=%d %s(%q) -> %q always=%v enabled=%v %s\n",
				r.ID, r.Priority, r.Match, r.Pattern, r.Reply, r.Always, r.Enabled, r.Note)
		}
		fmt.Println("违禁词:")
		for _, w := range words {
			fmt.Printf("  #%d %s(%q) 撤回=%v 禁言=%ds 踢=%v 拉黑=%v 回复=%q\n",
				w.ID, w.Match, w.Word, w.Recall, w.MuteSeconds, w.Kick, w.Blacklist, w.Reply)
		}
		fmt.Println("改这些：网页后台（LianT admin 看地址）或群内中文指令；试跑：LianT reply /ping")
		return 0
	}

	addr, ok := ipc.FindDaemonAddr()
	if !ok {
		// 任务进程没跑：直接读数据库干跑一遍（只读）
		res, err := localRuleCheck(text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[X] 读数据库失败: %v\n", err)
			return 1
		}
		printReplyResult(text, res)
		return 0
	}
	resp, err := ipc.CallAt(addr, "test_reply", map[string]string{"text": text}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 测试失败: %v\n", err)
		return 1
	}
	printReplyResult(text, rules.Result{
		Send: get(resp.Data, "send", "false") == "true",
		Text: get(resp.Data, "reply", ""),
		Rule: get(resp.Data, "matched", "?"),
	})
	return 0
}

func printReplyResult(text string, res rules.Result) {
	if !res.Send {
		fmt.Printf("测试 %q → 不回（%s）\n", text, res.Rule)
		return
	}
	fmt.Printf("测试 %q → 会回复:\n  %s\n  命中规则: %s\n", text, res.Text, res.Rule)
}

// cmdInit 生成 .env 模板，并给数据库播种默认规则（都已存在则不动）。
func cmdInit() int {
	madeEnv := config.EnsureDotEnvTemplate(config.EnvPath)
	store.Quiet = true
	st, err := store.Open(config.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] 打开数据库失败 %s: %v\n", config.DBPath, err)
		return 1
	}
	defer st.Close()
	bots, _ := st.Bots()
	fmt.Printf("配置模板: %s%s\n", config.EnvPath, mark(madeEnv))
	fmt.Printf("注册表  : %s（Bot %d 个）\n", config.DBPath, len(bots))
	fmt.Println("Bot token 不在 .env：用 `LianT bot add`（按提示输入 token）或网页后台「机器人」添加。")
	fmt.Println("每个 Bot 会自动创建独立业务库（LianT-data/bots/<id>.db），规则/违禁词互不影响。")
	fmt.Println("路径/端口仍可用 .env 或 LIANT_* 环境变量覆盖；改完重启生效。")
	return 0
}

func mark(made bool) string {
	if made {
		return "  (新生成)"
	}
	return "  (已存在，未改动)"
}

// otherInstanceWarning 判断"正在运行的任务进程"是不是别的位置的实例：
// 换了目录/重新构建后最容易踩的坑——start 会直接说"已经在运行"，
// 你以为跑的是新二进制，其实是旧目录里那个进程。
func otherInstanceWarning(data map[string]string) string {
	var diffs []string
	if v := data["data_dir"]; v != "" && v != config.DataDir {
		diffs = append(diffs, fmt.Sprintf("数据目录 %s（当前目录对应 %s）", v, config.DataDir))
	}
	if v := data["env_file"]; v != "" && v != config.EnvPath {
		diffs = append(diffs, fmt.Sprintf("凭据文件 %s（当前目录对应 %s）", v, config.EnvPath))
	}
	if v := data["exe"]; v != "" {
		if self, err := os.Executable(); err == nil && v != self {
			diffs = append(diffs, fmt.Sprintf("可执行文件 %s（本次是 %s）", v, self))
		}
	}
	if len(diffs) == 0 {
		return ""
	}
	return "正在运行的是另一个位置的实例：" + strings.Join(diffs, "；")
}

// cmdSimple 把一条指令转发给任务进程。
func cmdSimple(cmd string) int {
	addr, ok := ipc.FindDaemonAddr()
	if !ok {
		fmt.Fprintln(os.Stderr, "任务进程没有在运行（用 LianT start 启动）。")
		return 1
	}
	resp, err := ipc.CallAt(addr, cmd, nil, 15*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[X] %s 失败: %v\n", cmd, err)
		return 1
	}
	if resp.Msg != "" {
		fmt.Println(resp.Msg)
	}
	if !resp.OK {
		return 1
	}
	return 0
}

// ---------------- logs ----------------

func cmdLogs(args []string) int {
	n := 50
	follow := false
	for _, a := range args {
		switch strings.TrimSpace(a) {
		case "":
		case "-f", "--follow", "f", "follow":
			follow = true
		default:
			v, err := strconv.Atoi(a)
			if err != nil || v <= 0 {
				fmt.Fprintf(os.Stderr, "无法识别的日志参数: %s（用法: LianT logs [行数] [-f]）\n", a)
				return 2
			}
			n = v
		}
	}

	path := config.LogPath
	if ok, _, data := ipc.DaemonStatus(); ok {
		if v := data["log_path"]; v != "" {
			path = v
		}
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "日志文件还不存在: %s\n", path)
		return 1
	}

	fmt.Printf("== %s ==\n", path)
	if !follow {
		if err := logging.Tail(path, n, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "[X] 读日志失败: %v\n", err)
			return 1
		}
		return 0
	}
	if err := logging.Tail(path, n, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "[X] 读日志失败: %v\n", err)
		return 1
	}
	fmt.Println("--- 开始跟随（只读日志，不影响任务进程），Ctrl-C 退出 ---")

	stop := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		close(stop)
	}()
	if err := logging.Follow(path, os.Stdout, stop); err != nil {
		fmt.Fprintf(os.Stderr, "[X] 跟随日志失败: %v\n", err)
		return 1
	}
	return 0
}

// ---------------- 交互控制台 ----------------

func interactive() int {
	fmt.Printf("LianT %s 控制台（主进程：只发指令；本进程退出不影响任务进程）\n", config.AppVersion)
	if ok, addr, data := ipc.DaemonStatus(); ok {
		fmt.Printf("任务进程: 运行中  pid=%s  端口=%s  机器人=%s\n",
			get(data, "pid", "?"), addr, get(data, "bot_state", "?"))
	} else {
		fmt.Println("任务进程: 未运行（输入 start 启动）")
	}
	fmt.Println("指令: start | stop | restart | status | logs [行数] [-f] | reload | reply [文本] | reconnect | ping | path | init | help | exit")
	fmt.Println("（exit 只退出控制台，任务进程继续在后台跑）")

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	fmt.Print("LianT> ")
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			fmt.Print("LianT> ")
			continue
		}
		fields := strings.Fields(line)
		cmd := strings.ToLower(fields[0])
		rest := fields[1:]

		switch cmd {
		case "exit", "quit", "q", "bye":
			fmt.Println("控制台退出；任务进程继续运行（LianT status 可查看）。")
			return 0
		case "help", "?", "h":
			printUsage()
		case "start", "up":
			cmdStart()
		case "stop", "down":
			cmdStop()
		case "restart":
			cmdRestart()
		case "status", "st", "ps":
			cmdStatus()
		case "logs", "log", "tail":
			cmdLogs(rest)
		case "reload":
			cmdReload()
		case "reply":
			cmdReply(rest)
		case "init":
			cmdInit()
		case "reconnect":
			cmdSimple("reconnect")
		case "ping":
			cmdSimple("ping")
		case "path", "where":
			runController([]string{"path"})
		default:
			fmt.Printf("未知指令: %s（输入 help 看用法）\n", cmd)
		}
		fmt.Print("LianT> ")
	}
	fmt.Println("\n(stdin 已关闭，控制台退出；任务进程继续在后台运行)")
	return 0
}

// ---------------- 小工具 ----------------

func printUsage() {
	fmt.Printf(`LianT %s —— 主进程/任务进程两层结构

主进程(LianT)只发指令；任务进程(LianT __run)常驻跑业务，全局唯一。
主进程开多少次、退出多少次，都不会影响任务进程；业务输出全部写日志文件。

用法:
  LianT                    进入交互控制台
  LianT start              拉起任务进程（已在运行则什么都不做）
  LianT stop               停止任务进程
  LianT restart            重启任务进程（先 stop 再 start）
  LianT status             查看任务进程、机器人、回复规则状态
  LianT logs [行数] [-f]    查看日志，默认 50 行，-f 持续跟随
  LianT admin              网页管理端地址 + token（以及反代示例）
  LianT db                 看数据库统计（规则/违禁词/群/管理员…）
  LianT reload             同步 SQLite Bot 注册表并请求重连（不重启进程）
  LianT reply [文本]        不带文本=打印当前规则；带文本=干跑看会回什么
  LianT reconnect          让任务进程尽快重连 Telegram（进程不退出）
  LianT run                前台运行任务进程（调试用，日志同时打屏）
  LianT init               生成 .env 模板（Bot token 在 SQLite / 网页后台）
  LianT path               打印数据目录/日志/配置文件等路径
  LianT bot list           列出 SQLite 里的 Bot
  LianT bot add [名称]     交互输入 token；加 --platform qq 添加 QQ Bot
  LianT bot remove <id>    删除 Bot 注册
  LianT bot enable/disable <id>
  LianT help               本帮助

配置（改完保存即生效，不用重新编译）:
  Bot token                    SQLite bots 表：./LianT bot list 或网页「机器人」
  路径/端口                    .env：%s
  API / 代理                    LIANT_API_BASE / LIANT_PROXY
  环境变量优先级: 进程环境变量 > .env > config.go 常量
  日志文件  %s         （单文件 8MB 自动轮转，留 .1 ~ .5）
  数据目录  %s
  数据库    %s
  控制端口  %d          （LIANT_PORT，只在启动时读）
`,
		config.AppVersion, config.EnvPath, config.LogPath, config.DataDir, config.DBPath, config.ControlPort())
}

func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func get(m map[string]string, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

func selfPath() string {
	// os.Executable() 在个别环境（Android 上经 linker 垫片 exec）会指向 linker64 而不是自己，
	// 那种路径拿去拉起任务进程必然失败，所以这里要认一下。
	if exe, err := os.Executable(); err == nil && exe != "" {
		if !strings.Contains(strings.ToLower(filepath.Base(exe)), "linker") {
			if abs, err2 := filepath.Abs(exe); err2 == nil {
				return abs
			}
			return exe
		}
	}
	// 退路：argv 里第一个"真实存在的绝对路径文件"就是自己
	for _, a := range os.Args {
		if a == "" || strings.HasPrefix(a, "-") || !filepath.IsAbs(a) {
			continue
		}
		if st, err := os.Stat(a); err == nil && !st.IsDir() {
			return a
		}
	}
	if len(os.Args) > 0 {
		if abs, err := filepath.Abs(os.Args[0]); err == nil {
			return abs
		}
		return os.Args[0]
	}
	return ""
}

// cleanStale 清理残留的 runtime/pid 文件（仅在确认任务进程不在运行时）。
func cleanStale() {
	if _, alive := ipc.FindDaemonAddr(); alive {
		return
	}
	_ = os.Remove(config.RuntimePath)
	_ = os.Remove(config.PIDPath)
}

// normalizeArgs 兼容一类特殊运行环境（Termux / DSH 的 termux-exec force 模式）：
// 系统 linker 直接执行静态二进制时会把自己占在 argv[0]，把真正的程序路径挤到 argv[1]，
// 于是 os.Args 变成 [linker, 程序路径, 真实参数...]。这里识别并修正，避免指令错位。
func normalizeArgs() {
	if len(os.Args) < 2 {
		return
	}
	// Android 的 linker64 在 exec 时会把程序路径再塞进 argv[1]（argv[0] 可能是
	// linker64，也可能已经是程序自己）。Termux/手机环境常见，这里统一剥掉，
	// 否则 `LianT start` 会被当成 `LianT /path/to/LianT start`。
	// 情况一：argv[0] 和 argv[1] 指向同一个可执行文件（ld/linker 多塞了一份）。
	if samePath(os.Args[0], os.Args[1]) {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		return
	}
	// 情况二：os.Executable() 返回 linker64，argv[1] 才是真正的程序路径。
	exe, err := os.Executable()
	if err == nil && strings.Contains(strings.ToLower(filepath.Base(exe)), "linker") {
		if abs, aerr := filepath.Abs(os.Args[1]); aerr == nil && FileExists(abs) {
			os.Args = append(os.Args[:1], os.Args[2:]...)
			return
		}
	}
	// 情况三：argv[1] 就是 os.Executable() 指向的文件。
	if err == nil && exe != "" && samePath(os.Args[1], exe) {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
}

// FileExists 判断路径是不是存在的普通文件。
func FileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func samePath(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	return filepath.Base(a) == filepath.Base(b)
}

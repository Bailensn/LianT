package cfg

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// 全局配置
//
// 进程模型：
//   主进程(main.go)   —— 只收发指令的控制台，随时开关都不影响任务进程
//   任务进程(go) —— 真正跑业务(go)的常驻进程，全局唯一
//
// 唯一性靠"谁先绑定控制端口谁就是任务进程"保证：第二个进程绑定失败就直接退出。
// ============================================================================

// ---------------- 机器人凭据 ----------------
// 优先级：进程环境变量 > .env 文件 > 这里的常量
// 平时把凭据写在进行目录的 .env 里即可（dotenv.go），改完保存即生效，不用重新编译。
// 这里的常量只是最后兜底，留空就行。
var (
	AppID     = ""
	AppSecret = ""
)

// ---------------- QQ 开放平台常量 ----------------
const (
	// Intents：位掩码，1<<25 覆盖 群@ / 群全量 / 单聊，1<<30 是频道@
	IntentGroupC2C = 1 << 25
	IntentGuildAt  = 1 << 30

	// 群里开了"接收全部消息"时是否每条都回，现在由数据库 settings.reply_all 决定
	// （见 reply.go），不再写死在代码里。
)

// ---------------- 接口地址：默认值只是缺省，全部可用环境变量覆盖 ----------------
// 消息/网关路径里的 %s 是群号/用户号占位符。
func APIBase() string {
	return strings.TrimRight(EnvOr("QBOT_API_BASE", "https://api.bot.qq.com"), "/")
}

func TokenURL() string {
	return EnvOr("QBOT_TOKEN_URL", "https://bots.qq.com/app/getAppAccessToken")
}

func GatewayURL() string {
	return APIBase() + EnvOr("QBOT_GATEWAY_PATH", "/gateway")
}

func GroupMessageURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_MESSAGE_PATH", "/v2/groups/%s/messages"), groupOpenID)
}

func GroupRecallURL(groupOpenID, messageID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_RECALL_PATH", "/v2/groups/%s/messages/%s"), groupOpenID, messageID)
}

func GroupMuteURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_MUTE_PATH", "/v2/groups/%s/restrict_chat_setting"), groupOpenID)
}

func GroupInfoURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_INFO_PATH", "/v2/groups/%s/info"), groupOpenID)
}

func GroupBotStateURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_BOT_STATE_PATH", "/v2/groups/%s/bot_state"), groupOpenID)
}

func GroupMembersURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_MEMBERS_PATH", "/v2/groups/%s/members"), groupOpenID)
}

func GroupRemoveMembersURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_REMOVE_MEMBERS_PATH", "/v2/groups/%s/batch_remove_members"), groupOpenID)
}

func GroupJoinListURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_JOIN_LIST_PATH", "/v2/groups/%s/join_request_list"), groupOpenID)
}

func GroupApproveJoinURL(groupOpenID, memberOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_APPROVE_JOIN_PATH", "/v2/groups/%s/approval_join_request/%s"), groupOpenID, memberOpenID)
}

func GroupBlacklistURL(groupOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_GROUP_BLACKLIST_PATH", "/v2/groups/%s/member_blacklist"), groupOpenID)
}

func C2cMessageURL(userOpenID string) string {
	return APIBase() + fmt.Sprintf(EnvOr("QBOT_C2C_MESSAGE_PATH", "/v2/users/%s/messages"), userOpenID)
}

// ControlHost 控制端口绑定的地址，默认只绑本机回环；改成非回环会把机器人控制权暴露到网络。
func ControlHost() string {
	return EnvOr("QBOT_BIND", "127.0.0.1")
}

// IsLoopbackHost 判断控制端口是否只绑在本机。
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// EnvOr 取环境变量，空则用缺省值。
// EnvOr 按 进程环境变量 > .env 文件 > 默认值 的顺序取值。
// .env 是热读的（mtime 变了就重载），所以改完保存即生效，不需要重启。
func EnvOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	if d := EnvFile(); d != nil {
		if v, ok := d.Lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return def
}

// 控制端口：任务进程监听 该端口，这个监听本身即是"全局唯一"的锁。
const DefaultControlPort = 28731

// 主进程与任务进程的指令协议版本，不一致时会在状态里提示。
const ProtocolVersion = 1

const AppVersion = "2.0"

// ---------------- 运行目录与配置文件 ----------------
// 不写死任何绝对路径：目录由 QBOT_* 环境变量 / 当前目录 / 可执行文件位置 / home / 临时目录 动态决定，
// 文件名一律是相对名，可再分别用环境变量覆盖。
var (
	DataDir     string
	LogPath     string
	PIDPath     string
	RuntimePath string
	ReplyPath   string
	EnvPath     string
	DBPath      string
)

func init() { Init() }

// Init 解析全部路径（包加载时已自动调用；环境变量改了可再调一次）。
func Init() {
	// 第一步只能靠进程环境变量/默认搜索路径找到 .env；挂上它之后，
	// 下面所有 EnvOr 就都能读到 .env 里的值了（包括 QBOT_DATA_DIR 这种路径设置）。
	EnvPath = ResolveEnvFilePath()
	dotEnvInst = NewDotEnv(EnvPath)

	DataDir = ResolveDataDir()
	if strings.TrimSpace(DataDir) == "" {
		DataDir = filepath.Join(os.TempDir(), "qbot")
		_ = os.MkdirAll(DataDir, 0o755)
	}
	LogPath = EnvOr("QBOT_LOG_FILE", filepath.Join(DataDir, "qbot.log"))
	PIDPath = EnvOr("QBOT_PID_FILE", filepath.Join(DataDir, "qbot.pid"))
	RuntimePath = EnvOr("QBOT_RUNTIME_FILE", filepath.Join(DataDir, "qbot.runtime.json"))
	ReplyPath = ResolveReplyFilePath()
	// 业务数据库（SQLite）：路径同样动态获取 + 可覆盖，不写死
	DBPath = EnvOr("QBOT_DB_FILE", filepath.Join(DataDir, "qbot.db"))
	for name, v := range map[string]string{
		"数据目录": DataDir, "日志文件": LogPath, "pid 文件": PIDPath,
		"runtime": RuntimePath, "数据库": DBPath, "凭据文件": EnvPath,
	} {
		if strings.TrimSpace(v) == "" {
			log.Printf("[!] %s 解析为空（这不该发生，请把 qbot path 的输出发出来）", name)
		}
	}
}

// ResolveReplyFilePath 见下方（ConfigSearchDirs 之后）。

func FileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// ResolveDataDir 选一个可写的运行目录（日志/pid/runtime 放这里）。
// 顺序：QBOT_DATA_DIR > 当前目录/qbot-data > 可执行文件旁/qbot-data > ~/.qbot > 临时目录。
// 当前目录优先是有意的：go run . 时可执行文件在临时构建目录里（会被清掉），
// 放在当前目录才符合直觉、也才找得到。
func ResolveDataDir() string {
	var targets []string
	if d := strings.TrimSpace(EnvOr("QBOT_DATA_DIR", "")); d != "" {
		targets = append(targets, d)
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		targets = append(targets, filepath.Join(wd, "qbot-data"))
	}
	if exe, err := os.Executable(); err == nil {
		if d := filepath.Dir(exe); !LooksLikeGoRunTempDir(d) {
			targets = append(targets, filepath.Join(d, "qbot-data"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		targets = append(targets, filepath.Join(home, ".qbot"))
	}
	targets = append(targets, filepath.Join(os.TempDir(), "qbot"))
	for _, t := range targets {
		if err := os.MkdirAll(t, 0o755); err != nil {
			continue
		}
		// 能建目录不等于能写文件（Android 的 sdcard/FUSE、scoped storage 就常这样），
		// 真写一个临时文件验证，写不进去就换下一个候选目录。
		if WritableDir(t) {
			return t
		}
	}
	return os.TempDir()
}

// WritableDir 目录存在且真的能写文件。
func WritableDir(dir string) bool {
	if dir == "" {
		return false
	}
	f, err := os.CreateTemp(dir, ".qbot-w-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// LooksLikeGoRunTempDir 判断"这个目录不是放配置的地方"：
//  1. 目录名正好是 exe —— go run 的临时可执行文件就放在 <临时目录>/go-buildNNN/b001/exe；
//  2. 目录落在系统临时目录里（os.TempDir()，跟随 TMPDIR，不写死）。
//
// 这两处的文件随时会被清掉，配置和日志放进去必然找不到。
func LooksLikeGoRunTempDir(dir string) bool {
	if dir == "" {
		return false
	}
	if filepath.Base(filepath.Clean(dir)) == "exe" {
		return true
	}
	tmp := os.TempDir()
	if tmp == "" || tmp == string(filepath.Separator) {
		return false
	}
	cleanDir := filepath.Clean(dir)
	cleanTmp := filepath.Clean(tmp)
	return cleanDir == cleanTmp || strings.HasPrefix(cleanDir, cleanTmp+string(filepath.Separator))
}

// ConfigSearchDirs 配置文件的搜索/落地顺序：当前目录 → 可执行文件旁(跳过临时构建目录) → 数据目录。
func ConfigSearchDirs() []string {
	var dirs []string
	if wd, err := os.Getwd(); err == nil && wd != "" {
		dirs = append(dirs, wd)
	}
	if exe, err := os.Executable(); err == nil {
		if d := filepath.Dir(exe); !LooksLikeGoRunTempDir(d) && !ContainsString(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	if !ContainsString(dirs, DataDir) {
		dirs = append(dirs, DataDir)
	}
	return dirs
}

func ContainsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ResolveReplyFilePath 依次找 Reply.json：QBOT_REPLY_FILE > 已存在的（当前目录 → 可执行文件旁 → 数据目录）
// > 在第一个可写目录里新建。
func ResolveReplyFilePath() string {
	if p := strings.TrimSpace(EnvOr("QBOT_REPLY_FILE", "")); p != "" {
		return p
	}
	dirs := ConfigSearchDirs()
	for _, d := range dirs {
		if p := filepath.Join(d, "Reply.json"); FileExists(p) {
			return p
		}
	}
	for _, d := range dirs {
		if WritableDir(d) {
			return filepath.Join(d, "Reply.json")
		}
	}
	return filepath.Join(DataDir, "Reply.json")
}

// ControlPort 允许用 QBOT_PORT 覆盖端口（多个不同的机器人可以各占一个端口）。
func ControlPort() int {
	v := strings.TrimSpace(EnvOr("QBOT_PORT", ""))
	if v == "" {
		return DefaultControlPort
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 || n > 65535 {
		log.Printf("[!] QBOT_PORT=%q 不是合法端口（1-65535），改用默认 %d", v, DefaultControlPort)
		return DefaultControlPort
	}
	return n
}

// ControlAddr 控制端口的监听地址；只绑回环时外部连不上。
func ControlAddr() string {
	return net.JoinHostPort(ControlHost(), strconv.Itoa(ControlPort()))
}

// Credentials 取机器人凭据：进程环境变量 > .env > 代码常量。
func Credentials() (string, string) {
	id, sec, _ := CredentialsWithSource()
	return id, sec
}

// CredentialsWithSource 同时告诉调用方这份凭据是从哪来的（排错时特别有用）。
func CredentialsWithSource() (string, string, string) {
	id, idFrom := LookupCredential("QBOT_APP_ID", AppID)
	sec, secFrom := LookupCredential("QBOT_APP_SECRET", AppSecret)
	switch {
	case id == "" || sec == "":
		return id, sec, "未配置"
	case idFrom == secFrom:
		return id, sec, idFrom
	default:
		return id, sec, "环境变量 + " + idFrom + " 混用"
	}
}

// LookupCredential 按 环境变量 -> .env -> 常量 的顺序取值。
func LookupCredential(key, fallback string) (string, string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v, "环境变量"
	}
	if v, ok := EnvFile().get(key); ok {
		return strings.TrimSpace(v), "文件 " + EnvPath
	}
	if v := strings.TrimSpace(fallback); v != "" {
		return v, "config.go 常量"
	}
	return "", ""
}

// CredentialsMissing 判断凭据是否还没填（含模板占位符）。
func CredentialsMissing() bool {
	id, sec, _ := CredentialsWithSource()
	if id == "" || sec == "" {
		return true
	}
	return strings.Contains(id, "在这里") || strings.Contains(sec, "在这里")
}

func CredentialsError() string {
	if !CredentialsMissing() {
		return ""
	}
	_, _, src := CredentialsWithSource()
	return fmt.Sprintf("机器人凭据未配置(来源: %s)：请填 %s 里的 QBOT_APP_ID / QBOT_APP_SECRET，"+
		"或设置同名环境变量；填好后保存即生效（凭据最长 60 秒自动重试，也可以敲 qbot reload 立即生效）", src, EnvPath)
}

// ---------------- runtime 文件 ----------------
// 任务进程启动后写下自己的端口/pid/日志路径，主进程据此找到它。
type RuntimeInfo struct {
	Version    int    `json:"version"`
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	DataDir    string `json:"data_dir"`
	LogPath    string `json:"log_path"`
	Executable string `json:"executable"`
	StartedAt  string `json:"started_at"`
}

func WriteRuntimeInfo(info RuntimeInfo) error {
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	tmp := RuntimePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, RuntimePath)
}

func ReadRuntimeInfo() (RuntimeInfo, bool) {
	var info RuntimeInfo
	b, err := os.ReadFile(RuntimePath)
	if err != nil {
		return info, false
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return info, false
	}
	return info, info.Port > 0
}

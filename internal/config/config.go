package config

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============================================================================
// LianT 全局配置
//
// 进程模型：
//   主进程(LianT)      —— 只收发指令的控制台，随时开关不影响任务进程
//   任务进程(LianT __run) —— 常驻跑 Telegram Bot，全局唯一（靠绑定控制端口做单例）
//
// 配置优先级：进程环境变量 > .env 文件 > 代码默认值。
// 业务数据（规则/违禁词/群/管理员）全部在 SQLite 里，不在 .env。
// ============================================================================

const (
	DefaultControlPort = 28741
	DefaultAdminAddr   = "127.0.0.1:28742"

	ProtocolVersion = 1
	AppVersion      = "0.1.0"
	AppName         = "LianT"
)

// ---------------- Telegram API ----------------

// APIBase 允许走反代/自建 API 入口。
func APIBase() string {
	return strings.TrimRight(EnvOr("LIANT_API_BASE", "https://api.telegram.org"), "/")
}

// Proxy 支持 http:// 或 socks5:// 代理（手机/受限网络常用）。
func Proxy() string {
	return strings.TrimSpace(EnvOr("LIANT_PROXY", ""))
}

// ---------------- 控制端口 ----------------

// ControlHost 控制端口绑定地址，默认只绑回环。
func ControlHost() string {
	return EnvOr("LIANT_BIND", "127.0.0.1")
}

// IsLoopbackHost 判断控制端口是否只绑本机。
func IsLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// EnvOr 按 进程环境变量 > .env 文件 > 默认值 取值；.env 热读。
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

// ---------------- 运行目录与文件 ----------------

var (
	DataDir     string
	LogPath     string
	PIDPath     string
	RuntimePath string
	EnvPath     string
	DBPath      string
)

func init() { Init() }

// Init 解析全部路径（环境变量改了可再调一次）。
func Init() {
	EnvPath = ResolveEnvFilePath()
	dotEnvInst = NewDotEnv(EnvPath)

	DataDir = ResolveDataDir()
	if strings.TrimSpace(DataDir) == "" {
		DataDir = filepath.Join(os.TempDir(), "LianT")
		_ = os.MkdirAll(DataDir, 0o755)
	}
	LogPath = EnvOr("LIANT_LOG_FILE", filepath.Join(DataDir, "LianT.log"))
	PIDPath = EnvOr("LIANT_PID_FILE", filepath.Join(DataDir, "LianT.pid"))
	RuntimePath = EnvOr("LIANT_RUNTIME_FILE", filepath.Join(DataDir, "LianT.runtime.json"))
	DBPath = EnvOr("LIANT_DB_FILE", filepath.Join(DataDir, "LianT.db"))
}

func FileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// ResolveDataDir 选一个可写的运行目录。
// 顺序：LIANT_DATA_DIR > 当前目录/LianT-data > 可执行文件旁/LianT-data > ~/.LianT > 临时目录。
func ResolveDataDir() string {
	var targets []string
	if d := strings.TrimSpace(EnvOr("LIANT_DATA_DIR", "")); d != "" {
		targets = append(targets, d)
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		targets = append(targets, filepath.Join(wd, "LianT-data"))
	}
	if exe, err := os.Executable(); err == nil {
		if d := filepath.Dir(exe); !LooksLikeGoRunTempDir(d) {
			targets = append(targets, filepath.Join(d, "LianT-data"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		targets = append(targets, filepath.Join(home, ".LianT"))
	}
	targets = append(targets, filepath.Join(os.TempDir(), "LianT"))
	for _, t := range targets {
		if err := os.MkdirAll(t, 0o755); err != nil {
			continue
		}
		if WritableDir(t) {
			return t
		}
	}
	return os.TempDir()
}

// WritableDir 目录存在且真的能写文件（Android 的 sdcard/FUSE 常"能建目录不能写文件"）。
func WritableDir(dir string) bool {
	if dir == "" {
		return false
	}
	f, err := os.CreateTemp(dir, ".LianT-w-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// LooksLikeGoRunTempDir 判断目录是不是 go run 的临时构建目录（文件随时会被清掉）。
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

// ConfigSearchDirs 配置文件的搜索/落地顺序：当前目录 → 可执行文件旁 → 数据目录。
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

// ControlPort 允许用 LIANT_PORT 覆盖端口（多个机器人可各占一个）。
func ControlPort() int {
	v := strings.TrimSpace(EnvOr("LIANT_PORT", ""))
	if v == "" {
		return DefaultControlPort
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 || n > 65535 {
		return DefaultControlPort
	}
	return n
}

// ControlAddr 控制端口监听地址。
func ControlAddr() string {
	return net.JoinHostPort(ControlHost(), strconv.Itoa(ControlPort()))
}

// AdminAddr 网页后台监听地址。
func AdminAddr() string {
	return EnvOr("LIANT_ADMIN_ADDR", DefaultAdminAddr)
}

// AdminToken 网页后台 token（空 = 启动时自动生成并写库）。
func AdminToken() string {
	return strings.TrimSpace(EnvOr("LIANT_ADMIN_TOKEN", ""))
}

// AdminPrefix 反代挂载前缀，如 /LianT。
func AdminPrefix() string {
	return EnvOr("LIANT_ADMIN_PREFIX", "")
}

// AdminHTMLPath 自定义管理页面路径。
func AdminHTMLPath() string {
	return strings.TrimSpace(EnvOr("LIANT_ADMIN_HTML", ""))
}

// ---------------- runtime 文件 ----------------

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

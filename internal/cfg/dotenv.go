package cfg

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// .env 读取
//
// 优先级：进程环境变量 > .env 文件 > config.go 里的常量
// 热生效：每次取凭据都会先 stat 一下 .env，变了就重读，所以改完保存即生效
//        （凭据类改动的生效时机 = 下一次取 token 时，最长 60 秒，或敲一次 qbot reload）
//
// 文件格式：
//   # 整行注释（也支持行尾 " # 注释"，但值里有 # 时请用引号包起来）
//   QBOT_APP_ID=你的AppID
//   QBOT_APP_SECRET="你的 AppSecret"
//   export QBOT_APP_ID=xxx      # export 前缀可有可无
// ============================================================================

type DotEnv struct {
	mu        sync.Mutex
	path      string
	values    map[string]string
	modTime   time.Time
	size      int64
	loaded    bool
	lastErr   string
	loadedAt  time.Time
	lastCheck time.Time
}

var (
	dotEnvInst *DotEnv
)

// EnvFile 返回全局 .env 读取器（懒加载）。
// EnvFile 返回 .env 的读取器（Init() 里已建立；没建立时现场补一个）。
func EnvFile() *DotEnv {
	if dotEnvInst == nil {
		if EnvPath == "" {
			Init()
		} else {
			dotEnvInst = NewDotEnv(EnvPath)
		}
	}
	return dotEnvInst
}

func NewDotEnv(path string) *DotEnv {
	return &DotEnv{path: path, values: map[string]string{}}
}

// get 取一个键；每次调用前都会检查文件是否被改过。
func (d *DotEnv) get(key string) (string, bool) {
	d.reloadIfChanged()
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.values[key]
	return v, ok && v != ""
}

// reloadIfChanged 文件 mtime/size 变了才重读；500ms 内不重复 stat。
func (d *DotEnv) reloadIfChanged() {
	d.mu.Lock()
	if time.Since(d.lastCheck) < 500*time.Millisecond {
		d.mu.Unlock()
		return
	}
	d.lastCheck = time.Now()
	loaded, mt, sz := d.loaded, d.modTime, d.size
	d.mu.Unlock()

	st, err := os.Stat(d.path)
	if err != nil {
		return // 文件不在/读不了：保持现有值
	}
	if loaded && st.ModTime().Equal(mt) && st.Size() == sz {
		return
	}
	if err := d.Reload(); err != nil {
		log.Printf("[!] 读取 .env 失败（继续用上一版）: %v", err)
	}
}

// Reload 强制重读 .env。
func (d *DotEnv) Reload() error {
	b, err := os.ReadFile(d.path)

	d.mu.Lock()
	defer d.mu.Unlock()
	if st, serr := os.Stat(d.path); serr == nil {
		d.modTime, d.size = st.ModTime(), st.Size()
	}
	if err != nil {
		d.lastErr = err.Error()
		return err
	}
	d.values = ParseDotEnv(string(b))
	d.loaded = true
	d.loadedAt = time.Now()
	d.lastErr = ""
	return nil
}

// ParseDotEnv 解析 .env 文本（纯函数，方便测试）。
func ParseDotEnv(text string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		val = UnquoteEnvValue(val)
		out[key] = val
	}
	return out
}

// UnquoteEnvValue 处理引号、行尾注释和常见转义。
func UnquoteEnvValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			inner := v[1 : len(v)-1]
			if v[0] == '\'' {
				return inner // 单引号：原样
			}
			r := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\\`, `\`)
			return r.Replace(inner)
		}
	}
	// 未加引号：去掉行尾 " # 注释"
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// EnsureDotEnvTemplate 没有 .env 时生成一份注释模板，方便用户直接填。
// 返回是否新建了文件。
func EnsureDotEnvTemplate(path string) bool {
	if FileExists(path) {
		return false
	}
	content := "# qbot 配置（本文件会被自动读取，改完保存即生效，不用重新编译）\n" +
		"# 也支持行尾注释；值里含 # 请用引号包起来。\n" +
		"\n" +
		"# QQ 机器人凭据（必填）\n" +
		"QBOT_APP_ID=\n" +
		"QBOT_APP_SECRET=\n" +
		"\n" +
		"# ---- 可选：下面这些只在任务进程启动时读一次，改完要 qbot restart ----\n" +
		"# 端口/绑定地址/目录与文件名（默认都在当前目录或可执行文件旁，源码不写死路径）\n" +
		"# QBOT_PORT=28731\n" +
		"# QBOT_BIND=127.0.0.1\n" +
		"# QBOT_DATA_DIR=\n" +
		"# QBOT_LOG_FILE=\n" +
		"# QBOT_PID_FILE=\n" +
		"# QBOT_RUNTIME_FILE=\n" +
		"# QBOT_REPLY_FILE=\n" +
		"# QBOT_ENV_FILE=\n" +
		"# 接口地址（沙盒环境改这个）\n" +
		"# QBOT_API_BASE=https://api.bot.qq.com\n" +
		"# QBOT_TOKEN_URL=https://bots.qq.com/app/getAppAccessToken\n" +
		"# QBOT_GATEWAY_PATH=/gateway\n" +
		"# QBOT_GROUP_MESSAGE_PATH=/v2/groups/%s/messages\n" +
		"# QBOT_C2C_MESSAGE_PATH=/v2/users/%s/messages\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		log.Printf("[!] 生成 .env 模板失败 %s: %v", path, err)
		return false
	}
	return true
}

// ResolveEnvFilePath 依次找 .env：QBOT_ENV_FILE > 已存在的（当前目录 → 可执行文件旁 → 数据目录）
// > 在第一个可写目录里新建。go run 时不会落到临时构建目录里。
func ResolveEnvFilePath() string {
	if p := strings.TrimSpace(os.Getenv("QBOT_ENV_FILE")); p != "" {
		return p
	}
	dirs := ConfigSearchDirs()
	for _, d := range dirs {
		if p := filepath.Join(d, ".env"); FileExists(p) {
			return p
		}
	}
	for _, d := range dirs {
		if WritableDir(d) {
			return filepath.Join(d, ".env")
		}
	}
	return filepath.Join(DataDir, ".env")
}

// Lookup 供其它包查询 .env 里的值（内部仍走大小写敏感的 get）。
func (d *DotEnv) Lookup(key string) (string, bool) { return d.get(key) }

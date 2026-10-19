package admin

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"LianT/internal/config"
)

// ============================================================================
// 管理页面的 HTML
//
//   * 真正的 .html 文件：internal/admin/web/admin.html（编辑器里能高亮/格式化）
//   * 编译时用 go:embed 打一份进去兜底，磁盘上找不到文件也能用
//   * 运行时优先读磁盘上的 admin.html（改完刷新即生效，不用重编译）：
//       LIANT_ADMIN_HTML
//         > 当前目录/admin.html            > 当前目录/internal/admin/web/admin.html（在源码目录里跑时）
//         > 可执行文件旁/admin.html
// ============================================================================

//go:embed web/admin.html
var HTMLEmbedded string

var (
	adminHTMLMu      sync.Mutex
	adminHTMLCache   string
	adminHTMLPath    string
	adminHTMLModTime time.Time
	adminHTMLSize    int64
	adminHTMLChecked time.Time
)

// AdminHTMLCandidates 按优先级列出磁盘上的候选位置（路径全部动态获取，不写死）。
func AdminHTMLCandidates() []string {
	var out []string
	if p := strings.TrimSpace(config.EnvOr("LIANT_ADMIN_HTML", "")); p != "" {
		out = append(out, p)
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		out = append(out,
			filepath.Join(wd, "admin.html"),
			filepath.Join(wd, "internal", "admin", "web", "admin.html"),
			filepath.Join(wd, "internal", "admin", "admin.html"),
		)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		out = append(out,
			filepath.Join(d, "admin.html"),
			filepath.Join(d, "internal", "admin", "web", "admin.html"),
			filepath.Join(d, "internal", "admin", "admin.html"),
		)
	}
	return out
}

// LoadAdminHTML 返回要输出的 HTML 和它的来源（文件路径或 "内置"）。
// 500ms 内不重复 stat；文件改了下次请求就是新的。
func LoadAdminHTML() (string, string) {
	adminHTMLMu.Lock()
	if time.Since(adminHTMLChecked) < 500*time.Millisecond && adminHTMLCache != "" {
		html, src := adminHTMLCache, adminHTMLPath
		adminHTMLMu.Unlock()
		return html, src
	}
	adminHTMLChecked = time.Now()
	adminHTMLMu.Unlock()

	for _, p := range AdminHTMLCandidates() {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if p == adminHTMLPath && st.ModTime().Equal(adminHTMLModTime) && st.Size() == adminHTMLSize && adminHTMLCache != "" {
			return adminHTMLCache, adminHTMLPath
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			continue
		}
		adminHTMLMu.Lock()
		adminHTMLCache, adminHTMLPath, adminHTMLModTime, adminHTMLSize = string(b), p, st.ModTime(), st.Size()
		adminHTMLMu.Unlock()
		return string(b), p
	}

	adminHTMLMu.Lock()
	adminHTMLCache, adminHTMLPath = HTMLEmbedded, "内置(internal/admin/web/admin.html)"
	adminHTMLMu.Unlock()
	return HTMLEmbedded, "内置(internal/admin/web/admin.html)"
}

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
// 管理页面的 HTML / CSS / JS
//
//   * 真正的文件：internal/admin/web/admin.html、admin.css、admin.js
//   * 编译时用 go:embed 各打一份进去兜底，磁盘上找不到文件也能用
//   * 运行时优先读磁盘（改完刷新即生效，不用重编译）：
//       LIANT_ADMIN_HTML
//         > 当前目录/admin.html            > 当前目录/internal/admin/web/admin.html（在源码目录里跑时）
//         > 可执行文件旁/admin.html
//     css/js 与命中的 html 同目录。
// ============================================================================

//go:embed web/admin.html
var HTMLEmbedded string

//go:embed web/admin.css
var CSSEmbedded string

//go:embed web/admin.js
var JSEmbedded string

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

// AdminAssetCandidates 返回 css/js 的磁盘候选位置：与 admin.html 同目录。
func AdminAssetCandidates(name string) []string {
	htmls := AdminHTMLCandidates()
	out := make([]string, 0, len(htmls))
	for _, p := range htmls {
		out = append(out, filepath.Join(filepath.Dir(p), name))
	}
	return out
}

// LoadAdminAsset 返回 css/js 的内容和来源（文件路径或 "内置"）。
// 磁盘上有就跟 HTML 一样热更新；没有就回落到编译进去的那份。
func LoadAdminAsset(name string) (string, string) {
	for _, p := range AdminAssetCandidates(name) {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil || len(b) == 0 {
			continue
		}
		return string(b), p
	}
	switch name {
	case "admin.css":
		return CSSEmbedded, "内置(internal/admin/web/admin.css)"
	case "admin.js":
		return JSEmbedded, "内置(internal/admin/web/admin.js)"
	}
	return "", ""
}

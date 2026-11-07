// Package webui 内嵌管理端的网页构建产物。
//
// 页面与 WSS 同源提供，浏览器打开时只需信任一次服务端证书，
// 之后 ws/v1/ws 连接就是同源连接，不需要额外配置。
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist 由 scripts/build-web.sh 生成；仓库里保留占位文件以便 go build 直接可用。
//
//go:embed all:dist
var assets embed.FS

// Handler 返回一个单页应用处理器：静态文件命中就直接返回，其余路径回落到 index.html。
func Handler() http.Handler {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(r.URL.Path)
		if clean == "/" || clean == "." {
			serveIndex(w, r, sub)
			return
		}
		if f, err := sub.Open(strings.TrimPrefix(clean, "/")); err == nil {
			f.Close()
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			files.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, sub)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

package admin

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"LianT/internal/bot"
	"LianT/internal/manager"
	"LianT/internal/media"
	"LianT/internal/qqbot"
)

// ServeMedia 取回一条最近消息的媒体内容。
//
// 浏览器只给 chat_id + msg（可选 bot_id）；任务进程按本地消息库校验归属，
// 再换成平台的临时引用。Telegram 的 file_path/token、QQ 的签名直链都不会出现在
// 响应里，接口也不能凭任意 file_id / URL 取文件。
func (a *Server) ServeMedia(w http.ResponseWriter, r *http.Request, rt *manager.Runtime, b *bot.Bot, qb *qqbot.Bot) {
	if r.Method != http.MethodGet {
		ApiErr(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	q := r.URL.Query()
	chatID := strings.TrimSpace(q.Get("chat_id"))
	msgID := strings.TrimSpace(q.Get("msg"))
	if chatID == "" || msgID == "" {
		ApiErr(w, http.StatusBadRequest, "需要 chat_id 和 msg")
		return
	}
	if rt == nil {
		ApiErr(w, http.StatusConflict, "还没有运行中的 Bot")
		return
	}

	var ref media.Ref
	var ok bool
	switch {
	case b != nil:
		ref, ok = b.LookupMedia(chatID, msgID)
	case qb != nil:
		ref, ok = qb.LookupMedia(chatID, msgID)
	}
	if !ok {
		ApiErr(w, http.StatusNotFound, "消息不在本地库里，或这条消息没有可取的媒体")
		return
	}
	if !ref.Fetchable() {
		ApiErr(w, http.StatusConflict, "这个媒体没有可下载内容（表情按 emoji 显示）")
		return
	}
	if ref.Size > media.MaxBytes {
		ApiErr(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("媒体 %d 字节，超过单次取回上限 %d 字节", ref.Size, media.MaxBytes))
		return
	}

	mark := func(st media.State) {
		if b != nil {
			b.MarkMediaState(chatID, msgID, st)
		} else if qb != nil {
			qb.MarkMediaState(chatID, msgID, st)
		}
	}

	var (
		rc   io.ReadCloser
		size int64
		err  error
	)
	if b != nil {
		rc, size, err = b.OpenMedia(ref)
	} else {
		rc, size, err = qb.OpenMedia(ref)
	}
	if err != nil {
		code, text, st := mediaError(err)
		if st != "" {
			mark(st)
		}
		ApiErr(w, code, text)
		return
	}
	defer rc.Close()
	if size > media.MaxBytes {
		ApiErr(w, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("媒体超过单次取回上限 %d 字节", media.MaxBytes))
		return
	}

	setMediaHeaders(w, ref, msgID, size)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, io.LimitReader(rc, media.MaxBytes+1)); err != nil {
		// 响应头已经发出，无法改成错误状态；断流让客户端自己报错即可。
		log.Printf("[!] 传输媒体中断 bot=%d chat=%s msg=%s: %v", rt.Rec.ID, chatID, msgID, err)
	}
}

// mediaError 把取回错误映射成 HTTP 状态与不泄露平台地址的说明。
func mediaError(err error) (int, string, media.State) {
	switch {
	case errors.Is(err, media.ErrGone):
		return http.StatusGone, "媒体已不存在或平台链接已失效", media.StateGone
	case errors.Is(err, media.ErrExpired):
		return http.StatusGone, "媒体引用已过期，请刷新后重试", media.StateExpired
	case errors.Is(err, media.ErrTooLarge):
		return http.StatusRequestEntityTooLarge,
			fmt.Sprintf("媒体超过单次取回上限 %d 字节", media.MaxBytes), ""
	case errors.Is(err, media.ErrNotFetchable):
		return http.StatusConflict, "这个媒体没有可下载内容", ""
	default:
		log.Printf("[!] 取回媒体失败: %v", err)
		return http.StatusBadGateway, "取回媒体失败：平台没有返回内容", ""
	}
}

func setMediaHeaders(w http.ResponseWriter, ref media.Ref, msgID string, size int64) {
	ct := mediaContentType(ref)
	name := mediaFileName(ref, msgID)
	disposition := "attachment"
	if inlineMedia(ref, ct) {
		disposition = "inline"
	}
	if cd := mime.FormatMediaType(disposition, map[string]string{"filename": name}); cd != "" {
		w.Header().Set("Content-Disposition", cd)
	} else {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// 媒体可能是 SVG/HTML：即使被当附件打开，也不让它在管理端源上执行脚本。
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "private, max-age=120")
	if size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
}

func mediaContentType(ref media.Ref) string {
	ct := strings.TrimSpace(ref.MIME)
	if ct == "" && strings.TrimSpace(ref.Name) != "" {
		if v := mime.TypeByExtension(strings.ToLower(filepath.Ext(ref.Name))); v != "" {
			ct = v
		}
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	return ct
}

func inlineMedia(ref media.Ref, contentType string) bool {
	if ref.Kind == media.KindFile {
		return false
	}
	base := contentType
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		base = mt
	}
	switch base {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/bmp":
		return true
	case "video/mp4", "video/webm", "video/quicktime":
		return ref.Kind == media.KindVideo || ref.Kind == media.KindAnimation || ref.Kind == media.KindVideoNote
	case "audio/ogg", "audio/mpeg", "audio/mp4", "audio/aac", "audio/wav", "audio/x-wav",
		"audio/webm", "audio/amr", "audio/silk":
		return true
	}
	return false
}

// mediaFileName 给下载一个可读名字；没有文件名时就生成一个稳定的占位名。
func mediaFileName(ref media.Ref, msgID string) string {
	if name := safeFileName(ref.Name); name != "" {
		return name
	}
	ext := ""
	if exts, _ := mime.ExtensionsByType(mediaContentType(ref)); len(exts) > 0 {
		ext = exts[0]
	}
	return "LianT-" + safeToken(msgID) + ext
}

// safeFileName 去掉路径分隔符与控制字符，防止 Content-Disposition 被注入。
func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	r := []rune(name)
	if len(r) > 120 {
		name = string(r[:120])
	}
	return name
}

func safeToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
		if b.Len() >= 40 {
			break
		}
	}
	if b.Len() == 0 {
		return "media"
	}
	return b.String()
}

// Package media 定义 LianT 内部共享的媒体词汇。
//
// 两个结构刻意分开：
//
//	Ref        任务进程持有的引用：含 Telegram file_id、QQ 附件直链；随消息落本机库，不进 API JSON；
//	Descriptor 投影给客户端的事实：类型、名称、大小与可取回状态，不含任何地址。
//
// 取回统一走管理端 GET /api/media：由任务进程按「会话 + 消息」查引用，
// 浏览器拿不到带 bot token 的 URL，接口也不会变成任意文件下载器。
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxBytes 是一次取回的上限。Telegram Bot API 本身只允许下载 20 MB；
// QQ 附件可能更大，这里统一封顶，避免一个请求把手机内存和流量吃满。
const MaxBytes int64 = 32 << 20

// State 是媒体引用「现在能不能取回」的已知状态。
//
// available 只表示引用可用（Telegram file_id 长期有效），不代表已经下载；
// unknown 表示还没验证过（QQ 的签名直链会过期，取回前无法确认）。
type State string

const (
	StateAvailable State = "available"
	StateExpired   State = "expired"
	StateGone      State = "gone"
	StateUnknown   State = "unknown"
)

// NormalizeState 把空值/未知值归到 unknown，避免下游把空字符串当成功。
func NormalizeState(s State) State {
	switch s {
	case StateAvailable, StateExpired, StateGone, StateUnknown:
		return s
	default:
		return StateUnknown
	}
}

// 归一化的媒体种类，跨平台共用。前端只按这些值分支渲染。
const (
	KindImage     = "image"
	KindVideo     = "video"
	KindAudio     = "audio"
	KindVoice     = "voice"
	KindFile      = "file"
	KindSticker   = "sticker"
	KindAnimation = "animation"
	KindVideoNote = "video_note"
)

// 取回失败的可判定原因；平台层负责把各自的错误翻译成这些值，
// 管理端只做 HTTP 状态映射，不解析平台错误文本。
var (
	ErrNotFetchable = errors.New("这个媒体没有可下载内容")
	ErrTooLarge     = errors.New("媒体超过单次取回上限")
	ErrExpired      = errors.New("媒体引用已过期")
	ErrGone         = errors.New("媒体已不存在")
)

// Ref 是任务进程内部持有的媒体引用。
//
// 带 JSON tag 是为了持久化到每个 Bot 的本地 SQLite：重启后 Telegram 仍能用
// file_id 取回；QQ 的签名直链过期后会由取回失败把状态改成 gone。
// Ref 本身不进任何对外 JSON，客户端只看到 Descriptor。
type Ref struct {
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration int    `json:"duration,omitempty"` // 秒
	Emoji    string `json:"emoji,omitempty"`

	// FileID 是 Telegram 的 file_id，只在进程内与本地库使用。
	FileID string `json:"file_id,omitempty"`
	// URL 是 QQ 附件签名直链，只在进程内与本地库使用，且随时可能失效。
	URL string `json:"url,omitempty"`

	State State `json:"state,omitempty"`
}

// Fetchable 表示是否真的有可取回的字节。
// 表情（sticker）只投影 emoji，没有可下载内容，不算可取回。
func (r Ref) Fetchable() bool { return r.FileID != "" || r.URL != "" }

// Descriptor 把内部引用投影成客户端可见的事实。
func (r Ref) Descriptor() *Descriptor {
	if strings.TrimSpace(r.Kind) == "" {
		return nil
	}
	return &Descriptor{
		Kind:      r.Kind,
		Name:      r.Name,
		MIME:      r.MIME,
		Size:      r.Size,
		Width:     r.Width,
		Height:    r.Height,
		Duration:  r.Duration,
		Emoji:     r.Emoji,
		State:     NormalizeState(r.State),
		Fetchable: r.Fetchable(),
	}
}

// Descriptor 是消息投影里的媒体描述：只有展示所需的事实，没有地址。
type Descriptor struct {
	Kind      string `json:"kind"`
	Name      string `json:"name,omitempty"`
	MIME      string `json:"mime,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Duration  int    `json:"duration,omitempty"`
	Emoji     string `json:"emoji,omitempty"`
	State     State  `json:"state"`
	Fetchable bool   `json:"fetchable"`
}

// OpenHTTP 取回一个平台提供的附件 URL（目前用于 QQ）。
//
// 这里是代理层，不是通用下载器：只允许 http/https、拒绝带凭据的 URL、
// 拒绝指向本机/内网的地址，并对重定向做同样的校验。
func OpenHTTP(raw string, limit int64) (io.ReadCloser, int64, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, 0, fmt.Errorf("附件地址不合法: %w", err)
	}
	if err := checkRemoteURL(u); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, errors.New("构造附件请求失败")
	}
	req.Header.Set("User-Agent", "LianT")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, errors.New("取回附件失败")
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusGone:
		_ = resp.Body.Close()
		return nil, 0, fmt.Errorf("%w（HTTP %d）", ErrGone, resp.StatusCode)
	default:
		_ = resp.Body.Close()
		return nil, 0, fmt.Errorf("取回附件失败(HTTP %d)", resp.StatusCode)
	}
	if limit > 0 && resp.ContentLength > limit {
		_ = resp.Body.Close()
		return nil, 0, ErrTooLarge
	}
	return resp.Body, resp.ContentLength, nil
}

var httpClient = &http.Client{
	Timeout: 5 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("附件重定向次数过多")
		}
		return checkRemoteURL(req.URL)
	},
}

// checkRemoteURL 是尽力而为的 SSRF 防护：拦掉 URL 里写死的内网地址，
// 并解析域名后检查。DNS 重绑定仍可能在解析与连接之间发生，不作为唯一防线。
func checkRemoteURL(u *url.URL) error {
	if u == nil {
		return errors.New("附件地址为空")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("附件地址只支持 http/https")
	}
	if u.User != nil {
		return errors.New("附件地址不能带凭据")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return errors.New("附件地址缺少主机名")
	}
	if unsafeHostname(host) {
		return errors.New("附件地址指向本机或内网，已拒绝")
	}
	if ip := net.ParseIP(host); ip != nil {
		if unsafeIP(ip) {
			return errors.New("附件地址指向内网地址，已拒绝")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		// 解析失败交给真正的请求去暴露错误；这里不把 DNS 故障当成安全问题。
		return nil
	}
	for _, ip := range ips {
		if unsafeIP(ip) {
			return errors.New("附件地址解析到内网地址，已拒绝")
		}
	}
	return nil
}

func unsafeHostname(host string) bool {
	return host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal")
}

func unsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

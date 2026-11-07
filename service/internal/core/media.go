package core

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

const (
	defaultMediaDir      = "data/media"
	defaultMaxMediaBytes = 100 << 20
	mediaURLTTL          = 10 * time.Minute
	mediaDownloadTimeout = 60 * time.Second
)

type mediaConfig struct {
	dir      string
	baseURL  string
	secret   []byte
	client   *http.Client
	maxBytes int64
}

func (c *Core) initMedia() error {
	cfg := mediaConfig{
		dir:      c.opt.MediaDir,
		baseURL:  strings.TrimSuffix(c.opt.MediaURLBase, "/"),
		secret:   c.opt.MediaSecret,
		client:   c.opt.HTTPClient,
		maxBytes: c.opt.MaxMediaBytes,
	}
	if cfg.dir == "" {
		cfg.dir = defaultMediaDir
	}
	if cfg.client == nil {
		cfg.client = &http.Client{Timeout: mediaDownloadTimeout}
	}
	if cfg.maxBytes <= 0 {
		cfg.maxBytes = defaultMaxMediaBytes
	}
	if len(cfg.secret) == 0 {
		cfg.secret = make([]byte, 32)
		if _, err := rand.Read(cfg.secret); err != nil {
			return fmt.Errorf("core: 生成媒体签名密钥: %w", err)
		}
	}
	if err := os.MkdirAll(cfg.dir, 0o700); err != nil {
		return fmt.Errorf("core: 创建媒体目录: %w", err)
	}
	c.media = cfg
	return nil
}

// downloadMedia 把入站附件落到本地，并登记为 ready。
//
// 失败只记录不重试：平台链接通常有时效，消息仍在库里，客户端看到的是占位。
func (c *Core) downloadMedia(botID, msgID string, atts []platform.Attachment) {
	for idx, att := range atts {
		path, size, mime, err := c.fetchToFile(botID, msgID, idx, att)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = c.opt.Store.PutMedia(ctx, store.Media{
			BotID: botID,
			MsgID: msgID,
			Index: idx,
			MIME:  mime,
			Path:  path,
			Size:  size,
			State: store.MediaReady,
		})
		cancel()
		if err != nil {
			_ = os.Remove(path)
		}
	}
}

func (c *Core) fetchToFile(botID, msgID string, idx int, att platform.Attachment) (string, int64, string, error) {
	req, err := http.NewRequest(http.MethodGet, att.URL, nil)
	if err != nil {
		return "", 0, "", err
	}
	resp, err := c.media.client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("core: 下载媒体状态码 %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, c.media.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", 0, "", err
	}
	if int64(len(data)) > c.media.maxBytes {
		return "", 0, "", fmt.Errorf("core: 媒体超过 %d 字节上限", c.media.maxBytes)
	}

	mime := att.MIME
	if mime == "" {
		mime = resp.Header.Get("Content-Type")
	}
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i]
	}
	dir := filepath.Join(c.media.dir, safeSegment(botID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, "", err
	}
	path := filepath.Join(dir, safeSegment(msgID)+"_"+strconv.Itoa(idx)+extensionFor(mime))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", 0, "", err
	}
	return path, int64(len(data)), mime, nil
}

// FetchMedia 返回本地媒体的下载地址；地址带过期时间与 HMAC 签名。
func (c *Core) FetchMedia(ctx context.Context, _ server.Session, raw json.RawMessage) (any, *protocol.Error) {
	var req protocol.MediaFetch
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, "media.fetch 字段不合法")
	}
	if !platform.IsValidID(req.BotID) || req.MsgID == "" {
		return nil, bad(protocol.CodeInvalidPayload, "media.fetch 需要 bot_id 与 msg_id")
	}
	items, err := c.opt.Store.MediaOf(ctx, req.BotID, req.MsgID)
	if err != nil {
		return nil, internal(err)
	}
	for _, item := range items {
		if item.State != store.MediaReady || item.Path == "" {
			continue
		}
		exp := c.now().Add(mediaURLTTL)
		return protocol.MediaFetchOK{
			URL:  c.mediaURL(req.BotID, req.MsgID, item.Index, exp),
			Exp:  exp.Unix(),
			Size: item.Size,
			MIME: item.MIME,
		}, nil
	}
	return nil, bad(protocol.CodeNotFound, "媒体不存在或已被清理")
}

func (c *Core) mediaURL(botID, msgID string, idx int, exp time.Time) string {
	path := "/v1/media/" + url.PathEscape(botID) + "/" + url.PathEscape(msgID) + "/" + strconv.Itoa(idx)
	expiry := exp.Unix()
	sig := c.signMedia(path, expiry)
	return fmt.Sprintf("%s%s?exp=%d&sig=%s", c.media.baseURL, path, expiry, sig)
}

func (c *Core) signMedia(path string, exp int64) string {
	mac := hmac.New(sha256.New, c.media.secret)
	fmt.Fprintf(mac, "%s|%d", path, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// MediaHandler 提供 /v1/media/{bot}/{msg}/{idx} 的下载，签名过期或文件缺失都返回 404。
func (c *Core) MediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/media/"), "/")
		if len(parts) != 3 {
			http.NotFound(w, r)
			return
		}
		botID, err1 := url.PathUnescape(parts[0])
		msgID, err2 := url.PathUnescape(parts[1])
		idx, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			http.NotFound(w, r)
			return
		}
		exp, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
		if err != nil || c.now().Unix() > exp {
			http.NotFound(w, r)
			return
		}
		path := "/v1/media/" + parts[0] + "/" + parts[1] + "/" + parts[2]
		if !hmac.Equal([]byte(r.URL.Query().Get("sig")), []byte(c.signMedia(path, exp))) {
			http.NotFound(w, r)
			return
		}
		items, err := c.opt.Store.MediaOf(r.Context(), botID, msgID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		for _, item := range items {
			if item.Index != idx || item.State != store.MediaReady || item.Path == "" {
				continue
			}
			if item.MIME != "" {
				w.Header().Set("Content-Type", item.MIME)
			}
			w.Header().Set("Cache-Control", "private, max-age=600")
			http.ServeFile(w, r, item.Path)
			return
		}
		http.NotFound(w, r)
	})
}

func safeSegment(s string) string {
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", "..", "_")
	out := replacer.Replace(s)
	if out == "" {
		return "unknown"
	}
	return out
}

func extensionFor(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "audio/silk", "audio/amr":
		return ".amr"
	case "video/mp4":
		return ".mp4"
	default:
		return ".bin"
	}
}

package botruntime

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	tele "gopkg.in/telebot.v4"

	"github.com/LensnTeam/LianT/service/config"
	"github.com/LensnTeam/LianT/service/crypto"
	"github.com/LensnTeam/LianT/service/wss"
)

func getTokenByID(id int64) (string, error) {
	cfg := config.LoadConfig()
	db, err := sql.Open(
		"sqlite",
		cfg.Storage.Database,
	)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var encrypted string
	err = db.QueryRow(
		`
SELECT token
FROM bots
WHERE bot_id = ?
`,
		id,
	).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New(
			"Bot不存在",
		)
	}
	if err != nil {
		return "", err
	}

	return crypto.DecryptToken(
		encrypted,
	), nil
}

func Run(
	strid string,
	ws *wss.Server,
) {
	id, err := strconv.ParseInt(strid, 10, 64)
	if err != nil {
		fmt.Println("不是合法数字:", err)
		return
	}
	token, err := getTokenByID(id)
	if err != nil {
		fmt.Println(err)
		return
	}

	cfg := config.LoadConfig()
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		URL:    cfg.Connect.Url,
	}
	if cfg.Proxy.Enabled {
		proxy, pErr := url.Parse(cfg.Proxy.URL)
		if pErr != nil {
			fmt.Fprintln(os.Stderr, "代理地址错误:", pErr)
			return
		}
		pref.Client = &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxy),
			},
		}
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		fmt.Fprintln(os.Stderr, "创建 Bot 失败:", err)
		return
	}
	cache := newChatCache()
	b.Handle("/start", func(c tele.Context) error {
		cache.remember(c.Chat())
		wsssend(ws, wrapMessage(id, c, c.Text()))
		return c.Send("欢迎使用本 Bot")
	})
	b.Handle(tele.OnText, func(c tele.Context) error {
		if strings.HasPrefix(c.Text(), "/") {
			cache.remember(c.Chat())
			sender := c.Sender()
			name := "未知用户"
			if sender != nil && sender.Username != "" {
				name = sender.Username
			}
			wsssend(ws, wrapMessage(id, c, name+"发送了不存在的指令"))
			return c.Send("指令不存在")
		}
		cache.remember(c.Chat())
		wsssend(ws, wrapMessage(id, c, c.Text()))
		return nil
	})
	// 非文本消息：遍历 mediaSupport 统一注册。有文件换下载链接转发 client，
	// 无文件/无法识别（位置/联系人/未知）则答复蓝字提示并按普通文本下行。
	for _, m := range mediaSupport {
		m := m // 捕获循环变量
		b.Handle(m.endpoint, func(c tele.Context) error {
			cache.remember(c.Chat())
			forwardMedia(b, id, cache, ws, c, m.name)
			return nil
		})
	}
	go func() {
		for msg := range ws.Receive {
			handleWSMessage(b, id, cache, msg)
		}
	}()
	b.Start()
}

func wsssend(s *wss.Server, msg string) {
	s.Send(msg)
}

type wsElement struct {
	BotID   string `json:"bot_id,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Type    string `json:"type,omitempty"`
	Content string `json:"content,omitempty"`
}

type chatCache struct {
	mu    sync.Mutex
	chats map[int64]*tele.Chat
}

func newChatCache() *chatCache {
	return &chatCache{chats: make(map[int64]*tele.Chat)}
}

func (cc *chatCache) remember(chat *tele.Chat) {
	if chat == nil {
		return
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.chats[chat.ID] = chat
}

func (cc *chatCache) get(uid int64) *tele.Chat {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.chats[uid]
}

func wrapElement(key, val string) wsElement {
	switch key {
	case "bot_id":
		return wsElement{BotID: val}
	case "user_id":
		return wsElement{UserID: val}
	case "type":
		return wsElement{Type: val}
	default:
		return wsElement{Content: val}
	}
}

func encodePayload(elems []wsElement) string {
	inner, err := json.Marshal(elems)
	if err != nil {
		fmt.Fprintln(os.Stderr, "编码下行消息失败:", err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(inner)
}

// wrapMessage 生成一条下行的文本消息，text 需为明文。
// 统一了原先重复的 wrapOutgoing / wrapOutgoingText。
func wrapMessage(botID int64, c tele.Context, text string) string {
	uid := int64(0)
	if s := c.Sender(); s != nil {
		uid = s.ID
	}
	return encodePayload([]wsElement{
		wrapElement("bot_id", strconv.FormatInt(botID, 10)),
		wrapElement("user_id", strconv.FormatInt(uid, 10)),
		wrapElement("type", "text"),
		wrapElement("content", base64.StdEncoding.EncodeToString([]byte(text))),
	})
}

// wrapOutgoingMedia 生成下行消息：媒体的一次性代理下载链接推给 client。
// type=媒体类型(photo/video/...)，content=base64(代理下载链接)。
func wrapOutgoingMedia(botID int64, c tele.Context, msgType string, dlURL string) string {
	uid := int64(0)
	if s := c.Sender(); s != nil {
		uid = s.ID
	}
	return encodePayload([]wsElement{
		wrapElement("bot_id", strconv.FormatInt(botID, 10)),
		wrapElement("user_id", strconv.FormatInt(uid, 10)),
		wrapElement("type", msgType),
		wrapElement("content", base64.StdEncoding.EncodeToString([]byte(dlURL))),
	})
}

// mediaSpec 描述一种 bot 支持的非文本消息类型。
//   - name：下行推给 client 的 type 字段值（与 client 端保持一致）
//   - endpoint：telebot 注册端点
//   - extract：从消息里取出文件；无文件的类型返回 nil
//
// 需要新增/调整支持的类型时，只改这个 list 即可，注册和识别逻辑复用。
type mediaSpec struct {
	name     string
	endpoint string
	extract  func(m *tele.Message) *tele.File
}

func photoFile(m *tele.Message) *tele.File {
	if m.Photo != nil {
		return &m.Photo.File
	}
	return nil
}
func videoFile(m *tele.Message) *tele.File {
	if m.Video != nil {
		return &m.Video.File
	}
	return nil
}
func audioFile(m *tele.Message) *tele.File {
	if m.Audio != nil {
		return &m.Audio.File
	}
	return nil
}
func voiceFile(m *tele.Message) *tele.File {
	if m.Voice != nil {
		return &m.Voice.File
	}
	return nil
}
func documentFile(m *tele.Message) *tele.File {
	if m.Document != nil {
		return &m.Document.File
	}
	return nil
}
func videoNoteFile(m *tele.Message) *tele.File {
	if m.VideoNote != nil {
		return &m.VideoNote.File
	}
	return nil
}
func stickerFile(m *tele.Message) *tele.File {
	if m.Sticker != nil {
		return &m.Sticker.File
	}
	return nil
}
func animationFile(m *tele.Message) *tele.File {
	if m.Animation != nil {
		return &m.Animation.File
	}
	return nil
}

// mediaSupport 是 bot 支持的所有非文本消息类型清单。
var mediaSupport = []mediaSpec{
	{"photo", tele.OnPhoto, photoFile},
	{"video", tele.OnVideo, videoFile},
	{"audio", tele.OnAudio, audioFile},
	{"voice", tele.OnVoice, voiceFile},
	{"document", tele.OnDocument, documentFile},
	{"video_note", tele.OnVideoNote, videoNoteFile},
	{"sticker", tele.OnSticker, stickerFile},
	{"animation", tele.OnAnimation, animationFile},
	{"location", tele.OnLocation, func(*tele.Message) *tele.File { return nil }},
	{"contact", tele.OnContact, func(*tele.Message) *tele.File { return nil }},
}

// extractMediaFile 按类型名从消息里取文件；无文件/无法识别返回 nil。
func extractMediaFile(name string, m *tele.Message) *tele.File {
	for _, s := range mediaSupport {
		if s.name == name && m != nil {
			return s.extract(m)
		}
	}
	return nil
}

// fileDownloadURL 用 file_id 调 Telegram API 换取 file_path，拼出真实可下载的地址。
// 注意：真实地址里含 bot token，仅供服务端使用，永不直接下发给客户端。
// 对外请通过 wss.RegisterFile 换取代理地址。
func fileDownloadURL(b *tele.Bot, file *tele.File) (string, error) {
	f, err := b.FileByID(file.FileID)
	if err != nil {
		return "", err
	}
	if f.FilePath == "" {
		return "", errors.New("未获取到文件路径")
	}
	cfg := config.LoadConfig()
	base := cfg.Connect.Url
	if base == "" {
		base = "https://api.telegram.org"
	}
	return fmt.Sprintf(
		"%s/file/bot%s/%s",
		strings.TrimRight(base, "/"),
		b.Token,
		f.FilePath,
	), nil
}

// mediaHTTPClient 构造用于代理下载媒体的 HTTP 客户端，遵循配置里的代理设置。
func mediaHTTPClient() *http.Client {
	cfg := config.LoadConfig()
	cl := &http.Client{}
	if cfg.Proxy.Enabled {
		proxy, err := url.Parse(cfg.Proxy.URL)
		if err != nil {
			return cl
		}
		cl.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
	}
	return cl
}

// unrecognizedReply 在 Telegram 端答复一句蓝字自动回复提示。
// Telegram 的 HTML 解析没有直接的颜色参数，用"空链接"把 [自动回复] 染成蓝字，
// 这是 Bot API 常用来做彩色文字的技巧。前面加无干扰控制字符避免被当作纯链接折叠。
func unrecognizedReply(c tele.Context) {
	const hint = "\u200b" // 零宽空格
	_ = c.Send(
		`<a href="https://t.me">`+hint+`[自动回复]</a>该消息类型无法被识别`,
		tele.ModeHTML,
	)
}

// forwardMedia 处理一条非文本消息，name 是其在 mediaSupport 里的类型名：
//   - 有文件（photo/video/audio/...）：file_id 换下载链接，按媒体格式转发 client；
//   - 无文件/无法识别（location/contact/未知）：Telegram 端答复蓝字提示，
//     下行用"普通文本"格式转发，content=base64("该消息类型无法被识别")。
func forwardMedia(b *tele.Bot, botID int64, cc *chatCache, ws *wss.Server, c tele.Context, name string) {
	cc.remember(c.Chat())
	file := extractMediaFile(name, c.Message())
	if file == nil {
		// 无法识别：答复蓝字提示 + 下行按普通文本转发，不传媒体 base64。
		unrecognizedReply(c)
		wsssend(ws, wrapMessage(botID, c, "该消息类型无法被识别"))
		return
	}
	dlURL, err := fileDownloadURL(b, file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "获取文件下载链接失败:", err)
		return
	}
	// 用一次性代理地址对外下发，避免暴露真实地址里的 bot token。
	proxied, err := ws.RegisterFile(dlURL, mediaHTTPClient())
	if err != nil {
		fmt.Fprintln(os.Stderr, "登记媒体代理失败:", err)
		return
	}
	wsssend(ws, wrapOutgoingMedia(botID, c, name, proxied))
}

func decodeIncoming(raw string) ([]map[string]json.RawMessage, error) {
	var b []byte
	if d, err := base64.StdEncoding.DecodeString(raw); err == nil {
		b = d
	} else {
		b = []byte(raw)
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(b, &arr); err != nil {
		var single map[string]json.RawMessage
		if e2 := json.Unmarshal(b, &single); e2 == nil {
			arr = []map[string]json.RawMessage{single}
			return arr, nil
		}
		return nil, err
	}
	return arr, nil
}

func getKey(arr []map[string]json.RawMessage, key string) (string, bool) {
	for _, obj := range arr {
		if v, ok := obj[key]; ok && len(v) > 0 {
			var s string
			if json.Unmarshal(v, &s) == nil {
				return s, true
			}
			return string(bytes.Trim(v, `"`)), true
		}
	}
	return "", false
}

func handleWSMessage(b *tele.Bot, botID int64, cc *chatCache, raw string) {
	arr, err := decodeIncoming(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "解析客户端消息失败:", err)
		return
	}
	userID, _ := getKey(arr, "user_id")
	content, _ := getKey(arr, "content")
	if userID == "" {
		fmt.Fprintln(os.Stderr, "客户端消息缺少 user_id")
		return
	}
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "user_id 不是合法数字:", userID)
		return
	}
	if content == "" {
		fmt.Fprintln(os.Stderr, "客户端消息缺少 content, 无法发送空文本 uid=", uid)
		return
	}
	plain, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "内层 base64 解码失败:", err)
		return
	}
	if len(plain) == 0 {
		fmt.Fprintln(os.Stderr, "content 解码后为空文本 uid=", uid)
		return
	}
	recipient := cc.get(uid)
	if recipient == nil {
		recipient = &tele.Chat{ID: uid, Type: tele.ChatPrivate}
	}
	if _, err := b.Send(recipient, string(plain)); err != nil {
		fmt.Fprintln(os.Stderr, "发送给用户失败 uid="+strconv.FormatInt(uid, 10)+":", err)
		fmt.Fprintln(os.Stderr, "提示: 若目标用户未先主动给本 bot 发过消息(/start), Telegram 禁止 bot 主动私聊(403 can't initiate conversation)。请先让该用户在 Telegram 里给 bot 发一条消息。")
		return
	}
	fmt.Fprintln(os.Stderr, "已转发给用户:", uid)
}

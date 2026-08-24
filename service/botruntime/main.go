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
		Token: token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		URL: cfg.Connect.Url,
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
		wsssend(ws, wrapOutgoing(id, c))
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
			wsssend(ws, wrapOutgoingText(id, c, name+"发送了不存在的指令"))
			return c.Send("指令不存在")
		}
		cache.remember(c.Chat())
		wsssend(ws, wrapOutgoing(id, c))
		return nil
	})
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
	BotID string `json:"bot_id,omitempty"`
	UserID string `json:"user_id,omitempty"`
	Type string `json:"type,omitempty"`
	Content string `json:"content,omitempty"`
}

type chatCache struct {
	mu sync.Mutex
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
	inner, _ := json.Marshal(elems)
	return base64.StdEncoding.EncodeToString(inner)
}

func wrapOutgoing(botID int64, c tele.Context) string {
	uid := int64(0)
	if s := c.Sender(); s != nil {
		uid = s.ID
	}
	text := c.Text()
	return encodePayload([]wsElement{
		wrapElement("bot_id", strconv.FormatInt(botID, 10)),
		wrapElement("user_id", strconv.FormatInt(uid, 10)),
		wrapElement("type", "text"),
		wrapElement("content", base64.StdEncoding.EncodeToString([]byte(text))),
	})
}

func wrapOutgoingText(botID int64, c tele.Context, text string) string {
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

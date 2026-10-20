package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// QQ 机器人 API 与网关协议结构
// 这份文件属于业务层，只在任务进程里被调用。
// ============================================================================

var HTTPClient = &http.Client{Timeout: 15 * time.Second}

type AccessTokenResp struct {
	AccessToken string          `json:"access_token"`
	ExpiresIn   json.RawMessage `json:"expires_in"` // 可能是 "7200" 也可能是 7200
}

// Expires 兼容 expires_in 的字符串/数字两种返回。
func (r AccessTokenResp) Expires() time.Duration {
	s := strings.Trim(strings.TrimSpace(string(r.ExpiresIn)), `"`)
	if s == "" {
		return 0
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 0
}

type GatewayResp struct {
	URL string `json:"url"`
}

// Payload 是网关上下行的统一信封。
type Payload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int             `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

type HelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type IdentifyData struct {
	Token   string            `json:"token"`
	Intents int               `json:"intents"`
	Shard   []int             `json:"shard"`
	Props   map[string]string `json:"properties,omitempty"`
}

type Author struct {
	ID           string `json:"id"`
	MemberOpenID string `json:"member_openid"`
	UserOpenID   string `json:"user_openid"`
	UnionOpenID  string `json:"union_openid"`
	Username     string `json:"username"`
	MemberRole   string `json:"member_role"` // member | admin | owner
	Bot          bool   `json:"bot"`
}

// MentionedUser 是消息里被 @ 的人（官方事件 mentions 字段，不含 @机器人自身）。
type MentionedUser struct {
	ID           string `json:"id"`
	MemberOpenID string `json:"member_openid"`
	Username     string `json:"username"`
	MemberRole   string `json:"member_role"`
	Bot          bool   `json:"bot"`
}

// MsgElement 是消息元素（message_type=103 表示引用消息，Author 是被引用消息的发送者）。
type MsgElement struct {
	MsgIdx      string       `json:"msg_idx"`
	Author      Author       `json:"author"`
	MessageType int          `json:"message_type"`
	Content     string       `json:"content"`
	MsgElements []MsgElement `json:"msg_elements"`
}

// MessageScene 消息场景：ext 里可能有 msg_idx / ref_msg_idx（引用）/ auth_token。
type MessageScene struct {
	Source string   `json:"source"`
	Ext    []string `json:"ext"`
}

func (m MessageScene) Get(key string) string {
	for _, kv := range m.Ext {
		if strings.HasPrefix(kv, key+"=") {
			return strings.TrimPrefix(kv, key+"=")
		}
	}
	return ""
}

// Attachment 是群/单聊消息里的附件（图片、语音、视频、文件）。
// url 是平台给的签名直链，会过期；只在任务进程内使用，不投影给浏览器。
type Attachment struct {
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	Height      int    `json:"height"`
	Width       int    `json:"width"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
	ID          string `json:"id"`
}

type MessageData struct {
	ID           string          `json:"id"`
	Content      string          `json:"content"`
	GroupOpenID  string          `json:"group_openid"`
	MsgSeq       int             `json:"msg_seq"`
	Timestamp    string          `json:"timestamp"`
	MessageType  int             `json:"message_type"`
	Author       Author          `json:"author"`
	Mentions     []MentionedUser `json:"mentions"`
	Attachments  []Attachment    `json:"attachments"`
	MessageScene MessageScene    `json:"message_scene"`
	MsgElements  []MsgElement    `json:"msg_elements"`
}

// QuotedAuthor 从消息元素里找出"引用消息"的作者（type=103），拿不到就返回 false。
func (d MessageData) QuotedAuthor() (Author, bool) {
	var walk func(list []MsgElement) (Author, bool)
	walk = func(list []MsgElement) (Author, bool) {
		for _, e := range list {
			if e.MessageType == 103 && (e.Author.MemberOpenID != "" || e.Author.UserOpenID != "") {
				return e.Author, true
			}
			if len(e.MsgElements) > 0 {
				if a, ok := walk(e.MsgElements); ok {
					return a, true
				}
			}
		}
		return Author{}, false
	}
	return walk(d.MsgElements)
}

func MustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// FetchAccessToken 取 access_token，同时返回有效期。
func FetchAccessToken(id, secret string) (string, time.Duration, error) {
	body, _ := json.Marshal(map[string]string{
		"appId":        id,
		"clientSecret": secret,
	})
	resp, err := HTTPClient.Post(TokenURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var r AccessTokenResp
	if err := json.Unmarshal(b, &r); err != nil || r.AccessToken == "" {
		return "", 0, fmt.Errorf("获取token失败(HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return r.AccessToken, r.Expires(), nil
}

func FetchGateway(token string) (string, error) {
	req, err := http.NewRequest("GET", GatewayURL(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var r GatewayResp
	if err := json.Unmarshal(b, &r); err != nil || r.URL == "" {
		return "", fmt.Errorf("获取网关失败(HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return r.URL, nil
}

// SendResult 是一次发消息请求的结果。
// 网页后台需要把 QQ 的错误原样展示给管理员，所以这里不只返回 message_id。
type SendResult struct {
	ID      string
	Status  int
	Code    int
	Message string
}

// Ok 表示服务端明确接受了消息并返回了消息 ID。
func (r SendResult) Ok() bool {
	return r.Status >= 200 && r.Status < 300 && r.Code == 0 && r.ID != ""
}

// PostMessageResult 发一条消息并解析响应里的 id / code / message。
func PostMessageResult(token, url string, body map[string]interface{}) SendResult {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return SendResult{Message: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return SendResult{Message: "请求失败: " + err.Error()}
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	log.Printf("[API] HTTP %d %s", resp.StatusCode, clip(string(rb), 300))
	var out struct {
		ID      string      `json:"id"`
		Code    json.Number `json:"code"`
		Message string      `json:"message"`
	}
	_ = json.Unmarshal(rb, &out)
	r := SendResult{ID: out.ID, Status: resp.StatusCode, Message: out.Message}
	if out.Code != "" {
		if n, err := strconv.Atoi(out.Code.String()); err == nil {
			r.Code = n
		}
	}
	if strings.TrimSpace(r.Message) == "" {
		r.Message = strings.TrimSpace(string(rb))
	}
	return r
}

// PostMessage 发消息；失败只写日志，不 panic、不影响连接。
// PostMessageID 发一条消息并解析响应里的消息 ID（撤回自己发的消息要用它）。
func PostMessageID(token, url string, body map[string]interface{}) string {
	return PostMessageResult(token, url, body).ID
}

func PostMessage(token, url string, body map[string]interface{}) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		log.Printf("[X] 构造请求失败: %v", err)
		return
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		log.Printf("[X] 发送失败: %v", err)
		return
	}
	defer resp.Body.Close()

	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	log.Printf("[API] HTTP %d %s", resp.StatusCode, clip(string(rb), 300))
}

// SendGroupText 发群文本消息，返回完整结果。
func SendGroupText(token, groupOpenID, content, msgID string, msgSeq int) SendResult {
	m := map[string]interface{}{"content": content, "msg_type": 0}
	if msgID != "" {
		m["msg_id"] = msgID
		m["msg_seq"] = msgSeq
	}
	return PostMessageResult(token, GroupMessageURL(groupOpenID), m)
}

// SendGroupMessage 发群消息。msgID 非空=被动回复（5 分钟内有效，最多 5 次）；
// msgSeq 与 msgID 搭配，避免同一条消息重复发送被去重。
// send 返回服务端给的 message_id（拿不到就返回空串）。
func SendGroupMessage(token, groupOpenID, content, msgID string, msgSeq int) string {
	return SendGroupText(token, groupOpenID, content, msgID, msgSeq).ID
}

// SendGroupMarkdown 发群 Markdown 卡片（msg_type=2）。
// QQ 群聊已对全部机器人开放自定义 Markdown，无需模板；content 必须为空。
func SendGroupMarkdown(token, groupOpenID, markdown, msgID string, msgSeq int) SendResult {
	m := map[string]interface{}{
		"msg_type": 2,
		"markdown": map[string]interface{}{"content": markdown},
	}
	if msgID != "" {
		m["msg_id"] = msgID
		m["msg_seq"] = msgSeq
	}
	return PostMessageResult(token, GroupMessageURL(groupOpenID), m)
}

// SendC2CText 发单聊消息，返回完整结果。
func SendC2CText(token, userOpenID, content, msgID string, msgSeq int) SendResult {
	m := map[string]interface{}{"content": content, "msg_type": 0}
	if msgID != "" {
		m["msg_id"] = msgID
		m["msg_seq"] = msgSeq
	}
	return PostMessageResult(token, C2cMessageURL(userOpenID), m)
}

// SendC2CMessage 发单聊消息，返回服务端给的 message_id。
func SendC2CMessage(token, userOpenID, content, msgID string, msgSeq int) string {
	return SendC2CText(token, userOpenID, content, msgID, msgSeq).ID
}

// clip 把响应体截断到 n 个字符再写日志：整段响应可能有几百 KB，
// 也可能带着不该落盘的字段。
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

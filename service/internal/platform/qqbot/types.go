package qqbot

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Bailensn/LianT/service/internal/platform"
)

const (
	defaultTokenURL   = "https://bots.qq.com/app/getAppAccessToken"
	defaultAPIBase    = "https://api.sgroup.qq.com"
	defaultSandboxAPI = "https://sandbox.api.sgroup.qq.com"
)

// QQ 机器人的 WebSocket 操作码。
const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatAck   = 11
)

// 只订阅群/单聊与公域频道 @ 消息，其余事件不需要。
const (
	intentGroupAndC2C = 1 << 25
	intentPublicGuild = 1 << 30
)

type credentials struct {
	appID   string
	secret  string
	sandbox bool
}

func parseCredentials(c platform.Credentials) (credentials, error) {
	if len(c) == 0 {
		return credentials{}, errMissingCredentials
	}
	appID := strings.TrimSpace(c["app_id"])
	secret := strings.TrimSpace(c["app_secret"])
	if appID == "" || secret == "" {
		return credentials{}, errMissingCredentials
	}
	sandbox := strings.EqualFold(c["sandbox"], "true") || c["sandbox"] == "1"
	return credentials{appID: appID, secret: secret, sandbox: sandbox}, nil
}

// flexInt 兼容 expires_in 的两种编码：数字或字符串。
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = flexInt(v)
	return nil
}

type wsPayload struct {
	Op int             `json:"op"`
	S  int64           `json:"s"`
	T  string          `json:"t"`
	D  json.RawMessage `json:"d"`
}

type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type messageEvent struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	GroupOpenID string `json:"group_openid"`
	ChannelID   string `json:"channel_id"`
	GuildID     string `json:"guild_id"`
	Timestamp   string `json:"timestamp"`
	Attachments []struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
		Filename    string `json:"filename"`
	} `json:"attachments"`
	Author      struct {
		ID           string `json:"id"`
		MemberOpenID string `json:"member_openid"`
		UserOpenID   string `json:"user_openid"`
		Username     string `json:"username"`
		MemberRole   string `json:"member_role"`
	} `json:"author"`
	MessageScene struct {
		Ext []string `json:"ext"`
	} `json:"message_scene"`
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}

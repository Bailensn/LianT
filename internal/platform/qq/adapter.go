package qq

import (
	"fmt"
	"strings"

	"LianT/internal/platform"
)

type adapter struct{}

func init() { platform.Register(adapter{}) }

func (adapter) ID() string   { return "qq" }
func (adapter) Name() string { return "QQ" }

func (adapter) Capabilities() platform.Capabilities {
	return platform.Capabilities{
		Text: true, Media: true, Buttons: false, Inline: false,
		Topics: false, InviteLinks: false, Webhook: false, JoinApproval: true,
		MemberList: false, Roles: true, Payments: false, Recall48h: false,
		MaxMessageLen: 2000,
	}
}

// QQ token 在 LianT 里统一存成 "AppID:AppSecret"，和 Telegram token 一样加密。
func (adapter) ValidateToken(token string) (platform.Identity, error) {
	token = strings.TrimSpace(token)
	id, secret, ok := strings.Cut(token, ":")
	id, secret = strings.TrimSpace(id), strings.TrimSpace(secret)
	if !ok || id == "" || secret == "" {
		return platform.Identity{}, fmt.Errorf("QQ token 格式应为 AppID:AppSecret")
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			return platform.Identity{}, fmt.Errorf("QQ AppID 应为数字")
		}
	}
	if _, _, err := FetchAccessToken(id, secret); err != nil {
		return platform.Identity{}, fmt.Errorf("QQ AppID/AppSecret 校验失败（检查凭据、网络或代理）: %w", err)
	}
	name := "QQ " + id
	return platform.Identity{ID: id, Username: "qq" + id, Name: name}, nil
}

// SplitToken 返回 AppID / AppSecret。
func SplitToken(token string) (string, string, bool) {
	id, secret, ok := strings.Cut(strings.TrimSpace(token), ":")
	id, secret = strings.TrimSpace(id), strings.TrimSpace(secret)
	if !ok || id == "" || secret == "" {
		return "", "", false
	}
	return id, secret, true
}

package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"LianT/internal/config"
	"LianT/internal/platform"
	tgclient "LianT/internal/telegram"
)

type adapter struct{}

func init() { platform.Register(adapter{}) }

func (adapter) ID() string   { return "telegram" }
func (adapter) Name() string { return "Telegram" }

func (adapter) Capabilities() platform.Capabilities {
	return platform.Capabilities{
		Text: true, Media: true, Buttons: true, Inline: true,
		Topics: true, InviteLinks: true, Webhook: true, JoinApproval: true,
		MemberList: false, Roles: true, Payments: true, Recall48h: true,
		MaxMessageLen: 4096,
	}
}

func (adapter) ValidateToken(token string) (platform.Identity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return platform.Identity{}, fmt.Errorf("token 不能为空")
	}
	if parts := strings.SplitN(token, ":", 2); len(parts) != 2 || parts[0] == "" || len(parts[1]) < 20 {
		return platform.Identity{}, fmt.Errorf("token 格式不对：应为 BotFather 给的 数字:字母数字_- 长串")
	} else {
		for _, ch := range parts[0] {
			if ch < '0' || ch > '9' {
				return platform.Identity{}, fmt.Errorf("token 格式不对：冒号前应是 Bot 数字 ID")
			}
		}
	}
	c, err := tgclient.New(token, config.APIBase(), config.Proxy())
	if err != nil {
		return platform.Identity{}, err
	}
	me, err := c.GetMe()
	if err != nil {
		return platform.Identity{}, fmt.Errorf("校验 Telegram token 失败: %w", err)
	}
	name := strings.TrimSpace(me.FirstName + " " + me.LastName)
	if name == "" {
		name = "@" + me.Username
	}
	return platform.Identity{
		ID:       strconv.FormatInt(me.ID, 10),
		Username: me.Username,
		Name:     name,
	}, nil
}

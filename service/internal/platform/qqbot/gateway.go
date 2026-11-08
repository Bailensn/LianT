package qqbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/gorilla/websocket"
)

var errReconnect = errors.New("qqbot: 服务端要求重连")

// Run 维持一条 WSS 连接，断线后指数退避重连，直到 ctx 取消。
func (a *Adapter) Run(
	ctx context.Context,
	acc platform.Account,
	creds platform.Credentials,
	hooks platform.Hooks,
) error {
	c, err := parseCredentials(creds)
	if err != nil {
		return err
	}

	backoff := time.Second
	for {
		err := a.session(ctx, c, hooks)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if hooks.OnStatus != nil {
			hooks.OnStatus(platform.Status{
				State:  platform.StateError,
				Reason: err.Error(),
				Since:  time.Now(),
			})
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

// session 建立一次连接并读到断开为止。
func (a *Adapter) session(ctx context.Context, c credentials, hooks platform.Hooks) error {
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return err
	}
	gateway, err := a.gatewayURL(ctx, c, token)
	if err != nil {
		return err
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, gateway, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("qqbot: 连接网关失败，HTTP %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("qqbot: 连接网关失败: %w", err)
	}
	defer conn.Close()

	// ctx 取消时 ReadJSON 不会自己返回，需要主动关连接。
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var hello wsPayload
	if err := conn.ReadJSON(&hello); err != nil {
		return fmt.Errorf("qqbot: 读取 hello 失败: %w", err)
	}
	if hello.Op != opHello {
		return fmt.Errorf("qqbot: 首帧 op=%d，期望 hello", hello.Op)
	}

	var hd helloData
	if len(hello.D) > 0 {
		if err := json.Unmarshal(hello.D, &hd); err != nil {
			return fmt.Errorf("qqbot: 解析 hello 失败: %w", err)
		}
	}
	interval := time.Duration(hd.HeartbeatInterval) * time.Millisecond
	if interval <= 0 {
		interval = 30 * time.Second
	}

	identify := map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":      "QQBot " + token,
			"intents":    intentGroupAndC2C | intentPublicGuild,
			"shard":      []int{0, 1},
			"properties": map[string]string{},
		},
	}
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteJSON(identify); err != nil {
		return fmt.Errorf("qqbot: 发送 identify 失败: %w", err)
	}

	if hooks.OnStatus != nil {
		hooks.OnStatus(platform.Status{State: platform.StateRunning, Since: time.Now()})
	}

	var seq atomic.Int64
	hbCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go a.heartbeat(hbCtx, conn, interval, &seq)

	for {
		conn.SetReadDeadline(time.Now().Add(interval*2 + 10*time.Second))

		var p wsPayload
		if err := conn.ReadJSON(&p); err != nil {
			return err
		}
		if p.S != 0 {
			seq.Store(p.S)
		}

		switch p.Op {
		case opDispatch:
			a.handleDispatch(ctx, p, hooks)
		case opHeartbeatAck:
			// 正常心跳回执，无需处理。
		case opReconnect:
			return errReconnect
		case opInvalidSession:
			return errors.New("qqbot: 会话失效")
		}
	}
}

func (a *Adapter) heartbeat(
	ctx context.Context,
	conn *websocket.Conn,
	interval time.Duration,
	seq *atomic.Int64,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(map[string]any{
				"op": opHeartbeat,
				"d":  seq.Load(),
			}); err != nil {
				return
			}
		}
	}
}

func (a *Adapter) gatewayURL(ctx context.Context, c credentials, token string) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		a.apiBaseFor(c)+"/gateway",
		nil,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qqbot: 获取网关地址失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qqbot: 获取网关返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}

	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("qqbot: 解析网关响应失败: %w", err)
	}
	if out.URL == "" {
		return "", errors.New("qqbot: 网关地址为空")
	}
	return out.URL, nil
}

// 事件类型只记一次，避免刷屏；排查「平台到底推没推」时很关键。
var loggedEvents sync.Map

func (a *Adapter) handleDispatch(ctx context.Context, p wsPayload, hooks platform.Hooks) {
	if p.T != "" {
		if _, seen := loggedEvents.LoadOrStore(p.T, struct{}{}); !seen {
			slog.Info("网关事件类型", "t", p.T, "op", p.Op)
		}
	}

	switch p.T {
	case "GROUP_AT_MESSAGE_CREATE", "C2C_MESSAGE_CREATE", "AT_MESSAGE_CREATE":
		// 其它事件（成员变动、频道管理等）v1 不关心。
	default:
		return
	}
	if hooks.OnMessage == nil {
		return
	}

	var ev messageEvent
	if err := json.Unmarshal(p.D, &ev); err != nil {
		return
	}
	msg, ok := toInbound(p.T, ev)
	if !ok {
		slog.Warn("消息事件缺少会话标识，已丢弃", "t", p.T,
			"group_openid", ev.GroupOpenID, "channel_id", ev.ChannelID,
			"user_openid", ev.Author.UserOpenID, "member_openid", ev.Author.MemberOpenID)
		return
	}
	_ = hooks.OnMessage(ctx, msg)
}

// toInbound 把平台事件映射成核心的 Inbound。
// peer_id 只在单个 bot 内唯一，因此必须带命名空间。
func toInbound(event string, ev messageEvent) (platform.Inbound, bool) {
	switch event {
	case "GROUP_AT_MESSAGE_CREATE":
		if ev.GroupOpenID == "" {
			return platform.Inbound{}, false
		}
		sender := ev.Author.MemberOpenID
		if sender == "" {
			sender = ev.Author.ID
		}
		return platform.Inbound{
			MessageID: ev.ID,
			Peer: platform.Peer{
				ID:   platform.MakeID("qqbot", ev.GroupOpenID),
				Kind: platform.PeerGroup,
			},
			SenderID: peerID(sender),
			Kind:     platform.CapText,
			Text:     strings.TrimSpace(ev.Content),
			Media:    attachments(ev.Attachments),
			At:       parseTime(ev.Timestamp),
		}, true

	case "C2C_MESSAGE_CREATE":
		user := ev.Author.UserOpenID
		if user == "" {
			user = ev.Author.ID
		}
		if user == "" {
			return platform.Inbound{}, false
		}
		return platform.Inbound{
			MessageID: ev.ID,
			Peer: platform.Peer{
				ID:   platform.MakeID("qqbot", user),
				Kind: platform.PeerPrivate,
			},
			SenderID: peerID(user),
			Kind:     platform.CapText,
			Text:     strings.TrimSpace(ev.Content),
			Media:    attachments(ev.Attachments),
			At:       parseTime(ev.Timestamp),
		}, true

	case "AT_MESSAGE_CREATE":
		if ev.ChannelID == "" {
			return platform.Inbound{}, false
		}
		return platform.Inbound{
			MessageID: ev.ID,
			Peer: platform.Peer{
				ID:   platform.MakeID("qqbot", ev.ChannelID),
				Kind: platform.PeerChannel,
			},
			SenderID: peerID(ev.Author.ID),
			Kind:     platform.CapText,
			Text:     strings.TrimSpace(ev.Content),
			Media:    attachments(ev.Attachments),
			At:       parseTime(ev.Timestamp),
		}, true

	default:
		return platform.Inbound{}, false
	}
}

// attachments 把平台附件换成核心的媒体描述；没有 url 的忽略。
func attachments(in []struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Filename    string `json:"filename"`
}) []platform.Attachment {
	out := make([]platform.Attachment, 0, len(in))
	for _, a := range in {
		if a.URL == "" {
			continue
		}
		out = append(out, platform.Attachment{URL: a.URL, MIME: a.ContentType, Size: a.Size})
	}
	return out
}

func peerID(external string) string {
	if external == "" {
		return ""
	}
	return platform.MakeID("qqbot", external)
}

// Package qqbot 是 QQ 官方机器人（群 / 单聊 / 频道）的平台适配器。
//
// 接入方式是 WSS：Service 主动连出，不需要公网端口。凭据是
// app_id + app_secret，access_token 在适配器内部刷新，核心不感知。
package qqbot

import (
	"bytes"
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/Bailensn/LianT/service/internal/platform"
)

// Adapter 实现 platform.Adapter。实例由注册表持有，必须无状态；
// token 缓存与消息序号是进程级的，按 key 隔离。
type Adapter struct {
	httpClient *http.Client
	tokenURL   string
	apiBase    string
	sandboxAPI string
	tokens     tokenCache
	seq        atomic.Uint32
}

// New 创建适配器。测试可以替换 tokenURL / apiBase 指向本地假服务。
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		tokenURL:   defaultTokenURL,
		apiBase:    defaultAPIBase,
		sandboxAPI: defaultSandboxAPI,
	}
}

func init() {
	platform.Register(New())
}

func (a *Adapter) Name() string { return "qqbot" }

// CredentialFields 声明 QQ 需要的两项凭据，CLI 会据此生成 --app-id/--app-secret。
func (a *Adapter) CredentialFields() []platform.CredentialField {
	return []platform.CredentialField{
		{Key: "app_id", Label: "AppID"},
		{Key: "app_secret", Label: "AppSecret", Secret: true},
	}
}

// Format 指向 format 注册表里的纯文本实现。
func (a *Adapter) Format() string { return "qq-plain" }

// Validate 用一次 token 请求证明凭据有效。
func (a *Adapter) Validate(ctx context.Context, creds platform.Credentials) (platform.Account, error) {
	c, err := parseCredentials(creds)
	if err != nil {
		return platform.Account{}, err
	}
	if _, err := a.accessToken(ctx, c); err != nil {
		return platform.Account{}, err
	}
	return platform.Account{
		ID:          c.appID,
		DisplayName: "QQ Bot " + c.appID,
		Caps:        []string{platform.CapText},
		MaxTextLen:  2000,
		Limits:      map[string]any{"max_text_len": 2000},
		// QQ 不能主动发起；被动回复窗口 300s，同一条入站最多回 5 次。
		Constraints: map[string]any{
			"can_initiate":          false,
			"reply_window_s":        300,
			"max_replies_per_inbound": 5,
			"requires_mention":      true,
		},
	}, nil
}

// SendText 按会话类型选择发送端点。
//
// QQ 的被动回复必须带触发消息的 msg_id 与递增的 msg_seq，
// 否则会被平台拒绝；主动消息则不带这两个字段。
func (a *Adapter) SendText(
	ctx context.Context,
	acc platform.Account,
	creds platform.Credentials,
	req platform.SendRequest,
) (platform.MessageID, error) {
	c, err := parseCredentials(creds)
	if err != nil {
		return "", err
	}
	_, external, err := platform.SplitID(req.Peer.ID)
	if err != nil {
		return "", err
	}
	path, err := sendPath(req.Peer.Kind, external)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"content":  req.Text,
		"msg_type": 0,
	}
	if req.InReplyTo != "" {
		payload["msg_id"] = req.InReplyTo
		payload["msg_seq"] = a.nextSeq()
	}

	id, err := a.postMessage(ctx, c, path, payload)
	if err != nil {
		return "", err
	}
	return platform.MessageID(id), nil
}

func sendPath(kind, external string) (string, error) {
	switch kind {
	case platform.PeerGroup:
		return "/v2/groups/" + url.PathEscape(external) + "/messages", nil
	case platform.PeerPrivate:
		return "/v2/users/" + url.PathEscape(external) + "/messages", nil
	case platform.PeerChannel:
		return "/channels/" + url.PathEscape(external) + "/messages", nil
	default:
		return "", fmt.Errorf("qqbot: 不支持的会话类型 %q", kind)
	}
}

// nextSeq 返回非 0 的递增序号；QQ 要求同一 msg_id 下 msg_seq 唯一。
func (a *Adapter) nextSeq() uint32 {
	for {
		if v := a.seq.Add(1); v != 0 {
			return v
		}
	}
}

func (a *Adapter) apiBaseFor(c credentials) string {
	if c.sandbox {
		return a.sandboxAPI
	}
	return a.apiBase
}

// accessToken 取缓存中的 token，过期则重新申请。
func (a *Adapter) accessToken(ctx context.Context, c credentials) (string, error) {
	key := a.tokenKey(c)
	if tok, ok := a.tokens.get(key); ok {
		return tok, nil
	}

	body, err := json.Marshal(map[string]string{
		"appId":        c.appID,
		"clientSecret": c.secret,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// 只保留网络层原因，别把整条 URL 糊到用户脸上。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return "", fmt.Errorf("连接 QQ 开放平台失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return "", fmt.Errorf("%w: HTTP %d %s", platform.ErrInvalidCredentials, resp.StatusCode, readSnippet(resp.Body))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qqbot: 申请 token 返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}

	var out struct {
		AccessToken string  `json:"access_token"`
		ExpiresIn   flexInt `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("qqbot: 解析 token 响应失败: %w", err)
	}
	if out.AccessToken == "" {
		return "", errEmptyToken
	}

	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	a.tokens.put(key, out.AccessToken, ttl)
	return out.AccessToken, nil
}

// postMessage 调用平台 HTTP API，返回平台消息 ID。
func (a *Adapter) postMessage(
	ctx context.Context,
	c credentials,
	path string,
	payload map[string]any,
) (string, error) {
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.apiBaseFor(c)+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("qqbot: 发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qqbot: 发送返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("qqbot: 解析发送响应失败: %w", err)
	}
	return out.ID, nil
}

func readSnippet(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 512))
	return string(b)
}

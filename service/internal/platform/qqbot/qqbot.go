// Package qqbot 是 QQ 官方机器人（群 / 单聊 / 频道）的平台适配器。
//
// 接入方式是 WSS：Service 主动连出，不需要公网端口。凭据是
// app_id + app_secret，access_token 在适配器内部刷新，核心不感知。
package qqbot

import (
	"bytes"
	"encoding/base64"
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	roles      sync.Map
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
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return platform.Account{}, err
	}
	acc := platform.Account{
		ID:          c.appID,
		DisplayName: "QQ Bot " + c.appID,
		Caps:        []string{platform.CapText, platform.CapMemberMute, platform.CapMemberKick},
		MaxTextLen:  2000,
		Limits:      map[string]any{"max_text_len": 2000},
		// 新一代机器人（19 开头的 appid）默认带全量群消息与主动消息权限，
		// 所以不再把「必须 @」和「不能主动发起」写死；被动回复窗口与次数仍然生效。
		Constraints: map[string]any{
			"can_initiate":          true,
			"reply_window_s":        300,
			"max_replies_per_inbound": 5,
			"requires_mention":      false,
		},
	}
	// 拉一次机器人自身资料，拿平台昵称；失败不影响登记。
	if name, err := a.fetchProfile(ctx, c, token); err == nil && name != "" {
		acc.Username = name
		acc.DisplayName = name
	}
	return acc, nil
}

// fetchProfile 读取机器人自己的昵称（/users/@me）。
func (a *Adapter) fetchProfile(ctx context.Context, c credentials, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.apiBaseFor(c)+"/users/@me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qqbot: /users/@me 返回 %d", resp.StatusCode)
	}
	var out struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Username, nil
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
// SendMedia 走 QQ 的富媒体直传（file_data ≤5M），拿到 file_info 后再发 msg_type=7。
func (a *Adapter) SendMedia(
	ctx context.Context,
	acc platform.Account,
	creds platform.Credentials,
	req platform.MediaRequest,
) (platform.MessageID, error) {
	c, err := parseCredentials(creds)
	if err != nil {
		return "", err
	}
	_, external, err := platform.SplitID(req.Peer.ID)
	if err != nil {
		return "", err
	}
	if req.Peer.Kind != platform.PeerGroup && req.Peer.Kind != platform.PeerPrivate {
		return "", fmt.Errorf("qqbot: 该会话类型不支持媒体")
	}
	if len(req.Media.Data) == 0 || len(req.Media.Data) > 5<<20 {
		return "", fmt.Errorf("qqbot: 媒体为空或超过 5 MiB")
	}
	fileType := 4
	switch req.Media.Kind {
	case "image":
		fileType = 1
	case "video":
		fileType = 2
	case "audio":
		fileType = 3
	case "file":
		fileType = 4
	default:
		return "", fmt.Errorf("qqbot: 不支持的媒体类型 %q", req.Media.Kind)
	}
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return "", err
	}
	base := "/v2/groups/"
	if req.Peer.Kind == platform.PeerPrivate {
		base = "/v2/users/"
	}
	body, err := json.Marshal(map[string]any{
		"file_type": fileType,
		"file_name": req.Media.Name,
		"file_data": base64.StdEncoding.EncodeToString(req.Media.Data),
	})
	if err != nil {
		return "", err
	}
	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.apiBaseFor(c)+base+url.PathEscape(external)+"/files", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	uploadReq.Header.Set("Authorization", "QQBot "+token)
	uploadReq.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("qqbot: 媒体上传失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("qqbot: 媒体上传返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	var uploaded struct {
		FileInfo string `json:"file_info"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&uploaded); err != nil {
		return "", fmt.Errorf("qqbot: 解析媒体上传响应失败: %w", err)
	}
	if uploaded.FileInfo == "" {
		return "", fmt.Errorf("qqbot: 媒体上传没有返回 file_info")
	}
	path, err := sendPath(req.Peer.Kind, external)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"content":  "",
		"msg_type": 7,
		"media":    map[string]any{"file_info": uploaded.FileInfo},
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

func (a *Adapter) MuteMember(
	ctx context.Context,
	acc platform.Account,
	creds platform.Credentials,
	req platform.MemberModeration,
) error {
	c, err := parseCredentials(creds)
	if err != nil {
		return err
	}
	_, group, err := platform.SplitID(req.Peer.ID)
	if err != nil {
		return err
	}
	if req.Peer.Kind != platform.PeerGroup {
		return fmt.Errorf("qqbot: 只有群聊支持禁言")
	}
	member := map[string]any{
		"member_openid": req.MemberID,
	}
	if req.Seconds > 0 {
		member["op"] = "add"
		member["mute_expire_at"] = time.Now().
			Add(time.Duration(req.Seconds) * time.Second).
			Format(time.RFC3339)
	} else {
		member["op"] = "del"
	}
	return a.postNoContent(
		ctx,
		c,
		"/v2/groups/"+url.PathEscape(group)+"/restrict_chat_setting",
		map[string]any{"members": []any{member}},
	)
}

func (a *Adapter) KickMember(
	ctx context.Context,
	acc platform.Account,
	creds platform.Credentials,
	req platform.MemberModeration,
) error {
	c, err := parseCredentials(creds)
	if err != nil {
		return err
	}
	_, group, err := platform.SplitID(req.Peer.ID)
	if err != nil {
		return err
	}
	if req.Peer.Kind != platform.PeerGroup {
		return fmt.Errorf("qqbot: 只有群聊支持移出成员")
	}
	return a.postNoContent(
		ctx,
		c,
		"/v2/groups/"+url.PathEscape(group)+"/batch_remove_members",
		map[string]any{
			"member_openids":          []string{req.MemberID},
			"add_to_member_blacklist": req.Blacklist,
		},
	)
}

func (a *Adapter) postNoContent(
	ctx context.Context,
	c credentials,
	path string,
	payload map[string]any,
) error {
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.apiBaseFor(c)+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qqbot: 群管理请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qqbot: 群管理返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	return nil
}

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

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("qqbot: 读取 token 响应失败: %w", err)
	}
	var out struct {
		AccessToken string  `json:"access_token"`
		ExpiresIn   flexInt `json:"expires_in"`
		Code        int     `json:"code"`
		Message     string  `json:"message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("qqbot: 解析 token 响应失败: %w", err)
	}
	if out.AccessToken == "" {
		// QQ 对错误的 app_id/secret 也可能回 200 + 错误码。
		if out.Code != 0 || out.Message != "" {
			return "", fmt.Errorf("%w: %d %s", platform.ErrInvalidCredentials, out.Code, out.Message)
		}
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

// memberInfo 取群成员角色与昵称：owner/admin/member。
//
// QQ 的群消息事件通常不带角色，只能查成员信息接口；该接口需要白名单权限，
// 失败就返回空，不阻塞消息投递。每个成员缓存 10 分钟。
func (a *Adapter) memberInfo(ctx context.Context, c credentials, token, group, member string) (string, string) {
	if group == "" || member == "" {
		return "", ""
	}
	key := c.appID + "\x00" + group + "\x00" + member
	if v, ok := a.roles.Load(key); ok {
		entry := v.(roleEntry)
		if time.Now().Before(entry.until) {
			return entry.role, entry.name
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	role, name, err := a.fetchMemberInfo(ctx, c, token, group, member)
	if err != nil {
		slog.Debug("获取 QQ 群成员信息失败", "member", member, "err", err)
	}
	a.roles.Store(key, roleEntry{role: role, name: name, until: time.Now().Add(10 * time.Minute)})
	return role, name
}

func (a *Adapter) fetchMemberInfo(ctx context.Context, c credentials, token, group, member string) (string, string, error) {
	u := a.apiBaseFor(c) + "/v2/groups/" + url.PathEscape(group) + "/members/" + url.PathEscape(member)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("qqbot: 成员信息返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}
	var out struct {
		MemberRole string `json:"member_role"`
		Username   string `json:"username"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", "", err
	}
	return normalizeRole(out.MemberRole), strings.TrimSpace(out.Username), nil
}

type roleEntry struct {
	role  string
	name  string
	until time.Time
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner":
		return "owner"
	case "admin":
		return "admin"
	case "member":
		return "member"
	}
	return ""
}

func readSnippet(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 512))
	return string(b)
}

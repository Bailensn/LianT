package qqbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Bailensn/LianT/service/internal/platform"
)

// PeerProfile 查群资料。QQ 要求该接口走白名单，没有授权会返回 403/404，
// 这里统一转成 ErrProfileForbidden，让上层提示用户而不是假装成功。
func (a *Adapter) PeerProfile(ctx context.Context, creds platform.Credentials, peerID string) (platform.PeerProfile, error) {
	c, err := parseCredentials(creds)
	if err != nil {
		return platform.PeerProfile{}, err
	}
	_, external, err := platform.SplitID(peerID)
	if err != nil {
		return platform.PeerProfile{}, err
	}
	token, err := a.accessToken(ctx, c)
	if err != nil {
		return platform.PeerProfile{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.apiBaseFor(c)+"/v2/groups/"+url.PathEscape(external)+"/info", nil)
	if err != nil {
		return platform.PeerProfile{}, err
	}
	req.Header.Set("Authorization", "QQBot "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return platform.PeerProfile{}, fmt.Errorf("qqbot: 查询群资料失败: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden,
		resp.StatusCode == http.StatusNotFound:
		return platform.PeerProfile{}, platform.ErrProfileForbidden
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return platform.PeerProfile{}, fmt.Errorf("qqbot: 查询群资料返回 %d: %s", resp.StatusCode, readSnippet(resp.Body))
	}

	var out struct {
		GroupOpenID string `json:"group_openid"`
		Name        string `json:"group_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return platform.PeerProfile{}, fmt.Errorf("qqbot: 解析群资料失败: %w", err)
	}
	return platform.PeerProfile{Title: out.Name}, nil
}

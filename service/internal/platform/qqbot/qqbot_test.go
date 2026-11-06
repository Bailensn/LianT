package qqbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Bailensn/LianT/service/internal/platform"
)

func TestParseCredentials(t *testing.T) {
	c, err := parseCredentials(platform.Credentials{
		"app_id":     " 10001 ",
		"app_secret": "secret",
		"sandbox":    "true",
	})
	if err != nil {
		t.Fatalf("合法凭据被拒绝: %v", err)
	}
	if c.appID != "10001" || c.secret != "secret" || !c.sandbox {
		t.Fatalf("解析结果不对: %+v", c)
	}

	for _, creds := range []platform.Credentials{
		nil,
		{},
		{"app_id": "10001"},
		{"app_secret": "secret"},
	} {
		if _, err := parseCredentials(creds); err == nil {
			t.Fatalf("残缺凭据 %v 没有报错", creds)
		}
	}
}

func TestFlexInt(t *testing.T) {
	cases := []string{`7200`, `"7200"`}
	for _, raw := range cases {
		var v flexInt
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatalf("解析 %s 失败: %v", raw, err)
		}
		if v != 7200 {
			t.Fatalf("解析 %s = %d", raw, v)
		}
	}
}

func TestToInboundGroup(t *testing.T) {
	ev := messageEvent{
		ID:          "m1",
		Content:     " 你好 ",
		GroupOpenID: "g1",
	}
	ev.Author.MemberOpenID = "u1"

	msg, ok := toInbound("GROUP_AT_MESSAGE_CREATE", ev)
	if !ok {
		t.Fatal("群消息没有映射成功")
	}
	if msg.Peer.ID != "qqbot:g1" || msg.Peer.Kind != platform.PeerGroup {
		t.Fatalf("peer 映射错误: %+v", msg.Peer)
	}
	if msg.SenderID != "qqbot:u1" {
		t.Fatalf("sender 映射错误: %q", msg.SenderID)
	}
	if msg.Text != "你好" {
		t.Fatalf("文本没有 trim: %q", msg.Text)
	}
}

func TestToInboundC2C(t *testing.T) {
	ev := messageEvent{ID: "m2", Content: "hi"}
	ev.Author.UserOpenID = "u2"

	msg, ok := toInbound("C2C_MESSAGE_CREATE", ev)
	if !ok {
		t.Fatal("单聊消息没有映射成功")
	}
	if msg.Peer.ID != "qqbot:u2" || msg.Peer.Kind != platform.PeerPrivate {
		t.Fatalf("peer 映射错误: %+v", msg.Peer)
	}
}

func TestToInboundIgnoresUnknown(t *testing.T) {
	if _, ok := toInbound("GUILD_CREATE", messageEvent{}); ok {
		t.Fatal("未知事件不应被映射")
	}
}

func TestSendPath(t *testing.T) {
	cases := []struct {
		kind string
		id   string
		want string
	}{
		{platform.PeerGroup, "g1", "/v2/groups/g1/messages"},
		{platform.PeerPrivate, "u1", "/v2/users/u1/messages"},
		{platform.PeerChannel, "c1", "/channels/c1/messages"},
	}
	for _, c := range cases {
		got, err := sendPath(c.kind, c.id)
		if err != nil {
			t.Fatalf("%s: %v", c.kind, err)
		}
		if got != c.want {
			t.Fatalf("%s: got %q want %q", c.kind, got, c.want)
		}
	}
	if _, err := sendPath("unknown", "x"); err == nil {
		t.Fatal("未知会话类型没有报错")
	}
}

func TestValidateAndSendText(t *testing.T) {
	var tokenCalls, sendCalls int
	var gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls++
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
		case "/v2/groups/g1/messages":
			sendCalls++
			gotAuth = r.Header.Get("Authorization")
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_, _ = w.Write([]byte(`{"id":"m2"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := New()
	a.tokenURL = srv.URL + "/token"
	a.apiBase = srv.URL
	a.sandboxAPI = srv.URL

	creds := platform.Credentials{"app_id": "app", "app_secret": "secret"}
	acc, err := a.Validate(context.Background(), creds)
	if err != nil {
		t.Fatalf("Validate 失败: %v", err)
	}
	if acc.ID != "app" {
		t.Fatalf("账号 ID = %q", acc.ID)
	}

	id, err := a.SendText(context.Background(), acc, creds, platform.SendRequest{
		Peer:      platform.Peer{ID: "qqbot:g1", Kind: platform.PeerGroup},
		Text:      "hi",
		InReplyTo: "m1",
	})
	if err != nil {
		t.Fatalf("SendText 失败: %v", err)
	}
	if string(id) != "m2" {
		t.Fatalf("消息 ID = %q", id)
	}
	if tokenCalls != 1 {
		t.Fatalf("token 请求次数 = %d，期望复用缓存", tokenCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("发送请求次数 = %d", sendCalls)
	}
	if gotAuth != "QQBot tok" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody["msg_id"] != "m1" {
		t.Fatalf("msg_id = %v", gotBody["msg_id"])
	}
	seq, ok := gotBody["msg_seq"].(float64)
	if !ok || seq <= 0 {
		t.Fatalf("msg_seq 非法: %v", gotBody["msg_seq"])
	}
}

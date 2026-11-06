package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Bailensn/LianT/service/internal/core"
	_ "github.com/Bailensn/LianT/service/internal/format/qqplain"
	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

const (
	testBotID  = "qqbot:1234567890"
	testPeerID = "qqbot:openid-abc"
)

type fakeAdapter struct {
	name string
	sent []platform.SendRequest
	err  error
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Validate(context.Context, platform.Credentials) (platform.Account, error) {
	return platform.Account{ID: "1234567890", Caps: []string{platform.CapText}}, nil
}

func (f *fakeAdapter) Run(context.Context, platform.Account, platform.Credentials, platform.Hooks) error {
	return nil
}

func (f *fakeAdapter) SendText(_ context.Context, _ platform.Account, _ platform.Credentials, req platform.SendRequest) (platform.MessageID, error) {
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, req)
	return "sent-1", nil
}

func (f *fakeAdapter) Format() string { return "qq-plain" }

type fakeBroadcaster struct {
	events []struct {
		typ     string
		payload any
	}
}

func (b *fakeBroadcaster) Broadcast(typ string, payload any) {
	b.events = append(b.events, struct {
		typ     string
		payload any
	}{typ, payload})
}

type testEnv struct {
	store *store.Store
	core  *core.Core
	ad    *fakeAdapter
	bc    *fakeBroadcaster
	now   time.Time
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/db.sqlite")
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ad := &fakeAdapter{name: "qqbot"}
	bc := &fakeBroadcaster{}
	now := time.UnixMilli(1_800_000_000_000)
	c, err := core.New(core.Options{
		Store: db,
		Adapter: func(name string) (platform.Adapter, bool) {
			if name == ad.name {
				return ad, true
			}
			return nil, false
		},
		Broadcaster: bc,
		DecodeCredentials: func(store.Bot) (platform.Credentials, error) {
			return platform.Credentials{"app_id": "1", "app_secret": "s"}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("构造 Core: %v", err)
	}
	return &testEnv{store: db, core: c, ad: ad, bc: bc, now: now}
}

func (e *testEnv) seedBot(t *testing.T, constraints map[string]any) {
	t.Helper()
	err := e.store.UpsertBot(context.Background(), store.Bot{
		ID:          testBotID,
		Platform:    "qqbot",
		Credentials: []byte("opaque"),
		Username:    "example_bot",
		DisplayName: "Example",
		Caps:        []string{platform.CapText},
		Limits:      map[string]any{"max_text_len": float64(2000)},
		Constraints: constraints,
		Status:      platform.Status{State: platform.StateRunning},
	})
	if err != nil {
		t.Fatalf("写入 Bot: %v", err)
	}
}

func (e *testEnv) seedPeer(t *testing.T) {
	t.Helper()
	_, err := e.store.AppendMessage(context.Background(), store.Message{
		BotID:     testBotID,
		MsgID:     "in-1",
		PeerID:    testPeerID,
		PeerKind:  platform.PeerGroup,
		Direction: store.DirectionIn,
		Type:      "text",
		TS:        1000,
		Plain:     "在吗",
	})
	if err != nil {
		t.Fatalf("写入入站消息: %v", err)
	}
}

func TestListBots(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false})

	got, perr := e.core.ListBots(context.Background(), server.Session{}, nil)
	if perr != nil {
		t.Fatalf("bot.list 失败: %+v", perr)
	}
	list := got.(protocol.BotList)
	if len(list.Bots) != 1 {
		t.Fatalf("Bot 数量不符: %d", len(list.Bots))
	}
	b := list.Bots[0]
	if b.BotID != testBotID || b.Platform != "qqbot" || b.DisplayName != "Example" {
		t.Fatalf("字段不符: %+v", b)
	}
	if b.Status != platform.StateRunning {
		t.Fatalf("状态不符: %s", b.Status)
	}
	if b.Caps == nil || b.Limits == nil || b.Constraints == nil {
		t.Fatalf("空集合应为 {} 而不是 nil: %+v", b)
	}
}

func TestSendRequiresReplyWindow(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false})
	e.seedPeer(t)

	payload, _ := json.Marshal(protocol.MsgSend{
		BotID: testBotID, PeerID: testPeerID,
		Body: protocol.Body{Plain: "主动打招呼"},
	})
	_, perr := e.core.Send(context.Background(), server.Session{ManagerID: "dev-1"}, payload)
	if perr == nil || perr.Code != protocol.CodeReplyWindowClosed {
		t.Fatalf("期望 reply_window_closed，得到 %+v", perr)
	}
	if len(e.ad.sent) != 0 {
		t.Fatal("被拒绝的请求不应调用平台")
	}
}

func TestSendRendersDocAndPersists(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false})
	e.seedPeer(t)

	doc := &format.Doc{
		V: format.Version,
		Blocks: []format.Block{{
			Type: format.BlockParagraph,
			Spans: []format.Span{
				{T: format.SpanText, Text: "看 "},
				{T: format.SpanBold, Text: "这里"},
			},
		}},
	}
	payload, _ := json.Marshal(protocol.MsgSend{
		BotID: testBotID, PeerID: testPeerID,
		Body:      protocol.Body{Plain: "看 这里", Doc: doc},
		InReplyTo: "in-1",
	})
	got, perr := e.core.Send(context.Background(), server.Session{ManagerID: "dev-1"}, payload)
	if perr != nil {
		t.Fatalf("msg.send 失败: %+v", perr)
	}
	ok := got.(protocol.MsgSendOK)
	if ok.MsgID != "sent-1" || ok.TS != e.now.UnixMilli() {
		t.Fatalf("响应不符: %+v", ok)
	}

	if len(e.ad.sent) != 1 {
		t.Fatalf("平台调用次数不符: %d", len(e.ad.sent))
	}
	if e.ad.sent[0].Text != "看 这里" {
		t.Fatalf("渲染结果不符: %q", e.ad.sent[0].Text)
	}
	if e.ad.sent[0].InReplyTo != "in-1" || e.ad.sent[0].Peer.Kind != platform.PeerGroup {
		t.Fatalf("投递参数不符: %+v", e.ad.sent[0])
	}

	saved, err := e.store.Message(context.Background(), testBotID, "sent-1")
	if err != nil {
		t.Fatalf("出站消息未落库: %v", err)
	}
	if saved.Direction != store.DirectionOut || saved.OperatorID != "dev-1" || saved.InReplyTo != "in-1" {
		t.Fatalf("落库字段不符: %+v", saved)
	}
}

func TestSendRejectsUnknownPeer(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": true})

	payload, _ := json.Marshal(protocol.MsgSend{
		BotID: testBotID, PeerID: testPeerID,
		Body: protocol.Body{Plain: "hi"},
	})
	_, perr := e.core.Send(context.Background(), server.Session{}, payload)
	if perr == nil || perr.Code != protocol.CodeNotFound {
		t.Fatalf("期望 not_found，得到 %+v", perr)
	}
}

func TestSendUpstreamFailure(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false})
	e.seedPeer(t)
	e.ad.err = errors.New("platform said no")

	payload, _ := json.Marshal(protocol.MsgSend{
		BotID: testBotID, PeerID: testPeerID,
		Body:      protocol.Body{Plain: "hi"},
		InReplyTo: "in-1",
	})
	_, perr := e.core.Send(context.Background(), server.Session{}, payload)
	if perr == nil || perr.Code != protocol.CodeUpstreamError {
		t.Fatalf("期望 upstream_error，得到 %+v", perr)
	}
	if _, err := e.store.Message(context.Background(), testBotID, "sent-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("失败的消息不应落库: %v", err)
	}
}

func TestIngestIsIdempotentAndBroadcasts(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, nil)

	in := platform.Inbound{
		MessageID: "m-1",
		Peer:      platform.Peer{ID: testPeerID, Kind: platform.PeerGroup},
		SenderID:  "qqbot:user-1",
		Kind:      "text",
		Text:      "你好",
		At:        time.UnixMilli(1234),
	}
	inserted, err := e.core.Ingest(context.Background(), testBotID, in)
	if err != nil || !inserted {
		t.Fatalf("首次入站失败: inserted=%v err=%v", inserted, err)
	}
	if len(e.bc.events) != 1 || e.bc.events[0].typ != protocol.TypeMsgInbound {
		t.Fatalf("推送不符: %+v", e.bc.events)
	}
	pushed := e.bc.events[0].payload.(protocol.MsgInbound)
	if pushed.MsgID != "m-1" || pushed.Body.Plain != "你好" || pushed.TS != 1234 {
		t.Fatalf("推送载荷不符: %+v", pushed)
	}

	inserted, err = e.core.Ingest(context.Background(), testBotID, in)
	if err != nil {
		t.Fatalf("重复入站报错: %v", err)
	}
	if inserted {
		t.Fatal("重复入站应返回 inserted=false")
	}
	if len(e.bc.events) != 1 {
		t.Fatal("重复入站不应重复推送")
	}
}

func TestListMessages(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, nil)
	e.seedPeer(t)

	payload, _ := json.Marshal(protocol.MsgList{BotID: testBotID, PeerID: testPeerID, Limit: 10})
	got, perr := e.core.ListMessages(context.Background(), server.Session{}, payload)
	if perr != nil {
		t.Fatalf("msg.list 失败: %+v", perr)
	}
	list := got.(protocol.MsgListOK)
	if len(list.Messages) != 1 || list.HasMore {
		t.Fatalf("分页结果不符: %+v", list)
	}
	if list.Messages[0].MsgID != "in-1" || list.Messages[0].Direction != store.DirectionIn {
		t.Fatalf("消息字段不符: %+v", list.Messages[0])
	}
}

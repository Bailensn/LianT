package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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
	acc  platform.Account
	sent []platform.SendRequest
	err  error
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Validate(context.Context, platform.Credentials) (platform.Account, error) {
	acc := f.acc
	if acc.ID == "" {
		acc.ID = "1234567890"
	}
	if acc.Caps == nil {
		acc.Caps = []string{platform.CapText}
	}
	return acc, nil
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

type fakeController struct {
	mu      sync.Mutex
	started []string
	stopped []string
}

func (f *fakeController) Start(_ context.Context, botID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, botID)
	return nil
}

func (f *fakeController) Stop(botID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, botID)
	return nil
}

type testEnv struct {
	store *store.Store
	core  *core.Core
	ad    *fakeAdapter
	bc    *fakeBroadcaster
	ctl   *fakeController
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
	ctl := &fakeController{}
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
		Controller:  ctl,
		Encrypt: func(raw []byte) (string, error) {
			return "enc:" + string(raw), nil
		},
		DecodeCredentials: func(store.Bot) (platform.Credentials, error) {
			return platform.Credentials{"app_id": "1", "app_secret": "s"}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("构造 Core: %v", err)
	}
	return &testEnv{store: db, core: c, ad: ad, bc: bc, ctl: ctl, now: now}
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
	// 登记不写运行状态，状态由 SetBotStatus 维护。
	if err := e.store.SetBotStatus(context.Background(), testBotID, platform.Status{State: platform.StateRunning}); err != nil {
		t.Fatalf("设置 Bot 状态: %v", err)
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
		TS:        e.now.UnixMilli() - 10_000,
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

func TestSendRejectsExpiredWindow(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false, "reply_window_s": float64(300), "max_replies_per_inbound": float64(5)})
	if _, err := e.store.AppendMessage(context.Background(), store.Message{
		BotID: testBotID, MsgID: "old-1", PeerID: testPeerID, PeerKind: platform.PeerGroup,
		Direction: store.DirectionIn, Type: "text", TS: e.now.UnixMilli() - 3600_000, Plain: "一小时前",
	}); err != nil {
		t.Fatalf("写入旧消息: %v", err)
	}

	payload, _ := json.Marshal(protocol.MsgSend{
		BotID: testBotID, PeerID: testPeerID,
		Body:      protocol.Body{Plain: "迟到的回复"},
		InReplyTo: "old-1",
	})
	_, perr := e.core.Send(context.Background(), server.Session{}, payload)
	if perr == nil || perr.Code != protocol.CodeReplyWindowClosed {
		t.Fatalf("期望 reply_window_closed，得到 %+v", perr)
	}
}

func TestSendEnforcesReplyLimit(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, map[string]any{"can_initiate": false, "reply_window_s": float64(300), "max_replies_per_inbound": float64(1)})
	e.seedPeer(t)

	send := func() *protocol.Error {
		payload, _ := json.Marshal(protocol.MsgSend{
			BotID: testBotID, PeerID: testPeerID,
			Body:      protocol.Body{Plain: "回复"},
			InReplyTo: "in-1",
		})
		_, perr := e.core.Send(context.Background(), server.Session{}, payload)
		return perr
	}
	if perr := send(); perr != nil {
		t.Fatalf("第一次回复应成功: %+v", perr)
	}
	if perr := send(); perr == nil || perr.Code != protocol.CodeReplyWindowClosed {
		t.Fatalf("第二次回复应被限制，得到 %+v", perr)
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

func TestAddBotStoresEncryptedCredentials(t *testing.T) {
	e := newEnv(t)
	e.ad.acc = platform.Account{
		ID:          "42",
		Caps:        []string{platform.CapText},
		Limits:      map[string]any{"max_text_len": float64(1000)},
		Constraints: map[string]any{"can_initiate": false},
	}
	payload, _ := json.Marshal(protocol.BotAdd{
		Platform:    "qqbot",
		Credentials: map[string]string{"app_id": "42", "app_secret": "secret"},
		Label:       "测试号",
	})
	got, perr := e.core.AddBot(context.Background(), server.Session{}, payload)
	if perr != nil {
		t.Fatalf("bot.add 失败: %+v", perr)
	}
	ref := got.(protocol.BotRef)
	if ref.BotID != "qqbot:42" {
		t.Fatalf("bot_id 不符: %s", ref.BotID)
	}
	b, err := e.store.Bot(context.Background(), ref.BotID)
	if err != nil {
		t.Fatalf("读取 Bot: %v", err)
	}
	if string(b.Credentials) == `{"app_id":"42","app_secret":"secret"}` {
		t.Fatal("凭据不应明文落库")
	}
	if b.Caps == nil || b.Constraints["can_initiate"] != false {
		t.Fatalf("适配器声明的能力未保存: %+v", b)
	}
}

func TestBotStartStop(t *testing.T) {
	e := newEnv(t)
	e.seedBot(t, nil)
	payload, _ := json.Marshal(protocol.BotRef{BotID: testBotID})

	if _, perr := e.core.StartBot(context.Background(), server.Session{}, payload); perr != nil {
		t.Fatalf("bot.start 失败: %+v", perr)
	}
	if len(e.ctl.started) != 1 || e.ctl.started[0] != testBotID {
		t.Fatalf("控制器未收到启动: %+v", e.ctl.started)
	}
	if _, perr := e.core.StopBot(context.Background(), server.Session{}, payload); perr != nil {
		t.Fatalf("bot.stop 失败: %+v", perr)
	}
	if len(e.ctl.stopped) != 1 {
		t.Fatalf("控制器未收到停止: %+v", e.ctl.stopped)
	}

	status, perr := e.core.BotStatus(context.Background(), server.Session{}, payload)
	if perr != nil {
		t.Fatalf("bot.status 失败: %+v", perr)
	}
	if status.(protocol.BotStatusOK).Status != platform.StateRunning {
		t.Fatalf("状态不符: %+v", status)
	}
}

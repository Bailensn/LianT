package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/platform"
)

const (
	testBotID  = "qqbot:1234567890"
	testPeerID = "qqbot:openid-abc"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedBot(t *testing.T, s *Store) {
	t.Helper()
	err := s.UpsertBot(context.Background(), Bot{
		ID:          testBotID,
		Platform:    "qqbot",
		Label:       "测试号",
		Credentials: []byte("encrypted-blob"),
		Username:    "example_bot",
		DisplayName: "Example",
		Caps:        []string{platform.CapText},
		Limits:      map[string]any{"max_text_len": float64(2000)},
		Constraints: map[string]any{"can_initiate": false},
		Status:      platform.Status{State: platform.StateRunning},
	})
	if err != nil {
		t.Fatalf("写入 bot: %v", err)
	}
}

func msg(n int, direction string) Message {
	return Message{
		BotID:     testBotID,
		MsgID:     "m" + itoa(n),
		PeerID:    testPeerID,
		PeerKind:  platform.PeerGroup,
		Direction: direction,
		Type:      "text",
		TS:        int64(n) * 1000,
		Plain:     "消息 " + itoa(n),
	}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestBotLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, err := s.Bot(ctx, testBotID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("空库应返回 ErrNotFound，得到 %v", err)
	}

	seedBot(t, s)
	b, err := s.Bot(ctx, testBotID)
	if err != nil {
		t.Fatalf("读取 bot: %v", err)
	}
	if b.DisplayName != "Example" || b.Platform != "qqbot" {
		t.Fatalf("字段不符: %+v", b)
	}
	// 登记不等于启动：Upsert 不碰运行状态，状态由 SetBotStatus 维护。
	if b.Status.State != platform.StateStopped {
		t.Fatalf("新登记的 Bot 应为 stopped: %+v", b.Status)
	}
	if string(b.Credentials) != "encrypted-blob" {
		t.Fatalf("凭据被改写: %q", b.Credentials)
	}

	if err := s.SetBotStatus(ctx, testBotID, platform.Status{State: platform.StateRunning}); err != nil {
		t.Fatalf("设置状态: %v", err)
	}

	// 二次 Upsert 不应改变 created_at，也不应影响状态。
	created := b.CreatedAt
	if err := s.UpsertBot(ctx, Bot{
		ID:          testBotID,
		Platform:    "qqbot",
		Label:       "改名",
		Credentials: []byte("new-blob"),
		DisplayName: "Renamed",
	}); err != nil {
		t.Fatalf("更新 bot: %v", err)
	}
	b, err = s.Bot(ctx, testBotID)
	if err != nil {
		t.Fatalf("再次读取 bot: %v", err)
	}
	if b.DisplayName != "Renamed" || b.Label != "改名" {
		t.Fatalf("更新未生效: %+v", b)
	}
	if !b.CreatedAt.Equal(created) {
		t.Fatalf("created_at 被改写: %v -> %v", created, b.CreatedAt)
	}
	if b.Status.State != platform.StateRunning {
		t.Fatalf("更新把状态弄丢了: %+v", b.Status)
	}

	if err := s.SetBotStatus(ctx, testBotID, platform.Status{State: platform.StateError, Reason: "token 失效"}); err != nil {
		t.Fatalf("更新状态: %v", err)
	}
	b, _ = s.Bot(ctx, testBotID)
	if b.Status.State != platform.StateError || b.Status.Reason != "token 失效" {
		t.Fatalf("状态未更新: %+v", b.Status)
	}

	if err := s.DeleteBot(ctx, testBotID); err != nil {
		t.Fatalf("删除 bot: %v", err)
	}
	if _, err := s.Bot(ctx, testBotID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后仍能读到: %v", err)
	}
}

func TestUpsertBotRejectsBadID(t *testing.T) {
	s := newTestStore(t)
	cases := []Bot{
		{ID: "1234567890", Platform: "qqbot", Credentials: []byte("x")},
		{ID: "qqbot:123", Platform: "telegram", Credentials: []byte("x")},
		{ID: "qqbot:123", Platform: "qqbot"},
	}
	for i, c := range cases {
		if err := s.UpsertBot(context.Background(), c); !errors.Is(err, ErrInvalid) {
			t.Fatalf("第 %d 个非法 Bot 未被拒绝: %v", i, err)
		}
	}
}

func TestAppendMessageIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)

	m := msg(1, DirectionIn)
	inserted, err := s.AppendMessage(ctx, m)
	if err != nil || !inserted {
		t.Fatalf("首次写入失败: inserted=%v err=%v", inserted, err)
	}
	inserted, err = s.AppendMessage(ctx, m)
	if err != nil {
		t.Fatalf("重复写入报错: %v", err)
	}
	if inserted {
		t.Fatal("重复写入应返回 inserted=false")
	}

	got, err := s.Message(ctx, testBotID, "m1")
	if err != nil {
		t.Fatalf("读取消息: %v", err)
	}
	if got.Plain != "消息 1" || got.PeerID != testPeerID {
		t.Fatalf("字段不符: %+v", got)
	}
}

func TestAppendMessageStoresDoc(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)

	m := msg(1, DirectionIn)
	m.Doc = &format.Doc{
		V: format.Version,
		Blocks: []format.Block{{
			Type: format.BlockParagraph,
			Spans: []format.Span{
				{T: format.SpanText, Text: "看 "},
				{T: format.SpanBold, Text: "这里"},
			},
		}},
	}
	if _, err := s.AppendMessage(ctx, m); err != nil {
		t.Fatalf("写入带 doc 的消息: %v", err)
	}
	got, err := s.Message(ctx, testBotID, "m1")
	if err != nil {
		t.Fatalf("读取消息: %v", err)
	}
	if got.Doc == nil || len(got.Doc.Blocks) != 1 || got.Doc.Blocks[0].Spans[1].T != format.SpanBold {
		t.Fatalf("doc 未完整保存: %+v", got.Doc)
	}
}

func TestAppendMessageRejectsUnknownKind(t *testing.T) {
	s := newTestStore(t)
	seedBot(t, s)

	m := msg(1, DirectionIn)
	m.PeerKind = "room"
	if _, err := s.AppendMessage(context.Background(), m); !errors.Is(err, ErrInvalid) {
		t.Fatalf("未知 peer_kind 未被拒绝: %v", err)
	}
}

func TestMessagesPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)
	for i := 1; i <= 5; i++ {
		if _, err := s.AppendMessage(ctx, msg(i, DirectionIn)); err != nil {
			t.Fatalf("写入第 %d 条: %v", i, err)
		}
	}

	ids := func(ms []Message) []string {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m.MsgID
		}
		return out
	}
	eq := func(got []string, want ...string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	latest, hasMore, err := s.Messages(ctx, ListQuery{BotID: testBotID, PeerID: testPeerID, Limit: 2})
	if err != nil {
		t.Fatalf("取最新一页: %v", err)
	}
	if !eq(ids(latest), "m4", "m5") || !hasMore {
		t.Fatalf("最新一页不符: %v hasMore=%v", ids(latest), hasMore)
	}

	older, hasMore, err := s.Messages(ctx, ListQuery{
		BotID: testBotID, PeerID: testPeerID, Limit: 2,
		Before: &Cursor{TS: 4000, MsgID: "m4"},
	})
	if err != nil {
		t.Fatalf("向前翻页: %v", err)
	}
	if !eq(ids(older), "m2", "m3") || !hasMore {
		t.Fatalf("向前翻页不符: %v hasMore=%v", ids(older), hasMore)
	}

	oldest, hasMore, err := s.Messages(ctx, ListQuery{
		BotID: testBotID, PeerID: testPeerID, Limit: 2,
		Before: &Cursor{TS: 2000, MsgID: "m2"},
	})
	if err != nil {
		t.Fatalf("取最早一页: %v", err)
	}
	if !eq(ids(oldest), "m1") || hasMore {
		t.Fatalf("最早一页不符: %v hasMore=%v", ids(oldest), hasMore)
	}

	newer, hasMore, err := s.Messages(ctx, ListQuery{
		BotID: testBotID, PeerID: testPeerID, Limit: 2,
		After: &Cursor{TS: 2000, MsgID: "m2"},
	})
	if err != nil {
		t.Fatalf("向后翻页: %v", err)
	}
	if !eq(ids(newer), "m3", "m4") || !hasMore {
		t.Fatalf("向后翻页不符: %v hasMore=%v", ids(newer), hasMore)
	}

	tail, hasMore, err := s.Messages(ctx, ListQuery{
		BotID: testBotID, PeerID: testPeerID, Limit: 2,
		After: &Cursor{TS: 4000, MsgID: "m4"},
	})
	if err != nil {
		t.Fatalf("取末尾: %v", err)
	}
	if !eq(ids(tail), "m5") || hasMore {
		t.Fatalf("末尾不符: %v hasMore=%v", ids(tail), hasMore)
	}
}

func TestPeersOrderAndCount(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)

	other := "qqbot:openid-xyz"
	if _, err := s.AppendMessage(ctx, msg(1, DirectionIn)); err != nil {
		t.Fatalf("写入: %v", err)
	}
	m2 := msg(2, DirectionIn)
	m2.PeerID = other
	if _, err := s.AppendMessage(ctx, m2); err != nil {
		t.Fatalf("写入: %v", err)
	}
	if _, err := s.AppendMessage(ctx, msg(3, DirectionOut)); err != nil {
		t.Fatalf("写入: %v", err)
	}

	peers, err := s.Peers(ctx, testBotID)
	if err != nil {
		t.Fatalf("列会话: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("会话数不符: %+v", peers)
	}
	if peers[0].ID != testPeerID || peers[0].Count != 2 || peers[0].LastMsgID != "m3" {
		t.Fatalf("最近会话不符: %+v", peers[0])
	}
	if peers[0].LastPlain != "消息 3" {
		t.Fatalf("最近消息预览不符: %q", peers[0].LastPlain)
	}
	if peers[1].ID != other || peers[1].Count != 1 {
		t.Fatalf("次近会话不符: %+v", peers[1])
	}
}

func TestPurgeExpiredMessages(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)

	now := time.Now()
	old := msg(1, DirectionIn)
	old.TS = now.Add(-8 * 24 * time.Hour).UnixMilli()
	if _, err := s.AppendMessage(ctx, old); err != nil {
		t.Fatalf("写入旧消息: %v", err)
	}
	fresh := msg(2, DirectionIn)
	fresh.TS = now.Add(-time.Hour).UnixMilli()
	if _, err := s.AppendMessage(ctx, fresh); err != nil {
		t.Fatalf("写入新消息: %v", err)
	}

	res, err := s.Purge(ctx, DefaultRetention(), now)
	if err != nil {
		t.Fatalf("清理: %v", err)
	}
	if res.Messages != 1 {
		t.Fatalf("应清理 1 条，得到 %d", res.Messages)
	}
	if _, err := s.Message(ctx, testBotID, old.MsgID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("旧消息仍在: %v", err)
	}
	if _, err := s.Message(ctx, testBotID, fresh.MsgID); err != nil {
		t.Fatalf("新消息被误删: %v", err)
	}
}

func TestPurgeEnforcesMediaCap(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	seedBot(t, s)

	now := time.Now()
	for i := 1; i <= 3; i++ {
		m := msg(i, DirectionIn)
		m.TS = now.Add(-time.Duration(10-i) * time.Hour).UnixMilli()
		if _, err := s.AppendMessage(ctx, m); err != nil {
			t.Fatalf("写入消息 %d: %v", i, err)
		}
		if err := s.PutMedia(ctx, Media{
			BotID: testBotID, MsgID: m.MsgID, Index: 0,
			MIME: "image/png", Path: "/tmp/" + m.MsgID + ".png",
			Size: 1000, State: MediaReady,
		}); err != nil {
			t.Fatalf("写入媒体 %d: %v", i, err)
		}
	}
	if used, _, err := s.MediaUsage(ctx, testBotID); err != nil || used != 3000 {
		t.Fatalf("媒体占用不符: %d err=%v", used, err)
	}

	r := DefaultRetention()
	r.MediaBytes = 1500
	res, err := s.Purge(ctx, r, now)
	if err != nil {
		t.Fatalf("清理: %v", err)
	}
	if res.MediaEvicted != 2 {
		t.Fatalf("应淘汰 2 个媒体，得到 %d", res.MediaEvicted)
	}
	if used, _, err := s.MediaUsage(ctx, testBotID); err != nil || used != 1000 {
		t.Fatalf("淘汰后占用不符: %d err=%v", used, err)
	}

	list, err := s.MediaOf(ctx, testBotID, "m3")
	if err != nil || len(list) != 1 || list[0].State != MediaReady {
		t.Fatalf("最新媒体不应被淘汰: %+v err=%v", list, err)
	}
	old, err := s.MediaOf(ctx, testBotID, "m1")
	if err != nil || len(old) != 1 || old[0].State != MediaEvicted {
		t.Fatalf("最旧媒体应被淘汰: %+v err=%v", old, err)
	}
}

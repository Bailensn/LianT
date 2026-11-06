package protocol_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/protocol"
)

func TestEnvelopeValidate(t *testing.T) {
	cases := []struct {
		name string
		env  protocol.Envelope
		want error
	}{
		{"正常", protocol.Envelope{V: 1, ID: "a", Type: "ping"}, nil},
		{"版本不支持", protocol.Envelope{V: 2, ID: "a", Type: "ping"}, protocol.ErrUnsupportedVersion},
		{"缺 id", protocol.Envelope{V: 1, Type: "ping"}, protocol.ErrInvalidEnvelope},
		{"缺 type", protocol.Envelope{V: 1, ID: "a"}, protocol.ErrInvalidEnvelope},
		{"帧过大", protocol.Envelope{V: 1, ID: "a", Type: "x", Payload: make([]byte, protocol.MaxFrame+1)}, protocol.ErrFrameTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.env.Validate()
			if c.want == nil {
				if err != nil {
					t.Fatalf("合法信封被拒: %v", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("期望 %v，得到 %v", c.want, err)
			}
		})
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	req, err := protocol.New("req-1", protocol.TypeMsgSend, 123, protocol.MsgSend{
		BotID:  "qqbot:1",
		PeerID: "qqbot:2",
		Body:   protocol.Body{Plain: "你好"},
	})
	if err != nil {
		t.Fatalf("构造请求: %v", err)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("编码: %v", err)
	}
	var got protocol.Envelope
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("解码: %v", err)
	}
	if got.ID != "req-1" || got.Type != protocol.TypeMsgSend || got.TS != 123 {
		t.Fatalf("信封字段不符: %+v", got)
	}
	var send protocol.MsgSend
	if err := got.DecodePayload(&send); err != nil {
		t.Fatalf("解析 payload: %v", err)
	}
	if send.Body.Plain != "你好" || send.BotID != "qqbot:1" {
		t.Fatalf("payload 不符: %+v", send)
	}

	ok, err := protocol.NewReply(req, "resp-1", protocol.MsgSendOK{MsgID: "m1", TS: 9})
	if err != nil {
		t.Fatalf("构造响应: %v", err)
	}
	if ok.ReplyTo != "req-1" {
		t.Fatalf("reply_to 不符: %q", ok.ReplyTo)
	}
}

func TestDecodePayloadEmpty(t *testing.T) {
	env := protocol.Envelope{V: 1, ID: "a", Type: "ping"}
	var v struct{ X int }
	if err := env.DecodePayload(&v); err != nil {
		t.Fatalf("空 payload 应视为零值: %v", err)
	}
}

func TestRetryable(t *testing.T) {
	retry := []string{protocol.CodeRateLimited, protocol.CodeClockSkew, protocol.CodeInternal, protocol.CodeUpstreamError}
	for _, c := range retry {
		if !protocol.Retryable(c) {
			t.Fatalf("%s 应可重试", c)
		}
	}
	no := []string{protocol.CodeUnauthorized, protocol.CodeForbidden, protocol.CodeNotFound, protocol.CodeInvalidPayload}
	for _, c := range no {
		if protocol.Retryable(c) {
			t.Fatalf("%s 不应可重试", c)
		}
	}
}

func TestBodyValidate(t *testing.T) {
	if err := (protocol.Body{}).Validate(); err == nil {
		t.Fatal("空 body 应被拒绝")
	}
	if err := (protocol.Body{Plain: "hi"}).Validate(); err != nil {
		t.Fatalf("纯文本 body 被拒: %v", err)
	}
	doc := &format.Doc{
		V: format.Version,
		Blocks: []format.Block{{
			Type:  format.BlockParagraph,
			Spans: []format.Span{{T: format.SpanText, Text: "hi"}},
		}},
	}
	if err := (protocol.Body{Plain: "hi", Doc: doc}).Validate(); err != nil {
		t.Fatalf("合法 doc 被拒: %v", err)
	}
	bad := &format.Doc{V: format.Version, Blocks: []format.Block{{Type: "table"}}}
	if err := (protocol.Body{Plain: "hi", Doc: bad}).Validate(); err == nil {
		t.Fatal("非法 doc 应被拒绝")
	}
}

func TestMsgSendValidate(t *testing.T) {
	valid := protocol.MsgSend{BotID: "qqbot:1", PeerID: "qqbot:2", Body: protocol.Body{Plain: "hi"}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("合法请求被拒: %v", err)
	}

	cases := map[string]protocol.MsgSend{
		"非法 bot_id":  {BotID: "1", PeerID: "qqbot:2", Body: protocol.Body{Plain: "hi"}},
		"非法 peer_id": {BotID: "qqbot:1", PeerID: "2", Body: protocol.Body{Plain: "hi"}},
		"空 body":     {BotID: "qqbot:1", PeerID: "qqbot:2"},
	}
	for name, req := range cases {
		if err := req.Validate(); err == nil {
			t.Fatalf("%s 未被拒绝", name)
		} else if strings.TrimSpace(err.Error()) == "" {
			t.Fatalf("%s 的错误信息为空", name)
		}
	}
}

func TestMsgListValidate(t *testing.T) {
	valid := protocol.MsgList{BotID: "qqbot:1", PeerID: "qqbot:2", Limit: 50}
	if err := valid.Validate(); err != nil {
		t.Fatalf("合法请求被拒: %v", err)
	}
	both := protocol.MsgList{BotID: "qqbot:1", PeerID: "qqbot:2", Before: &protocol.Cur{TS: 1}, After: &protocol.Cur{TS: 2}}
	if err := both.Validate(); err == nil {
		t.Fatal("before 与 after 同时出现应被拒绝")
	}
	tooMany := protocol.MsgList{BotID: "qqbot:1", PeerID: "qqbot:2", Limit: 101}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("超限 limit 应被拒绝")
	}
}

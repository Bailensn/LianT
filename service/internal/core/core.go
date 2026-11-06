// Package core 把协议方法接到存储与平台适配器上。
//
// 这里是「业务」层：bot.list 从 store 取，msg.send 调平台适配器再落库，
// msg.inbound 落库并推给在线客户端。传输与鉴权在 internal/server，不在这里。
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/Bailensn/LianT/service/internal/crypto"
	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/platform"
	"github.com/Bailensn/LianT/service/internal/protocol"
	"github.com/Bailensn/LianT/service/internal/server"
	"github.com/Bailensn/LianT/service/internal/store"
)

// Broadcaster 是向在线客户端推送事件的出口（server.Server 满足）。
type Broadcaster interface {
	Broadcast(typ string, payload any)
}

// Options 是 Core 的依赖。
type Options struct {
	Store *store.Store

	// Adapter 按平台名取适配器；留空则用全局注册表。
	Adapter func(platformName string) (platform.Adapter, bool)

	// Broadcaster 可空：为空时 inbound 只落库不推送（CLI/测试）。
	Broadcaster Broadcaster

	// DecodeCredentials 把 store 里的不透明凭据还原成平台凭据；
	// 留空则按主密钥解密 JSON。测试可注入固定实现。
	DecodeCredentials func(bot store.Bot) (platform.Credentials, error)

	Now func() time.Time
}

// Core 是协议方法的实现。
type Core struct {
	opt Options
	now func() time.Time
}

// New 校验依赖并构造 Core。
func New(opt Options) (*Core, error) {
	if opt.Store == nil {
		return nil, errors.New("core: 缺少存储")
	}
	if opt.Adapter == nil {
		opt.Adapter = platform.Get
	}
	if opt.DecodeCredentials == nil {
		opt.DecodeCredentials = decodeCredentials
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	return &Core{opt: opt, now: opt.Now}, nil
}

// Register 把 v1 的核心方法挂到服务端。
func (c *Core) Register(s *server.Server) {
	s.Register(protocol.TypeBotList, c.ListBots)
	s.Register(protocol.TypeMsgSend, c.Send)
	s.Register(protocol.TypeMsgList, c.ListMessages)
	s.Register(protocol.TypeMsgAck, c.Ack)
}

func decodeCredentials(b store.Bot) (platform.Credentials, error) {
	raw, err := crypto.DecryptBytes(string(b.Credentials))
	if err != nil {
		return nil, fmt.Errorf("core: 解密凭据: %w", err)
	}
	var creds platform.Credentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return nil, fmt.Errorf("core: 解析凭据: %w", err)
	}
	return creds, nil
}

// ListBots 返回全部 Bot 的公开元数据（不含凭据）。
func (c *Core) ListBots(ctx context.Context, _ server.Session, _ json.RawMessage) (any, *protocol.Error) {
	bots, err := c.opt.Store.Bots(ctx)
	if err != nil {
		return nil, internal(err)
	}
	out := make([]protocol.Bot, 0, len(bots))
	for _, b := range bots {
		out = append(out, toProtocolBot(b))
	}
	return protocol.BotList{Bots: out}, nil
}

// ListMessages 返回会话内一页消息。
func (c *Core) ListMessages(ctx context.Context, _ server.Session, raw json.RawMessage) (any, *protocol.Error) {
	var req protocol.MsgList
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, "msg.list 字段不合法")
	}
	if err := req.Validate(); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, err.Error())
	}
	q := store.ListQuery{BotID: req.BotID, PeerID: req.PeerID, Limit: req.Limit}
	if req.Before != nil {
		q.Before = &store.Cursor{TS: req.Before.TS, MsgID: req.Before.MsgID}
	}
	if req.After != nil {
		q.After = &store.Cursor{TS: req.After.TS, MsgID: req.After.MsgID}
	}

	msgs, hasMore, err := c.opt.Store.Messages(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) {
			return nil, bad(protocol.CodeInvalidPayload, err.Error())
		}
		return nil, internal(err)
	}
	out := make([]protocol.MsgInbound, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toProtocolMessage(m))
	}
	return protocol.MsgListOK{Messages: out, HasMore: hasMore}, nil
}

// Send 发出一条消息：按适配器渲染、投递，再把出站消息落库。
func (c *Core) Send(ctx context.Context, sess server.Session, raw json.RawMessage) (any, *protocol.Error) {
	var req protocol.MsgSend
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, "msg.send 字段不合法")
	}
	if err := req.Validate(); err != nil {
		return nil, bad(protocol.CodeInvalidPayload, err.Error())
	}

	bot, err := c.opt.Store.Bot(ctx, req.BotID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, bad(protocol.CodeNotFound, "Bot 不存在")
	}
	if err != nil {
		return nil, internal(err)
	}
	adapter, ok := c.opt.Adapter(bot.Platform)
	if !ok {
		return nil, bad(protocol.CodeUnsupportedCapability, "该平台没有注册适配器")
	}
	creds, err := c.opt.DecodeCredentials(bot)
	if err != nil {
		return nil, internal(err)
	}
	peer, err := c.opt.Store.Peer(ctx, req.BotID, req.PeerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, bad(protocol.CodeNotFound, "会话不存在：只能回复已存在的会话")
	}
	if err != nil {
		return nil, internal(err)
	}

	// 投递约束：QQ 之类不允许主动发起，必须带上触发消息。
	if req.InReplyTo == "" {
		if !canInitiate(bot) {
			return nil, bad(protocol.CodeReplyWindowClosed, "该平台不允许主动发起消息，需在会话内回复")
		}
	} else {
		trigger, err := c.opt.Store.Message(ctx, req.BotID, req.InReplyTo)
		if errors.Is(err, store.ErrNotFound) {
			return nil, bad(protocol.CodeNotFound, "要回复的消息不存在或已过期")
		}
		if err != nil {
			return nil, internal(err)
		}
		if trigger.Direction != store.DirectionIn {
			return nil, bad(protocol.CodeInvalidPayload, "in_reply_to 必须是一条入站消息")
		}
	}

	text := req.Body.Plain
	if req.Body.Doc != nil {
		f, ok := format.Get(adapter.Format())
		if !ok {
			return nil, bad(protocol.CodeInternal, "平台声明的格式适配器没有注册")
		}
		rendered, err := f.Render(req.Body.Doc)
		if err != nil {
			return nil, bad(protocol.CodeInvalidDoc, err.Error())
		}
		text = rendered
	}
	if max := maxTextLen(bot); max > 0 && utf8.RuneCountInString(text) > max {
		return nil, bad(protocol.CodeContentTooLong, fmt.Sprintf("渲染后超过 %d 字符", max))
	}

	mid, err := adapter.SendText(ctx, accountOf(bot), creds, platform.SendRequest{
		Peer:      platform.Peer{ID: peer.ID, Kind: peer.Kind},
		Text:      text,
		InReplyTo: req.InReplyTo,
	})
	if err != nil {
		return nil, &protocol.Error{
			Code:      protocol.CodeUpstreamError,
			Message:   "平台投递失败",
			Retryable: protocol.Retryable(protocol.CodeUpstreamError),
			Details:   map[string]any{"reason": err.Error()},
		}
	}

	ts := c.now().UnixMilli()
	operator := req.OperatorID
	if operator == "" {
		operator = sess.ManagerID
	}
	_, err = c.opt.Store.AppendMessage(ctx, store.Message{
		BotID:      req.BotID,
		MsgID:      string(mid),
		PeerID:     peer.ID,
		PeerKind:   peer.Kind,
		Direction:  store.DirectionOut,
		Type:       "text",
		TS:         ts,
		Plain:      text,
		Doc:        req.Body.Doc,
		InReplyTo:  req.InReplyTo,
		OperatorID: operator,
	})
	if err != nil {
		return nil, internal(err)
	}
	return protocol.MsgSendOK{MsgID: string(mid), TS: ts}, nil
}

// Ack 记录客户端已落库；v1 服务端没有未读表，只做应答。
func (c *Core) Ack(context.Context, server.Session, json.RawMessage) (any, *protocol.Error) {
	return struct{}{}, nil
}

// Ingest 把适配器的入站消息落库，并推给所有在线客户端。
//
// 返回 inserted=false 表示这条消息此前已经落过（平台重推、重连补拉），
// 此时不重复推送。
func (c *Core) Ingest(ctx context.Context, botID string, in platform.Inbound) (bool, error) {
	if in.MessageID == "" {
		return false, fmt.Errorf("%w: 入站消息缺少 msg_id", store.ErrInvalid)
	}
	ts := in.At.UnixMilli()
	if in.At.IsZero() {
		ts = c.now().UnixMilli()
	}
	kind := in.Kind
	if kind == "" {
		kind = "text"
	}
	msg := store.Message{
		BotID:      botID,
		MsgID:      in.MessageID,
		PeerID:     in.Peer.ID,
		PeerKind:   in.Peer.Kind,
		Direction:  store.DirectionIn,
		SenderID:   in.SenderID,
		SenderName: in.SenderName,
		Type:       kind,
		TS:         ts,
		Plain:      in.Text,
		Doc:        in.Doc,
	}
	inserted, err := c.opt.Store.AppendMessage(ctx, msg)
	if err != nil {
		return false, err
	}
	if inserted && c.opt.Broadcaster != nil {
		c.opt.Broadcaster.Broadcast(protocol.TypeMsgInbound, toProtocolMessage(msg))
	}
	return inserted, nil
}

func toProtocolMessage(m store.Message) protocol.MsgInbound {
	return protocol.MsgInbound{
		BotID:      m.BotID,
		PeerID:     m.PeerID,
		PeerKind:   m.PeerKind,
		SenderID:   m.SenderID,
		SenderName: m.SenderName,
		MsgType:    m.Type,
		Body:       protocol.Body{Plain: m.Plain, Doc: m.Doc},
		MsgID:      m.MsgID,
		TS:         m.TS,
		Direction:  m.Direction,
	}
}

func toProtocolBot(b store.Bot) protocol.Bot {
	caps := b.Caps
	if caps == nil {
		caps = []string{}
	}
	limits := b.Limits
	if limits == nil {
		limits = map[string]any{}
	}
	constraints := b.Constraints
	if constraints == nil {
		constraints = map[string]any{}
	}
	bot := protocol.Bot{
		BotID:       b.ID,
		Platform:    b.Platform,
		Username:    b.Username,
		DisplayName: b.DisplayName,
		Status:      b.Status.State,
		Caps:        caps,
		Limits:      limits,
		Constraints: constraints,
	}
	if !b.Status.Since.IsZero() {
		bot.Since = b.Status.Since.UnixMilli()
	}
	return bot
}

func accountOf(b store.Bot) platform.Account {
	_, external, _ := platform.SplitID(b.ID)
	return platform.Account{
		ID:          external,
		Username:    b.Username,
		DisplayName: b.DisplayName,
		Caps:        b.Caps,
		MaxTextLen:  maxTextLen(b),
	}
}

func maxTextLen(b store.Bot) int {
	v, ok := b.Limits["max_text_len"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}

func canInitiate(b store.Bot) bool {
	v, ok := b.Constraints["can_initiate"]
	if !ok {
		return true
	}
	allowed, ok := v.(bool)
	if !ok {
		return true
	}
	return allowed
}

func bad(code, msg string) *protocol.Error {
	return &protocol.Error{Code: code, Message: msg, Retryable: protocol.Retryable(code)}
}

// internal 只回通用文案：协议 §10 要求错误里不出现内部路径与细节。
func internal(_ error) *protocol.Error {
	return &protocol.Error{
		Code:      protocol.CodeInternal,
		Message:   "服务端内部错误",
		Retryable: true,
	}
}

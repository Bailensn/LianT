package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Bailensn/LianT/service/internal/format"
	"github.com/Bailensn/LianT/service/internal/platform"
)

// 消息方向。
const (
	DirectionIn  = "in"
	DirectionOut = "out"
)

// 分页大小。
const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// Message 是一条已落盘的消息。
//
// TS 是 Unix 毫秒；Doc 为 nil 表示平台给不出结构化内容，只按 Plain 降级。
type Message struct {
	BotID      string
	MsgID      string
	PeerID     string
	PeerKind   string
	Direction  string
	SenderID   string
	SenderName string
	Type       string
	TS         int64
	Plain      string
	Doc        *format.Doc
	InReplyTo  string
	OperatorID string
}

// Cursor 是分页游标，排序键为 (ts, msg_id)。
type Cursor struct {
	TS    int64
	MsgID string
}

// ListQuery 描述一次会话内分页。
//
// Before 取更旧的一页，After 取更新的一页；两者都为空时取最新一页。
// 返回的消息始终按 (ts, msg_id) 升序排列。
type ListQuery struct {
	BotID  string
	PeerID string
	Before *Cursor
	After  *Cursor
	Limit  int
}

// Peer 是收件箱里的一个会话。
type Peer struct {
	ID          string
	Kind        string
	DisplayName string
	LastTS      int64
	LastMsgID   string
	LastPlain   string
	Count       int
}

func (m Message) validate() error {
	if !platform.IsValidID(m.BotID) {
		return fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, m.BotID)
	}
	if m.MsgID == "" {
		return fmt.Errorf("%w: 空 msg_id", ErrInvalid)
	}
	if !platform.IsValidID(m.PeerID) {
		return fmt.Errorf("%w: 非法 peer_id %q", ErrInvalid, m.PeerID)
	}
	botPlatform, _, err := platform.SplitID(m.BotID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	peerPlatform, _, err := platform.SplitID(m.PeerID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if botPlatform != peerPlatform {
		return fmt.Errorf("%w: peer 平台 %q 与 bot 平台 %q 不一致", ErrInvalid, peerPlatform, botPlatform)
	}
	switch m.PeerKind {
	case platform.PeerPrivate, platform.PeerGroup, platform.PeerChannel:
	default:
		return fmt.Errorf("%w: 未知 peer_kind %q", ErrInvalid, m.PeerKind)
	}
	switch m.Direction {
	case DirectionIn, DirectionOut:
	default:
		return fmt.Errorf("%w: 未知 direction %q", ErrInvalid, m.Direction)
	}
	if m.Doc != nil {
		if err := format.Validate(m.Doc); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	}
	return nil
}

// AppendMessage 落盘一条消息，按 (bot_id, msg_id) 幂等。
//
// 重复写入返回 inserted=false 且不报错——平台重推、客户端重试都会走到这里。
// 首次写入会顺带更新会话表的最近消息指针。
func (s *Store) AppendMessage(ctx context.Context, m Message) (inserted bool, err error) {
	if err := m.validate(); err != nil {
		return false, err
	}
	ts := m.TS
	if ts <= 0 {
		ts = s.nowMillis()
	}
	var doc []byte
	if m.Doc != nil {
		doc, err = json.Marshal(m.Doc)
		if err != nil {
			return false, fmt.Errorf("store: 序列化 doc: %w", err)
		}
	}
	msgType := m.Type
	if msgType == "" {
		msgType = "text"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: 开始事务: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
INSERT INTO messages (
	bot_id, msg_id, peer_id, peer_kind, direction, sender_id, sender_name,
	msg_type, ts, body_plain, body_doc, in_reply_to, operator_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bot_id, msg_id) DO NOTHING`,
		m.BotID, m.MsgID, m.PeerID, m.PeerKind, m.Direction, m.SenderID, m.SenderName,
		msgType, ts, m.Plain, doc, m.InReplyTo, m.OperatorID, s.nowMillis())
	if err != nil {
		return false, fmt.Errorf("store: 写入消息 %s: %w", m.MsgID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: 写入消息 %s: %w", m.MsgID, err)
	}
	if n == 0 {
		return false, nil
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO peers (bot_id, peer_id, kind, last_ts, last_msg_id)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(bot_id, peer_id) DO UPDATE SET
	kind        = excluded.kind,
	last_ts     = excluded.last_ts,
	last_msg_id = excluded.last_msg_id
WHERE excluded.last_ts > peers.last_ts
   OR (excluded.last_ts = peers.last_ts AND excluded.last_msg_id > peers.last_msg_id)`,
		m.BotID, m.PeerID, m.PeerKind, ts, m.MsgID)
	if err != nil {
		return false, fmt.Errorf("store: 更新会话 %s: %w", m.PeerID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: 提交消息 %s: %w", m.MsgID, err)
	}
	return true, nil
}

// Message 按 (bot_id, msg_id) 取一条消息。
func (s *Store) Message(ctx context.Context, botID, msgID string) (Message, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT bot_id, msg_id, peer_id, peer_kind, direction, sender_id, sender_name,
       msg_type, ts, body_plain, body_doc, in_reply_to, operator_id
FROM messages WHERE bot_id = ? AND msg_id = ?`, botID, msgID)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, fmt.Errorf("%w: 消息 %s/%s", ErrNotFound, botID, msgID)
	}
	if err != nil {
		return Message{}, fmt.Errorf("store: 读取消息 %s/%s: %w", botID, msgID, err)
	}
	return m, nil
}

// Messages 返回会话内一页消息，hasMore 表示该方向上还有更早/更新的消息。
func (s *Store) Messages(ctx context.Context, q ListQuery) ([]Message, bool, error) {
	if !platform.IsValidID(q.BotID) {
		return nil, false, fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, q.BotID)
	}
	if !platform.IsValidID(q.PeerID) {
		return nil, false, fmt.Errorf("%w: 非法 peer_id %q", ErrInvalid, q.PeerID)
	}
	if q.Before != nil && q.After != nil {
		return nil, false, fmt.Errorf("%w: before 与 after 不能同时出现", ErrInvalid)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	const cols = `bot_id, msg_id, peer_id, peer_kind, direction, sender_id, sender_name,
	              msg_type, ts, body_plain, body_doc, in_reply_to, operator_id`

	var (
		query string
		args  []any
		desc  bool
	)
	switch {
	case q.After != nil:
		// 更新的一页：从游标往后取，天然升序。
		query = `SELECT ` + cols + ` FROM messages
		         WHERE bot_id = ? AND peer_id = ?
		           AND (ts > ? OR (ts = ? AND msg_id > ?))
		         ORDER BY ts ASC, msg_id ASC LIMIT ?`
		args = []any{q.BotID, q.PeerID, q.After.TS, q.After.TS, q.After.MsgID, limit + 1}
	case q.Before != nil:
		// 更旧的一页：倒序取，最后再翻回升序。
		desc = true
		query = `SELECT ` + cols + ` FROM messages
		         WHERE bot_id = ? AND peer_id = ?
		           AND (ts < ? OR (ts = ? AND msg_id < ?))
		         ORDER BY ts DESC, msg_id DESC LIMIT ?`
		args = []any{q.BotID, q.PeerID, q.Before.TS, q.Before.TS, q.Before.MsgID, limit + 1}
	default:
		// 最新一页，同样倒序取再翻回。
		desc = true
		query = `SELECT ` + cols + ` FROM messages
		         WHERE bot_id = ? AND peer_id = ?
		         ORDER BY ts DESC, msg_id DESC LIMIT ?`
		args = []any{q.BotID, q.PeerID, limit + 1}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("store: 查询消息: %w", err)
	}
	defer rows.Close()

	out := make([]Message, 0, limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, false, fmt.Errorf("store: 读取消息行: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("store: 遍历消息: %w", err)
	}

	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	if desc {
		reverse(out)
	}
	return out, hasMore, nil
}

// ReplyCount 统计某条入站消息已经被回复了多少次（出站消息按 in_reply_to 关联）。
func (s *Store) ReplyCount(ctx context.Context, botID, inReplyTo string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM messages
WHERE bot_id = ? AND in_reply_to = ? AND direction = ?`,
		botID, inReplyTo, DirectionOut).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: 统计回复数: %w", err)
	}
	return n, nil
}

// Peer 按 (bot_id, peer_id) 取一个会话；不存在时返回 ErrNotFound。
func (s *Store) Peer(ctx context.Context, botID, peerID string) (Peer, error) {
	if !platform.IsValidID(botID) {
		return Peer{}, fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, botID)
	}
	if !platform.IsValidID(peerID) {
		return Peer{}, fmt.Errorf("%w: 非法 peer_id %q", ErrInvalid, peerID)
	}
	var p Peer
	err := s.db.QueryRowContext(ctx, `
SELECT p.peer_id, p.kind, p.display_name, p.last_ts, p.last_msg_id,
       COALESCE((SELECT m.body_plain FROM messages m
                 WHERE m.bot_id = p.bot_id AND m.msg_id = p.last_msg_id), ''),
       (SELECT COUNT(*) FROM messages m2
        WHERE m2.bot_id = p.bot_id AND m2.peer_id = p.peer_id)
FROM peers p WHERE p.bot_id = ? AND p.peer_id = ?`, botID, peerID).
		Scan(&p.ID, &p.Kind, &p.DisplayName, &p.LastTS, &p.LastMsgID, &p.LastPlain, &p.Count)
	if errors.Is(err, sql.ErrNoRows) {
		return Peer{}, fmt.Errorf("%w: 会话 %s/%s", ErrNotFound, botID, peerID)
	}
	if err != nil {
		return Peer{}, fmt.Errorf("store: 读取会话 %s/%s: %w", botID, peerID, err)
	}
	return p, nil
}

// Peers 返回某个 Bot 的会话列表，按最近消息倒序。
func (s *Store) Peers(ctx context.Context, botID string) ([]Peer, error) {
	if !platform.IsValidID(botID) {
		return nil, fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, botID)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT p.peer_id, p.kind, p.display_name, p.last_ts, p.last_msg_id,
       COALESCE((SELECT m.body_plain FROM messages m
                 WHERE m.bot_id = p.bot_id AND m.msg_id = p.last_msg_id), ''),
       (SELECT COUNT(*) FROM messages m2
        WHERE m2.bot_id = p.bot_id AND m2.peer_id = p.peer_id)
FROM peers p
WHERE p.bot_id = ?
ORDER BY p.last_ts DESC, p.peer_id DESC`, botID)
	if err != nil {
		return nil, fmt.Errorf("store: 列出会话: %w", err)
	}
	defer rows.Close()

	var out []Peer
	for rows.Next() {
		var p Peer
		if err := rows.Scan(&p.ID, &p.Kind, &p.DisplayName, &p.LastTS, &p.LastMsgID, &p.LastPlain, &p.Count); err != nil {
			return nil, fmt.Errorf("store: 读取会话行: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 遍历会话: %w", err)
	}
	return out, nil
}

func scanMessage(r rowScanner) (Message, error) {
	var (
		m   Message
		doc []byte
	)
	if err := r.Scan(
		&m.BotID, &m.MsgID, &m.PeerID, &m.PeerKind, &m.Direction,
		&m.SenderID, &m.SenderName, &m.Type, &m.TS, &m.Plain,
		&doc, &m.InReplyTo, &m.OperatorID,
	); err != nil {
		return Message{}, err
	}
	if len(doc) > 0 {
		var d format.Doc
		if err := json.Unmarshal(doc, &d); err != nil {
			return Message{}, fmt.Errorf("解析 doc: %w", err)
		}
		m.Doc = &d
	}
	return m, nil
}

func reverse(ms []Message) {
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
}

// SetPeerDisplayName 写入会话标题（群名等）；没有该会话时静默忽略。
func (s *Store) SetPeerDisplayName(ctx context.Context, botID, peerID, name string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE peers SET display_name = ? WHERE bot_id = ? AND peer_id = ?`, name, botID, peerID)
	if err != nil {
		return fmt.Errorf("store: 更新会话标题: %w", err)
	}
	return nil
}

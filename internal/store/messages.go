package store

import (
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// 聊天消息持久化
//
// 消息只存在每个 Bot 自己的业务库（bots/<id>.db），不设条数上限：
//   * 按 (chat_id, message_id) 去重，重复投递只更新状态；
//   * 撤回是墓碑：保留发送者与时间，正文与媒体引用一并清空；
//   * at / updated_at 统一用 UTC 定宽文本，字典序与时间序一致（SSE 靠它做快照）。
//
// MediaJSON 是 media.Ref 的 JSON，含 Telegram file_id / QQ 签名附件直链；
// 只在本机 SQLite 与任务进程之间流转，不直接进 API。
// ============================================================================

const messageTimeLayout = "2006-01-02T15:04:05.000000000Z"

func messageTime(t time.Time) string { return t.UTC().Format(messageTimeLayout) }

func parseMessageTime(s string) time.Time {
	t, _ := time.Parse(messageTimeLayout, s)
	return t
}

// ConversationKey 把平台会话 + 话题/频道拼成稳定的本地会话键。
// 普通群聊/私聊 threadID 为空，键就是 chat_id，兼容旧数据。
func ConversationKey(chatID, threadID string) string {
	chatID = strings.TrimSpace(chatID)
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return chatID
	}
	return chatID + "::" + threadID
}

// ConversationRec 是会话目录的一项（群、话题、私聊、频道）。
type ConversationRec struct {
	Key        string
	ChatID     string
	ThreadID   string
	Kind       string // group | topic | private | channel
	Title      string
	TitleFinal bool
	Note       string
	UpdatedAt  time.Time
}

// MessageRec 是持久化的一条聊天消息。
type MessageRec struct {
	Seq            int64
	ChatID         string
	MessageID      string
	MsgIdx         string
	ConvKey        string
	ThreadID       string
	ChatKind       string
	ChatTitle      string
	ChatTitleFinal bool
	UserID         string
	Username       string
	Content        string
	Kind           string
	IsBot          bool
	State          string
	MediaJSON      string
	At             time.Time
	UpdatedAt      time.Time
}

const messageColumns = `seq, chat_id, message_id, msg_idx, conv_key, thread_id,
	user_id, username, content, kind, is_bot, state, media, at, updated_at`

type messageScanner interface {
	Scan(dest ...interface{}) error
}

func scanMessage(sc messageScanner) (MessageRec, error) {
	var (
		m             MessageRec
		isBot         int
		at, updatedAt string
	)
	err := sc.Scan(&m.Seq, &m.ChatID, &m.MessageID, &m.MsgIdx, &m.ConvKey, &m.ThreadID,
		&m.UserID, &m.Username, &m.Content, &m.Kind, &isBot, &m.State, &m.MediaJSON, &at, &updatedAt)
	if err != nil {
		return MessageRec{}, err
	}
	m.IsBot = isBot != 0
	m.At = parseMessageTime(at)
	m.UpdatedAt = parseMessageTime(updatedAt)
	return m, nil
}

// SaveMessage 写入或更新一条消息，并同步维护会话目录。
func (s *Store) SaveMessage(m MessageRec) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("存储未就绪")
	}
	m.ChatID = strings.TrimSpace(m.ChatID)
	m.MessageID = strings.TrimSpace(m.MessageID)
	m.ThreadID = strings.TrimSpace(m.ThreadID)
	if m.ChatID == "" || m.MessageID == "" {
		return fmt.Errorf("消息缺少 chat_id / message_id")
	}
	if strings.TrimSpace(m.ConvKey) == "" {
		m.ConvKey = ConversationKey(m.ChatID, m.ThreadID)
	}
	if strings.TrimSpace(m.ChatKind) == "" {
		m.ChatKind = "group"
	}
	if strings.TrimSpace(m.State) == "" {
		m.State = "received"
	}
	now := time.Now()
	if m.At.IsZero() {
		m.At = now
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.saveConversationLocked(m, now); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO messages(chat_id, message_id, msg_idx, conv_key, thread_id,
			user_id, username, content, kind, is_bot, state, media, at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(chat_id, message_id) DO UPDATE SET
			msg_idx   = excluded.msg_idx,
			conv_key  = excluded.conv_key,
			thread_id = excluded.thread_id,
			user_id   = excluded.user_id,
			username  = excluded.username,
			content   = CASE WHEN messages.state = 'recalled' THEN '' ELSE excluded.content END,
			kind      = excluded.kind,
			is_bot    = excluded.is_bot,
			state     = CASE WHEN messages.state = 'recalled' THEN 'recalled' ELSE excluded.state END,
			media     = CASE WHEN messages.state = 'recalled' THEN '' ELSE excluded.media END,
			updated_at = excluded.updated_at`,
		m.ChatID, m.MessageID, m.MsgIdx, m.ConvKey, m.ThreadID,
		m.UserID, m.Username, m.Content, m.Kind,
		BoolInt(m.IsBot), m.State, m.MediaJSON, messageTime(m.At), messageTime(now))
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) saveConversationLocked(m MessageRec, now time.Time) error {
	kind := strings.TrimSpace(m.ChatKind)
	if kind == "" {
		kind = "group"
	}
	title := strings.TrimSpace(m.ChatTitle)
	_, err := s.db.Exec(`INSERT INTO conversations(key, chat_id, thread_id, kind, title, title_final, updated_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET
			chat_id     = excluded.chat_id,
			thread_id   = excluded.thread_id,
			kind        = CASE WHEN excluded.kind <> '' THEN excluded.kind ELSE conversations.kind END,
			title       = CASE
				WHEN excluded.title_final = 1 AND excluded.title <> '' THEN excluded.title
				WHEN conversations.title_final = 1 AND conversations.title <> '' THEN conversations.title
				WHEN excluded.title <> '' THEN excluded.title
				ELSE conversations.title END,
			title_final = CASE WHEN excluded.title_final = 1 OR conversations.title_final = 1 THEN 1 ELSE 0 END,
			updated_at  = excluded.updated_at`,
		m.ConvKey, m.ChatID, m.ThreadID, kind, title, BoolInt(m.ChatTitleFinal), messageTime(now))
	return err
}

// Messages 取一个会话的消息（时间顺序）。
// limit <= 0 表示不设上限；since 非零时只取该时间之后。
func (s *Store) Messages(chatID string, limit int, since time.Time) ([]MessageRec, error) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" {
		return nil, nil
	}
	q := `SELECT ` + messageColumns + ` FROM messages WHERE chat_id = ?`
	args := []interface{}{chatID}
	if !since.IsZero() {
		q += ` AND at >= ?`
		args = append(args, messageTime(since))
	}
	q += ` ORDER BY seq DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageRec
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 查询是倒序（为了 LIMIT 取最新），返回前恢复时间顺序。
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Message 按消息 ID 查一条消息。
func (s *Store) Message(chatID, messageID string) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" || strings.TrimSpace(messageID) == "" {
		return MessageRec{}, false
	}
	row := s.db.QueryRow(`SELECT `+messageColumns+` FROM messages WHERE chat_id = ? AND message_id = ?
		ORDER BY seq DESC LIMIT 1`, chatID, messageID)
	m, err := scanMessage(row)
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// MessageByMsgIdx 按 QQ 的 msg_idx 查一条消息（引用/撤回用）。
func (s *Store) MessageByMsgIdx(chatID, msgIdx string) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" || strings.TrimSpace(msgIdx) == "" {
		return MessageRec{}, false
	}
	row := s.db.QueryRow(`SELECT `+messageColumns+` FROM messages WHERE chat_id = ? AND msg_idx = ?
		ORDER BY seq DESC LIMIT 1`, chatID, msgIdx)
	m, err := scanMessage(row)
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// LastMessageByMember 找某成员最后一条消息。
func (s *Store) LastMessageByMember(chatID, userID string, since time.Time) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" || strings.TrimSpace(userID) == "" {
		return MessageRec{}, false
	}
	q := `SELECT ` + messageColumns + ` FROM messages WHERE chat_id = ? AND user_id = ?`
	args := []interface{}{chatID, userID}
	if !since.IsZero() {
		q += ` AND at >= ?`
		args = append(args, messageTime(since))
	}
	q += ` ORDER BY seq DESC LIMIT 1`
	m, err := scanMessage(s.db.QueryRow(q, args...))
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// LastBotMessage 找机器人自己最后一条消息。
func (s *Store) LastBotMessage(chatID string, since time.Time) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" {
		return MessageRec{}, false
	}
	q := `SELECT ` + messageColumns + ` FROM messages WHERE chat_id = ? AND is_bot = 1`
	args := []interface{}{chatID}
	if !since.IsZero() {
		q += ` AND at >= ?`
		args = append(args, messageTime(since))
	}
	q += ` ORDER BY seq DESC LIMIT 1`
	m, err := scanMessage(s.db.QueryRow(q, args...))
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// RecallMessage 把消息标记成撤回墓碑：正文与媒体引用清空，保留发送者与时间。
func (s *Store) RecallMessage(chatID, messageID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("存储未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE messages SET state = 'recalled', content = '', media = '', updated_at = ?
		WHERE chat_id = ? AND message_id = ?`, messageTime(time.Now()), chatID, messageID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE key =
			(SELECT conv_key FROM messages WHERE chat_id = ? AND message_id = ?)`,
			messageTime(time.Now()), chatID, messageID)
		s.bump()
	}
	return nil
}

// SetMessageMedia 只更新一条消息的媒体引用（取回失败时标记过期/不存在）。
func (s *Store) SetMessageMedia(chatID, messageID, mediaJSON string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("存储未就绪")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE messages SET media = ?, updated_at = ?
		WHERE chat_id = ? AND message_id = ?`, mediaJSON, messageTime(time.Now()), chatID, messageID)
	if err == nil {
		_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE key =
			(SELECT conv_key FROM messages WHERE chat_id = ? AND message_id = ?)`,
			messageTime(time.Now()), chatID, messageID)
		s.bump()
	}
	return err
}

// MessageSignature 是一个会话的消息快照签名：最大序号 + 最后更新时间。
// 两者都走索引，历史再长也只做 O(log n) 查询；新增消息会推高 seq，
// 撤回/媒体状态会推高 updated_at，所以足以判断"有没有变化"。
func (s *Store) MessageSignature(chatID string) string {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" {
		return ""
	}
	var (
		maxSeq int64
		last   string
	)
	err := s.db.QueryRow(`SELECT
		COALESCE((SELECT MAX(seq) FROM messages WHERE chat_id = ?), 0),
		COALESCE((SELECT updated_at FROM messages WHERE chat_id = ?
			ORDER BY updated_at DESC LIMIT 1), '')`, chatID, chatID).Scan(&maxSeq, &last)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", maxSeq, last)
}

// MessageCount 返回整个业务库的消息条数（CLI 统计用）。
func (s *Store) MessageCount() int64 {
	if s == nil || s.db == nil {
		return 0
	}
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		return 0
	}
	return n
}

// LastMessage 取一个会话最后一条消息（会话列表预览用）。
func (s *Store) LastMessage(chatID string) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(chatID) == "" {
		return MessageRec{}, false
	}
	m, err := scanMessage(s.db.QueryRow(`SELECT `+messageColumns+` FROM messages
		WHERE chat_id = ? ORDER BY seq DESC LIMIT 1`, chatID))
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// MessagesByConversation 取一个具体会话（群/话题/私聊/频道）的消息。
func (s *Store) MessagesByConversation(convKey string, limit int, since time.Time) ([]MessageRec, error) {
	if s == nil || s.db == nil || strings.TrimSpace(convKey) == "" {
		return nil, nil
	}
	q := `SELECT ` + messageColumns + ` FROM messages WHERE conv_key = ?`
	args := []interface{}{convKey}
	if !since.IsZero() {
		q += ` AND at >= ?`
		args = append(args, messageTime(since))
	}
	q += ` ORDER BY seq DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageRec
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// LastMessageByConversation 取一个具体会话的最后一条消息。
func (s *Store) LastMessageByConversation(convKey string) (MessageRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(convKey) == "" {
		return MessageRec{}, false
	}
	m, err := scanMessage(s.db.QueryRow(`SELECT `+messageColumns+` FROM messages
		WHERE conv_key = ? ORDER BY seq DESC LIMIT 1`, convKey))
	if err != nil {
		return MessageRec{}, false
	}
	return m, true
}

// Conversations 返回会话目录（按最近活动倒序）。
func (s *Store) Conversations() ([]ConversationRec, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT key, chat_id, thread_id, kind, title, title_final, note, updated_at
		FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConversationRec
	for rows.Next() {
		var (
			c        ConversationRec
			isFinal  int
			updated  string
		)
		if err := rows.Scan(&c.Key, &c.ChatID, &c.ThreadID, &c.Kind, &c.Title, &isFinal, &c.Note, &updated); err != nil {
			return nil, err
		}
		c.TitleFinal = isFinal != 0
		c.UpdatedAt = parseMessageTime(updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Conversation 按本地会话键查会话。
func (s *Store) Conversation(key string) (ConversationRec, bool) {
	if s == nil || s.db == nil || strings.TrimSpace(key) == "" {
		return ConversationRec{}, false
	}
	var (
		c       ConversationRec
		isFinal int
		updated string
	)
	err := s.db.QueryRow(`SELECT key, chat_id, thread_id, kind, title, title_final, note, updated_at
		FROM conversations WHERE key = ?`, key).
		Scan(&c.Key, &c.ChatID, &c.ThreadID, &c.Kind, &c.Title, &isFinal, &c.Note, &updated)
	if err != nil {
		return ConversationRec{}, false
	}
	c.TitleFinal = isFinal != 0
	c.UpdatedAt = parseMessageTime(updated)
	return c, true
}

// ConversationSignature 是具体会话的 SSE 快照签名。
func (s *Store) ConversationSignature(convKey string) string {
	if s == nil || s.db == nil || strings.TrimSpace(convKey) == "" {
		return ""
	}
	var (
		maxSeq int64
		last   string
	)
	err := s.db.QueryRow(`SELECT
		COALESCE((SELECT MAX(seq) FROM messages WHERE conv_key = ?), 0),
		COALESCE((SELECT updated_at FROM messages WHERE conv_key = ?
			ORDER BY updated_at DESC LIMIT 1), '')`, convKey, convKey).Scan(&maxSeq, &last)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", maxSeq, last)
}

// EnsureConversation 只写会话目录，不写消息（用于话题创建等服务消息）。
func (s *Store) EnsureConversation(c ConversationRec) error {
	if s == nil || s.db == nil || strings.TrimSpace(c.Key) == "" {
		return nil
	}
	if strings.TrimSpace(c.Kind) == "" {
		c.Kind = "group"
	}
	m := MessageRec{
		ConvKey: c.Key, ChatID: c.ChatID, ThreadID: c.ThreadID,
		ChatKind: c.Kind, ChatTitle: c.Title, ChatTitleFinal: c.TitleFinal,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveConversationLocked(m, time.Now())
}

// ConversationsSignature 是 Bot 级会话目录签名：会话数量 + 最近更新时间。
// 新增消息、撤回、媒体状态变化都会 touch 会话，SSE 据此刷新会话列表。
func (s *Store) ConversationsSignature() string {
	if s == nil || s.db == nil {
		return ""
	}
	var (
		n       int64
		maxConv string
	)
	err := s.db.QueryRow(`SELECT
		COALESCE((SELECT COUNT(*) FROM conversations), 0),
		COALESCE((SELECT MAX(updated_at) FROM conversations), '')`).Scan(&n, &maxConv)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%s", n, maxConv)
}

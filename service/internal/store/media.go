package store

import (
	"context"
	"fmt"

	"github.com/Bailensn/LianT/service/internal/platform"
)

// 媒体状态。与 docs/PROTOCOL.md §8 的两种失败口径对应：
// expired 是超过保留期，evicted 是单 Bot 空间不足被淘汰。
const (
	MediaPending = "pending"
	MediaReady   = "ready"
	MediaExpired = "expired"
	MediaEvicted = "evicted"
)

// Media 是一条消息附带的媒体文件记录。
//
// Path 是 Service 本地缓存路径；客户端拿到的下载地址由 media.fetch 生成，
// 与这里的 Path 无关。Size 为字节数，用于单 Bot 配额计量。
type Media struct {
	BotID string
	MsgID string
	Index int
	MIME  string
	Path  string
	Size  int64
	State string
}

func validMediaState(state string) bool {
	switch state {
	case MediaPending, MediaReady, MediaExpired, MediaEvicted:
		return true
	default:
		return false
	}
}

// PutMedia 写入或更新一条媒体记录。
func (s *Store) PutMedia(ctx context.Context, m Media) error {
	if !platform.IsValidID(m.BotID) {
		return fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, m.BotID)
	}
	if m.MsgID == "" {
		return fmt.Errorf("%w: 空 msg_id", ErrInvalid)
	}
	if m.Index < 0 {
		return fmt.Errorf("%w: 负的媒体序号 %d", ErrInvalid, m.Index)
	}
	if m.Size < 0 {
		return fmt.Errorf("%w: 负的媒体大小 %d", ErrInvalid, m.Size)
	}
	if m.State == "" {
		m.State = MediaPending
	}
	if !validMediaState(m.State) {
		return fmt.Errorf("%w: 未知媒体状态 %q", ErrInvalid, m.State)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO media (bot_id, msg_id, idx, mime, path, size, state)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bot_id, msg_id, idx) DO UPDATE SET
	mime  = excluded.mime,
	path  = excluded.path,
	size  = excluded.size,
	state = excluded.state`,
		m.BotID, m.MsgID, m.Index, m.MIME, m.Path, m.Size, m.State)
	if err != nil {
		return fmt.Errorf("store: 写入媒体 %s/%s: %w", m.BotID, m.MsgID, err)
	}
	return nil
}

// MediaOf 返回一条消息的全部媒体记录，按序号升序。
func (s *Store) MediaOf(ctx context.Context, botID, msgID string) ([]Media, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT bot_id, msg_id, idx, mime, path, size, state
FROM media WHERE bot_id = ? AND msg_id = ? ORDER BY idx`, botID, msgID)
	if err != nil {
		return nil, fmt.Errorf("store: 查询媒体: %w", err)
	}
	defer rows.Close()

	var out []Media
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.BotID, &m.MsgID, &m.Index, &m.MIME, &m.Path, &m.Size, &m.State); err != nil {
			return nil, fmt.Errorf("store: 读取媒体行: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 遍历媒体: %w", err)
	}
	return out, nil
}

// SetMediaState 更新一条媒体记录的状态。
func (s *Store) SetMediaState(ctx context.Context, botID, msgID string, index int, state string) error {
	if !validMediaState(state) {
		return fmt.Errorf("%w: 未知媒体状态 %q", ErrInvalid, state)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE media SET state = ? WHERE bot_id = ? AND msg_id = ? AND idx = ?`,
		state, botID, msgID, index)
	if err != nil {
		return fmt.Errorf("store: 更新媒体状态: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: 更新媒体状态: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: 媒体 %s/%s#%d", ErrNotFound, botID, msgID, index)
	}
	return nil
}

// MediaUsage 返回某个 Bot 已落盘媒体的总字节数与文件数（只计 ready）。
func (s *Store) MediaUsage(ctx context.Context, botID string) (int64, int, error) {
	var (
		total int64
		count int
	)
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(size), 0), COUNT(*)
FROM media WHERE bot_id = ? AND state = ?`, botID, MediaReady).Scan(&total, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("store: 统计媒体占用: %w", err)
	}
	return total, count, nil
}


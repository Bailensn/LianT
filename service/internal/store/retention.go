package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Retention 是 Service 侧的保留策略（docs/DESIGN.md D2）。
//
// 按时间承诺：Text 是消息的保留期，Media 是媒体文件的保留期，
// 两者都必须 ≥ 7 天；MediaBytes 是单 Bot 媒体的硬上限（超限按时间淘汰最旧的）。
type Retention struct {
	Text       time.Duration
	Media      time.Duration
	MediaBytes int64
}

// MinRetention 是保留期的下限：可调大、不可调小。
const MinRetention = 7 * 24 * time.Hour

// DefaultMediaBytes 是单 Bot 媒体的默认上限：3 GB。
const DefaultMediaBytes int64 = 3 << 30

// DefaultRetention 返回默认策略：不主动限制消息时间或媒体总量。
// 实际能存多久只受磁盘容量限制；平台单条消息/媒体的上限仍由平台决定。
func DefaultRetention() Retention {
	return Retention{
		Text:       100 * 365 * 24 * time.Hour,
		Media:      100 * 365 * 24 * time.Hour,
		MediaBytes: 1 << 62,
	}
}

func (r Retention) validate() error {
	if r.Text < MinRetention {
		return fmt.Errorf("%w: 文本保留期 %s 短于下限 %s", ErrInvalid, r.Text, MinRetention)
	}
	if r.Media < MinRetention {
		return fmt.Errorf("%w: 媒体保留期 %s 短于下限 %s", ErrInvalid, r.Media, MinRetention)
	}
	if r.MediaBytes <= 0 {
		return fmt.Errorf("%w: 媒体上限必须为正数", ErrInvalid)
	}
	return nil
}

// PurgeResult 是一次清理的结果。
//
// Files 是被删除/标记失效的媒体文件路径，由调用方负责从磁盘上 unlink；
// store 只管数据库，不碰文件系统。
type PurgeResult struct {
	Messages     int64
	MediaExpired int64
	MediaEvicted int64
	Files        []string
}

// Purge 按保留策略清理过期消息与媒体，并对超额的媒体做淘汰。
//
// 语义（docs/PROTOCOL.md §8）：
//   - 文本超过保留期即删，连带删除其媒体记录；
//   - 媒体超过保留期标为 expired（消息本身可能还在）；
//   - 单 Bot 媒体超过配额时按消息时间从旧到新标为 evicted。
//
// 已标记 expired / evicted 的记录不会再被重复计数或再次标记。
func (s *Store) Purge(ctx context.Context, r Retention, now time.Time) (PurgeResult, error) {
	var res PurgeResult
	if err := r.validate(); err != nil {
		return res, err
	}
	textCutoff := now.Add(-r.Text).UnixMilli()
	mediaCutoff := now.Add(-r.Media).UnixMilli()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("store: 开始清理事务: %w", err)
	}
	defer tx.Rollback()

	// 1. 过期消息：先收媒体路径，再删消息（外键级联删媒体记录）。
	rows, err := tx.QueryContext(ctx, `
SELECT m.path FROM media m
JOIN messages g ON g.bot_id = m.bot_id AND g.msg_id = m.msg_id
WHERE g.ts < ? AND m.path <> ''`, textCutoff)
	if err != nil {
		return res, fmt.Errorf("store: 查询待删媒体: %w", err)
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return res, fmt.Errorf("store: 读取待删媒体: %w", err)
		}
		res.Files = append(res.Files, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, fmt.Errorf("store: 遍历待删媒体: %w", err)
	}
	rows.Close()

	del, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE ts < ?`, textCutoff)
	if err != nil {
		return res, fmt.Errorf("store: 删除过期消息: %w", err)
	}
	if res.Messages, err = del.RowsAffected(); err != nil {
		return res, fmt.Errorf("store: 统计过期消息: %w", err)
	}

	// 2. 过期媒体：消息可能因文本保留期更长而仍在，只把媒体标成 expired。
	rows, err = tx.QueryContext(ctx, `
SELECT m.path FROM media m
JOIN messages g ON g.bot_id = m.bot_id AND g.msg_id = m.msg_id
WHERE g.ts < ? AND m.state IN (?, ?) AND m.path <> ''`,
		mediaCutoff, MediaPending, MediaReady)
	if err != nil {
		return res, fmt.Errorf("store: 查询过期媒体: %w", err)
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return res, fmt.Errorf("store: 读取过期媒体: %w", err)
		}
		res.Files = append(res.Files, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, fmt.Errorf("store: 遍历过期媒体: %w", err)
	}
	rows.Close()

	exp, err := tx.ExecContext(ctx, `
UPDATE media SET state = ?
WHERE state IN (?, ?)
  AND EXISTS (
    SELECT 1 FROM messages g
    WHERE g.bot_id = media.bot_id AND g.msg_id = media.msg_id AND g.ts < ?
  )`, MediaExpired, MediaPending, MediaReady, mediaCutoff)
	if err != nil {
		return res, fmt.Errorf("store: 标记过期媒体: %w", err)
	}
	if res.MediaExpired, err = exp.RowsAffected(); err != nil {
		return res, fmt.Errorf("store: 统计过期媒体: %w", err)
	}

	// 3. 单 Bot 媒体配额：按消息时间从旧到新淘汰，直到不超额。
	if err := s.enforceMediaCap(ctx, tx, r.MediaBytes, &res); err != nil {
		return res, err
	}

	// 4. 清掉已经没有消息的会话记录。
	if _, err := tx.ExecContext(ctx, `
DELETE FROM peers
WHERE NOT EXISTS (
  SELECT 1 FROM messages g
  WHERE g.bot_id = peers.bot_id AND g.peer_id = peers.peer_id
)`); err != nil {
		return res, fmt.Errorf("store: 清理空会话: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("store: 提交清理: %w", err)
	}
	return res, nil
}

func (s *Store) enforceMediaCap(ctx context.Context, tx *sql.Tx, capBytes int64, res *PurgeResult) error {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT bot_id FROM media WHERE state = ?`, MediaReady)
	if err != nil {
		return fmt.Errorf("store: 统计 Bot 媒体: %w", err)
	}
	var bots []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("store: 读取 Bot 媒体: %w", err)
		}
		bots = append(bots, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("store: 遍历 Bot 媒体: %w", err)
	}
	rows.Close()

	for _, botID := range bots {
		items, err := tx.QueryContext(ctx, `
SELECT m.msg_id, m.idx, m.size, m.path
FROM media m
JOIN messages g ON g.bot_id = m.bot_id AND g.msg_id = m.msg_id
WHERE m.bot_id = ? AND m.state = ?
ORDER BY g.ts ASC, g.msg_id ASC, m.idx ASC`, botID, MediaReady)
		if err != nil {
			return fmt.Errorf("store: 查询媒体配额: %w", err)
		}
		type item struct {
			msgID string
			idx   int
			size  int64
			path  string
		}
		var list []item
		var total int64
		for items.Next() {
			var it item
			if err := items.Scan(&it.msgID, &it.idx, &it.size, &it.path); err != nil {
				items.Close()
				return fmt.Errorf("store: 读取媒体配额: %w", err)
			}
			list = append(list, it)
			total += it.size
		}
		if err := items.Err(); err != nil {
			items.Close()
			return fmt.Errorf("store: 遍历媒体配额: %w", err)
		}
		items.Close()

		for _, it := range list {
			if total <= capBytes {
				break
			}
			if _, err := tx.ExecContext(ctx, `
UPDATE media SET state = ? WHERE bot_id = ? AND msg_id = ? AND idx = ?`,
				MediaEvicted, botID, it.msgID, it.idx); err != nil {
				return fmt.Errorf("store: 淘汰媒体: %w", err)
			}
			total -= it.size
			res.MediaEvicted++
			if it.path != "" {
				res.Files = append(res.Files, it.path)
			}
		}
	}
	return nil
}


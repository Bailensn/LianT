package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Bailensn/LianT/service/internal/platform"
)

// Bot 是一个已登记的 Bot。
//
// Credentials 是调用方（supervisor/协议层）用主密钥加密后的不透明字节；
// store 只负责原样存取，既不解析也不回显明文。
type Bot struct {
	ID          string
	Platform    string
	Label       string
	Credentials []byte
	Username    string
	DisplayName string
	Caps        []string
	Limits      map[string]any
	Constraints map[string]any
	Status      platform.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (b Bot) validate() error {
	if !platform.IsValidID(b.ID) {
		return fmt.Errorf("%w: 非法 bot_id %q", ErrInvalid, b.ID)
	}
	name, _, err := platform.SplitID(b.ID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if name != b.Platform {
		return fmt.Errorf("%w: bot_id 平台 %q 与 platform 字段 %q 不一致", ErrInvalid, name, b.Platform)
	}
	if len(b.Credentials) == 0 {
		return fmt.Errorf("%w: 凭据为空", ErrInvalid)
	}
	return nil
}

// UpsertBot 插入或更新一个 Bot 的元数据。
//
// created_at 只在首次插入时写入；更新不会改写它，也不会碰消息与会话。
func (s *Store) UpsertBot(ctx context.Context, b Bot) error {
	if err := b.validate(); err != nil {
		return err
	}
	caps, err := json.Marshal(nonNilSlice(b.Caps))
	if err != nil {
		return fmt.Errorf("store: 序列化 caps: %w", err)
	}
	limits, err := json.Marshal(nonNilMap(b.Limits))
	if err != nil {
		return fmt.Errorf("store: 序列化 limits: %w", err)
	}
	constraints, err := json.Marshal(nonNilMap(b.Constraints))
	if err != nil {
		return fmt.Errorf("store: 序列化 constraints: %w", err)
	}
	now := s.nowMillis()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO bots (
	bot_id, platform, label, credentials, username, display_name,
	caps, limits, constraints, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(bot_id) DO UPDATE SET
	label        = excluded.label,
	credentials  = excluded.credentials,
	username     = excluded.username,
	display_name = excluded.display_name,
	caps         = excluded.caps,
	limits       = excluded.limits,
	constraints  = excluded.constraints,
	updated_at   = excluded.updated_at`,
		b.ID, b.Platform, b.Label, b.Credentials, b.Username, b.DisplayName,
		string(caps), string(limits), string(constraints), now, now)
	if err != nil {
		return fmt.Errorf("store: 写入 bot %s: %w", b.ID, err)
	}
	return nil
}

// Bot 按 bot_id 取一个 Bot；不存在时返回 ErrNotFound。
func (s *Store) Bot(ctx context.Context, id string) (Bot, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT bot_id, platform, label, credentials, username, display_name,
       caps, limits, constraints, status, status_reason, status_since,
       created_at, updated_at
FROM bots WHERE bot_id = ?`, id)
	b, err := scanBot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Bot{}, fmt.Errorf("%w: bot %s", ErrNotFound, id)
	}
	if err != nil {
		return Bot{}, fmt.Errorf("store: 读取 bot %s: %w", id, err)
	}
	return b, nil
}

// Bots 按登记顺序返回全部 Bot，供 bot.list 使用。
func (s *Store) Bots(ctx context.Context) ([]Bot, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT bot_id, platform, label, credentials, username, display_name,
       caps, limits, constraints, status, status_reason, status_since,
       created_at, updated_at
FROM bots ORDER BY created_at, bot_id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出 bot: %w", err)
	}
	defer rows.Close()

	var out []Bot
	for rows.Next() {
		b, err := scanBot(rows)
		if err != nil {
			return nil, fmt.Errorf("store: 读取 bot 行: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 遍历 bot: %w", err)
	}
	return out, nil
}

// DeleteBot 删除 Bot 及其会话、消息、媒体记录（外键级联）。
// 磁盘上的媒体文件由调用方负责清理。
func (s *Store) DeleteBot(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM bots WHERE bot_id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: 删除 bot %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: 删除 bot %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: bot %s", ErrNotFound, id)
	}
	return nil
}

// SetBotStatus 更新 Bot 的运行状态快照。
func (s *Store) SetBotStatus(ctx context.Context, id string, st platform.Status) error {
	switch st.State {
	case platform.StateStarting, platform.StateRunning, platform.StateStopped, platform.StateError:
	default:
		return fmt.Errorf("%w: 未知状态 %q", ErrInvalid, st.State)
	}
	since := st.Since
	if since.IsZero() {
		since = s.now()
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE bots SET status = ?, status_reason = ?, status_since = ?, updated_at = ?
WHERE bot_id = ?`, st.State, st.Reason, since.UnixMilli(), s.nowMillis(), id)
	if err != nil {
		return fmt.Errorf("store: 更新 bot %s 状态: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: 更新 bot %s 状态: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: bot %s", ErrNotFound, id)
	}
	return nil
}

// rowScanner 兼容 *sql.Row 与 *sql.Rows。
type rowScanner interface {
	Scan(dest ...any) error
}

func scanBot(r rowScanner) (Bot, error) {
	var (
		b          Bot
		caps       string
		limits     string
		constraints string
		statusSince int64
		createdAt  int64
		updatedAt  int64
	)
	if err := r.Scan(
		&b.ID, &b.Platform, &b.Label, &b.Credentials, &b.Username, &b.DisplayName,
		&caps, &limits, &constraints, &b.Status.State, &b.Status.Reason, &statusSince,
		&createdAt, &updatedAt,
	); err != nil {
		return Bot{}, err
	}
	if err := json.Unmarshal([]byte(caps), &b.Caps); err != nil {
		return Bot{}, fmt.Errorf("解析 caps: %w", err)
	}
	if err := json.Unmarshal([]byte(limits), &b.Limits); err != nil {
		return Bot{}, fmt.Errorf("解析 limits: %w", err)
	}
	if err := json.Unmarshal([]byte(constraints), &b.Constraints); err != nil {
		return Bot{}, fmt.Errorf("解析 constraints: %w", err)
	}
	if statusSince > 0 {
		b.Status.Since = time.UnixMilli(statusSince)
	}
	b.CreatedAt = time.UnixMilli(createdAt)
	b.UpdatedAt = time.UnixMilli(updatedAt)
	return b, nil
}

func nonNilSlice(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func nonNilMap(v map[string]any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

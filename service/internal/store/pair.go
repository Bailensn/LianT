package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoPendingPair 表示没有待处理的配对码。
var ErrNoPendingPair = errors.New("store: 没有待处理的配对码")

// SavePendingPair 写入当前唯一一条配对码，覆盖旧值。
func (s *Store) SavePendingPair(ctx context.Context, code, nonce string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO pair_pending (id, code, nonce, expires) VALUES (1, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET code = excluded.code, nonce = excluded.nonce, expires = excluded.expires`,
		code, nonce, expires.UnixMilli())
	if err != nil {
		return fmt.Errorf("store: 保存配对码: %w", err)
	}
	return nil
}

// LoadPendingPair 读取尚未过期的配对码。
func (s *Store) LoadPendingPair(ctx context.Context) (code, nonce string, expires time.Time, err error) {
	var ms int64
	err = s.db.QueryRowContext(ctx, `SELECT code, nonce, expires FROM pair_pending WHERE id = 1`).
		Scan(&code, &nonce, &ms)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", time.Time{}, ErrNoPendingPair
	}
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("store: 读取配对码: %w", err)
	}
	return code, nonce, time.UnixMilli(ms), nil
}

// ClearPendingPair 消费掉配对码，保证一次性。
func (s *Store) ClearPendingPair(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pair_pending WHERE id = 1`); err != nil {
		return fmt.Errorf("store: 清除配对码: %w", err)
	}
	return nil
}

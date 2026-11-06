package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Device 是一台已授权的 Manager。
//
// PublicKey 是 Manager 的 P-256 公钥（PKIX DER），私钥永不出设备；
// 撤销授权就是删除这条记录，之后同公钥的认证一律 unauthorized。
type Device struct {
	ID         string
	Name       string
	PublicKey  []byte
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// PutDevice 登记或更新一台设备。
func (s *Store) PutDevice(ctx context.Context, d Device) error {
	if d.ID == "" || len(d.ID) > 64 {
		return fmt.Errorf("%w: 非法 device_id %q", ErrInvalid, d.ID)
	}
	if len(d.PublicKey) == 0 {
		return fmt.Errorf("%w: 设备公钥为空", ErrInvalid)
	}
	now := s.nowMillis()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO devices (device_id, name, public_key, created_at, last_seen_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(device_id) DO UPDATE SET
	name       = excluded.name,
	public_key = excluded.public_key`,
		d.ID, d.Name, d.PublicKey, now, now)
	if err != nil {
		return fmt.Errorf("store: 登记设备 %s: %w", d.ID, err)
	}
	return nil
}

// Device 按 device_id 取一台设备；不存在时返回 ErrNotFound。
func (s *Store) Device(ctx context.Context, id string) (Device, error) {
	var (
		d          Device
		createdAt  int64
		lastSeenAt int64
	)
	err := s.db.QueryRowContext(ctx, `
SELECT device_id, name, public_key, created_at, last_seen_at
FROM devices WHERE device_id = ?`, id).
		Scan(&d.ID, &d.Name, &d.PublicKey, &createdAt, &lastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, fmt.Errorf("%w: 设备 %s", ErrNotFound, id)
	}
	if err != nil {
		return Device{}, fmt.Errorf("store: 读取设备 %s: %w", id, err)
	}
	d.CreatedAt = time.UnixMilli(createdAt)
	if lastSeenAt > 0 {
		d.LastSeenAt = time.UnixMilli(lastSeenAt)
	}
	return d, nil
}

// Devices 按登记顺序返回全部设备。
func (s *Store) Devices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT device_id, name, public_key, created_at, last_seen_at
FROM devices ORDER BY created_at, device_id`)
	if err != nil {
		return nil, fmt.Errorf("store: 列出设备: %w", err)
	}
	defer rows.Close()

	var out []Device
	for rows.Next() {
		var (
			d          Device
			createdAt  int64
			lastSeenAt int64
		)
		if err := rows.Scan(&d.ID, &d.Name, &d.PublicKey, &createdAt, &lastSeenAt); err != nil {
			return nil, fmt.Errorf("store: 读取设备行: %w", err)
		}
		d.CreatedAt = time.UnixMilli(createdAt)
		if lastSeenAt > 0 {
			d.LastSeenAt = time.UnixMilli(lastSeenAt)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: 遍历设备: %w", err)
	}
	return out, nil
}

// DeleteDevice 撤销一台设备的授权。
func (s *Store) DeleteDevice(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM devices WHERE device_id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: 删除设备 %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: 删除设备 %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: 设备 %s", ErrNotFound, id)
	}
	return nil
}

// TouchDevice 记录一次成功认证的时间。
func (s *Store) TouchDevice(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE devices SET last_seen_at = ? WHERE device_id = ?`, s.nowMillis(), id)
	if err != nil {
		return fmt.Errorf("store: 更新设备 %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: 更新设备 %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: 设备 %s", ErrNotFound, id)
	}
	return nil
}

// CountDevices 返回已授权设备数，用于配对时判断 8 台上限。
func (s *Store) CountDevices(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: 统计设备: %w", err)
	}
	return n, nil
}

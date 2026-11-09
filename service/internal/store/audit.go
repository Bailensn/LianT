package store

import (
	"context"
	"fmt"
)

// Audit 是一条安全事件：认证、配对成功与失败都记。
type Audit struct {
	ID       int64  `json:"id,omitempty"`
	TS       int64  `json:"ts"`
	Kind     string `json:"kind"`
	DeviceID string `json:"device_id,omitempty"`
	Name     string `json:"name,omitempty"`
	IP       string `json:"ip,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// AddAudit 追加一条事件；失败不应该影响主流程，调用方负责只记日志。
func (s *Store) AddAudit(ctx context.Context, a Audit) error {
	if a.TS == 0 {
		a.TS = s.nowMillis()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO audit (ts, kind, device_id, name, ip, reason) VALUES (?, ?, ?, ?, ?, ?)`,
		a.TS, a.Kind, a.DeviceID, a.Name, a.IP, a.Reason)
	if err != nil {
		return fmt.Errorf("store: 记录审计事件: %w", err)
	}
	return nil
}

// RecentAudit 返回最近的 limit 条事件，新的在前。
func (s *Store) RecentAudit(ctx context.Context, limit int) ([]Audit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, ts, kind, device_id, name, ip, reason FROM audit ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: 读取审计事件: %w", err)
	}
	defer rows.Close()

	var out []Audit
	for rows.Next() {
		var a Audit
		if err := rows.Scan(&a.ID, &a.TS, &a.Kind, &a.DeviceID, &a.Name, &a.IP, &a.Reason); err != nil {
			return nil, fmt.Errorf("store: 扫描审计事件: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PruneAudit 只保留最近 keep 条，避免表无限增长。
func (s *Store) PruneAudit(ctx context.Context, keep int) error {
	if keep <= 0 {
		keep = 2000
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM audit WHERE id NOT IN (SELECT id FROM audit ORDER BY id DESC LIMIT ?)`, keep)
	if err != nil {
		return fmt.Errorf("store: 清理审计事件: %w", err)
	}
	return nil
}


// RecentAuditBefore 按 (ts,id) 倒序游标分页，新的在前。
func (s *Store) RecentAuditBefore(ctx context.Context, limit int, beforeTS, beforeID int64) ([]Audit, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, ts, kind, device_id, name, ip, reason FROM audit
WHERE ts < ? OR (ts = ? AND id < ?)
ORDER BY ts DESC, id DESC LIMIT ?`, beforeTS, beforeTS, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: 读取审计事件: %w", err)
	}
	defer rows.Close()
	out := make([]Audit, 0, limit)
	for rows.Next() {
		var a Audit
		if err := rows.Scan(&a.ID, &a.TS, &a.Kind, &a.DeviceID, &a.Name, &a.IP, &a.Reason); err != nil {
			return nil, fmt.Errorf("store: 扫描审计事件: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

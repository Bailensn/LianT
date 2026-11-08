package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func init() {
	sql.Register("postgresq", rebindDriver{base: &pq.Driver{}})
}

// rebindDriver 让现有 ? 占位 SQL 能直接在 PostgreSQL 上跑。
type rebindDriver struct{ base driver.Driver }

func (d rebindDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return rebindConn{Conn: c}, nil
}

type rebindConn struct{ driver.Conn }

func (c rebindConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(rebind(query))
}

func (c rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.Conn.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func (c rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.Conn.(driver.QueryerContext); ok {
		return q.QueryContext(ctx, rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func rebind(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 1
	for _, r := range query {
		if r == '?' {
			fmt.Fprintf(&b, "$%d", n)
			n++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) migratePostgres(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: 开始 PostgreSQL 迁移: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_meta (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		version BIGINT NOT NULL
	)`); err != nil {
		return fmt.Errorf("store: 创建 schema_meta: %w", err)
	}
	var version int
	err = tx.QueryRowContext(ctx, `SELECT version FROM schema_meta WHERE id = 1`).Scan(&version)
	if err == sql.ErrNoRows {
		version = 0
	} else if err != nil {
		return fmt.Errorf("store: 读取 PostgreSQL schema 版本: %w", err)
	}
	if version < 3 {
		for _, stmt := range pgSchema {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("store: PostgreSQL 建表: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta (id, version) VALUES (1, $1)
			ON CONFLICT (id) DO UPDATE SET version = EXCLUDED.version`, 3); err != nil {
			return fmt.Errorf("store: 写入 PostgreSQL schema 版本: %w", err)
		}
	}
	return tx.Commit()
}

var pgSchema = []string{
	`CREATE TABLE IF NOT EXISTS bots (
		bot_id        TEXT PRIMARY KEY,
		platform      TEXT NOT NULL,
		label         TEXT NOT NULL DEFAULT '',
		credentials   BYTEA NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		display_name  TEXT NOT NULL DEFAULT '',
		caps          TEXT NOT NULL DEFAULT '[]',
		limits        TEXT NOT NULL DEFAULT '{}',
		constraints   TEXT NOT NULL DEFAULT '{}',
		status        TEXT NOT NULL DEFAULT 'stopped',
		status_reason TEXT NOT NULL DEFAULT '',
		status_since  BIGINT NOT NULL DEFAULT 0,
		created_at    BIGINT NOT NULL,
		updated_at    BIGINT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS devices (
		device_id    TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		public_key   BYTEA NOT NULL,
		created_at   BIGINT NOT NULL,
		last_seen_at BIGINT NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE IF NOT EXISTS peers (
		bot_id       TEXT NOT NULL,
		peer_id      TEXT NOT NULL,
		kind         TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		last_ts      BIGINT NOT NULL DEFAULT 0,
		last_msg_id  TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (bot_id, peer_id),
		FOREIGN KEY (bot_id) REFERENCES bots(bot_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS messages (
		bot_id      TEXT NOT NULL,
		msg_id      TEXT NOT NULL,
		peer_id     TEXT NOT NULL,
		peer_kind   TEXT NOT NULL,
		direction   TEXT NOT NULL,
		sender_id   TEXT NOT NULL DEFAULT '',
		sender_name TEXT NOT NULL DEFAULT '',
		sender_role TEXT NOT NULL DEFAULT '',
		msg_type    TEXT NOT NULL DEFAULT 'text',
		ts          BIGINT NOT NULL,
		body_plain  TEXT NOT NULL,
		body_doc    BYTEA,
		in_reply_to TEXT NOT NULL DEFAULT '',
		operator_id TEXT NOT NULL DEFAULT '',
		created_at  BIGINT NOT NULL,
		PRIMARY KEY (bot_id, msg_id),
		FOREIGN KEY (bot_id) REFERENCES bots(bot_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS messages_peer_ts ON messages (bot_id, peer_id, ts, msg_id)`,
	`CREATE INDEX IF NOT EXISTS messages_ts ON messages (ts)`,
	`CREATE TABLE IF NOT EXISTS media (
		bot_id TEXT NOT NULL,
		msg_id TEXT NOT NULL,
		idx    INTEGER NOT NULL,
		mime   TEXT NOT NULL DEFAULT '',
		path   TEXT NOT NULL DEFAULT '',
		size   BIGINT NOT NULL DEFAULT 0,
		state  TEXT NOT NULL DEFAULT 'pending',
		PRIMARY KEY (bot_id, msg_id, idx),
		FOREIGN KEY (bot_id, msg_id) REFERENCES messages(bot_id, msg_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS audit (
		id        BIGSERIAL PRIMARY KEY,
		ts        BIGINT NOT NULL,
		kind      TEXT NOT NULL,
		device_id TEXT NOT NULL DEFAULT '',
		name      TEXT NOT NULL DEFAULT '',
		ip        TEXT NOT NULL DEFAULT '',
		reason    TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS audit_ts ON audit (ts)`,
	`CREATE TABLE IF NOT EXISTS pair_pending (
		id      INTEGER PRIMARY KEY CHECK (id = 1),
		code    TEXT NOT NULL,
		nonce   TEXT NOT NULL,
		expires BIGINT NOT NULL
	)`,
}

// WipePostgres 删除 LianT 自己的表，不 DROP 整个 database。
func WipePostgres(dsn string) error {
	s, err := openPostgres(dsn)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.db.Exec(`DROP TABLE IF EXISTS
		media, messages, peers, bots, devices, audit, pair_pending, schema_meta
		CASCADE`)
	return err
}

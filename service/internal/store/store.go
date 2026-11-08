// Package store 是 LianT 的持久化层：Bot 元数据、会话与消息都在这里落盘。
//
// 设计约束（docs/DESIGN.md D2/D7、docs/PROTOCOL.md §2/§8）：
//   - bot_id、peer_id 都是平台命名空间字符串，不是整数；
//   - 会话键是 (bot_id, peer_id)，peer_id 只在单个 Bot 内唯一；
//   - 消息存 body 全量（plain + 可选 doc），不只存降级文本；
//   - 凭据只以调用方加密后的不透明字节落库，store 不解释、不解密。
//
// 保留策略不放在写入路径上，由 Purge 按时间清理（见 retention.go）；
// 写入永远成功，读不到的旧消息就是过期了，不假装还在。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// schemaVersion 记录在 PRAGMA user_version 里。改表结构时 +1 并补迁移。
const schemaVersion = 3

var (
	// ErrNotFound 表示按主键没有查到记录。
	ErrNotFound = errors.New("store: 记录不存在")
	// ErrInvalid 表示调用方传入了不合法的参数。
	ErrInvalid = errors.New("store: 参数不合法")
)

// Store 是持久化句柄，可被多个 goroutine 并发使用。
// 支持 SQLite（默认）与 PostgreSQL，SQL 用 ? 占位，PG 驱动层负责改成 $N。
type Store struct {
	db      *sql.DB
	dialect string
	// now 可在测试里替换，避免依赖真实时钟。
	now func() time.Time
}

// Open 打开（必要时创建）SQLite 数据库并完成迁移。
func Open(path string) (*Store, error) {
	return OpenDatabase("sqlite", "", path)
}

// OpenDatabase 按配置打开数据库。driver 为空等同 sqlite。
func OpenDatabase(driver, dsn, path string) (*Store, error) {
	if driver == "" || driver == "sqlite" {
		return openSQLite(path)
	}
	if driver == "postgres" {
		return openPostgres(dsn)
	}
	return nil, fmt.Errorf("%w: 不支持的数据库驱动 %q", ErrInvalid, driver)
}

func openSQLite(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: 数据库路径为空", ErrInvalid)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("store: 创建数据目录: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: 打开数据库: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, dialect: "sqlite", now: time.Now}
	if err := s.configure(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func openPostgres(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("%w: PostgreSQL DSN 为空", ErrInvalid)
	}
	db, err := sql.Open("postgresq", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: 打开 PostgreSQL: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: 连接 PostgreSQL 失败: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	s := &Store{db: db, dialect: "postgres", now: time.Now}
	if err := s.migratePostgres(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB 暴露底层句柄，仅给同一进程内的迁移/诊断工具用。
// 业务代码不要绕过本包直接写表。
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) nowMillis() int64 { return s.now().UnixMilli() }

func (s *Store) configure() error {
	if s.dialect != "sqlite" {
		return nil
	}
	// journal_mode 会返回一行结果，用 Query 读掉再关，避免连接被占住。
	row, err := s.db.Query(`PRAGMA journal_mode = WAL`)
	if err != nil {
		return fmt.Errorf("store: 设置 WAL: %w", err)
	}
	row.Close()

	for _, p := range []string{
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := s.db.Exec(p); err != nil {
			return fmt.Errorf("store: 设置 %s: %w", p, err)
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	if s.dialect == "postgres" {
		return s.migratePostgres(ctx)
	}
	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("store: 读取 schema 版本: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("store: schema 版本 %d 高于本程序支持的 %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	steps := []struct {
		version int
		stmts   []string
	}{
		{1, schemaV1},
		{2, schemaV2},
		{3, schemaV3},
	}
	for _, step := range steps {
		if version >= step.version {
			continue
		}
		for _, stmt := range step.stmts {
			if _, err := s.db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("store: 迁移到 v%d: %w", step.version, err)
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return fmt.Errorf("store: 写入 schema 版本: %w", err)
	}
	return nil
}

// schemaV2 增加已授权的 Manager 设备表（配对后落库，撤销即删除）。
var schemaV2 = []string{
	`CREATE TABLE IF NOT EXISTS devices (
		device_id    TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		public_key   BLOB NOT NULL,
		created_at   INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL DEFAULT 0
	)`,
}

// schemaV3 给消息加发送者角色（owner/admin/member），用于群里给群主/管理员标色。
var schemaV3 = []string{
	`ALTER TABLE messages ADD COLUMN sender_role TEXT NOT NULL DEFAULT ''`,
}

// schemaV1 是 v1 的全部建表语句。
//
// 时间列除明确说明外都是 Unix 毫秒（与协议 ts 一致），便于排序与游标。
var schemaV1 = []string{
	// Bot 元数据。credentials 是加密后的不透明字节。
	`CREATE TABLE IF NOT EXISTS bots (
		bot_id       TEXT PRIMARY KEY,
		platform     TEXT NOT NULL,
		label        TEXT NOT NULL DEFAULT '',
		credentials  BLOB NOT NULL,
		username     TEXT NOT NULL DEFAULT '',
		display_name TEXT NOT NULL DEFAULT '',
		caps         TEXT NOT NULL DEFAULT '[]',
		limits       TEXT NOT NULL DEFAULT '{}',
		constraints  TEXT NOT NULL DEFAULT '{}',
		status       TEXT NOT NULL DEFAULT 'stopped',
		status_reason TEXT NOT NULL DEFAULT '',
		status_since INTEGER NOT NULL DEFAULT 0,
		created_at   INTEGER NOT NULL,
		updated_at   INTEGER NOT NULL
	)`,
	// 会话表：只为收件箱与最近消息服务，消息内容仍以 messages 为准。
	`CREATE TABLE IF NOT EXISTS peers (
		bot_id       TEXT NOT NULL,
		peer_id      TEXT NOT NULL,
		kind         TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		last_ts      INTEGER NOT NULL DEFAULT 0,
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
		msg_type    TEXT NOT NULL DEFAULT 'text',
		ts          INTEGER NOT NULL,
		body_plain  TEXT NOT NULL,
		body_doc    BLOB,
		in_reply_to TEXT NOT NULL DEFAULT '',
		operator_id TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL,
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
		size   INTEGER NOT NULL DEFAULT 0,
		state  TEXT NOT NULL DEFAULT 'pending',
		PRIMARY KEY (bot_id, msg_id, idx),
		FOREIGN KEY (bot_id, msg_id) REFERENCES messages(bot_id, msg_id) ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS audit (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		ts        INTEGER NOT NULL,
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
		expires INTEGER NOT NULL
	)`,
}

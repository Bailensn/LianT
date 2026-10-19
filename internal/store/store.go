package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// 存储层：SQLite
//
// 全业务数据都在这里：群、管理员、回复规则、违禁词、禁言、黑名单、群名册、审计。
//   * 驱动见 sqlitedriver.go（默认纯 Go 的 modernc.org/sqlite，不需要 cgo）
//   * 单连接串行 + WAL：手机上的读写量很小，这样最不容易出 "database is locked"
//   * rev 是进程内版本号，任何写操作 +1；热路径靠它判断要不要重载缓存，
//     所以每收到一条消息也不会反复查库
//   * 建表用 PRAGMA user_version 记版本，后续加字段就在这里追加迁移
// ============================================================================

// SchemaVersion 每次改表结构都 +1，并在 migrations 里追加一段。
const SchemaVersion = 3

// SchemaStatements 是当前版本的全量建表语句（幂等，可重复执行）。
var SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS groups (
		chat_id            TEXT PRIMARY KEY,
		name                    TEXT NOT NULL DEFAULT '',
		enabled                 INTEGER NOT NULL DEFAULT 1,
		reply_all               INTEGER NOT NULL DEFAULT 1,
		commands_enabled        INTEGER NOT NULL DEFAULT 1,
		group_admin_can_command INTEGER NOT NULL DEFAULT 1,
		bot_is_admin            INTEGER NOT NULL DEFAULT 0,
		note                    TEXT NOT NULL DEFAULT '',
		joined_at               TEXT NOT NULL DEFAULT '',
		updated_at              TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS admins (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT 'admin',
		chat_id  TEXT NOT NULL DEFAULT '',
		enabled       INTEGER NOT NULL DEFAULT 1,
		created_at    TEXT NOT NULL DEFAULT '',
		UNIQUE(user_id, chat_id)
	)`,
	`CREATE TABLE IF NOT EXISTS reply_rules (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		priority   INTEGER NOT NULL DEFAULT 100,
		match      TEXT NOT NULL DEFAULT 'exact',
		pattern    TEXT NOT NULL DEFAULT '',
		reply      TEXT NOT NULL DEFAULT '',
		fold       INTEGER NOT NULL DEFAULT 0,
		always     INTEGER NOT NULL DEFAULT 1,
		scope      TEXT NOT NULL DEFAULT '',
		enabled    INTEGER NOT NULL DEFAULT 1,
		note       TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS banned_words (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		word         TEXT NOT NULL,
		match        TEXT NOT NULL DEFAULT 'contains',
		fold         INTEGER NOT NULL DEFAULT 0,
		scope        TEXT NOT NULL DEFAULT '',
		reply        TEXT NOT NULL DEFAULT '',
		recall       INTEGER NOT NULL DEFAULT 1,
		mute_seconds INTEGER NOT NULL DEFAULT 0,
		kick         INTEGER NOT NULL DEFAULT 0,
		blacklist    INTEGER NOT NULL DEFAULT 0,
		severity     INTEGER NOT NULL DEFAULT 100,
		enabled      INTEGER NOT NULL DEFAULT 1,
		note         TEXT NOT NULL DEFAULT '',
		created_at   TEXT NOT NULL DEFAULT '',
		updated_at   TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS mutes (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id  TEXT NOT NULL,
		user_id TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		expire_at     TEXT NOT NULL DEFAULT '',
		reason        TEXT NOT NULL DEFAULT '',
		operator      TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'active',
		created_at    TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS blacklist (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id  TEXT NOT NULL,
		user_id TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		reason        TEXT NOT NULL DEFAULT '',
		operator      TEXT NOT NULL DEFAULT '',
		tg_synced     INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL DEFAULT '',
		UNIQUE(chat_id, user_id)
	)`,
	`CREATE TABLE IF NOT EXISTS members (
		chat_id  TEXT NOT NULL,
		user_id TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT '',
		Bot INTEGER NOT NULL DEFAULT 0,
		msgs          INTEGER NOT NULL DEFAULT 0,
		first_seen    TEXT NOT NULL DEFAULT '',
		last_seen     TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (chat_id, user_id)
	)`,
	`CREATE TABLE IF NOT EXISTS bots (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		name           TEXT NOT NULL DEFAULT '',
		token          TEXT NOT NULL,
		username       TEXT NOT NULL DEFAULT '',
		telegram_id    INTEGER NOT NULL DEFAULT 0,
		platform       TEXT NOT NULL DEFAULT 'telegram',
		platform_id    TEXT NOT NULL DEFAULT '',
		mode           TEXT NOT NULL DEFAULT 'polling',
		webhook_url    TEXT NOT NULL DEFAULT '',
		webhook_secret TEXT NOT NULL DEFAULT '',
		webhook_listen TEXT NOT NULL DEFAULT '',
		webhook_path   TEXT NOT NULL DEFAULT '',
		enabled        INTEGER NOT NULL DEFAULT 1,
		created_at     TEXT NOT NULL DEFAULT '',
		updated_at     TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_bots_token ON bots(token)`,
	`CREATE TABLE IF NOT EXISTS join_requests (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id    TEXT NOT NULL,
		user_id    TEXT NOT NULL,
		username   TEXT NOT NULL DEFAULT '',
		bio        TEXT NOT NULL DEFAULT '',
		status     TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE(chat_id, user_id)
	)`,
	`CREATE TABLE IF NOT EXISTS audit (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		ts            TEXT NOT NULL DEFAULT '',
		kind          TEXT NOT NULL DEFAULT '',
		chat_id  TEXT NOT NULL DEFAULT '',
		user_id TEXT NOT NULL DEFAULT '',
		username      TEXT NOT NULL DEFAULT '',
		content       TEXT NOT NULL DEFAULT '',
		hit           TEXT NOT NULL DEFAULT '',
		actions       TEXT NOT NULL DEFAULT '',
		result        TEXT NOT NULL DEFAULT '',
		detail        TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit(ts DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_rules_enabled ON reply_rules(enabled, priority)`,
	`CREATE INDEX IF NOT EXISTS idx_words_enabled ON banned_words(enabled)`,
	`CREATE INDEX IF NOT EXISTS idx_mutes_active ON mutes(status, expire_at)`,
	`CREATE INDEX IF NOT EXISTS idx_blacklist_group ON blacklist(chat_id)`,
	`CREATE INDEX IF NOT EXISTS idx_join_status ON join_requests(status, updated_at DESC)`,
}

// ---------------- 记录类型 ----------------

type GroupRec struct {
	ChatID           string `json:"chat_id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	ReplyAll         bool   `json:"reply_all"`
	CommandsEnabled  bool   `json:"commands_enabled"`
	GroupAdminCanCmd bool   `json:"group_admin_can_command"`
	BotIsAdmin       bool   `json:"bot_is_admin"`
	Note             string `json:"note"`
	JoinedAt         string `json:"joined_at"`
	UpdatedAt        string `json:"updated_at"`
}

type AdminRec struct {
	ID        int64  `json:"id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`    // super | admin
	ChatID    string `json:"chat_id"` // 空 = 全局管理员
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type RuleRec struct {
	ID        int64  `json:"id"`
	Priority  int    `json:"priority"`
	Match     string `json:"match"`
	Pattern   string `json:"pattern"`
	Reply     string `json:"reply"`
	Fold      bool   `json:"fold"`
	Always    bool   `json:"always"`
	Scope     string `json:"scope"`
	Enabled   bool   `json:"enabled"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type WordRec struct {
	ID          int64  `json:"id"`
	Word        string `json:"word"`
	Match       string `json:"match"`
	Fold        bool   `json:"fold"`
	Scope       string `json:"scope"`
	Reply       string `json:"reply"`
	Recall      bool   `json:"recall"`
	MuteSeconds int    `json:"mute_seconds"`
	Kick        bool   `json:"kick"`
	Blacklist   bool   `json:"blacklist"`
	Severity    int    `json:"severity"`
	Enabled     bool   `json:"enabled"`
	Note        string `json:"note"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type MuteRec struct {
	ID        int64  `json:"id"`
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	ExpireAt  string `json:"expire_at"`
	Reason    string `json:"reason"`
	Operator  string `json:"operator"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type BlackRec struct {
	ID        int64  `json:"id"`
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Reason    string `json:"reason"`
	Operator  string `json:"operator"`
	TGSynced  bool   `json:"tg_synced"`
	CreatedAt string `json:"created_at"`
}

type MemberRec struct {
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Bot       bool   `json:"bot"`
	Msgs      int64  `json:"msgs"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

type BotRec struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Token         string `json:"-"`
	Username      string `json:"username"`
	TelegramID    int64  `json:"telegram_id"`
	Platform      string `json:"platform"`
	PlatformID    string `json:"platform_id"`
	Mode          string `json:"mode"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"`
	WebhookListen string `json:"webhook_listen"`
	WebhookPath   string `json:"webhook_path"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type JoinRec struct {
	ID        int64  `json:"id"`
	ChatID    string `json:"chat_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Bio       string `json:"bio"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AuditRec struct {
	ID       int64  `json:"id"`
	Ts       string `json:"ts"`
	Kind     string `json:"kind"`
	ChatID   string `json:"chat_id"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Content  string `json:"content"`
	Hit      string `json:"hit"`
	Actions  string `json:"actions"`
	Result   string `json:"result"`
	Detail   string `json:"detail"`
}

// ---------------- 打开 / 迁移 ----------------

type Store struct {
	db   *sql.DB
	path string
	rev  int64
	mu   sync.Mutex // 写操作串行（SQLite 单写者）
}

func NowStr() string { return time.Now().Format(time.RFC3339) }

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("建数据目录失败 %s: %w", dir, err)
		}
	}
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("数据库路径为空（路径初始化失败，先跑 LianT path 看看）")
	}
	db, err := sql.Open(SqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败 %s: %w", path, err)
	}
	// SQLite 在手机上并发写容易锁：单连接最稳，读也走这一条。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=10000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("设置 %s 失败: %w", p, err)
		}
	}

	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Revision() int64 { return atomic.LoadInt64(&s.rev) }

func (s *Store) bump() { atomic.AddInt64(&s.rev, 1) }

// migrate 按 PRAGMA user_version 做版本迁移。
// Quiet 为 true 时不打"SQLite 就绪"这类日志（CLI 里用）。
var Quiet bool

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("读 user_version 失败: %w", err)
	}
	if v >= SchemaVersion {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range SchemaStatements {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("建表失败: %w\nSQL: %s", err, stmt)
		}
	}
	// 后续版本在这里追加：if v < 2 { ... }
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if !Quiet {
		log.Printf("[OK] SQLite 就绪: %s (schema v%d)", s.path, SchemaVersion)
	}
	s.bump()
	return nil
}

// ---------------- settings ----------------

func (s *Store) Setting(key, def string) string {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, NowStr())
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Settings() map[string]string {
	out := map[string]string{}
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	return out
}

// ---------------- groups ----------------

const GroupCols = `chat_id, name, enabled, reply_all, commands_enabled,
	group_admin_can_command, bot_is_admin, note, joined_at, updated_at`

func ScanGroup(sc interface{ Scan(...interface{}) error }) (GroupRec, error) {
	var g GroupRec
	var enabled, replyAll, cmds, adminsCan, botAdmin int
	err := sc.Scan(&g.ChatID, &g.Name, &enabled, &replyAll, &cmds, &adminsCan, &botAdmin,
		&g.Note, &g.JoinedAt, &g.UpdatedAt)
	g.Enabled = enabled != 0
	g.ReplyAll = replyAll != 0
	g.CommandsEnabled = cmds != 0
	g.GroupAdminCanCmd = adminsCan != 0
	g.BotIsAdmin = botAdmin != 0
	return g, err
}

func (s *Store) Groups() ([]GroupRec, error) {
	rows, err := s.db.Query(`SELECT ` + GroupCols + ` FROM groups ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupRec
	for rows.Next() {
		g, err := ScanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) Group(chatID string) (GroupRec, bool) {
	row := s.db.QueryRow(`SELECT `+GroupCols+` FROM groups WHERE chat_id = ?`, chatID)
	g, err := ScanGroup(row)
	if err != nil {
		return GroupRec{}, false
	}
	return g, true
}

// EnsureGroup 机器人入群/首次见到消息时登记群（默认开启，且不清空已有设置）。
func (s *Store) EnsureGroup(chatID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	_, err := s.db.Exec(`INSERT INTO groups(chat_id, name, joined_at, updated_at)
		VALUES(?,?,?,?)
		ON CONFLICT(chat_id) DO UPDATE SET
			name = CASE WHEN excluded.Name <> '' THEN excluded.Name ELSE groups.Name END,
			updated_at = excluded.updated_at`,
		chatID, name, now, now)
	if err == nil {
		s.bump()
	}
	return err
}

// UpdateGroup 只更新显式给出的字段（用指针区分"没传"和"设为空/false"）。
func (s *Store) UpdateGroup(chatID string, name *string, enabled, replyAll, cmds, groupAdminsCanCmd, botIsAdmin *bool, note *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sets := []string{"updated_at = ?"}
	args := []interface{}{NowStr()}
	add := func(expr string, val interface{}) {
		sets = append(sets, expr)
		args = append(args, val)
	}
	if name != nil {
		add("name = ?", *name)
	}
	if enabled != nil {
		add("enabled = ?", BoolInt(*enabled))
	}
	if replyAll != nil {
		add("reply_all = ?", BoolInt(*replyAll))
	}
	if cmds != nil {
		add("commands_enabled = ?", BoolInt(*cmds))
	}
	if groupAdminsCanCmd != nil {
		add("group_admin_can_command = ?", BoolInt(*groupAdminsCanCmd))
	}
	if botIsAdmin != nil {
		add("bot_is_admin = ?", BoolInt(*botIsAdmin))
	}
	if note != nil {
		add("note = ?", *note)
	}
	args = append(args, chatID)
	if _, err := s.db.Exec(`UPDATE groups SET `+strings.Join(sets, ", ")+` WHERE chat_id = ?`, args...); err != nil {
		return err
	}
	s.bump()
	return nil
}

// DeleteGroup 删除群记录（会连带清掉该群的禁言/黑名单记录）。
func (s *Store) DeleteGroup(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range []string{
		`DELETE FROM groups WHERE chat_id = ?`,
		`DELETE FROM mutes WHERE chat_id = ?`,
		`DELETE FROM blacklist WHERE chat_id = ?`,
		`DELETE FROM members WHERE chat_id = ?`,
	} {
		if _, err := s.db.Exec(q, chatID); err != nil {
			return err
		}
	}
	s.bump()
	return nil
}

// ---------------- admins ----------------

func (s *Store) Admins() ([]AdminRec, error) {
	rows, err := s.db.Query(`SELECT id, user_id, username, role, chat_id, enabled, created_at
		FROM admins ORDER BY role, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminRec
	for rows.Next() {
		var a AdminRec
		var en int
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.Role, &a.ChatID, &en, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Enabled = en != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) AddAdmin(a AdminRec) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.Role == "" {
		a.Role = "admin"
	}
	res, err := s.db.Exec(`INSERT INTO admins(user_id, username, role, chat_id, enabled, created_at)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(user_id, chat_id) DO UPDATE SET
			username=excluded.Username, role=excluded.Role, enabled=excluded.Enabled`,
		a.UserID, a.Username, a.Role, a.ChatID, BoolInt(a.Enabled || a.ID == 0), NowStr())
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

func (s *Store) DeleteAdmin(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM admins WHERE id = ?`, id); err != nil {
		return err
	}
	s.bump()
	return nil
}

// AdminRole 判断某人是不是管理员：返回角色（super/admin）和是否命中。
// scope 为群 chatID：全局管理员（chat_id=”)与群级管理员都算。
func (s *Store) AdminRole(groupChatID, memberChatID string) (string, bool) {
	if memberChatID == "" {
		return "", false
	}
	rows, err := s.db.Query(`SELECT role FROM admins
		WHERE user_id = ? AND enabled = 1 AND (chat_id = '' OR chat_id = ?)
		ORDER BY CASE role WHEN 'super' THEN 0 ELSE 1 END LIMIT 1`, memberChatID, groupChatID)
	if err != nil {
		return "", false
	}
	defer rows.Close()
	if rows.Next() {
		var role string
		if rows.Scan(&role) == nil {
			return role, true
		}
	}
	return "", false
}

// ---------------- reply_rules ----------------

const RuleCols = `id, priority, match, pattern, reply, fold, always, scope, enabled, note, created_at, updated_at`

func (s *Store) Rules() ([]RuleRec, error) {
	rows, err := s.db.Query(`SELECT ` + RuleCols + ` FROM reply_rules ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuleRec
	for rows.Next() {
		r, err := ScanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func ScanRule(sc interface{ Scan(...interface{}) error }) (RuleRec, error) {
	var r RuleRec
	var fold, always, en int
	err := sc.Scan(&r.ID, &r.Priority, &r.Match, &r.Pattern, &r.Reply, &fold, &always, &r.Scope, &en, &r.Note, &r.CreatedAt, &r.UpdatedAt)
	r.Fold = fold != 0
	r.Always = always != 0
	r.Enabled = en != 0
	return r, err
}

func (s *Store) SaveRule(r RuleRec) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	if r.ID > 0 {
		_, err := s.db.Exec(`UPDATE reply_rules SET priority=?, match=?, pattern=?, reply=?, fold=?,
			always=?, scope=?, enabled=?, note=?, updated_at=? WHERE id=?`,
			r.Priority, r.Match, r.Pattern, r.Reply, BoolInt(r.Fold), BoolInt(r.Always),
			r.Scope, BoolInt(r.Enabled), r.Note, now, r.ID)
		if err != nil {
			return 0, err
		}
		s.bump()
		return r.ID, nil
	}
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	res, err := s.db.Exec(`INSERT INTO reply_rules(priority, match, pattern, reply, fold, always, scope, enabled, note, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		r.Priority, r.Match, r.Pattern, r.Reply, BoolInt(r.Fold), BoolInt(r.Always),
		r.Scope, BoolInt(r.Enabled), r.Note, r.CreatedAt, now)
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

func (s *Store) DeleteRule(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM reply_rules WHERE id = ?`, id); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- banned_words ----------------

const WordCols = `id, word, match, fold, scope, reply, recall, mute_seconds, kick, blacklist,
	severity, enabled, note, created_at, updated_at`

func (s *Store) Words() ([]WordRec, error) {
	rows, err := s.db.Query(`SELECT ` + WordCols + ` FROM banned_words ORDER BY severity, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WordRec
	for rows.Next() {
		w, err := ScanWord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func ScanWord(sc interface{ Scan(...interface{}) error }) (WordRec, error) {
	var w WordRec
	var fold, recall, kick, blacklist, en int
	err := sc.Scan(&w.ID, &w.Word, &w.Match, &fold, &w.Scope, &w.Reply, &recall, &w.MuteSeconds,
		&kick, &blacklist, &w.Severity, &en, &w.Note, &w.CreatedAt, &w.UpdatedAt)
	w.Fold = fold != 0
	w.Recall = recall != 0
	w.Kick = kick != 0
	w.Blacklist = blacklist != 0
	w.Enabled = en != 0
	return w, err
}

func (s *Store) SaveWord(w WordRec) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	if w.ID > 0 {
		_, err := s.db.Exec(`UPDATE banned_words SET word=?, match=?, fold=?, scope=?, reply=?,
			recall=?, mute_seconds=?, kick=?, blacklist=?, severity=?, enabled=?, note=?, updated_at=? WHERE id=?`,
			w.Word, w.Match, BoolInt(w.Fold), w.Scope, w.Reply, BoolInt(w.Recall), w.MuteSeconds,
			BoolInt(w.Kick), BoolInt(w.Blacklist), w.Severity, BoolInt(w.Enabled), w.Note, now, w.ID)
		if err != nil {
			return 0, err
		}
		s.bump()
		return w.ID, nil
	}
	res, err := s.db.Exec(`INSERT INTO banned_words(word, match, fold, scope, reply, recall, mute_seconds,
		kick, blacklist, severity, enabled, note, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.Word, w.Match, BoolInt(w.Fold), w.Scope, w.Reply, BoolInt(w.Recall), w.MuteSeconds,
		BoolInt(w.Kick), BoolInt(w.Blacklist), w.Severity, BoolInt(w.Enabled), w.Note, now, now)
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

func (s *Store) DeleteWord(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM banned_words WHERE id = ?`, id); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- mutes ----------------

func (s *Store) AddMute(m MuteRec) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.Status == "" {
		m.Status = "active"
	}
	res, err := s.db.Exec(`INSERT INTO mutes(chat_id, user_id, username, expire_at, reason, operator, status, created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		m.ChatID, m.UserID, m.Username, m.ExpireAt, m.Reason, m.Operator, m.Status, NowStr())
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

func (s *Store) Mutes(activeOnly bool) ([]MuteRec, error) {
	q := `SELECT id, chat_id, user_id, username, expire_at, reason, operator, status, created_at FROM mutes`
	if activeOnly {
		q += ` WHERE status = 'active' ORDER BY id DESC LIMIT 500`
	} else {
		q += ` ORDER BY id DESC LIMIT 500`
	}
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MuteRec
	for rows.Next() {
		var m MuteRec
		if err := rows.Scan(&m.ID, &m.ChatID, &m.UserID, &m.Username, &m.ExpireAt,
			&m.Reason, &m.Operator, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// EndMute 把某条禁言记录标记为已解除。
func (s *Store) EndMute(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE mutes SET status = 'ended' WHERE id = ?`, id); err != nil {
		return err
	}
	s.bump()
	return nil
}

// EndMuteByMember 按人解除（用于群内/网页解除禁言）。
func (s *Store) EndMuteByMember(groupChatID, memberChatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE mutes SET status = 'ended'
		WHERE chat_id = ? AND user_id = ? AND status = 'active'`, groupChatID, memberChatID); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- blacklist ----------------

func (s *Store) AddBlacklist(b BlackRec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO blacklist(chat_id, user_id, username, reason, operator, tg_synced, created_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(chat_id, user_id) DO UPDATE SET
			username=excluded.Username, reason=excluded.Reason, operator=excluded.Operator`,
		b.ChatID, b.UserID, b.Username, b.Reason, b.Operator, BoolInt(b.TGSynced), NowStr())
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Blacklists(groupChatID string) ([]BlackRec, error) {
	q := `SELECT id, chat_id, user_id, username, reason, operator, tg_synced, created_at FROM blacklist`
	args := []interface{}{}
	if groupChatID != "" {
		q += ` WHERE chat_id = ?`
		args = append(args, groupChatID)
	}
	q += ` ORDER BY id DESC LIMIT 500`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlackRec
	for rows.Next() {
		var b BlackRec
		var synced int
		if err := rows.Scan(&b.ID, &b.ChatID, &b.UserID, &b.Username, &b.Reason,
			&b.Operator, &synced, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.TGSynced = synced != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

// IsBlacklisted 命中本地黑名单（全局空群号的黑名单对所有群生效）。
func (s *Store) IsBlacklisted(groupChatID, memberChatID string) bool {
	if memberChatID == "" {
		return false
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM blacklist
		WHERE user_id = ? AND (chat_id = '' OR chat_id = ?)`, memberChatID, groupChatID).Scan(&n)
	return err == nil && n > 0
}

func (s *Store) RemoveBlacklist(groupChatID, memberChatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM blacklist WHERE chat_id = ? AND user_id = ?`,
		groupChatID, memberChatID); err != nil {
		return err
	}
	s.bump()
	return nil
}

func (s *Store) MarkBlacklistSynced(groupChatID, memberChatID string, synced bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE blacklist SET tg_synced = ? WHERE chat_id = ? AND user_id = ?`,
		BoolInt(synced), groupChatID, memberChatID); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- members（自建群名册） ----------------

// TouchMember 每条群消息都会调用：记下昵称/角色，供"禁言 张三"这类指令解析用。
func (s *Store) TouchMember(groupChatID, memberChatID, username, role string, isBot bool) error {
	if groupChatID == "" || memberChatID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	_, err := s.db.Exec(`INSERT INTO members(chat_id, user_id, username, role, bot, msgs, first_seen, last_seen)
		VALUES(?,?,?,?,?,1,?,?)
		ON CONFLICT(chat_id, user_id) DO UPDATE SET
			username = CASE WHEN excluded.Username <> '' THEN excluded.Username ELSE members.Username END,
			role     = CASE WHEN excluded.Role <> '' THEN excluded.Role ELSE members.Role END,
			msgs     = members.Msgs + 1,
			last_seen = excluded.last_seen`,
		groupChatID, memberChatID, username, role, BoolInt(isBot), now, now)
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Members(groupChatID string, limit int) ([]MemberRec, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	gid := strings.TrimSpace(groupChatID)
	var (
		rows *sql.Rows
		err  error
	)
	if gid == "" {
		// 空群号 = 跨群名册（网页端按昵称选人用）。
		rows, err = s.db.Query(`SELECT chat_id, user_id, username, role, bot, msgs, first_seen, last_seen
			FROM members ORDER BY last_seen DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`SELECT chat_id, user_id, username, role, bot, msgs, first_seen, last_seen
			FROM members WHERE chat_id = ? ORDER BY last_seen DESC LIMIT ?`, gid, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRec
	for rows.Next() {
		var m MemberRec
		var isBot int
		if err := rows.Scan(&m.ChatID, &m.UserID, &m.Username, &m.Role, &isBot, &m.Msgs, &m.FirstSeen, &m.LastSeen); err != nil {
			return nil, err
		}
		m.Bot = isBot != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// FindMembersByName 按昵称模糊找人（本地名册，不需要白名单接口）。
func (s *Store) FindMembersByName(groupChatID, name string) ([]MemberRec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT chat_id, user_id, username, role, bot, msgs, first_seen, last_seen
		FROM members WHERE chat_id = ? AND (username = ? OR username LIKE ?) ORDER BY last_seen DESC LIMIT 10`,
		groupChatID, name, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRec
	for rows.Next() {
		var m MemberRec
		var isBot int
		if err := rows.Scan(&m.ChatID, &m.UserID, &m.Username, &m.Role, &isBot, &m.Msgs, &m.FirstSeen, &m.LastSeen); err != nil {
			return nil, err
		}
		m.Bot = isBot != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------------- 单 Bot → 多 Bot 迁移 ----------------

// HasBusinessData 判断旧版单 Bot 数据库里是否有业务数据。
func (s *Store) HasBusinessData() bool {
	var n int
	_ = s.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM groups) +
		(SELECT COUNT(*) FROM reply_rules) +
		(SELECT COUNT(*) FROM banned_words) +
		(SELECT COUNT(*) FROM admins) +
		(SELECT COUNT(*) FROM mutes) +
		(SELECT COUNT(*) FROM blacklist) +
		(SELECT COUNT(*) FROM members) +
		(SELECT COUNT(*) FROM audit)`).Scan(&n)
	return n > 0
}

// CopyBusinessTo 把旧单 Bot 库的业务表复制到目标 Bot 库。
// 目标库必须已经用当前 schema 初始化过；settings 里 admin_token/tg_offset 不复制。
func (s *Store) CopyBusinessTo(dstPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`ATTACH DATABASE ? AS dst`, dstPath); err != nil {
		return err
	}
	defer func() { _, _ = s.db.Exec(`DETACH DATABASE dst`) }()
	for _, t := range []string{"groups", "admins", "reply_rules", "banned_words", "mutes", "blacklist", "members", "audit", "join_requests"} {
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO dst.` + t + ` SELECT * FROM main.` + t); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO dst.settings SELECT * FROM main.settings WHERE key NOT IN ('admin_token')`); err != nil {
		return err
	}
	_, _ = s.db.Exec(`INSERT OR REPLACE INTO main.settings(key,value,updated_at) VALUES('multi_bot_migrated',?,?)`, NowStr(), NowStr())
	return nil
}

// ---------------- bots（多 Bot 注册表） ----------------

func (s *Store) Bots() ([]BotRec, error) {
	rows, err := s.db.Query(`SELECT id, name, token, username, telegram_id, platform, platform_id, mode, webhook_url, webhook_secret, webhook_listen, webhook_path, enabled, created_at, updated_at FROM bots ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BotRec
	for rows.Next() {
		var b BotRec
		var enabled int
		if err := rows.Scan(&b.ID, &b.Name, &b.Token, &b.Username, &b.TelegramID, &b.Platform, &b.PlatformID, &b.Mode,
			&b.WebhookURL, &b.WebhookSecret, &b.WebhookListen, &b.WebhookPath,
			&enabled, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.Enabled = enabled != 0
		if b.Mode == "" {
			b.Mode = "polling"
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) Bot(id int64) (BotRec, bool) {
	row := s.db.QueryRow(`SELECT id, name, token, username, telegram_id, platform, platform_id, mode, webhook_url, webhook_secret, webhook_listen, webhook_path, enabled, created_at, updated_at FROM bots WHERE id = ?`, id)
	var b BotRec
	var enabled int
	if err := row.Scan(&b.ID, &b.Name, &b.Token, &b.Username, &b.TelegramID, &b.Platform, &b.PlatformID, &b.Mode,
		&b.WebhookURL, &b.WebhookSecret, &b.WebhookListen, &b.WebhookPath,
		&enabled, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return BotRec{}, false
	}
	b.Enabled = enabled != 0
	if b.Mode == "" {
		b.Mode = "polling"
	}
	return b, true
}

func (s *Store) AddBot(b BotRec) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	if b.Name == "" {
		b.Name = "未命名 Bot"
	}
	if b.Mode == "" {
		b.Mode = "polling"
	}
	if b.Platform == "" {
		b.Platform = "telegram"
	}
	res, err := s.db.Exec(`INSERT INTO bots(name, token, username, telegram_id, platform, platform_id, mode, webhook_url, webhook_secret, webhook_listen, webhook_path, enabled, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.Name, b.Token, b.Username, b.TelegramID, b.Platform, b.PlatformID, b.Mode,
		b.WebhookURL, b.WebhookSecret, b.WebhookListen, b.WebhookPath,
		BoolInt(b.Enabled), now, now)
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

// UpdateBot 更新注册表；b.Token 为空表示保留原 token。
func (s *Store) UpdateBot(b BotRec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.Mode == "" {
		b.Mode = "polling"
	}
	if b.Platform == "" {
		b.Platform = "telegram"
	}
	var err error
	if strings.TrimSpace(b.Token) != "" {
		_, err = s.db.Exec(`UPDATE bots SET name=?, token=?, username=?, telegram_id=?, platform=?, platform_id=?, mode=?, webhook_url=?, webhook_secret=?, webhook_listen=?, webhook_path=?, enabled=?, updated_at=? WHERE id=?`,
			b.Name, b.Token, b.Username, b.TelegramID, b.Platform, b.PlatformID, b.Mode,
			b.WebhookURL, b.WebhookSecret, b.WebhookListen, b.WebhookPath,
			BoolInt(b.Enabled), NowStr(), b.ID)
	} else {
		_, err = s.db.Exec(`UPDATE bots SET name=?, username=?, telegram_id=?, platform=?, platform_id=?, mode=?, webhook_url=?, webhook_secret=?, webhook_listen=?, webhook_path=?, enabled=?, updated_at=? WHERE id=?`,
			b.Name, b.Username, b.TelegramID, b.Platform, b.PlatformID, b.Mode,
			b.WebhookURL, b.WebhookSecret, b.WebhookListen, b.WebhookPath,
			BoolInt(b.Enabled), NowStr(), b.ID)
	}
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) DeleteBot(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM bots WHERE id = ?`, id); err != nil {
		return err
	}
	s.bump()
	return nil
}

func (s *Store) SetBotIdentity(id int64, username string, telegramID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE bots SET username=?, telegram_id=?, updated_at=? WHERE id=?`,
		username, telegramID, NowStr(), id); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- join requests ----------------

// UpsertJoinRequest 记录/刷新一条入群申请（同一用户重复申请会回到 pending）。
func (s *Store) UpsertJoinRequest(chatID, userID, username, bio string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	_, err := s.db.Exec(`INSERT INTO join_requests(chat_id, user_id, username, bio, status, created_at, updated_at)
		VALUES(?,?,?,?, 'pending', ?, ?)
		ON CONFLICT(chat_id, user_id) DO UPDATE SET
			username = excluded.username,
			bio      = excluded.bio,
			status   = 'pending',
			updated_at = excluded.updated_at`,
		chatID, userID, username, bio, now, now)
	if err == nil {
		s.bump()
	}
	return err
}

// JoinRequests 取入群申请。status 为空表示全部。
func (s *Store) JoinRequests(chatID, status string, limit int) ([]JoinRec, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, chat_id, user_id, username, bio, status, created_at, updated_at FROM join_requests WHERE 1=1`
	args := []interface{}{}
	if chatID != "" {
		q += ` AND chat_id = ?`
		args = append(args, chatID)
	}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JoinRec
	for rows.Next() {
		var j JoinRec
		if err := rows.Scan(&j.ID, &j.ChatID, &j.UserID, &j.Username, &j.Bio, &j.Status, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// SetJoinRequestStatus 更新入群申请状态（approved/declined）。
func (s *Store) SetJoinRequestStatus(chatID, userID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE join_requests SET status = ?, updated_at = ? WHERE chat_id = ? AND user_id = ?`,
		status, NowStr(), chatID, userID)
	if err == nil {
		s.bump()
	}
	return err
}

// ---------------- audit ----------------

func (s *Store) AddAudit(a AuditRec) error {
	if a.Ts == "" {
		a.Ts = NowStr()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO audit(ts, kind, chat_id, user_id, username, content, hit, actions, result, detail)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.Ts, a.Kind, a.ChatID, a.UserID, a.Username, a.Content, a.Hit, a.Actions, a.Result, a.Detail)
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Audit(limit int, groupChatID, kind string) ([]AuditRec, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	q := `SELECT id, ts, kind, chat_id, user_id, username, content, hit, actions, result, detail FROM audit WHERE 1=1`
	args := []interface{}{}
	if groupChatID != "" {
		q += ` AND chat_id = ?`
		args = append(args, groupChatID)
	}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRec
	for rows.Next() {
		var a AuditRec
		if err := rows.Scan(&a.ID, &a.Ts, &a.Kind, &a.ChatID, &a.UserID, &a.Username,
			&a.Content, &a.Hit, &a.Actions, &a.Result, &a.Detail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// PruneAudit 只保留最近 keep 条，避免手机存储被日志吃满。
func (s *Store) PruneAudit(keep int) error {
	if keep <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM audit WHERE id NOT IN (SELECT id FROM audit ORDER BY id DESC LIMIT ?)`, keep)
	return err
}

// PublicSettings 返回可以展示/导出的设置（去掉管理端 token 与长轮询 offset）。
func (s *Store) PublicSettings() map[string]string {
	m := s.Settings()
	delete(m, "admin_token")
	delete(m, "tg_offset")
	return m
}

// ExportJSON 把全部配置导出成一份 JSON（备份/迁移用）。
func (s *Store) ExportJSON() ([]byte, error) {
	rules, _ := s.Rules()
	words, _ := s.Words()
	groups, _ := s.Groups()
	admins, _ := s.Admins()
	bl, _ := s.Blacklists("")
	mutes, _ := s.Mutes(true)
	payload := map[string]interface{}{
		"exported_at": NowStr(),
		"settings":    s.PublicSettings(),
		"groups":      groups,
		"admins":      admins,
		"rules":       rules,
		"words":       words,
		"blacklist":   bl,
		"mutes":       mutes,
	}
	return json.MarshalIndent(payload, "", "  ")
}

// ---------------- 小工具 ----------------

func BoolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func BoolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func Itoa(n int) string { return fmt.Sprintf("%d", n) }

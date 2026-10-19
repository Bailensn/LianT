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
const SchemaVersion = 1

// SchemaStatements 是 v1 的全量建表语句（幂等，可重复执行）。
var SchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS groups (
		group_openid            TEXT PRIMARY KEY,
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
		member_openid TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT 'admin',
		group_openid  TEXT NOT NULL DEFAULT '',
		enabled       INTEGER NOT NULL DEFAULT 1,
		created_at    TEXT NOT NULL DEFAULT '',
		UNIQUE(member_openid, group_openid)
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
		group_openid  TEXT NOT NULL,
		member_openid TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		expire_at     TEXT NOT NULL DEFAULT '',
		reason        TEXT NOT NULL DEFAULT '',
		operator      TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'active',
		created_at    TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS blacklist (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		group_openid  TEXT NOT NULL,
		member_openid TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		reason        TEXT NOT NULL DEFAULT '',
		operator      TEXT NOT NULL DEFAULT '',
		qq_synced     INTEGER NOT NULL DEFAULT 0,
		created_at    TEXT NOT NULL DEFAULT '',
		UNIQUE(group_openid, member_openid)
	)`,
	`CREATE TABLE IF NOT EXISTS members (
		group_openid  TEXT NOT NULL,
		member_openid TEXT NOT NULL,
		username      TEXT NOT NULL DEFAULT '',
		role          TEXT NOT NULL DEFAULT '',
		Bot INTEGER NOT NULL DEFAULT 0,
		msgs          INTEGER NOT NULL DEFAULT 0,
		first_seen    TEXT NOT NULL DEFAULT '',
		last_seen     TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (group_openid, member_openid)
	)`,
	`CREATE TABLE IF NOT EXISTS audit (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		ts            TEXT NOT NULL DEFAULT '',
		kind          TEXT NOT NULL DEFAULT '',
		group_openid  TEXT NOT NULL DEFAULT '',
		member_openid TEXT NOT NULL DEFAULT '',
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
	`CREATE INDEX IF NOT EXISTS idx_blacklist_group ON blacklist(group_openid)`,
}

// ---------------- 记录类型 ----------------

type GroupRec struct {
	OpenID           string
	Name             string
	Enabled          bool
	ReplyAll         bool
	CommandsEnabled  bool
	GroupAdminCanCmd bool
	BotIsAdmin       bool
	Note             string
	JoinedAt         string
	UpdatedAt        string
}

type AdminRec struct {
	ID           int64
	MemberOpenID string
	Username     string
	Role         string // super | admin
	GroupOpenID  string // 空 = 全局管理员
	Enabled      bool
	CreatedAt    string
}

type RuleRec struct {
	ID        int64
	Priority  int
	Match     string
	Pattern   string
	Reply     string
	Fold      bool
	Always    bool
	Scope     string
	Enabled   bool
	Note      string
	CreatedAt string
	UpdatedAt string
}

type WordRec struct {
	ID          int64
	Word        string
	Match       string
	Fold        bool
	Scope       string
	Reply       string
	Recall      bool
	MuteSeconds int
	Kick        bool
	Blacklist   bool
	Severity    int
	Enabled     bool
	Note        string
	CreatedAt   string
	UpdatedAt   string
}

type MuteRec struct {
	ID           int64
	GroupOpenID  string
	MemberOpenID string
	Username     string
	ExpireAt     string
	Reason       string
	Operator     string
	Status       string
	CreatedAt    string
}

type BlackRec struct {
	ID           int64
	GroupOpenID  string
	MemberOpenID string
	Username     string
	Reason       string
	Operator     string
	QQSynced     bool
	CreatedAt    string
}

type MemberRec struct {
	GroupOpenID  string
	MemberOpenID string
	Username     string
	Role         string
	Bot          bool
	Msgs         int64
	FirstSeen    string
	LastSeen     string
}

type AuditRec struct {
	ID           int64
	Ts           string
	Kind         string
	GroupOpenID  string
	MemberOpenID string
	Username     string
	Content      string
	Hit          string
	Actions      string
	Result       string
	Detail       string
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
		return nil, fmt.Errorf("数据库路径为空（路径初始化失败，先跑 qbot path 看看）")
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

const GroupCols = `group_openid, name, enabled, reply_all, commands_enabled,
	group_admin_can_command, bot_is_admin, note, joined_at, updated_at`

func ScanGroup(sc interface{ Scan(...interface{}) error }) (GroupRec, error) {
	var g GroupRec
	var enabled, replyAll, cmds, adminsCan, botAdmin int
	err := sc.Scan(&g.OpenID, &g.Name, &enabled, &replyAll, &cmds, &adminsCan, &botAdmin,
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

func (s *Store) Group(openid string) (GroupRec, bool) {
	row := s.db.QueryRow(`SELECT `+GroupCols+` FROM groups WHERE group_openid = ?`, openid)
	g, err := ScanGroup(row)
	if err != nil {
		return GroupRec{}, false
	}
	return g, true
}

// EnsureGroup 机器人入群/首次见到消息时登记群（默认开启，且不清空已有设置）。
func (s *Store) EnsureGroup(openid, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	_, err := s.db.Exec(`INSERT INTO groups(group_openid, name, joined_at, updated_at)
		VALUES(?,?,?,?)
		ON CONFLICT(group_openid) DO UPDATE SET
			name = CASE WHEN excluded.Name <> '' THEN excluded.Name ELSE groups.Name END,
			updated_at = excluded.updated_at`,
		openid, name, now, now)
	if err == nil {
		s.bump()
	}
	return err
}

// UpdateGroup 只更新显式给出的字段（用指针区分"没传"和"设为空/false"）。
func (s *Store) UpdateGroup(openid string, name *string, enabled, replyAll, cmds, groupAdminsCanCmd, botIsAdmin *bool, note *string) error {
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
	args = append(args, openid)
	if _, err := s.db.Exec(`UPDATE groups SET `+strings.Join(sets, ", ")+` WHERE group_openid = ?`, args...); err != nil {
		return err
	}
	s.bump()
	return nil
}

// DeleteGroup 删除群记录（会连带清掉该群的禁言/黑名单记录）。
func (s *Store) DeleteGroup(openid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range []string{
		`DELETE FROM groups WHERE group_openid = ?`,
		`DELETE FROM mutes WHERE group_openid = ?`,
		`DELETE FROM blacklist WHERE group_openid = ?`,
		`DELETE FROM members WHERE group_openid = ?`,
	} {
		if _, err := s.db.Exec(q, openid); err != nil {
			return err
		}
	}
	s.bump()
	return nil
}

// ---------------- admins ----------------

func (s *Store) Admins() ([]AdminRec, error) {
	rows, err := s.db.Query(`SELECT id, member_openid, username, role, group_openid, enabled, created_at
		FROM admins ORDER BY role, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminRec
	for rows.Next() {
		var a AdminRec
		var en int
		if err := rows.Scan(&a.ID, &a.MemberOpenID, &a.Username, &a.Role, &a.GroupOpenID, &en, &a.CreatedAt); err != nil {
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
	res, err := s.db.Exec(`INSERT INTO admins(member_openid, username, role, group_openid, enabled, created_at)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(member_openid, group_openid) DO UPDATE SET
			username=excluded.Username, role=excluded.Role, enabled=excluded.Enabled`,
		a.MemberOpenID, a.Username, a.Role, a.GroupOpenID, BoolInt(a.Enabled || a.ID == 0), NowStr())
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
// scope 为群 openid：全局管理员（group_openid=”)与群级管理员都算。
func (s *Store) AdminRole(groupOpenID, memberOpenID string) (string, bool) {
	if memberOpenID == "" {
		return "", false
	}
	rows, err := s.db.Query(`SELECT role FROM admins
		WHERE member_openid = ? AND enabled = 1 AND (group_openid = '' OR group_openid = ?)
		ORDER BY CASE role WHEN 'super' THEN 0 ELSE 1 END LIMIT 1`, memberOpenID, groupOpenID)
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
	res, err := s.db.Exec(`INSERT INTO mutes(group_openid, member_openid, username, expire_at, reason, operator, status, created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		m.GroupOpenID, m.MemberOpenID, m.Username, m.ExpireAt, m.Reason, m.Operator, m.Status, NowStr())
	if err != nil {
		return 0, err
	}
	s.bump()
	return res.LastInsertId()
}

func (s *Store) Mutes(activeOnly bool) ([]MuteRec, error) {
	q := `SELECT id, group_openid, member_openid, username, expire_at, reason, operator, status, created_at FROM mutes`
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
		if err := rows.Scan(&m.ID, &m.GroupOpenID, &m.MemberOpenID, &m.Username, &m.ExpireAt,
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
func (s *Store) EndMuteByMember(groupOpenID, memberOpenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE mutes SET status = 'ended'
		WHERE group_openid = ? AND member_openid = ? AND status = 'active'`, groupOpenID, memberOpenID); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- blacklist ----------------

func (s *Store) AddBlacklist(b BlackRec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO blacklist(group_openid, member_openid, username, reason, operator, qq_synced, created_at)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(group_openid, member_openid) DO UPDATE SET
			username=excluded.Username, reason=excluded.Reason, operator=excluded.Operator`,
		b.GroupOpenID, b.MemberOpenID, b.Username, b.Reason, b.Operator, BoolInt(b.QQSynced), NowStr())
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Blacklists(groupOpenID string) ([]BlackRec, error) {
	q := `SELECT id, group_openid, member_openid, username, reason, operator, qq_synced, created_at FROM blacklist`
	args := []interface{}{}
	if groupOpenID != "" {
		q += ` WHERE group_openid = ?`
		args = append(args, groupOpenID)
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
		if err := rows.Scan(&b.ID, &b.GroupOpenID, &b.MemberOpenID, &b.Username, &b.Reason,
			&b.Operator, &synced, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.QQSynced = synced != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

// IsBlacklisted 命中本地黑名单（全局空群号的黑名单对所有群生效）。
func (s *Store) IsBlacklisted(groupOpenID, memberOpenID string) bool {
	if memberOpenID == "" {
		return false
	}
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM blacklist
		WHERE member_openid = ? AND (group_openid = '' OR group_openid = ?)`, memberOpenID, groupOpenID).Scan(&n)
	return err == nil && n > 0
}

func (s *Store) RemoveBlacklist(groupOpenID, memberOpenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM blacklist WHERE group_openid = ? AND member_openid = ?`,
		groupOpenID, memberOpenID); err != nil {
		return err
	}
	s.bump()
	return nil
}

func (s *Store) MarkBlacklistSynced(groupOpenID, memberOpenID string, synced bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`UPDATE blacklist SET qq_synced = ? WHERE group_openid = ? AND member_openid = ?`,
		BoolInt(synced), groupOpenID, memberOpenID); err != nil {
		return err
	}
	s.bump()
	return nil
}

// ---------------- members（自建群名册） ----------------

// TouchMember 每条群消息都会调用：记下昵称/角色，供"禁言 张三"这类指令解析用。
func (s *Store) TouchMember(groupOpenID, memberOpenID, username, role string, isBot bool) error {
	if groupOpenID == "" || memberOpenID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := NowStr()
	_, err := s.db.Exec(`INSERT INTO members(group_openid, member_openid, username, role, bot, msgs, first_seen, last_seen)
		VALUES(?,?,?,?,?,1,?,?)
		ON CONFLICT(group_openid, member_openid) DO UPDATE SET
			username = CASE WHEN excluded.Username <> '' THEN excluded.Username ELSE members.Username END,
			role     = CASE WHEN excluded.Role <> '' THEN excluded.Role ELSE members.Role END,
			msgs     = members.Msgs + 1,
			last_seen = excluded.last_seen`,
		groupOpenID, memberOpenID, username, role, BoolInt(isBot), now, now)
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Members(groupOpenID string, limit int) ([]MemberRec, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	gid := strings.TrimSpace(groupOpenID)
	var (
		rows *sql.Rows
		err  error
	)
	if gid == "" {
		// 空群号 = 跨群名册（网页端按昵称选人用）。
		rows, err = s.db.Query(`SELECT group_openid, member_openid, username, role, bot, msgs, first_seen, last_seen
			FROM members ORDER BY last_seen DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`SELECT group_openid, member_openid, username, role, bot, msgs, first_seen, last_seen
			FROM members WHERE group_openid = ? ORDER BY last_seen DESC LIMIT ?`, gid, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRec
	for rows.Next() {
		var m MemberRec
		var isBot int
		if err := rows.Scan(&m.GroupOpenID, &m.MemberOpenID, &m.Username, &m.Role, &isBot, &m.Msgs, &m.FirstSeen, &m.LastSeen); err != nil {
			return nil, err
		}
		m.Bot = isBot != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// FindMembersByName 按昵称模糊找人（本地名册，不需要白名单接口）。
func (s *Store) FindMembersByName(groupOpenID, name string) ([]MemberRec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`SELECT group_openid, member_openid, username, role, bot, msgs, first_seen, last_seen
		FROM members WHERE group_openid = ? AND (username = ? OR username LIKE ?) ORDER BY last_seen DESC LIMIT 10`,
		groupOpenID, name, "%"+name+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRec
	for rows.Next() {
		var m MemberRec
		var isBot int
		if err := rows.Scan(&m.GroupOpenID, &m.MemberOpenID, &m.Username, &m.Role, &isBot, &m.Msgs, &m.FirstSeen, &m.LastSeen); err != nil {
			return nil, err
		}
		m.Bot = isBot != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------------- audit ----------------

func (s *Store) AddAudit(a AuditRec) error {
	if a.Ts == "" {
		a.Ts = NowStr()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO audit(ts, kind, group_openid, member_openid, username, content, hit, actions, result, detail)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.Ts, a.Kind, a.GroupOpenID, a.MemberOpenID, a.Username, a.Content, a.Hit, a.Actions, a.Result, a.Detail)
	if err == nil {
		s.bump()
	}
	return err
}

func (s *Store) Audit(limit int, groupOpenID, kind string) ([]AuditRec, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	q := `SELECT id, ts, kind, group_openid, member_openid, username, content, hit, actions, result, detail FROM audit WHERE 1=1`
	args := []interface{}{}
	if groupOpenID != "" {
		q += ` AND group_openid = ?`
		args = append(args, groupOpenID)
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
		if err := rows.Scan(&a.ID, &a.Ts, &a.Kind, &a.GroupOpenID, &a.MemberOpenID, &a.Username,
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

// ---------------- JSON 导入（Reply.json → SQLite，一次性） ----------------

// ImportReplyConfig 把老的 Reply.json 规则导入数据库（只在没有导入过时执行）。
// 只收纯数据，避免 store 依赖上层类型。
func (s *Store) ImportReplyConfig(replyAll *bool, def *string, rules []RuleRec) (int, error) {
	if s.Setting("json_imported", "") != "" {
		return 0, nil
	}
	n := 0
	if replyAll != nil {
		if err := s.SetSetting("reply_all", BoolStr(*replyAll)); err != nil {
			return n, err
		}
	}
	if def != nil {
		if err := s.SetSetting("reply_default", *def); err != nil {
			return n, err
		}
	}
	for i := range rules {
		r := rules[i]
		r.ID = 0
		r.Note = "从 Reply.json 导入"
		r.Enabled = true
		r.CreatedAt = NowStr()
		if _, err := s.SaveRule(r); err != nil {
			return n, err
		}
		n++
	}
	if err := s.SetSetting("json_imported", NowStr()); err != nil {
		return n, err
	}
	return n, nil
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
		"settings":    s.Settings(),
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

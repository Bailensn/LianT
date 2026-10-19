package qqstore

import (
	"fmt"
	"time"

	lstore "LianT/internal/store"
)

// qbot 的 bot/actions/commands 是按 OpenID 字段写的。这里做一个薄适配器，
// 让 QQ runtime 复用 LianT 的 groups/admins/rules/... 表，同时不改 qbot 的业务代码。

type RuleRec = lstore.RuleRec
type WordRec = lstore.WordRec

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
	Role         string
	GroupOpenID  string
	Enabled      bool
	CreatedAt    string
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

type Store struct {
	L *lstore.Store
}

func New(l *lstore.Store) *Store { return &Store{L: l} }

func group(g lstore.GroupRec) GroupRec {
	return GroupRec{OpenID: g.ChatID, Name: g.Name, Enabled: g.Enabled, ReplyAll: g.ReplyAll,
		CommandsEnabled: g.CommandsEnabled, GroupAdminCanCmd: g.GroupAdminCanCmd,
		BotIsAdmin: g.BotIsAdmin, Note: g.Note, JoinedAt: g.JoinedAt, UpdatedAt: g.UpdatedAt}
}

func (s *Store) EnsureGroup(id, name string) error { return s.L.EnsureGroup(id, name) }

func (s *Store) Group(id string) (GroupRec, bool) {
	g, ok := s.L.Group(id)
	return group(g), ok
}

func (s *Store) UpdateGroup(gid string, name *string, enabled, replyAll, cmds, groupAdmin, botAdmin *bool, note *string) error {
	return s.L.UpdateGroup(gid, name, enabled, replyAll, cmds, groupAdmin, botAdmin, note)
}

func (s *Store) TouchMember(gid, mid, username, role string, isBot bool) error {
	return s.L.TouchMember(gid, mid, username, role, isBot)
}

func (s *Store) Setting(key, def string) string { return s.L.Setting(key, def) }
func (s *Store) Revision() int64               { return s.L.Revision() }

func (s *Store) Rules() ([]RuleRec, error) { return s.L.Rules() }
func (s *Store) Words() ([]WordRec, error) { return s.L.Words() }

func (s *Store) SaveWord(w WordRec) (int64, error) { return s.L.SaveWord(w) }
func (s *Store) DeleteWord(id int64) error          { return s.L.DeleteWord(id) }

func (s *Store) IsBlacklisted(gid, mid string) bool { return s.L.IsBlacklisted(gid, mid) }

func (s *Store) AddAudit(a AuditRec) error {
	return s.L.AddAudit(lstore.AuditRec{Ts: a.Ts, Kind: a.Kind, ChatID: a.GroupOpenID,
		UserID: a.MemberOpenID, Username: a.Username, Content: a.Content, Hit: a.Hit,
		Actions: a.Actions, Result: a.Result, Detail: a.Detail})
}

func (s *Store) AdminRole(gid, mid string) (string, bool) { return s.L.AdminRole(gid, mid) }

func (s *Store) Admins() ([]AdminRec, error) {
	list, err := s.L.Admins()
	out := make([]AdminRec, 0, len(list))
	for _, a := range list {
		out = append(out, AdminRec{ID: a.ID, MemberOpenID: a.UserID, Username: a.Username,
			Role: a.Role, GroupOpenID: a.ChatID, Enabled: a.Enabled, CreatedAt: a.CreatedAt})
	}
	return out, err
}

func (s *Store) AddAdmin(a AdminRec) (int64, error) {
	return s.L.AddAdmin(lstore.AdminRec{UserID: a.MemberOpenID, Username: a.Username,
		Role: a.Role, ChatID: a.GroupOpenID, Enabled: true})
}

func (s *Store) DeleteAdmin(id int64) error { return s.L.DeleteAdmin(id) }

func (s *Store) Members(gid string, limit int) ([]MemberRec, error) {
	list, err := s.L.Members(gid, limit)
	out := make([]MemberRec, 0, len(list))
	for _, m := range list {
		out = append(out, MemberRec{GroupOpenID: m.ChatID, MemberOpenID: m.UserID, Username: m.Username,
			Role: m.Role, Bot: m.Bot, Msgs: m.Msgs, FirstSeen: m.FirstSeen, LastSeen: m.LastSeen})
	}
	return out, err
}

func (s *Store) Mutes(activeOnly bool) ([]MuteRec, error) {
	list, err := s.L.Mutes(activeOnly)
	out := make([]MuteRec, 0, len(list))
	for _, m := range list {
		out = append(out, MuteRec{ID: m.ID, GroupOpenID: m.ChatID, MemberOpenID: m.UserID,
			Username: m.Username, ExpireAt: m.ExpireAt, Reason: m.Reason, Operator: m.Operator,
			Status: m.Status, CreatedAt: m.CreatedAt})
	}
	return out, err
}

func (s *Store) AddMute(m MuteRec) (int64, error) {
	return s.L.AddMute(lstore.MuteRec{ChatID: m.GroupOpenID, UserID: m.MemberOpenID,
		Username: m.Username, ExpireAt: m.ExpireAt, Reason: m.Reason, Operator: m.Operator,
		Status: m.Status})
}

func (s *Store) EndMuteByMember(gid, mid string) error { return s.L.EndMuteByMember(gid, mid) }

func (s *Store) Blacklists(gid string) ([]BlackRec, error) {
	list, err := s.L.Blacklists(gid)
	out := make([]BlackRec, 0, len(list))
	for _, b := range list {
		out = append(out, BlackRec{ID: b.ID, GroupOpenID: b.ChatID, MemberOpenID: b.UserID,
			Username: b.Username, Reason: b.Reason, Operator: b.Operator, QQSynced: b.TGSynced,
			CreatedAt: b.CreatedAt})
	}
	return out, err
}

func (s *Store) AddBlacklist(b BlackRec) error {
	return s.L.AddBlacklist(lstore.BlackRec{ChatID: b.GroupOpenID, UserID: b.MemberOpenID,
		Username: b.Username, Reason: b.Reason, Operator: b.Operator, TGSynced: b.QQSynced})
}

func (s *Store) RemoveBlacklist(gid, mid string) error { return s.L.RemoveBlacklist(gid, mid) }

func Itoa(n int) string       { return fmt.Sprintf("%d", n) }
func BoolStr(b bool) string   { return fmt.Sprintf("%t", b) }
func NowStr() string          { return time.Now().Format(time.RFC3339) }

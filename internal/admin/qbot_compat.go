package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"LianT/internal/bot"
	"LianT/internal/manager"
	"LianT/internal/store"
	"LianT/internal/tg"
)

// ============================================================================
// qbot 页面兼容层
//
// qbot 的 admin.html 用 OpenID / MemberOpenID / GroupOpenID 这套字段。
// LianT 内部统一用 chat_id / user_id；这里做双向映射，让同一份 UI 可以先跑在
// Telegram 上，也为以后 QQ adapter 保留字段语义。
//
// 只接管 qbot 页面使用的路由；LianT 原生 /api/chats、/api/bots 等保持不变。
// ============================================================================

type qctx struct {
	b      *bot.Bot
	st     *store.Store
	dbFile string
}

func qm(kv ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[fmt.Sprint(kv[i])] = kv[i+1]
	}
	return m
}

func qGetStr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case json.Number:
				return x.String()
			case float64:
				return strconv.FormatInt(int64(x), 10)
			}
		}
	}
	return ""
}

func qGetInt(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case json.Number:
				n, _ := x.Int64()
				return n
			case string:
				n, _ := strconv.ParseInt(x, 10, 64)
				return n
			}
		}
	}
	return 0
}

func qGetBool(m map[string]interface{}, keys ...string) (bool, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case bool:
				return x, true
			case string:
				return strings.EqualFold(x, "true") || x == "1" || x == "on", true
			case float64:
				return x != 0, true
			}
		}
	}
	return false, false
}

func qGroup(g store.GroupRec) map[string]interface{} {
	return qm(
		"OpenID", g.ChatID, "ChatID", g.ChatID, "chat_id", g.ChatID,
		"Name", g.Name, "name", g.Name,
		"Enabled", g.Enabled, "enabled", g.Enabled,
		"ReplyAll", g.ReplyAll, "reply_all", g.ReplyAll,
		"CommandsEnabled", g.CommandsEnabled, "commands_enabled", g.CommandsEnabled,
		"GroupAdminCanCmd", g.GroupAdminCanCmd, "group_admin_can_command", g.GroupAdminCanCmd,
		"BotIsAdmin", g.BotIsAdmin, "bot_is_admin", g.BotIsAdmin,
		"Note", g.Note, "note", g.Note,
		"JoinedAt", g.JoinedAt, "joined_at", g.JoinedAt,
		"UpdatedAt", g.UpdatedAt, "updated_at", g.UpdatedAt,
	)
}

func qRule(r store.RuleRec) map[string]interface{} {
	return qm(
		"ID", r.ID, "id", r.ID,
		"Priority", r.Priority, "priority", r.Priority,
		"Match", r.Match, "match", r.Match,
		"Pattern", r.Pattern, "pattern", r.Pattern,
		"Reply", r.Reply, "reply", r.Reply,
		"Fold", r.Fold, "fold", r.Fold,
		"Always", r.Always, "always", r.Always,
		"Scope", r.Scope, "scope", r.Scope,
		"Enabled", r.Enabled, "enabled", r.Enabled,
		"Note", r.Note, "note", r.Note,
		"CreatedAt", r.CreatedAt, "created_at", r.CreatedAt,
		"UpdatedAt", r.UpdatedAt, "updated_at", r.UpdatedAt,
	)
}

func qWord(w store.WordRec) map[string]interface{} {
	return qm(
		"ID", w.ID, "id", w.ID,
		"Word", w.Word, "word", w.Word,
		"Match", w.Match, "match", w.Match,
		"Fold", w.Fold, "fold", w.Fold,
		"Scope", w.Scope, "scope", w.Scope,
		"Reply", w.Reply, "reply", w.Reply,
		"Recall", w.Recall, "recall", w.Recall,
		"MuteSeconds", w.MuteSeconds, "mute_seconds", w.MuteSeconds,
		"Kick", w.Kick, "kick", w.Kick,
		"Blacklist", w.Blacklist, "blacklist", w.Blacklist,
		"Severity", w.Severity, "severity", w.Severity,
		"Enabled", w.Enabled, "enabled", w.Enabled,
		"Note", w.Note, "note", w.Note,
		"CreatedAt", w.CreatedAt, "created_at", w.CreatedAt,
		"UpdatedAt", w.UpdatedAt, "updated_at", w.UpdatedAt,
	)
}

func qAdmin(a store.AdminRec) map[string]interface{} {
	return qm(
		"ID", a.ID, "id", a.ID,
		"MemberOpenID", a.UserID, "UserID", a.UserID, "user_id", a.UserID,
		"Username", a.Username, "username", a.Username,
		"Role", a.Role, "role", a.Role,
		"GroupOpenID", a.ChatID, "ChatID", a.ChatID, "chat_id", a.ChatID,
		"Enabled", a.Enabled, "enabled", a.Enabled,
		"CreatedAt", a.CreatedAt, "created_at", a.CreatedAt,
	)
}

func qMember(m store.MemberRec) map[string]interface{} {
	return qm(
		"GroupOpenID", m.ChatID, "ChatID", m.ChatID, "chat_id", m.ChatID,
		"MemberOpenID", m.UserID, "UserID", m.UserID, "user_id", m.UserID,
		"Username", m.Username, "username", m.Username,
		"Role", m.Role, "role", m.Role,
		"Bot", m.Bot, "bot", m.Bot,
		"Msgs", m.Msgs, "msgs", m.Msgs,
		"FirstSeen", m.FirstSeen, "first_seen", m.FirstSeen,
		"LastSeen", m.LastSeen, "last_seen", m.LastSeen,
	)
}

func qMute(m store.MuteRec) map[string]interface{} {
	return qm(
		"ID", m.ID, "id", m.ID,
		"GroupOpenID", m.ChatID, "ChatID", m.ChatID, "chat_id", m.ChatID,
		"MemberOpenID", m.UserID, "UserID", m.UserID, "user_id", m.UserID,
		"Username", m.Username, "username", m.Username,
		"ExpireAt", m.ExpireAt, "expire_at", m.ExpireAt,
		"Reason", m.Reason, "reason", m.Reason,
		"Operator", m.Operator, "operator", m.Operator,
		"Status", m.Status, "status", m.Status,
		"CreatedAt", m.CreatedAt, "created_at", m.CreatedAt,
	)
}

func qBlack(b store.BlackRec) map[string]interface{} {
	return qm(
		"ID", b.ID, "id", b.ID,
		"GroupOpenID", b.ChatID, "ChatID", b.ChatID, "chat_id", b.ChatID,
		"MemberOpenID", b.UserID, "UserID", b.UserID, "user_id", b.UserID,
		"Username", b.Username, "username", b.Username,
		"Reason", b.Reason, "reason", b.Reason,
		"Operator", b.Operator, "operator", b.Operator,
		"QQSynced", b.TGSynced, "TGSynced", b.TGSynced, "tg_synced", b.TGSynced,
		"CreatedAt", b.CreatedAt, "created_at", b.CreatedAt,
	)
}

func qAudit(a store.AuditRec) map[string]interface{} {
	return qm(
		"ID", a.ID, "id", a.ID,
		"Ts", a.Ts, "ts", a.Ts,
		"Kind", a.Kind, "kind", a.Kind,
		"GroupOpenID", a.ChatID, "ChatID", a.ChatID, "chat_id", a.ChatID,
		"MemberOpenID", a.UserID, "UserID", a.UserID, "user_id", a.UserID,
		"Username", a.Username, "username", a.Username,
		"Content", a.Content, "content", a.Content,
		"Hit", a.Hit, "hit", a.Hit,
		"Actions", a.Actions, "actions", a.Actions,
		"Result", a.Result, "result", a.Result,
		"Detail", a.Detail, "detail", a.Detail,
	)
}

func (a *Server) compat(w http.ResponseWriter, r *http.Request, route, method string, rt *manager.Runtime, b *bot.Bot, st *store.Store) bool {
	if rt == nil || b == nil || st == nil {
		return false
	}
	c := &qctx{b: b, st: st, dbFile: manager.BotDBPath(rt.Rec.ID)}
	switch route {
	case "overview":
		c.overview(w, r)
	case "rules":
		c.rules(w, r, method)
	case "words":
		c.words(w, r, method)
	case "groups":
		c.groups(w, r, method)
	case "groups/sync-names":
		c.syncGroupNames(w, r, method)
	case "admins":
		c.admins(w, r, method)
	case "members":
		c.members(w, r)
	case "mutes":
		c.mutes(w, r, method)
	case "blacklist":
		c.blacklist(w, r, method)
	case "audit":
		c.audit(w, r)
	case "joins":
		c.joins(w, r, method)
	case "chat/groups":
		c.chatGroups(w, r)
	case "chat/recent":
		c.chatRecent(w, r)
	case "chat/send":
		c.chatSend(w, r, method)
	case "qq/botstate":
		c.qqBotState(w, r)
	case "qq/groupinfo":
		c.qqGroupInfo(w, r)
	case "qq/members":
		c.qqMembers(w, r)
	case "qq/mutestate":
		c.qqMuteState(w, r)
	case "qq/kick":
		c.qqKick(w, r, method)
	case "qq/recall":
		c.qqRecall(w, r, method)
	default:
		return false
	}
	return true
}

func (c *qctx) overview(w http.ResponseWriter, r *http.Request) {
	groups, _ := c.st.Groups()
	rules, _ := c.st.Rules()
	words, _ := c.st.Words()
	admins, _ := c.st.Admins()
	mutes, _ := c.st.Mutes(true)
	bl, _ := c.st.Blacklists("")
	recent, _ := c.st.Audit(20, "", "")
	joins, _ := c.st.JoinRequests("", "pending", 500)
	status := c.b.Status()
	outRecent := make([]map[string]interface{}, 0, len(recent))
	for _, item := range recent {
		outRecent = append(outRecent, qAudit(item))
	}
	ApiOK(w, map[string]interface{}{
		"bot":      status,
		"db_file":  c.dbFile,
		"counts":   map[string]int{"groups": len(groups), "chats": len(groups), "rules": len(rules), "words": len(words), "admins": len(admins), "mutes": len(mutes), "blacklist": len(bl), "joins": len(joins)},
		"settings":  c.st.PublicSettings(),
		"recent":    outRecent,
		"whitelist": "Telegram 不提供完整成员列表；群信息/管理员列表可用。",
		"cmd_help":  bot.HelpText(),
	})
}

func (c *qctx) rules(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case http.MethodGet:
		list, err := c.st.Rules()
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list { out = append(out, qRule(item)) }
		ApiOK(w, out)
	case http.MethodPost, http.MethodPut:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		rec := store.RuleRec{
			ID: qGetInt(in, "ID", "id"), Priority: int(qGetInt(in, "Priority", "priority")),
			Match: qGetStr(in, "Match", "match"), Pattern: qGetStr(in, "Pattern", "pattern"),
			Reply: qGetStr(in, "Reply", "reply"), Scope: qGetStr(in, "Scope", "scope"),
			Note: qGetStr(in, "Note", "note"),
		}
		if v, ok := qGetBool(in, "Fold", "fold"); ok { rec.Fold = v }
		if v, ok := qGetBool(in, "Always", "always"); ok { rec.Always = v }
		if v, ok := qGetBool(in, "Enabled", "enabled"); ok { rec.Enabled = v } else if rec.ID == 0 { rec.Enabled = true }
		if rec.Match == "" { rec.Match = "exact" }
		if rec.Priority == 0 { rec.Priority = 100 }
		id, err := c.st.SaveRule(rec)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		c.b.InvalidateRules()
		ApiOK(w, map[string]interface{}{"id": id})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 { ApiErr(w, 400, "缺少 id"); return }
		if err := c.st.DeleteRule(id); err != nil { ApiErr(w, 500, err.Error()); return }
		c.b.InvalidateRules()
		ApiOK(w, "deleted")
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) words(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case http.MethodGet:
		list, err := c.st.Words()
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list { out = append(out, qWord(item)) }
		ApiOK(w, out)
	case http.MethodPost, http.MethodPut:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		rec := store.WordRec{
			ID: qGetInt(in, "ID", "id"), Word: qGetStr(in, "Word", "word"),
			Match: qGetStr(in, "Match", "match"), Scope: qGetStr(in, "Scope", "scope"),
			Reply: qGetStr(in, "Reply", "reply"), Note: qGetStr(in, "Note", "note"),
			MuteSeconds: int(qGetInt(in, "MuteSeconds", "mute_seconds")),
			Severity: int(qGetInt(in, "Severity", "severity")),
		}
		if rec.Word == "" { ApiErr(w, 400, "违禁词不能为空"); return }
		if rec.Match == "" { rec.Match = "contains" }
		if rec.Severity == 0 { rec.Severity = 100 }
		if v, ok := qGetBool(in, "Recall", "recall"); ok { rec.Recall = v }
		if v, ok := qGetBool(in, "Kick", "kick"); ok { rec.Kick = v }
		if v, ok := qGetBool(in, "Blacklist", "blacklist"); ok { rec.Blacklist = v }
		if v, ok := qGetBool(in, "Fold", "fold"); ok { rec.Fold = v }
		if v, ok := qGetBool(in, "Enabled", "enabled"); ok { rec.Enabled = v } else if rec.ID == 0 { rec.Enabled = true }
		id, err := c.st.SaveWord(rec)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		c.b.InvalidateRules()
		ApiOK(w, map[string]interface{}{"id": id})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 { ApiErr(w, 400, "缺少 id"); return }
		if err := c.st.DeleteWord(id); err != nil { ApiErr(w, 500, err.Error()); return }
		c.b.InvalidateRules()
		ApiOK(w, "deleted")
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) groups(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case http.MethodGet:
		list, err := c.st.Groups()
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list { out = append(out, qGroup(item)) }
		ApiOK(w, out)
	case http.MethodPost:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		gid := qGetStr(in, "group_openid", "GroupOpenID", "chat_id", "ChatID")
		if gid == "" { ApiErr(w, 400, "需要 group_openid"); return }
		_ = c.st.EnsureGroup(gid, "")
		var name, note *string
		if _, ok := in["name"]; ok { v := qGetStr(in, "name"); name = &v }
		if _, ok := in["Name"]; ok { v := qGetStr(in, "Name"); name = &v }
		if _, ok := in["note"]; ok { v := qGetStr(in, "note"); note = &v }
		if _, ok := in["Note"]; ok { v := qGetStr(in, "Note"); note = &v }
		var enabled, replyAll, cmds, groupAdmins, botIsAdmin *bool
		if v, ok := qGetBool(in, "enabled", "Enabled"); ok { enabled = &v }
		if v, ok := qGetBool(in, "reply_all", "ReplyAll"); ok { replyAll = &v }
		if v, ok := qGetBool(in, "commands_enabled", "CommandsEnabled"); ok { cmds = &v }
		if v, ok := qGetBool(in, "group_admin_can_command", "GroupAdminCanCmd"); ok { groupAdmins = &v }
		if v, ok := qGetBool(in, "bot_is_admin", "BotIsAdmin"); ok { botIsAdmin = &v }
		if err := c.st.UpdateGroup(gid, name, enabled, replyAll, cmds, groupAdmins, botIsAdmin, note); err != nil {
			ApiErr(w, 500, err.Error()); return
		}
		ApiOK(w, "updated")
	case http.MethodDelete:
		gid := r.URL.Query().Get("group_openid")
		if gid == "" { gid = r.URL.Query().Get("chat_id") }
		if gid == "" { ApiErr(w, 400, "缺少 group_openid"); return }
		if err := c.st.DeleteGroup(gid); err != nil { ApiErr(w, 500, err.Error()); return }
		ApiOK(w, "deleted")
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) syncGroupNames(w http.ResponseWriter, r *http.Request, method string) {
	if method != http.MethodPost { ApiErr(w, 405, "不支持的方法"); return }
	client := c.b.Client()
	if client == nil { ApiErr(w, 503, "机器人尚未连接"); return }
	groups, _ := c.st.Groups()
	updated, tried, permission := 0, 0, true
	var problems []string
	for _, g := range groups {
		if strings.TrimSpace(g.Name) != "" || tried >= 8 { continue }
		tried++
		info, err := client.GetChat(g.ChatID)
		if err != nil { problems = append(problems, err.Error()); continue }
		name := info.Display()
		_ = c.st.UpdateGroup(g.ChatID, &name, nil, nil, nil, nil, nil, nil)
		updated++
	}
	ApiOK(w, map[string]interface{}{"updated": updated, "tried": tried, "permission": permission, "problems": problems, "remaining": len(groups) - updated})
}

func (c *qctx) admins(w http.ResponseWriter, r *http.Request, method string) {
	switch method {
	case http.MethodGet:
		list, err := c.st.Admins()
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list { out = append(out, qAdmin(item)) }
		ApiOK(w, out)
	case http.MethodPost:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		uid := qGetStr(in, "MemberOpenID", "UserID", "member_openid", "user_id")
		if uid == "" { ApiErr(w, 400, "需要 member_openid"); return }
		rec := store.AdminRec{UserID: uid, Username: qGetStr(in, "Username", "username"), Role: qGetStr(in, "Role", "role"), ChatID: qGetStr(in, "GroupOpenID", "ChatID", "group_openid", "chat_id"), Enabled: true}
		if rec.Role == "" { rec.Role = "admin" }
		id, err := c.st.AddAdmin(rec)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		ApiOK(w, map[string]interface{}{"id": id})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 { ApiErr(w, 400, "缺少 id"); return }
		if err := c.st.DeleteAdmin(id); err != nil { ApiErr(w, 500, err.Error()); return }
		ApiOK(w, "deleted")
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) members(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gid := q.Get("group")
	if gid == "" { gid = q.Get("group_openid") }
	var out []map[string]interface{}
	if gid != "" {
		list, _ := c.st.Members(gid, 500)
		for _, item := range list { out = append(out, qMember(item)) }
	} else {
		groups, _ := c.st.Groups()
		for _, g := range groups {
			list, _ := c.st.Members(g.ChatID, 200)
			for _, item := range list { out = append(out, qMember(item)) }
		}
	}
	ApiOK(w, out)
}

func (c *qctx) mutes(w http.ResponseWriter, r *http.Request, method string) {
	q := r.URL.Query()
	gid := q.Get("group")
	switch method {
	case http.MethodGet:
		list, err := c.st.Mutes(true)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			if gid != "" && item.ChatID != gid { continue }
			out = append(out, qMute(item))
		}
		ApiOK(w, out)
	case http.MethodPost:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		gid = qGetStr(in, "group_openid", "GroupOpenID", "chat_id")
		uid := qGetStr(in, "member_openid", "MemberOpenID", "user_id")
		secs := qGetInt(in, "seconds")
		if secs <= 0 { secs = 600 }
		if gid == "" || uid == "" { ApiErr(w, 400, "需要群和成员"); return }
		res := c.b.MuteMember(bot.ActionCtx{ChatID: gid, UserID: uid, Kind: "manual", Hit: "网页后台禁言", Operator: "网页后台"}, time.Duration(secs)*time.Second, qGetStr(in, "reason"))
		ApiOK(w, res)
	case http.MethodDelete:
		uid := q.Get("member")
		if uid == "" { uid = q.Get("user_id") }
		if gid == "" || uid == "" { ApiErr(w, 400, "需要 group 和 member"); return }
		res := c.b.UnmuteMember(bot.ActionCtx{ChatID: gid, UserID: uid, Kind: "manual", Hit: "网页后台解禁", Operator: "网页后台"})
		ApiOK(w, res)
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) blacklist(w http.ResponseWriter, r *http.Request, method string) {
	q := r.URL.Query()
	gid := q.Get("group")
	switch method {
	case http.MethodGet:
		list, err := c.st.Blacklists(gid)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list { out = append(out, qBlack(item)) }
		ApiOK(w, out)
	case http.MethodPost:
		var in map[string]interface{}
		if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
		gid = qGetStr(in, "group_openid", "GroupOpenID", "chat_id")
		uid := qGetStr(in, "member_openid", "MemberOpenID", "user_id")
		if uid == "" { ApiErr(w, 400, "需要成员"); return }
		rec := store.BlackRec{ChatID: gid, UserID: uid, Username: qGetStr(in, "username", "Username"), Reason: qGetStr(in, "reason", "Reason"), Operator: "网页后台"}
		if err := c.st.AddBlacklist(rec); err != nil { ApiErr(w, 500, err.Error()); return }
		msg := []string{"已加入黑名单"}
		if kick, _ := qGetBool(in, "kick", "Kick"); kick {
			msg = c.b.KickMember(bot.ActionCtx{ChatID: gid, UserID: uid, Kind: "manual", Hit: "网页后台拉黑", Operator: "网页后台"})
		}
		ApiOK(w, msg)
	case http.MethodDelete:
		uid := q.Get("member")
		if gid == "" || uid == "" { ApiErr(w, 400, "需要 group 和 member"); return }
		if err := c.st.RemoveBlacklist(gid, uid); err != nil { ApiErr(w, 500, err.Error()); return }
		ApiOK(w, []string{"已移出黑名单"})
	default:
		ApiErr(w, 405, "不支持的方法")
	}
}

func (c *qctx) audit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 { limit = 200 }
	list, err := c.st.Audit(limit, q.Get("group"), q.Get("kind"))
	if err != nil { ApiErr(w, 500, err.Error()); return }
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list { out = append(out, qAudit(item)) }
	ApiOK(w, out)
}

func (c *qctx) joins(w http.ResponseWriter, r *http.Request, method string) {
	q := r.URL.Query()
	gid := q.Get("group")
	if method == http.MethodGet {
		list, err := c.st.JoinRequests(gid, "pending", 200)
		if err != nil { ApiErr(w, 500, err.Error()); return }
		out := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			out = append(out, map[string]interface{}{
				"username": item.Username, "member_openid": item.UserID,
				"verify_info": map[string]string{"verify_message": item.Bio},
				"apply_at": item.UpdatedAt, "status": item.Status,
				"join_request_id": "", "risk_tips": "",
			})
		}
		ApiOK(w, map[string]interface{}{"ok": true, "list": out})
		return
	}
	if method != http.MethodPost { ApiErr(w, 405, "不支持的方法"); return }
	var in map[string]interface{}
	if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
	gid = qGetStr(in, "group_openid", "GroupOpenID", "chat_id")
	uid := qGetStr(in, "member_openid", "MemberOpenID", "user_id")
	approve, _ := qGetBool(in, "approve", "Approve")
	if gid == "" || uid == "" { ApiErr(w, 400, "需要群和成员"); return }
	client := c.b.Client()
	if client == nil { ApiErr(w, 503, "机器人尚未连接"); return }
	var err error
	if approve {
		err = client.ApproveChatJoinRequest(gid, uid)
		_ = c.st.SetJoinRequestStatus(gid, uid, "approved")
	} else {
		err = client.DeclineChatJoinRequest(gid, uid)
		_ = c.st.SetJoinRequestStatus(gid, uid, "declined")
	}
	if err != nil { ApiOK(w, map[string]interface{}{"ok": false, "result": err.Error()}); return }
	ApiOK(w, map[string]interface{}{"ok": true, "result": "已处理"})
}

func (c *qctx) chatGroups(w http.ResponseWriter, r *http.Request) {
	groups, _ := c.st.Groups()
	out := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		if !g.Enabled { continue }
		out = append(out, map[string]interface{}{"openid": g.ChatID, "name": g.Name, "last_at": g.UpdatedAt})
	}
	ApiOK(w, out)
}

func (c *qctx) chatRecent(w http.ResponseWriter, r *http.Request) {
	gid := r.URL.Query().Get("group")
	if gid == "" { gid = r.URL.Query().Get("group_openid") }
	if gid == "" { ApiErr(w, 400, "需要 group"); return }
	list := c.b.RecentList(gid, 60, 0)
	out := make([]map[string]interface{}, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]interface{}{
			"id": m.MessageID, "username": m.Username, "member_openid": m.UserID,
			"content": m.Content, "kind": m.Kind, "is_bot": m.IsBot, "at": m.At.Format(time.RFC3339),
		})
	}
	ApiOK(w, map[string]interface{}{"group": gid, "messages": out})
}

func (c *qctx) chatSend(w http.ResponseWriter, r *http.Request, method string) {
	if method != http.MethodPost { ApiErr(w, 405, "不支持的方法"); return }
	var in struct {
		GroupOpenID string `json:"group_openid"`
		Format      string `json:"format"`
		Title       string `json:"title"`
		Content     string `json:"content"`
	}
	if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
	content := strings.TrimSpace(in.Content)
	if strings.TrimSpace(in.GroupOpenID) == "" { ApiErr(w, 400, "需要 group_openid"); return }
	if content == "" { ApiErr(w, 400, "消息内容不能为空"); return }
	text := content
	if strings.TrimSpace(in.Title) != "" { text = strings.TrimSpace(in.Title) + "\n\n" + content }
	if len([]rune(text)) > 4000 { ApiErr(w, 400, "消息内容太长（最多 4000 字）"); return }
	id, err := c.b.SendMessageOpt(in.GroupOpenID, text, tg.SendOptions{})
	if err != nil { ApiErr(w, 502, err.Error()); return }
	ApiOK(w, map[string]interface{}{"id": id, "group_openid": in.GroupOpenID, "format": "text"})
}

func (c *qctx) qqBotState(w http.ResponseWriter, r *http.Request) {
	st := c.b.Status()
	msg := "机器人状态：" + st["bot_state"]
	if v := st["bot_username"]; v != "" { msg += " · " + v }
	ApiOK(w, map[string]interface{}{"ok": true, "result": msg, "status": st})
}

func (c *qctx) qqGroupInfo(w http.ResponseWriter, r *http.Request) {
	client := c.b.Client()
	if client == nil { ApiErr(w, 503, "机器人尚未连接"); return }
	gid := r.URL.Query().Get("group")
	info, err := client.GetChat(gid)
	if err != nil { ApiErr(w, 502, err.Error()); return }
	ApiOK(w, info)
}

func (c *qctx) qqMembers(w http.ResponseWriter, r *http.Request) {
	client := c.b.Client()
	if client == nil { ApiErr(w, 503, "机器人尚未连接"); return }
	gid := r.URL.Query().Get("group")
	list, err := client.GetChatAdministrators(gid)
	if err != nil { ApiErr(w, 502, err.Error()); return }
	ApiOK(w, map[string]interface{}{"members": list, "note": "Telegram 只提供管理员列表，不提供普通成员列表。"})
}

func (c *qctx) qqMuteState(w http.ResponseWriter, r *http.Request) {
	// qbot 页面查的是"群当前禁言名单 + 全员禁言模式"。
	// Telegram 没有等价的禁言名单接口；返回空列表，避免页面报错。
	ApiOK(w, map[string]interface{}{
		"ok": true, "members": []interface{}{}, "global_mode": "none",
		"note": "Telegram 不提供群禁言名单；请按成员查看禁言状态。",
	})
}

func (c *qctx) qqKick(w http.ResponseWriter, r *http.Request, method string) {
	if method != http.MethodPost { ApiErr(w, 405, "不支持的方法"); return }
	var in struct {
		GroupOpenID string   `json:"group_openid"`
		MemberIDs   []string `json:"member_openids"`
		MemberID    string   `json:"member_openid"`
		AddBlack    bool     `json:"add_to_member_blacklist"`
	}
	if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
	ids := in.MemberIDs
	if len(ids) == 0 && in.MemberID != "" { ids = []string{in.MemberID} }
	if in.GroupOpenID == "" || len(ids) == 0 { ApiErr(w, 400, "需要群和成员"); return }
	var all []string
	for _, uid := range ids {
		if in.AddBlack {
			all = append(all, c.b.BanMember(bot.ActionCtx{ChatID: in.GroupOpenID, UserID: uid, Kind: "manual", Hit: "网页后台踢出", Operator: "网页后台"}, true)...)
		} else {
			all = append(all, c.b.KickMember(bot.ActionCtx{ChatID: in.GroupOpenID, UserID: uid, Kind: "manual", Hit: "网页后台踢出", Operator: "网页后台"})...)
		}
	}
	ApiOK(w, map[string]interface{}{"ok": true, "result": strings.Join(all, "；")})
}

func (c *qctx) qqRecall(w http.ResponseWriter, r *http.Request, method string) {
	if method != http.MethodPost { ApiErr(w, 405, "不支持的方法"); return }
	var in struct {
		GroupOpenID string `json:"group_openid"`
		MessageID   string `json:"message_id"`
		MemberID    string `json:"member_openid"`
		Count       int    `json:"count"`
	}
	if err := DecodeBody(r, &in); err != nil { ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error()); return }
	if in.GroupOpenID == "" { ApiErr(w, 400, "需要 group_openid"); return }
	if in.MessageID != "" {
		id, _ := strconv.ParseInt(in.MessageID, 10, 64)
		if err := c.b.RecallMessage(in.GroupOpenID, id); err != nil { ApiErr(w, 502, err.Error()); return }
		ApiOK(w, map[string]interface{}{"ok": true, "result": "已撤回"})
		return
	}
	count := in.Count
	if count <= 0 { count = 1 }
	list := c.b.RecentList(in.GroupOpenID, count, 48*time.Hour)
	ids := make([]int64, 0, len(list))
	for _, m := range list { ids = append(ids, m.MessageID) }
	n, err := c.b.RecallMessages(in.GroupOpenID, ids)
	if err != nil { ApiErr(w, 502, err.Error()); return }
	ApiOK(w, map[string]interface{}{"ok": true, "result": fmt.Sprintf("已撤回 %d 条", n)})
}

package qqbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	
	"LianT/internal/platform/qq"
	qqstore "LianT/internal/platform/qq/store"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// QQ 群管理接口 + 处置动作执行
//
// 官方文档（Bot.q.qq.com/wiki/develop/api-v2/autogen/）：
//   撤回群消息        DELETE /v2/groups/{gid}/messages/{mid}        公开，管理员可撤他人消息，限 2 分钟
//   设置/查询禁言      POST|GET /v2/groups/{gid}/restrict_chat_setting 公开，需机器人是群管理员，最长 30 天，单次≤20
//   群信息/机器人群内状态 GET /v2/groups/{gid}/info、/bot_state       群信息接口需白名单（11253）
//   群成员批量移除     POST /v2/groups/{gid}/batch_remove_members    ⚠️ 仅白名单机器人，可顺带拉黑
//   群黑名单增删查     GET|POST /v2/groups/{gid}/member_blacklist    ⚠️ 仅白名单机器人
//   群成员列表/成员信息 GET /v2/groups/{gid}/members[/{mid}]          ⚠️ 仅白名单机器人
//
// 拿不到白名单也不影响可用性：本地黑名单/禁言记录会先落库并立刻生效
// （不回消息、命中违禁词自动撤回+禁言），QQ 侧动作失败时把原因写进审计和回复。
// ============================================================================

// ---------------- 统一响应 ----------------

type QqResp struct {
	status int
	body   []byte
	code   int    // 业务错误码（如 11253）
	msg    string // 业务错误描述
}

func (r QqResp) Ok() bool { return r.status >= 200 && r.status < 300 && r.code == 0 }

// ErrorCode 返回 QQ 业务错误码（0 表示没有业务错误）。
func (r QqResp) ErrorCode() int { return r.code }

// Hint 给出人话版排查建议。
func (r QqResp) Hint() string {
	if r.Ok() {
		return ""
	}
	if h, found := QqErrHints[r.code]; found {
		return h
	}
	switch r.status {
	case 401, 403:
		return "鉴权/权限问题：检查凭据与机器人是否被移出群"
	case 404:
		return "接口或对象不存在：检查接口路径配置（QBOT_*_PATH）"
	case 429:
		return "触发频率限制，稍后重试"
	}
	if r.msg != "" {
		return r.msg
	}
	if len(r.body) > 0 {
		return strings.TrimSpace(string(r.body))
	}
	return fmt.Sprintf("HTTP %d", r.status)
}

func (r QqResp) String() string {
	if r.Ok() {
		return "ok"
	}
	if r.code != 0 {
		return fmt.Sprintf("失败(code=%d %s)", r.code, r.Hint())
	}
	return fmt.Sprintf("失败(%s)", r.Hint())
}

// 官方文档里的常见错误码 → 人话
var QqErrHints = map[int]string{
	11253:    "该接口仅白名单机器人可用：需联系平台运营申请权限（内邀）",
	40062003: "无操作权限：机器人需要是群管理员，且目标需是普通成员",
	40064004: "消息发送已超过 2 分钟，无法撤回",
	40061001: "请求参数无效",
	50065001: "消息撤回失败，请稍后重试",
	40054002: "机器人被禁言，无法发送消息",
	40054003: "机器人不是群成员",
	40034100: "触发主动消息频控（每个群每天 1000 条 / 20 条每分钟）",
	40034105: "主动消息发送失败，无权限",
	40034006: "消息内容被判定违规",
	40054007: "消息长度超限",
	40054010: "不允许发送 URL",
	40054016: "机器人已下线",
}

// ---------------- 底层调用 ----------------

// AppID 当前 AppID（凭据可能来自 .env，所以每次现取）。
func (b *Bot) AppID() string {
	id, _ := b.Creds()
	return id
}

// QqAPI 调一个 QQ 开放平台接口；out 非空时解析响应体。
func (b *Bot) QqAPI(method, url string, payload interface{}, out interface{}) QqResp {
	var r QqResp

	token, err := b.EnsureToken(false)
	if err != nil {
		r.msg = "取 token 失败: " + err.Error()
		return r
	}

	var bodyReader io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			r.msg = "序列化请求失败: " + err.Error()
			return r
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		r.msg = "构造请求失败: " + err.Error()
		return r
	}
	req.Header.Set("Authorization", "QQBot "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if app := b.AppID(); app != "" {
		req.Header.Set("X-Union-Appid", app)
	}

	resp, err := qq.HTTPClient.Do(req)
	if err != nil {
		r.msg = "请求失败: " + err.Error()
		return r
	}
	defer resp.Body.Close()

	r.status = resp.StatusCode
	r.body, _ = io.ReadAll(resp.Body)

	if out != nil && len(r.body) > 0 {
		_ = json.Unmarshal(r.body, out)
	}
	// 业务错误统一形如 {"code":11253,"message":"..."}
	if len(r.body) > 0 {
		var e struct {
			Code    json.Number `json:"code"`
			Message string      `json:"message"`
		}
		if json.Unmarshal(r.body, &e) == nil && e.Code != "" {
			if n, err := strconv.Atoi(e.Code.String()); err == nil && n != 0 {
				r.code, r.msg = n, e.Message
			}
		}
	}
	return r
}

// ---------------- 具体接口 ----------------

type MuteOp struct {
	Op           string `json:"op"` // add 增加 | update 更新到期时间 | del 解除
	MemberOpenID string `json:"member_openid"`
	MuteExpireAt string `json:"mute_expire_at,omitempty"`
}

// QqSetMutes 设置/更新/解除群成员禁言（需机器人是群管理员）。
func (b *Bot) QqSetMutes(groupOpenID string, ops []MuteOp) QqResp {
	if len(ops) == 0 {
		return QqResp{status: 200}
	}
	if len(ops) > 20 {
		ops = ops[:20] // 单次上限 20
	}
	return b.QqAPI("POST", qq.GroupMuteURL(groupOpenID), map[string]interface{}{"members": ops}, nil)
}

// QqRecallMessage 撤回群消息（他人消息需机器人是群管理员，且 2 分钟内）。
func (b *Bot) QqRecallMessage(groupOpenID, messageID string) QqResp {
	if groupOpenID == "" || messageID == "" {
		return QqResp{status: 200}
	}
	return b.QqAPI("DELETE", qq.GroupRecallURL(groupOpenID, messageID), nil, nil)
}

// QqRemoveMembers 批量移除群成员，可选同时加入群黑名单（仅白名单机器人可用）。
func (b *Bot) QqRemoveMembers(groupOpenID string, openids []string, addToBlacklist bool) QqResp {
	body := map[string]interface{}{"member_openids": openids}
	if addToBlacklist {
		body["add_to_member_blacklist"] = true
	}
	var out struct {
		Result  string   `json:"remove_members_result"`
		FailIDs []string `json:"add_to_member_blacklist_fail_openids"`
	}
	return b.QqAPI("POST", qq.GroupRemoveMembersURL(groupOpenID), body, &out)
}

// QqBlacklistOp 群黑名单增删（op=add/del，目标在群中时无法加入，仅白名单机器人可用）。
func (b *Bot) QqBlacklistOp(groupOpenID, op string, openids []string) QqResp {
	var out struct {
		FailIDs []string `json:"fail_openids"`
	}
	return b.QqAPI("POST", qq.GroupBlacklistURL(groupOpenID),
		map[string]interface{}{"op": op, "member_openids": openids}, &out)
}

// QqBlacklist 查询群黑名单（仅白名单机器人可用）。
func (b *Bot) QqBlacklist(groupOpenID, cursor string, limit int) ([]qqstore.MemberRec, string, QqResp) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	var out struct {
		Users []struct {
			UnionOpenID  string `json:"union_openid"`
			MemberOpenID string `json:"member_openid"`
			Username     string `json:"username"`
			BannedAt     string `json:"banned_at"`
			Bot          bool   `json:"bot"`
		} `json:"users"`
		NextCursor string `json:"next_cursor"`
	}
	url := qq.GroupBlacklistURL(groupOpenID) + "?limit=" + qqstore.Itoa(limit)
	if cursor != "" {
		url += "&cursor=" + cursor
	}
	r := b.QqAPI("GET", url, nil, &out)
	members := make([]qqstore.MemberRec, 0, len(out.Users))
	for _, u := range out.Users {
		members = append(members, qqstore.MemberRec{
			GroupOpenID: groupOpenID, MemberOpenID: u.MemberOpenID,
			Username: u.Username, Bot: u.Bot, LastSeen: u.BannedAt,
		})
	}
	return members, out.NextCursor, r
}

// QqMembers 群成员列表（仅白名单机器人可用），同时回写本地名册。
func (b *Bot) QqMembers(groupOpenID, cursor string) ([]qqstore.MemberRec, string, QqResp) {
	var out struct {
		Members []struct {
			MemberOpenID string `json:"member_openid"`
			Username     string `json:"username"`
			MemberRole   string `json:"member_role"`
			Bot          bool   `json:"bot"`
			JoinedAt     string `json:"joined_at"`
		} `json:"members"`
		NextCursor string `json:"next_cursor"`
	}
	url := qq.GroupMembersURL(groupOpenID)
	if cursor != "" {
		url += "?cursor=" + cursor
	}
	r := b.QqAPI("GET", url, nil, &out)
	members := make([]qqstore.MemberRec, 0, len(out.Members))
	for _, m := range out.Members {
		members = append(members, qqstore.MemberRec{
			GroupOpenID: groupOpenID, MemberOpenID: m.MemberOpenID, Username: m.Username,
			Role: m.MemberRole, Bot: m.Bot, FirstSeen: m.JoinedAt,
		})
	}
	return members, out.NextCursor, r
}

type QqGroupInfo struct {
	OpenID    string   `json:"group_openid"`
	Name      string   `json:"group_name"`
	Memo      string   `json:"group_finger_memo"`
	ClassText string   `json:"group_class_text"`
	Tags      []string `json:"group_tags"`
	MemberNum int      `json:"group_member_num"`
}

// QqGroupInfo 群基本信息（需白名单）。
func (b *Bot) QqGroupInfo(groupOpenID string) (QqGroupInfo, QqResp) {
	var out QqGroupInfo
	r := b.QqAPI("GET", qq.GroupInfoURL(groupOpenID), nil, &out)
	return out, r
}

// QqMuteSetting 查询群禁言状态（需机器人是群管理员）：返回当前被禁言成员。
func (b *Bot) QqMuteSetting(groupOpenID string) ([]qqstore.MuteRec, string, QqResp) {
	var out struct {
		GlobalRule struct {
			Mode string `json:"mode"`
		} `json:"global_rule"`
		Members []struct {
			MemberOpenID string `json:"member_openid"`
			MuteExpireAt string `json:"mute_expire_at"`
			Username     string `json:"username"`
		} `json:"members"`
	}
	r := b.QqAPI("GET", qq.GroupMuteURL(groupOpenID), nil, &out)
	list := make([]qqstore.MuteRec, 0, len(out.Members))
	for _, m := range out.Members {
		list = append(list, qqstore.MuteRec{
			GroupOpenID: groupOpenID, MemberOpenID: m.MemberOpenID,
			Username: m.Username, ExpireAt: m.MuteExpireAt, Status: "active",
		})
	}
	return list, out.GlobalRule.Mode, r
}

// QqBotState 机器人在群里的状态（是否被禁言等），顺带更新"机器人是否管理员"。
func (b *Bot) QqBotState(groupOpenID string, st *qqstore.Store) QqResp {
	var out struct {
		MemberRole string `json:"member_role"`
	}
	r := b.QqAPI("GET", qq.GroupBotStateURL(groupOpenID), nil, &out)
	if r.Ok() && st != nil && out.MemberRole != "" {
		isAdmin := out.MemberRole == "admin" || out.MemberRole == "owner"
		cur, _ := st.Group(groupOpenID)
		if cur.BotIsAdmin != isAdmin {
			_ = st.UpdateGroup(groupOpenID, nil, nil, nil, nil, nil, &isAdmin, nil)
		}
	}
	return r
}

// JoinReq 一条入群申请。
type JoinReq struct {
	JoinRequestID string `json:"join_request_id"`
	RiskTips      string `json:"risk_tips"`
	UnionOpenID   string `json:"union_openid"`
	MemberOpenID  string `json:"member_openid"`
	Username      string `json:"username"`
	ApplyAt       string `json:"apply_at"`
	ApplySource   string `json:"apply_source"`
	InvitedBy     string `json:"invited_by"`
	Bot           bool   `json:"bot"`
	VerifyInfo    struct {
		Method        string `json:"method"`
		VerifyMessage string `json:"verify_message"`
	} `json:"verify_info"`
}

// QqJoinRequests 拉取入群申请列表（需机器人是群管理员）。
func (b *Bot) QqJoinRequests(groupOpenID, cursor string) ([]JoinReq, string, QqResp) {
	var out struct {
		List       []JoinReq `json:"list"`
		NextCursor string    `json:"next_cursor"`
	}
	url := qq.GroupJoinListURL(groupOpenID)
	if cursor != "" {
		url += "?cursor=" + cursor
	}
	r := b.QqAPI("GET", url, nil, &out)
	return out.List, out.NextCursor, r
}

// QqApproveJoinRequest 审批入群申请（op=approve/decline；拒绝时可同时拉黑）。
func (b *Bot) QqApproveJoinRequest(groupOpenID, memberOpenID, joinRequestID string, approve bool, reason string, addToBlacklist bool) QqResp {
	body := map[string]interface{}{"op": map[bool]string{true: "approve", false: "decline"}[approve]}
	if joinRequestID != "" {
		body["join_request_id"] = joinRequestID
	}
	if !approve {
		if reason != "" {
			body["reject_reason"] = reason
		}
		if addToBlacklist {
			body["add_to_member_blacklist"] = true
		}
	}
	return b.QqAPI("POST", qq.GroupApproveJoinURL(groupOpenID, memberOpenID), body, nil)
}

// ---------------- 处置计划执行 ----------------

// ActionCtx 一次处置的上下文。
type ActionCtx struct {
	GroupOpenID  string
	MemberOpenID string
	Username     string
	MessageID    string
	Content      string
	Kind         string // word | command | manual | blacklist
	Hit          string
	Operator     string
}

// ActionPlan 一次要做的动作集合（回复/撤回/禁言/踢/拉黑）。
type ActionPlan struct {
	Reply       string
	Recall      bool
	RecallIDs   []string // 批量撤回（不填就用 ctx.MessageID 那条）
	MuteSeconds int
	Kick        bool
	Blacklist   bool
	Reason      string
}

const MaxMuteSeconds = 30 * 24 * 3600 // QQ 侧上限 30 天

// ApplyPlan 执行处置：先本地落库（立刻生效），再调 QQ，最后写审计。
// 返回人话结果列表（可直接拼成给管理员的回复）。
func (b *Bot) ApplyPlan(ctx ActionCtx, plan ActionPlan) []string {
	if b.st == nil {
		return nil
	}
	var results []string
	var actionNames []string
	okAll := true

	// 1) 撤回（支持一次撤多条：引用的、某人的最后一条、最近 N 条、机器人自己的）
	if plan.Recall || len(plan.RecallIDs) > 0 {
		actionNames = append(actionNames, "recall")
		ids := plan.RecallIDs
		if len(ids) == 0 && ctx.MessageID != "" {
			ids = []string{ctx.MessageID}
		}
		if len(ids) == 0 {
			okAll = false
			results = append(results, "撤回失败：没有可撤回的消息 ID")
		} else {
			done, fails := 0, []string{}
			for _, id := range ids {
				if strings.TrimSpace(id) == "" {
					continue
				}
				r := b.QqRecallMessage(ctx.GroupOpenID, id)
				if r.Ok() {
					done++
					continue
				}
				okAll = false
				fails = append(fails, r.String())
			}
			switch {
			case done > 0 && len(fails) == 0:
				if len(ids) == 1 {
					results = append(results, "已撤回消息")
				} else {
					results = append(results, fmt.Sprintf("已撤回 %d 条消息", done))
				}
			case done > 0:
				results = append(results, fmt.Sprintf("撤回 %d/%d 条，失败原因：%s", done, len(ids), strings.Join(dedupeStrings(fails), "；")))
			default:
				results = append(results, "撤回失败："+strings.Join(dedupeStrings(fails), "；"))
			}
		}
	}

	// 2) 禁言（先写本地记录，QQ 侧失败也能在管理页看到）
	if plan.MuteSeconds > 0 && ctx.MemberOpenID != "" {
		actionNames = append(actionNames, "mute")
		secs := plan.MuteSeconds
		if secs > MaxMuteSeconds {
			secs = MaxMuteSeconds
		}
		expire := time.Now().Add(time.Duration(secs) * time.Second)
		op := "add"
		if HasActiveMute(b.st, ctx.GroupOpenID, ctx.MemberOpenID) {
			op = "update"
		}
		rec := qqstore.MuteRec{
			GroupOpenID: ctx.GroupOpenID, MemberOpenID: ctx.MemberOpenID, Username: ctx.Username,
			ExpireAt: expire.Format(time.RFC3339), Reason: plan.Reason, Operator: ctx.Operator,
			Status: "active",
		}
		_, _ = b.st.AddMute(rec)
		r := b.QqSetMutes(ctx.GroupOpenID, []MuteOp{{
			Op: op, MemberOpenID: ctx.MemberOpenID, MuteExpireAt: expire.Format(time.RFC3339),
		}})
		if r.Ok() {
			results = append(results, fmt.Sprintf("已禁言 %s %s", DisplayName(ctx), HumanDuration(time.Duration(secs)*time.Second)))
		} else {
			okAll = false
			results = append(results, "禁言失败："+r.String()+"（已记入本地禁言表）")
		}
	}

	// 3) 踢出 / 拉黑
	if plan.Kick && ctx.MemberOpenID != "" {
		actionNames = append(actionNames, "kick")
		r := b.QqRemoveMembers(ctx.GroupOpenID, []string{ctx.MemberOpenID}, plan.Blacklist)
		if r.Ok() {
			if plan.Blacklist {
				results = append(results, "已踢出并拉黑 "+DisplayName(ctx))
			} else {
				results = append(results, "已踢出 "+DisplayName(ctx))
			}
		} else {
			okAll = false
			results = append(results, "踢出失败："+r.String())
		}
	} else if plan.Blacklist && ctx.MemberOpenID != "" {
		actionNames = append(actionNames, "blacklistQQ")
		// 目标还在群里时 QQ 侧会拒绝，属于正常现象；本地黑名单照样生效
		r := b.QqBlacklistOp(ctx.GroupOpenID, "add", []string{ctx.MemberOpenID})
		if !r.Ok() {
			results = append(results, "QQ 侧拉黑未生效（"+r.String()+"），本地黑名单已生效")
		}
	}

	if plan.Blacklist && ctx.MemberOpenID != "" {
		actionNames = append(actionNames, "blacklistLocal")
		_ = b.st.AddBlacklist(qqstore.BlackRec{
			GroupOpenID: ctx.GroupOpenID, MemberOpenID: ctx.MemberOpenID, Username: ctx.Username,
			Reason: plan.Reason, Operator: ctx.Operator, QQSynced: plan.Kick,
		})
		if !ContainsStr(results, "本地黑名单") && !plan.Kick {
			results = append(results, "已加入本地黑名单 "+DisplayName(ctx))
		}
	}

	// 4) 回复
	if strings.TrimSpace(plan.Reply) != "" {
		actionNames = append(actionNames, "reply")
		results = append(results, "已回复："+plan.Reply)
	}

	// 5) 审计
	result := "ok"
	if !okAll {
		result = "fail"
	}
	acts, _ := json.Marshal(actionNames)
	_ = b.st.AddAudit(qqstore.AuditRec{
		Kind: ctx.Kind, GroupOpenID: ctx.GroupOpenID, MemberOpenID: ctx.MemberOpenID,
		Username: ctx.Username, Content: Truncate(ctx.Content, 500), Hit: ctx.Hit,
		Actions: string(acts), Result: result, Detail: strings.Join(results, "；"),
	})

	return results
}

// MuteMember 手动禁言（给群内指令和网页后台共用）。
func (b *Bot) MuteMember(ctx ActionCtx, seconds int, reason string) []string {
	return b.ApplyPlan(ctx, ActionPlan{MuteSeconds: seconds, Reason: reason})
}

// UnmuteMember 手动解除禁言。
func (b *Bot) UnmuteMember(ctx ActionCtx) []string {
	if b.st == nil || ctx.MemberOpenID == "" {
		return nil
	}
	_ = b.st.EndMuteByMember(ctx.GroupOpenID, ctx.MemberOpenID)
	r := b.QqSetMutes(ctx.GroupOpenID, []MuteOp{{Op: "del", MemberOpenID: ctx.MemberOpenID, MuteExpireAt: ""}})
	var results []string
	if r.Ok() {
		results = append(results, "已解除禁言 "+DisplayName(ctx))
	} else {
		results = append(results, "解除禁言失败："+r.String())
	}
	acts, _ := json.Marshal([]string{"unmute"})
	_ = b.st.AddAudit(qqstore.AuditRec{
		Kind: "command", GroupOpenID: ctx.GroupOpenID, MemberOpenID: ctx.MemberOpenID,
		Username: ctx.Username, Hit: ctx.Hit, Actions: string(acts),
		Result: map[bool]string{true: "ok", false: "fail"}[r.Ok()], Detail: strings.Join(results, "；"),
	})
	return results
}

// ---------------- 小工具 ----------------

func HasActiveMute(st *qqstore.Store, groupOpenID, memberOpenID string) bool {
	if st == nil {
		return false
	}
	list, err := st.Mutes(true)
	if err != nil {
		return false
	}
	for _, m := range list {
		if m.GroupOpenID == groupOpenID && m.MemberOpenID == memberOpenID && m.Status == "active" {
			return true
		}
	}
	return false
}

func DisplayName(ctx ActionCtx) string {
	if ctx.Username != "" {
		return ctx.Username
	}
	if len(ctx.MemberOpenID) > 8 {
		return ctx.MemberOpenID[:8] + "…"
	}
	return ctx.MemberOpenID
}

func ContainsStr(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MuteDurationRe 抓 "1小时30分" 这种片段；没写单位时按分钟算。
var MuteDurationRe = regexp.MustCompile(`(\d+)\s*(秒钟|秒|分钟|分种|分|小时|时|天|日|周|星期|sec|min|hour|day|week|s|m|h|d|w)?`)

// ParseMuteDuration 解析中文禁言时长：10分钟 / 1小时30分 / 2天 / 1周 / 30天 / 600（纯数字=分钟）。
// 上限 30 天（QQ 侧限制），无法识别返回 false。
func ParseMuteDuration(s string) (time.Duration, bool) {
	str := strings.ToLower(strings.TrimSpace(s))
	if str == "" {
		return 0, false
	}
	total := time.Duration(0)
	found := false
	for _, m := range MuteDurationRe.FindAllStringSubmatch(str, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			continue
		}
		unit := time.Minute // 纯数字：按分钟
		switch m[2] {
		case "秒钟", "秒", "sec", "s":
			unit = time.Second
		case "分钟", "分种", "分", "min", "m":
			unit = time.Minute
		case "小时", "时", "hour", "h":
			unit = time.Hour
		case "天", "日", "day", "d":
			unit = 24 * time.Hour
		case "周", "星期", "week", "w":
			unit = 7 * 24 * time.Hour
		}
		total += time.Duration(n) * unit
		found = true
	}
	if !found || total <= 0 {
		return 0, false
	}
	if total > MaxMuteSeconds*time.Second {
		total = MaxMuteSeconds * time.Second
	}
	return total, true
}

// HumanDuration 把时长写成人话：1小时30分。
func HumanDuration(d time.Duration) string {
	secs := int64(d.Seconds())
	if secs <= 0 {
		return "0秒"
	}
	days := secs / 86400
	secs %= 86400
	hours := secs / 3600
	secs %= 3600
	mins := secs / 60
	secs %= 60
	var sb strings.Builder
	if days > 0 {
		fmt.Fprintf(&sb, "%d天", days)
	}
	if hours > 0 {
		fmt.Fprintf(&sb, "%d小时", hours)
	}
	if mins > 0 {
		fmt.Fprintf(&sb, "%d分", mins)
	}
	if secs > 0 && days == 0 && hours == 0 {
		fmt.Fprintf(&sb, "%d秒", secs)
	}
	if sb.Len() == 0 {
		return "0秒"
	}
	return sb.String()
}

// LogAction 记一条日志（同时方便排查）。
func LogAction(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// dedupeStrings 去掉重复的失败原因（批量撤回时同一种错误只报一次）。
func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

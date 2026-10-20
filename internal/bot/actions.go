package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"LianT/internal/store"
)

// ============================================================================
// 处置动作：撤回 / 禁言 / 解禁 / 踢出 / 封禁 / 拉黑
//
// 原则：先本地落库（网页后台立刻能看到），再调 Telegram；
// Telegram 侧失败不 panic，把原因写进日志/审计，并回一句人话给管理员。
//
// Telegram 权限对照：
//   * deleteMessage      —— 机器人是管理员才能删别人的消息；消息不能超过 48 小时
//   * restrictChatMember —— 禁言（restricted + can_send_messages=false）
//   * banChatMember      —— 封禁/踢出；踢出 = 封禁后立刻解封（允许重新加入）
// ============================================================================

type ActionCtx struct {
	ChatID    string
	UserID    string
	Username  string
	MessageID int64
	Content   string
	Kind      string // word | command | blacklist | manual
	Hit       string
	Operator  string
}

type ActionPlan struct {
	Reply       string
	Recall      bool
	RecallIDs   []int64
	MuteSeconds int
	Kick        bool
	Blacklist   bool
	Reason      string
}

const MaxMuteSeconds = 30 * 24 * 3600

// ApplyPlan 执行一套处置动作，返回给管理员看的结果列表。
func (b *Bot) ApplyPlan(ctx ActionCtx, plan ActionPlan) []string {
	var out []string
	var actions []string
	ok := true
	detail := ""

	if plan.Recall {
		ids := plan.RecallIDs
		if len(ids) == 0 && ctx.MessageID > 0 {
			ids = []int64{ctx.MessageID}
		}
		n, firstErr := b.RecallMessages(ctx.ChatID, ids)
		if n > 0 {
			actions = append(actions, "recall")
			out = append(out, fmt.Sprintf("已撤回 %d 条消息", n))
		}
		if firstErr != nil {
			ok = false
			detail = appendDetail(detail, "撤回失败: "+errText(firstErr))
			out = append(out, "撤回失败："+errText(firstErr))
		}
	}

	if plan.MuteSeconds > 0 {
		secs := plan.MuteSeconds
		if secs > MaxMuteSeconds {
			secs = MaxMuteSeconds
		}
		res := b.muteMember(ctx, time.Duration(secs)*time.Second, plan.Reason)
		actions = append(actions, "mute")
		out = append(out, res.Message)
		if !res.OK {
			ok = false
			detail = appendDetail(detail, res.Detail)
		}
	}

	switch {
	case plan.Kick && plan.Blacklist:
		res := b.banMember(ctx, true)
		actions = append(actions, "kick", "blacklist")
		out = append(out, res.Message)
		if !res.OK {
			ok = false
			detail = appendDetail(detail, res.Detail)
		}
	case plan.Kick:
		res := b.kickMember(ctx)
		actions = append(actions, "kick")
		out = append(out, res.Message)
		if !res.OK {
			ok = false
			detail = appendDetail(detail, res.Detail)
		}
	case plan.Blacklist:
		res := b.banMember(ctx, true)
		actions = append(actions, "blacklist")
		out = append(out, res.Message)
		if !res.OK {
			ok = false
			detail = appendDetail(detail, res.Detail)
		}
	}

	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 500), Hit: ctx.Hit,
		Actions: jsonActions(actions), Result: boolResult(ok), Detail: detail,
	})
	return out
}

type actionResult struct {
	OK      bool
	Message string
	Detail  string
}

// MuteMember 禁言并落库。
func (b *Bot) MuteMember(ctx ActionCtx, d time.Duration, reason string) []string {
	res := b.muteMember(ctx, d, reason)
	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 300), Hit: ctx.Hit,
		Actions: `["mute"]`, Result: boolResult(res.OK), Detail: res.Detail,
	})
	return []string{res.Message}
}

func (b *Bot) muteMember(ctx ActionCtx, d time.Duration, reason string) actionResult {
	if b.st == nil {
		return actionResult{Message: "数据库未就绪", Detail: "store nil"}
	}
	if d <= 0 {
		d = 10 * time.Minute
	}
	if d > MaxMuteSeconds*time.Second {
		d = MaxMuteSeconds * time.Second
	}
	until := time.Now().Add(d)
	expire := until.Format(time.RFC3339)
	if _, err := b.st.AddMute(store.MuteRec{
		ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		ExpireAt: expire, Reason: reason, Operator: ctx.Operator, Status: "active",
	}); err != nil {
		log.Printf("[!] 写禁言记录失败: %v", err)
	}

	c := b.clientOrNil()
	if c == nil {
		return actionResult{Message: "机器人未连接，已记录本地禁言但 Telegram 未执行", Detail: "not connected"}
	}
	if err := c.RestrictChatMember(ctx.ChatID, ctx.UserID, until, false); err != nil {
		return actionResult{
			Message: fmt.Sprintf("禁言 %s 失败：%s", displayTarget(ctx.UserID, ctx.Username), errText(err)),
			Detail:  errText(err),
		}
	}
	return actionResult{Message: fmt.Sprintf("已禁言 %s %s", displayTarget(ctx.UserID, ctx.Username), HumanDuration(d)), OK: true}
}

// UnmuteMember 解除禁言并结束本地记录。
func (b *Bot) UnmuteMember(ctx ActionCtx) []string {
	res := b.unmuteMember(ctx)
	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 300), Hit: ctx.Hit,
		Actions: `["unmute"]`, Result: boolResult(res.OK), Detail: res.Detail,
	})
	return []string{res.Message}
}

func (b *Bot) unmuteMember(ctx ActionCtx) actionResult {
	_ = b.st.EndMuteByMember(ctx.ChatID, ctx.UserID)
	c := b.clientOrNil()
	if c == nil {
		return actionResult{Message: "机器人未连接，已清除本地禁言记录", Detail: "not connected"}
	}
	if err := c.RestrictChatMember(ctx.ChatID, ctx.UserID, time.Time{}, true); err != nil {
		return actionResult{Message: "解除禁言失败：" + errText(err), Detail: errText(err)}
	}
	return actionResult{Message: "已解除 " + displayTarget(ctx.UserID, ctx.Username) + " 的禁言", OK: true}
}

// BanMember 封禁并踢出；addBlacklist=true 时同时写入本地黑名单。
func (b *Bot) BanMember(ctx ActionCtx, addBlacklist bool) []string {
	res := b.banMember(ctx, addBlacklist)
	acts := `["ban"]`
	if addBlacklist {
		acts = `["ban","blacklist"]`
	}
	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 300), Hit: ctx.Hit,
		Actions: acts, Result: boolResult(res.OK), Detail: res.Detail,
	})
	return []string{res.Message}
}

func (b *Bot) banMember(ctx ActionCtx, addBlacklist bool) actionResult {
	if addBlacklist && b.st != nil {
		if err := b.st.AddBlacklist(store.BlackRec{
			ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
			Reason: ctx.Hit, Operator: ctx.Operator, TGSynced: true,
		}); err != nil {
			log.Printf("[!] 写黑名单失败: %v", err)
		}
	}
	if strings.TrimSpace(ctx.ChatID) == "" {
		return actionResult{Message: "已加入全局黑名单（未指定群，未执行 Telegram 封禁）", OK: true}
	}
	c := b.clientOrNil()
	if c == nil {
		return actionResult{Message: "机器人未连接，已写入本地黑名单但 Telegram 未执行", Detail: "not connected"}
	}
	if err := c.BanChatMember(ctx.ChatID, ctx.UserID, true); err != nil {
		return actionResult{Message: "封禁失败：" + errText(err), Detail: errText(err)}
	}
	msg := "已封禁 " + displayTarget(ctx.UserID, ctx.Username)
	if addBlacklist {
		msg += " 并加入黑名单"
	}
	return actionResult{Message: msg, OK: true}
}

// KickMember 踢出但允许重新加入（Telegram = 封禁后立刻解封）。
func (b *Bot) KickMember(ctx ActionCtx) []string {
	res := b.kickMember(ctx)
	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 300), Hit: ctx.Hit,
		Actions: `["kick"]`, Result: boolResult(res.OK), Detail: res.Detail,
	})
	return []string{res.Message}
}

func (b *Bot) kickMember(ctx ActionCtx) actionResult {
	c := b.clientOrNil()
	if c == nil {
		return actionResult{Message: "机器人未连接", Detail: "not connected"}
	}
	if err := c.BanChatMember(ctx.ChatID, ctx.UserID, true); err != nil {
		return actionResult{Message: "踢出失败：" + errText(err), Detail: errText(err)}
	}
	// 立刻解封，让对方可以重新加入（这就是 Telegram 的"踢出"）。
	if err := c.UnbanChatMember(ctx.ChatID, ctx.UserID, true); err != nil {
		log.Printf("[!] 踢出后解封失败（对方可能仍处于封禁）: %v", err)
	}
	return actionResult{Message: "已踢出 " + displayTarget(ctx.UserID, ctx.Username), OK: true}
}

// UnbanMember 解除封禁（同时清本地黑名单由调用方决定）。
func (b *Bot) UnbanMember(ctx ActionCtx) []string {
	res := b.unbanMember(ctx)
	_ = b.st.AddAudit(store.AuditRec{
		Kind: ctx.Kind, ChatID: ctx.ChatID, UserID: ctx.UserID, Username: ctx.Username,
		Content: truncateLog(ctx.Content, 300), Hit: ctx.Hit,
		Actions: `["unban"]`, Result: boolResult(res.OK), Detail: res.Detail,
	})
	return []string{res.Message}
}

func (b *Bot) unbanMember(ctx ActionCtx) actionResult {
	if strings.TrimSpace(ctx.ChatID) == "" {
		return actionResult{Message: "已移出全局黑名单", OK: true}
	}
	c := b.clientOrNil()
	if c == nil {
		return actionResult{Message: "机器人未连接", Detail: "not connected"}
	}
	if err := c.UnbanChatMember(ctx.ChatID, ctx.UserID, false); err != nil {
		return actionResult{Message: "解封失败：" + errText(err), Detail: errText(err)}
	}
	return actionResult{Message: "已解封 " + displayTarget(ctx.UserID, ctx.Username), OK: true}
}

// RecallMessage 撤回单条。
func (b *Bot) RecallMessage(chatID string, messageID int64) error {
	c := b.clientOrNil()
	if c == nil {
		return fmt.Errorf("机器人未连接")
	}
	if messageID <= 0 {
		return fmt.Errorf("缺少消息 ID")
	}
	if err := c.DeleteMessage(chatID, messageID); err != nil {
		return err
	}
	b.MarkRecalled(chatID, messageID)
	return nil
}

// RecallMessages 批量撤回，返回成功条数和第一个错误。
func (b *Bot) RecallMessages(chatID string, ids []int64) (int, error) {
	var firstErr error
	n := 0
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if err := b.RecallMessage(chatID, id); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n++
	}
	return n, firstErr
}

// ---------------- helpers ----------------

func jsonActions(list []string) string {
	if len(list) == 0 {
		return "[]"
	}
	seen := map[string]bool{}
	var out []string
	for _, a := range list {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func boolResult(ok bool) string {
	if ok {
		return "ok"
	}
	return "fail"
}

func appendDetail(old, add string) string {
	if add == "" {
		return old
	}
	if old == "" {
		return add
	}
	return old + "；" + add
}

// IsNumericID 判断是不是纯数字 ID。
func IsNumericID(s string) bool {
	_, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return err == nil
}

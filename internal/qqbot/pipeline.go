package qqbot

import (
	"encoding/json"
	"fmt"
	"log"
	"LianT/internal/platform/qq"
	"LianT/internal/rules"
	qqstore "LianT/internal/platform/qq/store"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 消息处理管线（DB 驱动）
//
//   每条群消息：
//     1. 登记群 + 刷新群名册（昵称/角色）→ 供按昵称下指令
//     2. 记住这条消息（引用它撤回时要用消息 ID）
//     3. 群被停用 → 什么都不做
//     4. 本地黑名单 → 不回 + 尽量撤回
//     5. 管理员指令（管理员的指令优先，避免"加违禁词 广告"被自己的违禁词打中）
//     6. 违禁词 → 按配置处置（撤回/禁言/踢/拉黑/警告/审计）
//     7. 回复规则 → 命中就回，没命中就安静（default 为空时）
//
//   规则/违禁词都带内存缓存，靠 revision() 失效：
//   网页后台一改，下一条消息就按新规则走。
// ============================================================================

type RulesCache struct {
	mu     sync.RWMutex
	rev    int64
	group  rules.Config // 群里用：遵守 reply_all
	c2c    rules.Config // 单聊用：不受 reply_all 限制
	words  *rules.WordEngine
	prefix string
}

// Rules 取当前生效的规则（缓存 + 按版本失效）。
func (b *Bot) Rules() (rules.Config, rules.Config, *rules.WordEngine) {
	if b.st == nil {
		// 没有数据库（理论上不会发生）：退回内置默认规则
		cfg := rules.DefaultConfig()
		_ = cfg.Compile()
		return cfg, cfg, rules.BuildWordEngine(nil)
	}
	rev := b.st.Revision()
	b.rc.mu.RLock()
	if b.rc.rev == rev && b.rc.words != nil {
		g, c, w := b.rc.group, b.rc.c2c, b.rc.words
		b.rc.mu.RUnlock()
		return g, c, w
	}
	b.rc.mu.RUnlock()

	ruleRecs, err := b.st.Rules()
	if err != nil {
		log.Printf("[!] 读回复规则失败: %v", err)
	}
	wordRecs, err := b.st.Words()
	if err != nil {
		log.Printf("[!] 读违禁词失败: %v", err)
	}
	replyAll := strings.EqualFold(b.st.Setting("reply_all", "true"), "true")
	def := b.st.Setting("reply_default", "")

	group := rules.FromRecords(ruleRecs, replyAll, def, "")
	c2c := rules.FromRecords(ruleRecs, true, def, "c2c")
	words := rules.BuildWordEngine(wordRecs)

	b.rc.mu.Lock()
	b.rc.rev = rev
	b.rc.group, b.rc.c2c, b.rc.words = group, c2c, words
	b.rc.mu.Unlock()
	return group, c2c, words
}

// ---------------- 管线主体 ----------------

// HandleGroupMessage 处理一条群消息（@机器人 或 全量模式都会走到这里）。
func (b *Bot) HandleGroupMessage(d qq.MessageData, scope string) {
	if b.st != nil {
		// 1) 登记群 + 刷新名册（昵称/角色来自事件，不依赖白名单接口）
		_ = b.st.EnsureGroup(d.GroupOpenID, "")
		role := d.Author.MemberRole
		if err := b.st.TouchMember(d.GroupOpenID, d.Author.MemberOpenID, d.Author.Username, role, d.Author.Bot); err != nil {
			log.Printf("[!] 记录群成员失败: %v", err)
		}
	}
	// 2) 记住消息（引用撤回要用它反查消息 ID）
	b.RememberMessage(d)

	g, hasGroup := qqstore.GroupRec{}, false
	if b.st != nil {
		g, hasGroup = b.st.Group(d.GroupOpenID)
		if hasGroup && !g.Enabled {
			log.Printf("[skip] 群 %s 已停用，忽略消息", d.GroupOpenID)
			return
		}
	}

	// 3) 本地黑名单：不回 + 尽量撤回
	if b.st != nil && b.st.IsBlacklisted(d.GroupOpenID, d.Author.MemberOpenID) {
		results := b.ApplyPlan(ActionCtx{
			GroupOpenID: d.GroupOpenID, MemberOpenID: d.Author.MemberOpenID, Username: d.Author.Username,
			MessageID: d.ID, Content: d.Content, Kind: "blacklist", Hit: "本地黑名单", Operator: "system",
		}, ActionPlan{Recall: true, Reason: "黑名单成员发言"})
		log.Printf("[blacklist] %s(%s) 被拦截：%s", d.Author.Username, d.Author.MemberOpenID, strings.Join(results, "；"))
		return
	}

	// 4) 管理员指令优先（避免"加违禁词 广告"被自己的违禁词打中）
	if handled, reply := b.HandleGroupCommand(d); handled {
		log.Printf("[cmd] %s: %s", d.Author.Username, d.Content)
		if reply != "" {
			b.SendText(scope, d.GroupOpenID, "", d.ID, reply)
		}
		return
	}

	group, _, words := b.Rules()

	// 5) 违禁词处置
	if w, word, ok := words.Match(d.GroupOpenID, d.Content); ok {
		plan := ActionPlan{
			Reply:       w.Reply,
			Recall:      w.Recall,
			MuteSeconds: w.MuteSeconds,
			Kick:        w.Kick,
			Blacklist:   w.Blacklist,
			Reason:      "违禁词:" + word,
		}
		log.Printf("[words] 命中违禁词 %q（severity=%d）: %s", word, w.Severity, d.Content)
		results := b.ApplyPlan(ActionCtx{
			GroupOpenID: d.GroupOpenID, MemberOpenID: d.Author.MemberOpenID, Username: d.Author.Username,
			MessageID: d.ID, Content: d.Content, Kind: "word", Hit: "违禁词:" + word, Operator: "system",
		}, plan)
		if plan.Reply != "" {
			b.SendText(scope, d.GroupOpenID, "", d.ID, plan.Reply)
		}
		log.Printf("[words] 处置结果: %s", strings.Join(results, "；"))
		return
	}

	// 6) 回复规则
	res := group.Evaluate(d.Content, scope, d.GroupOpenID, d.Author.MemberOpenID)
	if !res.Send {
		log.Printf("[skip] %s 不回（%s）", scope, res.Rule)
		return
	}
	log.Printf("[回复] %s -> %q（命中 %s）", scope, res.Text, res.Rule)
	b.SendText(scope, d.GroupOpenID, "", d.ID, res.Text)
	if b.st != nil {
		_ = b.st.AddAudit(qqstore.AuditRec{
			Kind: "reply", GroupOpenID: d.GroupOpenID, MemberOpenID: d.Author.MemberOpenID,
			Username: d.Author.Username, Content: Truncate(d.Content, 500), Hit: res.Rule,
			Actions: `["reply"]`, Result: "ok", Detail: Truncate(res.Text, 200),
		})
	}
}

// HandleC2CMessage 处理单聊消息（没有群管理动作，只走规则 + 黑名单）。
func (b *Bot) HandleC2CMessage(d qq.MessageData) {
	b.RememberDirectMessage(d)
	if b.st != nil && b.st.IsBlacklisted("", d.Author.UserOpenID) {
		log.Printf("[blacklist] 单聊用户 %s 在黑名单，忽略", d.Author.UserOpenID)
		return
	}
	_, c2c, _ := b.Rules()
	res := c2c.Evaluate(d.Content, "私聊", "", d.Author.UserOpenID)
	if !res.Send {
		log.Printf("[skip] 私聊 不回（%s）", res.Rule)
		return
	}
	log.Printf("[回复] 私聊 -> %q（命中 %s）", res.Text, res.Rule)
	b.SendText("私聊", "", d.Author.UserOpenID, d.ID, res.Text)
}

// HandleGroupJoinRequest 入群申请：按设置自动审批，并记审计。
func (b *Bot) HandleGroupJoinRequest(raw json.RawMessage) {
	var req struct {
		GroupOpenID   string `json:"group_openid"`
		MemberOpenID  string `json:"member_openid"`
		Username      string `json:"username"`
		ApplySource   string `json:"apply_source"`
		JoinRequestID string `json:"join_request_id"`
		VerifyInfo    struct {
			VerifyMessage string `json:"verify_message"`
		} `json:"verify_info"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		log.Printf("[!] 解析入群申请失败: %v", err)
		return
	}
	log.Printf("[入群申请] 群=%s 用户=%s(%s) 来源=%s 验证信息=%q",
		req.GroupOpenID, req.Username, req.MemberOpenID, req.ApplySource, req.VerifyInfo.VerifyMessage)
	if b.st == nil {
		return
	}
	_ = b.st.EnsureGroup(req.GroupOpenID, "")
	_ = b.st.TouchMember(req.GroupOpenID, req.MemberOpenID, req.Username, "member", false)
	auto := strings.EqualFold(b.st.Setting("join_auto_approve", "false"), "true")
	result, detail := "待审批", "等待管理员在网页后台处理"
	if auto {
		if r := b.QqApproveJoinRequest(req.GroupOpenID, req.MemberOpenID, req.JoinRequestID, true, "", false); r.Ok() {
			result, detail = "ok", "已自动通过"
		} else {
			result, detail = "fail", "自动通过失败："+r.String()
		}
	}
	_ = b.st.AddAudit(qqstore.AuditRec{
		Kind: "join", GroupOpenID: req.GroupOpenID, MemberOpenID: req.MemberOpenID,
		Username: req.Username, Content: req.VerifyInfo.VerifyMessage, Hit: "入群申请",
		Actions: `["join"]`, Result: result, Detail: detail,
	})
}

// SendText 发送回复（独立协程，不阻塞网关读循环）。
func (b *Bot) SendText(scope, groupOpenID, userOpenID, msgID, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	go func() {
		defer RecoverLog()
		token, err := b.EnsureToken(false)
		if err != nil {
			log.Printf("[X] 取 token 失败: %v", err)
			return
		}
		msgSeq := int(time.Now().UnixNano()/1e6) % 100000
		if scope == "私聊" {
			if outID := qq.SendC2CMessage(token, userOpenID, text, msgID, msgSeq); outID != "" {
				b.RememberOutgoingIn(userOpenID, "private", "", outID, text)
			}
			return
		}
		if outID := qq.SendGroupMessage(token, groupOpenID, text, msgID, msgSeq); outID != "" {
			b.RememberOutgoing(groupOpenID, outID, text)
		}
	}()
}

// sendResultError 把 qq.SendResult 翻成人话。
func sendResultError(r qq.SendResult) error {
	message := strings.TrimSpace(r.Message)
	if message == "" {
		if r.Status > 0 {
			message = fmt.Sprintf("HTTP %d", r.Status)
		} else {
			message = "未知错误"
		}
	}
	if r.Code != 0 {
		return fmt.Errorf("QQ 接口错误 %d：%s", r.Code, message)
	}
	return fmt.Errorf("发送失败：%s", message)
}

// SendGroupTextNow 同步发一条群文本消息（网页后台需要确切的成功/失败）。
func (b *Bot) SendGroupTextNow(groupOpenID, content, replyTo string) (string, error) {
	if strings.TrimSpace(groupOpenID) == "" {
		return "", fmt.Errorf("缺少群 openid")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("消息内容为空")
	}
	token, err := b.EnsureToken(false)
	if err != nil {
		return "", err
	}
	msgSeq := int(time.Now().UnixNano()/1e6) % 100000
	result := qq.SendGroupText(token, groupOpenID, content, replyTo, msgSeq)
	if !result.Ok() {
		return "", sendResultError(result)
	}
	b.RememberOutgoing(groupOpenID, result.ID, content)
	return result.ID, nil
}

// SendGroupCardNow 同步发一张 Markdown 卡片（网页后台群发用）。
func (b *Bot) SendGroupCardNow(groupOpenID, markdown, replyTo string) (string, error) {
	if strings.TrimSpace(groupOpenID) == "" {
		return "", fmt.Errorf("缺少群 openid")
	}
	if strings.TrimSpace(markdown) == "" {
		return "", fmt.Errorf("消息内容为空")
	}
	token, err := b.EnsureToken(false)
	if err != nil {
		return "", err
	}
	msgSeq := int(time.Now().UnixNano()/1e6) % 100000
	result := qq.SendGroupMarkdown(token, groupOpenID, markdown, replyTo, msgSeq)
	if !result.Ok() {
		return "", sendResultError(result)
	}
	b.RememberOutgoing(groupOpenID, result.ID, markdown)
	return result.ID, nil
}

// DescribeCounts 给状态/日志用。
func (b *Bot) DescribeCounts() string {
	_, _, words := b.Rules()
	group, _, _ := b.Rules()
	return fmt.Sprintf("回复规则 %d 条、违禁词 %d 条", len(group.Rules), words.Count())
}

// SendC2CTextNow 同步发一条单聊文本（网页后台需要确切的成功/失败）。
func (b *Bot) SendC2CTextNow(userOpenID, content, replyTo string) (string, error) {
	if strings.TrimSpace(userOpenID) == "" {
		return "", fmt.Errorf("缺少用户 openid")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("消息内容为空")
	}
	token, err := b.EnsureToken(false)
	if err != nil {
		return "", err
	}
	msgSeq := int(time.Now().UnixNano()/1e6) % 100000
	result := qq.SendC2CText(token, userOpenID, content, replyTo, msgSeq)
	if !result.Ok() {
		return "", sendResultError(result)
	}
	b.RememberOutgoingIn(userOpenID, "private", "", result.ID, content)
	return result.ID, nil
}

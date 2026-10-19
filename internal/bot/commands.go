package bot

import (
	"fmt"
	"qbot/internal/qq"
	"qbot/internal/store"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// 群内中文指令（给网页后台配置的管理员用）
//
// 目标怎么定位（按优先级）：
//   1. @提及        —— 事件里的 mentions（官方字段，最准）
//   2. 引用消息      —— 引用某人的消息发指令，从 msg_elements(type=103).author 取
//   3. 文本里的 openid（32 位十六进制）
//   4. 昵称          —— 查机器人自建的群名册（每条消息都会刷新，不依赖白名单接口）
//   5. "我"          —— 指发送者自己
//
// 谁能用：网页后台配的管理员（全局/群级）∪ 群内 admin/owner（可在网页按群关掉）。
// 怎么触发：以动作词开头，或者短句里同时出现"动作词 + 能定位到的人"。
//           （普通成员的消息永远不会被当成指令，所以不会误伤正常聊天）
// ============================================================================

type CmdTarget struct {
	MemberOpenID string
	Username     string
	Source       string // mention | quote | openid | name | self
}

type CmdIntent struct {
	Action   string // mute | unmute | kick | kickban | unban | recall | addword | delword | addadmin | deladmin | status | help
	Duration time.Duration
	Target   CmdTarget
	Arg      string // 违禁词/备注等附加参数
	Keyword  string // 命中的动作词（回显用）
}

var (
	HexOpenIDRe = regexp.MustCompile(`\b[0-9A-Fa-f]{32}\b`)
	CmdPrefixes = []string{"请", "帮我", "麻烦", "给我"}
	// 把字结构（把/将/给/对 + 目标 + 动作）在 CommandishHead 里处理：
	// 它们不是客套话，而是"目标在前"的正常语序。
	CmdObjectMarkers = []string{"把", "将", "给", "对"}
)

// 动作词表：越靠前优先级越高（"踢黑"要排在"踢"前面）。
var CmdKeywords = []struct {
	Action  string
	Words   []string
	NeedArg bool // 是否需要目标
}{
	{"kickban", []string{"踢黑", "踢出拉黑", "踢了拉黑", "飞黑"}, true},
	{"unban", []string{"解黑", "移出黑名单", "取消拉黑", "解除拉黑"}, true},
	{"unmute", []string{"解禁", "解除禁言", "取消禁言", "撤销禁言", "解封"}, true},
	{"mute", []string{"禁言", "闭嘴", "静音", "封口", "口球"}, true},
	{"kick", []string{"踢出群", "踢出去", "踢", "移出群", "飞了", "t了", "T了", "送走"}, true},
	{"recall", []string{"撤回", "撤了", "删掉这条", "删除这条", "删了这条"}, false},
	{"addword", []string{"加违禁词", "添加违禁词", "新增违禁词"}, true},
	{"delword", []string{"删违禁词", "删除违禁词", "移除违禁词"}, true},
	{"addadmin", []string{"加管理", "添加管理", "加管理员"}, true},
	{"deladmin", []string{"删管理", "删除管理", "取消管理"}, true},
	{"status", []string{"状态", "群状态", "机器人状态"}, false},
	{"help", []string{"帮助", "help", "菜单", "指令"}, false},
}

// ChatterMarkers 出现这些词说明这是"一句话"，不是一条指令。
var ChatterMarkers = []string{
	// 口语代词/动词：句首出现这些基本就说明是在"说话"而不是在下指令
	"我", "你", "他", "她", "它", "咱", "俺", "想", "要", "看", "说", "问", "聊", "听", "知道", "记", "忘",
	"了", "过", "吗", "吧", "呢", "啊", "呀", "哦", "嘛", "都", "也", "还", "就", "才", "刚",
	"已经", "正在", "现在", "今天", "明天", "昨天", "怎么", "为什么", "是不是", "能不能", "可以吗",
	"天气", "时候", "觉得", "以为", "听说", "据说", "不要", "别", "顺便", "其实", "好像",
}

// MentionTokenRe 匹配开头的 @某人 / <@!openid> 这类提及。
var MentionTokenRe = regexp.MustCompile(`^\s*(?:<@!?[0-9A-Za-z]+>|@[^\s@，。！？,]{1,32})\s*`)

// StripLeadingMentions 去掉消息开头的 @提及（@机器人 或 @别人），可重复。
func StripLeadingMentions(s string) string {
	for {
		m := MentionTokenRe.FindString(s)
		if m == "" || strings.TrimSpace(m) == "" {
			return s
		}
		s = strings.TrimPrefix(s, m)
	}
}

// CommandishHead 判断动作词前面那一小段是不是"指令的一部分"（而不是闲聊）。
// 允许：空 / @某人 / 把|将|给|对 + 目标 / 我|自己 / 简短昵称；其余一律算闲聊。
func CommandishHead(head string, maxRunes int) bool {
	// @提及只是"条件词"：把它们删掉，剩下的整句仍然是指令就算数
	h := strings.TrimSpace(StripLeadingMentions(head))
	if h == "" {
		return true
	}
	// 把字结构：把/将/给/对 + 目标（"把张三踢了"、"把我禁言"、"给他禁言"）
	objectForm := false
	for _, m := range CmdObjectMarkers {
		if strings.HasPrefix(h, m) {
			h = strings.TrimSpace(strings.TrimPrefix(h, m))
			objectForm = true
			break
		}
	}
	h = strings.TrimSpace(StripLeadingMentions(h))
	if h == "" {
		return true
	}
	// 代词目标只有在把字结构里才算（"把他踢了"✓、"他踢了"✗）
	if objectForm && (h == "我" || h == "自己" || h == "他" || h == "她" || h == "它" || h == "你" || h == "大家") {
		return true
	}
	if h == "我" || h == "自己" {
		return true
	}
	limit := maxRunes
	if objectForm {
		limit = 12
	}
	if len([]rune(h)) > limit {
		return false
	}
	if strings.ContainsAny(h, "，。！？；：、,.!?;:…\n\r\t") {
		return false
	}
	for _, m := range ChatterMarkers {
		if strings.Contains(h, m) {
			return false
		}
	}
	return true
}

// NormalizeCommandText 去掉常见前缀/标点，方便匹配动作词。
func NormalizeCommandText(s string) string {
	s = strings.TrimSpace(s)
	s = StripLeadingMentions(s)
	s = strings.TrimLeft(s, "/＃#。，,、!！~ ")
	for {
		trimmed := false
		for _, p := range CmdPrefixes {
			if strings.HasPrefix(s, p) {
				s = strings.TrimSpace(strings.TrimPrefix(s, p))
				trimmed = true
				break
			}
		}
		if !trimmed {
			break
		}
	}
	return strings.TrimSpace(s)
}

// ParseGroupCommand 解析一条群消息里的管理指令（纯函数，方便测试）。
// nicknames 是本地名册（用于按昵称找人）。
func ParseGroupCommand(content string, mentions []qq.MentionedUser, quoted *qq.Author, nicknames []store.MemberRec, selfOpenID, selfName string) (CmdIntent, bool) {
	raw := NormalizeCommandText(content)
	if raw == "" {
		return CmdIntent{}, false
	}

	// 1) 找动作词
	var intent CmdIntent
	matchedAt := -1
	for _, kw := range CmdKeywords {
		for _, w := range kw.Words {
			idx := strings.Index(raw, w)
			if idx < 0 {
				continue
			}
			intent.Action = kw.Action
			intent.Keyword = w
			matchedAt = idx
			break
		}
		if intent.Action != "" {
			break
		}
	}
	if intent.Action == "" {
		return CmdIntent{}, false
	}
	// 只有"这条消息本身就是指令"才生效：动作词必须在句首，
	// 或前面只有 目标/把字结构（如"把张三踢了"）；像"我刚把他踢了"这种闲聊一律不算。
	needsTarget := false
	for _, kw := range CmdKeywords {
		if kw.Action == intent.Action {
			needsTarget = kw.NeedArg
		}
	}
	maxHead := 8
	if needsTarget {
		maxHead = 16
	}
	if matchedAt != 0 && !CommandishHead(raw[:matchedAt], maxHead) {
		// 动作词出现在一句话中间（"我刚把他踢了"、"今天天气不错，禁言他"）→ 不是指令
		return CmdIntent{}, false
	}

	// 2) 时长（禁言类；只按位置删，避免把 openid/违禁词里的数字删掉）
	body := raw
	if d, ds, de, ok := FindDuration(raw, intent.Action == "mute"); ok {
		intent.Duration = d
		body = RemoveSpan(raw, ds, de)
	}

	// 3) 目标
	// 3a) 句首显式写的名字优先（"@张三 把李四踢了" → 目标是李四）
	head := strings.TrimSpace(StripLeadingMentions(raw[:matchedAt]))
	head = strings.TrimSpace(strings.TrimPrefix(head, ""))
	for _, om := range CmdObjectMarkers {
		head = strings.TrimSpace(strings.TrimPrefix(head, om))
	}
	head = strings.TrimSpace(StripLeadingMentions(head))
	if head != "" {
		for _, n := range nicknames {
			if n.Username != "" && strings.Contains(head, n.Username) {
				intent.Target = CmdTarget{MemberOpenID: n.MemberOpenID, Username: n.Username, Source: "name-head"}
				break
			}
		}
	}
	if intent.Target.MemberOpenID == "" && len(mentions) > 0 {
		for _, m := range mentions {
			if m.Bot {
				continue
			}
			intent.Target = CmdTarget{MemberOpenID: m.MemberOpenID, Username: m.Username, Source: "mention"}
			break
		}
	}
	if intent.Target.MemberOpenID == "" && quoted != nil {
		if quoted.MemberOpenID != "" || quoted.UserOpenID != "" {
			intent.Target = CmdTarget{MemberOpenID: quoted.MemberOpenID, Username: quoted.Username, Source: "quote"}
		}
	}
	if intent.Target.MemberOpenID == "" {
		if m := HexOpenIDRe.FindString(raw); m != "" {
			intent.Target = CmdTarget{MemberOpenID: strings.ToUpper(m), Source: "openid"}
		}
	}
	if intent.Target.MemberOpenID == "" && len(nicknames) > 0 {
		for _, n := range nicknames {
			if n.Username == "" {
				continue
			}
			if strings.Contains(body, n.Username) {
				intent.Target = CmdTarget{MemberOpenID: n.MemberOpenID, Username: n.Username, Source: "name"}
				break
			}
		}
	}
	// "我/自己"只有在紧贴动作词时才算目标（"解禁我"✓、"把我禁言"✓、"我刚把他踢了"✗）
	if intent.Target.MemberOpenID == "" && selfOpenID != "" {
		kw := regexp.QuoteMeta(intent.Keyword)
		if regexp.MustCompile(kw+`\s*(?:我|自己)`).MatchString(raw) ||
			regexp.MustCompile(`(?:把|将)\s*(?:我|自己)\s*`+kw).MatchString(raw) {
			intent.Target = CmdTarget{MemberOpenID: selfOpenID, Username: selfName, Source: "self"}
		}
	}

	// 走到这里说明消息本身就是一条指令；目标定位不到时下面照常返回，
	// 由调用方回一句"怎么用"的提示（而不是静默失败）。
	intent.Arg = ExtractArg(intent.Action, body, intent.Keyword, intent.Target.Username)

	if intent.Action == "recall" {
		// 撤回：优先撤回引用的消息（message_id 由调用方从事件里取）
		return intent, true
	}
	return intent, true
}

// RecallMaxBatch 一次最多撤几条（太多了又慢又容易触发限频）。
const RecallMaxBatch = 20

// RecallCountRe 匹配"3条/3 个/3"这样的条数。
var RecallCountRe = regexp.MustCompile(`(\d+)\s*(?:条|个|次)?`)

// ParseRecallCount 解析要撤几条："撤回 3条"→3，"撤回全部"→上限，缺省→1。
func ParseRecallCount(content string) int {
	s := NormalizeCommandText(content)
	if strings.Contains(s, "全部") || strings.Contains(s, "所有") || strings.Contains(s, "清屏") {
		return RecallMaxBatch
	}
	for _, w := range []string{"撤回", "撤了", "删掉这条", "删除这条", "删了这条"} {
		if strings.HasPrefix(s, w) {
			s = strings.TrimSpace(strings.TrimPrefix(s, w))
			break
		}
	}
	if m := RecallCountRe.FindStringSubmatch(s); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			if n > RecallMaxBatch {
				n = RecallMaxBatch
			}
			return n
		}
	}
	return 1
}

// 时长必须带单位才算数（避免把"违禁词123"里的数字当时长）。
var MuteDurationUnitRe = regexp.MustCompile(`(\d+)\s*(?:秒钟|秒|分钟|分种|分|小时|时|天|日|周|星期|sec|min|hour|day|week|h|d|w|s|m)`)

// BareNumberRe 裸数字（只给禁言类指令用，按分钟）。\b 保证不会命中 openid 里的数字。
var BareNumberRe = regexp.MustCompile(`\b\d{1,4}\b`)

// FindDuration 找时长，并返回它在原串里的起止位置（精确删除，避免误删别处的数字）。
func FindDuration(s string, allowBare bool) (time.Duration, int, int, bool) {
	if loc := MuteDurationUnitRe.FindStringIndex(s); loc != nil {
		if d, ok := ParseMuteDuration(s[loc[0]:loc[1]]); ok {
			return d, loc[0], loc[1], true
		}
	}
	if allowBare {
		if loc := BareNumberRe.FindStringIndex(s); loc != nil {
			if d, ok := ParseMuteDuration(s[loc[0]:loc[1]]); ok {
				return d, loc[0], loc[1], true
			}
		}
	}
	return 0, 0, 0, false
}

// RemoveSpan 删掉 [start,end) 这段（换成空格）。
func RemoveSpan(s string, start, end int) string {
	if start < 0 || end > len(s) || start >= end {
		return s
	}
	return strings.TrimSpace(s[:start] + " " + s[end:])
}

// ExtractArg 取附加参数：违禁词、备注等（去掉动作词、目标、时长之后的剩余内容）。
func ExtractArg(action, body, keyword, TargetName string) string {
	out := body
	out = strings.Replace(out, keyword, " ", 1)
	if TargetName != "" {
		out = strings.Replace(out, TargetName, " ", 1)
	}
	out = strings.TrimSpace(out)
	out = strings.Trim(out, "，。,.、:：!！~ 　")
	if action == "addword" || action == "delword" {
		return out
	}
	return out
}

// ---------------- 执行 ----------------

// CommandContext 执行一条指令所需的上下文。
type CommandContext struct {
	GroupOpenID  string
	MemberOpenID string
	Username     string
	MsgID        string
	Content      string
	Role         string // 发送者的群内角色：member/admin/owner
	IsAdmin      bool   // 网页后台配的管理员
	AdminRole    string // super/admin
}

// CommandAllowed 判断发送者能不能用指令。
func (b *Bot) CommandAllowed(ctx CommandContext) bool {
	if b.st == nil {
		return false
	}
	if ctx.IsAdmin {
		return true
	}
	g, ok := b.st.Group(ctx.GroupOpenID)
	if !ok || !g.GroupAdminCanCmd {
		return false
	}
	return ctx.Role == "admin" || ctx.Role == "owner"
}

// HandleGroupCommand 尝试把消息当指令执行；返回 (是否已处理, 给发送者的回复)。
func (b *Bot) HandleGroupCommand(d qq.MessageData) (bool, string) {
	if b.st == nil {
		return false, ""
	}
	g, _ := b.st.Group(d.GroupOpenID)
	if !g.CommandsEnabled && g.OpenID != "" {
		return false, ""
	}
	adminRole, isAdmin := b.st.AdminRole(d.GroupOpenID, d.Author.MemberOpenID)
	cctx := CommandContext{
		GroupOpenID: d.GroupOpenID, MemberOpenID: d.Author.MemberOpenID, Username: d.Author.Username,
		MsgID: d.ID, Content: d.Content, Role: d.Author.MemberRole, IsAdmin: isAdmin, AdminRole: adminRole,
	}
	if !b.CommandAllowed(cctx) {
		return false, ""
	}

	var quoted *qq.Author
	if a, ok := d.QuotedAuthor(); ok {
		quoted = &a
	}
	nicknames, _ := b.st.Members(d.GroupOpenID, 200)
	intent, ok := ParseGroupCommand(d.Content, d.Mentions, quoted, nicknames, d.Author.MemberOpenID, d.Author.Username)
	if !ok {
		return false, ""
	}
	reply := b.RunGroupCommand(cctx, intent, d, quoted)
	return true, reply
}

// RunGroupCommand 执行解析出来的指令，返回人话结果。
func (b *Bot) RunGroupCommand(cctx CommandContext, intent CmdIntent, d qq.MessageData, quoted *qq.Author) string {
	actor := DisplayName(ActionCtx{Username: cctx.Username, MemberOpenID: cctx.MemberOpenID})
	switch intent.Action {
	case "help":
		return HelpText()

	case "status":
		lines := []string{"【群状态】"}
		if g, ok := b.st.Group(cctx.GroupOpenID); ok {
			lines = append(lines, fmt.Sprintf("机器人管理员：%v；指令开关：%v；回复全部：%v",
				g.BotIsAdmin, g.CommandsEnabled, g.ReplyAll))
		}
		Rules, _ := b.st.Rules()
		words, _ := b.st.Words()
		mutes, _ := b.st.Mutes(true)
		bl, _ := b.st.Blacklists(cctx.GroupOpenID)
		lines = append(lines, fmt.Sprintf("回复规则 %d 条；违禁词 %d 条；禁言中 %d 人；本群黑名单 %d 人",
			len(Rules), len(words), len(mutes), len(bl)))
		return strings.Join(lines, "\n")

	case "recall":
		// 撤回和消息类型无关（文字/图片/语音/视频/文件/卡片都能撤），
		// 唯一的限制是：机器人得"见过"那条消息（撤回接口要消息 ID，而引用事件只给 ref_msg_idx），
		// 以及群成员的消息要在 2 分钟内（机器人自己发的不受这条限制）。
		const recallWin = 2 * time.Minute
		raw := strings.ToLower(strings.TrimSpace(d.Content))
		refIdx := d.MessageScene.Get("ref_msg_idx")
		count := ParseRecallCount(d.Content)

		wantBot := strings.Contains(raw, "机器人") || strings.Contains(raw, "你自己") ||
			strings.HasSuffix(raw, "撤回你") || strings.Contains(raw, "撤回自己")
		wantAll := count > 1

		var ids []string
		desc := ""

		switch {
		// 1) 撤回机器人自己发的（"撤回机器人" / "撤回你"）
		case wantBot:
			if rm, ok := b.LookupBotRecent(cctx.GroupOpenID, 0); ok {
				ids = []string{rm.ID}
				desc = "机器人自己的消息"
			}

		// 2) 一次撤最近 N 条 / 全部
		case wantAll:
			for _, rm := range b.RecentList(cctx.GroupOpenID, count, recallWin) {
				if rm.ID == d.ID {
					continue // 别把这条指令本身算进去
				}
				ids = append(ids, rm.ID)
			}
			if len(ids) > 0 {
				desc = fmt.Sprintf("最近 %d 条", len(ids))
			}

		// 3) 引用的那条（任何类型）
		case refIdx != "":
			if rm, ok := b.LookupRecent(cctx.GroupOpenID, refIdx, ""); ok {
				ids = []string{rm.ID}
				desc = "引用的那条" + KindLabel(rm.Kind)
			}

		// 4) 指定的人最近一条（"撤回 @张三" / "撤回 张三"）
		case intent.Target.MemberOpenID != "":
			if rm, ok := b.LookupRecentByMember(cctx.GroupOpenID, intent.Target.MemberOpenID); ok {
				ids = []string{rm.ID}
				desc = fmt.Sprintf("%s 的最后一条%s", rm.Username, KindLabel(rm.Kind))
			}

		// 5) 光说"撤回"：撤上一条（不算这条指令）
		default:
			for _, rm := range b.RecentList(cctx.GroupOpenID, 2, recallWin) {
				if rm.ID == d.ID {
					continue
				}
				ids = append(ids, rm.ID)
				desc = "上一条" + KindLabel(rm.Kind)
				break
			}
		}

		if len(ids) == 0 {
			hint := "找不到要撤回的消息：引用它再说「撤回」，或者「撤回 @某人」「撤回 3条」。"
			if refIdx != "" || intent.Target.MemberOpenID != "" {
				hint += "（这条我没收到过，或已超过 2 分钟；群成员的消息 QQ 只允许撤回 2 分钟内的）"
			}
			return hint
		}

		results := b.ApplyPlan(ActionCtx{
			GroupOpenID: cctx.GroupOpenID, MemberOpenID: cctx.MemberOpenID, Username: cctx.Username,
			MessageID: ids[0], Kind: "command", Hit: "撤回", Operator: actor,
		}, ActionPlan{Recall: true, RecallIDs: ids, Reason: "管理员撤回(" + desc + ")"})
		return JoinResults(results, "撤回指令已执行")

	case "mute":
		if intent.Target.MemberOpenID == "" {
			return "要禁言谁？@他一下，或者引用他的消息，也可以直接写昵称"
		}
		secs := int(intent.Duration.Seconds())
		if secs <= 0 {
			secs = 600 // 默认 10 分钟
		}
		results := b.MuteMember(ActionCtx{
			GroupOpenID: cctx.GroupOpenID, MemberOpenID: intent.Target.MemberOpenID,
			Username: intent.Target.Username, Kind: "command", Hit: "禁言", Operator: actor,
		}, secs, "管理员禁言")
		return JoinResults(results, fmt.Sprintf("已禁言 %s %s", TargetName(intent), HumanDuration(time.Duration(secs)*time.Second)))

	case "unmute":
		if intent.Target.MemberOpenID == "" {
			return "要解禁谁？@他一下或写昵称"
		}
		results := b.UnmuteMember(ActionCtx{
			GroupOpenID: cctx.GroupOpenID, MemberOpenID: intent.Target.MemberOpenID,
			Username: intent.Target.Username, Kind: "command", Hit: "解禁", Operator: actor,
		})
		return JoinResults(results, "已解除 "+TargetName(intent)+" 的禁言")

	case "kick", "kickban":
		if intent.Target.MemberOpenID == "" {
			return "要踢谁？@他一下或写昵称"
		}
		ban := intent.Action == "kickban"
		results := b.ApplyPlan(ActionCtx{
			GroupOpenID: cctx.GroupOpenID, MemberOpenID: intent.Target.MemberOpenID,
			Username: intent.Target.Username, Kind: "command", Hit: intent.Keyword, Operator: actor,
		}, ActionPlan{Kick: true, Blacklist: ban, Reason: "管理员" + intent.Keyword})
		return JoinResults(results, "已执行 "+intent.Keyword)

	case "unban":
		if intent.Target.MemberOpenID == "" {
			return "要解除谁的黑名单？@他一下或写昵称"
		}
		_ = b.st.RemoveBlacklist(cctx.GroupOpenID, intent.Target.MemberOpenID)
		r := b.QqBlacklistOp(cctx.GroupOpenID, "del", []string{intent.Target.MemberOpenID})
		if r.Ok() {
			return "已移出黑名单（本地+QQ）"
		}
		return "已移出本地黑名单；QQ 侧未同步：" + r.String()

	case "addword":
		if strings.TrimSpace(intent.Arg) == "" {
			return "用法：加违禁词 词  [时长]（默认命中自动撤回）"
		}
		secs := int(intent.Duration.Seconds())
		id, err := b.st.SaveWord(store.WordRec{
			Word: intent.Arg, Match: "contains", Recall: true, MuteSeconds: secs,
			Severity: 100, Enabled: true, Note: "由 " + actor + " 在群里添加",
		})
		if err != nil {
			return "添加失败：" + err.Error()
		}
		msg := fmt.Sprintf("已添加违禁词「%s」(id=%d)，命中自动撤回", intent.Arg, id)
		if secs > 0 {
			msg += "并禁言 " + HumanDuration(time.Duration(secs)*time.Second)
		}
		return msg

	case "delword":
		if strings.TrimSpace(intent.Arg) == "" {
			return "用法：删违禁词 词"
		}
		n := 0
		words, _ := b.st.Words()
		for _, w := range words {
			if w.Word == intent.Arg {
				if err := b.st.DeleteWord(w.ID); err == nil {
					n++
				}
			}
		}
		if n == 0 {
			return "没找到违禁词「" + intent.Arg + "」"
		}
		return fmt.Sprintf("已删除违禁词「%s」(%d 条)", intent.Arg, n)

	case "addadmin", "deladmin":
		if cctx.AdminRole != "super" && !cctx.IsAdmin {
			return "只有管理员能改管理员"
		}
		if intent.Target.MemberOpenID == "" {
			return "用法：加管理 @某人"
		}
		if intent.Action == "addadmin" {
			_, err := b.st.AddAdmin(store.AdminRec{
				MemberOpenID: intent.Target.MemberOpenID, Username: intent.Target.Username,
				Role: "admin", GroupOpenID: cctx.GroupOpenID, Enabled: true,
			})
			if err != nil {
				return "添加失败：" + err.Error()
			}
			return "已把 " + TargetName(intent) + " 设为本群管理员"
		}
		admins, _ := b.st.Admins()
		for _, a := range admins {
			if a.MemberOpenID == intent.Target.MemberOpenID && a.GroupOpenID == cctx.GroupOpenID {
				_ = b.st.DeleteAdmin(a.ID)
			}
		}
		return "已取消 " + TargetName(intent) + " 的本群管理员"
	}
	return ""
}

func TargetName(intent CmdIntent) string {
	if intent.Target.Username != "" {
		return intent.Target.Username
	}
	if len(intent.Target.MemberOpenID) > 8 {
		return intent.Target.MemberOpenID[:8] + "…"
	}
	return intent.Target.MemberOpenID
}

func JoinResults(results []string, fallback string) string {
	if len(results) == 0 {
		return fallback
	}
	return strings.Join(results, "；")
}

func HelpText() string {
	return strings.Join([]string{
		"【可用指令】@机器人 或直接发（需管理员身份）",
		"禁言 @某人 10分钟     —— 不写时长默认 10 分钟，最长 30 天",
		"解禁 @某人",
		"踢 @某人 / 踢黑 @某人  —— 踢黑＝踢出并拉黑（QQ 侧需白名单权限）",
		"解黑 @某人",
		"引用某人的消息再发「撤回」",
		"加违禁词 词 [时长] / 删违禁词 词",
		"加管理 @某人 / 删管理 @某人",
		"状态 / 帮助",
	}, "\n")
}

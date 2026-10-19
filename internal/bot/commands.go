package bot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"LianT/internal/store"
	"LianT/internal/tg"
)

// ============================================================================
// 群内指令（中文 + /斜杠）
//
// 目标怎么定位（按优先级）：
//   1. 回复某人的消息（Telegram 的 reply，最准）
//   2. @提及（text_mention 带用户对象；普通 @username 查本地名册）
//   3. 文本里的数字用户 ID
//   4. 本地名册里的昵称
//   5. "我/自己"
//
// 谁能用：网页后台配的管理员（全局/群级）∪ Telegram 群管理员（可在网页按群关掉）。
// 触发：整条消息本身就是指令（动作词开头，或"把张三踢了"这种目标前置），
//      普通聊天里出现"禁言"不会被当成指令。
// ============================================================================

type Mention struct {
	UserID   string
	Username string
	Bot      bool
}

type Quote struct {
	UserID   string
	Username string
}

type CmdTarget struct {
	UserID   string
	Username string
	Source   string // reply | mention | id | name | self
}

type CmdIntent struct {
	Action   string // mute | unmute | kick | kickban | unban | recall | addword | delword | addadmin | deladmin | status | help
	Duration time.Duration
	Target   CmdTarget
	Arg      string
	Keyword  string
}

var (
	// Telegram 用户 ID 是 5-15 位数字（且不会和"10分钟"这类时长混淆）。
	UserIDRe         = regexp.MustCompile(`\b\d{5,15}\b`)
	CmdPrefixes      = []string{"请", "帮我", "麻烦", "给我"}
	CmdObjectMarkers = []string{"把", "将", "给", "对"}
)

// 动作词表：越靠前优先级越高（"踢黑"要排在"踢"前面）。
var CmdKeywords = []struct {
	Action  string
	Words   []string
	NeedArg bool
}{
	{"kickban", []string{"踢黑", "踢出拉黑", "踢了拉黑", "飞黑", "封禁"}, true},
	{"unban", []string{"解黑", "移出黑名单", "取消拉黑", "解除拉黑", "解封"}, true},
	{"unmute", []string{"解禁", "解除禁言", "取消禁言", "撤销禁言"}, true},
	{"mute", []string{"禁言", "闭嘴", "静音", "封口", "口球"}, true},
	{"kick", []string{"踢出群", "踢出去", "踢", "移出群", "飞了", "送走"}, true},
	{"recall", []string{"撤回", "撤了", "删掉这条", "删除这条", "删了这条"}, false},
	{"addword", []string{"加违禁词", "添加违禁词", "新增违禁词"}, true},
	{"delword", []string{"删违禁词", "删除违禁词", "移除违禁词"}, true},
	{"addadmin", []string{"加管理", "添加管理", "加管理员"}, true},
	{"deladmin", []string{"删管理", "删除管理", "取消管理"}, true},
	{"status", []string{"状态", "群状态", "机器人状态"}, false},
	{"help", []string{"帮助", "help", "菜单", "指令"}, false},
}

// ChatterMarkers 句首出现这些词说明是在"说话"而不是下指令。
var ChatterMarkers = []string{
	"我", "你", "他", "她", "它", "咱", "俺", "想", "要", "看", "说", "问", "聊", "听", "知道", "记", "忘",
	"了", "过", "吗", "吧", "呢", "啊", "呀", "哦", "嘛", "都", "也", "还", "就", "才", "刚",
	"已经", "正在", "现在", "今天", "明天", "昨天", "怎么", "为什么", "是不是", "能不能", "可以吗",
	"天气", "时候", "觉得", "以为", "听说", "据说", "不要", "别", "顺便", "其实", "好像",
}

// MentionTokenRe 匹配开头的 @某人 这类提及。
var MentionTokenRe = regexp.MustCompile(`^\s*@[^\s@，。！？,]{1,32}\s*`)

func StripLeadingMentions(s string) string {
	for {
		m := MentionTokenRe.FindString(s)
		if strings.TrimSpace(m) == "" {
			return s
		}
		s = strings.TrimPrefix(s, m)
	}
}

// CommandishHead 判断动作词前面那一小段是不是指令的一部分（而不是闲聊）。
func CommandishHead(head string, maxRunes int) bool {
	h := strings.TrimSpace(StripLeadingMentions(head))
	if h == "" {
		return true
	}
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
func ParseGroupCommand(content string, mentions []Mention, quoted *Quote, nicknames []store.MemberRec, selfID, selfName string) (CmdIntent, bool) {
	raw := NormalizeCommandText(content)
	if raw == "" {
		return CmdIntent{}, false
	}

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
		return CmdIntent{}, false
	}

	body := raw
	if d, ds, de, ok := FindDuration(raw, intent.Action == "mute"); ok {
		intent.Duration = d
		body = RemoveSpan(raw, ds, de)
	}

	head := strings.TrimSpace(StripLeadingMentions(raw[:matchedAt]))
	for _, om := range CmdObjectMarkers {
		head = strings.TrimSpace(strings.TrimPrefix(head, om))
	}
	head = strings.TrimSpace(StripLeadingMentions(head))
	if head != "" {
		for _, n := range nicknames {
			if n.Username != "" && strings.Contains(head, n.Username) {
				intent.Target = CmdTarget{UserID: n.UserID, Username: n.Username, Source: "name-head"}
				break
			}
		}
	}
	if intent.Target.UserID == "" && len(mentions) > 0 {
		for _, m := range mentions {
			if m.Bot {
				continue
			}
			if m.UserID == "" && m.Username == "" {
				continue
			}
			intent.Target = CmdTarget{UserID: m.UserID, Username: m.Username, Source: "mention"}
			break
		}
	}
	if intent.Target.UserID == "" && quoted != nil && quoted.UserID != "" {
		intent.Target = CmdTarget{UserID: quoted.UserID, Username: quoted.Username, Source: "reply"}
	}
	if intent.Target.UserID == "" {
		if m := UserIDRe.FindString(body); m != "" {
			intent.Target = CmdTarget{UserID: m, Source: "id"}
		}
	}
	if intent.Target.UserID == "" && len(nicknames) > 0 {
		for _, n := range nicknames {
			if n.Username == "" {
				continue
			}
			if strings.Contains(body, n.Username) {
				intent.Target = CmdTarget{UserID: n.UserID, Username: n.Username, Source: "name"}
				break
			}
		}
	}
	if intent.Target.UserID == "" && selfID != "" {
		kw := regexp.QuoteMeta(intent.Keyword)
		if regexp.MustCompile(kw+`\s*(?:我|自己)`).MatchString(raw) ||
			regexp.MustCompile(`(?:把|将)\s*(?:我|自己)\s*`+kw).MatchString(raw) {
			intent.Target = CmdTarget{UserID: selfID, Username: selfName, Source: "self"}
		}
	}

	intent.Arg = ExtractArg(intent.Action, body, intent.Keyword, intent.Target.Username)
	return intent, true
}

const RecallMaxBatch = 20

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

var MuteDurationUnitRe = regexp.MustCompile(`(\d+)\s*(?:秒钟|秒|分钟|分种|分|小时|时|天|日|周|星期|sec|min|hour|day|week|h|d|w|s|m)`)

var BareNumberRe = regexp.MustCompile(`\b\d{1,4}\b`)

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

func ParseMuteDuration(s string) (time.Duration, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	re := regexp.MustCompile(`^(\d+)\s*(秒钟|秒|分钟|分种|分|小时|时|天|日|周|星期|sec|min|hour|day|week|h|d|w|s|m)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		// 裸数字：按分钟
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Minute, true
		}
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	unit := m[2]
	var d time.Duration
	switch unit {
	case "秒", "秒钟", "s", "sec":
		d = time.Duration(n) * time.Second
	case "分", "分钟", "分种", "m", "min":
		d = time.Duration(n) * time.Minute
	case "时", "小时", "h", "hour":
		d = time.Duration(n) * time.Hour
	case "天", "日", "d", "day":
		d = time.Duration(n) * 24 * time.Hour
	case "周", "星期", "w", "week":
		d = time.Duration(n) * 7 * 24 * time.Hour
	default:
		return 0, false
	}
	if d <= 0 {
		return 0, false
	}
	if d > 30*24*time.Hour {
		d = 30 * 24 * time.Hour
	}
	return d, true
}

func RemoveSpan(s string, start, end int) string {
	if start < 0 || end > len(s) || start >= end {
		return s
	}
	return strings.TrimSpace(s[:start] + " " + s[end:])
}

func ExtractArg(action, body, keyword, targetName string) string {
	out := body
	out = strings.Replace(out, keyword, " ", 1)
	if targetName != "" {
		out = strings.Replace(out, targetName, " ", 1)
	}
	out = strings.TrimSpace(out)
	out = strings.Trim(out, "，。,.、:：!！~ 　")
	if action == "addword" || action == "delword" {
		return out
	}
	return out
}

// ---------------- 上下文与执行 ----------------

type CommandContext struct {
	ChatID    string
	UserID    string
	Username  string
	MessageID int64
	Content   string
	Role      string // member/admin/owner
	IsAdmin   bool
	AdminRole string
}

func (b *Bot) CommandAllowed(ctx CommandContext) bool {
	if b.st == nil {
		return false
	}
	if ctx.IsAdmin {
		return true
	}
	g, ok := b.st.Group(ctx.ChatID)
	if !ok || !g.GroupAdminCanCmd {
		return false
	}
	return ctx.Role == "admin" || ctx.Role == "owner"
}

// handleCommand 尝试把消息当指令执行；返回 (是否已处理, 回复)。
func (b *Bot) handleCommand(c *tg.Client, m *tg.Message, chatID, userID, role string) (bool, string) {
	if b.st == nil {
		return false, ""
	}
	g, _ := b.st.Group(chatID)
	if g.ChatID != "" && !g.CommandsEnabled {
		return false, ""
	}
	adminRole, isAdmin := b.st.AdminRole(chatID, userID)
	cctx := CommandContext{
		ChatID: chatID, UserID: userID, Username: m.From.Display(),
		MessageID: m.MessageID, Content: m.Content(), Role: role,
		IsAdmin: isAdmin, AdminRole: adminRole,
	}

	text := strings.TrimSpace(m.Content())
	if strings.HasPrefix(text, "/") {
		return b.handleSlashCommand(c, m, cctx)
	}

	if !b.CommandAllowed(cctx) {
		return false, ""
	}
	quoted := quoteFrom(m)
	mentions, _ := b.mentionsFrom(m, chatID)
	nicknames, _ := b.st.Members(chatID, 200)
	intent, ok := ParseGroupCommand(text, mentions, quoted, nicknames, userID, m.From.Display())
	if !ok {
		return false, ""
	}
	reply := b.RunGroupCommand(cctx, intent, m, quoted)
	return true, reply
}

// handleSlashCommand 处理 /mute /ban 等 Telegram 风格命令。
func (b *Bot) handleSlashCommand(c *tg.Client, m *tg.Message, cctx CommandContext) (bool, string) {
	cmd, rest := splitSlash(m.Content())
	cmd = strings.ToLower(cmd)
	switch cmd {
	case "start":
		return true, "LianT 在线 ✅\n把机器人设为群管理员后可用：/help 查看指令。"
	case "help":
		return true, HelpText()
	case "id":
		return true, fmt.Sprintf("会话 ID：%s\n你的 ID：%s", cctx.ChatID, cctx.UserID)
	}
	if !b.CommandAllowed(cctx) {
		return true, ""
	}

	quoted := quoteFrom(m)
	mentions, _ := b.mentionsFrom(m, cctx.ChatID)
	target, targetName := resolveTarget(rest, mentions, quoted, cctx)
	switch cmd {
	case "status":
		return true, b.groupStatus(cctx.ChatID)
	case "mute", "禁言":
		if target == "" {
			return true, "用法：回复某人的消息发 /mute 10m，或 /mute 10m @某人"
		}
		d, _ := commandDuration(rest, true)
		if d <= 0 {
			d = 10 * time.Minute
		}
		return true, JoinResults(b.MuteMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: target, Username: targetName,
			Kind: "command", Hit: "禁言", Operator: cctx.Username,
		}, d, "管理员禁言"), "已禁言 "+displayTarget(target, targetName)+" "+HumanDuration(d))
	case "unmute", "解禁":
		if target == "" {
			return true, "用法：回复某人的消息发 /unmute"
		}
		return true, JoinResults(b.UnmuteMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: target, Username: targetName,
			Kind: "command", Hit: "解禁", Operator: cctx.Username,
		}), "已解除 "+displayTarget(target, targetName)+" 的禁言")
	case "ban", "kickban":
		if target == "" {
			return true, "用法：回复某人的消息发 /ban（封禁并踢出）"
		}
		return true, JoinResults(b.BanMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: target, Username: targetName,
			Kind: "command", Hit: "封禁", Operator: cctx.Username,
		}, true), "已封禁 "+displayTarget(target, targetName))
	case "kick":
		if target == "" {
			return true, "用法：回复某人的消息发 /kick（踢出但允许重新加入）"
		}
		return true, JoinResults(b.KickMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: target, Username: targetName,
			Kind: "command", Hit: "踢出", Operator: cctx.Username,
		}), "已踢出 "+displayTarget(target, targetName))
	case "unban":
		if target == "" {
			return true, "用法：/unban 用户ID"
		}
		_ = b.st.RemoveBlacklist(cctx.ChatID, target)
		return true, JoinResults(b.UnbanMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: target, Username: targetName,
			Kind: "command", Hit: "解封", Operator: cctx.Username,
		}), "已解除 "+displayTarget(target, targetName)+" 的封禁")
	case "del", "delete", "recall":
		return true, b.recallByReply(cctx, m, rest)
	}
	return false, ""
}

// RunGroupCommand 执行中文指令。
func (b *Bot) RunGroupCommand(cctx CommandContext, intent CmdIntent, m *tg.Message, quoted *Quote) string {
	actor := cctx.Username
	switch intent.Action {
	case "help":
		return HelpText()
	case "status":
		return b.groupStatus(cctx.ChatID)
	case "recall":
		return b.recallByReply(cctx, m, "")
	case "mute":
		if intent.Target.UserID == "" {
			return "要禁言谁？回复他的消息，或 @他一下（对方说过话才能按昵称找到）"
		}
		d := intent.Duration
		if d <= 0 {
			d = 10 * time.Minute
		}
		results := b.MuteMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: intent.Target.UserID, Username: intent.Target.Username,
			Kind: "command", Hit: "禁言", Operator: actor,
		}, d, "管理员禁言")
		return JoinResults(results, fmt.Sprintf("已禁言 %s %s", TargetName(intent), HumanDuration(d)))
	case "unmute":
		if intent.Target.UserID == "" {
			return "要解禁谁？回复他的消息或写昵称"
		}
		return JoinResults(b.UnmuteMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: intent.Target.UserID, Username: intent.Target.Username,
			Kind: "command", Hit: "解禁", Operator: actor,
		}), "已解除 "+TargetName(intent)+" 的禁言")
	case "kick", "kickban":
		if intent.Target.UserID == "" {
			return "要踢谁？回复他的消息或 @他"
		}
		ctx := ActionCtx{
			ChatID: cctx.ChatID, UserID: intent.Target.UserID, Username: intent.Target.Username,
			Kind: "command", Hit: intent.Keyword, Operator: actor,
		}
		if intent.Action == "kickban" {
			return JoinResults(b.BanMember(ctx, true), "已封禁 "+TargetName(intent))
		}
		return JoinResults(b.KickMember(ctx), "已踢出 "+TargetName(intent))
	case "unban":
		if intent.Target.UserID == "" {
			return "要解除谁？回复他的消息或写用户 ID"
		}
		_ = b.st.RemoveBlacklist(cctx.ChatID, intent.Target.UserID)
		return JoinResults(b.UnbanMember(ActionCtx{
			ChatID: cctx.ChatID, UserID: intent.Target.UserID, Username: intent.Target.Username,
			Kind: "command", Hit: "解封", Operator: actor,
		}), "已解除 "+TargetName(intent))
	case "addword":
		if strings.TrimSpace(intent.Arg) == "" {
			return "用法：加违禁词 词 [时长]（默认命中自动撤回）"
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
		if intent.Target.UserID == "" {
			return "用法：加管理 @某人（或回复他的消息）"
		}
		if intent.Action == "addadmin" {
			_, err := b.st.AddAdmin(store.AdminRec{
				UserID: intent.Target.UserID, Username: intent.Target.Username,
				Role: "admin", ChatID: cctx.ChatID, Enabled: true,
			})
			if err != nil {
				return "添加失败：" + err.Error()
			}
			return "已把 " + TargetName(intent) + " 设为本群管理员"
		}
		admins, _ := b.st.Admins()
		for _, a := range admins {
			if a.UserID == intent.Target.UserID && a.ChatID == cctx.ChatID {
				_ = b.st.DeleteAdmin(a.ID)
			}
		}
		return "已取消 " + TargetName(intent) + " 的本群管理员"
	}
	return ""
}

// recallByReply 处理"撤回"：优先回复的那条，其次目标最后一条，最后上一条。
func (b *Bot) recallByReply(cctx CommandContext, m *tg.Message, rest string) string {
	count := ParseRecallCount(m.Content())
	raw := strings.ToLower(m.Content())
	wantBot := strings.Contains(raw, "机器人") || strings.Contains(raw, "你")

	var ids []int64
	desc := ""
	switch {
	case wantBot:
		if rm, ok := b.LookupBotRecent(cctx.ChatID, 0); ok {
			ids, desc = []int64{rm.MessageID}, "机器人自己的消息"
		}
	case count > 1:
		for _, rm := range b.RecentList(cctx.ChatID, count, 48*time.Hour) {
			if rm.MessageID == m.MessageID {
				continue
			}
			ids = append(ids, rm.MessageID)
		}
		if len(ids) > 0 {
			desc = fmt.Sprintf("最近 %d 条", len(ids))
		}
	case m.ReplyToMessage != nil:
		ids, desc = []int64{m.ReplyToMessage.MessageID}, "回复的那条"+KindLabel(Classify(m.ReplyToMessage))
	case rest != "":
		// /del <message_id>
		if n, err := strconv.ParseInt(strings.Fields(rest)[0], 10, 64); err == nil && n > 0 {
			ids, desc = []int64{n}, "指定的消息"
		}
	default:
		for _, rm := range b.RecentList(cctx.ChatID, 2, 48*time.Hour) {
			if rm.MessageID == m.MessageID {
				continue
			}
			ids, desc = []int64{rm.MessageID}, "上一条"+KindLabel(rm.Kind)
			break
		}
	}
	if len(ids) == 0 {
		return "找不到要撤回的消息：回复它再说「撤回」，或「撤回 3条」。"
	}
	results := b.ApplyPlan(ActionCtx{
		ChatID: cctx.ChatID, UserID: cctx.UserID, Username: cctx.Username,
		MessageID: ids[0], Kind: "command", Hit: "撤回", Operator: cctx.Username,
	}, ActionPlan{Recall: true, RecallIDs: ids, Reason: "管理员撤回(" + desc + ")"})
	return JoinResults(results, "撤回指令已执行")
}

func (b *Bot) groupStatus(chatID string) string {
	lines := []string{"【群状态】"}
	if g, ok := b.st.Group(chatID); ok {
		lines = append(lines, fmt.Sprintf("机器人管理员：%v；指令开关：%v；回复全部：%v",
			g.BotIsAdmin, g.CommandsEnabled, g.ReplyAll))
	}
	ruleList, _ := b.st.Rules()
	words, _ := b.st.Words()
	mutes, _ := b.st.Mutes(true)
	bl, _ := b.st.Blacklists(chatID)
	lines = append(lines, fmt.Sprintf("回复规则 %d 条；违禁词 %d 条；禁言中 %d 人；本群黑名单 %d 人",
		len(ruleList), len(words), len(mutes), len(bl)))
	return strings.Join(lines, "\n")
}

// ---------------- 解析辅助 ----------------

func splitSlash(text string) (string, string) {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "/")
	parts := strings.SplitN(s, " ", 2)
	cmd := parts[0]
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	rest := ""
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}
	return cmd, rest
}

func quoteFrom(m *tg.Message) *Quote {
	if m == nil || m.ReplyToMessage == nil || m.ReplyToMessage.From == nil {
		return nil
	}
	u := m.ReplyToMessage.From
	return &Quote{UserID: chatIDStr(u.ID), Username: u.Display()}
}

func (b *Bot) mentionsFrom(m *tg.Message, chatID string) ([]Mention, string) {
	var out []Mention
	var firstUsername string
	for _, e := range m.Entities {
		switch e.Type {
		case "text_mention":
			if e.User == nil {
				continue
			}
			out = append(out, Mention{UserID: chatIDStr(e.User.ID), Username: e.User.Display(), Bot: e.User.IsBot})
			if firstUsername == "" {
				firstUsername = e.User.Username
			}
		case "mention":
			name := strings.TrimPrefix(m.EntityText(e), "@")
			if name == "" {
				continue
			}
			uid := ""
			if b.st != nil {
				if list, err := b.st.FindMembersByName(chatID, name); err == nil {
					for _, mm := range list {
						if strings.EqualFold(mm.Username, name) || strings.EqualFold(strings.TrimPrefix(mm.Username, "@"), name) {
							uid = mm.UserID
							break
						}
					}
				}
			}
			out = append(out, Mention{UserID: uid, Username: name, Bot: strings.EqualFold(name, b.me.Username)})
			if firstUsername == "" {
				firstUsername = name
			}
		}
	}
	return out, firstUsername
}

func resolveTarget(rest string, mentions []Mention, quoted *Quote, cctx CommandContext) (string, string) {
	if quoted != nil && quoted.UserID != "" {
		return quoted.UserID, quoted.Username
	}
	for _, m := range mentions {
		if m.Bot || m.UserID == "" {
			continue
		}
		return m.UserID, m.Username
	}
	if m := UserIDRe.FindString(rest); m != "" {
		return m, ""
	}
	return "", ""
}

func commandDuration(s string, allowBare bool) (time.Duration, bool) {
	d, _, _, ok := FindDuration(s, allowBare)
	return d, ok
}

func displayTarget(id, name string) string {
	if name != "" {
		return name
	}
	if len(id) > 8 {
		return id[:6] + "…"
	}
	return id
}

func TargetName(intent CmdIntent) string {
	return displayTarget(intent.Target.UserID, intent.Target.Username)
}

func JoinResults(results []string, fallback string) string {
	if len(results) == 0 {
		return fallback
	}
	return strings.Join(results, "；")
}

// HumanDuration 把秒数写成中文。
func HumanDuration(d time.Duration) string {
	if d <= 0 {
		return "永久"
	}
	sec := int(d.Seconds())
	switch {
	case sec%86400 == 0:
		return fmt.Sprintf("%d 天", sec/86400)
	case sec%3600 == 0:
		return fmt.Sprintf("%d 小时", sec/3600)
	case sec%60 == 0:
		return fmt.Sprintf("%d 分钟", sec/60)
	default:
		return fmt.Sprintf("%d 秒", sec)
	}
}

func HelpText() string {
	return strings.Join([]string{
		"【LianT 可用指令】",
		"回复某人的消息，然后：",
		"  /mute 10m     禁言（不写时长默认 10 分钟，最长 30 天）",
		"  /unmute       解除禁言",
		"  /kick         踢出（允许重新加入）",
		"  /ban          封禁并踢出",
		"  /unban        解除封禁",
		"  /del          撤回这条消息（48 小时内）",
		"中文也可以：禁言 @某人 10分钟 / 解禁 @某人 / 踢 @某人 / 撤回 / 状态 / 帮助",
		"改违禁词：加违禁词 词 [时长] / 删违禁词 词",
		"管理员：加管理 @某人 / 删管理 @某人",
		"（以上管理指令仅群管理员或后台配置的管理员可用）",
	}, "\n")
}

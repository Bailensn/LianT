package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"LianT/internal/cfg"
	"LianT/internal/rules"
)

// ============================================================================
// 规则缓存 + 试跑
//
// 规则/违禁词存在 SQLite，热路径靠 store.Revision() 失效：
// 网页后台一改，下一条消息就按新规则走（不用重启）。
// ============================================================================

type RulesCache struct {
	mu     sync.RWMutex
	rev    int64
	group  rules.Config
	c2c    rules.Config
	words  *rules.WordEngine
	prefix string
}

// Rules 取当前生效的规则（缓存 + 按版本失效）。
func (b *Bot) Rules() (rules.Config, rules.Config, *rules.WordEngine) {
	if b.st == nil {
		c := rules.DefaultConfig()
		_ = c.Compile()
		return c, c, rules.BuildWordEngine(nil)
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

// InvalidateRules 让规则/违禁词缓存下次访问时重载。
func (b *Bot) InvalidateRules() {
	b.rc.mu.Lock()
	b.rc.rev = -1
	b.rc.mu.Unlock()
}

// ReplyInfo 状态快照（规则/违禁词/数据库）。
func (b *Bot) ReplyInfo() map[string]string {
	group, _, words := b.Rules()
	dbFile := cfg.DBPath
	if b.cfg.DBFile != "" {
		dbFile = b.cfg.DBFile
	}
	d := map[string]string{
		"db_file":     dbFile,
		"reply_rules": strconv.Itoa(len(group.Rules)),
		"reply_all":   strconv.FormatBool(group.IsReplyAll()),
		"reply_ver":   strconv.FormatInt(b.st.Revision(), 10),
		"words":       strconv.Itoa(words.Count()),
	}
	if b.st != nil {
		d["reply_loaded"] = time.Now().Format(time.RFC3339)
		d["reply_file"] = dbFile
	}
	return d
}

// TestReply 干跑一条消息，看规则会怎么回（CLI: LianT reply "xx"）。
func (b *Bot) TestReply(content, scope string) rules.Result {
	if scope == "" {
		scope = "群@(测试)"
	}
	group, c2c, _ := b.Rules()
	if scope == "私聊" {
		return c2c.Evaluate(content, scope, "", "测试用户")
	}
	return group.Evaluate(content, scope, "测试群", "测试用户")
}

// DescribeCounts 给状态/日志用。
func (b *Bot) DescribeCounts() string {
	group, _, words := b.Rules()
	return fmt.Sprintf("回复规则 %d 条、违禁词 %d 条", len(group.Rules), words.Count())
}

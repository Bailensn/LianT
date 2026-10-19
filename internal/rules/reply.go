package rules

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"LianT/internal/store"
)

// ============================================================================
// 回复规则引擎 + 违禁词匹配（纯逻辑，不碰文件/网络，方便单测）
//
// 规则来源：SQLite（见 internal/store），由 bot 组装成 Config 后调用 Evaluate。
//
// 匹配方式：exact(默认) | prefix | contains | regex | any，按顺序第一条命中即停。
// 模板占位符：
//   {text}  去空格后的整条消息      {raw}   原消息（不去空格）
//   {arg}   prefix = 去掉前缀后的内容；regex = 第 1 个捕获组；其它 = 整条消息
//   {1}~{9} regex 捕获组
//   {scope} 场景(群@/群消息/私聊)   {group} 群ID   {user} 用户ID   {time} 当前时间
// ============================================================================

// Rule 一条规则（对应数据库 reply_rules 里的一行）。
type Rule struct {
	Match   string  `json:"match,omitempty"`
	Pattern string  `json:"pattern"`
	Reply   *string `json:"reply,omitempty"`
	Always  bool    `json:"always,omitempty"`
	Fold    bool    `json:"fold,omitempty"`
	Note    string  `json:"note,omitempty"`

	re *regexp.Regexp // regex 规则的编译结果
}

// Config 一份完整的规则配置。
type Config struct {
	ReplyAll *bool   `json:"reply_all,omitempty"`
	Default  *string `json:"default,omitempty"`
	Rules    []Rule  `json:"rules"`
}

// Result 一次判定的结果。
type Result struct {
	Text   string // 要发的内容
	Send   bool   // 是否真的发
	Rule   string // 命中描述（日志/试跑用）
	Silent bool   // 明确不回（命中静默规则）
}

func StrPtr(s string) *string { return &s }
func BoolPtr(b bool) *bool    { return &b }

// DefaultConfig 新库播种用的默认模板：只保留最标准的一条 /ping。
// default 故意留空：没命中就不回，避免刷屏。
func DefaultConfig() Config {
	return Config{
		ReplyAll: BoolPtr(true),
		Default:  StrPtr(""),
		Rules: []Rule{
			{Match: "exact", Pattern: "/ping", Reply: StrPtr("pong! 机器人在线 ✅"), Always: true},
		},
	}
}

// Compile 校验并预编译规则。
func (c *Config) Compile() error {
	for i := range c.Rules {
		r := &c.Rules[i]
		m := strings.ToLower(strings.TrimSpace(r.Match))
		if m == "" {
			m = "exact"
		}
		r.Match = m
		switch m {
		case "exact", "prefix", "contains", "any", "*":
		case "regex":
			pat := r.Pattern
			if r.Fold {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return fmt.Errorf("第 %d 条规则(%q)的 regex 不合法: %v", i+1, r.Pattern, err)
			}
			r.re = re
		default:
			return fmt.Errorf("第 %d 条规则的 match 不认识: %q（可用 exact/prefix/contains/regex/any）", i+1, r.Match)
		}
	}
	return nil
}

func (c *Config) IsReplyAll() bool {
	if c.ReplyAll == nil {
		return true
	}
	return *c.ReplyAll
}

func (c *Config) DefaultText() string {
	if c.Default == nil {
		return ""
	}
	return *c.Default
}

// FromRecords 把数据库记录转成规则配置（scope 为空=任意场景；"c2c"=单聊专用）。
func FromRecords(recs []store.RuleRec, replyAll bool, def, scope string) Config {
	cfg := Config{ReplyAll: BoolPtr(replyAll), Default: StrPtr(def)}
	for _, r := range recs {
		if !r.Enabled {
			continue
		}
		if r.Scope != "" && r.Scope != scope {
			continue
		}
		reply := r.Reply
		match := strings.ToLower(strings.TrimSpace(r.Match))
		if match == "" {
			match = "exact"
		}
		rr := Rule{
			Match:   match,
			Pattern: r.Pattern,
			Reply:   &reply,
			Always:  r.Always,
			Fold:    r.Fold,
			Note:    r.Note,
		}
		if match == "regex" {
			pat := r.Pattern
			if r.Fold {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				log.Printf("[!] 规则 #%d 正则不合法，已跳过: %v", r.ID, err)
				continue
			}
			rr.re = re
		}
		cfg.Rules = append(cfg.Rules, rr)
	}
	return cfg
}

// Evaluate 按这份配置判定一条消息该怎么回。
func (c *Config) Evaluate(content, scope, group, user string) Result {
	cfg := *c
	text := strings.TrimSpace(content)
	gate := cfg.IsReplyAll() || strings.HasPrefix(text, "/")

	for i := range cfg.Rules {
		r := &cfg.Rules[i]
		m := r.match(text)
		if !m.OK {
			continue
		}
		if !gate && !r.Always {
			continue
		}
		desc := fmt.Sprintf("rule#%d %s(%q)", i+1, r.Match, r.Pattern)
		if r.Note != "" {
			desc += " " + r.Note
		}
		if r.Reply == nil || *r.Reply == "" {
			return Result{Send: false, Rule: desc, Silent: true}
		}
		return Result{
			Send: true,
			Text: renderTemplate(*r.Reply, text, content, scope, group, user, m.Arg, m.Groups),
			Rule: desc,
		}
	}

	if !gate {
		return Result{Send: false, Rule: "reply_all=false 且不是 / 指令"}
	}
	def := cfg.DefaultText()
	if strings.TrimSpace(def) == "" {
		return Result{Send: false, Rule: "default 为空=不回", Silent: true}
	}
	return Result{Send: true, Text: renderTemplate(def, text, content, scope, group, user, "", nil), Rule: "default"}
}

// ruleMatch 一次规则匹配的结果：是否命中、参数、捕获组。
type ruleMatch struct {
	OK     bool
	Arg    string
	Groups []string
}

func (r *Rule) match(text string) ruleMatch {
	pat := r.Pattern
	subject := text
	if r.Fold && r.Match != "regex" {
		pat = strings.ToLower(pat)
		subject = strings.ToLower(subject)
	}
	switch r.Match {
	case "prefix":
		if !strings.HasPrefix(subject, pat) {
			return ruleMatch{}
		}
		arg := text
		if len(r.Pattern) <= len(text) {
			arg = text[len(r.Pattern):]
		}
		return ruleMatch{OK: true, Arg: strings.TrimSpace(arg)}

	case "contains":
		if !strings.Contains(subject, pat) {
			return ruleMatch{}
		}
		return ruleMatch{OK: true, Arg: text}

	case "regex":
		if r.re == nil {
			return ruleMatch{}
		}
		g := r.re.FindStringSubmatch(text)
		if g == nil {
			return ruleMatch{}
		}
		arg := g[0]
		if len(g) > 1 {
			arg = g[1]
		}
		return ruleMatch{OK: true, Arg: arg, Groups: g}

	case "any", "*":
		return ruleMatch{OK: true, Arg: text}

	default: // exact
		if subject != pat {
			return ruleMatch{}
		}
		return ruleMatch{OK: true, Arg: text}
	}
}

func renderTemplate(tpl, text, raw, scope, group, user, arg string, groups []string) string {
	pairs := []string{
		"{text}", text,
		"{raw}", raw,
		"{arg}", arg,
		"{scope}", scope,
		"{group}", group,
		"{user}", user,
		"{time}", time.Now().Format("2006-01-02 15:04:05"),
	}
	for i := 1; i <= 9; i++ {
		v := ""
		if i < len(groups) {
			v = groups[i]
		}
		pairs = append(pairs, fmt.Sprintf("{%d}", i), v)
	}
	return strings.NewReplacer(pairs...).Replace(tpl)
}

// ---------------- 违禁词 ----------------

type wordRuleX struct {
	rec store.WordRec
	rr  Rule
}

// WordEngine 违禁词匹配器（多条命中时取 severity 最小的一条）。
type WordEngine struct {
	rules []wordRuleX
}

// BuildWordEngine 用数据库记录构造违禁词匹配器。
func BuildWordEngine(recs []store.WordRec) *WordEngine {
	e := &WordEngine{}
	for _, r := range recs {
		if !r.Enabled || strings.TrimSpace(r.Word) == "" {
			continue
		}
		match := strings.ToLower(strings.TrimSpace(r.Match))
		if match == "" {
			match = "contains"
		}
		rr := Rule{Match: match, Pattern: r.Word, Fold: r.Fold}
		if match == "regex" {
			pat := r.Word
			if r.Fold {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				log.Printf("[!] 违禁词 #%d 正则不合法，已跳过: %v", r.ID, err)
				continue
			}
			rr.re = re
		}
		e.rules = append(e.rules, wordRuleX{rec: r, rr: rr})
	}
	return e
}

// Match 返回命中的违禁词（scope 为空=所有群）。
func (e *WordEngine) Match(chatID, text string) (store.WordRec, string, bool) {
	if e == nil {
		return store.WordRec{}, "", false
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return store.WordRec{}, "", false
	}
	var best *store.WordRec
	for _, r := range e.rules {
		if r.rec.Scope != "" && r.rec.Scope != chatID {
			continue
		}
		if !r.rr.match(t).OK {
			continue
		}
		if best == nil || r.rec.Severity < best.Severity {
			rec := r.rec
			best = &rec
		}
	}
	if best == nil {
		return store.WordRec{}, "", false
	}
	return *best, best.Word, true
}

// Count 规则条数（状态展示用）。
func (e *WordEngine) Count() int {
	if e == nil {
		return 0
	}
	return len(e.rules)
}

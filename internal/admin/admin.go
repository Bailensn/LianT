package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"qbot/internal/bot"
	"qbot/internal/cfg"
	"qbot/internal/store"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// 网页管理端
//
//   * 只认 token（QBOT_ADMIN_TOKEN > .env 里的同名项 > 数据库里自动生成的）
//   * 默认只绑回环 127.0.0.1:28732（QBOT_ADMIN_ADDR 可改）；绑到非回环又没配 token 会拒绝启动
//   * 支持 Nginx / Caddy 反代：
//       - 反代剥掉前缀（location /qbot/ { proxy_pass http://127.0.0.1:28732/; }）
//       - 或不剥前缀（配 QBOT_ADMIN_PREFIX=/qbot，或让反代传 X-Forwarded-Prefix）
//   * 所有写操作都会写审计（谁、什么时候、对谁做了什么）
// ============================================================================

type Server struct {
	b      *bot.Bot
	st     *store.Store
	token  string
	prefix string
	addr   string
	srv    *http.Server
}

// chatGroupView 是网页「群消息」左侧群列表的展示结构（不暴露多余字段）。
type chatGroupView struct {
	OpenID     string `json:"openid"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	BotIsAdmin bool   `json:"bot_is_admin"`
	Note       string `json:"note"`
	LastAt     string `json:"last_at,omitempty"`
	LastText   string `json:"last_text,omitempty"`
}

func ListenAddr() string { return cfg.EnvOr("QBOT_ADMIN_ADDR", "127.0.0.1:28732") }

// NormalizePrefix 把前缀规整成 "" 或 "/xxx"（不带结尾斜杠）。
func NormalizePrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

// TokenValue 解析管理端 token：环境变量 > .env > 数据库（没有就生成一个存库）。
// TokenSource 返回管理端 token 以及它的来源（环境变量 > .env > 数据库；都没有就现场生成并落库）。
// st 传 nil 表示调用方没有数据库连接（比如主进程），此时只能拿到 env/.env 里的值。
func TokenSource(st *store.Store) (string, string) {
	if v := strings.TrimSpace(cfg.EnvOr("QBOT_ADMIN_TOKEN", "")); v != "" {
		return v, "环境变量 QBOT_ADMIN_TOKEN"
	}
	if v, ok := cfg.EnvFile().Lookup("QBOT_ADMIN_TOKEN"); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v), ".env 文件"
	}
	if st == nil {
		return "", "读不到（调用方没有数据库连接）"
	}
	if v := st.Setting("admin_token", ""); v != "" {
		return v, "数据库 settings.admin_token"
	}
	tok := RandomToken(24)
	if err := st.SetSetting("admin_token", tok); err != nil {
		log.Printf("[!] 生成管理端 token 失败: %v", err)
		return "", "生成失败"
	}
	return tok, "新生成（已写入数据库）"
}

// TokenValue 只要 token 本身（来源见 TokenSource）。
func TokenValue(st *store.Store) string {
	tok, _ := TokenSource(st)
	return tok
}

func RandomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("qbot-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// Start 启动管理端；返回 nil 表示没启动（并已打日志说明原因）。
func Start(b *bot.Bot, st *store.Store) *Server {
	if strings.EqualFold(cfg.EnvOr("QBOT_ADMIN_ENABLE", "true"), "false") {
		log.Printf("[i] 网页管理端已按 QBOT_ADMIN_ENABLE=false 关闭")
		return nil
	}
	tok, tokSrc := TokenSource(st)
	if tok == "" {
		log.Printf("[!] 管理端没有可用 token（来源: %s）", tokSrc)
	} else {
		log.Printf("[i] 网页管理端 token 来源: %s", tokSrc)
	}
	a := &Server{
		b:      b,
		st:     st,
		token:  tok,
		prefix: NormalizePrefix(cfg.EnvOr("QBOT_ADMIN_PREFIX", "")),
		addr:   ListenAddr(),
	}
	host := a.addr
	if i := strings.LastIndex(a.addr, ":"); i > 0 {
		host = a.addr[:i]
	}
	loopback := cfg.IsLoopbackHost(host) || host == ""
	if a.token == "" && !loopback {
		log.Printf("[X] 管理端绑在 %s（非回环）但没有 token，出于安全拒绝启动。请设 QBOT_ADMIN_TOKEN 或改回 127.0.0.1", a.addr)
		return nil
	}
	ln, err := net.Listen("tcp", a.addr)
	if err != nil {
		log.Printf("[X] 管理端启动失败 %s: %v", a.addr, err)
		return nil
	}
	a.srv = &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := a.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[X] 管理端退出: %v", err)
		}
	}()
	base := "http://" + a.addr + a.prefix
	log.Printf("[OK] 网页管理端: %s/  (token: %s)", base, MaskToken(a.token))
	if !loopback {
		log.Printf("[!] 管理端绑在非回环地址 %s：请务必用反代加 HTTPS/额外鉴权", a.addr)
	}
	return a
}

func MaskToken(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:4] + "…" + t[len(t)-4:]
}

func (a *Server) Close() {
	if a != nil && a.srv != nil {
		_ = a.srv.Close()
	}
}

// ---------------- 路由 ----------------

func (a *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := a.Path(r)
		if p == "/" || p == "/index.html" {
			a.ServeUI(w, r)
			return
		}
		if !strings.HasPrefix(p, "/api/") {
			http.NotFound(w, r)
			return
		}
		if !a.Authorized(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="qbot"`)
			WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{"ok": false, "error": "未授权：请带上 token（Authorization: Bearer <token> 或 ?token=）"})
			return
		}
		a.Api(w, r, strings.TrimPrefix(p, "/api/"))
	})
}

// Path 计算出去掉反代前缀后的路径（兼容剥前缀/不剥前缀两种反代写法）。
func (a *Server) Path(r *http.Request) string {
	p := r.URL.Path
	if fp := r.Header.Get("X-Forwarded-Prefix"); fp != "" {
		p = strings.TrimPrefix(p, strings.TrimRight(fp, "/"))
	}
	if a.prefix != "" {
		p = strings.TrimPrefix(p, a.prefix)
	}
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// BasePath 给前端用：算出"页面挂在哪"，让 fetch 用相对路径也能工作。
func (a *Server) BasePath(r *http.Request) string {
	if fp := r.Header.Get("X-Forwarded-Prefix"); fp != "" {
		return NormalizePrefix(fp)
	}
	if a.prefix != "" {
		return a.prefix
	}
	p := r.URL.Path
	if p == "/" || p == "" {
		return ""
	}
	return strings.TrimSuffix(p, "/")
}

func (a *Server) Authorized(r *http.Request) bool {
	if a.token == "" {
		return true // 只有回环且没配 token 的情况会走到这
	}
	given := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		given = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if given == "" {
		given = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if given == "" {
		if c, err := r.Cookie("qbot_admin_token"); err == nil {
			given = c.Value
		}
	}
	if given == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(a.token)) == 1
}

func WriteJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ApiOK(w http.ResponseWriter, v interface{}) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "data": v})
}

func ApiErr(w http.ResponseWriter, code int, msg string) {
	WriteJSON(w, code, map[string]interface{}{"ok": false, "error": msg})
}

func DecodeBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// ---------------- API ----------------

func (a *Server) Api(w http.ResponseWriter, r *http.Request, route string) {
	method := r.Method
	q := r.URL.Query()
	st := a.st
	if st == nil {
		ApiErr(w, http.StatusInternalServerError, "数据库未就绪")
		return
	}

	switch route {
	case "overview":
		group, _, words := a.b.Rules()
		rulesN := len(group.Rules)
		groups, _ := st.Groups()
		ruleList, _ := st.Rules()
		wordRecs, _ := st.Words()
		admins, _ := st.Admins()
		mutes, _ := st.Mutes(true)
		bl, _ := st.Blacklists("")
		recent, _ := st.Audit(20, "", "")
		status := a.b.Status()
		ApiOK(w, map[string]interface{}{
			"bot":       status,
			"db_file":   cfg.DBPath,
			"counts":    map[string]int{"groups": len(groups), "rules": len(ruleList), "words": len(wordRecs), "admins": len(admins), "mutes": len(mutes), "blacklist": len(bl)},
			"settings":  st.Settings(),
			"recent":    recent,
			"whitelist": "踢出/拉黑/群成员列表/群信息需要 QQ 白名单权限（错误码 11253，联系平台运营申请）",
			"cmd_help":  bot.HelpText(),
			"admin_url": a.addr + a.prefix,
			"_rules":    rulesN,
			"_words":    words.Count(),
		})

	// ---- 回复规则 ----
	case "rules":
		switch method {
		case http.MethodGet:
			list, err := st.Rules()
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost, http.MethodPut:
			var rec store.RuleRec
			if err := DecodeBody(r, &rec); err != nil {
				ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
				return
			}
			if rec.ID == 0 {
				rec.Enabled = true
			}
			id, err := st.SaveRule(rec)
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			a.b.InvalidateRules()
			ApiOK(w, map[string]interface{}{"id": id})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
			if id == 0 {
				ApiErr(w, 400, "缺少 id")
				return
			}
			if err := st.DeleteRule(id); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			a.b.InvalidateRules()
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 违禁词 ----
	case "words":
		switch method {
		case http.MethodGet:
			list, err := st.Words()
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost, http.MethodPut:
			var rec store.WordRec
			if err := DecodeBody(r, &rec); err != nil {
				ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
				return
			}
			if rec.Word == "" {
				ApiErr(w, 400, "违禁词不能为空")
				return
			}
			if rec.ID == 0 {
				rec.Enabled = true
			}
			if rec.Match == "" {
				rec.Match = "contains"
			}
			id, err := st.SaveWord(rec)
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			a.b.InvalidateRules()
			ApiOK(w, map[string]interface{}{"id": id})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
			if err := st.DeleteWord(id); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			a.b.InvalidateRules()
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 群 ----
	case "groups":
		switch method {
		case http.MethodGet:
			list, err := st.Groups()
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in struct {
				GroupOpenID string  `json:"group_openid"`
				Name        *string `json:"name"`
				Enabled     *bool   `json:"enabled"`
				ReplyAll    *bool   `json:"reply_all"`
				Commands    *bool   `json:"commands_enabled"`
				GroupAdmins *bool   `json:"group_admin_can_command"`
				BotIsAdmin  *bool   `json:"bot_is_admin"`
				Note        *string `json:"note"`
			}
			if err := DecodeBody(r, &in); err != nil || in.GroupOpenID == "" {
				ApiErr(w, 400, "需要 group_openid")
				return
			}
			if err := st.EnsureGroup(in.GroupOpenID, ""); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			if err := st.UpdateGroup(in.GroupOpenID, in.Name, in.Enabled, in.ReplyAll, in.Commands, in.GroupAdmins, in.BotIsAdmin, in.Note); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "updated")
		case http.MethodDelete:
			gid := q.Get("group_openid")
			if gid == "" {
				ApiErr(w, 400, "缺少 group_openid")
				return
			}
			if err := st.DeleteGroup(gid); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 自动补全群名（QQ 群信息接口，需白名单；30 QPM）----
	case "groups/sync-names":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		groups, err := st.Groups()
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		const maxPerRun = 8
		updated, tried, remaining := 0, 0, 0
		permission := true
		problems := make([]string, 0, 4)
		for i, g := range groups {
			if strings.TrimSpace(g.Name) != "" {
				continue
			}
			if tried >= maxPerRun {
				break
			}
			tried++
			info, r := a.b.QqGroupInfo(g.OpenID)
			if !r.Ok() {
				if r.ErrorCode() == 11253 {
					permission = false
					problems = append(problems, "机器人没有「群信息」接口白名单（11253），无法自动获取群名")
					break
				}
				problems = append(problems, fmt.Sprintf("第 %d 个群：%s", i+1, r.String()))
				continue
			}
			if strings.TrimSpace(info.Name) == "" {
				problems = append(problems, fmt.Sprintf("第 %d 个群：QQ 返回的群名为空", i+1))
				continue
			}
			if err := st.EnsureGroup(g.OpenID, info.Name); err != nil {
				problems = append(problems, fmt.Sprintf("第 %d 个群：写入数据库失败", i+1))
				continue
			}
			updated++
		}
		if list, err := st.Groups(); err == nil {
			for _, g := range list {
				if strings.TrimSpace(g.Name) == "" {
					remaining++
				}
			}
		}
		if updated > 0 {
			_ = st.AddAudit(store.AuditRec{Kind: "manual", Hit: "自动补全群名",
				Actions: `["rename"]`, Result: "ok",
				Detail: fmt.Sprintf("更新 %d 个群名，剩余 %d 个未命名", updated, remaining)})
		}
		ApiOK(w, map[string]interface{}{
			"updated": updated, "tried": tried, "remaining": remaining,
			"permission": permission, "problems": problems,
		})

	// ---- 管理员 ----
	case "admins":
		switch method {
		case http.MethodGet:
			list, _ := st.Admins()
			ApiOK(w, list)
		case http.MethodPost:
			var rec store.AdminRec
			if err := DecodeBody(r, &rec); err != nil || rec.MemberOpenID == "" {
				ApiErr(w, 400, "需要 member_openid")
				return
			}
			rec.Enabled = true
			id, err := st.AddAdmin(rec)
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, map[string]interface{}{"id": id})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
			if err := st.DeleteAdmin(id); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 群名册（本地）----
	// group 为空时返回跨群名册，网页端用它把成员 ID 换成昵称。
	case "members":
		gid := strings.TrimSpace(q.Get("group"))
		limit := 500
		if gid == "" {
			limit = 2000
		}
		list, err := st.Members(gid, limit)
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		ApiOK(w, list)

	// ---- 禁言 ----
	case "mutes":
		switch method {
		case http.MethodGet:
			list, err := st.Mutes(q.Get("all") != "1")
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in struct {
				GroupOpenID  string `json:"group_openid"`
				MemberOpenID string `json:"member_openid"`
				Username     string `json:"username"`
				Seconds      int    `json:"seconds"`
				Reason       string `json:"reason"`
			}
			if err := DecodeBody(r, &in); err != nil || in.GroupOpenID == "" || in.MemberOpenID == "" {
				ApiErr(w, 400, "需要 group_openid 和 member_openid")
				return
			}
			if in.Seconds <= 0 {
				in.Seconds = 600
			}
			results := a.b.MuteMember(bot.ActionCtx{
				GroupOpenID: in.GroupOpenID, MemberOpenID: in.MemberOpenID, Username: in.Username,
				Kind: "manual", Hit: "网页禁言", Operator: "web",
			}, in.Seconds, OrDefault(in.Reason, "网页后台禁言"))
			ApiOK(w, results)
		case http.MethodDelete:
			gid, mid := q.Get("group"), q.Get("member")
			if gid == "" || mid == "" {
				ApiErr(w, 400, "需要 group 和 member")
				return
			}
			results := a.b.UnmuteMember(bot.ActionCtx{GroupOpenID: gid, MemberOpenID: mid, Kind: "manual", Hit: "网页解禁", Operator: "web"})
			ApiOK(w, results)
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 黑名单 ----
	case "blacklist":
		switch method {
		case http.MethodGet:
			list, err := st.Blacklists(q.Get("group"))
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in struct {
				GroupOpenID  string `json:"group_openid"`
				MemberOpenID string `json:"member_openid"`
				Username     string `json:"username"`
				Reason       string `json:"reason"`
				Kick         bool   `json:"kick"`
				QQSync       bool   `json:"qq_sync"`
			}
			if err := DecodeBody(r, &in); err != nil || in.MemberOpenID == "" {
				ApiErr(w, 400, "需要 member_openid")
				return
			}
			ctx := bot.ActionCtx{
				GroupOpenID: in.GroupOpenID, MemberOpenID: in.MemberOpenID, Username: in.Username,
				Kind: "manual", Hit: "网页拉黑", Operator: "web",
			}
			results := a.b.ApplyPlan(ctx, bot.ActionPlan{
				Kick: in.Kick, Blacklist: true, Reason: OrDefault(in.Reason, "网页后台拉黑"),
			})
			ApiOK(w, results)
		case http.MethodDelete:
			gid, mid := q.Get("group"), q.Get("member")
			if mid == "" {
				ApiErr(w, 400, "需要 member")
				return
			}
			_ = st.RemoveBlacklist(gid, mid)
			r := a.b.QqBlacklistOp(gid, "del", []string{mid})
			ApiOK(w, map[string]interface{}{"local": "removed", "qq": r.String()})
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 审计 ----
	case "audit":
		limit, _ := strconv.Atoi(q.Get("limit"))
		list, err := st.Audit(limit, q.Get("group"), q.Get("kind"))
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		ApiOK(w, list)
	case "audit/prune":
		keep, _ := strconv.Atoi(q.Get("keep"))
		if keep <= 0 {
			keep = 2000
		}
		if err := st.PruneAudit(keep); err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		ApiOK(w, "pruned")

	// ---- 设置 ----
	case "settings":
		switch method {
		case http.MethodGet:
			ApiOK(w, st.Settings())
		case http.MethodPost:
			var in map[string]string
			if err := DecodeBody(r, &in); err != nil {
				ApiErr(w, 400, "请求体不是合法 JSON")
				return
			}
			for k, v := range in {
				if err := st.SetSetting(k, v); err != nil {
					ApiErr(w, 500, err.Error())
					return
				}
			}
			a.b.InvalidateRules()
			ApiOK(w, "saved")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 网页群消息（聊天室 / 卡片群发）----
	case "chat/groups":
		groups, err := st.Groups()
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		out := make([]chatGroupView, 0, len(groups))
		for _, g := range groups {
			if !g.Enabled {
				continue // 只列出当前可发送（已启用）的群
			}
			item := chatGroupView{OpenID: g.OpenID, Name: g.Name, Enabled: g.Enabled, BotIsAdmin: g.BotIsAdmin, Note: g.Note}
			if last := a.b.RecentList(g.OpenID, 1, 0); len(last) > 0 {
				item.LastAt = last[0].At.Format(time.RFC3339)
				text := last[0].Content
				if len([]rune(text)) > 60 {
					text = string([]rune(text)[:60]) + "…"
				}
				item.LastText = text
			}
			out = append(out, item)
		}
		ApiOK(w, out)

	case "chat/recent":
		gid := strings.TrimSpace(q.Get("group"))
		if gid == "" {
			ApiErr(w, 400, "需要 group")
			return
		}
		if _, ok := st.Group(gid); !ok {
			ApiErr(w, 404, "群不存在或还没有登记")
			return
		}
		list := a.b.RecentList(gid, 60, 0)
		messages := make([]map[string]interface{}, 0, len(list))
		for _, m := range list {
			messages = append(messages, map[string]interface{}{
				"id":            m.ID,
				"username":      m.Username,
				"member_openid": m.MemberOpenID,
				"content":       m.Content,
				"kind":          m.Kind,
				"is_bot":        m.IsBot,
				"at":            m.At.Format(time.RFC3339),
			})
		}
		ApiOK(w, map[string]interface{}{"group": gid, "messages": messages})

	case "chat/send":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			GroupOpenID string `json:"group_openid"`
			Format      string `json:"format"`
			Title       string `json:"title"`
			Content     string `json:"content"`
		}
		if err := DecodeBody(r, &in); err != nil {
			ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
			return
		}
		gid := strings.TrimSpace(in.GroupOpenID)
		title := strings.Join(strings.Fields(in.Title), " ")
		content := strings.TrimSpace(in.Content)
		if gid == "" {
			ApiErr(w, 400, "需要 group_openid")
			return
		}
		if content == "" {
			ApiErr(w, 400, "消息内容不能为空")
			return
		}
		if len([]rune(content)) > 2000 {
			ApiErr(w, 400, "消息内容太长（最多 2000 字）")
			return
		}
		if len([]rune(title)) > 60 {
			ApiErr(w, 400, "卡片标题最多 60 字")
			return
		}
		g, ok := st.Group(gid)
		if !ok {
			ApiErr(w, 404, "群还没有登记，请先在「群」里添加")
			return
		}
		if !g.Enabled {
			ApiErr(w, 400, "该群已停用，请先在「群」里启用")
			return
		}
		format := strings.ToLower(strings.TrimSpace(in.Format))
		if format != "text" {
			format = "card"
		}
		var (
			messageID string
			sendErr   error
			markdown  string
		)
		if format == "card" {
			markdown = BuildCardMarkdown(title, content)
			messageID, sendErr = a.b.SendGroupCardNow(gid, markdown)
		} else {
			messageID, sendErr = a.b.SendGroupTextNow(gid, content)
		}
		result, detail := "ok", fmt.Sprintf("id=%s format=%s", messageID, format)
		if sendErr != nil {
			result, detail = "fail", sendErr.Error()
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "push", GroupOpenID: gid, Username: "网页后台", Content: content,
			Hit: "网页群发", Actions: `["send"]`, Result: result, Detail: detail,
		})
		if sendErr != nil {
			ApiErr(w, 502, sendErr.Error())
			return
		}
		ApiOK(w, map[string]interface{}{
			"id": messageID, "group_openid": gid, "format": format, "markdown": markdown,
		})

	// ---- 导出 / 重载 ----
	case "export":
		buf, err := st.ExportJSON()
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="qbot-config.json"`)
		_, _ = w.Write(buf)
	case "reload":
		_ = a.b.ReloadAll()
		ApiOK(w, "已重载规则缓存与凭据，并重建网关连接")

	// ---- 直接调 QQ 的动作 ----
	case "qq/mute":
		var in struct {
			GroupOpenID  string `json:"group_openid"`
			MemberOpenID string `json:"member_openid"`
			Op           string `json:"op"` // add|update|del
			Seconds      int    `json:"seconds"`
		}
		if err := DecodeBody(r, &in); err != nil || in.GroupOpenID == "" || in.MemberOpenID == "" {
			ApiErr(w, 400, "需要 group_openid 与 member_openid")
			return
		}
		if in.Op == "" {
			in.Op = "add"
		}
		expire := ""
		if in.Op != "del" && in.Seconds > 0 {
			if in.Seconds > bot.MaxMuteSeconds {
				in.Seconds = bot.MaxMuteSeconds
			}
			expire = time.Now().Add(time.Duration(in.Seconds) * time.Second).Format(time.RFC3339)
		}
		r := a.b.QqSetMutes(in.GroupOpenID, []bot.MuteOp{{Op: in.Op, MemberOpenID: in.MemberOpenID, MuteExpireAt: expire}})
		ApiOK(w, map[string]interface{}{"ok": r.Ok(), "result": r.String()})

	case "qq/kick":
		var in struct {
			GroupOpenID   string   `json:"group_openid"`
			MemberOpenIDs []string `json:"member_openids"`
			WithBlacklist bool     `json:"add_to_member_blacklist"`
		}
		if err := DecodeBody(r, &in); err != nil || in.GroupOpenID == "" || len(in.MemberOpenIDs) == 0 {
			ApiErr(w, 400, "需要 group_openid 与 member_openids")
			return
		}
		r := a.b.QqRemoveMembers(in.GroupOpenID, in.MemberOpenIDs, in.WithBlacklist)
		if in.WithBlacklist {
			for _, mid := range in.MemberOpenIDs {
				_ = st.AddBlacklist(store.BlackRec{GroupOpenID: in.GroupOpenID, MemberOpenID: mid,
					Reason: "网页后台踢出", Operator: "web", QQSynced: r.Ok() && in.WithBlacklist})
			}
		}
		_ = st.AddAudit(store.AuditRec{Kind: "manual", GroupOpenID: in.GroupOpenID, Hit: "网页踢人",
			Actions: `["kick"]`, Result: map[bool]string{true: "ok", false: "fail"}[r.Ok()], Detail: r.String()})
		ApiOK(w, map[string]interface{}{"ok": r.Ok(), "result": r.String()})

	case "qq/recall":
		var in struct {
			GroupOpenID string `json:"group_openid"`
			MessageID   string `json:"message_id"`
			MemberID    string `json:"member_openid"`
			Count       int    `json:"count"`
		}
		if err := DecodeBody(r, &in); err != nil {
			ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
			return
		}
		if in.GroupOpenID == "" {
			ApiErr(w, 400, "需要 group_openid")
			return
		}
		// 三种用法：指定 message_id 撤一条；指定 member_openid 撤他最后一条；count 撤最近 N 条
		var ids []string
		switch {
		case in.MessageID != "":
			ids = []string{in.MessageID}
		case in.MemberID != "":
			if rm, ok := a.b.LookupRecentByMember(in.GroupOpenID, in.MemberID); ok {
				ids = []string{rm.ID}
			}
		default:
			n := in.Count
			if n <= 0 {
				n = 1
			}
			if n > bot.RecallMaxBatch {
				n = bot.RecallMaxBatch
			}
			for _, rm := range a.b.RecentList(in.GroupOpenID, n, 2*time.Minute) {
				ids = append(ids, rm.ID)
			}
		}
		if len(ids) == 0 {
			ApiErr(w, 400, "没有可撤回的消息（机器人没见过 / 超出 2 分钟窗口）")
			return
		}
		results := a.b.ApplyPlan(bot.ActionCtx{
			GroupOpenID: in.GroupOpenID, MessageID: ids[0], Kind: "admin", Hit: "网页撤回", Operator: "网页后台",
		}, bot.ActionPlan{Recall: true, RecallIDs: ids, Reason: "网页后台撤回"})
		ApiOK(w, map[string]interface{}{"ok": true, "count": len(ids), "result": bot.JoinResults(results, "撤回已执行")})

	case "qq/groupinfo":
		gid := q.Get("group")
		info, r := a.b.QqGroupInfo(gid)
		if !r.Ok() {
			ApiOK(w, map[string]interface{}{"ok": false, "error": r.String(), "Hint": r.Hint()})
			return
		}
		_ = st.EnsureGroup(gid, info.Name)
		ApiOK(w, info)

	case "qq/mutestate":
		gid := q.Get("group")
		list, mode, r := a.b.QqMuteSetting(gid)
		if !r.Ok() {
			ApiOK(w, map[string]interface{}{"ok": false, "error": r.String(), "Hint": r.Hint()})
			return
		}
		ApiOK(w, map[string]interface{}{"ok": true, "global_mode": mode, "members": list})

	case "qq/members":
		gid := q.Get("group")
		list, cursor, r := a.b.QqMembers(gid, q.Get("cursor"))
		if !r.Ok() {
			ApiOK(w, map[string]interface{}{"ok": false, "error": r.String(), "Hint": r.Hint()})
			return
		}
		for _, m := range list {
			_ = st.TouchMember(gid, m.MemberOpenID, m.Username, m.Role, m.Bot)
		}
		ApiOK(w, map[string]interface{}{"ok": true, "members": list, "next_cursor": cursor})

	case "qq/botstate":
		gid := q.Get("group")
		r := a.b.QqBotState(gid, st)
		ApiOK(w, map[string]interface{}{"ok": r.Ok(), "result": r.String()})

	// ---- 入群申请 ----
	case "joins":
		gid := q.Get("group")
		switch method {
		case http.MethodGet:
			list, cursor, r := a.b.QqJoinRequests(gid, q.Get("cursor"))
			if !r.Ok() {
				ApiOK(w, map[string]interface{}{"ok": false, "error": r.String(), "Hint": r.Hint()})
				return
			}
			ApiOK(w, map[string]interface{}{"ok": true, "list": list, "next_cursor": cursor})
		case http.MethodPost:
			var in struct {
				GroupOpenID    string `json:"group_openid"`
				MemberOpenID   string `json:"member_openid"`
				JoinRequestID  string `json:"join_request_id"`
				Approve        bool   `json:"approve"`
				Reason         string `json:"reason"`
				AddToBlacklist bool   `json:"add_to_member_blacklist"`
			}
			if err := DecodeBody(r, &in); err != nil || in.GroupOpenID == "" || in.MemberOpenID == "" {
				ApiErr(w, 400, "需要 group_openid 与 member_openid")
				return
			}
			r := a.b.QqApproveJoinRequest(in.GroupOpenID, in.MemberOpenID, in.JoinRequestID, in.Approve, in.Reason, in.AddToBlacklist)
			if !in.Approve && in.AddToBlacklist && r.Ok() {
				_ = st.AddBlacklist(store.BlackRec{GroupOpenID: in.GroupOpenID, MemberOpenID: in.MemberOpenID,
					Reason: OrDefault(in.Reason, "入群申请被拒绝"), Operator: "web", QQSynced: true})
			}
			_ = st.AddAudit(store.AuditRec{Kind: "join", GroupOpenID: in.GroupOpenID, MemberOpenID: in.MemberOpenID,
				Hit: "入群审批", Actions: `["join"]`, Result: map[bool]string{true: "ok", false: "fail"}[r.Ok()], Detail: r.String()})
			ApiOK(w, map[string]interface{}{"ok": r.Ok(), "result": r.String()})
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	case "help":
		ApiOK(w, map[string]interface{}{"commands": bot.HelpText(), "actions": "禁言/解禁/踢出/拉黑/撤回/入群审批/规则/违禁词/管理员"})

	default:
		ApiErr(w, http.StatusNotFound, "未知接口: "+route)
	}
}

func OrDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// BuildCardMarkdown 把标题 + 正文包装成 QQ 群 Markdown 卡片。
// 群聊自定义 Markdown 已对全部机器人开放，不需要模板 ID。
func BuildCardMarkdown(title, content string) string {
	title = strings.Join(strings.Fields(title), " ")
	content = strings.TrimSpace(content)
	if title == "" {
		title = "群消息"
	}
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(content)
	b.WriteString("\n\n> botgo-plus · ")
	b.WriteString(time.Now().Format("2006-01-02 15:04"))
	return b.String()
}

// ServeUI 输出管理页面（token 从 URL 带进来时会写 cookie，之后刷新不用再带）。
func (a *Server) ServeUI(w http.ResponseWriter, r *http.Request) {
	if t := strings.TrimSpace(r.URL.Query().Get("token")); t != "" && a.token != "" {
		if subtle.ConstantTimeCompare([]byte(t), []byte(a.token)) == 1 {
			http.SetCookie(w, &http.Cookie{
				Name: "qbot_admin_token", Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
				MaxAge: 30 * 24 * 3600,
			})
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	html, _ := LoadAdminHTML()
	// 页面里是 `const BASE = "__BASE_PATH__";`，前缀来自反代头，必须按 JS 字符串转义。
	page := strings.ReplaceAll(html, "__BASE_PATH__", jsString(a.BasePath(r)))
	_, _ = w.Write([]byte(page))
}

// jsString 把任意字符串安全地放进双引号 JS 字符串里。
func jsString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "<", `\u003c`)
	s = strings.ReplaceAll(s, ">", `\u003e`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\u2028", `\u2028`)
	s = strings.ReplaceAll(s, "\u2029", `\u2029`)
	return s
}

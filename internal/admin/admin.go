package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"LianT/internal/bot"
	"LianT/internal/config"
	"LianT/internal/manager"
	"LianT/internal/platform"
	"LianT/internal/store"
	"LianT/internal/telegram"
)

// ============================================================================
// 网页管理端
//
//   * 只认 token（LIANT_ADMIN_TOKEN > .env 里的同名项 > 数据库里自动生成的）
//   * 默认只绑回环 127.0.0.1:28742（LIANT_ADMIN_ADDR 可改）；
//     绑到非回环又没配 token 会拒绝启动
//   * 支持 Nginx / Caddy 反代（剥前缀 / 带前缀都行）
//   * 所有写操作都会写审计
// ============================================================================

type Server struct {
	m      *manager.Manager
	reg    *store.Store
	token  string
	prefix string
	addr   string
	srv    *http.Server
}

// botInput 是多 Bot 管理接口的输入（token 单独传，不在 BotRec 里暴露）。
type botInput struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Token         string `json:"token"`
	Mode          string `json:"mode"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"`
	WebhookListen string `json:"webhook_listen"`
	WebhookPath   string `json:"webhook_path"`
	Platform      string `json:"platform"`
	Enabled       bool   `json:"enabled"`
}

func (in botInput) toRec() store.BotRec {
	return store.BotRec{
		ID: in.ID, Name: in.Name, Token: in.Token, Mode: in.Mode,
		WebhookURL: in.WebhookURL, WebhookSecret: in.WebhookSecret,
		WebhookListen: in.WebhookListen, WebhookPath: in.WebhookPath,
		Platform: in.Platform, Enabled: in.Enabled,
	}
}

// chatView 是网页会话列表的展示结构。
type chatView struct {
	ChatID     string `json:"chat_id"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	BotIsAdmin bool   `json:"bot_is_admin"`
	Note       string `json:"note"`
	LastAt     string `json:"last_at,omitempty"`
	LastText   string `json:"last_text,omitempty"`
}

func ListenAddr() string { return config.AdminAddr() }

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

// TokenSource 返回管理端 token 以及它的来源。
func TokenSource(st *store.Store) (string, string) {
	if v := strings.TrimSpace(config.AdminToken()); v != "" {
		return v, "环境变量 LIANT_ADMIN_TOKEN"
	}
	if v, ok := config.EnvFile().Lookup("LIANT_ADMIN_TOKEN"); ok && strings.TrimSpace(v) != "" {
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

func TokenValue(st *store.Store) string {
	tok, _ := TokenSource(st)
	return tok
}

func RandomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("LianT-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// Start 启动管理端；返回 nil 表示没启动。
func Start(m *manager.Manager, reg *store.Store) *Server {
	if strings.EqualFold(config.EnvOr("LIANT_ADMIN_ENABLE", "true"), "false") {
		log.Printf("[i] 网页管理端已按 LIANT_ADMIN_ENABLE=false 关闭")
		return nil
	}
	tok, tokSrc := TokenSource(reg)
	if tok == "" {
		log.Printf("[!] 管理端没有可用 token（来源: %s）", tokSrc)
	} else {
		log.Printf("[i] 网页管理端 token 来源: %s", tokSrc)
	}
	a := &Server{
		m: m, reg: reg, token: tok,
		prefix: NormalizePrefix(config.AdminPrefix()),
		addr:   ListenAddr(),
	}
	host := a.addr
	if i := strings.LastIndex(a.addr, ":"); i > 0 {
		host = a.addr[:i]
	}
	loopback := config.IsLoopbackHost(host) || host == ""
	if a.token == "" && !loopback {
		log.Printf("[X] 管理端绑在 %s（非回环）但没有 token，出于安全拒绝启动。请设 LIANT_ADMIN_TOKEN 或改回 127.0.0.1", a.addr)
		return nil
	}
	ln, err := net.Listen("tcp", a.addr)
	if err != nil {
		log.Printf("[X] 管理端启动失败 %s: %v", a.addr, err)
		return nil
	}
	a.srv = &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	tlsCert := strings.TrimSpace(config.EnvOr("LIANT_ADMIN_TLS_CERT", ""))
	tlsKey := strings.TrimSpace(config.EnvOr("LIANT_ADMIN_TLS_KEY", ""))
	if (tlsCert == "") != (tlsKey == "") {
		log.Printf("[X] LIANT_ADMIN_TLS_CERT 和 LIANT_ADMIN_TLS_KEY 必须同时设置")
		return nil
	}
	scheme := "http"
	if tlsCert != "" {
		scheme = "https"
	}
	go func() {
		var err error
		if tlsCert != "" {
			err = a.srv.ServeTLS(ln, tlsCert, tlsKey)
		} else {
			err = a.srv.Serve(ln)
		}
		if err != nil && err != http.ErrServerClosed {
			log.Printf("[X] 管理端退出: %v", err)
		}
	}()
	base := scheme + "://" + a.addr + a.prefix
	if views, err := m.List(); err == nil {
		running := 0
		for _, v := range views {
			if v.Running {
				running++
			}
		}
		log.Printf("[i] 已注册 %d 个 Bot，运行中 %d 个", len(views), running)
	}
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

func secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
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
		if p == "/api/logout" {
			http.SetCookie(w, &http.Cookie{
				Name: "LianT_admin_token", Value: "", Path: "/", HttpOnly: true,
				Secure: secureRequest(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
			})
			ApiOK(w, "logged out")
			return
		}
		if !a.Authorized(r) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="LianT"`)
			WriteJSON(w, http.StatusUnauthorized, map[string]interface{}{"ok": false, "error": "未授权：请带上 token（Authorization: Bearer <token> 或 ?token=）"})
			return
		}
		a.Api(w, r, strings.TrimPrefix(p, "/api/"))
	})
}

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
		return true
	}
	given := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		given = strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if given == "" {
		given = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if given == "" {
		if c, err := r.Cookie("LianT_admin_token"); err == nil {
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
	reg := a.reg
	if reg == nil || a.m == nil {
		ApiErr(w, http.StatusInternalServerError, "存储未就绪")
		return
	}

	// 多 Bot：所有业务接口用 ?bot_id= 选择；不带则退回第一个运行中的 Bot。
	var rt *manager.Runtime
	if id, err := strconv.ParseInt(strings.TrimSpace(q.Get("bot_id")), 10, 64); err == nil && id > 0 {
		rt, _ = a.m.Get(id)
	} else {
		rt, _ = a.m.First()
	}
	var b *bot.Bot
	var st *store.Store
	if rt != nil {
		b, st = rt.Bot, rt.Store
	}
	if route == "overview" && rt == nil {
		ApiOK(w, map[string]interface{}{
			"bot":      map[string]string{"bot_state": "未配置"},
			"counts":   map[string]int{},
			"settings": map[string]string{},
			"recent":   []store.AuditRec{},
			"cmd_help": bot.HelpText(),
			"no_bot":   true,
			"db_file":  config.DBPath,
			"admin_url": a.addr + a.prefix,
		})
		return
	}
	if rt == nil && route != "bots" && route != "bots/update" && route != "bots/restart" && route != "platforms" && route != "help" {
		ApiErr(w, http.StatusConflict, "还没有运行中的 Bot：请先在「机器人」里添加 token 并启用")
		return
	}

	// qbot 页面兼容层：qbot admin.html 使用 OpenID/MemberOpenID 字段。
	if a.compat(w, r, route, method, rt, b, st) {
		return
	}

	switch route {
	case "overview":
		group, _, words := b.Rules()
		groups, _ := st.Groups()
		ruleList, _ := st.Rules()
		wordRecs, _ := st.Words()
		admins, _ := st.Admins()
		mutes, _ := st.Mutes(true)
		bl, _ := st.Blacklists("")
		joins, _ := st.JoinRequests("", "pending", 500)
		recent, _ := st.Audit(20, "", "")
		tokenSource := "未配置"
		if _, src := TokenSource(reg); src != "" {
			tokenSource = src
		}
		dbFile := config.DBPath
		if rt != nil {
			dbFile = manager.BotDBPath(rt.Rec.ID)
		}
		ApiOK(w, map[string]interface{}{
			"bot":          b.Status(),
			"db_file":      dbFile,
			"counts":       map[string]int{"chats": len(groups), "rules": len(ruleList), "words": len(wordRecs), "admins": len(admins), "mutes": len(mutes), "blacklist": len(bl), "joins": len(joins)},
			"settings":     st.PublicSettings(),
			"recent":       recent,
			"cmd_help":     bot.HelpText(),
			"admin_url":    a.addr + a.prefix,
			"token_source": tokenSource,
			"_rules":       len(group.Rules),
			"_words":       words.Count(),
		})

	// ---- 多 Bot 注册表 ----
	case "bots":
		switch method {
		case http.MethodGet:
			list, err := a.m.List()
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in botInput
			if err := DecodeBody(r, &in); err != nil {
				ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
				return
			}
			rec := in.toRec()
			if strings.TrimSpace(rec.Token) == "" {
				ApiErr(w, 400, "需要 token")
				return
			}
			rec.Enabled = true
			id, err := a.m.Add(rec)
			if err != nil {
				ApiErr(w, 502, err.Error())
				return
			}
			_ = reg.AddAudit(store.AuditRec{Kind: "manual", Username: "网页后台", Hit: "添加 Bot", Actions: `["bot"]`, Result: "ok", Detail: fmt.Sprintf("bot_id=%d", id)})
			ApiOK(w, map[string]interface{}{"id": id})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
			if id <= 0 {
				ApiErr(w, 400, "缺少 id")
				return
			}
			if err := a.m.Delete(id); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	case "platforms":
		var out []map[string]interface{}
		for _, ad := range platform.List() {
			out = append(out, map[string]interface{}{
				"id": ad.ID(), "name": ad.Name(), "capabilities": ad.Capabilities(),
			})
		}
		ApiOK(w, out)

	case "bots/update":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in botInput
		if err := DecodeBody(r, &in); err != nil || in.ID <= 0 {
			ApiErr(w, 400, "需要 id")
			return
		}
		if err := a.m.Update(in.toRec()); err != nil {
			ApiErr(w, 502, err.Error())
			return
		}
		ApiOK(w, "updated")

	case "bots/restart":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			ID int64 `json:"id"`
		}
		if err := DecodeBody(r, &in); err != nil || in.ID <= 0 {
			ApiErr(w, 400, "需要 id")
			return
		}
		if err := a.m.Restart(in.ID); err != nil {
			ApiErr(w, 502, err.Error())
			return
		}
		ApiOK(w, "restarted")

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
			b.InvalidateRules()
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
			b.InvalidateRules()
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

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
			if strings.TrimSpace(rec.Word) == "" {
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
			b.InvalidateRules()
			ApiOK(w, map[string]interface{}{"id": id})
		case http.MethodDelete:
			id, _ := strconv.ParseInt(q.Get("id"), 10, 64)
			if err := st.DeleteWord(id); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			b.InvalidateRules()
			ApiOK(w, "deleted")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 会话（群/频道）----
	case "chats":
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
				ChatID      string  `json:"chat_id"`
				Name        *string `json:"name"`
				Enabled     *bool   `json:"enabled"`
				ReplyAll    *bool   `json:"reply_all"`
				Commands    *bool   `json:"commands_enabled"`
				GroupAdmins *bool   `json:"group_admin_can_command"`
				BotIsAdmin  *bool   `json:"bot_is_admin"`
				Note        *string `json:"note"`
			}
			if err := DecodeBody(r, &in); err != nil || strings.TrimSpace(in.ChatID) == "" {
				ApiErr(w, 400, "需要 chat_id（群 ID 通常是负数，如 -1001234567890）")
				return
			}
			if err := st.EnsureGroup(in.ChatID, ""); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			if err := st.UpdateGroup(in.ChatID, in.Name, in.Enabled, in.ReplyAll, in.Commands, in.GroupAdmins, in.BotIsAdmin, in.Note); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "updated")
		case http.MethodDelete:
			gid := q.Get("chat_id")
			if gid == "" {
				ApiErr(w, 400, "缺少 chat_id")
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

	// ---- 从 Telegram 拉群名 + 机器人管理员状态 ----
	case "chats/sync":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		c := b.Client()
		if c == nil {
			ApiErr(w, 503, "机器人尚未连接 Telegram")
			return
		}
		groups, err := st.Groups()
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		updated, tried := 0, 0
		var problems []string
		for i, g := range groups {
			if tried >= 20 {
				break
			}
			tried++
			info, err := c.GetChat(g.ChatID)
			if err != nil {
				problems = append(problems, fmt.Sprintf("第 %d 个会话：%s", i+1, tgErr(err)))
				continue
			}
			name := info.Display()
			_ = st.UpdateGroup(g.ChatID, &name, nil, nil, nil, nil, nil, nil)
			if me := b.Me(); me.ID > 0 {
				if m, err := c.GetChatMember(g.ChatID, strconv.FormatInt(me.ID, 10)); err == nil {
					ok := m.IsAdmin()
					_ = st.UpdateGroup(g.ChatID, nil, nil, nil, nil, nil, &ok, nil)
				}
			}
			updated++
		}
		if updated > 0 {
			_ = st.AddAudit(store.AuditRec{Kind: "manual", Hit: "同步群信息",
				Actions: `["sync"]`, Result: "ok", Detail: fmt.Sprintf("更新 %d 个会话", updated)})
		}
		ApiOK(w, map[string]interface{}{"updated": updated, "tried": tried, "problems": problems})

	// ---- 单个会话详情 ----
	case "chats/info":
		c := b.Client()
		if c == nil {
			ApiErr(w, 503, "机器人尚未连接 Telegram")
			return
		}
		chatID := strings.TrimSpace(q.Get("chat_id"))
		if chatID == "" {
			ApiErr(w, 400, "需要 chat_id")
			return
		}
		info, err := c.GetChat(chatID)
		if err != nil {
			ApiOK(w, map[string]interface{}{"ok": false, "error": tgErr(err)})
			return
		}
		count, _ := c.GetChatMemberCount(chatID)
		botState := map[string]interface{}{}
		if me := b.Me(); me.ID > 0 {
			if m, err := c.GetChatMember(chatID, strconv.FormatInt(me.ID, 10)); err == nil {
				botState = map[string]interface{}{"status": m.Status, "is_admin": m.IsAdmin()}
			}
		}
		ApiOK(w, map[string]interface{}{
			"ok": true, "chat": info, "member_count": count, "bot": botState,
		})

	case "chats/admins":
		c := b.Client()
		if c == nil {
			ApiErr(w, 503, "机器人尚未连接 Telegram")
			return
		}
		chatID := strings.TrimSpace(q.Get("chat_id"))
		if chatID == "" {
			ApiErr(w, 400, "需要 chat_id")
			return
		}
		list, err := c.GetChatAdministrators(chatID)
		if err != nil {
			ApiOK(w, map[string]interface{}{"ok": false, "error": tgErr(err)})
			return
		}
		ApiOK(w, map[string]interface{}{"ok": true, "administrators": list})

	// ---- 管理员 ----
	case "admins":
		switch method {
		case http.MethodGet:
			list, _ := st.Admins()
			ApiOK(w, list)
		case http.MethodPost:
			var rec store.AdminRec
			if err := DecodeBody(r, &rec); err != nil || rec.UserID == "" {
				ApiErr(w, 400, "需要 user_id（Telegram 用户数字 ID）")
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

	// ---- 群名册（本地，来自消息）----
	case "members":
		gid := strings.TrimSpace(q.Get("chat_id"))
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
				ChatID   string `json:"chat_id"`
				UserID   string `json:"user_id"`
				Username string `json:"username"`
				Seconds  int    `json:"seconds"`
				Reason   string `json:"reason"`
			}
			if err := DecodeBody(r, &in); err != nil || in.ChatID == "" || in.UserID == "" {
				ApiErr(w, 400, "需要 chat_id 和 user_id")
				return
			}
			if in.Seconds <= 0 {
				in.Seconds = 600
			}
			results := b.MuteMember(bot.ActionCtx{
				ChatID: in.ChatID, UserID: in.UserID, Username: in.Username,
				Kind: "manual", Hit: "网页禁言", Operator: "web",
			}, time.Duration(in.Seconds)*time.Second, OrDefault(in.Reason, "网页后台禁言"))
			ApiOK(w, results)
		case http.MethodDelete:
			cid, uid := q.Get("chat_id"), q.Get("user_id")
			if cid == "" || uid == "" {
				ApiErr(w, 400, "需要 chat_id 和 user_id")
				return
			}
			results := b.UnmuteMember(bot.ActionCtx{ChatID: cid, UserID: uid, Kind: "manual", Hit: "网页解禁", Operator: "web"})
			ApiOK(w, results)
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 黑名单 ----
	case "blacklist":
		switch method {
		case http.MethodGet:
			list, err := st.Blacklists(q.Get("chat_id"))
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in struct {
				ChatID   string `json:"chat_id"`
				UserID   string `json:"user_id"`
				Username string `json:"username"`
				Reason   string `json:"reason"`
				Ban      bool   `json:"ban"`
			}
			if err := DecodeBody(r, &in); err != nil || in.UserID == "" {
				ApiErr(w, 400, "需要 user_id")
				return
			}
			if in.Ban {
				results := b.BanMember(bot.ActionCtx{
					ChatID: in.ChatID, UserID: in.UserID, Username: in.Username,
					Kind: "manual", Hit: "网页拉黑", Operator: "web",
				}, true)
				ApiOK(w, results)
				return
			}
			if err := st.AddBlacklist(store.BlackRec{
				ChatID: in.ChatID, UserID: in.UserID, Username: in.Username,
				Reason: OrDefault(in.Reason, "网页后台拉黑"), Operator: "web",
			}); err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, "added")
		case http.MethodDelete:
			cid, uid := q.Get("chat_id"), q.Get("user_id")
			if uid == "" {
				ApiErr(w, 400, "需要 user_id")
				return
			}
			_ = st.RemoveBlacklist(cid, uid)
			results := b.UnbanMember(bot.ActionCtx{ChatID: cid, UserID: uid, Kind: "manual", Hit: "网页解黑", Operator: "web"})
			ApiOK(w, map[string]interface{}{"local": "removed", "telegram": results})
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 入群申请 ----
	case "joins":
		switch method {
		case http.MethodGet:
			list, err := st.JoinRequests(q.Get("chat_id"), q.Get("status"), 300)
			if err != nil {
				ApiErr(w, 500, err.Error())
				return
			}
			ApiOK(w, list)
		case http.MethodPost:
			var in struct {
				ChatID string `json:"chat_id"`
				UserID string `json:"user_id"`
				Action string `json:"action"` // approve | decline
				Reason string `json:"reason"`
			}
			if err := DecodeBody(r, &in); err != nil || in.ChatID == "" || in.UserID == "" {
				ApiErr(w, 400, "需要 chat_id 和 user_id")
				return
			}
			c := b.Client()
			if c == nil {
				ApiErr(w, 503, "机器人尚未连接 Telegram")
				return
			}
			status, result := "", "ok"
			var opErr error
			switch strings.ToLower(strings.TrimSpace(in.Action)) {
			case "approve":
				opErr = c.ApproveChatJoinRequest(in.ChatID, in.UserID)
				status = "approved"
			case "decline":
				opErr = c.DeclineChatJoinRequest(in.ChatID, in.UserID)
				status = "declined"
			default:
				ApiErr(w, 400, "action 只能是 approve / decline")
				return
			}
			if opErr != nil {
				result = "fail"
			} else {
				_ = st.SetJoinRequestStatus(in.ChatID, in.UserID, status)
			}
			_ = st.AddAudit(store.AuditRec{
				Kind: "join", ChatID: in.ChatID, UserID: in.UserID,
				Content: in.Reason, Hit: "网页审批入群", Actions: `["join"]`,
				Result: result, Detail: tgErr(opErr),
			})
			if opErr != nil {
				ApiErr(w, 502, tgErr(opErr))
				return
			}
			ApiOK(w, map[string]interface{}{"chat_id": in.ChatID, "user_id": in.UserID, "status": status})
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 群发 ----
	case "broadcast":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			ChatIDs    []string `json:"chat_ids"`
			Content    string   `json:"content"`
			Kind       string   `json:"kind"`
			Media      string   `json:"media"`
			Caption    string   `json:"caption"`
			ParseMode  string   `json:"parse_mode"`
			ThreadID   int64    `json:"thread_id"`
		}
		if err := DecodeBody(r, &in); err != nil {
			ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
			return
		}
		if len(in.ChatIDs) == 0 {
			ApiErr(w, 400, "需要 chat_ids")
			return
		}
		if len(in.ChatIDs) > 50 {
			ApiErr(w, 400, "一次最多群发 50 个会话")
			return
		}
		kind := strings.ToLower(strings.TrimSpace(in.Kind))
		if kind == "" {
			kind = "text"
		}
		opt := telegram.SendOptions{ThreadID: in.ThreadID, ParseMode: strings.TrimSpace(in.ParseMode)}
		type oneResult struct {
			ChatID string `json:"chat_id"`
			OK     bool   `json:"ok"`
			ID     int64  `json:"id,omitempty"`
			Error  string `json:"error,omitempty"`
		}
		results := make([]oneResult, 0, len(in.ChatIDs))
		okCount := 0
		for _, cid := range in.ChatIDs {
			cid = strings.TrimSpace(cid)
			if cid == "" {
				continue
			}
			g, ok := st.Group(cid)
			if !ok || !g.Enabled {
				results = append(results, oneResult{ChatID: cid, Error: "会话未登记或已停用"})
				continue
			}
			var id int64
			var err error
			if kind == "text" {
				id, err = b.SendMessageOpt(cid, strings.TrimSpace(in.Content), opt)
			} else {
				id, err = b.SendMediaNow(cid, kind, strings.TrimSpace(in.Media), strings.TrimSpace(in.Caption), opt)
			}
			if err != nil {
				results = append(results, oneResult{ChatID: cid, Error: tgErr(err)})
				continue
			}
			okCount++
			results = append(results, oneResult{ChatID: cid, OK: true, ID: id})
			time.Sleep(80 * time.Millisecond)
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "push", Username: "网页后台", Content: strings.TrimSpace(in.Content),
			Hit: "网页群发", Actions: `["broadcast"]`,
			Result: map[bool]string{true: "ok", false: "fail"}[okCount > 0],
			Detail: fmt.Sprintf("成功 %d/%d", okCount, len(results)),
		})
		ApiOK(w, map[string]interface{}{"ok_count": okCount, "results": results})

	// ---- 审计 ----
	case "audit":
		limit, _ := strconv.Atoi(q.Get("limit"))
		list, err := st.Audit(limit, q.Get("chat_id"), q.Get("kind"))
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
			ApiOK(w, st.PublicSettings())
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
			b.InvalidateRules()
			ApiOK(w, "saved")
		default:
			ApiErr(w, 405, "不支持的方法")
		}

	// ---- 最近消息 ----
	case "messages":
		gid := strings.TrimSpace(q.Get("chat_id"))
		if gid == "" {
			ApiErr(w, 400, "需要 chat_id")
			return
		}
		list := b.RecentList(gid, 60, 0)
		messages := make([]map[string]interface{}, 0, len(list))
		for _, m := range list {
			messages = append(messages, map[string]interface{}{
				"message_id": m.MessageID,
				"user_id":    m.UserID,
				"username":   m.Username,
				"content":    m.Content,
				"kind":       m.Kind,
				"is_bot":     m.IsBot,
				"at":         m.At.Format(time.RFC3339),
			})
		}
		ApiOK(w, messages)

	// ---- 发消息 ----
	case "send":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			ChatID        string          `json:"chat_id"`
			Content       string          `json:"content"`
			Kind          string          `json:"kind"`
			Media         string          `json:"media"`
			Caption       string          `json:"caption"`
			ParseMode     string          `json:"parse_mode"`
			ReplyTo       int64           `json:"reply_to"`
			ThreadID      int64           `json:"thread_id"`
			DisablePreview bool           `json:"disable_preview"`
			Buttons       json.RawMessage `json:"buttons"`
		}
		if err := DecodeBody(r, &in); err != nil {
			ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
			return
		}
		chatID := strings.TrimSpace(in.ChatID)
		if chatID == "" {
			ApiErr(w, 400, "需要 chat_id")
			return
		}
		kind := strings.ToLower(strings.TrimSpace(in.Kind))
		if kind == "" {
			kind = "text"
		}
		g, ok := st.Group(chatID)
		if !ok {
			ApiErr(w, 404, "会话还没有登记，请先在「群组」里添加")
			return
		}
		if !g.Enabled {
			ApiErr(w, 400, "该会话已停用，请先在「群组」里启用")
			return
		}

		opt := telegram.SendOptions{
			ReplyTo:        in.ReplyTo,
			ThreadID:       in.ThreadID,
			ParseMode:      strings.TrimSpace(in.ParseMode),
			DisablePreview: in.DisablePreview,
		}
		if len(in.Buttons) > 0 {
			var v interface{}
			if err := json.Unmarshal(in.Buttons, &v); err != nil {
				ApiErr(w, 400, "buttons 不是合法 JSON: "+err.Error())
				return
			}
			// 直接传二维数组时自动包成 inline_keyboard；传完整对象则原样使用。
			if arr, ok := v.([]interface{}); ok {
				opt.ReplyMarkup = map[string]interface{}{"inline_keyboard": arr}
			} else {
				opt.ReplyMarkup = v
			}
		}

		var (
			messageID int64
			sendErr   error
			content   string
			detailTxt string
		)
		if kind == "text" {
			content = strings.TrimSpace(in.Content)
			if content == "" {
				ApiErr(w, 400, "消息内容不能为空")
				return
			}
			messageID, sendErr = b.SendMessageOpt(chatID, content, opt)
			detailTxt = fmt.Sprintf("id=%d kind=text", messageID)
		} else {
			media := strings.TrimSpace(in.Media)
			if media == "" && kind == "poll" {
				media = strings.TrimSpace(in.Content)
			}
			if media == "" {
				ApiErr(w, 400, "需要 media（file_id / URL / 坐标 / 投票问题）")
				return
			}
			content = media
			messageID, sendErr = b.SendMediaNow(chatID, kind, media, strings.TrimSpace(in.Caption), opt)
			detailTxt = fmt.Sprintf("id=%d kind=%s media=%s", messageID, kind, media)
		}
		result := "ok"
		if sendErr != nil {
			result, detailTxt = "fail", sendErr.Error()
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "push", ChatID: chatID, Username: "网页后台", Content: content,
			Hit: "网页发消息", Actions: `["send"]`, Result: result, Detail: detailTxt,
		})
		if sendErr != nil {
			ApiErr(w, 502, tgErr(sendErr))
			return
		}
		ApiOK(w, map[string]interface{}{"id": messageID, "chat_id": chatID, "kind": kind})

	// ---- 撤回 ----
	case "recall":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			ChatID     string  `json:"chat_id"`
			MessageID  int64   `json:"message_id"`
			MessageIDs []int64 `json:"message_ids"`
		}
		if err := DecodeBody(r, &in); err != nil || in.ChatID == "" {
			ApiErr(w, 400, "需要 chat_id 和 message_id(s)")
			return
		}
		ids := in.MessageIDs
		if in.MessageID > 0 {
			ids = append(ids, in.MessageID)
		}
		if len(ids) == 0 {
			ApiErr(w, 400, "没有要撤回的消息")
			return
		}
		n, err := b.RecallMessages(in.ChatID, ids)
		result := "ok"
		if err != nil {
			result = "fail"
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "manual", ChatID: in.ChatID, Hit: "网页撤回",
			Actions: `["recall"]`, Result: result,
			Detail: fmt.Sprintf("成功 %d/%d %s", n, len(ids), tgErr(err)),
		})
		if err != nil && n == 0 {
			ApiErr(w, 502, tgErr(err))
			return
		}
		ApiOK(w, map[string]interface{}{"ok": true, "count": n, "error": tgErr(err)})

	// ---- 上传本地文件并调用 Bot API（multipart） ----
	case "upload":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			ApiErr(w, 400, "上传表单解析失败: "+err.Error())
			return
		}
		m := strings.TrimSpace(r.FormValue("method"))
		if !validAPIMethod(m) {
			ApiErr(w, 400, "需要合法的 method（字母数字）")
			return
		}
		field := strings.TrimSpace(r.FormValue("file_field"))
		if field == "" {
			field = "photo"
		}
		params := map[string]interface{}{}
		if raw := strings.TrimSpace(r.FormValue("params")); raw != "" {
			if err := json.Unmarshal([]byte(raw), &params); err != nil {
				ApiErr(w, 400, "params 不是合法 JSON: "+err.Error())
				return
			}
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			ApiErr(w, 400, "缺少文件: "+err.Error())
			return
		}
		tmp, err := os.CreateTemp(config.DataDir, "LianT-upload-*")
		if err != nil {
			_ = file.Close()
			ApiErr(w, 500, "创建临时文件失败: "+err.Error())
			return
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if _, err := io.Copy(tmp, file); err != nil {
			_ = file.Close()
			_ = tmp.Close()
			ApiErr(w, 500, "保存上传文件失败: "+err.Error())
			return
		}
		_ = file.Close()
		_ = tmp.Close()
		c := b.Client()
		if c == nil {
			ApiErr(w, 503, "机器人尚未连接 Telegram")
			return
		}
		out, err := c.CallMultipart(m, params, field, tmpName)
		if err != nil {
			ApiErr(w, 502, tgErr(err))
			return
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "manual", Username: "网页后台", Hit: "Bot API 上传: " + m,
			Actions: `["upload"]`, Result: "ok",
		})
		ApiOK(w, map[string]interface{}{"method": m, "result": out})

	// ---- 通用 Bot API 控制台：任意方法都能调 ----
	case "botapi":
		if method != http.MethodPost {
			ApiErr(w, 405, "不支持的方法")
			return
		}
		var in struct {
			Method string                 `json:"method"`
			Params map[string]interface{} `json:"params"`
		}
		if err := DecodeBody(r, &in); err != nil {
			ApiErr(w, 400, "请求体不是合法 JSON: "+err.Error())
			return
		}
		m := strings.TrimSpace(in.Method)
		if m == "" {
			ApiErr(w, 400, "需要 method")
			return
		}
		if !validAPIMethod(m) {
			ApiErr(w, 400, "method 只能是字母数字")
			return
		}
		c := b.Client()
		if c == nil {
			ApiErr(w, 503, "机器人尚未连接 Telegram")
			return
		}
		var out json.RawMessage
		if err := c.Call(m, in.Params, &out); err != nil {
			ApiErr(w, 502, tgErr(err))
			return
		}
		_ = st.AddAudit(store.AuditRec{
			Kind: "manual", Username: "网页后台", Hit: "Bot API: " + m,
			Actions: `["botapi"]`, Result: "ok",
		})
		ApiOK(w, map[string]interface{}{"method": m, "result": out})

	// ---- 导出 / 重载 / 帮助 ----
	case "export":
		buf, err := st.ExportJSON()
		if err != nil {
			ApiErr(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="LianT-config.json"`)
		_, _ = w.Write(buf)
	case "reload":
		_ = b.ReloadAll()
		ApiOK(w, "已重载规则缓存与 Token，已请求重连 Telegram")
	case "help":
		ApiOK(w, map[string]interface{}{"commands": bot.HelpText(), "actions": "禁言/解禁/踢出/封禁/撤回/规则/违禁词/管理员"})
	default:
		ApiErr(w, http.StatusNotFound, "未知接口: "+route)
	}
}

// validAPIMethod 只允许字母数字，避免 method 被拼进 URL 做路径注入。
func validAPIMethod(m string) bool {
	if m == "" {
		return false
	}
	for _, ch := range m {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}

func OrDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// tgErr 把 Telegram 错误翻成人话（带中文提示）。
func tgErr(err error) string {
	if err == nil {
		return ""
	}
	var ae *telegram.APIError
	if ok := asAPIError(err, &ae); ok {
		if h := ae.Hint(); h != "" {
			return ae.Description + "（" + h + "）"
		}
		return ae.Description
	}
	return err.Error()
}

func asAPIError(err error, target **telegram.APIError) bool {
	for err != nil {
		if e, ok := err.(*telegram.APIError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// ServeUI 输出管理页面（token 从 URL 带进来时会写 cookie）。
func (a *Server) ServeUI(w http.ResponseWriter, r *http.Request) {
	if t := strings.TrimSpace(r.URL.Query().Get("token")); t != "" && a.token != "" {
		if subtle.ConstantTimeCompare([]byte(t), []byte(a.token)) == 1 {
			http.SetCookie(w, &http.Cookie{
				Name: "LianT_admin_token", Value: t, Path: "/", HttpOnly: true, Secure: secureRequest(r),
				SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600,
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
	page := strings.ReplaceAll(html, "__BASE_PATH__", jsString(a.BasePath(r)))
	_, _ = w.Write([]byte(page))
}

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

(() => {
  "use strict";

  /* ======================================================================
     常量与状态
     ====================================================================== */
  const BASE = "__BASE_PATH__";
  const TOKEN_KEY = "liant_token";
  const GROUP_KEY = "liant_gid";
  const CHAT_GROUP_KEY = "liant_chat_group";
  const CHAT_FORMAT_KEY = "liant_chat_format";

  const TABS = [
    { id: "overview", label: "总览", icon: "grid", desc: "任务进程、数据规模与最近操作。" },
    { id: "chat", label: "消息", icon: "send", desc: "完整聊天记录、媒体与发送。" },
    { id: "rules", label: "回复规则", icon: "message", desc: "按优先级匹配消息并自动回复，命中第一条即停止。" },
    { id: "words", label: "违禁词", icon: "shield", desc: "命中后撤回、禁言、踢出或拉黑；优先级数字小的先匹配。" },
    { id: "joins", label: "入群申请", icon: "user-plus", desc: "手动审批入群申请，自动通过由设置控制。" },
    { id: "groups", label: "群", icon: "users", desc: "每个群的开关、权限与机器人身份。" },
    { id: "admins", label: "管理员", icon: "key", desc: "决定谁能在群里使用中文指令。" },
    { id: "mutes", label: "禁言", icon: "clock", desc: "本地禁言记录与平台侧状态，最长 30 天。" },
    { id: "blacklist", label: "黑名单", icon: "ban", desc: "本地立即生效；平台侧同步需要相应权限。" },
    { id: "bots", label: "所有 Bot", icon: "bot", desc: "选择要进入的 Bot，或添加一个新的。" },
    { id: "audit", label: "操作日志", icon: "list", desc: "谁、在哪个群、对谁做了什么。" },
    { id: "qq", label: "手动操作", icon: "terminal", desc: "直接调用平台接口，所有操作写入日志。" },
    { id: "settings", label: "设置", icon: "settings", desc: "回复策略、入群审批与配置导出。" }
  ];
  const TAB_MAP = Object.fromEntries(TABS.map(t => [t.id, t]));
  const NAV_GROUPS = [
    ["会话", ["chat"]],
    ["控制台", ["overview"]],
    ["运营", ["rules", "words", "joins"]],
    ["社群", ["groups", "admins", "mutes", "blacklist"]],
    ["系统", ["audit", "qq", "settings"]]
  ];
  const MATCHES = [
    ["exact", "精确匹配"],
    ["prefix", "前缀匹配"],
    ["contains", "包含"],
    ["regex", "正则"],
    ["any", "任意消息"]
  ];
  const MATCH_LABEL = Object.fromEntries(MATCHES);
  const GROUP_FIELD = {
    Enabled: "enabled",
    ReplyAll: "reply_all",
    CommandsEnabled: "commands_enabled",
    GroupAdminCanCmd: "group_admin_can_command",
    BotIsAdmin: "bot_is_admin"
  };
  const KIND_LABEL = {
    push: "群发", join: "入群", manual: "手动", admin: "管理", message: "消息",
    reply: "回复", word: "违禁词", blacklist: "黑名单"
  };
  const MEDIA_KIND_LABEL = {
    image: "图片", video: "视频", audio: "音频", voice: "语音",
    file: "文件", sticker: "表情", animation: "动图", video_note: "视频留言"
  };
  const ROLE_LABEL = { super: "超级管理员", admin: "管理员", owner: "群主", member: "成员" };
  const ACTION_LABEL = {
    kick: "踢出", recall: "撤回", mute: "禁言", unmute: "解禁", blacklist: "拉黑",
    send: "群发", join: "审批", reply: "回复", warn: "警告", rename: "改群名"
  };
  const CARD_TEMPLATES = [
    { id: "notice", label: "群公告", title: "群公告", content: "**请注意**\n\n- 文明发言，遵守群规\n- 广告与刷屏会被处理\n\n有问题可以 @管理员。" },
    { id: "maintenance", label: "维护通知", title: "维护通知", content: "群助手将在 **今晚 23:00** 进行一次维护，期间可能短暂无法响应，请见谅。" },
    { id: "event", label: "活动提醒", title: "活动提醒", content: "**活动开始啦**\n\n- 时间：本周六 20:00\n- 地点：本群\n\n回复「参加」即可报名。" },
    { id: "welcome", label: "欢迎语", title: "欢迎新成员", content: "欢迎加入本群 👋\n\n- 先看群公告\n- 有问题直接 @管理员\n- 机器人指令发「帮助」" }
  ];

  const store = {
    get(key, fallback = "") {
      try { const v = localStorage.getItem(key); return v === null ? fallback : v; } catch { return fallback; }
    },
    set(key, value) { try { localStorage.setItem(key, value); } catch {} },
    del(key) { try { localStorage.removeItem(key); } catch {} }
  };

  const state = {
    tab: "overview",
    token: "",
    bots: [],
    platforms: [],
    botID: store.get("liant_bot", ""),
    groups: [],
    members: [],
    membersLoaded: false,
    rules: [],
    words: [],
    admins: [],
    mutes: [],
    blacklist: [],
    seq: 0,
    lastSync: "",
    groupSyncTried: false,
    groupSyncBusy: false,
    groupSyncDenied: false,
    qqMessages: [],
    chat: {
      groups: [],
      messages: [],
      group: store.get(CHAT_GROUP_KEY, ""),
      format: store.get(CHAT_FORMAT_KEY, "text") === "card" ? "card" : "text",
      draft: { title: "", content: "" },
      error: "",
      timer: 0,
      refreshing: false
    }
  };

  const view = document.getElementById("view");
  const $ = (sel, root = document) => root.querySelector(sel);

  /* ======================================================================
     基础工具
     ====================================================================== */
  const ESC = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
  const esc = value => String(value == null ? "" : value).replace(/[&<>"']/g, c => ESC[c]);
  const icon = (name, cls = "i") => `<svg class="${cls}" aria-hidden="true"><use href="#i-${name}"></use></svg>`;

  const NUM = new Intl.NumberFormat("zh-CN");
  const DATE_TIME = new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
  const DATE_ONLY = new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" });
  const TIME_ONLY = new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });

  const fmtNum = value => NUM.format(Number(value) || 0);
  const fmtBytes = value => {
    const n = Number(value) || 0;
    if (n <= 0) return "";
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(n < 10 * 1024 ? 1 : 0) + " KB";
    return (n / 1024 / 1024).toFixed(n < 10 * 1024 * 1024 ? 1 : 0) + " MB";
  };
  const truncate = (value, max = 48) => {
    const s = String(value == null ? "" : value);
    return s.length > max ? s.slice(0, Math.max(1, max - 1)) + "…" : s;
  };
  const initial = value => {
    const s = String(value || "").trim();
    return s ? Array.from(s)[0] : "群";
  };
  const avatarClass = seed => {
    const text = String(seed || "群");
    let hash = 0;
    for (let i = 0; i < text.length; i++) hash = (hash * 31 + text.charCodeAt(i)) >>> 0;
    return "av-" + (hash % 8);
  };
  const fmtDateTime = value => {
    if (!value) return "";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return String(value).replace("T", " ").replace(/\.\d+.*$/, "");
    return DATE_TIME.format(d).replace(/\//g, "-");
  };
  const fmtDate = value => {
    if (!value) return "";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return String(value).slice(0, 10);
    return DATE_ONLY.format(d).replace(/\//g, "-");
  };
  const fmtTime = value => {
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? "" : TIME_ONLY.format(d);
  };
  const fmtRelative = value => {
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return "";
    const diff = Date.now() - d.getTime();
    if (diff < 60000) return "刚刚";
    if (diff < 3600000) return Math.floor(diff / 60000) + " 分钟前";
    if (diff < 86400000) return Math.floor(diff / 3600000) + " 小时前";
    if (diff < 7 * 86400000) return Math.floor(diff / 86400000) + " 天前";
    return fmtDate(value);
  };
  const fmtDuration = value => {
    if (!value) return "—";
    return String(value)
      .replace(/([\d.]+)h/, "$1 小时 ")
      .replace(/([\d.]+)m/, "$1 分 ")
      .replace(/([\d.]+)s/, "$1 秒")
      .trim();
  };
  const fmtSeconds = value => {
    const n = Number(value) || 0;
    if (n <= 0) return "—";
    if (n % 86400 === 0) return n / 86400 + " 天";
    if (n % 3600 === 0) return n / 3600 + " 小时";
    if (n % 60 === 0) return n / 60 + " 分钟";
    return n + " 秒";
  };
  const plainPreview = (value, max = 26) => {
    const s = String(value || "")
      .replace(/^#{1,6}\s+/gm, "")
      .replace(/[*_~>`#]/g, "")
      .replace(/\s+/g, " ")
      .trim();
    return truncate(s, max);
  };

  const botTone = value => {
    const s = String(value || "");
    if (!s) return "muted";
    if (s.includes("已连接")) return "ok";
    if (s.includes("连接") || s.includes("启动") || s.includes("等待")) return "warn";
    if (s.includes("断开") || s.includes("错误") || s.includes("停止") || s.includes("失败")) return "danger";
    return "";
  };
  const TONE_LABEL = { ok: "运行中", warn: "连接中", danger: "异常", muted: "未知", "": "状态" };

  /* ======================================================================
     网络层
     ====================================================================== */
  class ApiError extends Error {
    constructor(message, status = 0) {
      super(message);
      this.name = "ApiError";
      this.status = status;
    }
  }

  function withBot(path) {
    if (!state.botID || path.startsWith("bots") || path.startsWith("platforms")) return path;
    return path + (path.includes("?") ? "&" : "?") + "bot_id=" + encodeURIComponent(state.botID);
  }

  async function api(path, opts = {}) {
    const controller = new AbortController();
    const timeout = Number(opts.timeout) || 15000;
    const timer = setTimeout(() => controller.abort(), timeout);
    const headers = {};
    if (state.token) headers.Authorization = "Bearer " + state.token;
    let body;
    if (opts.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(opts.body);
    }
    try {
      const res = await fetch(BASE + "/api/" + withBot(path), {
        method: opts.method || "GET",
        headers,
        body,
        credentials: "same-origin",
        cache: "no-store",
        signal: controller.signal
      });
      const text = await res.text();
      let json = null;
      try { json = text ? JSON.parse(text) : null; } catch {}
      if (res.status === 401) throw new ApiError((json && json.error) || "未授权，请检查管理端 token", 401);
      if (!json) throw new ApiError("响应不是 JSON（HTTP " + res.status + "）", res.status);
      if (!json.ok) throw new ApiError(json.error || ("请求失败（HTTP " + res.status + "）"), res.status);
      return json.data;
    } catch (err) {
      if (err && err.name === "AbortError") throw new ApiError("请求超时，请检查网络或任务进程状态", 0);
      if (err instanceof ApiError) throw err;
      throw new ApiError("无法连接管理端：" + (err && err.message ? err.message : String(err)), 0);
    } finally {
      clearTimeout(timer);
    }
  }

  async function groupsSafe(force = false) {
    if (!force && state.groups.length) return state.groups;
    const list = await api("groups");
    state.groups = Array.isArray(list) ? list : [];
    return state.groups;
  }

  const groupLabel = id => {
    if (!id) return "全部群";
    const g = state.groups.find(x => x.OpenID === id);
    return g && g.Name ? g.Name : "未知群";
  };
  const groupOptionLabel = g => (g.Name || "未命名群") + (g.Enabled ? "" : "（已停用）");
  const roleLabel = role => ROLE_LABEL[role] || role || "成员";
  const humanActor = value => {
    const v = String(value || "").trim();
    if (!v) return "—";
    if (v === "web") return "网页";
    if (v === "system") return "系统";
    if (v === "manual") return "手动";
    return v;
  };
  const memberName = id => {
    if (!id) return "—";
    const m = state.members.find(item => item.MemberOpenID === id);
    return m && m.Username ? m.Username : "未知成员";
  };
  const humanize = text => {
    let out = String(text == null ? "" : text);
    if (!out) return "";
    if (!/[A-Za-z0-9_-]{16,}/.test(out)) return out;
    state.groups.forEach(g => {
      if (g.OpenID && g.Name) out = out.split(g.OpenID).join(g.Name);
    });
    state.members.forEach(m => {
      if (m.MemberOpenID && m.Username) out = out.split(m.MemberOpenID).join(m.Username);
    });
    // 仍未识别的 QQ 消息 ID / 引用 ID / 32 位 openid 不再展示。
    return out
      .replace(/\bROBOT1\.[0-9A-Za-z_-]{8,}\b/g, "…")
      .replace(/\bREFIDX_[0-9A-Za-z_=-]{8,}\b/g, "…")
      .replace(/\b[0-9A-Fa-f]{32}\b/g, "…");
  };
  async function loadMembers(force = false) {
    if (!force && state.membersLoaded) return state.members;
    const list = await api("members");
    state.members = Array.isArray(list) ? list.filter(item => !item.Bot) : [];
    state.membersLoaded = true;
    return state.members;
  }
  function groupSelectHTML(name = "group_openid", opts = {}) {
    const selected = opts.selected || "";
    const list = state.groups.slice().sort((a, b) => String(a.Name || "").localeCompare(String(b.Name || ""), "zh-CN"));
    const first = opts.placeholder || (list.length ? "选择群…" : "还没有登记群");
    const options = [`<option value=""${selected ? "" : " selected"}>${esc(first)}</option>`];
    list.forEach(g => {
      const label = groupOptionLabel(g);
      options.push(`<option value="${esc(g.OpenID)}"${g.OpenID === selected ? " selected" : ""}>${esc(label)}</option>`);
    });
    return `<select class="input select" name="${esc(name)}"${opts.required ? " required" : ""}${opts.attrs || ""}>${options.join("")}</select>`;
  }
  function memberSelectHTML(name = "member_openid", opts = {}) {
    const selected = opts.selected || "";
    const groups = new Map();
    state.members.forEach(member => {
      if (member.Bot) return;
      const gid = member.GroupOpenID || "";
      if (!groups.has(gid)) groups.set(gid, []);
      groups.get(gid).push(member);
    });
    const keys = Array.from(groups.keys()).sort((a, b) => groupLabel(a).localeCompare(groupLabel(b), "zh-CN"));
    const options = [`<option value="">${state.members.length ? "选择成员…" : "名册里还没有成员"}</option>`];
    keys.forEach(gid => {
      options.push(`<optgroup label="${esc(groupLabel(gid))}">`);
      groups.get(gid)
        .slice()
        .sort((a, b) => String(a.Username || "").localeCompare(String(b.Username || ""), "zh-CN"))
        .forEach(member => {
          const role = member.Role === "owner" ? "群主" : member.Role === "admin" ? "管理员" : "";
          const label = (member.Username || "未命名成员") + (role ? "（" + role + "）" : "");
          options.push(`<option value="${esc(member.MemberOpenID)}" data-group="${esc(gid)}" data-name="${esc(member.Username || "")}"${member.MemberOpenID === selected ? " selected" : ""}>${esc(label)}</option>`);
        });
      options.push("</optgroup>");
    });
    return `<select class="input select" name="${esc(name)}" aria-label="成员" data-member-select${opts.autoGroup ? ' data-auto-group="1"' : ""}${opts.required ? " required" : ""}>${options.join("")}</select>
      <details class="manual-input"><summary>成员不在名册里？手动输入 ID</summary>
        <input class="input mono" name="member_manual" aria-label="成员 ID" placeholder="成员 ID（手动输入）" autocomplete="off" spellcheck="false">
      </details>`;
  }
  function memberFromForm(fd) {
    const manual = String(fd.get("member_manual") || "").trim();
    if (manual) return manual;
    return String(fd.get("member_openid") || "").trim();
  }

  async function copyText(value) {
    const text = String(value == null ? "" : value);
    if (!text) return;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
      } else {
        const area = document.createElement("textarea");
        area.value = text;
        area.setAttribute("readonly", "");
        area.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
        document.body.append(area);
        area.select();
        const ok = document.execCommand("copy");
        area.remove();
        if (!ok) throw new Error("浏览器拒绝了复制");
      }
      toast.success("已复制");
    } catch (err) {
      toast.error("复制失败：" + (err.message || "浏览器不允许"));
    }
  }

  /* ======================================================================
     连接状态 / 加载状态
     ====================================================================== */
  function setConn(kind, text) {
    const pill = document.getElementById("status-pill");
    const label = document.getElementById("status-text");
    if (pill) { pill.dataset.state = kind; pill.title = text; }
    if (label) label.textContent = text;
    const conn = document.getElementById("conn");
    const connText = document.getElementById("conn-text");
    if (conn) conn.dataset.state = kind;
    if (connText) connText.textContent = kind === "ok" && state.lastSync ? "已连接 · " + state.lastSync : text;
  }

  function markSynced() {
    state.lastSync = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    setConn("ok", "已连接");
  }

  function setBusy(on) {
    const progress = document.getElementById("progress");
    if (progress) progress.hidden = !on;
  }

  function skeleton(meta) {
    const head = `<div class="page-head"><div><div class="sk sk-title"></div><div class="sk sk-sub"></div></div></div>`;
    if (meta.id === "overview") {
      return head + `<div class="stat-grid">${'<div class="sk sk-stat"></div>'.repeat(6)}</div><div class="grid two"><div class="sk sk-card"></div><div class="sk sk-card"></div></div>`;
    }
    if (meta.id === "chat") {
      return `<div class="chat-app chat-app-skeleton">
        <aside class="chat-side"><div class="sk sk-chat-title"></div><div class="sk sk-chat-row"></div><div class="sk sk-chat-row"></div><div class="sk sk-chat-row"></div></aside>
        <section class="chat-main"><div class="sk sk-chat-head"></div><div class="sk sk-chat-body"></div><div class="sk sk-chat-composer"></div></section>
      </div>`;
    }
    return head + `<div class="card sk-table"><div class="sk sk-bar"></div>${'<div class="sk sk-row"></div>'.repeat(6)}</div>`;
  }

  function authHTML(message) {
    return `<div class="state-wrap"><div class="card state-card">
      <div class="state-icon">${icon("lock")}</div>
      <h1>连接管理端</h1>
      <p>${esc(message || "这个页面需要管理端 token。运行 LianT admin 可以看到地址和 token。")}</p>
      <form class="auth-form" data-form="auth">
        <label class="field"><span class="field-label">管理端 token</span>
          <input class="input mono" name="token" type="password" autocomplete="off" spellcheck="false" placeholder="LIANT_ADMIN_TOKEN" value="${esc(state.token)}">
        </label>
        <button class="btn primary" type="submit">连接</button>
      </form>
      <p class="state-hint">token 会写进 HttpOnly cookie，不保存在 localStorage；也可以在地址后加 <code>?token=...</code> 打开一次。</p>
    </div></div>`;
  }

  function errorHTML(err) {
    const msg = err && err.message ? err.message : String(err || "未知错误");
    return `<div class="state-wrap"><div class="card state-card">
      <div class="state-icon danger">${icon("alert")}</div>
      <h1>加载失败</h1>
      <p>${esc(msg)}</p>
      <div class="state-actions">
        <button class="btn primary" type="button" data-act="retry">重试</button>
        <button class="btn" type="button" data-act="open-token">连接设置</button>
      </div>
      <p class="state-hint">如果任务进程刚启动，稍等片刻再试；也可以查看 <code>LianT-data</code> 里的日志。</p>
    </div></div>`;
  }

  /* ======================================================================
     导航 / 路由
     ====================================================================== */
  function buildNav() {
    const nav = document.getElementById("nav");
    nav.innerHTML = NAV_GROUPS.map(([title, ids]) => `<div class="nav-group">
      <p class="nav-title">${esc(title)}</p>
      ${ids.map(id => {
        const tab = TAB_MAP[id];
        return `<a class="nav-link" href="#${id}" data-tab="${id}">${icon(tab.icon)}<span>${esc(tab.label)}</span></a>`;
      }).join("")}
    </div>`).join("");
  }

  function syncNav() {
    document.querySelectorAll(".nav-link").forEach(link => {
      if (link.dataset.tab === state.tab) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
  }

  function tabFromHash() {
    const id = String(location.hash || "").replace(/^#/, "");
    return TAB_MAP[id] ? id : "bots";
  }

  async function enterBot(id) {
    const bot = botByID(id);
    if (bot && !bot.running) toast("这个 Bot 未运行；可在工作台里重启或编辑配置", { type: "info", duration: 5000 });
    state.botID = String(id || "");
    if (state.botID) store.set("liant_bot", state.botID);
    state.tab = "chat";
    updateBotSelect();
    if (location.hash === "#chat") await loadCurrent();
    else location.hash = "#chat";
  }

  /* ======================================================================
     加载主流程
     ====================================================================== */
  async function loadCurrent(opts = {}) {
    let tab = state.tab;
    if (tab !== "bots" && !state.botID) {
      tab = "bots";
      state.tab = tab;
      if (location.hash !== "#bots") history.replaceState(null, "", "#bots");
    }
    const meta = TAB_MAP[tab] || TAB_MAP.overview;
    const my = ++state.seq;
    stopChatPoll();
    const home = tab === "bots";
    document.getElementById("app").classList.toggle("is-home", home);
    document.getElementById("app").classList.toggle("chat-mode", tab === "chat");
    document.getElementById("bot-context").hidden = home;
    document.getElementById("crumb-bot").textContent = home ? "选择 Bot" : (botByID(state.botID) || {}).name || "Bot";
    document.getElementById("crumb-tab").textContent = meta.label;
    document.title = (home ? "选择 Bot" : meta.label) + " · LianT";
    syncNav();
    if (!home) renderBotContext();
    const changed = view.dataset.tab !== tab;
    if (changed && !opts.quiet) {
      view.innerHTML = skeleton(meta);
      view.dataset.tab = "";
      window.scrollTo(0, 0);
    }
    setBusy(true);
    setConn("loading", "连接中");
    try {
      const html = await RENDER[tab]();
      if (my !== state.seq) return;
      view.innerHTML = html;
      view.dataset.tab = tab;
      markSynced();
      afterRender(tab);
    } catch (err) {
      if (my !== state.seq) return;
      handleLoadError(err);
    } finally {
      if (my === state.seq) setBusy(false);
    }
  }

  function afterRender(tab) {
    if (tab === "chat") {
      const timeline = document.getElementById("chat-timeline");
      if (timeline) {
        timeline.scrollTop = timeline.scrollHeight;
        // 图片/视频是后加载的，布局变高后再贴一次底。
        setTimeout(() => {
          if (state.tab === "chat" && state.chat.group) timeline.scrollTop = timeline.scrollHeight;
        }, 260);
      }
      startChatPoll();
    }
    if ((tab === "groups" || tab === "chat") && !state.groupSyncTried && !state.groupSyncDenied) {
      const list = tab === "chat" ? state.chat.groups : state.groups;
      const hasUnnamed = Array.isArray(list) && list.some(group => !String(tab === "chat" ? group.name : group.Name || "").trim());
      if (hasUnnamed) {
        state.groupSyncTried = true;
        setTimeout(() => {
          if (state.tab === tab) syncGroupNames({ silent: true });
        }, 400);
      }
    }
  }

  function handleLoadError(err) {
    if (err && err.status === 401) {
      view.innerHTML = authHTML(err.message);
      view.dataset.tab = "";
      setConn("auth", "需要 token");
      return;
    }
    view.innerHTML = errorHTML(err);
    view.dataset.tab = "";
    setConn("error", "连接失败");
  }

  /* ======================================================================
     通知 / 确认框 / 表单弹窗
     ====================================================================== */
  const TOAST_ICONS = { ok: "check", error: "alert", info: "info" };

  function toast(message, opts = {}) {
    const type = opts.type === "error" ? "error" : opts.type === "ok" ? "ok" : "info";
    const box = document.getElementById("toasts");
    if (!box) return;
    while (box.children.length >= 4) box.firstElementChild.remove();
    const node = document.createElement("div");
    node.className = "toast toast-" + type;
    node.setAttribute("role", type === "error" ? "alert" : "status");
    node.innerHTML = `<span class="toast-icon">${icon(TOAST_ICONS[type])}</span><p>${esc(message)}</p><button class="toast-close" type="button" aria-label="关闭通知">${icon("x")}</button>`;
    node.classList.add("is-enter");
    box.append(node);
    setTimeout(() => node.classList.remove("is-enter"), 20);

    let remaining = Number(opts.duration) || (type === "error" ? 6000 : 3600);
    let started = Date.now();
    let timer = 0;
    let closed = false;
    const dismiss = () => {
      if (closed) return;
      closed = true;
      node.classList.add("is-enter");
      setTimeout(() => node.remove(), 220);
    };
    const arm = () => {
      if (closed) return;
      started = Date.now();
      timer = setTimeout(dismiss, Math.max(600, remaining));
    };
    const pause = () => {
      clearTimeout(timer);
      remaining -= Date.now() - started;
    };
    node.addEventListener("pointerenter", pause);
    node.addEventListener("pointerleave", arm);
    node.querySelector(".toast-close").addEventListener("click", dismiss);
    arm();
  }
  toast.success = (message, opts) => toast(message, { ...opts, type: "ok" });
  toast.error = (message, opts) => toast(message, { ...opts, type: "error" });

  function syncScrollLock() {
    const open = document.querySelector("dialog[open]");
    document.documentElement.classList.toggle("modal-lock", Boolean(open));
  }

  function confirmAction(opts = {}) {
    const dialog = document.getElementById("confirm");
    if (!dialog) return Promise.resolve(false);
    const ok = document.getElementById("confirm-ok");
    const iconBox = document.getElementById("confirm-icon");
    document.getElementById("confirm-title").textContent = opts.title || "确认操作";
    document.getElementById("confirm-msg").textContent = opts.message || "";
    ok.textContent = opts.confirmText || "确认";
    ok.className = "btn " + (opts.tone === "primary" ? "primary" : "danger");
    iconBox.className = "confirm-icon" + (opts.tone === "primary" ? " primary" : "");
    iconBox.innerHTML = icon(opts.tone === "primary" ? "check" : "alert");
    if (dialog.open) dialog.close();
    dialog.returnValue = "cancel";
    return new Promise(resolve => {
      const onClose = () => {
        dialog.removeEventListener("close", onClose);
        resolve(dialog.returnValue === "confirm");
      };
      dialog.addEventListener("close", onClose);
      dialog.showModal();
      syncScrollLock();
      ok.focus();
    });
  }

  function openForm({ title, desc = "", body = "", submitText = "保存", onSubmit }) {
    const dialog = document.getElementById("form-modal");
    dialog.innerHTML = `<form class="modal-form" id="app-form" novalidate>
      <header class="modal-head">
        <div><h2 id="form-title">${esc(title)}</h2>${desc ? `<p>${esc(desc)}</p>` : ""}</div>
        <button class="icon-btn" type="button" data-close aria-label="关闭">${icon("x")}</button>
      </header>
      <div class="modal-body"><div class="form-grid">${body}</div></div>
      <footer class="modal-foot">
        <button class="btn" type="button" data-close>取消</button>
        <button class="btn primary" type="submit">${esc(submitText)}</button>
      </footer>
    </form>`;
    if (!dialog.open) dialog.showModal();
    syncScrollLock();
    const form = dialog.querySelector("#app-form");
    const submit = form.querySelector('button[type="submit"]');
    dialog.querySelectorAll("[data-close]").forEach(btn => btn.addEventListener("click", () => dialog.close()));
    form.addEventListener("submit", async event => {
      event.preventDefault();
      if (submit.dataset.busy) return;
      if (!form.reportValidity()) return;
      submit.dataset.busy = "1";
      submit.setAttribute("aria-busy", "true");
      try {
        const message = await onSubmit(form);
        dialog.close();
        if (message) toast.success(message);
      } catch (err) {
        toast.error(err && err.message ? err.message : String(err));
      } finally {
        delete submit.dataset.busy;
        submit.removeAttribute("aria-busy");
      }
    });
    dialog.addEventListener("close", () => {
      dialog.innerHTML = "";
      syncScrollLock();
    }, { once: true });
    requestAnimationFrame(() => {
      const first = form.querySelector("input:not([type=hidden]):not([disabled]), select, textarea");
      if (first) {
        first.focus();
        if (first.tagName === "INPUT" && first.select) first.select();
      }
    });
  }

  function openToken() {
    const dialog = document.getElementById("token-modal");
    const input = document.getElementById("token-input");
    input.value = state.token || "";
    if (!dialog.open) dialog.showModal();
    syncScrollLock();
    requestAnimationFrame(() => {
      input.focus();
      input.select();
    });
  }

  /* ======================================================================
     展示片段
     ====================================================================== */
  function statCard({ label, value, hint, icon: iconName, href }) {
    return `<a class="stat" href="${esc(href)}">
      <div class="stat-top"><span class="stat-icon">${icon(iconName)}</span><span>${esc(label)}</span></div>
      <div class="stat-value">${fmtNum(value)}</div>
      <div class="stat-hint">${esc(hint)}</div>
      <span class="stat-arrow">${icon("chevron-right")}</span>
    </a>`;
  }

  function metric(label, value) {
    return `<div><div class="metric-label">${esc(label)}</div><div class="metric-value">${esc(value == null || value === "" ? "—" : value)}</div></div>`;
  }

  function addrRow(label, value) {
    const v = value == null || value === "" ? "—" : String(value);
    return `<div class="addr">
      <span class="addr-label">${esc(label)}</span>
      <code title="${esc(v)}">${esc(v)}</code>
      <button class="icon-btn sm" type="button" data-act="copy" data-copy="${esc(v)}" aria-label="复制${esc(label)}" title="复制">${icon("copy")}</button>
    </div>`;
  }

  function kindBadge(kind) {
    const key = String(kind || "");
    const label = KIND_LABEL[key] || key || "—";
    const cls = key === "push" ? " accent" : key === "join" ? " warn" : "";
    return `<span class="badge${cls}">${esc(label)}</span>`;
  }

  function resultBadge(value) {
    const text = String(value || "");
    if (!text) return "—";
    const low = text.toLowerCase();
    let cls = "";
    let label = text;
    if (low === "ok" || low.includes("success") || text.includes("成功")) { cls = "ok"; label = "成功"; }
    else if (low === "fail" || low.includes("error") || text.includes("失败")) { cls = "danger"; label = "失败"; }
    else if (text.includes("待")) { cls = "warn"; label = text; }
    return `<span class="badge ${cls}">${esc(label)}</span>`;
  }

  function actionChips(raw) {
    if (!raw) return "—";
    let list = null;
    try {
      const value = JSON.parse(raw);
      if (Array.isArray(value)) list = value;
    } catch {}
    if (!list || !list.length) return `<span class="mono">${esc(raw)}</span>`;
    return `<div class="chip-row">${list.map(item => {
      const key = String(item);
      return `<span class="badge">${esc(ACTION_LABEL[key] || key)}</span>`;
    }).join("")}</div>`;
  }

  function pageHead(meta, actions = "") {
    return `<div class="page-head">
      <div><h1>${esc(meta.label)}</h1><p>${esc(meta.desc)}</p></div>
      ${actions ? `<div class="page-actions">${actions}</div>` : ""}
    </div>`;
  }

  function cardToolbar({ placeholder = "搜索…", count = 0, extra = "" }) {
    return `<div class="card-toolbar">
      <label class="search">${icon("search")}<input class="input" type="search" data-filter placeholder="${esc(placeholder)}" aria-label="${esc(placeholder)}"></label>
      <span class="toolbar-count"><span data-count>${count}</span> 条</span>
      ${extra}
    </div>`;
  }

  function dataTable(cols, rows, opts = {}) {
    return `<div class="table-wrap"><table class="data${opts.wide ? " wide" : ""}">
      <thead><tr>${cols.map(c => `<th scope="col"${c.cls ? ` class="${c.cls}"` : ""}>${esc(c.label)}</th>`).join("")}</tr></thead>
      <tbody>${rows}
        <tr data-no-results hidden><td class="no-results" colspan="${cols.length}">没有匹配的结果</td></tr>
      </tbody>
    </table></div>`;
  }

  function emptyState({ icon: iconName = "info", title, text = "", action = "" }) {
    return `<div class="empty">
      <div class="empty-icon">${icon(iconName)}</div>
      <h3>${esc(title)}</h3>
      ${text ? `<p>${esc(text)}</p>` : ""}
      ${action}
    </div>`;
  }

  function fieldHTML(label, control, hint = "", extra = "") {
    return `<label class="field${extra ? " " + extra : ""}">
      <span class="field-label">${esc(label)}</span>
      ${control}
      ${hint ? `<span class="field-hint">${esc(hint)}</span>` : ""}
    </label>`;
  }

  function fieldBlock(label, control, hint = "", extra = "") {
    return `<div class="field${extra ? " " + extra : ""}">
      <span class="field-label">${esc(label)}</span>
      ${control}
      ${hint ? `<span class="field-hint">${esc(hint)}</span>` : ""}
    </div>`;
  }

  function inputHTML(name, opts = {}) {
    const attrs = [
      `class="input${opts.mono ? " mono" : ""}"`,
      `name="${esc(name)}"`,
      `type="${esc(opts.type || "text")}"`,
      `value="${esc(opts.value == null ? "" : opts.value)}"`
    ];
    if (opts.placeholder) attrs.push(`placeholder="${esc(opts.placeholder)}"`);
    if (opts.required) attrs.push("required");
    if (opts.min !== undefined) attrs.push(`min="${esc(opts.min)}"`);
    if (opts.max !== undefined) attrs.push(`max="${esc(opts.max)}"`);
    if (opts.step !== undefined) attrs.push(`step="${esc(opts.step)}"`);
    if (opts.maxlength) attrs.push(`maxlength="${esc(opts.maxlength)}"`);
    if (opts.autocomplete === false) attrs.push('autocomplete="off"');
    if (opts.spellcheck === false) attrs.push('spellcheck="false"');
    if (opts.autofocus) attrs.push("autofocus");
    return `<input ${attrs.join(" ")}>`;
  }

  function textareaHTML(name, opts = {}) {
    const attrs = [`class="input textarea"`, `name="${esc(name)}"`, `rows="${esc(opts.rows || 3)}"`];
    if (opts.placeholder) attrs.push(`placeholder="${esc(opts.placeholder)}"`);
    if (opts.required) attrs.push("required");
    if (opts.maxlength) attrs.push(`maxlength="${esc(opts.maxlength)}"`);
    return `<textarea ${attrs.join(" ")}>${esc(opts.value == null ? "" : opts.value)}</textarea>`;
  }

  function selectHTML(name, options, selected = "") {
    return `<select class="input select" name="${esc(name)}">${options.map(([value, label]) => {
      return `<option value="${esc(value)}"${String(value) === String(selected) ? " selected" : ""}>${esc(label)}</option>`;
    }).join("")}</select>`;
  }

  function switchHTML(name, label, hint, checked) {
    return `<label class="check">
      <input class="sw" type="checkbox" name="${esc(name)}"${checked ? " checked" : ""}>
      <span class="check-text"><span>${esc(label)}</span>${hint ? `<small>${esc(hint)}</small>` : ""}</span>
    </label>`;
  }

  function groupOptions(selected, allLabel = "") {
    const out = [];
    if (allLabel) out.push(`<option value=""${selected ? "" : " selected"}>${esc(allLabel)}</option>`);
    if (selected && !state.groups.some(g => g.OpenID === selected)) {
      out.push(`<option value="${esc(selected)}" selected>未知群</option>`);
    }
    state.groups.forEach(g => {
      out.push(`<option value="${esc(g.OpenID)}"${g.OpenID === selected ? " selected" : ""}>${esc(groupOptionLabel(g))}</option>`);
    });
    return out.join("");
  }

  /* ======================================================================
     视图：机器人（多 Bot 注册表）
     ====================================================================== */
  function botByID(id) {
    return state.bots.find(b => String(b.id) === String(id)) || null;
  }
  function botStateTone(b) {
    if (!b) return "muted";
    if (b.online || b.state === "在线" || b.state === "已连接") return "ok";
    if (b.state) return "warn";
    if (b.running) return "ok";
    if (b.enabled) return "warn";
    return "muted";
  }
  function botStateText(b) {
    if (!b) return "未选择";
    if (b.state) return b.state;
    if (b.running) return "运行中";
    return b.enabled ? "未运行" : "已停用";
  }
  function updateBotSelect() {
    const sel = document.getElementById("bot-select");
    if (!sel) return;
    if (!state.bots.length) {
      sel.innerHTML = `<option value="">未添加 Bot</option>`;
      sel.disabled = true;
      return;
    }
    sel.disabled = false;
    sel.innerHTML = state.bots.map(b => {
      const name = b.name || ("Bot #" + b.id);
      const who = b.username ? " @" + b.username : "";
      return `<option value="${esc(b.id)}">${esc(name + who)} · ${esc(botStateText(b))}</option>`;
    }).join("");
    sel.value = String(state.botID || "");
  }
  function renderBotContext() {
    const bot = botByID(state.botID);
    const box = document.getElementById("bot-context");
    if (!box || !bot) return;
    const name = bot.name || ("Bot #" + bot.id);
    const avatar = document.getElementById("bot-context-avatar");
    const title = document.getElementById("bot-context-name");
    const meta = document.getElementById("bot-context-meta");
    const dot = document.getElementById("bot-context-dot");
    if (avatar) avatar.textContent = String(name).trim().slice(0, 1).toUpperCase() || "B";
    if (title) title.textContent = name;
    if (meta) meta.textContent = [bot.platform_name || bot.platform, bot.username ? "@" + bot.username : "", botStateText(bot)].filter(Boolean).join(" · ");
    if (dot) dot.className = "bot-context-dot " + botStateTone(bot);
  }

  async function loadBots(preferID = "") {
    const bots = await api("bots");
    state.bots = Array.isArray(bots) ? bots : [];
    if (!state.platforms.length) {
      try { state.platforms = await api("platforms"); } catch { state.platforms = []; }
      if (!Array.isArray(state.platforms)) state.platforms = [];
    }
    let pick = String(preferID || state.botID || store.get("liant_bot", ""));
    let bot = state.bots.find(b => String(b.id) === pick);
    if (!bot) bot = state.bots.find(b => b.online) || state.bots.find(b => b.running) || state.bots.find(b => b.enabled) || state.bots[0];
    state.botID = bot ? String(bot.id) : "";
    if (state.botID) store.set("liant_bot", state.botID);
    else store.del("liant_bot");
    updateBotSelect();
    return state.bots;
  }
  function botFormHTML(bot) {
    const isEdit = Boolean(bot);
    const b = bot || {};
    const platforms = state.platforms.length ? state.platforms : [{ id: "telegram", name: "Telegram" }];
    const platform = (b.platform || "telegram").toLowerCase();
    const isQQ = platform === "qq";
    return `
      ${fieldHTML("平台", selectHTML("platform", platforms.map(p => [p.id, p.name]), platform), "Telegram 用 Bot Token；QQ 用 AppID + AppSecret")}
      ${fieldHTML("显示名称", inputHTML("name", { value: b.name || "", placeholder: "留空自动用平台 Bot 名称", maxlength: 60 }), isEdit ? "留空保留现有名称" : "留空自动使用平台 Bot 名称（QQ 用 AppID）")}
      <div class="span-2" data-cred-wrap>
        <div data-cred="telegram" ${isQQ ? "hidden" : ""}>
          ${fieldHTML("Bot Token", inputHTML("token", { type: "password", value: "", placeholder: isEdit ? "留空 = 不修改" : "123456789:AA…", autocomplete: false }), isEdit ? "留空保留原 token" : "必填；AES-256-GCM 加密存储")}
        </div>
        <div class="form-grid" data-cred="qq" ${isQQ ? "" : "hidden"}>
          ${fieldHTML("AppID", inputHTML("app_id", { value: platform === "qq" ? (b.platform_id || "") : "", placeholder: "QQ 机器人 AppID", mono: true, autocomplete: false }), "QQ 开放平台的机器人 AppID")}
          ${fieldHTML("AppSecret", inputHTML("app_secret", { type: "password", value: "", placeholder: isEdit ? "留空 = 不修改" : "QQ 机器人 AppSecret", autocomplete: false }), isEdit ? "留空保留原 AppSecret" : "必填；AES-256-GCM 加密存储")}
        </div>
      </div>
      <div class="span-2 form-grid" data-tg-only ${isQQ ? "hidden" : ""}>
        ${fieldHTML("收消息方式", selectHTML("mode", [["polling", "长轮询"], ["webhook", "Webhook"]], b.mode || "polling"))}
        ${fieldHTML("Webhook URL", inputHTML("webhook_url", { value: b.webhook_url || "", placeholder: "https://example.com/telegram/webhook", mono: true }), "webhook 模式必填")}
        ${fieldHTML("Webhook secret", inputHTML("webhook_secret", { value: b.webhook_secret || "", placeholder: "A-Za-z0-9_-" }))}
        ${fieldHTML("本地监听", inputHTML("webhook_listen", { value: b.webhook_listen || ("127.0.0.1:" + (28743 + state.bots.length)), mono: true }))}
        ${fieldHTML("Webhook 路径", inputHTML("webhook_path", { value: b.webhook_path || "/telegram/webhook", mono: true }))}
      </div>
      <p class="field-hint span-2" style="margin:0">${isQQ ? "QQ 使用 WebSocket 网关，不需要 webhook。" : "多个 webhook Bot 请使用不同本地端口。"}</p>
      <div class="span-2">${switchHTML("enabled", "启用并启动", "保存后立即启动该 Bot", isEdit ? Boolean(b.enabled) : true)}</div>
    `;
  }

  function openBotForm(bot) {
    const isEdit = Boolean(bot);
    openForm({
      title: isEdit ? "编辑 Bot" : "添加 Bot",
      desc: "凭据加密存入 SQLite；每个 Bot 拥有独立业务库。",
      submitText: isEdit ? "保存并重启" : "添加并启动",
      body: botFormHTML(bot),
      onSubmit: async form => {
        const fd = new FormData(form);
        const platform = String(fd.get("platform") || "telegram").toLowerCase();
        const appID = String(fd.get("app_id") || "").trim();
        const appSecret = String(fd.get("app_secret") || "").trim();
        const token = String(fd.get("token") || "").trim();
        if (isEdit && platform !== String(bot.platform || "")) {
          if (platform === "qq" && (!appID || !appSecret)) throw new Error("切换到 QQ 需要完整填写 AppID 和 AppSecret");
          if (platform !== "qq" && !token) throw new Error("切换到 Telegram 需要填写新的 Bot Token");
        }
        if (platform === "qq") {
          if (!isEdit && (!appID || !appSecret)) throw new Error("QQ 需要填写 AppID 和 AppSecret");
          if (isEdit && appSecret && !appID) throw new Error("填写 AppSecret 时也要填写 AppID");
          if (isEdit && appID !== String(bot.platform_id || "") && !appSecret) throw new Error("修改 AppID 需要同时填写新的 AppSecret");
        } else if (!isEdit && !token) {
          throw new Error("Telegram 需要填写 Bot Token");
        }
        const body = {
          id: isEdit ? bot.id : 0,
          platform,
          name: String(fd.get("name") || "").trim(),
          token: platform === "qq" ? "" : token,
          app_id: platform === "qq" ? appID : "",
          app_secret: platform === "qq" ? appSecret : "",
          mode: String(fd.get("mode") || "polling"),
          webhook_url: String(fd.get("webhook_url") || "").trim(),
          webhook_secret: String(fd.get("webhook_secret") || "").trim(),
          webhook_listen: String(fd.get("webhook_listen") || "").trim(),
          webhook_path: String(fd.get("webhook_path") || "").trim(),
          enabled: fd.get("enabled") === "on"
        };
        const data = await api(isEdit ? "bots/update" : "bots", { method: "POST", body, timeout: 60000 });
        const newID = isEdit ? bot.id : (data && data.id);
        await loadBots(newID);
        await loadCurrent();
        return isEdit ? "Bot 已保存并重启" : "Bot 已添加";
      }
    });
  }

  function syncBotCredentialFields(form, platform) {
    if (!form) return;
    form.querySelectorAll("[data-cred]").forEach(el => {
      el.hidden = el.dataset.cred !== platform;
    });
    form.querySelectorAll("[data-tg-only]").forEach(el => {
      el.hidden = platform !== "telegram";
    });
  }

  async function renderBots() {
    await loadBots();
    const total = state.bots.length;
    const online = state.bots.filter(b => b.online || b.state === "在线" || b.state === "已连接").length;
    const platforms = [...new Set(state.bots.map(b => b.platform_name || b.platform || "telegram"))];
    const cards = state.bots.map(b => {
      const caps = b.capabilities || {};
      const name = b.name || ("Bot #" + b.id);
      const who = b.username ? "@" + b.username : (b.platform_id || "—");
      const platform = (b.platform || "telegram").toLowerCase();
      const stateText = botStateText(b);
      const tone = botStateTone(b);
      const capChips = [
        caps.text ? "文本" : "", caps.media ? "媒体" : "",
        caps.buttons ? "按钮" : "", caps.inline ? "内联" : "",
        caps.topics ? "话题" : "", caps.join_approval ? "入群" : "",
        caps.webhook ? "Webhook" : ""
      ].filter(Boolean).slice(0, 4);
      return `<article class="bot-card ${(b.online || b.state === "在线" || b.state === "已连接") ? "is-running" : ""}" data-act="enter-bot" data-id="${esc(b.id)}" aria-label="进入 ${esc(name)}">
        <div class="bot-card-top">
          <span class="platform-tag platform-${esc(platform)}">${esc(b.platform_name || platform)}</span>
          <span class="bot-card-state ${tone}"><i></i>${esc(stateText)}</span>
          <div class="bot-card-menu">
            <button class="icon-btn sm" type="button" data-act="edit-bot" data-id="${esc(b.id)}" aria-label="编辑 ${esc(name)}" title="编辑">${icon("edit")}</button>
            <button class="icon-btn sm" type="button" data-act="restart-bot" data-id="${esc(b.id)}" aria-label="重启 ${esc(name)}" title="重启" ${b.enabled ? "" : "disabled"}>${icon("refresh")}</button>
            <button class="icon-btn sm danger" type="button" data-act="delete-bot" data-id="${esc(b.id)}" aria-label="删除 ${esc(name)}" title="删除">${icon("trash")}</button>
          </div>
        </div>
        <div class="bot-card-main">
          <span class="bot-card-avatar">${esc(String(name).trim().slice(0, 1).toUpperCase() || "B")}</span>
          <div class="bot-card-name"><h2>${esc(name)}</h2><p>${esc(who)}</p></div>
          <span class="bot-card-id mono">#${esc(b.id)}</span>
        </div>
        <div class="bot-card-meta">
          <span class="bot-card-mode">${esc(b.mode === "webhook" ? "Webhook" : b.mode === "websocket" ? "WebSocket" : "长轮询")}</span>
          ${capChips.map(c => `<span>${esc(c)}</span>`).join("")}
        </div>
        <div class="bot-card-foot">
          <button class="bot-card-enter" type="button" data-act="enter-bot" data-id="${esc(b.id)}">进入工作台${icon("arrow-right")}</button>
        </div>
      </article>`;
    }).join("");
    const addCard = `<button class="bot-add-card" type="button" data-act="new-bot">
      <span class="bot-add-icon">${icon("plus")}</span>
      <strong>添加 Bot</strong>
      <span>Telegram 或 QQ，凭据加密存储</span>
    </button>`;
    const empty = `<div class="launcher-empty">
      <div class="empty-icon">${icon("bot")}</div>
      <h2>还没有 Bot</h2>
      <p>先添加一个 Telegram Bot 或 QQ AppID + AppSecret，之后所有规则、群和操作都会收进它的工作台。</p>
      <button class="btn primary" type="button" data-act="new-bot">${icon("plus")}添加第一个 Bot</button>
    </div>`;
    return `<div class="launcher">
      <header class="launcher-top">
        <a class="launcher-brand" href="#bots" aria-label="LianT 首页">
          <span class="brand-mark">${icon("bot")}</span>
          <span><strong>LianT</strong><small>Bot 控制台</small></span>
        </a>
        <div class="launcher-tools">
          <button class="btn ghost" type="button" data-act="open-token">${icon("key")}连接设置</button>
          <button class="btn primary" type="button" data-act="new-bot">${icon("plus")}添加 Bot</button>
        </div>
      </header>

      <section class="launcher-hero">
        <p class="eyebrow">Bot workspace</p>
        <h1>选择一个 Bot，<br>开始管理。</h1>
        <p class="launcher-lede">每个 Bot 有独立数据库、规则和成员。进入工作台后，再处理群、消息、违禁词和审计。</p>
        <div class="launcher-stats">
          <div><strong>${total}</strong><span>Bot</span></div>
          <div><strong>${online}</strong><span>在线</span></div>
          <div><strong>${platforms.length}</strong><span>平台</span></div>
        </div>
      </section>

      ${total ? `<section class="bot-grid">${cards}${addCard}</section>` : empty}

      <footer class="launcher-foot">
        <span>${state.platforms.map(p => p.name).join(" · ") || "Telegram"}</span>
        <span>凭据存入 SQLite · AES-256-GCM 加密</span>
        <span>${total ? online + " / " + total + " 在线" : "等待第一个 Bot"}</span>
      </footer>
    </div>`;
  }


  /* ======================================================================
     视图：总览
     ====================================================================== */
  async function renderOverview() {
    const data = await api("overview");
    const counts = data.counts || {};
    const bot = data.bot || {};
    const status = String(bot.bot_state || "未知");
    const tone = botTone(status);
    const recent = Array.isArray(data.recent) ? data.recent.slice(0, 8) : [];

    const stats = [
      { label: "群", value: counts.groups, hint: "已登记群", icon: "users", href: "#groups" },
      { label: "回复规则", value: counts.rules, hint: "按优先级回复", icon: "message", href: "#rules" },
      { label: "违禁词", value: counts.words, hint: "命中即处置", icon: "shield", href: "#words" },
      { label: "管理员", value: counts.admins, hint: "可用中文指令", icon: "key", href: "#admins" },
      { label: "禁言中", value: counts.mutes, hint: "本地记录", icon: "clock", href: "#mutes" },
      { label: "黑名单", value: counts.blacklist, hint: "本地黑名单", icon: "ban", href: "#blacklist" }
    ];

    const recentRows = recent.map(a => {
      const search = [a.Ts, a.Kind, a.GroupOpenID, a.Username, a.MemberOpenID, a.Hit, a.Result, a.Detail].join(" ");
      return `<tr data-search="${esc(search)}">
        <td class="nowrap">${esc(fmtDateTime(a.Ts))}</td>
        <td>${kindBadge(a.Kind)}</td>
        <td class="hide-sm">${esc(a.Username || memberName(a.MemberOpenID))}</td>
        <td>${esc(truncate(humanize(a.Hit), 22))}</td>
        <td>${resultBadge(a.Result)}</td>
      </tr>`;
    }).join("");

    const recentCard = recent.length
      ? `<section class="card">
          <div class="card-head"><div><h2>最近操作</h2><p>最新 ${recent.length} 条审计记录</p></div><a class="btn ghost sm" href="#audit">查看全部</a></div>
          ${dataTable([
            { label: "时间" }, { label: "类型" }, { label: "成员", cls: "hide-sm" },
            { label: "命中" }, { label: "结果" }
          ], recentRows)}
        </section>`
      : `<section class="card">
          <div class="card-head"><div><h2>最近操作</h2><p>任务进程还没有记录操作。</p></div><a class="btn ghost sm" href="#audit">查看全部</a></div>
          ${emptyState({ icon: "list", title: "还没有操作记录", text: "禁言、撤回、群发等动作会出现在这里。" })}
        </section>`;

    return `${pageHead(TAB_MAP.overview, `
        <button class="btn" type="button" data-act="export-config">${icon("download")}导出配置</button>
        <button class="btn" type="button" data-act="refresh">${icon("refresh")}刷新</button>`)}
      ${bot.last_error ? `<div class="callout danger">${icon("alert")}<span><b>最近错误：</b>${esc(bot.last_error)}</span></div>` : ""}
      <div class="stat-grid">${stats.map(statCard).join("")}</div>

      <div class="grid two">
        <section class="card card-pad">
          <div class="head-row">
            <div><h2>任务进程</h2><p>${esc(status)} · 已运行 ${esc(fmtDuration(bot.uptime))}</p></div>
            <span class="badge ${tone}">${esc(TONE_LABEL[tone] || "状态")}</span>
          </div>
          <div class="metric-grid">
            ${metric("累计消息", fmtNum(bot.messages))}
            ${metric("重连次数", fmtNum(bot.reconnects))}
            ${metric("最近消息", fmtDateTime(bot.last_message_at) || "—")}
            ${metric("凭据到期", fmtDateTime(bot.token_expires_at) || "—")}
          </div>
          <div class="addr-list">
            ${addrRow("数据库", data.db_file)}
            ${addrRow("管理端", location.origin + BASE)}
            ${bot.gateway ? addrRow("网关", bot.gateway) : ""}
          </div>
        </section>

        <section class="card">
          <div class="card-head"><div><h2>快捷操作</h2><p>常用入口</p></div></div>
          <div class="quick-list">
            <a class="quick-link" href="#chat">${icon("send")}<span>发送群消息</span>${icon("chevron-right", "i chev")}</a>
            <a class="quick-link" href="#joins">${icon("user-plus")}<span>审批入群申请</span>${icon("chevron-right", "i chev")}</a>
            <a class="quick-link" href="#words">${icon("shield")}<span>新增违禁词</span>${icon("chevron-right", "i chev")}</a>
            <a class="quick-link" href="#audit">${icon("list")}<span>查看操作日志</span>${icon("chevron-right", "i chev")}</a>
          </div>
          <div class="perm-note">${icon("info")}<span>${esc(data.whitelist || "")}</span></div>
        </section>
      </div>

      ${recentCard}

      <details class="card help-card">
        <summary>${icon("message")}<span>群内指令</span>${icon("chevron-right", "i chev")}</summary>
        <div class="help-body">
          <pre class="code">${esc(data.cmd_help || "暂无")}</pre>
          <div class="callout">${icon("info")}<span>${esc(data.whitelist || "")}</span></div>
        </div>
      </details>`;
  }

  /* ======================================================================
     视图：回复规则
     ====================================================================== */
  async function renderRules() {
    const [rules, groups] = await Promise.all([api("rules"), groupsSafe().catch(() => [])]);
    state.rules = Array.isArray(rules) ? rules.slice() : [];
    state.groups = Array.isArray(groups) ? groups : state.groups;
    const list = state.rules.slice().sort((a, b) => (a.Priority - b.Priority) || (a.ID - b.ID));

    const rows = list.map(rule => {
      const scope = rule.Scope ? groupLabel(rule.Scope) : "全部群";
      const search = [rule.ID, rule.Priority, rule.Match, rule.Pattern, rule.Reply, rule.Note, scope].join(" ");
      return `<tr data-search="${esc(search)}">
        <td><span class="badge">${esc(rule.Priority)}</span></td>
        <td><span class="badge accent">${esc(MATCH_LABEL[rule.Match] || rule.Match)}</span></td>
        <td><code class="clamp" title="${esc(rule.Pattern)}">${esc(truncate(rule.Pattern, 42))}</code></td>
        <td>${rule.Reply ? esc(truncate(rule.Reply, 36)) : '<span class="muted">不回复</span>'}</td>
        <td class="hide-sm">${esc(scope)}</td>
        <td>${rule.Enabled ? '<span class="badge ok">启用</span>' : '<span class="badge muted">停用</span>'}</td>
        <td class="actions"><div class="row-actions">
          <button class="icon-btn sm" type="button" data-act="edit-rule" data-id="${rule.ID}" aria-label="编辑规则 ${rule.ID}" title="编辑">${icon("edit")}</button>
          <button class="icon-btn sm danger" type="button" data-act="delete-rule" data-id="${rule.ID}" aria-label="删除规则 ${rule.ID}" title="删除">${icon("trash")}</button>
        </div></td>
      </tr>`;
    }).join("");

    return `${pageHead(TAB_MAP.rules, `<button class="btn primary" type="button" data-act="new-rule">${icon("plus")}新增规则</button>`)}
      <section class="card">
        ${cardToolbar({ placeholder: "搜索规则、回复或备注…", count: list.length })}
        ${list.length ? dataTable([
          { label: "优先级" }, { label: "匹配" }, { label: "内容" },
          { label: "回复" }, { label: "范围", cls: "hide-sm" }, { label: "状态" }, { label: "", cls: "actions" }
        ], rows) : emptyState({
          icon: "message", title: "还没有回复规则",
          text: "规则按优先级从小到大匹配，命中第一条即停止。",
          action: `<button class="btn primary" type="button" data-act="new-rule">${icon("plus")}新增规则</button>`
        })}
      </section>`;
  }

  /* ======================================================================
     视图：违禁词
     ====================================================================== */
  async function renderWords() {
    const [words, groups] = await Promise.all([api("words"), groupsSafe().catch(() => [])]);
    state.words = Array.isArray(words) ? words.slice() : [];
    state.groups = Array.isArray(groups) ? groups : state.groups;
    const list = state.words.slice().sort((a, b) => (a.Severity - b.Severity) || (a.ID - b.ID));

    const rows = list.map(word => {
      const acts = [
        word.Recall ? "撤回" : "",
        word.MuteSeconds > 0 ? "禁言 " + fmtSeconds(word.MuteSeconds) : "",
        word.Kick ? "踢出" : "",
        word.Blacklist ? "拉黑" : "",
        word.Reply ? "警告" : ""
      ].filter(Boolean);
      const scope = word.Scope ? groupLabel(word.Scope) : "全部群";
      const search = [word.ID, word.Word, word.Match, word.Reply, word.Note, scope, acts.join(" ")].join(" ");
      return `<tr data-search="${esc(search)}">
        <td><code>${esc(word.Word)}</code></td>
        <td><span class="badge">${esc(MATCH_LABEL[word.Match] || word.Match)}</span></td>
        <td>${acts.length ? `<div class="chip-row">${acts.map(a => `<span class="badge ${a === "拉黑" || a === "踢出" ? "warn" : ""}">${esc(a)}</span>`).join("")}</div>` : '<span class="muted">仅记录</span>'}</td>
        <td>${esc(word.Severity)}</td>
        <td class="hide-sm">${esc(scope)}</td>
        <td>${word.Enabled ? '<span class="badge ok">启用</span>' : '<span class="badge muted">停用</span>'}</td>
        <td class="actions"><div class="row-actions">
          <button class="icon-btn sm" type="button" data-act="edit-word" data-id="${word.ID}" aria-label="编辑违禁词 ${word.ID}" title="编辑">${icon("edit")}</button>
          <button class="icon-btn sm danger" type="button" data-act="delete-word" data-id="${word.ID}" aria-label="删除违禁词 ${word.ID}" title="删除">${icon("trash")}</button>
        </div></td>
      </tr>`;
    }).join("");

    return `${pageHead(TAB_MAP.words, `<button class="btn primary" type="button" data-act="new-word">${icon("plus")}新增违禁词</button>`)}
      <section class="card">
        ${cardToolbar({ placeholder: "搜索违禁词或备注…", count: list.length })}
        ${list.length ? dataTable([
          { label: "词" }, { label: "匹配" }, { label: "处置" }, { label: "优先级" },
          { label: "范围", cls: "hide-sm" }, { label: "状态" }, { label: "", cls: "actions" }
        ], rows) : emptyState({
          icon: "shield", title: "还没有违禁词",
          text: "命中后可按配置撤回、禁言、踢出、拉黑，或回复警告；优先级数字小的先匹配。",
          action: `<button class="btn primary" type="button" data-act="new-word">${icon("plus")}新增违禁词</button>`
        })}
      </section>`;
  }

  /* ======================================================================
     视图：群
     ====================================================================== */
  function groupToggle(group, field, label, hint) {
    return `<label class="toggle">
      <span class="toggle-text"><span>${esc(label)}</span><small>${esc(hint)}</small></span>
      <input class="sw" type="checkbox" data-toggle="group" data-gid="${esc(group.OpenID)}" data-field="${esc(field)}"${group[field] ? " checked" : ""} aria-label="${esc(label)}">
    </label>`;
  }

  async function renderGroups() {
    const list = await api("groups");
    state.groups = Array.isArray(list) ? list : [];

    const cards = state.groups.map(group => `<article class="card group-card">
      <div class="group-head">
        <div class="group-avatar avatar ${avatarClass(group.Name || group.OpenID)}">${esc(initial(group.Name))}</div>
        <div class="group-title">
          <h3>${esc(group.Name || "未命名群")}</h3>
          <span class="group-sub">${esc(group.Note || "机器人已登记")}</span>
        </div>
        <div class="group-actions">
          <button class="icon-btn sm" type="button" data-act="edit-group" data-gid="${esc(group.OpenID)}" aria-label="编辑群名和备注" title="编辑群名/备注">${icon("edit")}</button>
          <button class="icon-btn sm" type="button" data-act="group-status" data-gid="${esc(group.OpenID)}" aria-label="查询机器人群内状态" title="查询群内状态">${icon("activity")}</button>
          <button class="icon-btn sm danger" type="button" data-act="delete-group" data-gid="${esc(group.OpenID)}" aria-label="删除群记录" title="删除群记录">${icon("trash")}</button>
        </div>
      </div>
      <div class="toggle-grid">
        ${groupToggle(group, "Enabled", "启用", "群消息总开关")}
        ${groupToggle(group, "ReplyAll", "全量匹配", "每条消息都比对规则")}
        ${groupToggle(group, "CommandsEnabled", "群内指令", "允许管理员用中文指令")}
        ${groupToggle(group, "GroupAdminCanCmd", "群管可用指令", "群主 / 管理员也能用")}
        ${groupToggle(group, "BotIsAdmin", "机器人是管理员", "决定能否禁言 / 撤回他人")}
      </div>
      <div class="group-foot">
        <span>加入于 ${esc(fmtDate(group.JoinedAt) || "—")}</span>
        <span class="save-flash" data-save>已保存</span>
        <span class="badge ${group.BotIsAdmin ? "ok" : "muted"}">${group.BotIsAdmin ? "机器人是管理员" : "机器人非管理员"}</span>
      </div>
    </article>`).join("");

    return `${pageHead(TAB_MAP.groups, `
        <button class="btn" type="button" data-act="sync-group-names">${icon("refresh")}补全群名</button>
        <button class="btn primary" type="button" data-act="new-group">${icon("plus")}添加群</button>`)}
      <p class="section-note">机器人收到群消息后会自动登记群。能读取群信息时会自动补全群名；没有权限时点群卡片的编辑按钮手动填写。这里只显示已登记群，不会去平台拉取完整的群列表。</p>
      ${cards ? `<div class="group-grid">${cards}</div>` : `<section class="card">${emptyState({
        icon: "users", title: "还没有登记群",
        text: "把机器人拉进群并 @ 它一次，群就会出现在这里；也可以手动补录群。",
        action: `<button class="btn primary" type="button" data-act="new-group">${icon("plus")}添加群</button>`
      })}</section>`}`;
  }

  /* ======================================================================
     视图：管理员
     ====================================================================== */
  async function renderAdmins() {
    const [admins, groups] = await Promise.all([api("admins"), groupsSafe().catch(() => [])]);
    state.admins = Array.isArray(admins) ? admins.slice() : [];
    state.groups = Array.isArray(groups) ? groups : state.groups;

    const rows = state.admins.map(admin => {
      const scope = admin.GroupOpenID ? groupLabel(admin.GroupOpenID) : "全局";
      const search = [admin.ID, admin.MemberOpenID, admin.Username, admin.Role, scope].join(" ");
      return `<tr data-search="${esc(search)}">
        <td>${esc(admin.Username || "未命名管理员")}</td>
        <td><span class="badge ${admin.Role === "super" ? "accent" : ""}">${esc(roleLabel(admin.Role))}</span></td>
        <td>${esc(scope)}</td>
        <td>${admin.Enabled ? '<span class="badge ok">启用</span>' : '<span class="badge muted">停用</span>'}</td>
        <td class="actions"><div class="row-actions">
          <button class="icon-btn sm" type="button" data-act="copy" data-copy="${esc(admin.MemberOpenID)}" aria-label="复制成员 ID" title="复制成员 ID">${icon("copy")}</button>
          <button class="icon-btn sm danger" type="button" data-act="delete-admin" data-id="${admin.ID}" aria-label="删除管理员" title="删除">${icon("trash")}</button>
        </div></td>
      </tr>`;
    }).join("");

    return `${pageHead(TAB_MAP.admins, `<button class="btn primary" type="button" data-act="new-admin">${icon("plus")}添加管理员</button>`)}
      <p class="section-note">全局管理员在所有群可用；群级管理员只在该群可用。超级管理员目前只作标记。</p>
      <section class="card">
        ${cardToolbar({ placeholder: "搜索昵称或范围…", count: state.admins.length })}
        ${state.admins.length ? dataTable([
          { label: "昵称" }, { label: "角色" }, { label: "范围" },
          { label: "状态" }, { label: "", cls: "actions" }
        ], rows) : emptyState({
          icon: "key", title: "还没有管理员",
          text: "这些人在群里可以直接使用禁言、撤回、加违禁词等中文指令。",
          action: `<button class="btn primary" type="button" data-act="new-admin">${icon("plus")}添加管理员</button>`
        })}
      </section>`;
  }

  /* ======================================================================
     视图：禁言
     ====================================================================== */
  function muteRemaining(value) {
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return { text: "—", cls: "" };
    const ms = d.getTime() - Date.now();
    if (ms <= 0) return { text: "已到期", cls: "muted" };
    const minutes = Math.ceil(ms / 60000);
    if (minutes < 60) return { text: "剩 " + minutes + " 分钟", cls: "warn" };
    const hours = Math.ceil(minutes / 60);
    if (hours < 24) return { text: "剩 " + hours + " 小时", cls: "warn" };
    return { text: "剩 " + Math.ceil(hours / 24) + " 天", cls: "ok" };
  }

  async function renderMutes() {
    const [mutes, groups] = await Promise.all([api("mutes"), groupsSafe().catch(() => [])]);
    state.mutes = Array.isArray(mutes) ? mutes.slice() : [];
    state.groups = Array.isArray(groups) ? groups : state.groups;

    const rows = state.mutes.map(mute => {
      const left = muteRemaining(mute.ExpireAt);
      const search = [mute.ID, mute.GroupOpenID, mute.MemberOpenID, mute.Username, mute.Reason, mute.Operator].join(" ");
      return `<tr data-search="${esc(search)}">
        <td>${esc(mute.Username || memberName(mute.MemberOpenID))}</td>
        <td>${esc(groupLabel(mute.GroupOpenID))}</td>
        <td><span class="badge ${left.cls}">${esc(left.text)}</span><div class="muted" style="font-size:10px">${esc(fmtDateTime(mute.ExpireAt))}</div></td>
        <td>${esc(truncate(mute.Reason, 26))}</td>
        <td>${esc(humanActor(mute.Operator))}</td>
        <td class="actions"><div class="row-actions">
          <button class="btn sm" type="button" data-act="unmute" data-gid="${esc(mute.GroupOpenID)}" data-mid="${esc(mute.MemberOpenID)}">解除</button>
        </div></td>
      </tr>`;
    }).join("");

    const groupSelect = `<select class="input select select-inline" id="mute-state-group" aria-label="选择群">
      <option value="">选择群…</option>${groupOptions("")}
    </select>`;

    return `${pageHead(TAB_MAP.mutes, `<button class="btn primary" type="button" data-act="new-mute">${icon("plus")}手动禁言</button>`)}
      <section class="card">
        ${cardToolbar({ placeholder: "搜索成员、群或原因…", count: state.mutes.length })}
        ${state.mutes.length ? dataTable([
          { label: "成员" }, { label: "群" }, { label: "到期" }, { label: "原因" }, { label: "操作人" },
          { label: "", cls: "actions" }
        ], rows) : emptyState({
          icon: "clock", title: "没有进行中的禁言",
          text: "网页和群内指令设置的禁言都会记录在这里；平台侧需要机器人是群管理员。",
          action: `<button class="btn primary" type="button" data-act="new-mute">${icon("plus")}手动禁言</button>`
        })}
      </section>

      <section class="card card-pad" style="margin-top:14px">
        <div class="head-row"><div><h2>平台侧禁言状态</h2><p>查询群当前禁言名单与全员禁言模式。</p></div></div>
        <div class="inline-form">${groupSelect}<button class="btn" type="button" data-act="mute-state">查询</button></div>
        <div id="mute-state-out"></div>
      </section>`;
  }

  /* ======================================================================
     视图：黑名单
     ====================================================================== */
  async function renderBlacklist() {
    const [list, groups] = await Promise.all([api("blacklist"), groupsSafe().catch(() => [])]);
    state.blacklist = Array.isArray(list) ? list.slice() : [];
    state.groups = Array.isArray(groups) ? groups : state.groups;

    const rows = state.blacklist.map(item => {
      const scope = item.GroupOpenID ? groupLabel(item.GroupOpenID) : "全局";
      const search = [item.ID, item.GroupOpenID, item.MemberOpenID, item.Username, item.Reason, scope].join(" ");
      return `<tr data-search="${esc(search)}">
        <td>${esc(item.Username || memberName(item.MemberOpenID))}</td>
        <td>${esc(scope)}</td>
        <td>${esc(truncate(item.Reason, 28))}</td>
        <td>${item.QQSynced ? '<span class="badge ok">已同步</span>' : '<span class="badge muted">仅本地</span>'}</td>
        <td>${esc(fmtDateTime(item.CreatedAt))}</td>
        <td class="actions"><div class="row-actions">
          <button class="btn sm" type="button" data-act="remove-black" data-gid="${esc(item.GroupOpenID || "")}" data-mid="${esc(item.MemberOpenID)}">移出</button>
        </div></td>
      </tr>`;
    }).join("");

    return `${pageHead(TAB_MAP.blacklist, `<button class="btn primary" type="button" data-act="new-black">${icon("plus")}拉黑成员</button>`)}
      <p class="section-note">本地黑名单立刻生效（不回 + 尽量撤回）；平台侧同步需要相应权限。</p>
      <section class="card">
        ${cardToolbar({ placeholder: "搜索成员、群或原因…", count: state.blacklist.length })}
        ${state.blacklist.length ? dataTable([
          { label: "成员" }, { label: "范围" }, { label: "原因" }, { label: "平台同步" }, { label: "时间" },
          { label: "", cls: "actions" }
        ], rows) : emptyState({
          icon: "ban", title: "黑名单是空的",
          text: "被拉黑成员的消息不会再触发回复，并尽量撤回。",
          action: `<button class="btn primary" type="button" data-act="new-black">${icon("plus")}拉黑成员</button>`
        })}
      </section>`;
  }

  /* ======================================================================
     视图：入群申请
     ====================================================================== */
  async function renderJoins() {
    const groups = await groupsSafe(true);
    let gid = store.get(GROUP_KEY, "");
    if (gid && !groups.some(g => g.OpenID === gid)) gid = "";
    if (!gid && groups.length) gid = (groups.find(g => g.Enabled) || groups[0]).OpenID;
    if (gid) store.set(GROUP_KEY, gid);

    let data = null;
    let error = "";
    if (gid) {
      try {
        data = await api("joins?group=" + encodeURIComponent(gid));
        if (data && data.ok === false) {
          error = (data.error || "查询失败") + (data.Hint || data.hint ? " —— " + (data.Hint || data.hint) : "");
          data = null;
        }
      } catch (err) {
        if (err && err.status === 401) throw err;
        error = err && err.message ? err.message : String(err);
      }
    }
    const list = data && Array.isArray(data.list) ? data.list : [];
    state.joins = list;

    const groupSelect = `<select class="input select select-inline" data-joins-group aria-label="选择群">
      ${groupOptions(gid)}
    </select>`;

    let content = "";
    if (!groups.length) {
      content = emptyState({
        icon: "users", title: "还没有登记群",
        text: "机器人进群并收到消息后，群会出现在这里。",
        action: `<a class="btn primary" href="#groups">去添加群</a>`
      });
    } else if (error) {
      content = `<div class="card-pad"><div class="callout danger">${icon("alert")}<span>${esc(error)}</span></div><button class="btn" type="button" data-act="refresh">重试</button></div>`;
    } else if (!gid) {
      content = emptyState({ icon: "user-plus", title: "先选择一个群", text: "审批入群申请需要先选择群。" });
    } else if (!list.length) {
      content = emptyState({
        icon: "check", title: "暂无待审批申请",
        text: "打开设置里的「自动通过」后，机器人会直接批准入群申请。"
      });
    } else {
      const rows = list.map(item => {
        const verify = item.verify_info && item.verify_info.verify_message ? item.verify_info.verify_message : "";
        const name = item.username || memberName(item.member_openid);
        const search = [name, item.member_openid, verify, item.risk_tips, item.apply_source].join(" ");
        return `<tr data-search="${esc(search)}">
          <td>${esc(name)}</td>
          <td>${esc(item.apply_source || "—")}</td>
          <td>${verify ? esc(truncate(verify, 40)) : '<span class="muted">—</span>'}</td>
          <td>${item.risk_tips ? `<span class="badge warn">${esc(truncate(item.risk_tips, 24))}</span>` : '<span class="muted">—</span>'}</td>
          <td class="nowrap">${esc(fmtDateTime(item.apply_at))}</td>
          <td class="actions"><div class="row-actions">
            <button class="btn primary sm" type="button" data-act="approve-join" data-gid="${esc(gid)}" data-mid="${esc(item.member_openid)}" data-rid="${esc(item.join_request_id || "")}" data-name="${esc(name)}">通过</button>
            <button class="btn danger sm" type="button" data-act="reject-join" data-gid="${esc(gid)}" data-mid="${esc(item.member_openid)}" data-rid="${esc(item.join_request_id || "")}" data-name="${esc(name)}">拒绝</button>
          </div></td>
        </tr>`;
      }).join("");
      content = dataTable([
        { label: "申请人" }, { label: "来源" }, { label: "验证信息" }, { label: "风险提示" },
        { label: "申请时间" }, { label: "", cls: "actions" }
      ], rows);
    }

    return `${pageHead(TAB_MAP.joins, `<button class="btn" type="button" data-act="refresh">${icon("refresh")}刷新</button>`)}
      <section class="card">
        <div class="card-toolbar">
          <span class="toolbar-count">群</span>
          ${groupSelect}
          <span class="spacer"></span>
          <span class="toolbar-count">${gid ? esc(groupLabel(gid)) : "未选择"}</span>
        </div>
        ${content}
      </section>`;
  }

  /* ======================================================================
     视图：操作日志
     ====================================================================== */
  async function renderAudit() {
    const list = await api("audit?limit=200");
    await groupsSafe().catch(err => { if (err && err.status === 401) throw err; });
    await loadMembers().catch(err => { if (err && err.status === 401) throw err; });
    const items = Array.isArray(list) ? list : [];
    const kinds = Array.from(new Set(items.map(a => a.Kind).filter(Boolean)));

    const rows = items.map(a => {
      const search = [a.Ts, a.Kind, a.GroupOpenID, a.Username, a.MemberOpenID, a.Hit, a.Result, a.Detail, a.Content].join(" ");
      return `<tr data-search="${esc(search)}" data-kind="${esc(a.Kind || "")}">
        <td class="nowrap">${esc(fmtDateTime(a.Ts))}</td>
        <td>${kindBadge(a.Kind)}</td>
        <td>${esc(a.Username || memberName(a.MemberOpenID))}</td>
        <td>${esc(truncate(humanize(a.Hit), 24))}</td>
        <td>${actionChips(a.Actions)}</td>
        <td>${resultBadge(a.Result)}</td>
        <td class="hide-sm">${esc(truncate(humanize(a.Detail), 50))}</td>
      </tr>`;
    }).join("");

    const kindSelect = `<select class="input select select-inline" data-filter-kind aria-label="按类型筛选">
      <option value="">全部类型</option>
      ${kinds.map(kind => `<option value="${esc(kind)}">${esc(KIND_LABEL[kind] || kind)}</option>`).join("")}
    </select>`;

    return `${pageHead(TAB_MAP.audit, `<button class="btn danger" type="button" data-act="prune-audit">${icon("trash")}清理旧日志</button>`)}
      <section class="card">
        ${cardToolbar({ placeholder: "搜索成员、命中或详情…", count: items.length, extra: kindSelect })}
        ${items.length ? dataTable([
          { label: "时间" }, { label: "类型" }, { label: "成员" }, { label: "命中" }, { label: "动作" },
          { label: "结果" }, { label: "详情", cls: "hide-sm" }
        ], rows, { wide: true }) : emptyState({
          icon: "list", title: "还没有操作记录",
          text: "群内指令、自动处置和网页操作都会写入这里。"
        })}
      </section>`;
  }

  /* ======================================================================
     视图：手动操作
     ====================================================================== */
  function qqMessageOptionsHTML(messages) {
    const options = [`<option value="">不指定（按条数撤回）</option>`];
    (messages || []).slice(-30).reverse().forEach(message => {
      const who = message.is_bot ? "机器人" : (message.username || "成员");
      const text = plainPreview(message.content || "", 26) || "（无文字）";
      options.push(`<option value="${esc(message.id)}">${esc(who + "：" + text + " · " + fmtTime(message.at))}</option>`);
    });
    return options.join("");
  }

  async function loadQQMessages(gid, form) {
    const select = form ? form.querySelector("[data-qq-messages]") : null;
    if (!select) return;
    if (!gid) {
      state.qqMessages = [];
      select.innerHTML = qqMessageOptionsHTML([]);
      return;
    }
    select.innerHTML = `<option value="">加载最近消息…</option>`;
    try {
      const data = await api("chat/recent?group=" + encodeURIComponent(gid));
      state.qqMessages = data && Array.isArray(data.messages) ? data.messages : [];
      select.innerHTML = qqMessageOptionsHTML(state.qqMessages);
    } catch (err) {
      if (err && err.status === 401) throw err;
      select.innerHTML = `<option value="">最近消息读取失败</option>`;
    }
  }

  async function renderQQ() {
    await groupsSafe().catch(err => { if (err && err.status === 401) throw err; });
    await loadMembers().catch(err => { if (err && err.status === 401) throw err; });
    let messages = [];
    const first = state.groups.find(g => g.Enabled) || state.groups[0];
    if (first) {
      try {
        const data = await api("chat/recent?group=" + encodeURIComponent(first.OpenID));
        messages = data && Array.isArray(data.messages) ? data.messages : [];
      } catch (err) {
        if (err && err.status === 401) throw err;
      }
    }
    state.qqMessages = messages;

    return `${pageHead(TAB_MAP.qq)}
      <div class="callout">${icon("info")}<span>直接调用平台接口。群、成员、最近消息都从名册里选，不需要记 ID；禁言、撤回他人消息需要机器人是群管理员。</span></div>

      <form class="card card-pad" data-form="qq-recall">
        <div class="head-row"><div><h2>撤回消息</h2><p>选择群和最近消息，或按条数撤回；群成员的消息限 2 分钟内。</p></div></div>
        <div class="form-grid">
          ${fieldHTML("群", groupSelectHTML("group_openid", { selected: first ? first.OpenID : "", required: true, attrs: ' data-qq-group="1"' }), "从已登记的群里选")}
          ${fieldHTML("消息", `<select class="input select" name="message_id" data-qq-messages>${qqMessageOptionsHTML(messages)}</select>`, "留空＝按条数撤回")}
          ${fieldBlock("成员", memberSelectHTML("member_openid", { autoGroup: true }), "选择成员＝撤他最后一条")}
          ${fieldHTML("撤回条数", inputHTML("count", { type: "number", value: 1, min: 1, max: 20 }), "消息 / 成员都留空时生效")}
        </div>
        <div class="form-actions"><button class="btn primary" type="submit">执行撤回</button></div>
      </form>

      <form class="card card-pad" data-form="qq-kick">
        <div class="head-row"><div><h2>踢出成员</h2><p>需要平台权限；勾选后同步加入群黑名单。</p></div></div>
        <div class="form-grid">
          ${fieldHTML("群", groupSelectHTML("group_openid", { selected: first ? first.OpenID : "", required: true }), "选成员时会自动带出")}
          ${fieldBlock("成员", memberSelectHTML("member_openid", { autoGroup: true }), "从群名册里选")}
          <div class="span-2 switch-grid">${switchHTML("add_to_member_blacklist", "同时拉黑", "平台侧拉黑需要相应权限", true)}</div>
        </div>
        <div class="form-actions"><button class="btn danger" type="submit">踢出</button></div>
      </form>

      <form class="card card-pad" data-form="qq-view">
        <div class="head-row"><div><h2>查看</h2><p>群信息 / 成员列表 / 禁言状态。</p></div></div>
        <div class="form-grid">
          ${fieldHTML("群", groupSelectHTML("group_openid", { selected: first ? first.OpenID : "", required: true }), "从已登记的群里选", "span-2")}
        </div>
        <div class="form-actions">
          <button class="btn" type="button" data-act="qq-groupinfo">群信息</button>
          <button class="btn" type="button" data-act="qq-members">群成员</button>
          <button class="btn" type="button" data-act="qq-mutestate">禁言状态</button>
        </div>
      </form>

      <section class="card" id="qq-output" hidden>
        <div class="card-head">
          <div><h2 id="qq-output-title">结果</h2><p>平台接口返回的原始数据（含内部 ID，仅供排查）</p></div>
          <button class="icon-btn sm" type="button" data-act="copy-output" aria-label="复制结果" title="复制">${icon("copy")}</button>
        </div>
        <pre class="code" id="qq-out" style="margin-top:12px;border-top:1px solid var(--border)"></pre>
      </section>`;
  }

  /* ======================================================================
     视图：设置
     ====================================================================== */
  async function renderSettings() {
    const settings = await api("settings");
    const s = settings || {};
    const truthy = value => String(value || "").toLowerCase() === "true";
    const masked = {};
    Object.keys(s).sort().forEach(key => {
      masked[key] = /token|secret|pass(word)?/i.test(key) && s[key] ? "••••••（已隐藏）" : s[key];
    });

    const settingRow = (title, desc, control) => `<div class="setting-row">
      <div class="setting-text"><h3>${esc(title)}</h3><p>${esc(desc)}</p></div>
      ${control}
    </div>`;

    return `${pageHead(TAB_MAP.settings, `<button class="btn" type="button" data-act="export-config">${icon("download")}导出配置</button>`)}
      <form data-form="settings" class="settings-form">
        <section class="card card-pad">
          <div class="head-row"><div><h2>回复与入群</h2><p>保存后对下一条消息生效。</p></div></div>
          ${settingRow("全量回复", "关闭后只处理 / 开头的消息（「总是回复」规则例外）。",
            `<input class="sw" type="checkbox" name="reply_all" aria-label="全量回复"${truthy(s.reply_all || "true") ? " checked" : ""}>`)}
          ${settingRow("自动通过入群申请", "需要机器人是群管理员，否则审批会失败。",
            `<input class="sw" type="checkbox" name="join_auto_approve" aria-label="自动通过入群申请"${truthy(s.join_auto_approve) ? " checked" : ""}>`)}
          <label class="field" style="margin-top:14px">
            <span class="field-label">默认回复</span>
            <textarea class="input textarea" name="reply_default" rows="3" placeholder="留空＝没命中规则就不回">${esc(s.reply_default || "")}</textarea>
            <span class="field-hint">只有在「全量回复」开启、且没有命中任何规则时才会使用。</span>
          </label>
        </section>
        <div class="form-actions">
          <span class="dirty" data-dirty hidden>有未保存的更改</span>
          <button class="btn primary" type="submit">保存设置</button>
        </div>
      </form>

      <section class="card card-pad" style="margin-top:14px">
        <div class="head-row"><div><h2>运维</h2><p>重载凭据、导出数据库里的配置。</p></div></div>
        <div class="ops-row">
          <button class="btn" type="button" data-act="reload-bot">${icon("refresh")}重载规则与凭据</button>
          <button class="btn" type="button" data-act="export-config">${icon("download")}导出 JSON 配置</button>
        </div>
        <p class="field-hint" style="margin-top:10px">重载会重建网关连接，期间可能短暂离线；日常改规则不需要重载。</p>
      </section>

      <details class="card json-card">
        <summary>${icon("settings")}<span>全部设置</span><span class="badge">${Object.keys(s).length}</span>${icon("chevron-right", "i chev")}</summary>
        <p class="field-hint" style="padding:0 16px 10px">原始键值仅供排查；日常用上面的开关即可。</p>
        <pre class="code" style="border-top:1px solid var(--border)">${esc(JSON.stringify(masked, null, 2))}</pre>
      </details>`;
  }

  /* ======================================================================
     视图：群消息（聊天室）
     ====================================================================== */
  function parseCard(content) {
    const text = String(content || "");
    if (!text.startsWith("## ")) return null;
    const lines = text.split("\n");
    const title = lines.shift().replace(/^##\s+/, "").trim();
    let body = lines.join("\n").trim();
    let time = "";
    const idx = body.lastIndexOf("\n> LianT");
    if (idx >= 0) {
      time = body.slice(idx + 1).replace(/^> LianT\s*·?\s*/, "").trim();
      body = body.slice(0, idx).trim();
    }
    return { title: title || "群消息", body, time };
  }

  function cardBodyHTML(text) {
    // 先转义，再加最小的展示标签；绝不把用户内容当 HTML 解析。
    let out = esc(text || "（无正文）");
    out = out.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");
    out = out.replace(/__([^_\n]+)__/g, "<strong>$1</strong>");
    out = out.replace(/^[-*]\s+/gm, "• ");
    out = out.replace(/^#{1,6}\s+(.+)$/gm, "<strong>$1</strong>");
    return out.replace(/\n/g, "<br>");
  }

  function cardBubble(card) {
    return `<div class="qq-card">
      <div class="qq-card-title">${esc(card.title)}</div>
      <div class="qq-card-body">${cardBodyHTML(card.body)}</div>
      <div class="qq-card-foot"><span>LianT 卡片消息</span><span>${esc(card.time || "")}</span></div>
    </div>`;
  }

  /* 媒体：只拿到类型、名称、大小与状态；字节由 /api/media 按需取回，
     所以这里拼的是「取回地址」，不是平台直链。 */
  function mediaSource(message) {
    const gid = state.chat.group;
    if (!message || !message.id || !gid) return "";
    const params = new URLSearchParams();
    if (state.botID) params.set("bot_id", state.botID);
    params.set("chat_id", gid);
    params.set("msg", String(message.id));
    return BASE + "/api/media?" + params.toString();
  }

  function mediaStateNote(media) {
    const s = String((media && media.state) || "");
    if (s === "gone") return "平台侧文件已失效，或消息已被撤回";
    if (s === "expired") return "引用已过期，刷新后重试";
    if (media && media.fetchable === false) return "这条消息只有引用信息，没有可下载内容";
    return "暂时取不到媒体";
  }

  function mediaNote(media) {
    const label = MEDIA_KIND_LABEL[String((media && media.kind) || "")] || "附件";
    return `<div class="msg-media msg-media-note">${icon("alert")}<span>${esc(label)}不可取回</span><small>${esc(mediaStateNote(media))}</small></div>`;
  }

  function mediaHTML(message) {
    const media = message && message.media;
    if (!media || !media.kind) return "";
    const kind = String(media.kind);
    const stateName = String(media.state || "unknown");
    if (kind === "sticker") {
      const emoji = String(media.emoji || "").trim();
      return `<div class="msg-media msg-media-sticker" title="${esc(emoji ? "表情 " + emoji : "表情")}">${emoji ? esc(emoji) : icon("message")}</div>`;
    }
    const src = mediaSource(message);
    if (stateName === "gone" || stateName === "expired" || media.fetchable === false || !src) return mediaNote(media);
    const label = MEDIA_KIND_LABEL[kind] || "附件";
    const size = fmtBytes(media.size);
    const dims = media.width && media.height ? media.width + "×" + media.height : "";
    const meta = [label, media.mime, size, dims].filter(Boolean).join(" · ");
    if (kind === "image") {
      return `<figure class="msg-media msg-media-image">
        <button class="media-thumb" type="button" data-act="media-open" data-src="${esc(src)}" data-name="${esc(media.name || label)}" aria-label="放大图片">
          <img src="${esc(src)}" alt="${esc(media.name || label)}" loading="lazy" decoding="async" data-media-img>
        </button>
      </figure>`;
    }
    if (kind === "video" || kind === "animation" || kind === "video_note") {
      return `<figure class="msg-media msg-media-video">
        <video src="${esc(src)}" controls preload="none" playsinline></video>
        <figcaption>${esc(meta)}<a class="media-download" href="${esc(src)}" download="${esc(media.name || "")}">${icon("download")}下载</a></figcaption>
      </figure>`;
    }
    if (kind === "audio" || kind === "voice") {
      return `<figure class="msg-media msg-media-audio">
        <audio src="${esc(src)}" controls preload="none"></audio>
        <figcaption>${esc(meta)}<a class="media-download" href="${esc(src)}" download="${esc(media.name || "")}">${icon("download")}下载</a></figcaption>
      </figure>`;
    }
    return `<a class="msg-media msg-media-file" href="${esc(src)}" download="${esc(media.name || "")}">
      <span class="media-file-icon">${icon("download")}</span>
      <span class="media-file-text"><strong>${esc(media.name || label)}</strong><small>${esc(meta)}</small></span>
    </a>`;
  }

  function recentMessagePreview(message) {
    const kind = String(message.kind || "");
    if (String(message.state || "") === "recalled") return "撤回了一条消息";
    const text = String(message.content || "").trim();
    if (text) return text;
    return MEDIA_KIND_LABEL[kind] ? "[" + MEDIA_KIND_LABEL[kind] + "]" : "（无文字）";
  }

  function chatTimeMs(value) {
    const t = Date.parse(value);
    return Number.isFinite(t) ? t : 0;
  }

  function sameChatRun(a, b) {
    if (!a || !b) return false;
    if (String(a.state || "") === "recalled" || String(b.state || "") === "recalled") return false;
    if (Boolean(a.is_bot) !== Boolean(b.is_bot)) return false;
    if (String(a.username || "") !== String(b.username || "")) return false;
    if (fmtDate(a.at) !== fmtDate(b.at)) return false;
    const gap = Math.abs(chatTimeMs(b.at) - chatTimeMs(a.at));
    return gap < 5 * 60 * 1000;
  }

  function chatMessageHTML(message, opts = {}) {
    const bot = Boolean(message.is_bot);
    const kind = String(message.kind || "");
    const text = String(message.content || "");
    const card = bot ? parseCard(text) : null;
    const media = mediaHTML(message);
    let bubbleClass = "msg-bubble";
    let inner = "";
    if (card) {
      inner = cardBubble(card);
      bubbleClass += " is-card";
    } else if (media) {
      const caption = text && kind !== "sticker" ? `<div class="msg-caption">${esc(text)}</div>` : "";
      inner = media + caption;
      bubbleClass += " has-media";
      if (kind === "sticker") bubbleClass += " is-sticker";
    } else {
      inner = `<div class="msg-text">${esc(text || "（空消息）")}</div>`;
    }
    const actions = message.id ? `<span class="msg-actions">
      <button class="msg-action" type="button" data-act="chat-quote" data-id="${esc(message.id)}" aria-label="引用" title="引用">${icon("message")}</button>
      <button class="msg-action" type="button" data-act="chat-recall" data-id="${esc(message.id)}" aria-label="撤回" title="撤回">${icon("trash")}</button>
    </span>` : "";
    const avatar = bot ? "" : `<div class="msg-avatar-slot">${opts.grouped ? "" : `<span class="avatar msg-avatar ${avatarClass(message.username || message.member_openid)}">${esc(initial(message.username))}</span>`}</div>`;
    const author = !bot && !opts.grouped ? `<div class="msg-author">${esc(message.username || "成员")}</div>` : "";
    const meta = opts.lastOfRun ? `<div class="msg-meta">
      <span class="msg-time">${esc(fmtTime(message.at))}</span>
      ${bot && message.state === "sent" ? `<span class="msg-tick" title="平台已受理；不代表已送达或已读">${icon("check")}</span>` : ""}
    </div>` : "";
    return `<div class="msg ${bot ? "msg-out" : "msg-in"}${opts.grouped ? " is-grouped" : ""}" data-msg-id="${esc(message.id || "")}">
      ${avatar}
      <div class="msg-body">${author}<div class="${bubbleClass}">${inner}</div>${meta}${actions}</div>
    </div>`;
  }

  function chatMessagesHTML(messages) {
    if (!messages || !messages.length) {
      return `<div class="chat-empty">
        <div class="empty-icon">${icon("message")}</div>
        <strong>还没有消息</strong>
        <span>群里的新消息会自动出现在这里，并保存在本机。</span>
      </div>`;
    }
    const out = [];
    let lastDay = "";
    for (let i = 0; i < messages.length; i++) {
      const message = messages[i];
      const day = fmtDate(message.at);
      if (day && day !== lastDay) {
        out.push(`<div class="chat-day"><span>${esc(day)}</span></div>`);
        lastDay = day;
      }
      if (String(message.state || "") === "recalled") {
        const who = message.is_bot ? "LianT" : (message.username || "成员");
        out.push(`<div class="chat-system">${icon("info")}<span>${esc(who)} 撤回了一条消息</span></div>`);
        continue;
      }
      const prev = i > 0 ? messages[i - 1] : null;
      const next = i + 1 < messages.length ? messages[i + 1] : null;
      const grouped = Boolean(prev && sameChatRun(prev, message));
      const lastOfRun = !next || !sameChatRun(message, next);
      out.push(chatMessageHTML(message, { grouped, lastOfRun }));
    }
    return out.join("");
  }

  function chatSignature(messages) {
    return (messages || []).map(m => String(m.id || "") + ":" + String(m.at || "") + ":" + String(m.content || "") + ":" + String((m.media && m.media.state) || "")).join("|");
  }

  function chatTopHTML(group) {
    const name = group.name || "未命名群";
    const sub = group.note || (group.bot_is_admin ? "机器人是管理员" : "机器人已加入");
    return `<header class="chat-top">
      <button class="icon-btn chat-groups-btn" type="button" data-act="chat-groups-toggle" aria-label="会话列表">${icon("menu")}</button>
      <div class="chat-peer">
        <span class="avatar chat-peer-avatar ${avatarClass(name)}">${esc(initial(name))}</span>
        <div class="chat-peer-text"><h2>${esc(name)}</h2><span>${esc(sub)}</span></div>
      </div>
      <div class="chat-top-actions">
        <button class="icon-btn" type="button" data-act="chat-refresh" aria-label="刷新消息" title="刷新消息">${icon("refresh")}</button>
      </div>
    </header>`;
  }

  function chatComposerHTML() {
    const format = state.chat.format;
    const draft = state.chat.draft;
    const disabled = draft.content.trim() ? "" : " disabled";
    return `<div class="chat-composer">
      <form data-form="chat-send" id="chat-form">
        <div class="composer-box">
          <div class="composer-title" data-chat-title-wrap${format === "text" ? " hidden" : ""}>
            <input class="input chat-title" name="title" maxlength="60" placeholder="卡片标题（可选）" value="${esc(draft.title)}" autocomplete="off">
          </div>
          <textarea class="input chat-textarea" name="content" rows="1" maxlength="1800" placeholder="${format === "card" ? "输入卡片内容，支持 **加粗**、列表和链接…" : "发消息…"}" aria-label="消息内容">${esc(draft.content)}</textarea>
          <div class="composer-bar">
            <div class="composer-tools">
              <div class="segmented composer-seg" role="group" aria-label="消息格式">
                <button class="seg" type="button" data-act="chat-format" data-format="text" aria-pressed="${format === "text"}">文本</button>
                <button class="seg" type="button" data-act="chat-format" data-format="card" aria-pressed="${format === "card"}">卡片</button>
              </div>
              <select class="chat-template" data-chat-template aria-label="插入模板">
                <option value="">模板</option>
                ${CARD_TEMPLATES.map(t => `<option value="${t.id}">${esc(t.label)}</option>`).join("")}
              </select>
            </div>
            <span class="composer-count" data-chat-count>${draft.content.length} / 1800</span>
            <button class="chat-send" type="submit" data-chat-send aria-label="发送" title="发送"${disabled}>${icon("send")}</button>
          </div>
        </div>
        <p class="chat-note">Enter 发送 · Shift + Enter 换行 · 消息保存在本机</p>
      </form>
    </div>`;
  }

  function chatGroupsHTML() {
    if (!state.chat.groups.length) return `<p class="chat-side-empty">还没有已启用的群</p>`;
    return state.chat.groups.map(group => {
      const active = group.openid === state.chat.group;
      const preview = group.last_text
        ? (group.last_user ? group.last_user + "：" : "") + group.last_text
        : "还没有消息";
      return `<button class="chat-group${active ? " is-active" : ""}" type="button" data-act="chat-group" data-gid="${esc(group.openid)}" data-search="${esc((group.name || "") + " " + group.openid + " " + preview)}">
        <span class="avatar chat-group-avatar ${avatarClass(group.name || group.openid)}">${esc(initial(group.name))}</span>
        <span class="chat-group-text">
          <span class="chat-group-row"><strong>${esc(group.name || "未命名群")}</strong><span class="chat-group-time">${esc(group.last_at ? fmtRelative(group.last_at) : "")}</span></span>
          <span class="chat-group-preview">${esc(plainPreview(preview, 42))}</span>
        </span>
      </button>`;
    }).join("");
  }

  function toggleChatGroups(open) {
    const app = document.querySelector(".chat-app");
    if (!app) return;
    const next = open === undefined ? !app.classList.contains("is-groups-open") : Boolean(open);
    app.classList.toggle("is-groups-open", next);
  }

  async function renderChat() {
    const groups = await api("chat/groups");
    state.chat.groups = Array.isArray(groups) ? groups.slice() : [];
    state.chat.groups.sort((a, b) => {
      const ta = a.last_at ? Date.parse(a.last_at) || 0 : 0;
      const tb = b.last_at ? Date.parse(b.last_at) || 0 : 0;
      if (ta !== tb) return tb - ta;
      return String(a.name || "").localeCompare(String(b.name || ""), "zh-CN");
    });
    let gid = state.chat.group;
    if (!state.chat.groups.some(g => g.openid === gid)) gid = state.chat.groups.length ? state.chat.groups[0].openid : "";
    state.chat.group = gid;
    if (gid) store.set(CHAT_GROUP_KEY, gid);
    state.chat.error = "";
    state.chat.messages = [];
    if (gid) {
      try {
        const data = await api("chat/recent?group=" + encodeURIComponent(gid));
        state.chat.messages = data && Array.isArray(data.messages) ? data.messages : [];
        state.chat.signature = chatSignature(state.chat.messages);
      } catch (err) {
        if (err && err.status === 401) throw err;
        state.chat.error = err && err.message ? err.message : String(err);
      }
    }
    return chatPageHTML();
  }

  function chatPageHTML() {
    const groups = state.chat.groups;
    const current = groups.find(g => g.openid === state.chat.group) || null;
    const timeline = state.chat.error
      ? `<div class="chat-timeline" id="chat-timeline"><div class="chat-error">${icon("alert")}<span>${esc(state.chat.error)}</span><button class="btn sm" type="button" data-act="chat-refresh">重试</button></div></div>`
      : `<div class="chat-timeline" id="chat-timeline">${chatMessagesHTML(state.chat.messages)}</div>`;
    const main = current
      ? `${chatTopHTML(current)}${timeline}${chatComposerHTML()}`
      : `<div class="chat-empty">
          <div class="empty-icon">${icon("users")}</div>
          <strong>还没有可用的会话</strong>
          <span>机器人被拉进群并收到消息后会自动登记；也可以先去「群」里添加并启用。</span>
          <a class="btn primary" href="#groups">去群设置</a>
        </div>`;
    return `<div class="chat-app">
      <button class="chat-side-scrim" type="button" data-act="chat-groups-close" aria-label="关闭会话列表"></button>
      <aside class="chat-side" aria-label="会话列表">
        <div class="chat-side-head">
          <div><h1>消息</h1><p>${groups.length} 个会话</p></div>
          <button class="icon-btn" type="button" data-act="refresh" aria-label="刷新" title="刷新">${icon("refresh")}</button>
        </div>
        <label class="chat-search">${icon("search")}<input type="search" data-chat-search placeholder="搜索会话" aria-label="搜索会话"></label>
        <div class="chat-group-list" id="chat-group-list">${chatGroupsHTML()}</div>
        <p class="chat-side-foot">消息保存在本机 SQLite</p>
      </aside>
      <section class="chat-main">${main}</section>
    </div>`;
  }

  function stopChatPoll() {
    if (state.chat.timer) {
      clearInterval(state.chat.timer);
      state.chat.timer = 0;
    }
  }

  let chatSSE = null;
  function stopChatSSE() {
    if (chatSSE) {
      try { chatSSE.close(); } catch {}
      chatSSE = null;
    }
  }
  function startChatSSE() {
    stopChatSSE();
    if (typeof EventSource === "undefined" || !state.botID || !state.chat.group) return;
    const url = BASE + "/api/events?bot_id=" + encodeURIComponent(state.botID) + "&chat_id=" + encodeURIComponent(state.chat.group);
    chatSSE = new EventSource(url);
    chatSSE.addEventListener("message", () => {
      if (state.tab === "chat") refreshChatMessages(false).catch(() => {});
    });
  }
  function startChatPoll() {
    stopChatPoll();
    if (state.tab !== "chat" || !state.chat.group) {
      stopChatSSE();
      return;
    }
    // 有推送就不轮询；EventSource 不可用时退回 8 秒轮询。
    startChatSSE();
    if (!chatSSE) state.chat.timer = setInterval(() => refreshChatMessages(false), 8000);
  }

  async function refreshChatMessages(force) {
    const gid = state.chat.group;
    if (!gid || state.tab !== "chat") return;
    if (state.chat.refreshing && !force) return;
    if (document.hidden && !force) return;
    state.chat.refreshing = true;
    try {
      const data = await api("chat/recent?group=" + encodeURIComponent(gid));
      if (state.tab !== "chat" || state.chat.group !== gid) return;
      const messages = data && Array.isArray(data.messages) ? data.messages : [];
      const signature = chatSignature(messages);
      const hadError = Boolean(state.chat.error);
      state.chat.messages = messages;
      state.chat.error = "";
      if (signature === state.chat.signature && !force && !hadError) return;
      state.chat.signature = signature;
      const timeline = document.getElementById("chat-timeline");
      if (!timeline) return;
      const nearBottom = timeline.scrollHeight - timeline.scrollTop - timeline.clientHeight < 90;
      timeline.innerHTML = chatMessagesHTML(messages);
      if (nearBottom) timeline.scrollTop = timeline.scrollHeight;
    } catch (err) {
      if (err && err.status === 401) return;
    } finally {
      state.chat.refreshing = false;
    }
  }

  function captureChatDraft() {
    const form = document.querySelector('form[data-form="chat-send"]');
    if (!form) return;
    const title = form.elements.title ? form.elements.title.value : "";
    const content = form.elements.content ? form.elements.content.value : "";
    state.chat.draft = { title, content };
  }

  function updateChatComposerState() {
    const form = document.querySelector('form[data-form="chat-send"]');
    if (!form) return;
    const content = form.elements.content ? form.elements.content.value : "";
    const count = form.querySelector("[data-chat-count]");
    if (count) count.textContent = content.length + " / 1800";
    const send = form.querySelector("[data-chat-send]");
    if (send) send.disabled = !content.trim();
    const area = form.elements.content;
    if (area) {
      area.style.height = "auto";
      area.style.height = Math.min(area.scrollHeight || 0, 128) + "px";
    }
    captureChatDraft();
  }

  function switchChatGroup(gid) {
    if (!gid || gid === state.chat.group) return;
    captureChatDraft();
    state.chat.group = gid;
    store.set(CHAT_GROUP_KEY, gid);
    state.chat.error = "";
    state.chat.messages = [];
    state.chat.signature = "";
    loadCurrent({ quiet: true });
  }

  function setChatFormat(format) {
    const next = format === "text" ? "text" : "card";
    if (state.chat.format === next) return;
    captureChatDraft();
    state.chat.format = next;
    store.set(CHAT_FORMAT_KEY, next);
    const form = document.querySelector('form[data-form="chat-send"]');
    if (!form) return;
    form.querySelectorAll('[data-act="chat-format"]').forEach(btn => {
      btn.setAttribute("aria-pressed", String(btn.dataset.format === next));
    });
    const wrap = form.querySelector("[data-chat-title-wrap]");
    if (wrap) wrap.hidden = next === "text";
    const hint = form.querySelector(".composer-hint");
    if (hint) hint.textContent = next === "card" ? "Markdown · Telegram HTML 卡片" : "纯文本 · 直接发送";
    const area = form.elements.content;
    if (area) {
      area.placeholder = next === "card" ? "输入卡片内容，支持 **加粗**、列表和链接…" : "发消息…";
      area.focus();
    }
  }

  function applyCardTemplate(id) {
    const template = CARD_TEMPLATES.find(item => item.id === id);
    if (!template) return;
    const form = document.querySelector('form[data-form="chat-send"]');
    if (!form) return;
    setChatFormat("card");
    state.chat.draft = { title: template.title, content: template.content };
    if (form.elements.title) form.elements.title.value = template.title;
    form.elements.content.value = template.content;
    updateChatComposerState();
    form.elements.content.focus();
  }

  function openMediaDialog(src, name) {
    const dialog = document.getElementById("media-dialog");
    const img = document.getElementById("media-dialog-img");
    if (!dialog || !img || !src) return;
    img.src = src;
    img.alt = name || "图片";
    if (typeof dialog.showModal === "function") dialog.showModal();
    else dialog.setAttribute("open", "");
    syncScrollLock();
  }

  function quoteChatMessage(id) {
    const message = state.chat.messages.find(item => String(item.id) === String(id));
    if (!message) throw new Error("这条消息已经不在缓存里了");
    const form = document.querySelector('form[data-form="chat-send"]');
    if (!form) return;
    const card = parseCard(message.content);
    if (card) {
      setChatFormat("card");
      if (form.elements.title) form.elements.title.value = card.title;
      form.elements.content.value = card.body;
    } else {
      const mediaLabel = message.media ? (MEDIA_KIND_LABEL[String(message.media.kind)] || "附件") : "";
      form.elements.content.value = message.content || (mediaLabel ? "[" + mediaLabel + "]" : "");
    }
    updateChatComposerState();
    form.elements.content.focus();
    toast("已填入输入框", { type: "info" });
  }

  async function recallChatMessage(id) {
    if (!id) return;
    const ok = await confirmAction({
      title: "撤回这条消息？",
      message: "将通过平台接口撤回。群成员的消息受平台撤回时限约束。",
      confirmText: "撤回"
    });
    if (!ok) return;
    const data = await api("qq/recall", { method: "POST", body: { group_openid: state.chat.group, message_id: id } });
    toast.success(data && data.result ? data.result : "已撤回");
    await refreshChatMessages(true);
  }

  function filterChatGroups(query) {
    const q = String(query || "").trim().toLowerCase();
    document.querySelectorAll("#chat-group-list .chat-group").forEach(btn => {
      const hay = (btn.dataset.search || "") + " " + btn.textContent;
      btn.hidden = Boolean(q) && !hay.toLowerCase().includes(q);
    });
  }

  function scrollChatBottom() {
    const timeline = document.getElementById("chat-timeline");
    if (timeline) timeline.scrollTop = timeline.scrollHeight;
  }

  async function sendChatMessage(form) {
    const content = String(form.elements.content ? form.elements.content.value : "").trim();
    const title = String(form.elements.title ? form.elements.title.value : "").trim();
    const gid = state.chat.group;
    if (!gid) throw new Error("先选择一个群");
    if (!content) throw new Error("消息内容不能为空");
    const format = state.chat.format;
    const data = await api("chat/send", {
      method: "POST",
      timeout: 30000,
      body: { group_openid: gid, format, title, content }
    });
    const group = state.chat.groups.find(g => g.openid === gid);
    toast.success("已发送到「" + (group && group.name ? group.name : "群") + "」");
    state.chat.draft = { title: "", content: "" };
    if (form.elements.title) form.elements.title.value = "";
    form.elements.content.value = "";
    updateChatComposerState();
    const markdown = data && data.markdown
      ? data.markdown
      : (format === "card" ? "## " + (title || "群消息") + "\n\n" + content : content);
    state.chat.messages.push({
      id: data && data.id ? data.id : "",
      username: "LianT",
      content: markdown,
      kind: "bot",
      is_bot: true,
      at: new Date().toISOString()
    });
    const timeline = document.getElementById("chat-timeline");
    if (timeline) {
      timeline.innerHTML = chatMessagesHTML(state.chat.messages);
      scrollChatBottom();
    }
    setTimeout(() => { refreshChatMessages(true).catch(() => {}); }, 600);
  }

  /* ======================================================================
     表单：规则 / 违禁词 / 群 / 管理员 / 禁言 / 黑名单
     ====================================================================== */
  function scopeOptions(selected) {
    const options = [["", "全部群"], ...state.groups.map(g => [g.OpenID, groupOptionLabel(g)])];
    if (selected && !options.some(([value]) => value === selected)) options.push([selected, "未知群"]);
    return options;
  }

  async function openRuleForm(rule) {
    await groupsSafe().catch(() => []);
    const isEdit = Boolean(rule);
    openForm({
      title: isEdit ? "编辑回复规则" : "新增回复规则",
      desc: "优先级数字越小越先匹配，命中第一条就停止。",
      submitText: isEdit ? "保存修改" : "创建规则",
      body: `
        ${fieldHTML("优先级", inputHTML("priority", { type: "number", value: rule ? rule.Priority : 100, min: 0, required: true }), "数字越小越靠前")}
        ${fieldHTML("匹配方式", selectHTML("match", MATCHES, rule ? rule.Match : "exact"))}
        ${fieldHTML("匹配内容", inputHTML("pattern", { value: rule ? rule.Pattern : "", placeholder: "文本、前缀或正则", mono: true, maxlength: 300 }), "「任意消息」模式可以留空", "span-2")}
        ${fieldHTML("回复内容", textareaHTML("reply", { value: rule ? rule.Reply : "", placeholder: "留空表示命中后不回复", rows: 3, maxlength: 1500 }), "可以是多行文本", "span-2")}
        ${fieldHTML("限定群", selectHTML("scope", scopeOptions(rule ? rule.Scope : ""), rule ? rule.Scope : ""), "留空 = 所有群")}
        ${fieldHTML("备注", inputHTML("note", { value: rule ? rule.Note : "", placeholder: "可选", maxlength: 120 }))}
        <div class="span-2 switch-grid">
          ${switchHTML("fold", "忽略大小写", "仅对文本匹配生效", Boolean(rule && rule.Fold))}
          ${switchHTML("always", "总是回复", "无视「全量回复」开关", rule ? Boolean(rule.Always) : true)}
          ${switchHTML("enabled", "启用", "停用后不参与匹配", rule ? Boolean(rule.Enabled) : true)}
        </div>`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const body = {
          ID: isEdit ? rule.ID : 0,
          Priority: Number(fd.get("priority")) || 0,
          Match: String(fd.get("match") || "exact"),
          Pattern: String(fd.get("pattern") || "").trim(),
          Reply: String(fd.get("reply") || ""),
          Fold: fd.get("fold") === "on",
          Always: fd.get("always") === "on",
          Scope: String(fd.get("scope") || ""),
          Note: String(fd.get("note") || "").trim(),
          Enabled: fd.get("enabled") === "on"
        };
        if (!body.Pattern && body.Match !== "any") throw new Error("匹配内容不能为空");
        await api("rules", { method: "POST", body });
        await loadCurrent();
        return "规则已保存";
      }
    });
  }

  async function openWordForm(word) {
    await groupsSafe().catch(() => []);
    const isEdit = Boolean(word);
    openForm({
      title: isEdit ? "编辑违禁词" : "新增违禁词",
      desc: "优先级数字小的先匹配；命中后按下面的处置执行。",
      submitText: isEdit ? "保存修改" : "创建违禁词",
      body: `
        ${fieldHTML("词 / 正则", inputHTML("word", { value: word ? word.Word : "", placeholder: "例如：加群 / 广告", required: true, mono: true, maxlength: 120 }), "必填", "span-2")}
        ${fieldHTML("匹配方式", selectHTML("match", MATCHES, word ? word.Match : "contains"))}
        ${fieldHTML("限定群", selectHTML("scope", scopeOptions(word ? word.Scope : ""), word ? word.Scope : ""), "留空 = 所有群")}
        ${fieldHTML("警告回复", inputHTML("reply", { value: word ? word.Reply : "", placeholder: "可选，留空不警告", maxlength: 300 }), "命中后回复的内容", "span-2")}
        ${fieldHTML("禁言秒数", inputHTML("mute_seconds", { type: "number", value: word ? word.MuteSeconds : 0, min: 0, max: 2592000 }), "0 = 不禁言")}
        ${fieldHTML("优先级", inputHTML("severity", { type: "number", value: word ? word.Severity : 100, min: 0 }), "数字越小越先匹配")}
        ${fieldHTML("备注", inputHTML("note", { value: word ? word.Note : "", placeholder: "可选", maxlength: 120 }))}
        <div class="span-2 switch-grid">
          ${switchHTML("recall", "撤回消息", "命中后尝试撤回", word ? Boolean(word.Recall) : true)}
          ${switchHTML("kick", "踢出成员", "需要平台权限", Boolean(word && word.Kick))}
          ${switchHTML("blacklist", "拉黑成员", "需要平台权限", Boolean(word && word.Blacklist))}
          ${switchHTML("fold", "忽略大小写", "仅对文本匹配生效", Boolean(word && word.Fold))}
          ${switchHTML("enabled", "启用", "停用后不参与匹配", word ? Boolean(word.Enabled) : true)}
        </div>`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const body = {
          ID: isEdit ? word.ID : 0,
          Word: String(fd.get("word") || "").trim(),
          Match: String(fd.get("match") || "contains"),
          Scope: String(fd.get("scope") || ""),
          Reply: String(fd.get("reply") || ""),
          MuteSeconds: Number(fd.get("mute_seconds")) || 0,
          Severity: Number(fd.get("severity")) || 100,
          Note: String(fd.get("note") || "").trim(),
          Recall: fd.get("recall") === "on",
          Kick: fd.get("kick") === "on",
          Blacklist: fd.get("blacklist") === "on",
          Fold: fd.get("fold") === "on",
          Enabled: fd.get("enabled") === "on"
        };
        if (!body.Word) throw new Error("违禁词不能为空");
        await api("words", { method: "POST", body });
        await loadCurrent();
        return "违禁词已保存";
      }
    });
  }

  async function openGroupForm(group) {
    const isEdit = Boolean(group);
    openForm({
      title: isEdit ? "编辑群「" + (group.Name || "未命名群") + "」" : "添加群",
      desc: isEdit ? "修改群名、备注或启用状态；群名也可以让平台自动补全。" : "通常不用手动添加：机器人进群并收到消息后会自动登记。只有补录时才需要群 ID。",
      submitText: isEdit ? "保存修改" : "保存群",
      body: `
        ${isEdit ? "" : fieldHTML("群 ID", inputHTML("group_openid", { placeholder: "从群资料里复制一次", required: true, mono: true, autocomplete: false }), "只在手动补录时需要；之后都按群名选择", "span-2")}
        ${fieldHTML("群名称", inputHTML("name", { value: isEdit ? group.Name : "", placeholder: "例如：开发交流群", maxlength: 80 }), "没有「群信息」白名单时可手动填写")}
        ${fieldHTML("备注", inputHTML("note", { value: isEdit ? group.Note : "", placeholder: "可选", maxlength: 120 }))}
        <div class="span-2">${switchHTML("enabled", "启用", "关闭后机器人忽略该群的消息", isEdit ? Boolean(group.Enabled) : true)}</div>`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const gid = isEdit ? group.OpenID : String(fd.get("group_openid") || "").trim();
        if (!gid) throw new Error("群 ID 不能为空");
        await api("groups", {
          method: "POST",
          body: {
            group_openid: gid,
            name: String(fd.get("name") || ""),
            note: String(fd.get("note") || ""),
            enabled: fd.get("enabled") === "on"
          }
        });
        state.groups = [];
        await loadCurrent();
        return isEdit ? "群资料已保存" : "群已保存";
      }
    });
  }

  async function openAdminForm() {
    await groupsSafe().catch(() => []);
    await loadMembers().catch(() => []);
    const options = [["", "全局（所有群）"], ...state.groups.map(g => [g.OpenID, groupOptionLabel(g)])];
    openForm({
      title: "添加管理员",
      desc: "从群名册里选人；全局管理员在所有群可用，群级管理员只在该群可用。",
      submitText: "添加",
      body: `
        ${fieldBlock("成员", memberSelectHTML("member_openid"), "从机器人见过的群成员里选择", "span-2")}
        ${fieldHTML("显示昵称", inputHTML("username", { placeholder: "选择成员后自动填入", maxlength: 40 }))}
        ${fieldHTML("角色", selectHTML("role", [["admin", "管理员"], ["super", "超级管理员"]], "admin"))}
        ${fieldHTML("生效范围", selectHTML("group_openid", options, ""), "全局管理员留空", "span-2")}`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const mid = memberFromForm(fd);
        if (!mid) throw new Error("请选择成员，或展开手动输入");
        await api("admins", {
          method: "POST",
          body: {
            MemberOpenID: mid,
            Username: String(fd.get("username") || ""),
            Role: String(fd.get("role") || "admin"),
            GroupOpenID: String(fd.get("group_openid") || "")
          }
        });
        await loadCurrent();
        return "管理员已添加";
      }
    });
  }

  async function openMuteForm() {
    await groupsSafe().catch(() => []);
    await loadMembers().catch(() => []);
    openForm({
      title: "手动禁言",
      desc: "从名册里选人，系统会自动带出所在群；平台侧要求机器人是群管理员，最长 30 天。",
      submitText: "禁言",
      body: `
        ${fieldHTML("群", groupSelectHTML("group_openid", { required: true }), "从已登记的群里选")}
        ${fieldBlock("成员", memberSelectHTML("member_openid", { autoGroup: true }), "选成员会自动带出所在群")}
        ${fieldHTML("时长（秒）", inputHTML("seconds", { type: "number", value: 600, min: 1, max: 2592000 }), "默认 600 秒，最长 30 天")}
        ${fieldHTML("原因", inputHTML("reason", { placeholder: "网页后台禁言", maxlength: 80 }))}`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const gid = String(fd.get("group_openid") || "").trim();
        const mid = memberFromForm(fd);
        if (!gid) throw new Error("请选择群");
        if (!mid) throw new Error("请选择成员");
        const data = await api("mutes", {
          method: "POST",
          body: {
            group_openid: gid,
            member_openid: mid,
            seconds: Number(fd.get("seconds")) || 600,
            reason: String(fd.get("reason") || "")
          }
        });
        await loadCurrent();
        return Array.isArray(data) ? data.join("；") : "已禁言";
      }
    });
  }

  async function openBlackForm() {
    await groupsSafe().catch(() => []);
    await loadMembers().catch(() => []);
    const options = [["", "全局（所有群）"], ...state.groups.map(g => [g.OpenID, groupOptionLabel(g)])];
    openForm({
      title: "拉黑成员",
      desc: "本地立即生效；勾选踢出需要平台权限。全局拉黑会对所有群生效。",
      submitText: "加入黑名单",
      body: `
        ${fieldHTML("生效范围", selectHTML("group_openid", options, ""), "全局拉黑对所有群生效", "span-2")}
        ${fieldBlock("成员", memberSelectHTML("member_openid"), "从机器人见过的群成员里选择", "span-2")}
        ${fieldHTML("原因", inputHTML("reason", { placeholder: "网页后台拉黑", maxlength: 80 }), "", "span-2")}
        <div class="span-2">${switchHTML("kick", "同时踢出", "需要平台权限权限", false)}</div>`,
      onSubmit: async form => {
        const fd = new FormData(form);
        const mid = memberFromForm(fd);
        if (!mid) throw new Error("请选择成员，或展开手动输入");
        const data = await api("blacklist", {
          method: "POST",
          body: {
            group_openid: String(fd.get("group_openid") || ""),
            member_openid: mid,
            reason: String(fd.get("reason") || ""),
            kick: fd.get("kick") === "on"
          }
        });
        await loadCurrent();
        return Array.isArray(data) ? data.join("；") : "已加入黑名单";
      }
    });
  }

  /* ======================================================================
     数据操作
     ====================================================================== */
  async function deleteRule(id) {
    const rule = state.rules.find(item => String(item.ID) === String(id));
    const ok = await confirmAction({
      title: "删除回复规则？",
      message: "将删除" + (rule && rule.Pattern ? "「" + truncate(rule.Pattern, 38) + "」" : "这条规则") + "，该操作不可撤销。",
      confirmText: "删除"
    });
    if (!ok) return;
    await api("rules?id=" + encodeURIComponent(id), { method: "DELETE" });
    toast.success("规则已删除");
    await loadCurrent();
  }

  async function deleteWord(id) {
    const word = state.words.find(item => String(item.ID) === String(id));
    const ok = await confirmAction({
      title: "删除违禁词？",
      message: "将删除「" + (word && word.Word ? truncate(word.Word, 30) : "这条违禁词") + "」，该操作不可撤销。",
      confirmText: "删除"
    });
    if (!ok) return;
    await api("words?id=" + encodeURIComponent(id), { method: "DELETE" });
    toast.success("违禁词已删除");
    await loadCurrent();
  }

  async function deleteAdmin(id) {
    const admin = state.admins.find(item => String(item.ID) === String(id));
    const ok = await confirmAction({
      title: "删除管理员？",
      message: "将移除「" + (admin && admin.Username ? truncate(admin.Username, 34) : memberName(admin && admin.MemberOpenID)) + "」的管理权限。",
      confirmText: "删除"
    });
    if (!ok) return;
    await api("admins?id=" + encodeURIComponent(id), { method: "DELETE" });
    toast.success("管理员已删除");
    await loadCurrent();
  }

  async function deleteGroup(gid) {
    const group = state.groups.find(item => item.OpenID === gid);
    const ok = await confirmAction({
      title: "删除群记录？",
      message: "将删除「" + (group && group.Name ? group.Name : "这个群") + "」，并清掉该群的禁言和黑名单记录。不会让机器人退群。",
      confirmText: "删除"
    });
    if (!ok) return;
    await api("groups?group_openid=" + encodeURIComponent(gid), { method: "DELETE" });
    state.groups = [];
    toast.success("群记录已删除");
    await loadCurrent();
  }

  async function syncGroupNames(opts = {}) {
    if (state.groupSyncBusy) return;
    state.groupSyncBusy = true;
    try {
      const data = await api("groups/sync-names", { method: "POST", timeout: 40000 });
      if (data && data.permission === false) {
        state.groupSyncDenied = true;
        toast("机器人没有「群信息」白名单，无法自动获取群名；点群卡片的编辑按钮可以手动填写。", { type: "info", duration: 8000 });
        return;
      }
      if (data && data.updated > 0) {
        if (!opts.silent) toast.success("已补全 " + data.updated + " 个群名");
        await loadCurrent({ quiet: true });
        return;
      }
      if (!opts.silent) {
        if (data && Array.isArray(data.problems) && data.problems.length) {
          toast(data.problems[0], { type: "info", duration: 6000 });
        } else if (data && data.remaining > 0) {
          toast("还有群没有补全，稍后可再点一次", { type: "info" });
        } else {
          toast.success("所有群名都已补全");
        }
      }
    } catch (err) {
      if (!opts.silent) toast.error(err && err.message ? err.message : String(err));
    } finally {
      state.groupSyncBusy = false;
    }
  }

  async function toggleGroup(input) {
    const gid = input.dataset.gid;
    const field = input.dataset.field;
    const apiField = GROUP_FIELD[field];
    if (!gid || !apiField) return;
    const checked = input.checked;
    input.disabled = true;
    try {
      const body = { group_openid: gid };
      body[apiField] = checked;
      await api("groups", { method: "POST", body });
      const group = state.groups.find(item => item.OpenID === gid);
      if (group) group[field] = checked;
      const card = input.closest(".group-card");
      const flash = card && card.querySelector("[data-save]");
      if (flash) {
        flash.classList.add("on");
        setTimeout(() => flash.classList.remove("on"), 1400);
      }
    } catch (err) {
      input.checked = !checked;
      throw err;
    } finally {
      input.disabled = false;
    }
  }

  function handleMemberSelect(select) {
    const form = select.form;
    if (!form) return;
    const option = select.selectedOptions[0];
    if (!option) return;
    const gid = option.dataset.group || "";
    const name = option.dataset.name || "";
    const nameInput = form.querySelector('input[name="username"]');
    if (nameInput && name) nameInput.value = name;
    const manual = form.querySelector('input[name="member_manual"]');
    if (manual && manual.value) manual.value = "";
    if (select.dataset.autoGroup === "1" && gid) {
      const groupField = form.elements.group_openid;
      if (groupField) {
        groupField.value = gid;
        if (form.querySelector("[data-qq-messages]")) {
          loadQQMessages(gid, form).catch(err => toast.error(err && err.message ? err.message : String(err)));
        }
      }
    }
  }

  async function queryGroupStatus(gid) {
    const data = await api("qq/botstate?group=" + encodeURIComponent(gid));
    if (data && data.ok) toast(data.result || "已查询", { type: "info", duration: 6000 });
    else toast.error(data && data.result ? data.result : "查询失败");
  }

  async function queryMuteState() {
    const select = document.getElementById("mute-state-group");
    const out = document.getElementById("mute-state-out");
    if (!select || !out) return;
    if (!select.value) throw new Error("先选择一个群");
    const data = await api("qq/mutestate?group=" + encodeURIComponent(select.value));
    if (data && data.ok === false) {
      out.innerHTML = `<div class="callout danger mt-12">${icon("alert")}<span>${esc(data.error || "查询失败")}${data.Hint ? " —— " + esc(data.Hint) : ""}</span></div>`;
      return;
    }
    const members = data && Array.isArray(data.members) ? data.members : [];
    const mode = String((data && data.global_mode) || "none");
    const modeLabel = mode === "all" ? "全员禁言" : mode === "none" ? "未开启" : mode;
    out.innerHTML = `<div class="callout mt-12">${icon("info")}<span>全员禁言模式：<b>${esc(modeLabel)}</b> · 禁言中 ${members.length} 人</span></div>` +
      (members.length
        ? `<div class="chip-row">${members.map(member => `<span class="badge">${esc(member.username || memberName(member.member_openid))}</span>`).join("")}</div>`
        : "");
  }

  async function unmute(gid, mid) {
    const data = await api("mutes?group=" + encodeURIComponent(gid) + "&member=" + encodeURIComponent(mid), { method: "DELETE" });
    toast.success(Array.isArray(data) ? data.join("；") : "已解除");
    await loadCurrent();
  }

  async function removeBlack(gid, mid) {
    const data = await api("blacklist?group=" + encodeURIComponent(gid || "") + "&member=" + encodeURIComponent(mid), { method: "DELETE" });
    const qq = data && data.qq ? String(data.qq) : "";
    if (qq && !/^ok$/i.test(qq)) {
      toast("本地已移出；平台侧：" + qq, { type: "info", duration: 6000 });
    } else {
      toast.success("已移出黑名单");
    }
    await loadCurrent();
  }

  async function approveJoin(el) {
    const data = await api("joins", {
      method: "POST",
      body: {
        group_openid: el.dataset.gid,
        member_openid: el.dataset.mid,
        join_request_id: el.dataset.rid,
        approve: true
      }
    });
    if (data && data.ok === false) throw new Error(data.result || "审批失败");
    toast.success(data && data.result ? data.result : "已通过");
    await loadCurrent();
  }

  async function rejectJoin(el) {
    const name = el.dataset.name || el.dataset.mid || "该成员";
    const ok = await confirmAction({
      title: "拒绝入群申请？",
      message: "将拒绝「" + name + "」的申请，并同步加入群黑名单。",
      confirmText: "拒绝并拉黑"
    });
    if (!ok) return;
    const data = await api("joins", {
      method: "POST",
      body: {
        group_openid: el.dataset.gid,
        member_openid: el.dataset.mid,
        join_request_id: el.dataset.rid,
        approve: false,
        reason: "网页后台拒绝",
        add_to_member_blacklist: true
      }
    });
    if (data && data.ok === false) throw new Error(data.result || "拒绝失败");
    toast.success(data && data.result ? data.result : "已拒绝");
    await loadCurrent();
  }

  async function viewQQ(kind) {
    const form = document.querySelector('form[data-form="qq-view"]');
    if (!form) return;
    const gid = String(form.elements.group_openid.value || "").trim();
    if (!gid) throw new Error("请选择群");
    let data;
    if (kind === "groupinfo") data = await api("qq/groupinfo?group=" + encodeURIComponent(gid));
    else if (kind === "members") data = await api("qq/members?group=" + encodeURIComponent(gid));
    else data = await api("qq/mutestate?group=" + encodeURIComponent(gid));
    if (data && data.ok === false) {
      throw new Error((data.error || "查询失败") + (data.Hint || data.hint ? " —— " + (data.Hint || data.hint) : ""));
    }
    const title = kind === "groupinfo" ? "群信息" : kind === "members" ? "群成员" : "禁言状态";
    showQQOutput(title, kind === "members" ? (data.members || data) : data);
  }

  function showQQOutput(title, data) {
    const wrap = document.getElementById("qq-output");
    const out = document.getElementById("qq-out");
    const label = document.getElementById("qq-output-title");
    if (!wrap || !out || !label) return;
    label.textContent = title;
    out.textContent = typeof data === "string" ? data : JSON.stringify(data, null, 2);
    wrap.hidden = false;
  }

  async function exportConfig() {
    const headers = {};
    if (state.token) headers.Authorization = "Bearer " + state.token;
    const exportURL = BASE + "/api/export" + (state.botID ? "?bot_id=" + encodeURIComponent(state.botID) : "");
    const response = await fetch(exportURL, { headers, credentials: "same-origin" });
    if (response.status === 401) throw new Error("未授权，请检查管理端 token");
    if (!response.ok) throw new Error("导出失败（HTTP " + response.status + "）");
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "liant-config.json";
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
    toast.success("配置已导出");
  }

  async function reloadBot() {
    const ok = await confirmAction({
      title: "重载规则与凭据？",
      message: "会重建网关连接，期间可能短暂离线；日常改规则不需要重载。",
      confirmText: "重载",
      tone: "primary"
    });
    if (!ok) return;
    const result = await api("reload", { method: "POST" });
    toast.success(String(result || "已重载"));
  }

  async function pruneAudit() {
    const ok = await confirmAction({
      title: "清理旧日志？",
      message: "只保留最近 2000 条审计记录，更早的记录会被删除。",
      confirmText: "清理"
    });
    if (!ok) return;
    await api("audit/prune?keep=2000");
    toast.success("已清理旧日志");
    await loadCurrent();
  }

  /* ======================================================================
     表单提交
     ====================================================================== */
  async function formAuth(form) {
    const token = String(new FormData(form).get("token") || "").trim();
    if (!token) throw new Error("请输入 token");
    const url = new URL(location.href);
    url.searchParams.set("token", token);
    location.replace(url.pathname + url.search + url.hash);
    return;
  }

  async function saveSettings(form) {
    const elements = form.elements;
    const body = {
      reply_all: elements.reply_all.checked ? "true" : "false",
      reply_default: elements.reply_default.value,
      join_auto_approve: elements.join_auto_approve.checked ? "true" : "false"
    };
    await api("settings", { method: "POST", body });
    const dirty = form.querySelector("[data-dirty]");
    if (dirty) dirty.hidden = true;
    toast.success("已保存，下条消息生效");
  }

  async function qqRecall(form) {
    const fd = new FormData(form);
    const gid = String(form.elements.group_openid.value || "").trim();
    if (!gid) throw new Error("请选择群");
    const body = { group_openid: gid };
    const messageID = String(form.elements.message_id ? form.elements.message_id.value : "").trim();
    const memberID = memberFromForm(fd);
    if (messageID) body.message_id = messageID;
    else if (memberID) body.member_openid = memberID;
    else body.count = Number(fd.get("count")) || 1;
    const data = await api("qq/recall", { method: "POST", body });
    toast.success(data && data.result ? data.result : "已执行撤回");
    await loadCurrent({ quiet: true });
  }

  async function qqKick(form) {
    const fd = new FormData(form);
    const gid = String(form.elements.group_openid.value || "").trim();
    const mid = memberFromForm(fd);
    if (!gid) throw new Error("请选择群");
    if (!mid) throw new Error("请选择成员");
    const data = await api("qq/kick", {
      method: "POST",
      body: {
        group_openid: gid,
        member_openids: [mid],
        add_to_member_blacklist: fd.get("add_to_member_blacklist") === "on"
      }
    });
    if (data && data.ok === false) toast.error(data.result || "踢出失败");
    else toast.success(data && data.result ? data.result : "已踢出");
    await loadCurrent({ quiet: true });
  }

  function markDirty() {
    const form = document.querySelector('form[data-form="settings"]');
    if (!form) return;
    const dirty = form.querySelector("[data-dirty]");
    if (dirty) dirty.hidden = false;
  }

  function applyFilters(card) {
    if (!card) return;
    const search = card.querySelector("[data-filter]");
    const kind = card.querySelector("[data-filter-kind]");
    const query = search ? search.value.trim().toLowerCase() : "";
    const kindValue = kind ? kind.value : "";
    let shown = 0;
    card.querySelectorAll("tbody tr[data-search]").forEach(row => {
      const haystack = (row.dataset.search + " " + row.textContent).toLowerCase();
      const hit = (!query || haystack.includes(query)) && (!kindValue || row.dataset.kind === kindValue);
      row.hidden = !hit;
      if (hit) shown++;
    });
    const count = card.querySelector("[data-count]");
    if (count) count.textContent = String(shown);
    const none = card.querySelector("[data-no-results]");
    if (none) none.hidden = shown > 0;
  }

  /* ======================================================================
     事件映射
     ====================================================================== */
  const ACTIONS = {
    refresh: () => loadCurrent(),
    "enter-bot": el => enterBot(el.dataset.id),
    "back-bots": () => { location.hash = "#bots"; },
    "new-bot": () => openBotForm(null),
    "edit-bot": el => {
      const bot = botByID(el.dataset.id);
      if (bot) return openBotForm(bot);
    },
    "use-bot": el => {
      state.botID = String(el.dataset.id || "");
      store.set("liant_bot", state.botID);
      updateBotSelect();
      toast.success("已切换 Bot");
      return loadCurrent();
    },
    "restart-bot": async el => {
      await api("bots/restart", { method: "POST", body: { id: Number(el.dataset.id) } });
      await loadBots(el.dataset.id);
      toast.success("已重启");
      await loadCurrent();
    },
    "delete-bot": async el => {
      const bot = botByID(el.dataset.id);
      const ok = await confirmAction({
        title: "删除这个 Bot？",
        message: "将停止「" + (bot && bot.name ? bot.name : "该 Bot") + "」并删除注册信息；业务库文件会保留。",
        confirmText: "删除"
      });
      if (!ok) return;
      await api("bots?id=" + encodeURIComponent(el.dataset.id), { method: "DELETE" });
      await loadBots();
      toast.success("已删除");
      await loadCurrent();
    },
    retry: () => loadCurrent(),
    "open-token": () => openToken(),
    copy: el => copyText(el.dataset.copy),
    "export-config": () => exportConfig(),
    "reload-bot": () => reloadBot(),
    "prune-audit": () => pruneAudit(),
    "new-rule": () => openRuleForm(null),
    "edit-rule": el => {
      const rule = state.rules.find(item => String(item.ID) === String(el.dataset.id));
      if (rule) return openRuleForm(rule);
    },
    "delete-rule": el => deleteRule(el.dataset.id),
    "new-word": () => openWordForm(null),
    "edit-word": el => {
      const word = state.words.find(item => String(item.ID) === String(el.dataset.id));
      if (word) return openWordForm(word);
    },
    "delete-word": el => deleteWord(el.dataset.id),
    "new-group": () => openGroupForm(null),
    "edit-group": el => {
      const group = state.groups.find(item => item.OpenID === el.dataset.gid);
      if (group) return openGroupForm(group);
    },
    "sync-group-names": () => syncGroupNames({ silent: false }),
    "delete-group": el => deleteGroup(el.dataset.gid),
    "group-status": el => queryGroupStatus(el.dataset.gid),
    "new-admin": () => openAdminForm(),
    "delete-admin": el => deleteAdmin(el.dataset.id),
    "new-mute": () => openMuteForm(),
    "unmute": el => unmute(el.dataset.gid, el.dataset.mid),
    "mute-state": () => queryMuteState(),
    "new-black": () => openBlackForm(),
    "remove-black": el => removeBlack(el.dataset.gid, el.dataset.mid),
    "approve-join": el => approveJoin(el),
    "reject-join": el => rejectJoin(el),
    "qq-groupinfo": () => viewQQ("groupinfo"),
    "qq-members": () => viewQQ("members"),
    "qq-mutestate": () => viewQQ("mutestate"),
    "copy-output": () => copyText(document.getElementById("qq-out") ? document.getElementById("qq-out").textContent : ""),
    "chat-group": el => switchChatGroup(el.dataset.gid),
    "chat-refresh": () => refreshChatMessages(true),
    "chat-groups-toggle": () => toggleChatGroups(),
    "chat-groups-close": () => toggleChatGroups(false),
    "chat-format": el => setChatFormat(el.dataset.format),
    "chat-quote": el => quoteChatMessage(el.dataset.id),
    "chat-recall": el => recallChatMessage(el.dataset.id),
    "media-open": el => openMediaDialog(el.dataset.src, el.dataset.name)
  };

  const FORMS = {
    auth: formAuth,
    settings: saveSettings,
    "qq-recall": qqRecall,
    "qq-kick": qqKick,
    "chat-send": sendChatMessage
  };

  const RENDER = {
    overview: renderOverview,
    chat: renderChat,
    rules: renderRules,
    words: renderWords,
    joins: renderJoins,
    groups: renderGroups,
    admins: renderAdmins,
    mutes: renderMutes,
    blacklist: renderBlacklist,
    bots: renderBots,
    audit: renderAudit,
    qq: renderQQ,
    settings: renderSettings
  };

  /* ======================================================================
     启动 / 全局事件
     ====================================================================== */
  function captureTokenFromURL() {
    store.del(TOKEN_KEY);
    const url = new URL(location.href);
    const token = url.searchParams.get("token");
    if (!token) return;
    state.token = token.trim();
    store.set(TOKEN_KEY, state.token);
    url.searchParams.delete("token");
    history.replaceState(null, "", url.pathname + url.search + url.hash);
  }

  function bindChrome() {
    const app = document.getElementById("app");
    const menu = document.getElementById("btn-nav");

    const closeNav = () => {
      if (!app.classList.contains("nav-open")) return;
      app.classList.remove("nav-open");
      menu.setAttribute("aria-expanded", "false");
      document.documentElement.classList.remove("nav-lock");
    };

    menu.addEventListener("click", () => {
      const open = !app.classList.contains("nav-open");
      app.classList.toggle("nav-open", open);
      menu.setAttribute("aria-expanded", String(open));
      document.documentElement.classList.toggle("nav-lock", open);
      if (open) {
        const first = document.querySelector("#nav .nav-link");
        if (first) first.focus();
      }
    });
    document.getElementById("scrim").addEventListener("click", closeNav);
    document.getElementById("nav").addEventListener("click", event => {
      if (event.target.closest("a.nav-link")) closeNav();
    });

    document.getElementById("btn-refresh").addEventListener("click", () => loadCurrent());
    const botSelect = document.getElementById("bot-select");
    if (botSelect) botSelect.addEventListener("change", () => {
      state.botID = String(botSelect.value || "");
      if (state.botID) store.set("liant_bot", state.botID);
      else store.del("liant_bot");
      loadCurrent();
    });
    document.getElementById("btn-token").addEventListener("click", openToken);
    document.getElementById("btn-conn").addEventListener("click", openToken);

    window.addEventListener("hashchange", () => {
      state.tab = tabFromHash();
      loadCurrent();
      closeNav();
    });
    window.addEventListener("keydown", event => {
      if (event.key === "Escape") closeNav();
    });
    const wide = window.matchMedia("(min-width: 901px)");
    const onWide = event => { if (event.matches) closeNav(); };
    if (wide.addEventListener) wide.addEventListener("change", onWide);
    else if (wide.addListener) wide.addListener(onWide);

    document.querySelectorAll("[data-close-token]").forEach(button => {
      button.addEventListener("click", () => document.getElementById("token-modal").close());
    });
    document.getElementById("token-form").addEventListener("submit", event => {
      event.preventDefault();
      const value = document.getElementById("token-input").value.trim();
      if (!value) {
        toast.error("请输入 token");
        return;
      }
      const url = new URL(location.href);
      url.searchParams.set("token", value);
      location.replace(url.pathname + url.search + url.hash);
    });
    document.getElementById("token-clear").addEventListener("click", async () => {
      state.token = "";
      store.del(TOKEN_KEY);
      try {
        await fetch(BASE + "/api/logout", { method: "POST", credentials: "same-origin" });
      } catch {}
      location.reload();
    });

    document.getElementById("confirm").addEventListener("click", event => {
      if (event.target === event.currentTarget) event.currentTarget.close("cancel");
    });
    document.getElementById("token-modal").addEventListener("click", event => {
      if (event.target === event.currentTarget) event.currentTarget.close();
    });
    document.querySelectorAll("dialog").forEach(dialog => {
      dialog.addEventListener("close", () => syncScrollLock());
    });

    const mediaDialog = document.getElementById("media-dialog");
    const mediaDialogClose = document.getElementById("media-dialog-close");
    if (mediaDialogClose && mediaDialog) {
      mediaDialogClose.addEventListener("click", () => mediaDialog.close());
    }
    if (mediaDialog) {
      mediaDialog.addEventListener("click", event => {
        if (event.target === mediaDialog) mediaDialog.close();
      });
    }
    // 图片加载完成后，如果原本就在底部，重新贴底。
    view.addEventListener("load", event => {
      const img = event.target;
      if (!(img instanceof HTMLImageElement) || !img.hasAttribute("data-media-img")) return;
      const timeline = document.getElementById("chat-timeline");
      if (!timeline) return;
      if (timeline.scrollHeight - timeline.scrollTop - timeline.clientHeight < 180) {
        timeline.scrollTop = timeline.scrollHeight;
      }
    }, true);
    // 图片是延迟加载的：取回失败时不要在气泡里留一个破图标。
    view.addEventListener("error", event => {
      const img = event.target;
      if (!(img instanceof HTMLImageElement) || !img.hasAttribute("data-media-img")) return;
      const figure = img.closest(".msg-media");
      const button = img.closest("button");
      img.hidden = true;
      if (button) button.disabled = true;
      if (figure && !figure.querySelector(".media-fallback")) {
        const note = document.createElement("span");
        note.className = "media-fallback";
        note.textContent = "图片取回失败（可能已过期）";
        (button || figure).appendChild(note);
      }
    }, true);

    // 触摸设备没有 hover：长按气泡 420ms 打开"引用/撤回"浮层。
    let chatPressTimer = 0;
    view.addEventListener("pointerdown", event => {
      const msg = event.target.closest(".msg");
      if (!msg || event.target.closest("[data-act]")) return;
      const startX = event.clientX;
      const startY = event.clientY;
      clearTimeout(chatPressTimer);
      const cancel = () => clearTimeout(chatPressTimer);
      msg.addEventListener("pointerup", cancel, { once: true });
      msg.addEventListener("pointercancel", cancel, { once: true });
      msg.addEventListener("pointermove", e => {
        if (Math.hypot(e.clientX - startX, e.clientY - startY) > 8) cancel();
      }, { once: true });
      chatPressTimer = setTimeout(() => {
        view.querySelectorAll(".msg.is-actions-open").forEach(node => node.classList.remove("is-actions-open"));
        msg.classList.add("is-actions-open");
        if (navigator.vibrate) {
          try { navigator.vibrate(8); } catch {}
        }
      }, 420);
    });
    view.addEventListener("click", event => {
      if (!event.target.closest("[data-act]") && !event.target.closest(".msg")) {
        view.querySelectorAll(".msg.is-actions-open").forEach(node => node.classList.remove("is-actions-open"));
      }
    });

    view.addEventListener("click", event => {
      const target = event.target.closest("[data-act]");
      if (!target || target.disabled) return;
      const action = ACTIONS[target.dataset.act];
      if (!action) return;
      Promise.resolve().then(() => action(target, event)).catch(err => {
        toast.error(err && err.message ? err.message : String(err));
      });
    });

    // 表单弹窗在 #view 之外，成员选择器要在 document 上统一处理。
    document.addEventListener("change", event => {
      const memberSelect = event.target.closest("[data-member-select]");
      if (memberSelect) handleMemberSelect(memberSelect);
      const platformSelect = event.target.closest('#app-form select[name="platform"]');
      if (platformSelect) syncBotCredentialFields(platformSelect.form, platformSelect.value);
    });

    view.addEventListener("change", event => {
      const toggle = event.target.closest('[data-toggle="group"]');
      if (toggle) {
        toggleGroup(toggle).catch(err => toast.error(err && err.message ? err.message : String(err)));
        return;
      }
      const qqGroup = event.target.closest("[data-qq-group]");
      if (qqGroup) {
        loadQQMessages(qqGroup.value, qqGroup.form).catch(err => toast.error(err && err.message ? err.message : String(err)));
        return;
      }
      const joinsGroup = event.target.closest("[data-joins-group]");
      if (joinsGroup) {
        store.set(GROUP_KEY, joinsGroup.value);
        loadCurrent();
        return;
      }
      const chatSelect = event.target.closest("[data-chat-select]");
      if (chatSelect) {
        switchChatGroup(chatSelect.value);
        return;
      }
      const template = event.target.closest("[data-chat-template]");
      if (template) {
        if (template.value) applyCardTemplate(template.value);
        template.value = "";
        return;
      }
      if (event.target.closest("[data-filter-kind]")) {
        applyFilters(event.target.closest(".card"));
        return;
      }
      if (event.target.closest('form[data-form="settings"]')) markDirty();
    });

    view.addEventListener("input", event => {
      const target = event.target;
      if (target.matches("[data-filter]")) {
        applyFilters(target.closest(".card"));
        return;
      }
      if (target.matches("[data-chat-search]")) {
        filterChatGroups(target.value);
        return;
      }
      if (target.closest('form[data-form="chat-send"]')) {
        updateChatComposerState();
        return;
      }
      if (target.closest('form[data-form="settings"]')) markDirty();
    });

    view.addEventListener("submit", event => {
      const form = event.target.closest("form[data-form]");
      if (!form) return;
      event.preventDefault();
      const handler = FORMS[form.dataset.form];
      if (!handler) return;
      const submit = form.querySelector('button[type="submit"]');
      if (submit && submit.dataset.busy) return;
      if (submit) {
        submit.dataset.busy = "1";
        submit.setAttribute("aria-busy", "true");
      }
      Promise.resolve().then(() => handler(form))
        .catch(err => toast.error(err && err.message ? err.message : String(err)))
        .finally(() => {
          if (submit) {
            delete submit.dataset.busy;
            submit.removeAttribute("aria-busy");
          }
          if (form.dataset.form === "chat-send") updateChatComposerState();
        });
    });

    view.addEventListener("keydown", event => {
      const enter = event.target.closest('[data-act="enter-bot"][role="link"]');
      if (enter && (event.key === "Enter" || event.key === " ")) {
        event.preventDefault();
        enterBot(enter.dataset.id);
        return;
      }
      if (event.key === "Enter" && !event.shiftKey && !event.metaKey && !event.ctrlKey && !event.isComposing) {
        const area = event.target.closest('form[data-form="chat-send"] textarea[name="content"]');
        if (area) {
          event.preventDefault();
          const form = area.closest("form");
          if (form) form.requestSubmit();
          return;
        }
      }
      if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
        const form = event.target.closest('form[data-form="chat-send"]');
        if (form) {
          event.preventDefault();
          form.requestSubmit();
        }
      }
    });
  }

  async function init() {
    buildNav();
    captureTokenFromURL();
    state.tab = tabFromHash();
    if (location.hash !== "#" + state.tab) {
      history.replaceState(null, "", "#" + state.tab);
    }
    syncNav();
    bindChrome();
    try {
      await loadBots();
    } catch (err) {
      if (err && err.status !== 401) toast.error(err.message || String(err));
    }
    if (state.tab !== "bots" && !state.botID) {
      state.tab = "bots";
      if (location.hash !== "#bots") history.replaceState(null, "", "#bots");
      syncNav();
    }
    await loadCurrent();
  }

  init();
})();

# 当前状态与下一轮交接

> 这不是更新日志。每次任务的目标、过程、验证和遗留写在 [task-log.md](task-log.md)；
> 提交历史看 `git log`。本文件只回答：现在是什么样、哪些能用、哪里还有坑、下一步动哪里。

## 1. 项目定位

LianT 是**自托管的跨平台聊天软件**：

- **Manager**：主进程、管理端 Web、CLI、配置与 Bot 管理；
- **Service**：任务进程，负责平台接入、消息中继、规则、审计与临时状态；
- **Bot**：Telegram / QQ 两个平台的业务适配层。

设计原则见 [design.md](design.md)，接口契约见 [api.md](api.md)，平台差异见 [platforms.md](platforms.md)。

## 2. 当前可用能力

### 平台接入

- **Telegram**：群聊、私聊、论坛话题；文本/卡片/图片/视频/音频/文件发送；
  `getFile` 取回媒体；撤回；群管理、入群审批、禁言、黑名单等。
- **QQ**：群聊与 C2C 单聊；文本、群聊 Markdown 卡片；群管理、入群审批、禁言、黑名单；
  媒体取回走服务端代理。
- **QQ 频道（guild）**：尚未接入。会话模型保留了 `kind=channel`，但 QQ adapter
  目前只订阅群聊/C2C 事件，没有频道网关、频道列表或发送接口。

### 聊天页

- 全屏会话布局：会话列表 + 消息流 + 输入条；移动端会话列表是抽屉。
- 会话按上下文拆开：
  - 普通群：`chat_id`
  - 私聊：每个用户一个 `chat_id`，一人一个会话
  - Telegram 论坛话题：`chat_id::thread_id`
  - 未来的频道：`channel_id`（模型已留位）
- 会话列表显示群名/人名/话题名、最近一条消息和相对时间；同名私聊会带 ID 后缀。
- 消息气泡：日期胶囊、同一发送者短时间合并、入站/出站区分；
  文字、卡片、图片、文件、视频、音频、语音、贴纸都有对应展示。
- 每条消息的「⋯」按钮，或直接点头像，打开统一操作菜单：
  查看图片 / 引用 / 复制文本 / 下载附件 / 撤回。**没有长按菜单。**
- 输入条：默认文本，Enter 发送、Shift+Enter 换行、自动增高；可切换卡片并填标题。
- 附件按钮：图片/视频/音频/文件，Telegram 走 multipart 上传：
  `sendPhoto` / `sendVideo` / `sendAudio` / `sendDocument`，发出的消息写回会话历史。
- 附件下载：按钮触发带鉴权的 `fetch` → Blob → 下载；失败显示平台错误，不跳空白页。
- `⋯` 菜单和附件按钮均为显式按钮；卡片宽度按内容收缩，最大 320px。

### 数据与状态

- 每个 Bot 独立 SQLite：`LianT-data/bots/<botID>.db`（实际根目录由 `LIANT_DATA_DIR` 等决定）。
- schema **v5**：
  - `messages`：`chat_id + message_id` 唯一，`seq` 排序；新增 `conv_key`、`thread_id`；
    默认不设条数上限；撤回写 `state=recalled` 并清空正文/媒体引用。
  - `conversations`：`key / chat_id / thread_id / kind / title / title_final / note / updated_at`。
  - 旧 v4 消息回填为 `conv_key=chat_id`；v4 没有 `thread_id`，无法回溯拆话题。
- 媒体引用只存在本机库，前端只拿 `/api/media?bot_id=&chat_id=&msg=` 的代理地址。
- 媒体状态：`available` / `expired` / `gone` / `unknown`；取回失败会写回状态。
- SSE 不推 runtime 事件，而是按会话签名（最大 seq + 最新更新时间）变化后让浏览器重拉。

## 3. 关键约定

- **会话键**：`chat_id`（群/私聊）或 `chat_id::thread_id`（话题）。
  `/api/chat/groups` 返回的 `openid` 是会话键；`/api/chat/recent`、`/api/chat/send`、
  `/api/chat/upload` 都用它。
- 发消息时平台的真实 `chat_id` 与 `thread_id` 由后端从 `conversations` 解析，
  前端不要自己拼。
- 私聊不是“一个总私聊”：每个用户独立一条会话；不要把私聊聚成一个桶。
- Telegram 论坛话题判断以 `message_thread_id > 0` 为准，不依赖单一布尔字段；
  顶层缺失时从 `reply_to_message.message_thread_id` 兜底。
- 上传单文件上限 32MB；QQ 富媒体上传未接入，接口返回 `409`，前端隐藏附件按钮。
- 网页上传的 Telegram 媒体会写回 `messages.media`；
  旧的 `SendMediaNow` 等路径仍只记文本，没有媒体引用。

## 4. 验证状态（重要）

⚠️ **所有 Go 改动都没有项目内 `go build` 验证**。本机 Go 工具链损坏：

- Android 拒绝执行实验性 PIE 版 `go`；
- Termux 的 Go 1.27 经 `libtermux-exec` + `linker64` 启动时会把程序路径塞进 `argv[1]`，
  `go`/`gofmt` 会把自身路径当子命令。

本轮实际做过的替代验证：

- tree-sitter-go 全项目 41 个文件，0 语法错误；
- `node --check` 管理端 JS；CSS 括号平衡；
- Go Playground 单文件类型检查：`store/messages`、bot/qqbot 的新增函数、媒体函数通过；
  没有覆盖完整的 `internal/admin`（`qbot_compat.go` 很大，只能逐段复核）；
- sql.js 实际跑过 v4→v5 迁移、会话回填、按 `conv_key` 查询、标题 `title_final` 语义；
- 应用内 AI 浏览器 + mock API 实测：群/话题/私聊切换、两人私聊分行、操作菜单、
  卡片尺寸、文件下载按钮、附件按钮；隐藏 WebView 截图有合成残影时以 DOM 尺寸为准。

**这些不能替代真机构建。** 改完后必须重新编译/重启 LianT；管理页资源是嵌入二进制的，
只刷新浏览器不会加载新 Go 代码。

## 5. 已知限制与坑

- QQ 频道 runtime 未接入（模型留了 `kind=channel`，但事件/发送都没接）。
- QQ 私聊撤回、QQ 富媒体上传需要平台接口/权限，目前分别返回 `409` / 隐藏按钮。
- v5 之前的消息没有 `thread_id`，旧话题消息无法自动拆开，只能从新消息开始分。
- 命令引擎的历史窗口仍是群级（`RecentList` 不按话题拆）；管理端聊天页是会话级。
- 一条消息只投影第一个媒体；相册/多图只显示第一张。
- `/api/media` 不支持 Range，视频拖动进度受限；下载走完整 Blob。
- 历史仍是一次性全量返回，暂无分页；库会持续增长。
- SSE 仍是数据库签名轮询式重拉，不是真正的 runtime 事件推送。
- `qbot_compat.go` 与 `admin.go` 原生路由存在重复，compat 会抢占若干路由，后续需要二选一。
- `qctx` 的 `b`/`q` 双指针还没抽成平台无关接口，这是接 Discord/QQ 频道的前置。
- 全项目没有测试；`internal/rules`、`internal/chat` 最适合先补。
- 本机 `go` 不可用，验证方式和证据要按上面第 4 节写清楚。

## 6. 下一轮优先级

1. **QQ 频道 adapter**：若确实要支持，先补 guild 事件的 intent/路由、频道列表与发送 API；
   否则从文档和 UI 里明确标注“未支持”，不要只留一个 `channel` 类型。
2. **合并管理端路由**：`admin/qbot_compat.go` 与 `admin.go` 选一套，避免字段/行为继续分叉。
3. **真事件推送**：在 `remember` / `MarkRecalled` / 媒体状态变化处加 notifier，
   替换 SSE 的数据库轮询式重拉。
4. **补测试**：优先 `internal/rules`、`internal/chat`、store 的迁移/upsert/会话键。
5. 清理剩余体验问题：媒体 Range、分页、旧 `SendMediaNow` 的媒体引用、QQ 私聊撤回。

## 7. 代码地图

| 路径 | 内容 |
| --- | --- |
| `internal/bot` | Telegram 运行时：收发、命令、规则、持久化、媒体引用 |
| `internal/qqbot` | QQ 运行时：群/C2C、命令、规则、持久化 |
| `internal/platform/qq` | QQ 开放平台 API 封装与 URL |
| `internal/telegram` | Telegram Bot API 客户端与类型 |
| `internal/store` | SQLite schema、迁移、消息/会话/群/规则/审计 |
| `internal/media` | 媒体引用、状态、SSRF 防护、HTTP 取回 |
| `internal/admin` | 管理端 HTTP、鉴权、媒体代理、qbot 兼容路由 |
| `internal/admin/web` | 管理端 HTML/CSS/JS（聊天页、表单、导航） |
| `internal/chat` | 会话投影模型与 CLI `chat` |
| `docs/task-log.md` | 每次任务的过程记录，按时间倒序 |

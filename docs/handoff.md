# 现状与下一轮交接

## 项目定位

LianT 是**自托管的跨平台聊天软件**：Service（任务进程）负责平台接入、消息中继与临时存储；
Manager（主进程/管理端/CLI）负责管理与交互；机器人、规则、审计是辅助能力。详见 [design.md](design.md)。

## 已完成（最近提交）

| 提交 | 内容 |
| --- | --- |
| `9cb8d9a` | 聊天记录持久化：`messages` 表（schema v4），取消 200 条上限，历史全量读取 |
| `65629b7` | 聊天媒体：引用采集、`/api/media` 取回、媒体状态与前端渲染 |
| `c71a23e` | 消息状态语义：`received` / `sent` / `recalled`；撤回变墓碑（正文清空）；UI 显示状态 |
| `4ef9e15` | `internal/chat` 投影模型；IPC `Payload`；任务进程 `chat` 指令 |
| `acfde02` | 会话优先（默认 #chat、导航重排）；`LianT chat` CLI |
| `b088326` | `/api/events` SSE 推送消息变化（服务端快照比对，非 runtime 事件） |
| `ccc300a` | 管理端配色统一（冷灰白 + 靛蓝，单一变量集） |
| `7c47d22` | Bot 列表显示真实连接状态（`state` / `online`），不再一律「运行中」 |

更早：死代码清理、admin 页面拆 html/css/js、AGPL-3.0、文档扩充、平台校验走注册表、路径容错。

## 验证状态

⚠️ **所有 Go 改动都没有编译验证**。本机 Go 工具链损坏（`~/goroot/bin/go` 是实验性 PIE 版，Android 拒绝执行）；
Termux 的 Go 1.27 走 `libtermux-exec` + `linker64` 启动时会把程序路径塞进 argv[1]，
`go`/`gofmt` 因此把自身路径当成子命令，无法运行；QEMU 方式可跑但会让手机满载发热，已停用。
本轮改动做了：tree-sitter-go 全项目语法解析（41 个文件 0 错误）、JS `node --check`、逐行复核；
另外把 `internal/media`、`internal/telegram` 以及 bot/qqbot 的媒体函数、`admin/media.go`
拼成单文件交给 Go Playground 编译（类型检查通过）；`messages` 建表与 upsert 用 sql.js 实际跑过（含 v3→v4 迁移、撤回墓碑、重复投递）。
这些仍不是项目内 `go build`，不能替代真机构建。

## 聊天媒体（已完成）

1. **采集**：`RecentMsg.Media *media.Ref`（含平台引用；随消息落到本机库，不进 API JSON）。
   - Telegram：取最大的 `Photo`、`Document`/`Video`/`Animation`/`Audio`/`Voice`/`VideoNote`、
     `Sticker`（只取 `emoji`）；`Kind` 统一成跨平台值（`image`/`file`/`sticker`/…）。
   - QQ：`attachments[]` 的 `url`/`filename`/`size`/`content_type`/`width`/`height`，
     一条消息只投影第一个带 URL 的附件。
2. **取回**：`GET /api/media?bot_id=&chat_id=&msg=`（`internal/admin/media.go`）。
   - 只按「会话 + 消息」查本地消息库，不能凭任意 `file_id`/URL 取文件；
   - Telegram：每次现调 `getFile`，再用带 token 的 `/file/bot…` 流式转发，token 不落前端；
   - QQ：服务端代理签名直链，限制协议与内网地址（尽力而为的 SSRF 防护）；
   - 32 MB 上限、撤回消息不提供媒体、`file` 类型走 attachment，其余 inline。
3. **状态**：`available` / `expired` / `gone` / `unknown`（`internal/media`）。
   取回失败会写回该消息的 `media.state`，SSE 签名能看到变化，界面显示「不可取回」而不是伪造预览。
4. **前端**：图片懒加载 + `<dialog>` 放大；文件显示名称/大小/类型与下载；表情按 `emoji` 渲染；
   视频/音频用原生控件；图片 404 时替换为明确的失败提示。

## 聊天记录持久化（已完成）

- 每个 Bot 的业务库新增 `messages` 表（`internal/store/messages.go`，schema v4）：
  `chat_id + message_id` 唯一，按 `seq` 排序；正文、类型、发送者、状态、媒体引用、时间都在里面。
- 消息默认**不设条数上限**：`RecentList` / `/api/chat/recent` / `/api/messages` 读库，
  `limit<=0` 返回全部；CLI 与 daemon 默认也是全部，`-n` 可限制。
- 撤回写 `state=recalled` 并清空 `content`/`media`；媒体状态变化只更新 `media` 列。
- SSE 不再读取 80 条消息做快照，改用 `MessageSignature`（最大 seq + 最后更新时间，都走索引）。
- 内存里的 `recent` map 已删除；历史完全以 SQLite 为准，重启不丢。

已知取舍：

- 机器人自己发出的媒体（`RememberOutgoing`）还没带媒体引用，只显示卡片/文本；
- 一条消息只投影一个媒体，相册/多图消息只显示第一张；
- `/api/media` 不支持 Range，视频拖动进度可能受限；
- QQ 签名直链的 `unknown` 在取回成功后仍保持 `unknown`（成功只证明当时可用，不下“长期有效”的结论）；
- 历史一次性全量返回，暂无分页；库会随聊天持续增长，需要使用者自己备份/归档。

## 聊天页面重做（已完成）

- **全屏聊天应用**：`#chat` 不再套 `page-head` / `card`，`.app.chat-mode` 下由
  `.chat-app`（会话列表 + 对话区）直接占满应用区，只靠底色和 1px 分隔线分区。
- **会话列表**：平面行 + 头像 + 最近一条消息预览 + 相对时间；预览来自 `messages` 表
  （新增 `store.LastMessage`），不再是群表的 `updated_at`。
- **消息流**：日期胶囊、同人 5 分钟内合并、入站左侧气泡 / 出站右侧渐变气泡、
  引用/撤回图标悬停浮现；撤回改为居中的系统消息，不再留一个虚线气泡。
- **媒体/卡片**：图片圆角懒加载 + dialog 放大；文件、语音、视频、贴纸都在气泡里；
  卡片消息仍是内容卡片，不再把整个页面塞进卡片。
- **输入区**：一条圆角输入条；默认文本模式，Enter 发送、Shift+Enter 换行、自动增高；
  `卡片` 切换后才出现标题输入，模板/字数在工具条里。
- **移动端**：会话列表变成左侧抽屉（hamburger 打开、遮罩/选群关闭）；
  消息操作改为长按 420ms 浮出，避免每条消息下面常驻图标。
- **验证**：用 mock API 在应用内 AI 浏览器按 1920×1080 和 390×844 实际打开截图，
  检查了消息气泡、媒体、撤回、卡片模式、发送后状态；隐藏 WebView 的截图偶有合成残影，
  以 DOM 尺寸复核为准。

## 其他待办

- SSE 仍是"数据库签名变化后让浏览器重拉"；下一步给 `bot`/`qqbot` 的 `remember`、`MarkRecalled`
  加 notifier，走真事件；
- `admin/qbot_compat.go` 与 `admin.go` 原生路由重复（compat 会抢占多个路由），需要二选一；
- 全项目无测试；`internal/rules`、`internal/chat` 最适合先补；
- `qctx` 的 `b`/`q` 双指针待抽成平台无关接口（Discord 接入前置）。

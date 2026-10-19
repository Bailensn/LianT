# LianT

> **这个项目是 AI 写的。** 需求、方向与验收由开发者负责；代码、文档和提交信息由 AI Agent 生成。
> 请按「AI 辅助产出」对待，上生产前自己审一遍，尤其是鉴权、SQL 和平台接口部分。

**自托管的多平台群管理后台与机器人**（当前接入 Telegram、QQ，其他平台按 adapter 扩展）。

**许可证：AGPL-3.0**（[LICENSE](LICENSE)）——改过的版本对外提供网络服务时，必须公开源码，详见[许可证](#许可证)一节。

单文件 Go 程序：平台接入走各自的官方 API / 网关，不需要公网 IP、域名或 HTTPS 回调；
Bot 凭据存在 **SQLite 注册表**里，可以同时运行多个 Bot，每个 Bot 有独立业务库。
网页后台用同一套 UI 管理所有 Bot —— 平台差异收在 `platform.Adapter` 里，当前已接入 `telegram` 和 `qq`。

设计取向上参考了 [LensnTeam/LianT](https://github.com/LensnTeam/LianT)：Bot 管理、自托管、SQLite、网页后台。

---

## 文档

| 文档 | 内容 |
| --- | --- |
| 本页 | 功能、部署、配置、CLI、数据库 |
| [docs/api.md](docs/api.md) | 管理端 HTTP API：鉴权、返回格式、路由表、示例 |
| [docs/platforms.md](docs/platforms.md) | 平台兼容层：Adapter、能力声明、qbot 合并、接入 Discord |
| [docs/reverse-proxy.md](docs/reverse-proxy.md) | Nginx / Caddy 反代、前缀、Webhook、静态资源、排错 |

## 功能

- **回复规则**：exact / prefix / contains / regex / any，优先级、限定群/私聊、模板变量。
- **违禁词**：命中后撤回 / 禁言 / 踢出 / 封禁 / 拉黑 / 警告回复，全部写审计。
- **群内指令**：中文「禁言 @某人 10分钟」「把张三踢了」和 `/mute`、`/ban`、`/del` 都支持。
- **成员处置**：禁言、解禁、踢出（可重新加入）、封禁、解封、撤回单条/批量。
- **黑名单**：本地拦截 + 平台封禁（平台支持时），解除时自动解封。
- **入群**：欢迎语（`{name}` / `{username}` / `{chat}`）、入群申请自动通过或留在平台侧人工审批。
- **网页后台**：总览、群组、消息、群发、入群审批、规则、违禁词、管理员、禁言、黑名单、
  群工具、Bot 资料、Bot API/文件上传、审计、设置；支持搜索、筛选、明暗主题。
- **多 Bot**：一个任务进程跑 N 个 Bot，接入方式由平台适配器决定（长轮询 / Webhook / WebSocket）；注册表在 `LianT.db`，每个 Bot 的业务库在 `bots/<id>.db`。
- **平台兼容层**：`platform.Adapter` 统一身份校验与能力声明；所有平台共用同一套后台与业务库。
- **平台原生能力**：QQ 适配器提供群消息、Markdown 卡片、入群审批、禁言、群信息、成员列表、黑名单（视 QQ 白名单权限）；能力缺口由 `Capabilities` 在 UI 里隐藏。
- **单文件 + SQLite**：纯 Go 驱动（`modernc.org/sqlite`），不需要 cgo/gcc，方便手机和 arm 机器。
- **收消息**：各适配器自带接入方式——Telegram 默认长轮询（无需公网/证书），也可 webhook + 密钥；QQ 走官方 WebSocket 网关。
- **媒体与交互**：图片/文件/视频/音频/语音/贴纸/动图/位置/投票/骰子、相册、按钮回调、话题、置顶、编辑消息。
- **内联查询**：`@机器人 关键词` 返回配置好的结果（`inline_results`）。
- **Bot API 全覆盖**：管理端「Bot API」页可直接调用任意 Bot API 方法，并支持上传本地文件
  （邀请链接、论坛话题、权限、贴纸、支付、Business 等）；支付预结算/配送查询可按设置自动确认。

## 架构

```
┌──────────────┐   指令(回环 TCP)   ┌──────────────────────────┐
│ 主进程 LianT  │ ────────────────▶ │ 任务进程 LianT __run      │
│ 只发指令      │ ◀──────────────── │ 多 Bot 管理器 + 网页后台  │
└──────────────┘                   └──────────┬───────────────┘
                                              │
                    ┌─────────────────────────┼─────────────────────────┐
                    ▼                         ▼                         ▼
             LianT.db（注册表）        bots/1.db（Bot #1）        bots/2.db（Bot #2）
             bots / admin_token        规则/群/禁言/审计…        规则/群/禁言/审计…
                    │
                    ▼
              admin.html（浏览器/手机）
                    │
                    ▼
        平台 API / 网关（每个 Bot 独立凭据；长轮询 / webhook / WebSocket 由适配器决定）
```

- **主进程**：`LianT`，只发指令（start/stop/status/logs/admin/bot…），随时开关不影响机器人。
- **任务进程**：`LianT __run`，全局唯一；一个进程内跑 N 个 Bot。
- **注册表**：`LianT.db`，存 Bot token、平台、启停、webhook 配置、管理端 token。
- **业务库**：`bots/<id>.db`，每个 Bot 独立，互不污染。
- **日志**：`LianT.log`，8MB 轮转，保留 `.1`~`.5`。

## 快速开始

### 1. 创建机器人

1. Telegram 找 **@BotFather**，`/newbot` 拿到 token（形如 `123456789:AA...`）。
2. 把机器人拉进群，设为**管理员**，至少勾选「删除消息」「封禁用户」。
3. 想让机器人看到群里全部消息（而不只是 @ 它的）：@BotFather → `/setprivacy` → 选机器人 → **Disable**。

### QQ Bot（可选）

1. 在 QQ 开放平台创建机器人，拿 **AppID** 和 **AppSecret**。
2. 添加时选平台 `qq`，分别填 AppID 和 AppSecret：

```sh
./LianT bot add --platform qq
# QQ AppID: ...
# QQ AppSecret: ...
```

网页后台「机器人 → 添加 Bot」选 QQ，会自适应切换成 **AppID** + **AppSecret** 两个字段；Telegram 仍只填 Bot Token。显示名称留空即可，Telegram 会用 `getMe` 的 Bot 名称，QQ 用 AppID 作为默认名称。

QQ 网关是 WebSocket，不需要 webhook；入群审批、禁言、群信息/成员/黑名单接口是否可用取决于 QQ 平台给机器人开的权限（常见错误码 11253）。

### 2. 配置

```sh
cp config/.env.example .env
# .env 只放路径/端口/代理/管理端 token，不再放 Bot token。
# Bot token 存在 SQLite：用 `./LianT bot add`（按提示输入 token）或网页「机器人」添加。
```

### 3. 编译

```sh
go mod tidy
go build -trimpath -ldflags "-s -w" -o LianT .          # 根入口
# 或：go build -trimpath -ldflags "-s -w" -o LianT ./cmd/liant
```

> 只需要 Go 1.27+（依赖 modernc.org/sqlite 要求 1.26+），不需要 gcc/cgo。Windows 编译出来是 `LianT.exe`。

### 4. 启动

```sh
./LianT init                 # 生成 .env 模板 + 初始化注册表（可选）
./LianT bot add             # 交互输入 token，添加第一个 Bot（也可网页「机器人」添加）
./LianT start                # 后台启动任务进程（会启动所有 enabled 的 Bot）
./LianT status               # 看 Bot 数量与运行状态
./LianT admin                # 打印网页后台地址与 token
```

打开 `http://127.0.0.1:28742/?token=...`，之后 token 会写进 cookie。

### 5. 停止 / 重启

```sh
./LianT stop
./LianT restart
./LianT logs 100 -f
```

## 群内指令

**触发规则**：整条消息本身就是指令才生效，普通聊天里出现「禁言」不会被误判。
管理指令仅 **后台管理员** 或 **群管理员**（平台提供管理员信息时）可用，可在网页里按群关掉后者。

| 场景 | 用法 |
| --- | --- |
| 禁言 | 回复某人 → `/mute 10m`；或 `禁言 @某人 10分钟`、`把张三禁言 1小时` |
| 解禁 | 回复某人 → `/unmute`；或 `解禁 @某人` |
| 踢出 | 回复某人 → `/kick`；或 `踢 @某人`（允许重新加入） |
| 封禁 | 回复某人 → `/ban`；或 `踢黑 @某人` |
| 解封 | `/unban 用户ID`；或 `解黑 @某人` |
| 撤回 | 回复消息 → `/del`；或 `撤回`、`撤回 3条`、`撤回 @某人` |
| 违禁词 | `加违禁词 词 [时长]`、`删违禁词 词` |
| 管理员 | `加管理 @某人`、`删管理 @某人` |
| 其他 | `/status`、`/help`、`/id`、`状态`、`帮助` |

时长单位：`秒/分钟/小时/天/周`，也支持 `10m`、`1h`、`1d`；不写默认 10 分钟，上限 30 天。

### 目标怎么定位

按优先级：**回复某人的消息** → **@提及** → **数字用户 ID** → **本地名册昵称** → **我/自己**。

Telegram 普通 @username 不带用户 ID，所以 LianT 会用本地名册（成员发言时自动登记）反查；
找不到时会提示改用「回复消息」的方式。

## 网页后台

`./LianT admin` 打印地址与 token，默认 `127.0.0.1:28742`。

- **总览**：机器人状态、运行时长、消息数、规则/违禁词/禁言/待审批统计、最近审计。
- **群组**：自动登记 + 手动添加；启用/停用、回复所有消息、指令开关、群管理员权限、备注；
  「从 Telegram 同步」会拉群名并检查机器人是否管理员。
- **消息**：最近 60 条消息与撤回/置顶/转发/复制/编辑；主动发送文本/媒体/投票/位置/按钮。
- **回复规则 / 违禁词**：网页增删改，保存后下一条消息生效。
- **管理员**：全局或限定某个群。
- **禁言 / 黑名单**：手动处置、解除。
- **群发**：一次勾选最多 50 个会话，逐条发送并展示每个会话的成功/失败原因。
- **入群审批**：申请会落库；通过/拒绝直接调用 Telegram，并写审计。
- **群工具**：群信息、管理员、成员数、邀请链接、话题、群名/简介、全员禁言/恢复、置顶、退群。
- **Bot**：getMe、webhook 状态、命令菜单、描述/短描述。
- **消息操作**：撤回、置顶、转发、复制、编辑（文本）。
- **操作日志**：谁、在哪个群、对谁、命中什么、结果如何；可按类型/会话筛选。
- **设置**：回复策略、默认回复、入群自动审批、欢迎语、内联结果、支付自动确认；导出 JSON 配置。

鉴权：`LIANT_ADMIN_TOKEN`（环境变量 / `.env` / 数据库自动生成）。
支持 `Authorization: Bearer <token>`、`?token=<token>` 或 cookie。

绑定非回环地址又没配 token 时，管理端**拒绝启动**。

反代示例见 [docs/reverse-proxy.md](docs/reverse-proxy.md)。

## 媒体、按钮与内联

网页后台「消息」页可以直接发送：

- 文本、图片、文件、视频、音频、语音、贴纸、动图、位置、投票、骰子；
- 媒体用 `file_id` 或公开 URL（`sendPhoto`/`sendDocument` 等支持的形式）；
- 可选 `message_thread_id`（论坛话题）；
- 可选按钮 JSON：二维数组会自动包成 `inline_keyboard`，也可以传完整 `reply_markup`。

按钮回调支持 `del:<message_id>`、`mute:<user_id>:<seconds>`、`unmute:<user_id>`、`ban:<user_id>`、`unban:<user_id>`，执行前会校验操作者是不是管理员。

内联模式：在「设置」里填 `inline_results`（每行 `标题|内容`），并到 @BotFather `/setinline` 开启内联；
之后在任意聊天输入 `@你的机器人 关键词` 即可返回结果。

### 卡片消息

QQ Bot 直接发原生 Markdown 卡片。Telegram 没有等价消息类型，LianT 会把它转成 **HTML 富文本**发送：
标题加粗、正文保留换行、`**加粗**`、`- 列表`、行内代码和 http(s) 链接会被转换；
后台「最近消息」仍按卡片气泡显示（以 `## 标题` 存储），所以发送完刷新不会变成纯文本。

## Webhook（可选；当前由 Telegram 适配器提供）

QQ 使用 WebSocket 网关，不需要 webhook。



Webhook 按 Bot 配置，在网页后台「机器人 → 编辑」里填写：

- Webhook URL：公网 HTTPS 地址
- Webhook secret：随机字符串
- Webhook 本地监听：如 `127.0.0.1:28743`
- Webhook 路径：如 `/telegram/webhook`

多个 webhook Bot 要用不同的本地监听端口；反向代理按端口/路径转发。
程序启动或保存时会自动 `setWebhook`，并校验
`X-Telegram-Bot-Api-Secret-Token`；没有密钥的请求直接 403。
想切回长轮询：把该 Bot 的模式改成「长轮询」并保存。

## Bot API 控制台

网页后台「Bot API」页可以调用 Telegram 支持的任何方法，例如：

`getChat`、`getChatAdministrators`、`createChatInviteLink`、`createForumTopic`、
`setChatPermissions`、`pinChatMessage`、`sendMediaGroup`、`getWebhookInfo`、`getMyCommands` 等。

参数是 JSON 对象，结果原样展示。需要本地文件的方法（`sendPhoto`、`sendDocument`、`createNewStickerSet` 等）
用「上传本地文件并调用」：填方法名、文件字段名（`photo`/`document`/`png_sticker`…）和参数，选文件即可。
这样 Telegram 后续新增的方法不需要等 LianT 更新也能直接用；常用操作（发送/禁言/撤回/群组）仍有专门的 UI。

## 界面设计

后台 HTML 复用 qbot 的组件结构，但信息架构改成 **Bot 优先**：

1. 首屏是 Bot 列表（卡片、运行状态、平台、能力、编辑/重启/删除）；
2. 点进某个 Bot 才进入它的工作台，侧栏显示该 Bot 的消息、规则、违禁词、群、成员、审计、手动操作和设置；
3. 「所有 Bot」可随时返回首屏。

视觉上只有一套变量：冷灰白表面 + 单一靛蓝主色（浅色/深色各一组值），成功/警告/危险各一个语义色；
组件一律取 `var(--token)`，不再写死色值，也不再有第二套主题互相覆盖。以后再接平台不需要再长一套 UI。

交互细节遵循 [emilkowalski/skills](https://github.com/emilkowalski/skills) 的原则，重点不是加动效，而是减少错误感：

- 自定义 `ease-out` 曲线；按钮按下 `scale(0.97)`，反馈在 100–160ms 内完成。
- 只对 `transform` / `opacity` 做动画；弹层从 `scale(0.96)` + 透明开始，而不是 `scale(0)`。
- 悬停样式全部包在 `@media (hover: hover) and (pointer: fine)` 里，触屏不会粘住 hover。
- 手机端：`100dvh`、`viewport-fit=cover`、安全区、`-webkit-tap-highlight-color: transparent`、
  `touch-action: manipulation`、输入框 16px 防缩放、内层滚动 `overscroll-behavior: contain`。
- `prefers-reduced-motion` 下移除位移动画，保留透明度变化；支持 `prefers-reduced-transparency`。
- Toast 用 transition 而不是 keyframes，可被连续消息打断；关闭/ESC/遮罩都能关弹层。
- 长群名、长 ID、空列表、超长消息都有专门的截断/换行/空状态处理。

## 多 Bot 与平台兼容层

### 多 Bot

- 注册表 `LianT.db` 的 `bots` 表存 token、平台、启停、长轮询/webhook；不再放 `.env`。
- 每个 Bot 独立业务库 `bots/<id>.db`，规则/群/禁言/审计互不污染。
- 一个任务进程同时跑所有 enabled 的 Bot。
- 网页右上角切换 Bot；「机器人」页添加/编辑/启停/重启/删除。
- CLI：`LianT bot list|add|remove|enable|disable`。

旧版的 `LIANT_BOT_TOKEN` 会在注册表为空时自动导入一次，并把旧单 Bot 数据复制到新 Bot 库。

### 平台兼容层

后台和数据库只依赖中性的 `chat_id` / `user_id` / `capabilities`，平台实现放在 `internal/platform/`：

```go
type Adapter interface {
    ID() string
    Name() string
    Capabilities() Capabilities
    ValidateToken(token string) (Identity, error)
}
```

- 当前实现：`internal/platform/telegram` 和 `internal/platform/qq`；QQ 运行时在 `internal/qqbot`。
- 管理端所有页面通过 `/api/bots` 拿到 `platform` 与 `capabilities`，据此显示/隐藏话题、邀请链接、内联、支付等入口。
- 接入新平台时：实现一个 `platform.Adapter`，给 Bot 注册表加 `platform`，再用同一套 `admin.html` 与 `/api/*` 契约。
- 字段映射与 QQ 合并细节见 [docs/platforms.md](docs/platforms.md)。

## 配置

`.env` 只放启动期配置；业务数据都在数据库里。优先级：**环境变量 > `.env` > 代码默认值**。

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `LIANT_API_BASE` | `https://api.telegram.org` | 自建网关/反代时改 |
| `LIANT_PROXY` | 空 | `http://127.0.0.1:7890` 或 `socks5://...` |
| `LIANT_PORT` | `28741` | 控制端口（多实例时各用一个） |
| `LIANT_BIND` | `127.0.0.1` | 控制端口绑定地址 |
| `LIANT_DATA_DIR` | 当前目录/`LianT-data` | 数据目录 |
| `LIANT_DB_FILE` | `<数据目录>/LianT.db` | Bot 注册表 SQLite |
| `LIANT_LOG_FILE` / `LIANT_PID_FILE` / `LIANT_RUNTIME_FILE` | 数据目录下 | 单独指定 |
| `LIANT_ADMIN_ADDR` | `127.0.0.1:28742` | 管理端监听地址 |
| `LIANT_ADMIN_TOKEN` | 自动生成 | 管理端 token |
| `LIANT_ADMIN_PREFIX` | 空 | 反代挂载前缀，如 `/LianT` |
| `LIANT_ADMIN_HTML` | 内置 | 自定义管理页 |
| `LIANT_ADMIN_ENABLE` | `true` | `false` 关闭网页后台 |

**Bot token、webhook、启停都在 SQLite**：网页「机器人」或 `./LianT bot ...`。
新增/修改 Bot 后，运行中的任务进程用 `./LianT reload` 同步（网页保存会自动重启该 Bot）。
路径/端口/代理改完需要 `./LianT restart`。

### CLI

| 指令 | 作用 |
| --- | --- |
| `LianT` | 交互控制台 |
| `LianT start` / `stop` / `restart` | 启动 / 停止 / 重启任务进程 |
| `LianT status` | Bot 数量、运行状态、管理端地址 |
| `LianT logs [行数] [-f]` | 看日志 |
| `LianT admin` | 网页后台地址 + token |
| `LianT bot list` | 列出 SQLite 里的 Bot |
| `LianT bot add [名称]` | 交互输入 token；加 `--platform qq` 分别输入 AppID / AppSecret |
| `LianT bot platforms` | 查看已接入平台与能力 |
| `LianT bot remove <id>` | 删除 Bot 注册（业务库保留） |
| `LianT bot enable/disable <id>` | 启用/停用 Bot |
| `LianT db` | 注册表统计 |
| `LianT reply [文本]` | 干跑回复规则（不发消息） |
| `LianT reload` | 同步 Bot 注册表、启动新增 Bot、请求重连 |
| `LianT reconnect` | 尽快重连当前 Bot（各平台按适配器处理） |
| `LianT path` | 打印所有路径 |
| `LianT run` | 前台运行（调试，日志打屏） |

## 敏感数据与加密

- `bots.token`、`bots.webhook_secret`、`settings.admin_token` 使用 **AES-256-GCM** 加密后写入 SQLite。
- 默认密钥文件：`<数据目录>/secret.key`，首次启动生成，权限 `0600`；也可以设置 `LIANT_SECRET` 固定密钥。
- **备份或迁移时必须同时带走 `secret.key`**（或记住 `LIANT_SECRET`），只拷数据库无法解密 token。
- 数据库文件本身会 `chmod 0600`；密钥文件同样 `0600`。
- CLI **不接受** `--token` / `--app-secret` 这类密钥参数；直接 `LianT bot add` 隐藏输入，脚本用 `--token-stdin` / `--app-secret-stdin` 从管道读，避免 shell history。
- Web 后台 token 输入框是 password 类型；登录后只写 **HttpOnly + SameSite=Lax** cookie，不再把管理 token 存进 localStorage。
- 远程访问请用 HTTPS：可以设 `LIANT_ADMIN_TLS_CERT` / `LIANT_ADMIN_TLS_KEY` 让管理端自己上 TLS，也可以用 Caddy/Nginx 反代。

## 数据库

### 注册表 `<数据目录>/LianT.db`

| 表 | 内容 |
| --- | --- |
| `bots` | Bot 注册：name、token、platform、platform_id、mode、webhook、enabled |
| `settings` | 管理端 token、迁移标记等全局配置 |

### 每个 Bot 的业务库 `<数据目录>/bots/<id>.db`

| 表 | 内容 |
| --- | --- |
| `settings` | reply_all、reply_default、join_auto_approve、welcome_text、inline_results 等 |
| `reply_rules` | 回复规则 |
| `banned_words` | 违禁词与处置 |
| `groups` | 会话开关（chat_id） |
| `admins` | 后台管理员（全局/限定群） |
| `mutes` | 禁言记录 |
| `blacklist` | 本地黑名单 |
| `members` | 本地名册（来自成员发言） |
| `join_requests` | 入群申请 |
| `audit` | 操作日志 |

备份：拷整个 `LianT-data/`（注册表 + 所有 Bot 业务库），或单个 Bot 用网页「设置 → 导出 JSON 配置」。
升级旧版时，第一次添加 Bot 会自动把旧 `LianT.db` 里的单 Bot 业务数据复制到 `bots/<id>.db`（原库保留）。

## 平台限制

各平台的能力差异由 `platform.Capabilities` 声明，这里只记接入时的硬限制。

### QQ

- 凭据是 **AppID + AppSecret**，在网页后台选平台 `qq` 后自适应显示两个字段；同样加密存储。
- QQ 消息主动发送有频控；卡片消息用 QQ Markdown，长度上限约 2000 字。
- 撤回他人消息需要机器人是群管理员，窗口约 2 分钟。
- 入群审批、群信息、成员列表、黑名单接口需要 QQ 平台白名单；常见错误码 `11253`。
- 群成员列表/群信息不可用时，后台仍可从机器人见过的成员名册里选人。

### Telegram

- 机器人必须是群管理员才能撤回他人消息、禁言、封禁；否则 Telegram 返回 `not enough rights`。
- 撤回只对 **48 小时内**的消息有效。
- Telegram **没有普通成员列表**：本地名册靠成员发言积累；按昵称找人前先让对方发句话。
- 只有 @机器人 / 回复机器人 / 命令 / 关闭 privacy mode 后的消息才会送到 Bot。
- 群人数、管理员列表可以通过 API 查询；完整成员列表查不到。
- 机器人不能封禁群主/其他管理员。

## Android / 手机

- `/sdcard` 一般是 `noexec`：**不要**把二进制放那里直接跑，放到 Termux 私有目录或应用数据目录。
- 如果 SQLite 报 `disk I/O error` / `database is locked`，把 `LIANT_DATA_DIR` 指到应用私有目录（如 `$HOME/.LianT`），不要放在 `/sdcard`（FUSE 对文件锁支持不完整）。
- 后台保活：`termux-wake-lock`；系统设置里允许 Termux 后台运行、关闭电池优化。
- 长期运行建议用 `LianT start`（守护进程模式），不要 `go run`。

## 目录

```
LianT/
├── main.go                 薄入口（保留 `go build .`）
├── cmd/liant/main.go       标准入口（`go build ./cmd/liant`）
├── internal/
│   ├── app/                CLI 主进程（start/stop/status/bot…）
│   ├── platform/           平台兼容层（Adapter/Capabilities）
│   │   ├── telegram/       Telegram 适配器
│   │   └── qq/             QQ 适配器 + QQ API + store 适配
│   ├── telegram/           Telegram Bot API 客户端（长轮询/消息/成员/禁言）
│   ├── bot/                业务：事件循环、规则、指令、处置动作
│   ├── manager/            多 Bot 注册表与运行时管理器
│   ├── qqbot/              QQ 网关/业务运行时（qbot 移植）
│   ├── admin/              网页管理端（admin.go + qbot_compat.go）
│   │   └── web/            后台页面（qbot 同款 UI）
│   │       ├── admin.html  页面骨架
│   │       ├── admin.css   样式
│   │       └── admin.js    前端逻辑
│   ├── store/              SQLite 存储层（注册表 + 每 Bot 业务库）
│   ├── rules/              回复规则 + 违禁词引擎（纯逻辑）
│   ├── config/             配置、路径、.env
│   ├── daemon/             任务进程装配
│   ├── ipc/                主进程 ↔ 任务进程协议
│   ├── logging/            日志轮转
│   └── process/            独立会话拉起
├── config/.env.example
└── docs/
    ├── api.md              管理端 API 参考
    ├── platforms.md        平台兼容层与 qbot 合并
    └── reverse-proxy.md    反代配置与排错
```

## 路线图

- [x] qbot（QQ）平台适配器与统一事件模型
- [ ] 管理端兼容层抽成平台无关接口（去掉 `qctx` 的 `b`/`q` 双指针）
- [ ] Discord / 其他平台 adapter
- [ ] 规则模板变量与更细的匹配条件
- [ ] 封禁/禁言到期自动解封的任务
- [ ] 管理端内联键盘审批入群申请

## 说明

LianT 是一个自托管的群管理机器人，不是任何平台的客户端，也不使用用户账号；
所有操作都通过各平台官方 Bot API / 网关完成。请遵守对应平台的服务条款和所在群组规则。

## 许可证

本项目以 **GNU Affero General Public License v3.0（AGPL-3.0）** 发布，完整条款见 [LICENSE](LICENSE)。

```text
LianT - 自托管的多平台群管理后台与机器人
Copyright (C) 2026  LensnTeam
```

用白话说：

- **自己用、给自家群用**：没有额外义务，改了也不必公开；
- **分发**（发二进制、发源码、打包进镜像）：必须同样以 AGPL-3.0 提供完整源码，并保留版权声明；
- **作为网络服务对外提供**（AGPL 第 13 条）：比如你把改过的 LianT 托管给别人用、让别人通过网页后台访问，
  就必须向这些用户提供你运行的这一版的源码。开个公网地址给人用却不开源，是不行的。

不修改、只原样运行官方版本的话，源码在仓库里就有，通常不构成额外负担。

版权归 **LensnTeam** 所有；如需其他授权（商业闭源等），请通过 <https://github.com/LensnTeam> 联系。


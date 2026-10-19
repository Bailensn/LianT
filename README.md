# LianT

**自托管的 Telegram 群管理机器人 + 网页管理后台。**

单文件 Go 程序，不需要公网 IP、域名或 HTTPS 回调 —— 用 Telegram Bot API 长轮询收消息；
所有配置与运营数据都在一个 SQLite 文件里，用网页后台或群内中文指令管理。

设计取向上参考了 [LensnTeam/LianT](https://github.com/LensnTeam/LianT)：Telegram Bot 管理、自托管、SQLite、网页后台。
本仓库是独立分叉，只做 Telegram。

---

## 功能

- **回复规则**：exact / prefix / contains / regex / any，优先级、限定群/私聊、模板变量。
- **违禁词**：命中后撤回 / 禁言 / 踢出 / 封禁 / 拉黑 / 警告回复，全部写审计。
- **群内指令**：中文「禁言 @某人 10分钟」「把张三踢了」和 `/mute`、`/ban`、`/del` 都支持。
- **成员处置**：禁言、解禁、踢出（可重新加入）、封禁、解封、撤回单条/批量。
- **黑名单**：本地拦截 + Telegram 封禁，解除时自动解封。
- **入群**：欢迎语（`{name}` / `{username}` / `{chat}`）、入群申请自动通过或留在 Telegram 里人工审批。
- **网页后台**：总览、群组、最近消息与主动发送、回复规则、违禁词、管理员、禁言、黑名单、审计、设置。
- **单文件 + SQLite**：纯 Go 驱动（`modernc.org/sqlite`），不需要 cgo/gcc，方便手机和 arm 机器。
- **两种收消息方式**：默认长轮询（无需公网/证书）；也可设 webhook + 密钥走公网回调。
- **媒体与交互**：图片/文件/视频/音频/语音/贴纸/动图/位置/投票/骰子、相册、按钮回调、话题、置顶、编辑消息。
- **内联查询**：`@机器人 关键词` 返回配置好的结果（`inline_results`）。
- **Bot API 全覆盖**：管理端「Bot API」页可直接调用任意 Bot API 方法，并支持上传本地文件
  （邀请链接、论坛话题、权限、贴纸、支付、Business 等）；支付预结算/配送查询可按设置自动确认。

## 架构

```
┌──────────────┐   指令(回环 TCP)   ┌────────────────────┐   SQLite   ┌──────────────┐
│ 主进程 LianT  │ ────────────────▶ │ 任务进程 LianT __run │ ─────────▶ │  LianT.db    │
│ 只发指令      │ ◀──────────────── │ Bot 长轮询 + 管理端  │ ◀───────── │ 规则/违禁词… │
└──────────────┘                   └─────────┬──────────┘             └──────────────┘
                                             │ HTTP(可反代)
                                             ▼
                                      admin.html ── 浏览器/手机
                                             │
                                             ▼
                                  https://api.telegram.org
```

- **主进程**：`LianT`，只发指令（start/stop/status/logs/admin…），随时开关不影响机器人。
- **任务进程**：`LianT __run`，全局唯一（绑定控制端口做单例），跑 Telegram 长轮询和网页后台。
- **规则/数据**：`LianT.db`，SQLite。
- **日志**：`LianT.log`，8MB 轮转，保留 `.1`~`.5`。

## 快速开始

### 1. 创建机器人

1. Telegram 找 **@BotFather**，`/newbot` 拿到 token（形如 `123456789:AA...`）。
2. 把机器人拉进群，设为**管理员**，至少勾选「删除消息」「封禁用户」。
3. 想让机器人看到群里全部消息（而不只是 @ 它的）：@BotFather → `/setprivacy` → 选机器人 → **Disable**。

### 2. 配置

```sh
cp config/.env.example .env
# 编辑 .env，填：
# LIANT_BOT_TOKEN=123456789:AA...
```

### 3. 编译

```sh
go mod tidy
go build -trimpath -ldflags "-s -w" -o LianT .
```

> 只需要 Go 1.27+（依赖 modernc.org/sqlite 要求 1.26+），不需要 gcc/cgo。Windows 编译出来是 `LianT.exe`。

### 4. 启动

```sh
./LianT init      # 生成 .env 模板 + 给数据库播种默认规则（可选）
./LianT start     # 后台启动任务进程
./LianT status    # 看机器人状态
./LianT admin     # 打印网页后台地址与 token
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
管理指令仅 **后台管理员** 或 **Telegram 群管理员** 可用（可在网页按群关掉后者）。

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

- **总览**：机器人状态、运行时长、消息数、规则/违禁词/禁言统计、最近审计。
- **群组**：自动登记 + 手动添加；启用/停用、回复所有消息、指令开关、群管理员权限、备注；
  「从 Telegram 同步」会拉群名并检查机器人是否管理员。
- **消息**：最近 60 条消息、撤回、主动发送（纯文本，写入审计）。
- **回复规则 / 违禁词**：网页增删改，保存后下一条消息生效。
- **管理员**：全局或限定某个群。
- **禁言 / 黑名单**：手动处置、解除。
- **操作日志**：谁、在哪个群、对谁、命中什么、结果如何。
- **设置**：回复策略、默认回复、入群自动审批、欢迎语；导出 JSON 配置。

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

## Webhook（可选）

默认长轮询。要改用 webhook：

```env
LIANT_WEBHOOK_URL=https://你的域名/telegram/webhook
LIANT_WEBHOOK_SECRET=随机字符串
LIANT_WEBHOOK_LISTEN=127.0.0.1:28743
```

反向代理把 `/telegram/webhook` 转到 `127.0.0.1:28743`。LianT 启动时会自动 `setWebhook`，并校验
`X-Telegram-Bot-Api-Secret-Token`；没有密钥的请求直接 403。

## Bot API 控制台

网页后台「Bot API」页可以调用 Telegram 支持的任何方法，例如：

`getChat`、`getChatAdministrators`、`createChatInviteLink`、`createForumTopic`、
`setChatPermissions`、`pinChatMessage`、`sendMediaGroup`、`getWebhookInfo`、`getMyCommands` 等。

参数是 JSON 对象，结果原样展示。需要本地文件的方法（`sendPhoto`、`sendDocument`、`createNewStickerSet` 等）
用「上传本地文件并调用」：填方法名、文件字段名（`photo`/`document`/`png_sticker`…）和参数，选文件即可。
这样 Telegram 后续新增的方法不需要等 LianT 更新也能直接用；常用操作（发送/禁言/撤回/群组）仍有专门的 UI。

## 配置

`.env` 只放启动期配置；业务数据都在数据库里。优先级：**环境变量 > `.env` > 代码默认值**。

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `LIANT_BOT_TOKEN` | 空（必填） | BotFather 给的 token |
| `LIANT_API_BASE` | `https://api.telegram.org` | 自建网关/反代时改 |
| `LIANT_PROXY` | 空 | `http://127.0.0.1:7890` 或 `socks5://...` |
| `LIANT_PORT` | `28741` | 控制端口（多实例时各用一个） |
| `LIANT_BIND` | `127.0.0.1` | 控制端口绑定地址 |
| `LIANT_DATA_DIR` | 当前目录/`LianT-data` | 数据目录 |
| `LIANT_DB_FILE` | `<数据目录>/LianT.db` | SQLite 文件 |
| `LIANT_LOG_FILE` / `LIANT_PID_FILE` / `LIANT_RUNTIME_FILE` | 数据目录下 | 单独指定 |
| `LIANT_ADMIN_ADDR` | `127.0.0.1:28742` | 管理端监听地址 |
| `LIANT_ADMIN_TOKEN` | 自动生成 | 管理端 token |
| `LIANT_ADMIN_PREFIX` | 空 | 反代挂载前缀，如 `/LianT` |
| `LIANT_ADMIN_HTML` | 内置 | 自定义管理页 |
| `LIANT_ADMIN_ENABLE` | `true` | `false` 关闭网页后台 |
| `LIANT_WEBHOOK_URL` | 空 | 公网 HTTPS 回调地址；设置后从长轮询切到 webhook |
| `LIANT_WEBHOOK_SECRET` | 空 | webhook 校验密钥（`X-Telegram-Bot-Api-Secret-Token`） |
| `LIANT_WEBHOOK_LISTEN` | `127.0.0.1:28743` | webhook 本地监听地址 |
| `LIANT_WEBHOOK_PATH` | `/telegram/webhook` | webhook 本地路径 |

改 Token / 规则**不用重启**：保存后最长 60 秒生效，或 `./LianT reload`。
改端口 / 绑定地址 / 数据目录需要 `./LianT restart`。

### CLI

| 指令 | 作用 |
| --- | --- |
| `LianT` | 交互控制台 |
| `LianT start` / `stop` / `restart` | 启动 / 停止 / 重启任务进程 |
| `LianT status` | 机器人状态、规则数、数据库、管理端地址 |
| `LianT logs [行数] [-f]` | 看日志 |
| `LianT admin` | 网页后台地址 + token |
| `LianT db` | 数据库统计 |
| `LianT reply [文本]` | 干跑回复规则（不发消息） |
| `LianT reload` | 重载规则缓存与 Token |
| `LianT reconnect` | 尽快重连 Telegram |
| `LianT path` | 打印所有路径 |
| `LianT run` | 前台运行（调试，日志打屏） |

## 数据库

默认 `<数据目录>/LianT.db`，表：

| 表 | 内容 |
| --- | --- |
| `settings` | reply_all、reply_default、join_auto_approve、welcome_text、admin_token、tg_offset 等 |
| `reply_rules` | 回复规则 |
| `banned_words` | 违禁词与处置 |
| `groups` | 会话开关（chat_id 为 Telegram 群 ID） |
| `admins` | 后台管理员（全局/限定群） |
| `mutes` | 禁言记录 |
| `blacklist` | 本地黑名单 |
| `members` | 本地名册（来自成员发言） |
| `audit` | 操作日志 |

备份：拷 `LianT.db`，或网页「设置 → 导出 JSON 配置」。

## Telegram 侧限制

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
├── main.go                 CLI / 控制台 / 进程拉起
├── internal/
│   ├── cfg/                配置、路径、.env
│   ├── tg/                 Telegram Bot API 客户端（长轮询/消息/成员/禁言）
│   ├── bot/                业务：事件循环、规则、指令、处置动作
│   ├── admin/              网页管理端（admin.go + admin.html）
│   ├── store/              SQLite 存储层
│   ├── rules/              回复规则 + 违禁词引擎（纯逻辑）
│   ├── daemon/             任务进程装配
│   ├── ipc/                主进程 ↔ 任务进程协议
│   ├── logfile/            日志轮转
│   └── proc/               独立会话拉起
├── config/.env.example
└── docs/
```

## 路线图

- [ ] 多 Bot 管理（一个实例管理多个 token / 群）
- [ ] 规则模板变量与更细的匹配条件
- [ ] 封禁/禁言到期自动解封的任务
- [ ] 管理端内联键盘审批入群申请

## 说明

LianT 是一个 Telegram 群管理机器人，不是 Telegram 客户端，也不使用用户账号；
所有操作都通过官方 Bot API 完成。请遵守 Telegram 服务条款和所在群组规则。

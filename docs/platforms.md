# 平台兼容层与 qbot 合并说明

目标：**一套 Web 后台 + 一套管理 API**，同时管理 Telegram、QQ（qbot）以及以后新增的平台。
平台差异收在 `internal/platform/`，业务数据统一用中性的 `chat_id` / `user_id`。

## 1. 运行时模型

```
任务进程
├── 注册表 LianT.db
│   └── bots(id, name, token, platform, platform_id, mode, webhook…, enabled)
├── Bot #1 → bots/1.db + platform adapter + runtime
├── Bot #2 → bots/2.db + platform adapter + runtime
└── 网页后台（共享 admin.html，按 ?bot_id= 选择）
```

- 注册表只存 Bot 身份/启停/平台配置，不存业务规则。
- 每个 Bot 的业务库表结构一致（groups/admins/rules/words/mutes/blacklist/members/join_requests/audit）。
- 后台所有业务接口都接受 `?bot_id=<id>`；不带时退回第一个运行中的 Bot。

## 2. Adapter 契约

```go
type Adapter interface {
    ID() string
    Name() string
    Capabilities() Capabilities
    ValidateToken(token string) (Identity, error)
}

type Identity struct {
    ID       string // 平台账号 ID（字符串，避免 QQ/Telegram 数值差异）
    Username string
    Name     string
}
```

当前实现：

- `internal/platform/telegram`（Telegram Bot API）
- `internal/platform/qq`（QQ 开放平台 API + Adapter）
- `internal/qqbot`（QQ WebSocket 网关/业务运行时，从 qbot 移植）
- `internal/platform/qq/store`（把 LianT 的 chat_id/user_id 库适配成 qbot 的 OpenID 视图）

新增平台：

1. 新建 `internal/platform/<id>/<id>.go`，实现 `Adapter`。
2. `init()` 里 `platform.Register(adapter{})`。
3. `Capabilities` 按平台真实能力填，不要照抄 Telegram。
4. 平台的 runtime（QQ 网关、Discord Gateway 等）由平台自己的包实现，管理器只负责注册与启动。

## 3. Capabilities

| 能力 | Telegram | QQ（已实现） |
| --- | --- | --- |
| `text` | ✅ | ✅ |
| `media` | ✅ | ✅ |
| `buttons` | ✅ inline keyboard | QQ 按钮/ark 视接口而定 |
| `inline` | ✅ | ❌ |
| `topics` | ✅ forum | ❌ |
| `invite_links` | ✅ | QQ 群邀请链接接口受限 |
| `webhook` | ✅ | ❌（QQ 用 WebSocket 网关） |
| `join_approval` | ✅ | ✅（入群申请接口） |
| `member_list` | ❌（Telegram 不提供） | 视 QQ 白名单接口 |
| `roles` | ✅ | ✅ |
| `payments` | ✅ | ❌ |
| `recall_48h` | ✅ | QQ 撤回窗口更短 |

管理端根据这些字段显示/隐藏入口；例如 QQ 不会显示 Webhook、话题、内联、支付。

## 4. 后台 API 契约

平台无关接口（所有平台都要实现）：

| 路径 | 说明 |
| --- | --- |
| `GET /api/bots` | Bot 列表（含 platform / capabilities / running） |
| `POST /api/bots` | 添加 Bot |
| `POST /api/bots/update` | 更新并重启 |
| `POST /api/bots/restart` | 重启 |
| `DELETE /api/bots?id=` | 删除注册 |
| `GET /api/platforms` | 已注册平台与能力 |
| `GET /api/overview?bot_id=` | 状态与计数 |
| `GET/POST /api/chats` | 会话 |
| `GET/POST /api/rules` | 回复规则 |
| `GET/POST /api/words` | 违禁词 |
| `GET/POST /api/admins` | 管理员 |
| `GET/POST /api/mutes` | 禁言 |
| `GET/POST /api/blacklist` | 黑名单 |
| `GET /api/joins`、`POST /api/joins` | 入群申请审批 |
| `POST /api/send`、`POST /api/broadcast` | 发消息 / 群发 |
| `POST /api/recall` | 撤回 |
| `GET /api/audit` | 审计 |
| `GET/POST /api/settings` | Bot 级设置 |
| `POST /api/botapi`、`POST /api/upload` | 平台原生 API 透传 |

字段统一：`chat_id`、`user_id`、`message_id`、`username`、`platform`、`platform_id`。
不要引入 `group_openid` / `member_openid` 这类平台专有字段。

## 5. qbot 字段映射

| qbot 现有 | LianT 中性层 |
| --- | --- |
| `group_openid` | `chat_id` |
| `member_openid` / `user_openid` | `user_id` |
| QQ `message_id` / `msg_id` | `message_id` |
| `Author.Username` | `username` |
| `groups` 表 | 同名，列改 `chat_id` |
| `admins` 表 | 同名，列改 `user_id` / `chat_id` |
| QQ 网关事件 | 平台 runtime 内部转换，不进入 Admin API |

SQLite 迁移可以直接：

```sql
ALTER TABLE groups RENAME COLUMN group_openid TO chat_id;
ALTER TABLE admins RENAME COLUMN member_openid TO user_id;
-- …其余表同理
```

或者保留旧列，在 adapter 层做视图/映射；但新代码只认 `chat_id` / `user_id`。

## 6. qbot 合并状态

已完成：

1. **平台包**：`internal/platform/qq` 实现 Adapter，`internal/qqbot` 跑 QQ WebSocket 网关。
2. **数据模型**：QQ runtime 通过 `internal/platform/qq/store` 适配 LianT 的 `chat_id/user_id` 业务库，不维护第二套规则/群数据。
3. **管理 API**：qbot 页面所需的 `chat/*`、`qq/*`、`groups`、`joins` 已在 `admin/qbot_compat.go` 中分支到 QQ runtime。
4. **UI**：同一份 `admin/web/admin.html`，添加 Bot 时选 QQ，Token 填 `AppID:AppSecret`。
5. **凭据**：QQ token 与 Telegram token 一样 AES-256-GCM 加密；一个进程可同时跑两种平台。
6. **能力开关**：QQ 隐藏 inline/topics/webhook/payments；保留入群审批/禁言/撤回/卡片。

后续：

- 把 QQ 的 `QqResp` 错误码映射补全到 UI 提示（目前直接显示 `Hint()`）。
- Discord 等新平台继续按 Adapter 接入。

## 7. 已知差异

- Telegram 不提供完整成员列表，`member_list=false`；QQ 是否可用取决于白名单接口。
- QQ 没有 Telegram 的 inline/webhook/forum topic；用 capabilities 隐藏。
- Telegram 的撤回窗口 48 小时；QQ 更短，`max_recall_window` 可按平台声明。
- 支付只在声明 `payments=true` 的平台显示。

新增能力时，先加 `Capabilities` 字段，再在 UI 里按字段显示；不要在页面里写死平台名。

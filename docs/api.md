# 管理端 HTTP API

网页后台和 API 是**同一个 HTTP 服务**，默认监听 `127.0.0.1:28742`，同源、无 CORS。

| 路径 | 内容 |
| --- | --- |
| `/` | 管理页面（`admin.html`） |
| `/admin.css` | 页面样式 |
| `/admin.js` | 页面逻辑 |
| `/api/*` | 本文档描述的接口 |

三个静态文件都编译进二进制（`go:embed`），并带 `Cache-Control: no-store`；磁盘上有同名文件时优先用磁盘的（改完刷新即生效，不用重编译）。

---

## 1. 鉴权

三种方式任选其一：

| 方式 | 写法 |
| --- | --- |
| 请求头 | `Authorization: Bearer <token>` |
| 查询串 | `?token=<token>`（打开页面时会顺带写 cookie） |
| Cookie | `LianT_admin_token=<token>` |

- token 来源优先级：`LIANT_ADMIN_TOKEN` 环境变量 > `.env` 同名项 > 数据库自动生成（`./LianT admin` 查看）。
- 未授权返回 `401`，并带 `WWW-Authenticate: Bearer realm="LianT"`。
- 管理端绑非回环地址又没配 token 时**拒绝启动**。

## 2. 返回格式

成功：

```json
{ "ok": true, "data": { } }
```

失败：

```json
{ "ok": false, "error": "人类可读的原因" }
```

| 状态码 | 含义 |
| --- | --- |
| `200` | 成功 |
| `400` | 参数 / JSON 不合法 |
| `401` | 未授权 |
| `404` | 资源不存在（如消息不在本地库） |
| `405` | 方法不对 |
| `409` | 还没有可用 Bot，或该能力不支持当前平台 |
| `410` | 媒体已失效 / 引用已过期 |
| `413` | 媒体超过单次取回上限 |
| `500` | 存储 / 内部错误 |
| `502` | 平台 API 返回错误 |
| `503` | Bot 尚未连接（如 Telegram 客户端未就绪） |

## 3. 选择 Bot

所有业务接口都接受 `?bot_id=<id>`；不带时回退到「第一个运行中的 Bot」。

多 Bot 部署时，页面右上角切换 Bot 实际就是换这个参数。先 `GET /api/bots` 拿到 `id`。

## 4. 路由

### 4.1 平台无关

| 路由 | 方法 | 说明 |
| --- | --- | --- |
| `/api/overview` | GET | 概览：Bot 状态、各项计数、最近审计、指令帮助 |
| `/api/bots` | GET / POST / DELETE | Bot 列表 / 添加 / 删除（`?id=`） |
| `/api/bots/update` | POST | 更新配置并重启 |
| `/api/bots/restart` | POST | 重启 |
| `/api/platforms` | GET | 已注册平台与能力声明 |
| `/api/chats` | GET / POST / DELETE | 会话（群）列表、启停、备注 |
| `/api/rules` | GET / POST / PUT / DELETE | 回复规则 |
| `/api/words` | GET / POST / PUT / DELETE | 违禁词 |
| `/api/admins` | GET / POST / DELETE | 群管理员名单 |
| `/api/members` | GET | 成员名册（`?chat_id=` 可过滤） |
| `/api/mutes` | GET / POST / DELETE | 禁言 / 解禁 |
| `/api/blacklist` | GET / POST / DELETE | 黑名单 |
| `/api/joins` | GET / POST | 入群申请列表 / 审批 |
| `/api/audit` | GET | 审计日志 |
| `/api/audit/prune` | 任意 | 清理审计（`?keep=2000`） |
| `/api/settings` | GET / POST | Bot 级设置 |
| `/api/messages` | GET | 聊天记录（`?chat_id=` 必填；默认全部，`?limit=` 可限制） |
| `/api/media` | GET | 取回消息媒体（`?chat_id=&msg=`） |
| `/api/send` | POST | 发消息（文本 / 媒体 / 按钮） |
| `/api/broadcast` | POST | 群发 |
| `/api/recall` | POST | 撤回 |
| `/api/export` | GET | 导出业务库 JSON（`LianT-config.json`） |
| `/api/reload` | 任意 | 重载规则缓存、请求重连 |
| `/api/help` | 任意 | 群内指令帮助文本 |

### 4.2 仅 Telegram

| 路由 | 方法 | 说明 |
| --- | --- | --- |
| `/api/chats/sync` | POST | 拉取群名与机器人管理员状态 |
| `/api/chats/info` | GET | 单个会话详情（`?chat_id=`） |
| `/api/chats/admins` | GET | 群管理员列表（`?chat_id=`） |
| `/api/botapi` | POST | 透传任意 Bot API 方法 |
| `/api/upload` | POST | multipart 上传后调 Bot API |

QQ Bot 上调用这些路由会返回 `409`，不会 500。

### 4.3 兼容层（qbot 形态）

后台页面实际调用的这批路由由 `internal/admin/qbot_compat.go` 提供，字段同时接受 qbot 命名与中性命名：

| 路由 | 说明 |
| --- | --- |
| `/api/groups` | 群列表（`group_openid`） |
| `/api/groups/sync-names` | 批量同步群名 |
| `/api/admins`、`/api/members`、`/api/mutes`、`/api/blacklist` | 同上，兼容两种字段名 |
| `/api/joins` | 入群审批（`approve` / `decline`） |
| `/api/chat/groups`、`/api/chat/recent`、`/api/chat/send` | 会话、聊天记录、发消息 |
| `/api/qq/*` | QQ 专属：`botstate`、`groupinfo`、`members`、`mutestate`、`kick`、`recall` |

字段对照：

| qbot 写法 | 中性写法 |
| --- | --- |
| `group_openid` / `GroupOpenID` | `chat_id` |
| `member_openid` / `MemberOpenID` | `user_id` |
| `QQSynced` / `TGSynced` | 黑名单同步状态 |

响应里两套 key 都会给，旧页面不用改。

### 4.4 媒体

`GET /api/media?chat_id=<会话>&msg=<消息ID>&bot_id=<可选>`

只在本地消息库（`bots/<id>.db`）里按「会话 + 消息」查引用，不能凭任意 `file_id` / URL 取文件：

- **Telegram**：`getFile` 换临时 `file_path` 后由服务端流式转发；带 bot token 的
  `https://api.telegram.org/file/bot<token>/...` 不会出现在页面里。
- **QQ**：服务端代理平台附件直链；签名 URL 会过期，但不会暴露给浏览器。
- 响应是字节流，`Content-Type` 来自消息元数据；`file` 走 `attachment`，
  图片/音视频走 `inline`，并带 `nosniff` 与只读 CSP。
- 单次上限 32 MB；Telegram Bot API 自身只允许下载 20 MB。
- 撤回的消息不提供媒体：墓碑不保留附件引用。

| 状态码 | 含义 |
| --- | --- |
| `400` | 缺 `chat_id` / `msg` |
| `404` | 消息不在本地库，或没有媒体 |
| `409` | 这个媒体没有可下载内容（例如表情，按 emoji 显示） |
| `410` | 平台文件已失效或引用已过期；同时把该消息的 `media.state` 标为 `gone` / `expired` |
| `413` | 超过单次取回上限 |
| `502` | 平台请求失败；原因只写任务进程日志，不回传平台地址 |

网页里的 `<img>` / `<video>` 用 cookie 鉴权；API 客户端用
`Authorization: Bearer <token>`。不要把 token 拼进媒体 URL。

### 消息条目与状态

聊天记录保存在每个 Bot 的 SQLite（`bots/<id>.db` 的 `messages` 表），默认**不设条数上限**；
`/api/chat/recent`、`/api/messages` 默认返回该会话全部历史，`?limit=N` 可只取最近 N 条。
撤回只清正文与媒体引用，条目本身保留为墓碑。

`/api/chat/recent`、`/api/messages` 返回的每条消息：

| 字段 | 说明 |
| --- | --- |
| `id` / `message_id` | 平台消息 ID |
| `username`、`member_openid` / `user_id` | 发送者 |
| `content`、`kind` | 正文与类型（text/image/video/voice/file/sticker/…） |
| `is_bot` | 是否机器人自己发出 |
| `state` | `received` 群内收到 / `sent` 平台已受理 / `recalled` 已撤回（正文已清空） |
| `at` | RFC3339 时间 |
| `media` | 可选。媒体描述；没有媒体时不出现 |

`media` 只给事实，不含平台地址：

| 字段 | 说明 |
| --- | --- |
| `kind` | `image` / `video` / `audio` / `voice` / `file` / `sticker` / `animation` / `video_note` |
| `name`、`mime`、`size`、`width`、`height`、`duration` | 平台提供时才出现 |
| `emoji` | 表情对应的 emoji（`kind=sticker`） |
| `state` | `available` 引用可用 / `expired` 引用过期 / `gone` 平台已无此文件 / `unknown` 尚未验证（QQ 签名直链） |
| `fetchable` | 是否有可取回的字节；表情为 `false`，界面按 emoji 渲染 |

`available` 只表示引用可用，不代表已经下载；取回失败时接口会把状态改成
`expired` / `gone`，下一次消息查询就能看到，界面据此显示「不可取回」，而不是伪造缩略图。

`sent` 只代表平台受理了发送请求，**不代表送达或已读**；平台不提供回执时，界面不会伪造「已送达」。
`recalled` 是墓碑：条目保留发送者与时间，正文为空，不可再引用或撤回。

## 5. 示例

```bash
BASE=http://127.0.0.1:28742
TOKEN=$(./LianT admin | sed -n 's/.*token: *//p')

# 概览
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/api/overview?bot_id=1"

# 添加 Telegram Bot
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"群管","platform":"telegram","token":"123456:ABC-DEF","mode":"polling"}' \
  "$BASE/api/bots"

# 添加 QQ Bot（AppID:AppSecret）
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"QQ群管","platform":"qq","app_id":"1020xxxx","app_secret":"xxxx"}' \
  "$BASE/api/bots"

# 发文本
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"chat_id":"-1001234567890","content":"hello"}' \
  "$BASE/api/send?bot_id=1"

# 取回一条消息的媒体（浏览器用 cookie；curl 用 Bearer）
curl -s -H "Authorization: Bearer $TOKEN" -o media.bin \
  "$BASE/api/media?bot_id=1&chat_id=-1001234567890&msg=42"

# 上传文件并调用 Bot API（method / file_field / params / file）
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -F method=sendPhoto -F file_field=photo -F chat_id=-1001234567890 -F file=@cover.jpg \
  "$BASE/api/upload?bot_id=1"
```

## 6. 限制与安全

- 请求体上限：JSON `16MB`，multipart `32MB`；超限直接 `400`。
- 没有速率限制、没有 CORS：**不要**把管理端直接暴露到公网，前面加反代 + HTTPS + token。
- 所有写操作都会写 `audit` 表（网页「操作日志」可见）。
- 接口会校验 Bot API 方法名只含字母数字，避免拼进 URL 做路径注入。

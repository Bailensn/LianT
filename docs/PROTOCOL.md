# 协议 v1

Manager（Flutter 客户端）与 Service（Go 服务端）之间的唯一边界。改动这里等于改两端，
所以字段、错误码、签名编码都以本文为准；实现偏离时先改本文再改代码。
Go 侧的对应实现在 `service/internal/protocol/`，Flutter 侧在 `manager/lib/core/net/`。

## 1. 传输

| 项 | 值 |
| --- | --- |
| 地址 | `wss://<host>:<port>/v1/ws`，端口默认 `8443`（`connect.listen` 可改） |
| 帧 | 文本帧，UTF-8 JSON；单帧上限 **8 MiB**（`protocol.MaxFrame`），超限断开并回 `frame_too_large` |
| 心跳 | 服务端每 30s 发一次 WebSocket 控制帧 ping；协议层另有 `ping`/`pong` 消息类型 |
| 空闲 | 认证后 5 分钟收不到任何帧（含 ping）即断开，只是兜底回收死连接 |
| 握手超时 | 15s；写超时 10s |
| 证书 | 节点密钥自签，客户端**允许自签**：TLS 只做传输加密，身份靠 §4 的应用层自证 |
| 反代 | `LIANT_NO_TLS=1` 或 `serve --no-tls`，由 Caddy/Nginx 终止 TLS |

浏览器拿不到 TLS 证书，绕不过系统信任库，所以生产环境要给正式证书或反代；
Android/桌面端不校验 CA 链，改为校验 §4 的 `service_pub` 指纹。

**连接生命周期**：TCP → TLS → WS Upgrade → `auth.challenge` → `auth.response` →
`auth.ok` → 业务（`state.snapshot` / `bot.list` → 增量事件）→ 断开（重连或关闭码）。

## 2. 身份、指纹与命名空间

| 对象 | 密钥 | 私钥位置 | 指纹 |
| --- | --- | --- | --- |
| Service（节点） | ECDSA P-256 | 服务器 `data/node/`（0600，`node.key`） | `base32(SHA-256(公钥 DER))`，每 4 位分组 |
| Manager（设备） | ECDSA P-256 | 客户端本地存储/应用私有目录 | 同上，服务端只存公钥 |

- 指纹是**唯一的信任锚**：配对时钉住，之后每次连接都拿 `auth.challenge.service_pub`
  现算指纹比对。换节点密钥等于换身份，所有设备都要重新配对。
- 证书由私钥签发，剩余不足 30 天时自动用同一私钥重签，**指纹不变**。
- v1 用 P-256 而不是 ed25519：Android Keystore 对 ed25519 的支持依系统版本与厂商而异。
- **已知缺口**：客户端私钥还没进 Keystore/WebCrypto，明文存在应用私有目录。
  能读到应用数据的人就能拿到私钥，服务端只保存公钥。
- v1 没有 session 票据：每次连接都挑战应答，撤销设备 = 服务端删公钥。

**ID 命名空间** `"<platform>:<external_id>"`：

- `platform` 匹配 `^[a-z][a-z0-9_]{1,15}$`，v1 只有 `qqbot`，Telegram 是 v1.1。
- `external_id` 匹配 `^[A-Za-z0-9_-]{1,64}$`（QQ 的 openid / group_openid 都是字符串）。
- **`peer_id` 只在单个 `bot_id` 内唯一**：QQ 的 openid 按 App 隔离，同一个人在不同 Bot 下
  ID 不同。所以会话键永远是 `(bot_id, peer_id)`，每条消息都必须带 `bot_id`。
- **「当前活跃 Bot」是客户端本地状态**：服务端不感知、也不上报；切号不重连、不换 session。

## 3. 配对（一次性）

1. 服务端 `LianT pair` 生成 6 位配对码与二维码，终端同时打印配对信息 JSON
   （`data/pair-qr.png` 也留一份）：

   ```json
   {"v":1,"host":"liant.example.com","port":8443,"fp":"ABCD-EFGH-…","nonce":"…","exp":1793520000}
   ```

   | 字段 | 含义 |
   | --- | --- |
   | `host` / `port` | 手机可达的地址（见 DEPLOY §4 的取值优先级） |
   | `fp` | 节点指纹，客户端钉住 |
   | `nonce` | 一次性挑战，客户端待签 |
   | `exp` | 过期时间（unix 秒） |

2. Manager 连上 WS 后发 `pair.request {ticket, manager_name, manager_pub, sig}`，
   `sig` 是对二维码里 `nonce` 的签名，证明请求方确实持有待绑定的私钥。
3. 服务端**在终端要求本地确认**（显示设备名与指纹前 8 位）。二维码单独泄漏不足以完成绑定。
4. 确认 → `pair.ok {service_id, service_name, devices_count, device_id}`；
   拒绝或失败 → `pair.denied {reason}`，常见 `invalid_sig`、`expired`、`bad_ticket`、`limit`。
5. `device_id` 是本次授权的设备标识，之后的 `auth.response.manager_id` 用它。

限制：配对码**一次性、60 秒过期**；默认最多 8 台设备；过期或错码只能重新 `LianT pair`。

**auto_pair（Basic 认证下）**：配置了 HTTP Basic 认证时，`auth.challenge` 带
`auto_pair: true`，客户端可省掉 `ticket` 直接登记。这条路径以已通过的 HTTP 认证为准，
不要求 nonce 签名、也不消费配对码。没有 Basic 认证时 `auto_pair=false`，必须走配对码。

## 4. 握手（每次连接）

```text
Manager                          Service
   |── WS Upgrade ───────────────▶|   TLS 校验（客户端钉指纹，见下）
   |◀─ auth.challenge ─────────────|   {nonce, ts, service_pub, service_sig, auto_pair?}
   |── auth.response ─────────────▶|   {manager_id, sig}
   |◀─ auth.ok ────────────────────|   {version, service_time, caps, retention}
```

**`auth.challenge` 字段**：`nonce`（一次性随机串）、`ts`（服务端毫秒时间）、
`service_pub`（base64 PKIX DER 公钥）、`service_sig`（对该挑战的签名）、`auto_pair`（可选）。

**签名编码（v1 固定，差一个字节就失败）**：

- 待签字节：`nonce + "\n" + fp + "\n" + 十进制 ts`（UTF-8），`fp` 与二维码里的一致。
- 算法：ECDSA P-256 + SHA-256；签名输出 ASN.1 DER 后 base64。
- 服务端自证：客户端先算 `base32(SHA-256(service_pub DER))`，与钉住的 `fp` 比对，
  一致再用该公钥验 `service_sig`；不一致直接断开。
  这一步把证书钉扎补在应用层——反代、企业根证书、被攻陷的 CA 都冒充不了服务端。
  老客户端忽略这两个字段（仍受 TLS 约束）。

**`auth.response`**：`{manager_id, sig}`，`manager_id` 就是配对拿到的 `device_id`，
`sig` 用同一串字节、用客户端私钥签。

**校验与限流**：

- `ts` 偏差超过 60 秒直接拒绝（`clock_skew`）；`nonce` 一次性，重放必败。
- 认证失败统一 `auth_failed`（不区分「公钥未知」与「签名错」，避免枚举设备）。
- 同一 IP 每分钟最多 5 次失败；单连接最多 3 次尝试，超限用关闭码 4001 断开。
- 认证前只接受 `pair.request` / `auth.response` / `ping`，其它类型回 `unauthorized`。

**`auth.ok`**：`version`（服务端版本，客户端据此提示更新）、`service_time`、
`caps`、`retention`（见 §9）。

- `caps` 固定为 `["msg.list","bot.control","media.recv","rich_text","multi_bot"]`。
- `multi_bot` 表示一条连接能承载所有 Bot 的事件与请求（D9 依赖它）。
- 连上后客户端通常先发一次 `state.snapshot` 拿首屏，再按游标 `msg.list` 补拉。

## 5. 信封

```json
{"v":1,"id":"b3f1c0a2-…","type":"msg.send","ts":1793520000123,"reply_to":"b3f1c0a2-…","payload":{}}
```

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `v` | 是 | 协议版本，当前 1；不兼容时服务端回 `unsupported_version` 并用 4002 关闭 |
| `id` | 是 | 发送方生成、全局唯一；响应用 `reply_to` 指回它 |
| `type` | 是 | 见 §7；未知类型回 `unknown_type` |
| `ts` | 是 | 毫秒时间戳，**仅用于排序与调试**，不参与验签 |
| `reply_to` | 响应必有 | 指向被响应的 `id` |
| `payload` | 是 | 类型相关的对象，可以为 `{}` |

未知字段一律忽略（只加字段不升版本）；未知 `type` 不是致命错误，回 `error` 后连接继续。

## 6. Body 与 NDoc

文本内容一律走 `body`，不为每种消息类型定义不同字段：

```json
{
  "plain": "see this docs",
  "doc": {"v":1,"blocks":[
    {"type":"p","spans":[
      {"t":"text","text":"see "},
      {"t":"bold","text":"this"},
      {"t":"link","text":"docs","url":"https://example.com"}
    ]},
    {"type":"code","lang":"go","text":"fmt.Println()"}
  ]}
}
```

- `plain` 与 `doc` **至少有一个**（`Body.Validate`）；`plain` 用于通知栏、搜索与降级显示。
- **上限**（解析前检查，防内存放大）：`doc` ≤ 32 KiB、≤ 200 块、每块 ≤ 20 span、
  每块 ≤ 10000 字符、`url` ≤ 2048；超限回 `content_too_long`，结构不合法回 `invalid_doc`。
- 块：`p`、`code`（可带 `lang`）、`quote`。
- 行内 span：`text`、`bold`、`italic`、`strike`、`code`、`link`；**span 不嵌套**。
- 降级：未知块按 `p` 渲染，未知 span 只取文本，**不报错**。
- 客户端把 `doc` 映射到 Compose 等价物 `AnnotatedString`，**不拼 HTML、不进 WebView**。
- QQ 的 markdown 模板与 keyboard 属于适配器内部，**不进 `doc`**；v1 出站降级为纯文本，
  入站能识别的行内标记才进 `doc`。
- 金样本测试：同一 `doc` 渲染再 parse 回来应等值；恶意输入（`<b>`、`**`、`\`、
  超长嵌套、代理对）必须只当文本。

## 7. 方法与事件

### 7.1 配对与认证

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `pair.request` | M→S | 否 | `{ticket, manager_name, manager_pub, sig}` |
| `pair.ok` | S→M | — | `{service_id, service_name, devices_count, device_id}` |
| `pair.denied` | S→M | — | `{reason}` |
| `auth.challenge` | S→M | — | `{nonce, ts, service_pub, service_sig, auto_pair?}` |
| `auth.response` | M→S | 是 | `{manager_id, sig}` |
| `auth.ok` | S→M | — | `{version, service_time, caps, retention}` |

### 7.2 Bot

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `state.snapshot` | M→S | 是 | `{}` → `{bots, devices, audit, caps}`，首屏一次拿全 |
| `bot.list` | M→S | 是 | `{}` → `{bots:[…]}` |
| `bot.add` | M→S | 是 | `{platform, credentials, label?}` → `{bot_id}`；凭据形状由平台定义 |
| `bot.remove` | M→S | 是 | `{bot_id}` |
| `bot.start` / `bot.stop` | M→S | 是 | `{bot_id}` |
| `bot.status` | M→S | 是 | `{bot_id}` → `{status, pid?, since?, last_error?}` |
| `event.bot` | S→M | — | `{bot_id, status, reason?}`，单 Bot 粒度状态变化 |

`bot.status.status` ∈ `starting/running/stopped/error`（与 `platform.State*` 一致）。

### 7.3 消息

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `msg.list` | M→S | 是 | `{bot_id, peer_id, before?\|after?, limit≤100}` → `{messages[], has_more}` |
| `msg.send` | M→S | **是** | 见下 → `{msg_id, ts}` |
| `msg.ack` | M→S | 是 | `{msg_id}`，客户端已落库 |
| `msg.inbound` | S→M | — | 协议事件，同 `event.msg` |
| `event.msg` | S→M | — | 见下；**所有 Bot 都推**，响不响由客户端按当前账号决定 |

**`msg.send` 字段**：

| 字段 | 说明 |
| --- | --- |
| `bot_id` / `peer_id` | 必填，会话键 |
| `body` | 见 §6；带 `media` 时可不填（但不能两者都空） |
| `media` | `{kind, name, data}`，`kind ∈ image/video/audio/file`，`data` 为 ≤5 MiB 标准 base64 |
| `in_reply_to` | 触发本次回复的入站 `msg_id`。QQ 被动回复**必须**带，否则按主动消息处理 |
| `quote_ref` | QQ 引用所需的 `ref_idx`（来源入站消息的 `message_scene.ext.msg_idx`），**必须配合 `in_reply_to`** |
| `active` | `true` = 主动消息，不绑定触发消息（也就没有 @ 与引用样式），是否允许由 `constraints.can_initiate` 与配额决定 |
| `operator_id` | 只用于审计，**永不下发平台** |

**`msg.inbound` / `event.msg` 字段**：`bot_id, peer_id, peer_kind, sender_id?, sender_name?,
sender_role?, msg_type, body, msg_id, ts, in_reply_to?, quote_ref?, direction?`

- `peer_kind` ∈ `private/group/channel`，决定界面是单聊还是群聊。
- `sender_role` ∈ `owner/admin/member`（QQ 需要成员信息接口权限，拿不到就为空）。
- `msg_type` ∈ `text/image/video/audio/file`；媒体消息的 `body.plain` 是说明文字或文件名。
- QQ 只推送「@ 到机器人」的群消息，所以入站本身就代表「被提及」。

**`msg.list`**：游标 `{ts, msg_id}`，`before` 往前翻、`after` 往后补（两者不能同时出现），
`limit` 0–100（0 用服务端默认）；返回 `has_more`。服务端按 `(ts, msg_id)` 稳定排序，
所以同一毫秒的多条消息不会错位。

### 7.4 媒体

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `media.fetch` | M→S | 是 | `{bot_id, msg_id}` → `{url, exp, size, mime}` |

`url` 可能是**不带主机名的相对路径**（如 `/v1/media/qqbot%3A123/abc/0?exp=…&sig=…`），
客户端按当前连接的主机拼接；HMAC 签名、**10 分钟有效**。调用方先看 `size`/`mime`
再决定是否下载（D10）。失败码见 §8。

### 7.5 群成员管理（QQ 群）

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `member.mute` | M→S | 是 | `{bot_id, peer_id, member_id, seconds}`；`0` = 解除，上限 30 天 |
| `member.kick` | M→S | 是 | `{bot_id, peer_id, member_id, blacklist?}`，`blacklist=true` 同时拉黑 |

两条都要求 Bot 有对应 `caps`（`member.mute` / `member.kick`），否则回 `unsupported_capability`；
权限不足时回 `forbidden` 并附平台原因（踢人接口通常需要白名单/内邀）。

### 7.6 设备与审计

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `device.list` | M→S | 是 | `{}` → `{devices:[{device_id, name, created_at, last_seen_at}]}` |
| `device.revoke` | M→S | 是 | `{device_id}`；撤销后该设备立即不可认证 |
| `audit.list` | M→S | 是 | `{limit, before_ts, before_id}` → `{entries[], has_more}`，新的在前 |
| `event.audit` | S→M | — | 一条安全事件，字段同 `audit.list` 的 entry |

`audit` entry：`{ts, kind, device_id?, name?, ip?, reason?}`，覆盖配对、认证失败、
Basic 认证失败、设备撤销等事件；客户端「设置 → 安全日志」读它。

### 7.7 控制

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `event.peer` | S→M | — | `{bot_id, peer_id, title?, hint?}`，群名变化或需要白名单授权 |
| `error` | 双向 | — | `{code, message, retryable}`，见 §8 |
| `ping` / `pong` | 双向 | 是 | `{}` |

## 8. 错误码

`error.payload = {code, message, retryable}`；部分码带附加字段。

| code | 含义 | 可重试 |
| --- | --- | --- |
| `unauthorized` | 未认证 / 公钥未授权 / 认证前发了别的请求 | 否 |
| `forbidden` | 无该 Bot 权限，或平台权限不足 | 否 |
| `not_found` | Bot / 消息 / 设备不存在 | 否 |
| `invalid_payload` | 字段缺失、类型错误或 ID 不合法 | 否 |
| `invalid_doc` | `doc` 结构不合法（层级、未知必需字段） | 否 |
| `content_too_long` | 超过 `doc` 上限或渲染后长度上限 | 否 |
| `unsupported_capability` | 平台不支持该能力（例如没有 `member.kick`） | 否 |
| `reply_window_closed` | 被动回复窗口已过（QQ 5 分钟） | 否 |
| `mention_required` | 该会话必须先 @ 机器人才可回复 | 否 |
| `unsupported_version` | 版本不兼容，附 `min`/`max` | 否 |
| `unknown_type` | 未知 `type` | 否 |
| `frame_too_large` | 单帧超过 8 MiB | 否 |
| `media_expired` | 媒体已过保留期 | 否 |
| `media_evicted` | 媒体因单 Bot 空间不足被清理 | 否 |
| `rate_limited` | 触发限流，附 `retry_after_ms` | 是 |
| `clock_skew` | 时间偏差过大 | 是 |
| `bot_start_failed` | Bot 启动失败，附 `reason` | 是 |
| `upstream_error` | 平台 API 报错，附 `platform_code` | 视情况 |
| `internal` | 服务端未预期错误 | 是 |

## 9. 保留与清理

`auth.ok.retention = {days, text_max_bytes, media_max_bytes, cleanup_interval_s, evicted}`，
`evicted` 表示是否发生过提前淘汰，客户端据此提示「部分媒体已清理」。

**文本**：保留 7 天（配置下限也是 7 天，可调大不可调小），每小时清理过期消息；
`msg.ack` 不提前删除（ack 只用于多端未读与缓存清理）。条数与文本字节上限只作保护，
触发时从最旧淘汰并记日志，**不当作承诺**。

**媒体**：保留 7 天，且单 Bot 硬上限 3 GB，**两者先到先删**。
收到即落盘 `data/media/<bot_id>/`，不再依赖平台链接；`media.fetch` 只从本地返回。

- 过 7 天 → `media_expired`；超 3 GB → 按消息时间从旧到新淘汰，回 `media_evicted`，
  该消息标记「媒体已清理」。
- 客户端对两种失败都显示占位图，但文案区分「已过期」与「空间不足被清理」，**不假装能点开**。
- 缓存主键 `(bot_id, peer_id, ts, msg_id)`，**存 `body` 全量**，不只存 plain。
- 客户端本地不设上限（用户手动清理）；重装/换设备只能走 `msg.list` 补回 7 天内未过期的部分。

## 10. 幂等、超时与重连

- 请求超时 15s；可重试的请求**必须复用原 `id`**；服务端按 `id` 去重 5 分钟。
- `msg.send` 同 id 连发 3 次只投递 1 次（这是验收项，不是尽力而为）。
- 重连退避：1s → 2s → 4s → … → 30s，±20% 抖动；连上后先 `state.snapshot` 或 `bot.list`，
  再按游标补拉，不重复、不丢。
- 关闭码：`1000` 正常、`4001` 认证失败（**不重连**）、`4002` 版本不兼容（**不重连**）、
  `4003` 服务端过载（重连）。

## 11. 公网暴露与安全边界

**可选第二道门：HTTP Basic 认证**。`LIANT_BASIC_AUTH=user:pass`
（或 `config set auth.user / auth.pass`），覆盖 `/v1/ws` 握手、`/v1/media/` 与根路径；
用户名密码都用定长比较，同一 IP 每分钟 10 次失败后返回 429，失败事件写安全日志
（客户端「设置 → 安全日志」能看到来源 IP）。它挡的是「知道地址就来扫」的流量，
**不能替代设备密钥**：拿不到私钥仍然连不上。

两个限制：

- **当前 Flutter 客户端在 WebSocket 握手时不带 `Authorization` 头**。浏览器如果缓存了
  该源的 Basic 凭据，握手可能自动带上；Android/桌面端不会。启用前请确认客户端形态，
  或把访问控制放在反向代理层。
- `auto_pair` 只在启用 Basic 认证时可用，且只对已在 HTTP 层通过认证的连接开放；
  普通部署请用配对码。

**反代模式**：`LIANT_NO_TLS=1` 由 Caddy/Nginx 终止 TLS，**代理到本进程这一段是明文**，
只能走回环或容器内网，不要暴露到公网。

**部署侧**：配置、密钥、数据库都在 `data/`；用独立用户运行，systemd 只放开数据目录可写。
根路径与媒体响应带 CSP、HSTS（TLS 模式）、`X-Content-Type-Options`、`Referrer-Policy`、
`frame-ancestors 'none'`；服务端不再提供任何 HTML 页面。

**必须有的测试**（每条对应一个用例）：

1. 客户端钉指纹，不匹配即断开；**不提供「忽略」开关**。
2. 配对必须服务端终端确认，光有二维码不能绑定。
3. `nonce` 重放必败；`ts` 偏差 >60s 拒绝。
4. 凭据不出现在 URL / query / 日志。
5. 日志不含平台凭据、私钥、消息正文；错误不含内部路径。
6. 撤销设备后立即不可认证。
7. `msg.send` 同 id 连发 3 次只投递 1 次。
8. 服务重启后无残留会话（v1 无 session）。
9. 证书有效期 ≥ 1 年且支持原地轮换（同一私钥重签，指纹不变）。
10. 凭据只存加密 JSON，只在 `bot.add` 时入站一次，读取接口不回显明文。
11. 格式注入：渲染器先转义再拼标记；`<b>`、`**`、`\`、超长嵌套、代理对做金样本。
12. `doc` 资源上限在解析前检查（大小/块数/span 数），防内存放大。
13. 客户端渲染走 `AnnotatedString`，不拼 HTML/WebView；未知 span 只显示文本。
14. QQ 回调签名（若用 webhook）必须校验，失败直接 401，不做「尽力解析」。

**已知取舍**：客户端首次连接要信任自签证书（或用反代拿正式证书）；
客户端私钥没有硬件保护；吊销设备只挡住后续认证，已下载到本地的消息不会被远程清除。

## 12. 版本演进

- **只加字段不升版本**：新增可选字段、新增 `type`、新增 `caps` 项都不改 `v`。
- 删字段、改语义、改签名编码才升 `v`；不兼容时回 `unsupported_version` 并用 4002 关闭，
  客户端不要重连。
- v1.1 计划：媒体分片上传（>5 MiB）、多 Manager 未读聚合、`caps` 由适配器导出、
  `doc` 行内类型扩展（mention / emoji / 下标）。

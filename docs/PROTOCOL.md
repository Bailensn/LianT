# 协议 v1

Manager（Flutter 客户端）与 Service（Go 服务端）之间的唯一边界。改动这里等于改两端，
所以字段、错误码、签名编码都以本文为准；实现偏离时先改本文再改代码。

## 1. 传输

- **WSS**，路径 `/v1/ws`，端口固定（默认 `8443`，可配置）。
- 文本帧，UTF-8 JSON。单帧上限 **8 MiB**（`protocol.MaxFrame`），超限断开并回 `frame_too_large`。
- 服务端证书由节点密钥自签。Android/桌面端**允许自签**——TLS 只做传输加密，
  身份由 §4 的应用层自证把关；浏览器绕不过证书校验，生产环境需正规证书或反代。
- 心跳：每 30s 服务端发控制帧 ping，另有 `ping`/`pong` 消息类型；认证后 5 分钟收不到任何帧即断开。

## 2. 身份与命名空间

| 对象 | 密钥 | 私钥位置 | 指纹 |
| --- | --- | --- | --- |
| Service（节点） | ECDSA P-256 | `data/node/`（0600） | `base32(SHA-256(公钥 DER))`，每 4 位分组 |
| Manager（设备） | ECDSA P-256 | 客户端本地 KV（**未接 Keystore/WebCrypto**） | 同上，服务端只存公钥 |

换节点密钥等于换身份，所有设备要重新配对。

**ID 命名空间** `"<platform>:<external_id>"`：

- `platform` 匹配 `^[a-z][a-z0-9_]{1,15}$`，v1 只有 `qqbot`。
- `external_id` 匹配 `^[A-Za-z0-9_-]{1,64}$`。
- **`peer_id` 只在单个 `bot_id` 内唯一**（QQ 的 openid 按 App 隔离），
  所以会话键永远是 `(bot_id, peer_id)`，每条消息都必须带 `bot_id`。
- v1 没有 session 票据：每次连接都挑战应答，撤销设备 = 服务端删公钥。

## 3. 配对（一次性）

1. 服务端 `LianT pair` 输出 6 位配对码与二维码：
   `{"v":1,"host":"…","port":8443,"fp":"ABCD-EFGH-…","nonce":"…","exp":<unix>}`
2. Manager 连接 `wss://host:port/v1/ws`，发
   `pair.request {ticket, manager_name, manager_pub, sig(nonce)}`。
3. 服务端**在终端要求本地确认**（显示名称与指纹前 8 位），二维码单独泄漏不足以绑定。
4. 确认 → `pair.ok {service_id, service_name, devices_count, device_id}`；拒绝 → `pair.denied {reason}`。
   `device_id` 是之后 `auth.response.manager_id` 要用的值。
5. 配对码一次性、60 秒过期，设备上限默认 8 台。

配置了 Basic 认证时 `auth.challenge.auto_pair=true`：客户端可省掉 `ticket` 直接登记，
该路径以已通过的 HTTP 认证为准，不要求 nonce 签名、不消费配对码。

## 4. 握手（每次连接）

```text
Manager                          Service
   |── WS Upgrade ───────────────▶|
   |◀─ auth.challenge ─────────────|  {nonce, ts, service_pub, service_sig}
   |── auth.response ─────────────▶|  {manager_id, sig}
   |◀─ auth.ok ────────────────────|  {version, service_time, caps, retention}
```

- **待签字节固定**：`nonce + "\n" + fp + "\n" + 十进制 ts`（UTF-8），
  ECDSA P-256 + SHA-256，签名输出 ASN.1 DER 后 base64；`fp` 与二维码里的一致。
- **服务端自证**：`auth.challenge` 带 `service_pub`（base64 PKIX DER）与
  `service_sig = sign(nonce, fp, ts)`。客户端先算公钥指纹与配对时钉住的值比对，再验签，
  不一致直接断开——把证书钉扎补在应用层，反代、企业根证书、被攻陷的 CA 都冒充不了。
  老客户端忽略这两个字段。
- `ts` 偏差超过 60s 拒绝（`clock_skew`）；`nonce` 一次性，重放必败。
- 认证失败统一 `auth_failed`，同一 IP 每分钟最多 5 次失败；单连接最多 3 次尝试，超限关闭码 4001。
- 认证前只接受 `pair.request` / `auth.response` / `ping`。
- `auth.ok.caps`：`["msg.list","bot.control","media.recv","rich_text","multi_bot"]`。
  `multi_bot` 表示一条连接承载所有 Bot；**「当前活跃 Bot」是客户端本地状态**，服务端不感知，切号不重连。

## 5. 信封与 Body

```json
{"v":1,"id":"b3f1c0a2-…","type":"msg.send","ts":1793520000123,"reply_to":"b3f1c0a2-…","payload":{}}
```

- `id` 发送方生成、全局唯一；响应必须带 `reply_to`。`ts` 毫秒，仅排序与调试。
- 未知 `type` → `unknown_type`；未知字段忽略。

文本内容一律走 `body`：

```json
{"plain":"see this docs","doc":{"v":1,"blocks":[
  {"type":"p","spans":[{"t":"text","text":"see "},{"t":"bold","text":"this"},
                       {"t":"link","text":"docs","url":"https://example.com"}]}]}}
```

- `plain` 与 `doc` 至少有一个；`plain` 用于通知、搜索与降级。
- 上限：`doc` ≤ 32 KiB、≤ 200 块、每块 ≤ 20 span、每块 ≤ 10000 字符、`url` ≤ 2048；超限 `content_too_long`。
- 块：`p`、`code`（可带 `lang`）、`quote`；行内：`text`、`bold`、`italic`、`strike`、`code`、`link`。
- span 不嵌套；未知块按 `p`，未知 span 只取文本（降级不报错）。
- QQ 的 markdown 模板与 keyboard 属于适配器内部，**不进 `doc`**；v1 出站降级为纯文本。

## 6. 方法与事件

| type | 方向 | 幂等 | payload |
| --- | --- | --- | --- |
| `pair.request` / `pair.ok` / `pair.denied` | 双向 | 否 | 见 §3 |
| `auth.challenge` / `auth.response` / `auth.ok` | 双向 | 是 | 见 §4 |
| `state.snapshot` | M→S | 是 | `{}` → `{bots, devices, audit, caps}`，首屏一次拿全 |
| `bot.list` | M→S | 是 | `{}` → `{bots:[…]}` |
| `bot.add` | M→S | 是 | `{platform, credentials, label?}` → `{bot_id}`；凭据形状由平台定义 |
| `bot.remove` / `bot.start` / `bot.stop` | M→S | 是 | `{bot_id}` |
| `bot.status` | M→S | 是 | `{bot_id}` → `{status, pid?, since?, last_error?}` |
| `msg.list` | M→S | 是 | `{bot_id, peer_id, before?\|after?, limit≤100}` → `{messages[], has_more}` |
| `msg.send` | M→S | **是** | 见下 → `{msg_id, ts}` |
| `msg.ack` | M→S | 是 | `{msg_id}`，客户端已落库 |
| `media.fetch` | M→S | 是 | `{bot_id, msg_id}` → `{url, exp, size, mime}` |
| `member.mute` | M→S | 是 | `{bot_id, peer_id, member_id, seconds}`，0 = 解除，上限 30 天 |
| `member.kick` | M→S | 是 | `{bot_id, peer_id, member_id, blacklist?}` |
| `device.list` | M→S | 是 | `{}` → `{devices:[{device_id, name, created_at, last_seen_at}]}` |
| `device.revoke` | M→S | 是 | `{device_id}`，撤销后立即不可认证 |
| `audit.list` | M→S | 是 | `{limit, before_ts, before_id}` → `{entries[], has_more}`，新的在前 |
| `msg.inbound` | S→M | — | 同 `event.msg` |
| `event.msg` | S→M | — | 见下；**所有 Bot 都推**，响不响由客户端按当前账号决定 |
| `event.bot` | S→M | — | `{bot_id, status, reason?}` |
| `event.peer` | S→M | — | `{bot_id, peer_id, title?, hint?}`，群名变化或需要白名单授权 |
| `event.audit` | S→M | — | 一条安全事件（同 `audit.list` 的 entry） |
| `error` | 双向 | — | `{code, message, retryable}`，见 §7 |
| `ping` / `pong` | 双向 | 是 | `{}` |

**`msg.send`**：`{bot_id, peer_id, body, media?, in_reply_to?, quote_ref?, active?, operator_id?}`

- `in_reply_to` 是触发本次回复的入站 `msg_id`（QQ 被动回复必须带）；缺席则按主动消息处理。
- `active:true` 表示不绑定触发消息的主动消息，是否允许由 `constraints.can_initiate` 决定。
- `quote_ref` 是 QQ 引用所需的 `ref_idx`，必须配合 `in_reply_to`。
- `media = {kind, name, data}`：`kind ∈ image/video/audio/file`，`data` 为 ≤5 MiB 的标准 base64。
- `operator_id` 只用于审计，**永不下发平台**。

**`msg.inbound` / `event.msg`**：`{bot_id, peer_id, peer_kind, sender_id?, sender_name?, sender_role?,
msg_type, body, msg_id, ts, in_reply_to?, quote_ref?, direction?}`

- `peer_kind ∈ private/group/channel`；`sender_role ∈ owner/admin/member`（QQ 需成员信息权限）。
- `msg_type ∈ text/image/video/audio/file`；媒体消息的 `body.plain` 是说明文字或文件名。

**Bot 对象**：

```json
{"bot_id":"qqbot:1234567890","platform":"qqbot","username":"example_bot",
 "display_name":"Example","status":"running",
 "caps":["text","photo","markdown_template"],
 "limits":{"max_text_len":1000,"rich_text":true},
 "constraints":{"can_initiate":false,"reply_window_s":300,
                "max_replies_per_inbound":5,"requires_mention":true},
 "since":1793520000}
```

- Manager **只按 `caps`/`limits`/`constraints` 渲染**，不得内置平台假设；`caps` 是开放集合，未知项忽略。
- `limits.max_text_len` 指**渲染后**字符上限；`constraints` 描述「最严格限制」，供 UI 倒计时与禁用态。

## 7. 错误码

| code | 含义 | 可重试 |
| --- | --- | --- |
| `unauthorized` | 未认证 / 公钥未授权 | 否 |
| `forbidden` | 无该 Bot 权限 | 否 |
| `not_found` | Bot / 消息不存在 | 否 |
| `invalid_payload` | 字段缺失、类型错误或 ID 不合法 | 否 |
| `invalid_doc` | `doc` 结构不合法 | 否 |
| `content_too_long` | 超过 `doc` 或渲染后长度上限 | 否 |
| `unsupported_capability` | 平台不支持该能力 | 否 |
| `reply_window_closed` | 被动回复窗口已过 | 否 |
| `mention_required` | 该会话必须先 @ 机器人才可回复 | 否 |
| `unsupported_version` | 版本不兼容（附 `min`/`max`） | 否 |
| `unknown_type` | 未知 `type` | 否 |
| `frame_too_large` | 单帧超过 8 MiB | 否 |
| `media_expired` | 媒体已过保留期 | 否 |
| `media_evicted` | 媒体因单 Bot 空间不足被清理 | 否 |
| `rate_limited` | 触发限流（附 `retry_after_ms`） | 是 |
| `clock_skew` | 时间偏差过大 | 是 |
| `bot_start_failed` | Bot 启动失败（附 `reason`） | 是 |
| `upstream_error` | 平台 API 报错（附 `platform_code`） | 视情况 |
| `internal` | 服务端未预期错误 | 是 |

## 8. 保留与清理

**文本**：保留 7 天（配置下限也是 7 天，可调大不可调小），每小时清理过期；`msg.ack` 不提前删除。
条数与文本字节上限只作保护，触发时从最旧淘汰并记日志，**不当作承诺**。

**媒体**：保留 7 天，且单 Bot 硬上限 3 GB，**两者先到先删**。
收到即落盘 `data/media/<bot_id>/`，`media.fetch` 只从本地返回，不再访问平台。

- 过 7 天 → `media_expired`；超 3 GB → 按消息时间从旧到新淘汰，回 `media_evicted`，消息标记「媒体已清理」。
- 下载链接 `{base}/v1/media/{bot}/{msg}/{idx}?exp=&sig=`，HMAC 签名，**10 分钟有效**；
  `.url` 可能是无主机名的相对路径，客户端按当前连接拼接。
- 客户端对两种失败都显示占位图，但文案区分「已过期」与「空间不足被清理」，不假装能点开。

**下发与排序**：`auth.ok.retention = {days, text_max_bytes, media_max_bytes, cleanup_interval_s, evicted}`；
游标 `{ts, msg_id}`，服务端按 `(ts, msg_id)` 稳定排序；缓存主键 `(bot_id, peer_id, ts, msg_id)`，存 `body` 全量。

## 9. 超时、幂等与重连

- 请求超时 15s；可重试，**必须复用原 `id`**；服务端按 `id` 去重 5 分钟（窗口 `dedupeTTL`）。
- 重连退避：1s → 2s → 4s → … → 30s，±20% 抖动；连上后先 `bot.list`（或 `state.snapshot`），再按游标补拉。
- 关闭码：`1000` 正常、`4001` 认证失败（不重连）、`4002` 版本不兼容（不重连）、`4003` 服务端过载（重连）。

## 10. 公网暴露与安全边界

可以在设备密钥之外再加一道 HTTP Basic 认证：`LIANT_BASIC_AUTH=user:pass`
（或 `config set auth.user / auth.pass`），覆盖 `/v1/ws`、`/v1/media/` 与根路径；
用户名密码定长比较，同一 IP 每分钟 10 次失败后 429，失败写安全日志（客户端可见来源 IP）。

两个限制：

- **当前 Flutter 客户端在 WebSocket 握手时不带 `Authorization` 头**；浏览器可能因缓存的
  同源凭据自动带上，Android/桌面端不会。启用前请确认客户端形态，或把访问控制放到反代层。
- `auto_pair` 只在启用 Basic 认证时可用，且只对已在 HTTP 层通过认证的连接开放。

反代模式（`LIANT_NO_TLS=1`）由 Caddy/Nginx 终止 TLS，**代理到本进程这一段是明文**，
只能走回环或容器内网。配置、密钥、数据库都在 `data/`，用独立用户运行，
systemd 只放开数据目录可写。根路径与媒体响应带 CSP、HSTS（TLS 模式）、
`X-Content-Type-Options`、`Referrer-Policy`、`frame-ancestors 'none'`；服务端不再提供 HTML 页面。

**每条都要有测试**：指纹不匹配即断开且无「忽略」开关；配对必须终端确认；nonce 重放必败；
凭据不出现在 URL/query/日志；日志不含凭据、私钥、消息正文；撤销设备后立即不可认证；
`msg.send` 同 id 连发 3 次只投递 1 次；重启后无残留会话；证书有效期 ≥ 1 年可原地轮换；
凭据只存加密 JSON 且读取接口不回显；渲染先转义再拼标记（`<b>`、`**`、`\`、超长嵌套、代理对做金样本）；
`doc` 解析前检查大小/块数/span 数；客户端把 `doc` 映射到 `AnnotatedString`，**不拼 HTML/WebView**；
QQ 回调签名校验失败直接 401，不做「尽力解析」。

**已知取舍**：客户端首次连接需信任自签证书（或用反代拿正式证书）；客户端私钥无硬件保护，
能读到应用数据的人就能拿到私钥；吊销设备只挡住后续认证，已下载到本地的消息不会被远程清除。

## 11. 留给 v1.1

Manager 发送媒体已支持 ≤5 MiB 直传，更大的文件要分片上传；多 Manager 未读聚合（v1 只做单端 ack）；
`caps` 由适配器导出、`bot.list` 透传，核心不维护列表；`doc` 行内类型是否扩（mention / emoji / 下标）。

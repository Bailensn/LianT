# LianT 协议 v1（草案 · 已按决策更新）

> 状态：**草案，未实现**。已按 `docs/DESIGN.md` 的 D1–D9 更新（2026-11-07），首发平台为 QQ。
> 目标：契约先于 UI 冻结，使后续任何客户端、任何平台/格式适配器都只是消费者。

## 1. 传输

- **WSS**（TLS 1.3 起），路径 `/v1/ws`，**固定端口**（默认 `8443`，可配置）。
- 文本帧：UTF-8 JSON，单层，不嵌套 base64；二进制帧保留不使用。
- 单帧上限 1 MiB；超限断开并记 `frame_too_large`。
- 服务端证书由节点密钥签发（自签），客户端**钉指纹**；**不允许关闭校验**。

## 2. 身份与命名空间

| 对象 | 密钥 | 存储 | 指纹 |
| --- | --- | --- | --- |
| Service（节点） | ECDSA P-256 | `data/key/node.key`（0600） | `base32(SHA-256(pub))`，每 4 位分组 |
| Manager（Android 设备） | ECDSA P-256 | Android Keystore（不可导出） | 同上，用于 Service 侧设备列表 |

> D4 原写 ed25519。因 Android Keystore 对 ed25519 的支持依系统版本与厂商而异，v1 统一用 **P-256**；指纹与配对流程不变。

**ID 命名空间**：`"<platform>:<external_id>"`。

- `platform`：`^[a-z][a-z0-9_]{1,15}$`，v1 只有 `qqbot`（`telegram` 为 v1.1）。
- `external_id`：`^[A-Za-z0-9_-]{1,64}$`（QQ 的 openid / group_openid 都是字符串，正好按字符串处理）。
- **`peer_id` 只在单个 `bot_id` 内唯一**：QQ 的 openid 按 App 隔离，同一个人在不同 Bot 下 ID 不同。会话键永远是 `(bot_id, peer_id)`，消息必须带 `bot_id`。
- **v1 不使用 session 票据**：每次连接挑战应答，私钥不出 Keystore。撤销 = Service 删除该 Manager 公钥。

## 3. 配对（首次）

1. Service 本地 `service pair`，输出 6 位配对码 + 二维码：
   `{"v":1,"host":"…","port":8443,"fp":"ABCD-EFGH-…","nonce":"…","exp":<unix>}`
2. Manager 扫码，连接 `wss://host:port/v1/ws`。
3. `pair.request`：`{ticket, manager_name, manager_pub, sig(nonce)}`。
4. Service **终端要求本地确认**（显示名称与指纹前 8 位）——二维码单独泄漏不足以绑定。
5. 确认 → `pair.ok {service_id, service_name, devices_count, device_id}`；拒绝 → `pair.denied {reason}`。
   `device_id` 是服务端生成的设备标识，之后的 `auth.response.manager_id` 用它。
6. 配对码一次性、60 秒过期；默认最多 8 台 Manager。

## 4. 握手（每次连接）

```text
Manager                          Service
   |── WS Upgrade ────────────────▶|   TLS 证书指纹必须匹配
   |◀─ auth.challenge {nonce, ts} ─|
   |── auth.response ─────────────▶|   {manager_id, sig(nonce||fp||ts)}
   |◀─ auth.ok ────────────────────|   {service_time, caps, retention}
```

- 签名覆盖 `nonce || 服务端指纹 || ts`；`nonce` 一次性，重放必败。
- **编码固定**（v1 冻结）：待签字节为 `nonce + "\n" + fp + "\n" + 十进制 ts`（UTF-8），
  算法 ECDSA P-256 + SHA-256，签名输出 ASN.1 DER 后 base64；`fp` 与二维码里的一致。
- `ts` 偏差超过 60s 拒绝（`clock_skew`）。
- 认证失败统一 `auth_failed`；同 IP 每分钟 5 次上限。
- `auth.ok.caps`（服务端能力）：`["msg.list","bot.control","media.recv","rich_text","multi_bot"]`。
- `multi_bot` 表示该 Service 能同时下发多个 Bot 的事件；v1 必须支持（D9 依赖它）。
- **「当前活跃 Bot」是客户端本地状态**：服务端不感知、也不需要上报；切号不重连、不换 session。

## 5. 信封与 Body

```json
{"v":1,"id":"b3f1c0a2-…","type":"msg.send","ts":1793520000123,"reply_to":"b3f1c0a2-…","payload":{}}
```

- `id` 发送方生成、全局唯一；响应必须带 `reply_to`。
- `ts` 毫秒，仅排序与调试。
- 未知 `type` → `error.unknown_type`；未知字段忽略。

**Body 对象（D8）**：文本内容一律走 `body`。

```json
{
  "plain": "see this docs",
  "doc": {"v":1,"blocks":[
    {"type":"p","spans":[
      {"t":"text","text":"see "},
      {"t":"bold","text":"this"},
      {"t":"link","text":"docs","url":"https://example.com"}
    ]}
  ]}
}
```

- `plain` **必填**（通知、搜索、降级）；`doc` **可选**，不理解时退回 `plain`。
- `doc` 上限：≤ 32 KiB、≤ 200 块、每块 ≤ 20 span、`url` ≤ 2048；超限 `content_too_long`。
- 块：`p`、`code`（可带 `lang`）、`quote`；行内：`text`、`bold`、`italic`、`strike`、`code`、`link`。
- span 不嵌套；未知块按 `p`，未知 span 只取 `text`。
- **QQ 的 markdown 模板与 keyboard 不进 `doc`**：它们属于适配器内部，v1 出站降级为纯文本。

## 6. 方法与事件

| type | 方向 | 幂等 | payload | 说明 |
| --- | --- | --- | --- | --- |
| `pair.request` / `pair.ok` / `pair.denied` | 双向 | 否 | 见 §3 | 首次配对 |
| `auth.challenge` / `auth.response` / `auth.ok` | 双向 | 是 | 见 §4 | 每次连接 |
| `bot.list` | M→S | 是 | `{}` | 返回 `{bots:[…]}` |
| `bot.add` | M→S | 是 | `{platform, credentials, label?}` | 凭据形状由平台定义；`Validate` 反查 `external_id`，返回 `{bot_id}` |
| `bot.remove` | M→S | 是 | `{bot_id}` | — |
| `bot.start` / `bot.stop` | M→S | 是 | `{bot_id}` | — |
| `bot.status` | M→S | 是 | `{bot_id}` | `{status, pid?, since?, last_error?}` |
| `msg.list` | M→S | 是 | `{bot_id, peer_id, before?, after?, limit≤100}` | 会话内分页；返回 `{messages[], has_more}` |
| `msg.send` | M→S | **是** | `{bot_id, peer_id, body, in_reply_to?, operator_id?}` | `in_reply_to` 为触发本次回复的入站 `msg_id`（QQ 被动回复需要），缺席时按主动消息处理；`operator_id` 仅用于审计（哪台设备/哪个人回的），**永不下发平台** |
| `msg.inbound` | S→M | — | `{bot_id, peer_id, peer_kind, sender_id?, msg_type, body, msg_id}` | `peer_kind` ∈ `private/group/channel`；媒体消息的 `body.plain` 是说明文字 |
| `msg.ack` | M→S | 是 | `{msg_id}` | 客户端已落库；用于多端未读与缓存清理 |
| `media.fetch` | M→S | 是 | `{bot_id, msg_id}` | 返回 `{url, exp, size, mime}`；从本地缓存返回，不再访问平台。调用方据 `size`/`mime` 决定是否下载（D10） |
| `event.bot` | S→M | — | `{bot_id, status, reason?}` | 单 bot 粒度状态变化 |
| `event.msg` | S→M | — | 同 `msg.inbound` | 在线推送；**所有 Bot 都推**，通知由客户端按 D9 过滤 |
| `error` | 双向 | — | `{code, message, retryable}` | 见 §7 |
| `ping` / `pong` | 双向 | 是 | `{}` | 30s 心跳 |

**Bot 对象**：

```json
{
  "bot_id": "qqbot:1234567890",
  "platform": "qqbot",
  "username": "example_bot",
  "display_name": "Example",
  "status": "running",
  "caps": ["text", "image", "markdown_template"],
  "limits": { "max_text_len": 1000, "rich_text": true },
  "constraints": {
    "can_initiate": false,
    "reply_window_s": 300,
    "max_replies_per_inbound": 5,
    "requires_mention": true
  },
  "since": 1793520000
}
```

- Manager **只按 `caps` / `constraints` 渲染**，不得内置平台假设。
- `caps` 是开放集合；未知项忽略。
- `limits.max_text_len` 指**渲染后**的字符上限（转义会变长）。
- `constraints` 仅描述“最严格限制”，用于 UI 倒计时/禁用态，不是能力清单。

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
| `rate_limited` | 触发限流（附 `retry_after_ms`） | 是 |
| `clock_skew` | 时间偏差过大 | 是 |
| `bot_start_failed` | Bot 启动失败（附 `reason`） | 是 |
| `upstream_error` | 平台 API 报错（附 `platform_code`） | 视情况 |
| `internal` | 服务端未预期错误 | 是 |

## 8. 历史与保留（D2 = 7 天缓冲 + 本地留档）

**文本**

- 保留 **7 天**（配置下限 7 天，可调大、不可调小）；过期即删，每小时清理；`msg.ack` 不提前删除。
- 条数/文本字节上限只作保护，触发时从最旧淘汰并记日志，**不当作承诺**。

**媒体**

- 保留 **7 天**，且单 Bot 硬上限 **3 GB**——**两者先到先删**。
- 收到即下载落盘（不依赖平台链接），`media.fetch` 从本地返回；存储按 `bot_id` 计量。
- 到 7 天：随消息删除，`media.fetch` 返回 `media_expired`。
- 超 3 GB：按消息时间从旧到新淘汰，`media.fetch` 返回 `media_evicted`，该消息标记为「媒体已清理」。
- 客户端对两种失败都显示占位图，但文案区分「已过期」与「空间不足被清理」——不假装能点开。

**通用**

- `auth.ok.retention` 下发：`{days, text_max_bytes, media_max_bytes, cleanup_interval_s, evicted}`；`evicted` 表示是否发生过提前淘汰。
- 缓存主键 `(bot_id, peer_id, ts, msg_id)`；**存 `body` 全量**，不只存 plain。
- **本地不设上限**：Room 存全部消息，已下载媒体落 App 私有目录，直到用户清理（清理媒体不动文本）；重装/换设备只能走 `msg.list` 补回 7 天内未过期的部分。
- 游标 `{ts, msg_id}`，服务端按 `(ts, msg_id)` 稳定排序。

## 9. 超时、重试与重连

- 请求超时 15s；可重试，**必须复用原 `id`**；服务端按 `id` 去重 5 分钟。
- 重连：1s → 2s → 4s → … → 30s，±20% 抖动；连上后先 `bot.list`，再按游标补拉。
- 关闭码：`1000` 正常、`4001` 认证失败（不重连）、`4002` 版本不兼容（不重连）、`4003` 服务端过载（重连）。

## 10. 安全要求（每条对应测试）

1. 客户端钉指纹，不匹配即断开；**不提供“忽略”开关**。
2. 配对必须本地确认。
3. `nonce` 重放必败；`ts` 偏差 >60s 拒绝。
4. 凭据不出现在 URL / query / 日志。
5. 日志不含平台凭据、私钥、消息正文；错误不含内部路径。
6. 撤销 Manager 后立即不可认证。
7. `msg.send` 同 id 连发 3 次只投递 1 次。
8. Service 重启后无残留会话（v1 无 session）。
9. 证书有效期 ≥ 1 年且支持原地轮换（修掉 1 小时过期）。
10. 凭据只存加密 JSON，只在 `bot.add` 时入站一次，读取接口不回显明文。
11. **格式注入**：渲染器先转义再拼标记；恶意输入（`<b>`、`**`、`\`、超长嵌套、代理对）做金样本测试。
12. **doc 资源上限**：解析前检查大小/块数/span 数，防内存放大。
13. **客户端渲染**：`doc` 映射到 Compose `AnnotatedString`，**不拼 HTML/WebView**；未知 span 只显示文本。
14. QQ 回调签名（若用 webhook）必须校验，失败直接 401，不做“尽力解析”。

## 11. 仍开放

- `OPEN-P5`：v1.1 是否支持 Manager 发送媒体；`msg.send` 已预留 `file_ref`。
- `OPEN-P6`：Android 后台常驻方案（前台服务 / 厂商推送），决定“全部入库”的可靠性。
- `OPEN-P7`：多 Manager 的未读聚合（v1 只做单端 ack）。
- `OPEN-P8`：`caps` 由适配器导出、`bot.list` 透传，核心不维护列表。
- `OPEN-P9`：`doc` 行内类型是否扩（mention / emoji / 下标）；v1 冻结 6 种。

> 本地下载策略、存储可见与导入导出已按 D10 定稿，见 `docs/DESIGN.md` §5.1；它们不涉及线上协议。

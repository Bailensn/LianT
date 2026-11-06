# LianT 设计文档（v1.3 · 已决策）

> 状态：**已冻结 2026-11-07**。分叉由项目所有者拍板，记录在 §1；v1.3 追加 D9（通知策略）并把首发平台改为 QQ。
> 后续改动走「决策记录」增补，不直接改结论；实现偏离本文时，先改本文再改代码。

## 0. 一句话

LianT 让用户在自己服务器上托管多个 Bot，并在自己的设备上像用 IM 一样运营它们——运行权、数据、部署选择权都在用户手里。v1 首发 QQ，Telegram 随后。

**Bot 仍然是 Bot。** LianT 不做拟人化，也不让用户以为对面是真人；它做的是让**人**接管这个 Bot 的回复。产品卖点是「有人值守的 Bot」，不是「会聊天的假人」。

## 1. 决策记录

| ID | 决定 | 备选 | 为什么不选备选 | 接受的代价 |
| --- | --- | --- | --- | --- |
| D1 | **会话端为主**；账号感只对**运营者**：一个客户端汇总所有 Bot 的消息、入库、像聊天一样回复。**不做拟人化**——用户眼里它始终是 Bot | 控制台为主；用户视角的“像真人” | 控制台是已解决领域；让用户以为是真人在 QQ/Telegram 上都既做不到也不该做 | 必须有消息模型与历史；启停页要做但不抢主流程 |
| D2 | **Service 保留 7 天（时间口径）作为缓冲，Manager 本地留档**；文本与媒体都缓，媒体单 Bot 上限 3 GB；手机杀后台后靠这 7 天补拉 | Service 无状态；长期留存 | 无状态丢连续性；长期留存违反 P2 | 服务端要写盘、清理、备份说明；3 GB 是硬上限，超限会淘汰旧媒体 |
| D3 | **Android + Jetpack Compose 优先** | 桌面单二进制；继续 Python + QML | 桌面栈没收敛；QML 客户端是空壳 | 第 4 次 UI 重写；桌面端已删除 |
| D4 | **ECDSA P-256 节点密钥 + 指纹配对 + 固定端口** | ed25519；随机端口 + token；反代自理 | Keystore 对 ed25519 支持不稳；随机端口会漂移；反代门槛高 | 要写配对流程与本地确认 |
| D5 | **一 bot 一进程 + 启动 reconcile** | 单进程 goroutine；交给 systemd | 前者一 panic 全挂；后者失去「点一下就能管」 | 要补 PID、进程组、退避 |
| D6-1 | **协议：单层 JSON envelope** | 保留双层 base64 | 旧债进 v1 | 旧客户端不能直连 |
| D6-3 | **许可证** service = AGPL-3.0、manager = Apache-2.0 | 统一许可证 | 服务端 copyleft，客户端宽松 | README 写清边界 |
| D6-4 | **Pages 彻底删除**（已执行，连带 `deploy-on-release.yml`） | 移出仓库 | 不再维护下载站 | 官网另建 |
| D7 | **首发 QQ**，Telegram 作为 v1.1 的第二个适配器；预留 `platform.Adapter` 与命名空间 ID | 首发 Telegram；或完全不预留 | Telegram 不是当前真实场景；不预留则第二个平台动核心 | 核心多三个约束（§4.1） |
| D8 | **预留消息格式适配器**：核心持 `NDoc`，平台标记的渲染/解析由 `format` 适配器负责；v1 只实现 `qq-plain` + 解析降级 | 直接在适配器里拼字符串；或做「万能 Markdown」 | 前者无法复用/单测；后者是提前抽象 | 核心多一层 `NDoc`；Manager 必须能渲染 `doc` |
| D9 | **通知策略**：所有 Bot 的消息都入库，**只有「当前活跃 Bot」发系统通知**；活跃 = App 前台当前所在的那个账号（像 QQ 多号切换里的当前号），其余 Bot 在服务端照常挂着 | 全部通知；只收活跃 Bot | 全部通知在 3 个 Bot 时已不可用；只收活跃 Bot 会丢历史 | 需要 Android 前台服务维持连接；进程被杀期间只能靠服务端 7 天缓存补 |
| D10 | **本地媒体策略**：Wi-Fi 下自动下载图片/贴纸/语音/≤5 MB 文件；视频、文档、>5 MB 一律点开才下；本地存储可见可清理；支持导入导出 | 全部自动下载；全部点开才下 | 全部自动下载吃流量与空间；全部按需则超过 7 天窗口就永久丢图 | 需实现下载状态、存储占用页、导出格式与导入去重 |

> **D9 的账号模型（QQ 多号类比）**
>
> - Bot 在服务端常驻（D5），与 App 开不开无关；App 只是客户端，不是“登录器”。
> - App 里只有一个**当前账号**＝前台的那个 Bot，显式切换（像 QQ 切号），不是“最近打开的会话”。
> - 当前账号：收消息、发通知、可回复。其他 Bot：消息照常入库与计数、不响，切过去能看。
> - App 退到后台或锁屏时，**当前账号保持不变**（继续由它响）；只有用户显式切号才换。
> - **一条连接承载所有 Bot**（`multi_bot`），切号是本地 UI 状态：不重连、不重新配对、服务端不感知谁在前台。
> - 切号前若断过线，按游标补拉；超过 7 天保留期的消息就是没有，不假装有。
> - v1 不做：并排多窗口、每 Bot 独立登录态、按 Bot 独立免打扰。

> **D2 的 7 天缓冲（为什么按时间，不按条数）**
>
> - 手机杀后台是常态，真正的可靠性来自服务端那一份缓冲，不是客户端常驻。
> - 承诺按**时间**算：默认 7 天，**配置下限也是 7 天**（可调大、不可调小）；超过即删。
> - 条数/文本磁盘上限只是保护：触发时从最旧的淘汰并写日志，**不当作承诺**。
> - **媒体落盘**：收到即下载到 `data/media/<bot_id>/`，不再依赖平台链接；单 Bot 上限 **3 GB**，超限时按消息时间从旧到新淘汰，并把该消息标为「媒体已清理」（客户端显示占位，不假装能点开）。
> - 3 个 Bot 就是最多 9 GB，部署文档要写清磁盘规划；**备份只包含 DB 与主密钥，媒体可选**——媒体丢了只影响补看。
> - **本地不设上限**：Service 的 7 天 / 3 GB 只是“补货窗口”，消息与已下载媒体进了 App 就永久保留，和消息同一套规则；清理是用户手动动作（清理媒体缓存不动文本，或整体清空某会话）。
> - 验收：杀进程 1 小时后重开，7 天内的文本按游标全部补齐；3 GB 内的图片仍能打开。

## 2. 现状审计

基线 `a70a557` + 2026-11-07 工作区改动。**这是代码事实，不是设计意图。**

| # | 事实 | 证据 | 影响 | 处置 |
| --- | --- | --- | --- | --- |
| A1 | 每个 bot 的 WSS 只绑 `127.0.0.1`，对外只有 daemon 一个入口，按 `/<id>/` 路由 | `service/internal/supervisor/bot.go`、`service/internal/supervisor/wssproxy.go` | 攻击面最小，方向正确 | 保留 |
| A2 | 入口端口每次启动在 50000–60000 随机挑选 | `service/internal/supervisor/wssproxy.go` | 重启即失联 | 按 D4 重写 |
| A3 | 入口证书自签，`NotAfter = now + 1h`，daemon 全程复用 | `service/internal/wss/wss.go` | 运行超过 1 小时后校验必失败 | 按 D4 重写 |
| A4 | daemon 反代后端 `InsecureSkipVerify: true` | `service/internal/supervisor/wssproxy.go` | 回环内可接受，但 TLS 不构成信任边界 | 保留 + 注释 |
| A5 | 对外地址靠 `curl ifconfig.me` + IPv6 优先 | `service/internal/wss/wss.go` | 可能通告不可达地址 | 按 D4 删除 |
| A6 | daemon 启动不读数据库、不认领已有 bot 进程 | `service/internal/supervisor/daemon.go` | 重启后子进程成孤儿、代理 404 | 按 D5 重写 |
| A7 | 崩溃重启固定 3s，无退避无上限 | `service/internal/supervisor/manager.go` | 上游故障会反复打 API | 修复 |
| A8 | 配置/解密失败一律 `panic` | `service/internal/config/config.go`、`service/internal/crypto/chacha.go` | 未 init 时任何命令栈崩 | 修复 |
| A9 | token/session 走 query，`CheckOrigin` 恒真，非常量时间比较 | `service/internal/wss/wss.go` | 凭据易进日志；任意网页可连 | 按协议重写 |
| A10 | 下行是「JSON 数组套 base64 + content 再 base64」，无版本/id/ack | `service/internal/botruntime/main.go` | 第三个客户端接入即收税 | 按 D6-1 重写 |
| A11 | 消息与会话只存在内存 | `service/internal/botruntime/main.go` | 重启即失忆 | 按 D2 重写 |
| A12 | 0 个测试；`go.mod` 要求 1.27.0 而 CI 钉 1.26.5 | 仓库 | 回归不可控 | 修复 |
| A13 | Manager 长期只有空壳；旧 QML 与 Android 实现均已删除，现存仅为 Compose 骨架 | `manager/android` | 产品闭环不存在 | 按 D3 建设 |
| A14 | `pages/` 与 `deploy-on-release.yml` 已删除 | 2026-11-07 | 仓库不再含官网 | 完成 |
| A15 | `manager/desktop` 与桌面构建 workflow 已删除（2026-11-07） | 仓库 | 代码仍在 git 历史里，可随时取回 | 完成 |
| A16 | `bots` 表以 `INTEGER bot_id` 为主键，凭据是单个 `token` | `service/cmd/LianT/init.go`、`service/cmd/LianT/bot.go` | 撞键；凭据形状不同 | 按 D7 改为命名空间字符串 + 凭据 JSON |
| A17 | 入站只取 `c.Text()`，entities 全丢；出站只能纯文本 | `service/internal/botruntime/main.go` | 带格式的消息变裸文本 | 按 D8 重写 |
| A18 | 没有任何「多 Bot 收件箱」概念：一个进程一个 bot，客户端也没有汇总视图 | `service/internal/supervisor/manager.go` | D1/D9 的产品行为无处安放 | 新增（客户端 + 协议） |

## 3. 原则与非目标

**P1 默认不暴露。** 单入口、回环、最小权限；新增端口或公网面必须先写理由。

**P2 数据在用户手里。** 历史主存用户设备，且**本地不设保留上限**（清理由用户决定）；Service 只做 7 天缓冲（D2）。凭据与配置加密落盘。

**P3 契约先于实现。** 协议与状态模型先冻结，再写客户端。UI 只是协议的消费者。

**P4 差异留在适配器内。** 平台差异（传输、身份、凭据、投递约束）留在 `platform` 适配器；格式差异（标记、实体、转义）留在 `format` 适配器。核心不得出现 `qq`、`telegram`、`html` 分支。

**P5 不拟人化。** 不伪造在线状态，不隐藏平台 bot 标识，不做人格设定，不用客服话术包装自动回复。产品价值来自「人接管回复」，不是「让用户以为是真人」；所以宁可显示“只能窗口内回复”，也不假装能随时回。`typing` 类指示 v1 不发；将来若发，只在真有操作者正在输入时发。

**非目标（v1）**：不做公共 Bot 托管平台；不做 BotFather 替代品；不实现第三个平台；**不做跨平台 Message 模型**（线程、编辑、引用、权限、已读）；只做 §4.2 的最小文档；不做拟人化功能；不做通用 IM；不做插件市场；不做多 Manager 权限体系。

## 4. 适配器边界

两个缝 + 一组约束数据。验收问题：**接入 Telegram 时，需要改核心代码吗？**

### 4.1 平台适配器（D7）

1. **命名空间 ID**：`"<platform>:<external_id>"`，字符串存储；`bot_id` 与 `peer_id` 都遵守。
   - **重要（QQ 逼出来的）**：QQ 的 `openid` 按 App 隔离，同一个用户在不同 Bot 下不是同一个 ID。因此 **`peer_id` 只在单个 `bot_id` 内唯一**，所有消息必须同时带 `bot_id`，会话键是 `(bot_id, peer_id)`。
2. **接口**：

```go
package platform

type Adapter interface {
    Name() string                                          // "qqbot"
    Validate(ctx context.Context, c Credentials) (Account, error)
    Run(ctx context.Context, acc Account, h Hooks) error    // 回调 / WebSocket，事件回调
    SendText(ctx context.Context, acc Account, p Peer, text string) (MessageID, error)
    Format() string                                        // 该平台使用的格式适配器名
}

type MediaResolver interface { // 可选能力，小接口 + 类型断言
    ResolveMedia(ctx context.Context, acc Account, m Message) (url string, err error)
}
```

   - 凭据是适配器自定义 JSON：QQ 是 `{"app_id","app_secret"}`（access_token 刷新在适配器内部，不暴露给核心），Telegram 是 `{"token"}`。
3. **能力协商**：每个 Bot 带 `caps`、`limits`、`constraints`，Manager 只按数据渲染。

### 4.2 消息格式适配器（D8）

**中性文档 `NDoc`**（唯一的跨平台内容表示，刻意做小）：

```json
{"v":1,"blocks":[
  {"type":"p","spans":[
    {"t":"text","text":"see "},
    {"t":"bold","text":"this"},
    {"t":"link","text":"docs","url":"https://example.com"}
  ]},
  {"type":"code","lang":"go","text":"fmt.Println()"}
]}
```

- 块：`p`、`code`（可带 `lang`）、`quote`；行内：`text`、`bold`、`italic`、`strike`、`code`、`link`。
- **span 不嵌套**；未知块按 `p`，未知 span 只取文本（降级不报错）。

```go
package format

type Format interface {
    Name() string                                     // "qq-plain"
    Render(doc Doc) (text string, err error)
    Parse(text string, ents []Entity) (Doc, error)
}
```

- v1 只注册 `qq-plain`：QQ 的 markdown 是**模板 + 参数**，模板 ID 与 keyboard 结构属于适配器内部，**不进 NDoc**；v1 出站降级为纯文本，入站能识别的行内标记才进 doc。

### 4.3 投递约束（D9 与 QQ 逼出来的）

QQ 的“不能主动发起、只能窗口内回复”不能变成核心里的 `if platform == "qq"`，要变成数据：

```json
"constraints": {
  "can_initiate": false,
  "reply_window_s": 300,
  "max_replies_per_inbound": 5,
  "requires_mention": true
}
```

- Manager 按这组字段渲染倒计时、禁用输入框、以及“仅窗口内可回复”的提示。
- Telegram 的等价取值：`can_initiate: false`（需对方先 `/start`）、`reply_window_s: 0`（无窗口）、`requires_mention: false`。
- 语义是“最严格约束供 UI 使用”，不是“平台能力清单”——能力仍在 `caps` 里。

### 4.4 明确不做

- 不做统一 Message 模型（线程、编辑、引用、转发链、已读、权限）。
- 不做富文本编辑器：v1 Manager 只做纯文本 + 少量行内标记。
- 不做插件化格式/平台：编译期注册的固定列表。
- 不提前写 Telegram 适配器，也不把 Telegram 依赖加进 v1 的 `go.mod`。

### 4.5 第二个平台的验收清单

接入 Telegram 时，若下列任何一项需要改协议、DB 或 Manager 渲染逻辑，说明缝没留好：

1. 新增 `service/internal/platform/telegram/` 与 `service/internal/format/telegramhtml/` 并注册；
2. `caps` / `limits` / `constraints` 由适配器声明，Manager 原样消费；
3. 凭据字段由适配器定义，Manager 原样存传；
4. 协议、DB schema、Manager 的会话/消息渲染**零改动**。

### 4.6 现状到目标的最小改动

- `bots` 表：`bot_id TEXT PRIMARY KEY`、`platform TEXT NOT NULL`、`credentials BLOB`（加密 JSON）；会话键 `(bot_id, peer_id)`。
- 新增 `service/internal/platform`、`service/internal/format`（接口 + 注册表）。
- `service/internal/botruntime` 不再直接依赖 telebot；Telegram 相关代码移入 `service/internal/platform/telegram`（v1.1 再做）。
- 消息 payload 从 `content` 变为 `body{plain, doc?}`，入站消息增加 `peer_kind` 与 `sender_id`。

## 5. v1 垂直切片（验收标准）

七步全通才算 v1：

1. **配对**：Service 终端 `service pair` 出码 → Android 扫码 → 本地确认 → 显示指纹后四位。
2. **列 Bot**：一次 `bot.list` 拿到 id / platform / caps / constraints / 状态。
3. **收消息**：QQ 群里 @ 机器人 发一条 → App 5 秒内出现，且在正确的 `(bot, peer)` 会话下。
4. **本地入库**：杀进程重开，消息与格式仍在（Room）。
5. **回复**：App 发文本 → 在 `reply_window_s` 内发到群里；超时则按钮禁用并给出提示。
6. **重连与补拉**：断网 30 秒恢复 → 自动重连、不重复、不丢；杀进程 1 小时后重开 → 7 天内的消息按游标全部补齐。
7. **通知门控**：Bot A 是前台账号、Bot B 不是；两个 Bot 同时来消息 → 两条都入库并计数，只有 A 发系统通知；切到 B 后能看到那条消息，且不重连。

自动化验收：协议单测（握手、幂等、错误码、ID 校验）+ **NDoc 金样本**（同一 doc 渲染再 parse 回等值）+ 假 QQ API 的端到端测试。

### 5.1 本地媒体与导入导出（D10）

**下载策略**

- Wi-Fi：图片、贴纸、语音、≤5 MB 的文件与动画自动下载；视频、文档、>5 MB 一律不自动。
- 移动网络：一律不自动，只留占位与「点击下载」。
- 未下载态：占位 + 「等待 Wi-Fi」/「点击下载」；距 Service 过期 <48h 时，显示一行小字提示（不用红点）。
- 失败重试 3 次退避；仍失败标「下载失败，可重试」。

**存储可见**

- 设置页：总占用、按 Bot、按类型（图片 / 视频 / 文件 / 其他）。
- 操作：清理媒体（不动文本）、清空某会话、全部清空；清理后消息仍在，媒体显示占位。
- 不做“按时间自动清理本地”（D2 已定本地无上限）。

**导入导出**

- 文件：`LianT-export-<yyyymmdd>-<n>.zip`
  - `manifest.json`：`{format:1, exported_at, app_version, bots:[{bot_id, platform, display_name}], peers:[{peer_id, peer_kind, display_name}]}`
  - `messages.ndjson`：每行 `{bot_id, peer_id, msg_id, ts, sender_id, sender_name?, msg_type, body}`
  - `media/<msg_id>.<ext>`：已下载的媒体
- **不含凭据**：bot token、AppSecret、主密钥一律不进包。
- 可选口令加密（AES-GCM，密钥由口令经 Argon2id 派生）；默认不加密，导出时明确提示。
- 导出走系统文件选择器（SAF），由用户决定落点，App 不自动写公共目录；大包用后台任务 + 进度通知。
- 导入：流式读 zip，按 `msg_id` 去重（已存在则跳过），先写临时库再合并；有进度与取消。
- **导入不要求先配对**：Bot 未连接时按 `manifest.json` 里的名字显示为只读；配对后自动关联，消息不丢。
- 只恢复“曾经到达过设备”的内容；超出 7 天窗口且从未下载/导出的部分无法恢复。

## 6. 保留 / 重写 / 冻结 / 删除

**保留**：`service/internal/crypto`、单入口反代思路、CLI 四动词、按目录分许可、CI 构建矩阵。

**重写**：supervisor（D5）、协议（D6-1）、身份（D4）、配置错误处理、Manager（D3）、bot 存储与运行时（D7）、消息内容层（D8）、多 Bot 汇总（D1/D9）。

**删除**：`pages/`、`deploy-on-release.yml`、`manager/desktop/`、`build-manager-desktop.yml`（均已执行，历史可回看）。

## 7. 版本与发布

`manager-vX.Y.Z`（Android）与 `service-vA.B.C` 独立打 tag、独立 Release；协议 `v1` 只加字段不升版本；`NDoc` 自带 `v`，与协议解耦。

## 8. 后续动作

- **P0**：`service/internal/platform` 接口 + `qqbot` 适配器（token 刷新、回调签名、群/单聊/频道事件）；`service/internal/format` + `qq-plain`；Service 侧 `pair` / `auth` / `bot.list` / `msg.inbound` / `msg.send`；30 行假客户端跑通七步中的 1–6。
- **P1**：`manager/android`（Compose）多 Bot 收件箱 + 通知门控 + 前台服务；Room 存 `body`；媒体下载策略与存储占用页（D10）；导入导出（zip/ndjson）；游标 + 幂等 + 重连。
- **P2**：Telegram 适配器（v1.1，复用 telebot）；修 A2/A3/A5/A6/A7/A8；CI 加 `-race` 与协议单测。

## 9. 文档索引

- `docs/DESIGN.md`（本文）：决策、现状、原则、适配器边界、切片
- `docs/PROTOCOL.md`：v1 协议草案
- `README.md`：对外介绍 + 许可证

## 10. 仓库结构

```text
LianT/
├── docs/                 设计与协议（唯一真源）
│   ├── DESIGN.md
│   └── PROTOCOL.md
├── service/              服务端（Go，独立 module）
│   ├── cmd/LianT/        入口与 CLI（main / init / bot）
│   ├── internal/
│   │   ├── config/       配置（加密落盘）
│   │   ├── crypto/       主密钥 + HKDF 子密钥 + AEAD
│   │   ├── supervisor/   daemon、bot 进程管理、反代入口（原 service/service）
│   │   ├── wss/          WSS 传输与媒体服务
│   │   ├── botruntime/   单 bot 运行循环（P0 拆进 platform）
│   │   ├── platform/     Adapter 接口 + 注册表 + 命名空间 ID（已建；qqbot 待实现）
│   │   ├── format/       NDoc + Format 接口 + qqplain（已建）
│   │   └── store/        会话 / 消息 / 媒体（P0 创建）
│   ├── go.mod
│   └── .gitignore
├── manager/              客户端（Apache-2.0）
│   └── android/          Compose 骨架（P1，已从 Sort 拆出 UI 外壳）
└── .github/workflows/    service 构建（手动触发）
```

**结构规则**

1. 只有一个 Go module（`service/`），可执行入口只放 `cmd/`；库代码全在 `internal/`，不承诺对外 API。
2. `format` 不依赖任何内部包；`platform` 可依赖 `format`（承载入站文档），反向与对 `supervisor` 的依赖一律禁止。
3. 分层方向单向：`cmd → supervisor → {platform, format, wss, store, config, crypto}`；反向 import 一律拒绝。
4. 文档只在 `docs/`，代码注释不复制文档结论（注释解释“为什么”，文档记录“是什么”）。
5. 不保留“冻结目录”：不再维护的代码直接删，需要时从 git 历史取回。

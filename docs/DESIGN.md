# 设计

## 1. 定位

LianT 是自托管的 **Bot Engine**：让用户在自己的服务器上运行多个 Bot，并在自己的设备上像用 IM
一样运营它们——运行权、数据、部署选择权都在用户手里。v1 首发 QQ，Telegram 随后。

**Bot 仍然是 Bot。** 不做拟人化，也不让用户以为对面是真人；它做的是让**人**接管这个 Bot 的回复。
卖点是「有人值守的 Bot」，不是「会聊天的假人」。

两个交付物：

- `service/`：Go 写的服务端，Bot 在这里常驻运行，只提供 API；
- `manager/`：Flutter 客户端，**Android 是唯一发布目标**，Web 只在本地预览，不做桌面产品。

## 2. 原则与非目标

**P1 默认不暴露。** 单入口、回环、最小权限；新增端口或公网面必须先写理由。

**P2 数据在用户手里。** 历史主存用户设备，且**本地不设保留上限**（清理由用户决定）；
Service 只做 7 天缓冲（D2）。凭据与配置加密落盘。

**P3 契约先于实现。** 协议与状态模型先定，再写客户端；UI 只是协议的消费者。

**P4 差异留在适配器内。** 平台差异（传输、身份、凭据、投递约束）留在 `platform` 适配器；
格式差异（标记、实体、转义）留在 `format` 适配器。核心不得出现 `qq`、`telegram`、`html` 分支。

**P5 不拟人化。** 不伪造在线状态，不隐藏平台 bot 标识，不做人格设定，不用客服话术包装自动回复。
宁可显示「只能窗口内回复」，也不假装能随时回。`typing` 类指示 v1 不发。

**非目标（v1）**：公共 Bot 托管平台；BotFather 替代品；第三个平台；
跨平台 Message 模型（线程、编辑、引用链、权限、已读）；富文本编辑器；插件市场；
多 Manager 权限体系；拟人化功能；通用 IM。

## 3. 决策

| ID | 决定 | 备选 | 为什么不选备选 | 接受的代价 |
| --- | --- | --- | --- | --- |
| D1 | **会话端为主**；账号感只对运营者：一个客户端汇总所有 Bot 的消息、入库、像聊天一样回复。不做拟人化 | 控制台为主；用户视角的「像真人」 | 控制台是已解决领域；「像真人」在 QQ/Telegram 上都做不到也不该做 | 必须有消息模型与历史；启停页要做但不抢主流程 |
| D2 | **Service 保留 7 天（时间口径）做缓冲，Manager 本地留档**；文本与媒体都缓，媒体单 Bot ≤3 GB | Service 无状态；长期留存 | 无状态丢连续性；长期留存违反 P2 | 服务端要写盘、清理、备份说明；3 GB 是硬上限 |
| D3 | ~~Android + Jetpack Compose 优先~~ | — | 已被 D12 取代 | — |
| D4 | **ECDSA P-256 节点密钥 + 指纹配对 + 固定端口** | ed25519；随机端口 + token；反代自理 | Keystore 对 ed25519 支持依厂商而不稳；随机端口会漂移；反代门槛高 | 要写配对流程与终端本地确认 |
| D5 | 一 bot 一进程 + 启动 reconcile | 单进程 goroutine；交给 systemd | 前者一 panic 全挂；后者失去「点一下就能管」 | 要补 PID、进程组、退避 |
| D6 | **单层 JSON envelope**；全仓 AGPL-3.0；官网 Pages 已删除 | 保留双层 base64；统一许可证；留下载站 | 旧债不进 v1；服务端与客户端同仓同许可 | 旧客户端不能直连 |
| D7 | **首发 QQ**，预留 `platform.Adapter` 与命名空间 ID | 首发 Telegram；不预留 | Telegram 不是当前真实场景；不预留则第二个平台动核心 | 核心多三条约束（§4） |
| D8 | **预留格式适配器**：核心持 `NDoc`，平台标记的渲染/解析由 `format` 负责；v1 只实现 `qq-plain` | 直接在适配器里拼字符串；做「万能 Markdown」 | 前者无法复用与单测；后者是提前抽象 | 核心多一层 `NDoc`；Manager 必须能渲染 `doc` |
| D9 | **所有 Bot 的消息都入库，只有「当前活跃 Bot」发系统通知** | 全部通知；只收活跃 Bot | 全部通知在 3 个 Bot 时不可用；只收活跃 Bot 会丢历史 | 要前台服务维持连接；被杀期间靠服务端 7 天缓冲补 |
| D10 | **本地媒体策略**：Wi-Fi 下自动下载图片/贴纸/语音/≤5 MB 文件；视频、文档、>5 MB 点开才下；存储可见可清理；支持导入导出 | 全部自动下载；全部按需 | 全自动吃流量与空间；全按需超过 7 天窗口就永久丢图 | 要做下载状态、占用页、导出格式与去重 |
| D11 | **v1 的 Bot 跑在 serve 进程内（goroutine）**，D5 的进程隔离推后 | 立即做进程隔离 | 需要子进程 ↔ 父进程的事件通道，v1 先做客户端闭环 | 适配器 panic 威胁整个进程，用 recover 兜底；迁移时 Start/Stop/Reconcile 接口不变 |
| D12 | **客户端改用 Flutter**：Android 唯一目标，Web 只本地预览，服务端删除内嵌网页端 | Jetpack Compose；继续维护 React 网页端 | Compose 只覆盖 Android，网页端要另养一套；Flutter 能把协议层与预览收敛到一份实现 | 放弃 Compose 与 Keystore 直连；私钥暂存本地存储 |

## 4. 账号模型与通知（D9）

- Bot 在服务端常驻（D11），与 App 开不开无关；**App 只是客户端，不是「登录器」**。
- App 里只有一个**当前账号** = 前台的那个 Bot，显式切换（像 QQ 切号），不是「最近打开的会话」。
- 当前账号：收消息、发通知、可回复。其它 Bot：消息照常入库与计数、不响，切过去能看。
- App 退到后台或锁屏时，**当前账号保持不变**（继续由它响）；只有用户显式切号才换。
- **一条连接承载所有 Bot**（`multi_bot`）：切号是本地 UI 状态，不重连、不重新配对，
  服务端不感知谁在前台。
- 切号前若断过线，按游标补拉；超过 7 天保留期的消息就是没有，不假装有。
- v1 不做：并排多窗口、每 Bot 独立登录态、按 Bot 独立免打扰。

## 5. 数据与保留（D2 / D10）

**Service（补货窗口）**

- 文本保留 7 天，**配置下限也是 7 天**（可调大、不可调小），每小时清理。
  条数/文本字节上限只作保护，触发时从最旧淘汰并写日志，**不当作承诺**。
- 媒体同步 7 天 + 单 Bot 硬上限 3 GB，**先到先删**；收到即落盘 `data/media/<bot_id>/`，
  不再依赖平台链接。超限时按消息时间从旧到新淘汰，并把该消息标为「媒体已清理」。
- 3 个 Bot 最多 9 GB，部署要按这个规划磁盘；备份只含 DB 与主密钥，媒体可选
  （丢了只影响补看）。细节见 [PROTOCOL.md](PROTOCOL.md) §9。

**Manager（主存）**

- 消息与已下载媒体**本地不设上限**，清理是用户动作：清媒体不动文本，或整体清空某会话。
  当前用 Hive 持久化聊天记录、备注、已读与免打扰，已去掉早期 KV 方案的 2000 条上限。
- 会话键 `(bot_id, peer_id)`；`peer_id` 只在单 Bot 内唯一（QQ openid 按 App 隔离）。
- 重装或换设备只能走 `msg.list` 补回 7 天内未过期的部分——这一点要在空态里说清，不假装能恢复。

**媒体下载（D10）**

- Wi-Fi：图片、贴纸、语音、≤5 MB 的文件与动画自动下载；视频、文档、>5 MB 一律不自动。
- 移动网络：一律不自动，只留占位与「点击下载」。
- 未下载态：占位 + 「等待 Wi-Fi」/「点击下载」；距 Service 过期 <48h 时显示一行小字提示。
- 失败重试 3 次退避；仍失败标「下载失败，可重试」。

**导入导出（规格已定，尚未实现）**

- 包：`LianT-export-<yyyymmdd>-<n>.zip`，内含 `manifest.json`
  `{format:1, exported_at, app_version, bots:[…], peers:[…]}`、
  `messages.ndjson`（每行一条 `{bot_id, peer_id, msg_id, ts, sender_id, sender_name?, msg_type, body}`）、
  `media/<msg_id>.<ext>`。
- **不含凭据**：bot token、AppSecret、主密钥一律不进包。
- 可选口令加密（AES-GCM + Argon2id 派生），默认不加密，导出时明确提示。
- 走系统文件选择器（SAF），由用户决定落点；大包用后台任务 + 进度通知。
- 导入：流式读 zip，按 `msg_id` 去重，先写临时库再合并，有进度与取消；
  **不要求先配对**，未连接的 Bot 按 `manifest.json` 的名字显示为只读，配对后自动关联。

## 6. 适配器边界

验收问题：**接入 Telegram 时，需要改核心代码吗？**

### 6.1 平台适配器（D7）

命名空间 ID `"<platform>:<external_id>"`，字符串存储，`bot_id` 与 `peer_id` 都遵守。
**`peer_id` 只在单个 `bot_id` 内唯一**（QQ 的 openid 按 App 隔离），所以会话键是 `(bot_id, peer_id)`。

```go
package platform

type Adapter interface {
    Name() string                                          // "qqbot"
    Validate(ctx context.Context, c Credentials) (Account, error)
    Run(ctx context.Context, acc Account, h Hooks) error   // 回调 / WebSocket 事件
    SendText(ctx context.Context, acc Account, c Credentials, req SendRequest) (MessageID, error)
    Format() string                                        // 该平台使用的格式适配器名
}

type MediaResolver interface { // 可选能力：小接口 + 类型断言
    ResolveMedia(ctx context.Context, acc Account, m Message) (url string, err error)
}
```

- 凭据是适配器自定义 JSON：QQ 是 `{app_id, app_secret}`（access_token 刷新在适配器内部，
  不暴露给核心），Telegram 是 `{token}`。
- 每个 Bot 带 `caps`、`limits`、`constraints`，Manager 只按数据渲染。

### 6.2 格式适配器（D8）

**中性文档 `NDoc`**（唯一的跨平台内容表示，刻意做小）：

```json
{"v":1,"blocks":[
  {"type":"p","spans":[{"t":"text","text":"see "},{"t":"bold","text":"this"},
                       {"t":"link","text":"docs","url":"https://example.com"}]},
  {"type":"code","lang":"go","text":"fmt.Println()"}
]}
```

- 块：`p`、`code`（可带 `lang`）、`quote`；行内：`text`、`bold`、`italic`、`strike`、`code`、`link`。
- **span 不嵌套**；未知块按 `p`，未知 span 只取文本（降级不报错）。
- 上限与校验见 [PROTOCOL.md](PROTOCOL.md) §6。

```go
package format

type Format interface {
    Name() string                                     // "qq-plain"
    Render(doc Doc) (text string, err error)
    Parse(text string, ents []Entity) (Doc, error)
}
```

- v1 只注册 `qq-plain`：QQ 的 markdown 是**模板 + 参数**，模板 ID 与 keyboard 结构属于适配器内部，
  **不进 NDoc**；v1 出站降级为纯文本，入站能识别的行内标记才进 doc。

### 6.3 投递约束是数据不是分支（D9 + QQ 逼出来的）

QQ 的「不能主动发起、只能窗口内回复」不能变成核心里的 `if platform == "qq"`：

```json
"constraints": {
  "can_initiate": false,
  "reply_window_s": 300,
  "max_replies_per_inbound": 5,
  "requires_mention": true
}
```

- Manager 按这组字段渲染倒计时、禁用输入框、以及「仅窗口内可回复」的提示。
- Telegram 的等价取值：`can_initiate: false`（需对方先 `/start`）、`reply_window_s: 0`、
  `requires_mention: false`。
- 语义是「最严格约束供 UI 使用」，不是能力清单——能力在 `caps` 里。

### 6.4 第二个平台的验收清单

接入 Telegram 时，若下列任何一项需要改协议、DB 或 Manager 渲染逻辑，说明缝没留好：

1. 只新增 `service/internal/platform/telegram/` 与 `service/internal/format/telegramhtml/` 并注册；
2. `caps` / `limits` / `constraints` 由适配器声明，Manager 原样消费；
3. 凭据字段由适配器定义，Manager 原样存传；
4. 协议、DB schema、Manager 的会话/消息渲染**零改动**。

### 6.5 明确不做

统一 Message 模型（线程、编辑、引用链、转发、已读、权限）；富文本编辑器（v1 只做纯文本 +
少量行内标记）；插件化格式/平台（编译期注册的固定列表）；提前写 Telegram 适配器，
或把 Telegram 依赖加进 v1 的 `go.mod`。

## 7. 客户端进程架构

Android 上「App 被杀也要能收通知」逼出了双进程结构，细节与验证见
[README.md](README.md) §3，这里只记设计意图：

```
UI 进程（Flutter）                      :background 进程（MessageService）
  MainActivity                            startForeground(specialUse) + 常驻通知
  FlutterEngine(main)                     FlutterEngine(backgroundMain)
  BridgeMessageClient                     BridgeServer ── LiantClient ── WSS
        │  TCP 127.0.0.1 + 随机令牌             ▲
        └──────── JSON 行协议 ──────────────────┘
  写 liant_background_state.json          读同一文件
```

- **只有一条 WebSocket**，归后台进程所有；UI 的请求经桥接转发，事件经桥接实时回传。
- **通知只响一次**：两边用同一个 `notificationIdFor(msgId)`（FNV-1a，避开常驻通知的 1001），
  并带 `setOnlyAlertOnce(true)`；UI 在前台时后台不弹。
- **跨进程状态走文件**：`shared_preferences` 每进程一份内存缓存、`reload()` 不回读磁盘，
  所以改用 `liant_background_state.json`（先写 `.tmp` 再 rename，写操作串行化）。
- 前台服务类型 `specialUse`（`dataSync` 在 targetSdk 35+ 有每天 6 小时上限）。
- 长期方向是把网络/协议/签名/存储/通知下沉 Kotlin，Flutter 只留 UI：见
  [README.md](README.md) §4（含文件搬迁表与三步施工）。

## 8. 界面门禁

移动端惯例：留白优先、背景分层、动效克制。新增界面逐条自查。

| 条款 | 落地 |
| --- | --- |
| 顶部避让状态栏 | 页面根节点 `SafeArea` / `MediaQuery.paddingOf`；大标题页用 `SliverPersistentHeader` 吸顶 |
| 底部避让 Home Bar | 悬浮底栏距屏底 `16 + MediaQuery.padding.bottom`；输入条 `SafeArea(top:false)` |
| 软键盘不遮挡 | `Scaffold.resizeToAvoidBottomInset`；输入条贴 `Stack` 底部，弹起时整体上移 |
| 最小触控热区 44×44 | `LTokens.touchTarget = 44`；返回、备注、发送、行内操作都不小于它 |
| 相邻可点项留白 ≥8 | 列表行 `gap: 12`，底栏项之间 4–8；行高 ≥64 |
| 细分隔线 | 0.5 逻辑像素，`LColors.sep`（浅色 7% 黑、深色 10% 白） |
| 3–5 槽位底栏 | 会话 / 机器人 / 设置，悬浮玻璃胶囊，选中态品牌色 12% 底 |
| 空态与失败出口 | 每个列表都有空态文案；断线常驻说明条 + 「立即重试」，不拦住已加载内容 |
| 同心圆角 | 卡片 20、按钮 12–16、气泡 18；内层随外层收缩 |
| 单一主 CTA | 配对页只有「直接进入」是全宽主按钮，配对码是次级 |

**视觉语言**

- **背景分层**：页面 `#F2F2F7`（深色纯黑）、卡片 `#FFFFFF`（#1C1C1E）、按下态 `#E9E9EE`（#121214）。
  层级靠背景差，不靠投影；只有悬浮底栏带柔和投影。
- **强调色**：单一品牌蓝 `#2F6BFF`，只用于主按钮、选中态、未读数与链接。
- **文字层级**：34px 加粗大标题（滚动收到 20px）、17px 列表标题、16px 正文、13px 次级、
  11–12px 辅助；文字色只在 `LColors.text / text2 / text3` 三档里选。
- **玻璃**：底栏与聊天头部 `BackdropFilter(blur 24)` + 卡片色 72% 透明度，底下必须有滚动内容，
  不要铺在纯色背景上。
- **动效**：只在首屏与面板出现时用 180–220ms 过渡；不做滚动视差、不做循环动画。
- **深浅色**：只跟随系统（`ThemeMode.system`）；深色背景纯黑（OLED），卡片提亮一级。

**页面结构**

- **大标题页**：`LargeTitlePage`，34px 标题随滚动收到 20px 并吸顶，折叠后补 0.5px 底边；
  右侧只放一个轻量动作（状态徽标 / 添加 / 刷新）。
- **会话列表**：搜索框（44 高）+ 按「今天 / 昨天 / 本周 / 更早」分组；组内一张 20px 圆角卡片，
  行内是头像、标题、时间、预览、未读。
- **聊天页**：玻璃头部随滚动折叠（标题 17→15、隐藏副标题与回复窗口倒计时）；群聊在气泡上方
  显示发送者；时间间隔超过 5 分钟插一条时间分隔；气泡最多占宽 78%，出站品牌蓝、入站卡片色；
  底部输入条含模式说明。
- **机器人页**：状态徽标 + 启停按钮 + 移除；添加用底部抽屉，凭据不落客户端。
- **设置页**：服务、指纹、设备、机器人、安全日志、安全、能力、关于分组；危险操作二次确认。

**抽屉与输入**

- 抽屉用 `showLiantSheet`：拖拽条、最高 85vh、内容独立滚动、页脚固定；
  表单类不响应遮罩点击（`isDismissible: false`），只能用「取消」或返回键关闭。
- 键盘弹起时抽屉整体上移（`MediaQuery.viewInsetsOf`），输入框不会被盖住。
- 数字类输入用 `TextInputType.number`，地址用 `url`。
- 发送键在输入为空、断线或发送中时禁用，但**不禁用输入框本身**。

**与协议的关系**

- 界面只按 `caps` / `limits` / `constraints` 渲染，不内置平台假设。QQ 的 5 分钟被动回复窗口
  由 `constraints.reply_window_s` 驱动，聊天页头部显示倒计时。
- 发送策略不让用户选模式：普通消息默认主动，引用/@ 时自动被动（长按消息或点用户名）；
  过期后仍允许发送，但提示可能失败。
- 「当前活跃 Bot」是客户端本地状态，切换不重连。

**新增界面自查**

1. 顶部与底部是否避让了安全区？
2. 每个可点元素热区是否 ≥44px？相邻项是否留够间距？
3. 键盘弹起时输入框与主按钮是否仍可见？
4. 分隔线是否 0.5px 半透明？圆角是否内外同心？
5. 空列表、断网、超时是否都有明确文案与操作出口？
6. 一屏是否只有一个全宽主按钮？

**宽屏（仅 Web 预览）**：≥768px 时左侧 228px 导航、内容限宽 780、底栏隐藏。
这套布局只服务 Web 预览与横屏平板，不是桌面端产品。

**实现位置**：颜色字号圆角在 `manager/lib/ui/tokens.dart`，通用组件（大标题页、分组卡片、
玻璃底栏、抽屉、确认框）在 `widgets.dart`，时间与倒计时在 `format.dart`，
页面在 `manager/lib/pages/*.dart`。

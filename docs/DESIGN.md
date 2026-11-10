# 设计

## 1. 定位

LianT 是自托管的 **Bot Engine**：用户在自己的服务器上跑 Bot，在自己的设备上像用 IM 一样运营它们。
v1 首发 QQ，Telegram 随后。**Bot 仍然是 Bot**——卖点是「有人值守」，不是「会聊天的假人」。

两个交付物：`service/`（Go 服务端，Bot Engine）+ `manager/`（Flutter 客户端）。
Android 是唯一发布目标，Web 只在本地预览，不做桌面产品。

## 2. 原则

**P1 默认不暴露**：单入口、回环、最小权限；新增端口或公网面先写理由。
**P2 数据在用户手里**：历史主存设备，本地不设保留上限；Service 只做 7 天缓冲。
**P3 契约先于实现**：协议与状态模型先定，UI 只是消费者。
**P4 差异留在适配器内**：平台差异在 `platform`，格式差异在 `format`；核心不得出现 `qq`/`html` 分支。
**P5 不拟人化**：不伪造在线状态、不隐藏平台标识、不做人格设定；宁可显示「只能窗口内回复」，也不假装随时能回。

非目标（v1）：公共 Bot 托管平台、BotFather 替代品、第三个平台、跨平台 Message 模型
（线程/编辑/转发链/已读/权限）、富文本编辑器、插件市场、多 Manager 权限体系。

## 3. 决策

| ID | 决定 | 关键理由 / 代价 |
| --- | --- | --- |
| D1 | **会话端为主**，账号感只给运营者；一个客户端汇总所有 Bot 的消息 | 要消息模型与历史；启停页次要 |
| D2 | Service 保留 **7 天**（时间口径）做缓冲，Manager 本地留档；媒体单 Bot ≤3 GB | 手机杀后台是常态，可靠性来自服务端缓冲 |
| D3 | ~~Compose 优先~~ → 已由 D12 取代 | — |
| D4 | **ECDSA P-256 节点密钥 + 指纹配对 + 固定端口** | Keystore 对 ed25519 支持不稳；要写配对与终端确认 |
| D5 | 一 bot 一进程 + 启动 reconcile | 一 panic 不连坐；要做 PID/进程组/退避 |
| D6 | 单层 JSON envelope；全仓 AGPL-3.0；官网 Pages 已删除 | 旧客户端不能直连 |
| D7 | **首发 QQ**，`platform.Adapter` + 命名空间 ID 预留 | 核心多三条约束 |
| D8 | **`NDoc` 中性文档 + `format` 适配器**，v1 只实现 `qq-plain` | 核心多一层；Manager 必须能渲染 `doc` |
| D9 | 所有 Bot 都入库，**只有「当前活跃 Bot」发通知** | 需要前台服务维持连接；靠 7 天缓冲补 |
| D10 | 媒体：Wi-Fi 下自动下载图片/语音/≤5 MB 文件，其余点开才下；存储可见可清理；支持导入导出 | 要下载状态、占用页、导出格式与去重 |
| D11 | v1 的 Bot 跑在 serve 进程内（goroutine），进程隔离推后 | 适配器 panic 用 recover 兜底 |
| D12 | **客户端改用 Flutter**，Android 唯一目标，服务端删除内嵌网页端 | 放弃 Compose 与 Keystore 直连，私钥暂存本地 KV |

**D9 的账号模型**：Bot 在服务端常驻，与服务端无关；App 里只有一个「当前账号」，显式切换。
当前账号收消息、发通知、可回复；其他 Bot 消息照常入库计数、不响，切过去能看。
退到后台或锁屏时当前账号不变。**一条连接承载所有 Bot**，切号是本地 UI 状态：不重连、服务端不感知。

## 4. 适配器边界

验收问题：**接入 Telegram 时，需要改核心代码吗？**

- **平台适配器**：`Name() / Validate() / Run() / SendText() / Format()`，可选能力用 `MediaResolver` 小接口 + 类型断言。
  凭据是适配器自定义 JSON（QQ 是 `app_id` + `app_secret`，token 刷新在适配器内部）。
- **格式适配器**：`Render(doc) / Parse(text, entities)`。QQ 的 markdown 模板与 keyboard 属于适配器内部，
  **不进 `NDoc`**；v1 出站降级纯文本，入站能识别的行内标记才进 `doc`。
- **投递约束是数据不是分支**：`constraints{can_initiate, reply_window_s, max_replies_per_inbound, requires_mention}`
  由适配器声明，Manager 据此渲染倒计时与禁用态；能力清单在 `caps`，两者不混。
- 第二个平台验收：新增 `platform/telegram` + `format/telegramhtml` 并注册即可，
  协议、DB schema、Manager 渲染**零改动**。

## 5. 数据与保留

- Service：文本 7 天（配置下限 7 天，可调大不可调小）；媒体同步 7 天 + 单 Bot 3 GB，先到先删。
  收到即落盘 `data/media/<bot_id>/`，`media.fetch` 只从本地返回。细节见 [PROTOCOL.md](PROTOCOL.md) §8。
- Manager：消息与已下载媒体**本地不设上限**，清理是用户动作（清媒体不动文本）。
  当前用 Hive 持久化聊天记录、备注、已读与免打扰，已去掉早期 KV 方案的 2000 条上限。
- 会话键 `(bot_id, peer_id)`；`peer_id` 只在单 Bot 内唯一（QQ openid 按 App 隔离）。
- 导入导出（**未实现**，规格已定）：包名 `LianT-export-<yyyymmdd>-<n>.zip`，内含
  `manifest.json` + `messages.ndjson` + `media/<msg_id>.<ext>`，**不含凭据**；走 SAF 由用户选落点；
  导入按 `msg_id` 去重，先写临时库再合并，不要求先配对。

## 6. 客户端进程架构

D9 要求「App 被杀也能收到通知」，所以客户端是**双进程**：

- 主进程跑 UI（Flutter）。
- `:background` 进程跑前台服务（`specialUse`），**独占 WebSocket**，负责协议、重连、通知。
- 两进程用 **回环 TCP + 随机 token 的 JSON-lines 桥**通信：主进程通过桥收发消息，
  后台把事件实时转发回来；主进程在前台且桥连通时后台不再发通知（避免双通知）。
- 通知 ID 用 `msg_id` 的 FNV-1a 计算，两个进程算法一致 + `setOnlyAlertOnce`，避免重复响。
- 跨进程状态落一个 JSON 文件（`shared_preferences` 的进程内缓存不可靠，必须走文件）。
- 设计目标是最终把网络/协议/签名/存储/通知下沉到 Kotlin，Flutter 只留 UI；
  迁移路线见 [README.md](README.md) §5。

## 7. 界面门禁

移动端惯例：留白优先、背景分层、动效克制。新增界面逐条自查。

| 条款 | 落地 |
| --- | --- |
| 顶部避让状态栏 | 页面根节点 `SafeArea`；大标题页用 `SliverPersistentHeader` 吸顶 |
| 底部避让 Home Bar | 悬浮底栏 `16 + MediaQuery.padding.bottom`；输入条 `SafeArea(top:false)` |
| 键盘不遮挡 | `resizeToAvoidBottomInset`；输入条贴 `Stack` 底部，弹起时整体上移 |
| 触控热区 ≥44 | `LTokens.touchTarget`；返回/发送/行内操作都不小于它 |
| 相邻可点项留白 ≥8 | 列表行 `gap: 12`，行高 ≥64 |
| 细分隔线 | 0.5 逻辑像素，`LColors.sep`（浅 7% 黑 / 深 10% 白） |
| 圆角同心 | 卡片 20、按钮 12–16、气泡 18，内层随外层收缩 |
| 单一主 CTA | 一屏只有一个全宽主按钮 |
| 空态与失败出口 | 每个列表都有空态文案；断线说明条 + 「立即重试」，不拦住已加载内容 |

- **背景分层**：页面 `#F2F2F7`（深色纯黑）、卡片 `#FFFFFF`（#1C1C1E）、按下态 `#E9E9EE`。
  层级靠背景差，不靠投影；只有悬浮底栏带柔和阴影。
- **强调色**：单一品牌蓝 `#2F6BFF`，只用于主按钮、选中态、未读、链接。
- **文字**：34px 加粗大标题（滚动收到 20px）、17px 列表标题、16px 正文、13px 次级、11–12px 辅助；只用三档文字色。
- **玻璃**：底栏与聊天头部 `BackdropFilter(blur 24)` + 卡片色 72%，底下必须有滚动内容。
- **动效**：只在首屏与面板出现时用 180–220ms 过渡；不做滚动视差与循环动画。
- **深浅色**：只跟随系统；深色背景纯黑（OLED），卡片提亮一级。
- **页面**：会话列表按今天/昨天/本周/更早分组；聊天页玻璃头部滚动折叠、>5 分钟插时间分隔、
  气泡最宽 78%；机器人页状态徽标 + 启停 + 移除，添加走底部抽屉且凭据不落客户端；
  设置页分组，危险操作二次确认。
- **抽屉**：`showLiantSheet`，拖拽条、最高 85vh、内容独立滚动、页脚固定；表单类不响应遮罩点击。
- **发送**：默认按 `constraints.can_initiate` 决定主动/被动，会话设置可逐会话覆盖（存本地）；
  过期后仍允许发送，但提示可能失败。发送键在空/断线/发送中禁用，但不禁用输入框。
- **宽屏**（仅 Web 预览）：≥768px 左侧 228px 导航、内容限宽 780、底栏隐藏；不是桌面端产品。

实现位置：颜色字号圆角在 `manager/lib/ui/tokens.dart`，通用组件在 `widgets.dart`，
时间格式在 `format.dart`，页面在 `manager/lib/pages/*.dart`。

# 交接说明（给下一个会话）

先把这份文件读完再动手。仓库在 `/storage/emulated/0/LianT`，分支 `main`。

## 任务

把 Flutter 客户端做完：**接协议**和**重排视觉**同时进行。视觉目标见 `docs/UI-DESIGN.md`。
本轮已完成下面「待办」的 1、2；下一步是收尾（删网页端、桌面双栏、通知/媒体）。

## 当前状态

### 服务端（`service/`，Go 1.27）

已经能跑，别再大改。要点：

- 协议 v1：WSS 信封 `{v,id,type,ts,reply_to,payload}`，`/v1/ws`
- 双向签名握手：`auth.challenge` 带 `service_pub`/`service_sig`/`auto_pair`，客户端比对指纹并验签；
  `auth.response` 用 `nonce\nfp\nts` 签名
- 方法：`state.snapshot`、`bot.list|add|remove|start|stop|status`、`msg.list|send|ack`、`media.fetch`、
  `device.list|revoke`、`audit.list`；事件：`msg.inbound`/`event.msg`、`event.bot`、`event.audit`、`event.peer`
- 配对码落库（`pairing.Store`），容器/多进程下 `pair` 只生成码、由 `serve` 兑付
- **免码登记修好了**：`AutoPair` 只在配置了 Basic 认证时打开；`pair.request.ticket` 为空时
  不再要求 nonce 签名、也不消费配对码（之前这条路径必然 `invalid_sig`/`no_ticket`，网页和 App
  的「直接进入」都用不了）。有码配对流程未改。
- QQ 适配器同时接 `GROUP_AT_MESSAGE_CREATE` 与 `GROUP_MESSAGE_CREATE`；`can_initiate=true`；
  `msg.send.active=true` 表示主动消息（不带 `msg_id`，因此没有 @ 和引用）
- 群资料 `GET /v2/groups/{openid}/info`，403/404 → `event.peer {hint:"whitelist"}`

部署：二进制 + systemd，装在 `/opt/liant`，`scripts/install.sh` 从最新 pre-release 下载安装。
Docker 已全部移除，不要加回来。

### Flutter（`manager/`）

**协议与界面都已接真数据，不再有假数据占位。**

- `lib/core/p256.dart`：自己实现的 P-256（ECDSA/SHA-256、PKIX DER、base32 指纹），
  签名与验签都用 `dart:convert` 的 base64；与 openssl 双向交叉验证过（见 `tool/p256_check.dart`）。
  用 `package:crypto` 做 SHA-256，没有引 pointycastle。
- `lib/core/identity.dart`：设备身份、`host:port`/`ws://` 地址解析、配对信息解析（裸 JSON、
  base64url、`#pair=` 链接）。私钥是 P-256 标量，存本地；服务端只拿公钥。
- `lib/core/client.dart`：WSS 客户端。挑战自证校验（指纹不符/验签失败直接断开，不提供忽略开关）、
  30s 心跳、1→30s 指数退避 + ±20% 抖动、15s 请求超时、`msg.send` 幂等语义按协议走。
- `lib/core/app_state.dart`：状态层。`state.snapshot` 初始化（老服务端回退到逐个拉取）、
  四类事件、乐观发送与失败重试、`msg.list` 历史、`media.fetch`、设备/审计、
  备注/已读/主动消息开关持久化；消息最多留 2000 条（localStorage 上限保护），
  备注与已读不设上限。
- `lib/core/...` 没有任何平台插件依赖（除 shared_preferences），Android/Web 同一条代码路径；
  `browser_env*.dart` 用条件导入读网页地址栏参数。

**视觉**（`lib/ui/tokens.dart` + `widgets.dart` + 各页面）：

- 34px 大标题随滚动收缩成 20px 并吸顶，分隔线 0.5/1px 半透明；
- 会话按 今天/昨天/本周/更早 分组，组内是一张圆角卡片；
- 悬浮玻璃底栏（`BackdropFilter` 24px + `saturate` 近似），选中态是品牌色 12% 底；
- 聊天页玻璃头部随滚动折叠（标题 17→15、隐藏两行副标题、补 0.5px 底边）；
- 群聊显示发送者昵称；备注在会话设置抽屉里改，本地存储；主动/被动消息开关同处；
- 宽屏（≥768）改成左侧 228px 导航 + 限宽 780 内容；
- 每个列表都有空态、断线说明条与「立即重试」；断线不阻塞已有消息的阅读。

**本地构建**（已跑通）：

```bash
export PATH=$PREFIX/bin:$PREFIX/opt/flutter/bin:/system/bin
cd /storage/emulated/0/LianT/manager
flutter analyze
flutter build web --release               # 约 70 秒
```

**预览**：

```bash
# 1) 假服务端（协议 v1 握手/快照/历史/发送，数据只在内存）
dart run tool/mock_service.dart 8099
# 2) 静态服务
cd build/web && setsid nohup python3 -m http.server 8081 --bind 127.0.0.1 >/dev/null 2>&1 &
# 3) Chrome 打开（有 WebGL，正常渲染）：
#    http://127.0.0.1:8081/?host=ws%3A%2F%2F127.0.0.1%3A8099
#    AI 浏览器的 WebView 没有 WebGL：加 &cpu=1 走 CanvasKit CPU 渲染；
#    截图前确认侧栏是打开的（侧栏收起时 browser_screenshot 只会拿到空白）。
```

- 地址栏 `?host=…` 只预填服务地址；**自动连接**需要预览构建：
  `flutter build web --dart-define=LIANT_PREVIEW=true`，再加 `&auto=1`。
  正式构建不带这个 define，链接参数不会触发静默配对。
- `?cpu=1` 在 `web/index.html` 里设置 `window.flutterConfiguration = {renderer:'canvaskit', canvasKitForceCpuOnly:true}`，
  只对显式带参数的访问生效。
- 预览必须用 Chrome：AI 浏览器 WebView 没有 WebGL，Flutter web 白屏（当前环境如此）。
- Flutter SDK 在 `$PREFIX/opt/flutter`（`flutter-for-termux` 3.47.9 / Dart 3.13.7）。
  `dart` 不在 `$PREFIX/bin`，要把它加进 PATH。
- `pubspec.yaml` 里 `shared_preferences_foundation` 钉在 2.5.4：2.6 起依赖 `objective_c`
  的 build hook，Termux 上 hook 进程会被系统拒绝；2.5.4 在 CI 的 Flutter 3.24.5 上也能用。

**自检情况**：

- `flutter analyze` 0 issue；`flutter build web --release` 通过；
- `dart run tool/p256_check.dart` 与 `openssl` 双向验证签名/DER/指纹；
- `dart run tool/mock_smoke.dart` 跑通「挑战自证 → pair.ok → auth.ok → state.snapshot → 三类事件」；
- AI 浏览器里用 URL 预填 + `LIANT_PREVIEW` 自动配对，读到真实会话列表（今天/昨天、未读、机器人名），
  说明浏览器端协议、握手、事件、乐观发送链路是通的；
- 截图自检：本机 Chrome 渲染正常（已人工看过配对页）；`flutter test` 在本机跑不起来——
  Termux 版 `flutter_tester` 缺 `libvk_swiftshader.so`/`libfontconfig`，补上 swiftshader 包后仍段错误，
  与项目代码无关。要出 golden 请在桌面 Linux/macOS 或 CI 跑。

### CI

- `CI`：`scripts/check-protocol.sh` + `go vet` + `go test -race`
- `Service binaries`：构建 linux amd64/arm64 并发布 pre-release（`install.sh` 从这取）
- `Flutter`：官方 Flutter 3.24.5 出 web(gh-pages) + APK + Linux 桌面，路径 `manager/**`
- 推送：`git push https://x-access-token:$TOKEN@github.com/Bailensn/LianT.git HEAD:main`
  TOKEN 在 `$HOME/newtoken.txt`；这条网络链路时好时坏，用循环重试 5 次。

## 待办

1. ~~协议客户端（Dart）~~：已完成（配对/免码登记、握手、快照、事件、乐观发送、历史、重连心跳）
2. ~~视觉重排~~：已完成（大标题折叠、分组卡片、玻璃底栏、聊天页折叠头部、群聊发送者、备注）
3. Flutter 稳定后：删 `web/`，删 Go 里 `internal/webui` 嵌包与 `appHandler` 的静态路由
4. 宽屏双栏：现在 ≥768 只换成左侧导航，`UI-DESIGN.md` 里的「列表 360 + 聊天窗」还没做
5. 后台通知（Android 前台服务/厂商推送）与媒体离线缓存
6. 用真服务端 + 真 QQ 凭据走一遍配对码流程（本机只有 mock，`AutoPair` 修复也还没上真机验证）

## 已知缺口

- QQ 不给 openid 昵称/头像，只能本地备注；群名需要白名单授权（`event.peer.hint=whitelist`）
- @ 某人只能通过「回复某条消息」实现
- 后台通知 Flutter 侧未做
- 媒体只做按需拉取 + 图片渲染，未落盘缓存；失败文案区分「已过期」与「空间不足被清理」
- 本地消息留存上限 2000 条 / 约 2.5MB（localStorage 上限保护），不是协议里的「本地不设上限」
- 桌面双栏、通知、真机截图仍是缺口
- DSH 的 Linux 环境装不上（loader 资产校验写死在 APK 里），与项目无关，不要浪费时间

## 约定

- 中文交流；代码注释中文；提交信息中文
- 不留 `TODO` 占位、不留假数据上屏（mock 只在 `tool/`，不在 App 内）
- 改完必须 `flutter analyze` + `flutter build web` 通过再提交；截图自检用 Chrome 或桌面环境

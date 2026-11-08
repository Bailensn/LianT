# 交接说明（给下一个会话）

先把这份文件读完再动手。仓库在 `/storage/emulated/0/LianT`，分支 `main`。
文档索引在 `docs/README.md`。

## 任务

把 Flutter 客户端做完：**接协议**和**重排视觉**同时进行。视觉目标见 `docs/UI-DESIGN.md`。
本轮完成了协议接入、视觉重排、删除旧网页端，并把 Android 平台脚手架加入仓库；
下一步是通知与媒体缓存（Web 预览的宽屏布局可选，不是交付目标）。

## 当前状态

### 服务端（`service/`，Go 1.27）

- 协议 v1：WSS 信封 `{v,id,type,ts,reply_to,payload}`，`/v1/ws`
- 双向签名握手：`auth.challenge` 带 `service_pub`/`service_sig`/`auto_pair`，客户端比对指纹并验签；
  `auth.response` 用 `nonce\nfp\nts` 签名
- 方法：`state.snapshot`、`bot.list|add|remove|start|stop|status`、`msg.list|send|ack`、`media.fetch`、
  `device.list|revoke`、`audit.list`；事件：`msg.inbound`/`event.msg`、`event.bot`、`event.audit`、`event.peer`
- 配对码落库（`pairing.Store`），容器/多进程下 `pair` 只生成码、由 `serve` 兑付
- 免码登记：`AutoPair` 只在配置了 Basic 认证时打开；`pair.request.ticket` 为空时不再要求
  nonce 签名、也不消费配对码。注意当前 Flutter 原生端不会在 WebSocket 握手时带 `Authorization`，
  所以实际上仍以配对码为主（见 SECURITY.md）
- **不再内嵌网页管理端**：`internal/webui` 与 `web/` 已删除。根路径只返回一行纯文本说明，
  作用一是健康检查，二是让浏览器在这一层信任自签证书；`/v1/ws` 与 `/v1/media/` 保持
- `LianT pair` 打印**配对信息 JSON** 并把 JSON 本身渲染成二维码（`data/pair-qr.png`），
  不再生成 `https://…/#pair=` 管理页链接；`--host` / `--port` 可覆盖对外地址，
  公网 IPv4 会依次问 ipify / icanhazip / ifconfig.me，拿不到才退回内网地址
- 配对请求现在会在服务端日志里打出「收到 / 拒绝 / 成功」，用 `journalctl -u liant -f` 能看到
- QQ 适配器同时接 `GROUP_AT_MESSAGE_CREATE` 与 `GROUP_MESSAGE_CREATE`；`can_initiate=true`；
  `msg.send.active=true` 表示主动消息（不带 `msg_id`，因此没有 @ 和引用）
- 群资料 `GET /v2/groups/{openid}/info`，403/404 → `event.peer {hint:"whitelist"}`

部署：二进制 + systemd，装在 `/opt/liant`，`scripts/install.sh` 从最新 pre-release 下载安装
（GitHub 的 `/releases/latest` 会跳过 pre-release，脚本改用 `/releases?per_page=5`）。
Docker 已全部移除，不要加回来。

### Flutter（`manager/`）

协议与界面都已接真数据，不再有假数据占位。

- `lib/core/p256.dart`：自己实现的 P-256（ECDSA/SHA-256、PKIX DER、base32 指纹）。
  用 `package:crypto` 做 SHA-256，没有引 pointycastle；与 openssl 双向交叉验证过
  （`tool/p256_check.dart`）。
- `lib/core/identity.dart`：设备身份、`host:port`/`ws://` 地址解析、配对信息解析（裸 JSON、
  base64url、`#pair=` 链接）。
- `lib/core/client.dart`：WSS 客户端。挑战自证校验（指纹不符/验签失败直接断开）、30s 心跳、
  1→30s 指数退避 + ±20% 抖动、15s 请求超时、4001/4002 不重连。
- `lib/core/app_state.dart`：`state.snapshot`（老服务端回退逐个拉取）、四类事件、乐观发送与
  失败重试、`msg.list` 历史、`media.fetch`、设备/审计、备注/已读/主动消息持久化；
  消息最多留 2000 条，备注与已读不设上限。
- `lib/core/browser_env*.dart`：条件导入，Web 读地址栏参数，其它平台返回 null。
- `lib/core/socket*.dart`：条件导入；Android/桌面端接受自签证书（身份仍由 auth.challenge
  的节点指纹校验），Web 端交给浏览器。
- `android/` 已入库：应用 ID `com.bailensn.liant`，主清单带 `INTERNET` 权限，
  Gradle 工具链按 CI 的 Flutter 3.24.5 对齐（Gradle 8.3 / AGP 8.1.0 / Kotlin 1.8.22）。
  图标源文件是 `assets/icon.svg`（iOS squircle 蓝底 + 圆头 L）与 `assets/icon_foreground.svg`，
  各尺寸 PNG 由浏览器原生 SVG 渲染生成；含 Android 8+ 自适应图标与网页 favicon。
  **Android 是唯一交付目标**；Web 只在本地预览，不对外发布；不做桌面端。

**视觉**（`lib/ui/tokens.dart` + `widgets.dart` + 各页面）：

- 34px 大标题随滚动收缩成 20px 并吸顶，分隔线 0.5px 半透明；
- 会话按 今天/昨天/本周/更早 分组，组内一张圆角卡片；
- 悬浮玻璃底栏（`BackdropFilter` 24px 模糊），选中态品牌色 12% 底；
- 聊天页玻璃头部随滚动折叠；群聊显示发送者；聊天输入框按内容自适应
  （单行 44，多行最多 132；不要再套 `Center`，会把 `maxHeight` 吃满）；
- 备注与主动/被动开关在会话设置抽屉里，本地存储；
- 宽屏（≥768）改成左侧 228px 导航 + 限宽 780 内容；
- 每个列表都有空态、断线说明条与「立即重试」。

**本地构建**：

```bash
export PATH=$PREFIX/bin:$PREFIX/opt/flutter/bin:/system/bin
cd /storage/emulated/0/LianT/manager
flutter analyze
flutter build web --release               # 预览用，约 70 秒
flutter build apk --release               # Android 交付物；需要 Android SDK/JDK
flutter build apk --release --split-per-abi  # 拆出 arm64 / x86_64
```

`dart` 不在 `$PREFIX/bin`，要把 `$PREFIX/opt/flutter/bin` 加进 PATH。

**预览**（用完记得把后台进程关掉）：

```bash
# 1) 假服务端（协议 v1 握手/快照/历史/发送，数据只在内存）
dart run tool/mock_service.dart 8099
# 2) 静态服务
cd build/web && setsid nohup python3 -m http.server 8081 --bind 127.0.0.1 >/dev/null 2>&1 &
# 3) 浏览器打开：
#    http://127.0.0.1:8081/?host=ws%3A%2F%2F127.0.0.1%3A8099
```

- Web 构建只用于本地预览，**不发布到 gh-pages 或任何公开地址**。
- 地址栏 `?host=…` 只预填服务地址；自动连接需要预览构建
  `flutter build web --dart-define=LIANT_PREVIEW=true` 再加 `&auto=1`。
  正式构建不带这个 define，链接参数不会触发静默配对。
- AI 浏览器的 WebView 没有 WebGL：加 `&cpu=1` 走 CanvasKit CPU 渲染
  （`manager/web/index.html` 里设置 `flutterConfiguration`）；但侧栏收起时
  `browser_screenshot` 只会拿到空白，截图前确认预览是可见的。
- 预览必须用 Chrome：AI 浏览器 WebView 没有 WebGL，Flutter web 默认白屏。
- Flutter SDK 在 `$PREFIX/opt/flutter`（`flutter-for-termux` 3.47.9 / Dart 3.13.7）。
- `pubspec.yaml` 把 `shared_preferences_foundation` 钉在 2.5.4：2.6 起依赖 `objective_c`
  的 build hook，Termux 上 hook 进程会被拒绝；2.5.4 在 CI 的 Flutter 3.24.5 上也能用。

**自检情况**：

- `flutter analyze` 0 issue；`flutter build web --release` 通过；
- 一个 `CI` 工作流按变更路径分 job：`service-check`（协议对照 + vet + race）、
  `client-check`（analyze + test），push 时再分别跑 `service-release` 与 `android`；
- `dart run tool/p256_check.dart` 与 `openssl` 双向验证签名/DER/指纹；
- `dart run tool/mock_smoke.dart` 跑通「挑战自证 → pair.ok → auth.ok → snapshot →
  msg.list → msg.send → 事件」；
- AI 浏览器里用地址栏预填实际走了一遍：配对 → 会话列表 → 群聊（历史/发送者）→
  机器人 → 设置；
- 截图自检：Chrome 渲染正常；`flutter test` 在本机跑不起来——Termux 版
  `flutter_tester` 缺 `libvk_swiftshader.so`/`libfontconfig`，补齐后仍段错误，
  与项目代码无关。要出 golden 请在桌面 Linux/macOS 或 CI 跑。

### CI

- `CI`：单个工作流。`changes` 用 paths-filter 判定变更：
  - `service/**` → `service-check`，push 后 `service-release` 发 pre-release；
  - `manager/**` → `client-check`，push 后 `android` 出三个 release APK；
  - `scripts/**` 只跑 Go 检查；`docs/**` 跳过；
  - PR 只跑 check，不发版、不出包。
- 卸载：`scripts/uninstall.sh`（默认保数据，`--purge` 才删）
- 推送：`git push https://x-access-token:$TOKEN@github.com/Bailensn/LianT.git HEAD:main`
  TOKEN 在 `$HOME/newtoken.txt`；这条网络链路时好时坏，用循环重试 5 次。

## 待办

1. ~~协议客户端（Dart）~~：已完成
2. ~~视觉重排~~：已完成
3. ~~删 `web/` 与 `internal/webui`~~：已完成（含 `scripts/build-web.sh`、安装脚本与文档同步）
4. Web 预览的宽屏双栏（可选，不是交付目标）
5. 后台通知（Android 前台服务/厂商推送）与媒体离线缓存
6. 用真服务端 + 真 QQ 凭据走一遍配对码流程（本机只有 mock，`AutoPair` 修复也没上真机）
7. 本地消息换成数据库（现在 2000 条上限是为了避开 localStorage 容量）

## 已知缺口

- QQ 不给 openid 昵称/头像，只能本地备注；群名需要白名单授权（`event.peer.hint=whitelist`）
- @ 某人只能通过「回复某条消息」实现
- 后台通知 Flutter 侧未做
- 媒体只做按需拉取 + 图片渲染，未落盘缓存；失败文案区分「已过期」与「空间不足被清理」
- 本地消息留存上限 2000 条（约 2.5 MB），不是协议里的「本地不设上限」
- 客户端私钥存本地 KV，未接 Keystore/WebCrypto 不可导出密钥
- Android 还没有正式签名配置（release 包临时用 debug key 签名），正式上架需要接 signingConfig
- Android 真机还没跑过配对与收发（本机只有 Web 预览与 mock 服务）
- Basic 认证启用后，Flutter 原生端无法在 WebSocket 握手提供凭据；用反代侧访问控制或暂不开启
- Web 端连接自签证书仍会失败；需要先信任证书或反代换正式证书
- 通知、真机截图仍是缺口
- DSH 的 Linux 环境装不上（loader 资产校验写死在 APK 里），与项目无关，不要浪费时间

## 约定

- 中文交流；代码注释中文；提交信息中文
- 不留 `TODO` 占位、不留假数据上屏（mock 只在 `tool/`，不在 App 内）
- 改完必须 `flutter analyze` + `flutter build web` 通过再提交；截图自检用 Chrome 或桌面环境
- 长时间预览/构建结束后关掉后台进程，别占着端口和内存

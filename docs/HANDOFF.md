# 交接说明（给下一个会话）

先把这份文件读完再动手。仓库在 `/storage/emulated/0/LianT`，分支 `main`。

## 任务

把 Flutter 客户端做完：**接协议**和**重排视觉**同时进行。视觉目标见 `docs/UI-DESIGN.md`。

## 当前状态

### 服务端（`service/`，Go 1.27）

已经能跑，别再大改。要点：

- 协议 v1：WSS 信封 `{v,id,type,ts,reply_to,payload}`，`/v1/ws`
- 双向签名握手：`auth.challenge` 带 `service_pub`/`service_sig`/`auto_pair`，客户端比对指纹并验签；
  `auth.response` 用 `nonce\nfp\nts` 签名
- 方法：`state.snapshot`、`bot.list|add|remove|start|stop|status`、`msg.list|send|ack`、`media.fetch`、
  `device.list|revoke`、`audit.list`；事件：`msg.inbound`/`event.msg`、`event.bot`、`event.audit`、`event.peer`
- 配对码落库（`pairing.Store`），容器/多进程下 `pair` 只生成码、由 `serve` 兑付；
  有 Basic 认证时 `AutoPair=true`，网页可免码登记
- QQ 适配器同时接 `GROUP_AT_MESSAGE_CREATE` 与 `GROUP_MESSAGE_CREATE`；`can_initiate=true`；
  `msg.send.active=true` 表示主动消息（不带 `msg_id`，因此没有 @ 和引用）
- 群资料 `GET /v2/groups/{openid}/info`，403/404 → `event.peer {hint:"whitelist"}`

部署：二进制 + systemd，装在 `/opt/liant`，`scripts/install.sh` 从最新 pre-release 下载安装。
Docker 已全部移除，不要加回来。

### Flutter（`manager/`）

`lib/main.dart` 目前是**四屏假数据占位**：会话列表、聊天、机器人、设置。Material 3 默认观感。
`web/index.html` 是壳。`pubspec.yaml` 只有 flutter + cupertino_icons（`web_socket_channel`、
`shared_preferences` 曾加过又被移除，需要时再加）。

**本地构建与预览（关键，已跑通）**

```bash
export PATH=$PREFIX/bin:/system/bin
cd /storage/emulated/0/LianT/manager
flutter build web --release          # 约 60 秒
cd build/web && setsid nohup python3 -m http.server 8081 --bind 127.0.0.1 > /dev/null 2>&1 &
```

Flutter SDK 在 `$PREFIX/opt/flutter`（`flutter-for-termux` 的 3.47.9 / Dart 3.13.7）。
**解包教训**：用 `tar` 从 `~/fdext/data/data/com.termux/files/usr/opt/flutter` 复制，
不要 `cp -a`，会损坏 dart snapshot，报 `.dynamic section header was not found`。

预览必须用 **Chrome**：AI 浏览器 WebView 没有 WebGL，Flutter web 白屏。操作方式见下。

### 手机自动化

- 只有**无障碍通道**可用；Shizuku/ADB 都不可用，`android_app_launch` 会被策略拒
- 开应用：`android_ui_global(home)` → `android_ui_dump` → `android_ui_click(nx, ny)`
- 截图：`android_screenshot(screenId="real")`，自己看
- Chrome 已开在 `localhost:8081`，改完代码重建后按刷新即可

### CI

- `CI`：`scripts/check-protocol.sh` + `go vet` + `go test -race`
- `Service binaries`：构建 linux amd64/arm64 并发布 pre-release（`install.sh` 从这取）
- `Flutter`：官方 Flutter 3.24.5 出 web(gh-pages) + APK + Linux 桌面，路径 `manager/**`

推送：`git push https://x-access-token:$TOKEN@github.com/Bailensn/LianT.git HEAD:main`
TOKEN 在 `$HOME/newtoken.txt`；这条网络链路时好时坏，用循环重试 5 次。

## 待办

1. **协议客户端（Dart）**：配对（有 `auto_pair` 时免码自登记）、WSS 认证、`state.snapshot` 初始化、
   收 `msg.inbound`/`event.bot`/`event.audit`/`event.peer`、发 `msg.send`（乐观上屏 + 主动/被动开关）、
   `msg.list` 拉历史、断线重连与心跳（30s ping）
2. **视觉重排**：大标题（34px 折叠）、分组卡片、悬浮玻璃底栏、聊天页滚动折叠标题、
   群聊显示发送者、备注功能（本地存储）
3. Flutter 稳定后：删 `web/`，删 Go 里 `internal/webui` 嵌包与 `appHandler` 的静态路由

## 已知缺口

- QQ 不给 openid 昵称/头像，只能本地备注
- @ 某人只能通过"回复某条消息"实现
- 后台通知 Flutter 侧未做
- DSH 的 Linux 环境装不上（loader 资产校验写死在 APK 里），与项目无关，不要浪费时间

## 约定

- 中文交流；代码注释中文；提交信息中文
- 不留 `TODO` 占位、不留假数据上屏
- 改完必须 `flutter build web` 通过并截图自检，再提交

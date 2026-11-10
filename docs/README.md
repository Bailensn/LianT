# 文档与现状

`service/` 是 Go 服务端（Bot Engine），`manager/` 是 Flutter 客户端（只发 Android）。
本文件是接手项目的入口：现状、结构、待办、迁移路线。其余三份各管一件事：

| 文件 | 管什么 |
| --- | --- |
| [DESIGN.md](DESIGN.md) | 产品决策（D1–D12）、原则、适配器边界、数据保留、进程架构、界面门禁 |
| [PROTOCOL.md](PROTOCOL.md) | 线上协议 v1 与安全边界：传输、握手、方法、错误码、保留、公网暴露 |
| [DEPLOY.md](DEPLOY.md) | 服务端部署与运维、客户端构建工具链、CI 与发版 |

写文档只有两条规矩：**只写现状**（实现变了就直接改，不保留历史决策记录）；**重复内容只留一份**。

## 1. 现状

- Manager `1.0.0+106`；稳定版 `manager-v1.0.0`（正式签名，仅 arm64），
  测试包 `manager-v1.0.0-106` 由 CI 自动发 pre-release。
- Service `v1.0.0`，二进制 linux amd64/arm64。
- 本机没有 Flutter/JDK/Gradle，**不能本地构建**，改完 push 等 CI 出包。

已完成：协议 v1（握手、P-256 自证、消息、媒体、设备、审计）；引用消息 `quote_ref` 全链路
（QQ `message_scene.ext.msg_idx` → store schema v4 → 协议 → Manager → `message_reference`）；
群成员禁言/解除/踢人；扫码配对（`mobile_scanner`）；发送队列（文本 + 多图/多文件逐条）；
图片不套蓝气泡、文件显示为卡片；群聊/私聊标识；通知去重；独立后台进程 + 单一 WebSocket +
本地桥接；前台服务 `specialUse`；Hive 持久化聊天记录；JKS 正式签名与 Release workflow。

## 2. 代码结构

```
service/
  cmd/LianT/            init / serve / pair / daemon / db / version
  internal/
    protocol/           信封、方法、错误码（Go 版的 docs/PROTOCOL.md）
    server/             WS 接入、握手、限流、路由
    core/               Bot、消息、媒体、设备、审计的业务实现
    platform/           平台适配器接口 + qqbot
    format/             NDoc 与 qq-plain 格式适配器
    store/              消息、媒体、保留、设备、审计（SQLite / PostgreSQL）
    node/ crypto/ pairing/ 身份、密钥、配对
manager/lib/
  main.dart             入口 + backgroundMain（后台引擎入口，不能挪）
  core/
    app_state.dart      应用状态根（连接、Bot、消息、设备、审计、本地备注）
    net/                信封、模型、MessageClient 接口、直连实现、桥接/直连选择
    background/         独立进程：原生通道、共享状态、后台循环、本地桥接
    identity/           设备身份、P-256 签名（自己实现，Web 与 Android 一致）
    platform/           版本、更新检查、浏览器参数
  l10n/                 语言解析 + 每种语言一个 strings 文件
  pages/  ui/           页面与设计系统
```

## 3. 后台常驻（已实现，待真机验证）

需求：UI 进程被杀也要保持 WebSocket，且**全 App 只有一条连接**。结构见
[DESIGN.md](DESIGN.md) §6，实现要点：

- 清单里 `MessageService` 带 `android:process=":background"`、`stopWithTask="false"`、
  `foregroundServiceType="specialUse"`。
- Dart 入口 `backgroundMain` **必须放在 `lib/main.dart`**：AOT 按函数名在**根库**找入口，
  放进没人 import 的文件会被摇树删掉。
- 为什么不用 `dataSync`：targetSdk 35 的 Android 15 对它有每天 6 小时上限，会掐掉常驻收消息。
  `specialUse` 要声明 `PROPERTY_SPECIAL_USE_FGS_SUBTYPE`（否则 Android 14+ `startForeground` 抛异常），
  只在 `SDK_INT >= 34` 传显式类型。
- 生命周期：`START_STICKY`；`onTaskRemoved` 排 `setAndAllowWhileIdle` 闹钟兜底；
  `RestartReceiver` 处理开机/闹钟/常驻通知被划掉；UI 侧**不能**在 `detached`/`dispose` 停服务，
  只有 `unpair()` 才停。
- 降级：`createMessageClient()` 先等桥接最多 6 秒，等不到就直连，设置页「消息连接」会标明；
  回到前台且桥接心跳（90 秒内）活着时切回，最多 5 分钟试一次。直连降级期间可能出现两条连接。

**已知限制**：桥接是 127.0.0.1 的 TCP，任何 App 都能连端口，所以握手必须带后台随机令牌
（写在应用私有目录），端口每次启动都换；OEM 杀后台无法根治；身份（含私钥）明文存在应用私有目录。

真机验证清单：

```bash
adb shell ps -A | grep bailensn                 # 期望看到 :background 独立 PID
adb shell am kill com.bailensn.liant            # 杀 UI 进程后发消息，应仍收到通知
adb reboot                                      # 开机不打开 App 直接发消息
```

设置页三个诊断位：常驻通知「已收到 N 条」、设置「后台服务」（心跳）、「最近的错误」（`liant_crash.log`）。

## 4. 待办

1. 后台进程真机验证：划掉任务、强停、锁屏、重启、OEM 省电。
2. v106 通知文案与「后台服务」心跳行的实际表现。
3. 真机确认引用显示；`quote_ref` 需要 Service v1.0.0+ 与 Manager v1.0.0。
4. 扫码相机权限与配对流程。
5. 禁言/踢人权限（踢人接口要白名单/内邀）与失败提示。
6. 多图、文字 + 图片、文件发送。
7. SQLite v3→v4、PostgreSQL v3→v4 升级测试。
8. 安全日志分页：`audit.list` 已带 cursor，Manager 仍只加载最近 20 条。
9. 常驻通知没有「停用」入口，要不要在设置页给开关需要产品拍板。
10. `pubspec.lock` 比 `pubspec.yaml` 旧一个版本（lock 里还是 flutter_lints 4），
    找台有 Flutter 的机器跑 `flutter pub get` 后提交。

## 5. 迁移路线：Flutter 只做界面，核心搬 Kotlin

状态：**未开工**。后台进程现在是第二个 FlutterEngine 跑 Dart 协议客户端，约 50 MB，
低内存机型容易被杀；目标是 WS/协议/签名/身份/存储/通知全搬到 Kotlin，后台进程不再起 Flutter 引擎，
预期降到 15–20 MB。

```
UI 进程（Flutter）                    :background 进程（纯 Kotlin）
  pages/ ui/ l10n/                      LiantSocket   OkHttp WS + 握手 + 重连
  AppState（展示状态，不搬）             Auth          P-256 签名/验签
  BridgeMessageClient ──本地桥接──▶      BridgeServer  ServerSocket + JSON 行
                                        StateStore    读 liant_background_state.json
                                        Notifier      渠道/去重/前台判断
```

- **沿用现有 TCP 桥接**（127.0.0.1 + 随机令牌 + JSON 行），不改 Binder；Dart 侧
  `BridgeMessageClient` 一行都不动，只把对面的 `BridgeServer` 换成 Kotlin。
- **`AppState` 留在 Dart**：它把事件加工成展示状态，搬走只会让每屏改成订阅数据流，
  省不了内存（后台进程本来就不依赖 Flutter）。
- 要搬（按文件）：`net/client.dart` → `net/LiantSocket.kt`；`identity/p256.dart` →
  `crypto/P256.kt`；`background/bridge_server.dart` → `bridge/BridgeServer.kt`；
  `background/runtime_io.dart` → `BackgroundRuntime.kt`；`identity/identity.dart` → 状态解析。
  **不搬**：`app_state.dart`、`pages/`、`ui/`、`l10n/`、`BridgeMessageClient`。
- 三步走，每步可回退：① Kotlin 只做「连接 + 认证」，结果写 `liant_crash.log`，不动现有链路；
  ② 桥接搬到 Kotlin 并切流量，删掉 `MessageService` 里的 FlutterEngine；
  ③ 清理 Dart 侧残留，补 JVM 单测（P-256 用 `p256_test.dart` 的 openssl 向量验签）。
- 风险：签名互操作（Dart 自实现 vs JCA，被签字节/DER/base64 差一字节就失败）、
  OkHttp 要自定义 TrustManager 但不能丢掉指纹校验、桥接帧字段抄错即断、
  第一步会有两条连接并存（预期）。CI 只能验编译与 JVM 单测，认证必须真机看日志。

## 6. 约束

- 中文交流、中文注释、中文提交信息；不留假数据/假按钮，平台不支持就明确写不支持。
- 改 Dart 至少 `flutter analyze`（本机没有 Flutter，靠 CI 的 `client-check`）；改 Service 跑
  `go vet ./...`、`go test -race ./...`。Service 是 Go，不要引回 Spring/Java 后端。
- 不要提交 `key.properties`、`*.jks`、token、keystore。
- 本机没有 ssh 客户端，推 GitHub 用 HTTP + token（`~/newtoken.txt`，不要写进仓库或提交信息）：

```bash
TOKEN=$(tr -d '\r\n' < ~/newtoken.txt)
git -c user.name=Bailensn -c user.email=186718762+Bailensn@users.noreply.github.com commit -m "..."
git push "https://${TOKEN}@github.com/Bailensn/LianT.git" HEAD:main
```

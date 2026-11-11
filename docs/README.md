# 文档与现状

`service/` 是 Go 服务端（Bot Engine），`manager/` 是 Flutter 客户端（只发 Android）。
本文件是接手项目的入口：现状、结构、待办，以及**核心下沉 Kotlin 的施工说明**（§4）。
其余三份各管一件事：

| 文件 | 管什么 | 什么时候看 |
| --- | --- | --- |
| [DESIGN.md](DESIGN.md) | 产品决策（D1–D12）、原则、适配器边界、数据保留、进程架构、界面门禁 | 改产品方向、动数据模型或界面之前 |
| [PROTOCOL.md](PROTOCOL.md) | 线上协议 v1 与安全边界：传输、握手、方法、错误码、保留、公网暴露 | 改服务端接口或客户端协议层 |
| [DEPLOY.md](DEPLOY.md) | 服务端部署与运维、客户端构建工具链、CI 与发版 | 装机、升级、构建失败、发版 |

写文档只有两条规矩：**只写现状**（实现变了就直接改，不保留历史决策记录、不「冻结」）；
**重复内容只留一份**（协议细节只在 PROTOCOL，构建细节只在 DEPLOY）。

## 1. 现状

- Manager `1.0.0+107`，Service 同版本线；测试包 `manager-v1.0.0-107` 由 CI 自动发 pre-release。
- **正式版以后一个 Release 同时发两端**：tag `v<version>`，含 `LianT-linux-amd64`、
  `LianT-linux-arm64`、`LianT-android-arm64.apk`、`SHA256SUMS.txt`。
  历史遗留的 `v1.0.0`（只有 Service）与 `manager-v1.0.0`（只有 APK）是分开发的，不再沿用。
- 本机没有 Flutter/JDK/Gradle，**不能本地构建**：改完 push，等 CI 出包再装 APK 验证。

已完成：

- 协议 v1：WSS 握手、P-256 服务端自证、消息、媒体、设备、审计；
- 引用消息 `quote_ref` 全链路：QQ `message_scene.ext.msg_idx` → store schema v4 → 协议 →
  Manager → QQ `message_reference.message_id`；
- 群成员禁言 / 解除 / 踢人（`/v2/groups/{id}/restrict_chat_setting`、`batch_remove_members`）；
  「假 @」已删除（QQ 群发送接口没有 mention 字段，不再用纯文本冒充 @）；
- 扫码配对（`mobile_scanner`）与粘贴配对信息；相机权限；
- 发送队列：文本 + 多图/多文件逐条发送，输入框立即清空、失败不回填；
- 消息呈现：图片不套蓝气泡、文件显示为文件卡片、群聊/私聊标识；
- 通知：UI 与 `:background` 进程用同一通知 ID 去重，系统默认通知音；
- 独立后台进程 + 单一 WebSocket + 本地桥接，前台服务类型 `specialUse`（§2）；
- 本地聊天记录 Hive 持久化（不设条数上限），首次启动迁移旧 shared_preferences 数据；
- 性能：去掉实时模糊、MaterialApp 全局重建、会话列表懒构建、tab 隔离重绘；
- JKS 正式签名 + `Release` workflow。
- Kotlin 核心迁移第一步（§4）：OkHttp 连 WSS + JCA 完成认证，与 Dart 后台并存，待真机验证。

## 2. 代码结构

```
service/
  cmd/LianT/            init / serve / pair / daemon / db / version
  internal/
    protocol/           信封、方法、错误码（Go 版的 PROTOCOL.md）
    server/             WS 接入、握手、限流、路由、关闭码
    core/               Bot、消息、媒体、成员管理、设备、审计的业务实现
    platform/           平台适配器接口 + qqbot（token、gateway、types、profile）
    format/             NDoc 与 qq-plain 格式适配器
    store/              消息、媒体、保留、设备、审计（SQLite / PostgreSQL）
    node/ crypto/ pairing/  身份密钥、加密、配对
manager/android/app/src/main/kotlin/com/bailensn/liant/
  MessageService.kt     常驻服务、通知、桥接信息，以及第一步的 Kotlin 核心探针
  core/crypto/P256.kt   JCA 版 P-256（签名/验签/指纹，与 Dart、Go 互操作）
  core/net/LiantSocket.kt  OkHttp WSS、握手、心跳、退避重连
  core/state/StateStore.kt 读共享状态文件（身份/免打扰/标题/备注）
manager/lib/
  main.dart             入口 + backgroundMain（后台引擎入口，不能挪）
  core/
    app_state.dart      应用状态根（连接、Bot、消息、设备、审计、本地备注）
    net/                信封、模型、MessageClient 接口、直连 LiantClient、桥接/直连选择
    background/         独立进程：原生通道、共享状态、后台循环、本地桥接（协议/服务端/客户端）
    identity/           设备身份、P-256 签名（自己实现，Web 与 Android 一致）
    platform/           版本、更新检查、浏览器参数
  l10n/                 语言解析 + 每种语言一个 strings 文件（zh_Hant/en/ja/ko）
  pages/               会话、聊天、机器人、设置、配对、安全日志
  ui/                  tokens（颜色/字号/圆角）、widgets（大标题页/玻璃底栏/抽屉）、format
```

## 3. 后台常驻（已实现，待真机验证）

需求：App 的 UI 被杀也要保持 WebSocket，而且**全 App 只有一条连接**。

```
UI 进程（Flutter）                      :background 进程（MessageService）
  MainActivity                            startForeground(specialUse) + 常驻通知
  FlutterEngine(main)                     FlutterEngine(backgroundMain)
  BridgeMessageClient                     BridgeServer ── LiantClient ── WSS
        │  TCP 127.0.0.1 + 令牌                 ▲
        └──────── JSON 行（§4.6）───────────────┘
  写 liant_background_state.json          读同一文件（身份/前台/免打扰/标题/备注）
```

- 清单：`MessageService` 带 `android:process=":background"`、`stopWithTask="false"`、
  `foregroundServiceType="specialUse"`。
- **Dart 入口 `backgroundMain` 必须放在 `lib/main.dart`**：AOT 按函数名在**根库**里找入口，
  放进没人 import 的文件会被摇树删掉。后台循环在 `core/background/runtime_io.dart`。
- 桥接端口与令牌由原生通道 `liant/background_state` 写进 `liant_bridge.json`；
  UI 的 `bot.list` / `msg.send` / `state.snapshot` 全部经桥接转给后台，再由那条唯一的 WS 发出。
- 为什么不用 `dataSync`：targetSdk 35 的 Android 15 对它有每天 6 小时上限，会掐掉常驻收消息。
  `specialUse` 要声明 `PROPERTY_SPECIAL_USE_FGS_SUBTYPE`（否则 Android 14+ `startForeground`
  抛异常），只在 `SDK_INT >= 34` 传显式类型；没有时长限制，代价是上架 Google Play 要申报
  （本项目只侧载）。
- 生命周期：`START_STICKY`；系统重启服务时 `onStartCommand` 的 intent 可能为 null，引擎在
  `onCreate` 恢复；`onTaskRemoved` 排 `setAndAllowWhileIdle` 闹钟（60s 后）兜底拉起；
  `RestartReceiver` 处理开机/闹钟/常驻通知被划掉三个来源；UI 侧**不能**在
  `detached`/`dispose` 停服务，只有 `unpair()` 才停。
- 降级：`createMessageClient()` 先等桥接最多 6 秒，等不到就直连，设置页「消息连接」会标明
  「主进程直连（后台服务未就绪）」；回前台且桥接心跳（90 秒内）活着时切回，最多 5 分钟试一次。
  直连降级期间可能出现两条连接——这是唯一会出现双连接的情况。
- 跨进程状态不能用 `shared_preferences`（每进程一份内存缓存，`reload()` 也不回读磁盘），
  改成共享文件 `liant_background_state.json`：先写 `.tmp` 再 rename，写操作串行化。
  文件存在即代表「已配对」，`RestartReceiver` 靠它决定开机要不要拉起服务。
- 通知分工：两边用同一个 `notificationIdFor(msgId)`（FNV-1a，避开常驻通知的 1001），
  UI 在前台时后台不弹，且都带 `setOnlyAlertOnce(true)`。

**已知限制**

- 桥接是 127.0.0.1 的 TCP，任何 App 都能连端口，所以握手必须带后台随机令牌
  （写在应用私有目录，别的 App 读不到），端口每次启动都换。
- 后台被 OEM 杀掉后，UI 6 秒后降级直连，回前台再尝试切回。
- Android 12+ 从后台启动前台服务受限；省电白名单只能引导用户加，不能承诺永不杀。
- 身份（含私钥）明文存在应用私有目录的共享文件里，和 shared_preferences 一个级别。

**真机验证清单**

```bash
adb shell ps -A | grep bailensn              # 期望看到 :background 独立 PID
adb shell am kill com.bailensn.liant         # 杀 UI 进程后发消息，应仍收到通知
adb reboot                                   # 开机不打开 App 直接发消息
```

设置页三个诊断位：常驻通知「已连接 · 已收到 N 条」、设置「后台服务」（心跳）、
「最近的错误」（`liant_crash.log` 最后一条）。

## 4. 迁移：核心下沉 Kotlin

状态：**第一步已落地，待真机验证**（Kotlin 连接 + 认证，与现有 Dart 后台并存）。
本节的步骤与回退方式仍然有效，第二步开工前先读完。

### 4.1 为什么

后台进程现在是「第二个 FlutterEngine」在跑 Dart 版协议客户端，约 50 MB PSS，
低内存机型容易被杀。把 WebSocket / 协议 / 签名 / 身份 / 通知搬到 Kotlin 后，
后台进程不再起 Flutter 引擎，预期降到 **15–20 MB**；顺带让协议层能用 JCA 与 OkHttp，
不再自己维护一套 socket 与密码学实现。

### 4.2 目标结构

```
UI 进程（Flutter）                        :background 进程（纯 Kotlin）
  pages/ ui/ l10n/                          LiantSocket    OkHttp WS + 握手 + 重连
  AppState（展示状态，不搬）                  Auth           P-256 签名/验签
  BridgeMessageClient ──本地桥接──▶           BridgeServer   ServerSocket + JSON 行
                                            StateStore     读 liant_background_state.json
                                            Notifier       渠道/去重/免打扰/前台判断
```

`MessageService` 只做 `startForeground` + 启动 Kotlin 核心，不再创建 `FlutterEngine`。

### 4.3 关键决定

1. **沿用现有 TCP 桥接**（127.0.0.1 + 随机令牌 + JSON 行），不改 Binder。
   Dart 侧 `BridgeMessageClient` 一行都不用动，只把对面的 `BridgeServer` 换成 Kotlin 实现。
2. **`AppState` 留在 Dart**。它的职责是「把事件加工成页面展示状态」；搬过去只会让每个
   页面改成订阅数据流，省不了内存（后台进程本来就不依赖 Flutter）。
3. **`main.dart` 的 `backgroundMain` 与 Dart 后台运行时最后删**：迁移期间它是回退路径。

**不搬**：`app_state.dart`、`pages/`、`ui/`、`l10n/`、`BridgeMessageClient`、`MessageClient` 接口。

### 4.4 文件搬迁表

| Dart 现状 | 行数 | Kotlin 目标 | 职责 |
| --- | --- | --- | --- |
| `core/net/client.dart` `LiantClient` | 441 | `core/net/LiantSocket.kt`（已实现） | OkHttp WS、信封收发、`reply_to` 关联、30s 心跳、1→30s 退避重连、4001/4002 不重连 |
| `core/identity/p256.dart` | 477 | `core/crypto/P256.kt`（已实现） | `Signature("SHA256withECDSA")`，32 字节标量 → `ECPrivateKey`，SPKI 解析与验签 |
| `core/background/bridge_server.dart` | 206 | `core/bridge/BridgeServer.kt` | accept 一条连接、令牌握手、`req/send/ev/state/res` 帧 |
| `core/background/runtime_io.dart` | 190 | `core/BackgroundRuntime.kt` | 读共享状态、免打扰与前台判断、FNV-1a 通知 ID、通知渠道 |
| `core/identity/identity.dart` | 230 | `core/state/StateStore.kt`（已实现） | 身份 JSON 已在共享文件里，只做解析与 `device_id` 提取 |
| `core/background/state_io.dart` | — | `core/state/StateStore.kt`（已实现） | 读 `liant_background_state.json`；原子写（`.tmp` + rename）留给第二步 |
| 本地消息存储（Hive） | — | 第二阶段再定 | 可留 Dart（省事），或搬 SQLite/DataStore |

字段名以 `manager/lib/core/net/` 下的代码为准，**不要凭记忆写**。

### 4.5 Kotlin 侧要照做的协议细节

1. 连接 `wss://host:port/v1/ws`；自签证书要放行（OkHttp 自定义 `X509TrustManager` +
   `HostnameVerifier`）。真正的信任是下一步的指纹校验，**不要顺手关掉它**。
2. 服务端发 `auth.challenge {nonce, ts, service_pub, service_sig}`。
3. 验 `service_sig`：被签字节是 `"$nonce\n$fingerprint\n$ts"` 的 UTF-8，
   `service_pub` 是 base64(PKIX DER)，ECDSA-SHA256。**指纹以配对时钉下的为准**，
   服务端说不出自己是谁就断线。
4. 回 `auth.response {manager_id: deviceId, sig}`，`sig` 用同一串字节签，
   外层信封带 `reply_to: challenge.id`。
5. 收到 `auth.ok` 置在线，开始 30s `ping`；服务端发来的 `ping` 要回 `pong`。
6. 收到 `msg.inbound` / `event.msg`：解析 `msg_id / bot_id / peer_id / peer_kind /
   sender_id / sender_name / msg_type / body.plain`，走通知逻辑，**同时原样转发给桥接**。
7. 其它事件（`event.bot` / `event.peer` / `event.audit` / `auth.ok`）也必须转发，
   UI 的会话标题、Bot 状态、审计都靠它们。
8. 请求-响应：桥接 `req` 帧 → 用新信封 id 发出 → 等到 `reply_to == id` 的响应再回 `res` 帧；
   超时 15s 回 `code: "timeout"`。

### 4.6 桥接帧（现在就是这套，不要改字段）

JSON 行、TCP 127.0.0.1、版本 `v: 1`：

| 帧 | 方向 | 字段 |
| --- | --- | --- |
| `hello` | UI→后台 | `{hello: token, v: 1}`；令牌在 `liant_bridge.json` |
| `ready` | 后台→UI | 握手通过 |
| `state` | 后台→UI | `{state: ConnState.name}`，连接状态变化 |
| `ev` | 后台→UI | `{t, payload}`，协议事件原样转发 |
| `req` | UI→后台 | `{id, method, payload}`，需要响应 |
| `send` | UI→后台 | `{id, method, payload}`，不需要响应 |
| `res` | 后台→UI | `{id, ok, payload}` 或 `{id, ok:false, code, message}` |
| `bye` | UI→后台 | 主动断开 |

建议先把 Dart 里的帧常量（`core/background/bridge_protocol.dart`）抄成 Kotlin 常量，
再动手写逻辑，错一个字段 UI 就整条连不上。

### 4.7 三步施工（每步可回退）

**第一步：Kotlin 只做「连接 + 认证」**

- 新增 `LiantSocket.kt` + `P256.kt`，从状态文件读身份，连上并完成 `auth.response`。
- **不碰**现有 Dart 后台链路；成功与否只写 `liant_crash.log` 或一条调试日志（设置页能看）。
- 依赖：`app/build.gradle.kts` 加 OkHttp。
- 验证：真机装包后看日志出现 `auth.ok`（logcat 过滤 `LianT`）。这一步最容易翻车——
  DER 编码、SPKI、base64 差一个字节就失败；JVM 单测用 `p256_test.dart` 里 openssl
  生成的同一组向量（`P256Test.kt`，CI 的 `testDebugUnitTest` 会跑）。
- 现状：`MessageService` 在 `onCreate` 里另起一个线程跑探针，**不影响** Dart 链路；
  结果写 logcat，失败另落 `liant_crash.log`。两条连接同时存在是这一步的预期现象。
- 回退：删掉新增文件与 `MessageService` 里的探针调用即可，主链路没动过。

**第二步：桥接搬到 Kotlin，切流量**

- `BridgeServer.kt` 实现同样的帧协议，继续写 `liant_bridge.json`（端口/令牌/心跳）。
- 删掉 `MessageService` 里的 `FlutterEngine` 与 `backgroundMain` 调用。
- Dart 侧只保留 `BridgeMessageClient`；删除 `core/background/runtime*.dart`、
  `bridge_server.dart`、`core/net/client.dart`。
- 需要先决定降级路径（见 §4.8 第 5 条）。
- 验证：设置页「消息连接」显示后台常驻；收发消息、通知、免打扰都正常。
- 回退：把 `git revert` 到上一步的 commit。

**第三步：清理与收尾**

- 删 `main.dart` 的 `backgroundMain`、`core/net/socket*`、`p256.dart`（若不再用）。
- 补 JVM 单测：P-256 验签向量、桥接帧固定 JSON 往返。
- 更新本文 §1/§2 的代码结构与内存说明。

### 4.8 风险与对策

1. **签名互操作**：Dart 用自实现 P-256，Kotlin 用 JCA。被签字节、DER、base64 必须完全一致。
   对策：先跑 openssl 向量单测，再上真机。
2. **自签证书**：OkHttp 默认拒绝。对策：自定义 `TrustManager`，但**不能**因此丢掉指纹校验。
3. **桥接协议**：UI 侧不改，帧类型/字段错一个就断。对策：先抄常量，再写最小往返测试。
4. **两条连接并存期**（第一步）：服务端会看到同一设备两条连接，属预期，第二步后消失。
5. **降级路径要重新决定**：现在桥接连不上时 UI 会直连；核心搬走后 Dart 已没有直连实现。
   建议：桥接不可用就直接显示「后台未就绪」，不再直连，避免双连接与状态分叉。
6. **通知职责**：搬到 Kotlin 后前台判断改读共享文件，别再依赖 Dart 的 `AppState`；
   `foreground=true` 只有在桥接 `hasPeer` 为真时才可信（否则 UI 进程已死，通知会被吞）。

### 4.9 验证手段与完成标准

- CI 只能验证**编译**与 **JVM 单测**；认证是否通过必须真机看日志。
- 三个诊断位：常驻通知「已收到 N 条」、设置「后台服务」（心跳）、「最近的错误」。
- 内存对比：开发者选项 → 正在运行的服务 → `:background` 的 PSS，迁移前后各记一次。
- 完成标准：三个诊断位全部正常 + 杀 UI 进程后仍能收通知 + 内存降到 20 MB 级 +
  §2 的真机验证清单全过。

## 5. 待办

1. 后台进程真机验证：划掉任务、强停、锁屏、重启、OEM 省电（§3 清单）。
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
11. §4 第一步真机验证：装 v107 看 `LianT` 日志是否出现 `auth.ok`，记一次 `:background` 的 PSS。

## 6. 约束

- 中文交流、中文注释、中文提交信息；不留假数据/假按钮，平台不支持就明确写不支持。
- 改 Dart 至少 `flutter analyze`（本机没有 Flutter，靠 CI 的 `client-check`）；改 Service 跑
  `go vet ./...`、`go test -race ./...`。Service 是 Go，不要引回 Spring/Java 后端。
- 不要提交 `key.properties`、`*.jks`、token、keystore。
- 本机没有 ssh 客户端，推 GitHub 用 HTTP + token（`~/newtoken.txt`，不要写进仓库或提交信息）：

```bash
TOKEN=$(tr -d '\r\n' < ~/newtoken.txt)
git -c user.name=Bailensn -c user.email=186718762+Bailensn@users.noreply.github.com commit -m "..."
git push "https://x-access-token:${TOKEN}@github.com/Bailensn/LianT.git" HEAD:main
```

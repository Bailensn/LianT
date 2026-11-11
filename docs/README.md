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

- Manager `1.0.0+112`，Service 同版本线；测试包 `manager-v1.0.0-112` 由 CI 自动发 pre-release。
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
- 独立后台进程 + 单一 WebSocket + 本地桥接；后台是纯 Kotlin（OkHttp + JCA），不创建 FlutterEngine（§3）；
- 本地聊天记录 Hive 持久化（不设条数上限），首次启动迁移旧 shared_preferences 数据；
- 性能：去掉实时模糊、MaterialApp 全局重建、会话列表懒构建、tab 隔离重绘；
- JKS 正式签名 + `Release` workflow。
- 核心下沉 Kotlin（§4）：连接、认证、桥接、通知全部落地，Dart 只剩 UI 与 `BridgeMessageClient`，待真机验证。
- 原生 UI 试水：Compose 设置页，逐行对齐 Flutter 版；数据来自本机文件 + 桥接 `state.snapshot`；
  临时用 `adb shell am start -n com.bailensn.liant/.NativeSettingsActivity` 打开；
  圆角用与 Flutter `ContinuousRectangleBorder` 相同的连续曲率路径（`ui/theme/Squircle.kt`）。

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
  main.dart             入口（`:background` 不再跑 Dart，没有 backgroundMain）
  core/
    app_state.dart      应用状态根（连接、Bot、消息、设备、审计、本地备注）
    net/                信封、模型、MessageClient 接口、桥接客户端选择、占位客户端
    background/         共享状态文件、桥接信息与帧常量、服务控制
    identity/           设备身份、P-256 签名（配对仍走 Dart）
    platform/           版本、更新检查、浏览器参数
  l10n/                 语言解析 + 每种语言一个 strings 文件（zh_Hant/en/ja/ko）
  pages/               会话、聊天、机器人、设置、配对、安全日志
  ui/                  tokens（颜色/字号/圆角）、widgets（大标题页/玻璃底栏/抽屉）、format
```

## 3. 后台常驻（纯 Kotlin，待真机验证）

需求：App 的 UI 被杀也要保持 WebSocket，而且**全 App 只有一条连接**。

```
UI 进程（Flutter）                        :background 进程（MessageService）
  MainActivity                              startForeground(specialUse) + 常驻通知
  FlutterEngine(main)                       BackgroundRuntime
  BridgeMessageClient                       ├─ BridgeServer（本地 TCP + JSON 行）
        │  TCP 127.0.0.1 + 令牌              ├─ LiantSocket ── WSS
        └──────── JSON 行（§4.6）─────────────┤
  写 liant_background_state.json            ├─ StateStore（读同一文件）
                                            └─ Notifier（渠道/去重/免打扰）
```

- 清单：`MessageService` 带 `android:process=":background"`、`stopWithTask="false"`、
  `foregroundServiceType="specialUse"`。
- 后台进程**不再创建 FlutterEngine**：`BackgroundRuntime` 自己开线程读共享状态、
  连 WSS、转发事件、弹通知。Dart 侧只保留 `BridgeMessageClient`。
- 桥接端口与令牌由 Kotlin 写进 `liant_bridge.json`（30 秒刷新 `at`）；
  UI 的 `bot.list` / `msg.send` / `state.snapshot` 全部经桥接转给后台，再由那条唯一的 WS 发出。
- 为什么不用 `dataSync`：targetSdk 35 的 Android 15 对它有每天 6 小时上限，会掐掉常驻收消息。
  `specialUse` 要声明 `PROPERTY_SPECIAL_USE_FGS_SUBTYPE`（否则 Android 14+ `startForeground`
  抛异常），只在 `SDK_INT >= 34` 传显式类型；没有时长限制，代价是上架 Google Play 要申报
  （本项目只侧载）。
- 生命周期：`START_STICKY`；系统重启服务时 `onStartCommand` 的 intent 可能为 null，
  运行时在 `onCreate` 恢复；`onTaskRemoved` 排 `setAndAllowWhileIdle` 闹钟（60s 后）兜底拉起；
  `RestartReceiver` 处理开机/闹钟/常驻通知被划掉三个来源；UI 侧**不能**在
  `detached`/`dispose` 停服务，只有 `unpair()` 才停。
- 降级：`createMessageClient()` 先等桥接最多 6 秒，等不到返回 `UnavailableClient`，
  设置页「消息连接」显示「后台服务未就绪 · 不直连，等它起来」；回前台且桥接心跳
  （90 秒内）活着时切回，最多 5 分钟试一次。**不再有直连降级**，因此不会出现两条连接。
- 跨进程状态不能用 `shared_preferences`（每进程一份内存缓存，`reload()` 也不回读磁盘），
  改成共享文件 `liant_background_state.json`：先写 `.tmp` 再 rename，写操作串行化。
  文件存在即代表「已配对」，`RestartReceiver` 靠它决定开机要不要拉起服务。
- 通知分工：两边用同一个 `notificationIdFor(msgId)`（FNV-1a，避开常驻通知的 1001），
  UI 在前台时后台不弹，且都带 `setOnlyAlertOnce(true)`。Kotlin 侧的前台判断是
  「共享文件 `foreground` 为真 **且** 桥接有对端」，UI 进程没了就不会吞通知。

**已知限制**

- 桥接是 127.0.0.1 的 TCP，任何 App 都能连端口，所以握手必须带后台随机令牌
  （写在应用私有目录，别的 App 读不到），端口每次启动都换。
- 后台被 OEM 杀掉后，UI 显示「后台未就绪」，回前台再尝试切回。
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

状态：**第一、二步已完成，待真机验证**。连接、认证、桥接、通知都在 `:background`
的纯 Kotlin 运行时里；第三步只剩收尾与补测。

### 4.1 为什么

后台进程原来跑「第二个 FlutterEngine」执行 Dart 版协议客户端，约 50 MB PSS，
低内存机型容易被杀。WebSocket / 协议 / 签名 / 身份 / 通知搬到 Kotlin 后，
后台不再起 Flutter 引擎，预期降到 **15–20 MB**；协议层也用上了 JCA 与 OkHttp。

### 4.2 现在的结构

```
UI 进程（Flutter）                        :background 进程（纯 Kotlin）
  pages/ ui/ l10n/                          LiantSocket    OkHttp WS + 握手 + 重连
  AppState（展示状态，不搬）                  P256           JCA 签名/验签/指纹
  BridgeMessageClient ──本地桥接──▶           BridgeServer   ServerSocket + JSON 行
                                            BackgroundRuntime 状态/事件/通知
                                            StateStore     读 liant_background_state.json
                                            Notifier       渠道/去重/免打扰/前台判断
```

`MessageService` 只做 `startForeground`、常驻通知文案与兜底拉起。

### 4.3 关键决定

1. **沿用现有 TCP 桥接**（127.0.0.1 + 随机令牌 + JSON 行），不改 Binder。
   Dart 侧 `BridgeMessageClient` 只改了「对面换人」，帧协议没动。
2. **`AppState` 留在 Dart**。它的职责是「把事件加工成页面展示状态」；搬过去只会让每个
   页面改成订阅数据流，省不了内存（后台进程本来就不依赖 Flutter）。
3. **不再保留直连降级**：桥接不可用就显示「后台未就绪」，避免双连接与状态分叉。

**不搬**：`app_state.dart`、`pages/`、`ui/`、`l10n/`、`BridgeMessageClient`、`MessageClient` 接口。

### 4.4 文件对照

| Dart 原文件 | Kotlin 现在 | 说明 |
| --- | --- | --- |
| `core/net/client.dart`（已删） | `core/net/LiantSocket.kt` | OkHttp WS、信封收发、`reply_to` 关联、30s 心跳、1→30s 退避重连、4001/4002 不重连 |
| `core/background/bridge_server.dart`（已删） | `core/bridge/BridgeServer.kt` | accept 一条连接、令牌握手、`req/send/ev/state/res` 帧 |
| `core/background/runtime_io.dart`、`runtime.dart`、`runtime_stub.dart`（已删） | `core/BackgroundRuntime.kt` | 读共享状态、连接、免打扰与前台判断、5 分钟重试 |
| 通知（`runtime_io.dart` 里） | `core/Notifier.kt` | FNV-1a 通知 ID、渠道、只响一次 |
| `core/identity/p256.dart`（保留，配对用） | `core/crypto/P256.kt` | 认证用 JCA；配对仍在 Dart 生成密钥与签名 |
| `core/background/state_io.dart`（保留） | `core/state/StateStore.kt` | UI 写、Kotlin 读同一份 `liant_background_state.json` |
| `core/net/socket*.dart`（保留，配对用） | — | 配对时的一次性连接，不参与日常消息 |

字段名以 `manager/lib/core/net/` 下的代码为准，**不要凭记忆写**。

### 4.5 Kotlin 侧的协议细节

1. 连接 `wss://host:port/v1/ws`；自签证书放行（OkHttp 自定义 `X509TrustManager` +
   `HostnameVerifier`）。真正的信任是 `auth.challenge` 里的指纹校验，**不要顺手关掉它**。
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

### 4.6 桥接帧（不要改字段）

JSON 行、TCP 127.0.0.1、版本 `v: 1`：

| 帧 | 方向 | 字段 |
| --- | --- | --- |
| `hello` | UI→后台 | `{hello: token, v: 1}`；令牌在 `liant_bridge.json` |
| `ready` | 后台→UI | `{t: "ready", v: 1}`，握手通过 |
| `state` | 后台→UI | `{t: "state", state: ConnState.name, detail}` |
| `ev` | 后台→UI | `{t: "ev", env: <完整信封>}` |
| `req` | UI→后台 | `{t: "req", id, method, payload}`，需要响应 |
| `send` | UI→后台 | `{t: "send", id, method, payload}`，不需要响应 |
| `res` | 后台→UI | `{t: "res", id, ok, payload}` 或 `{t: "res", id, ok: false, code, message}` |
| `bye` | UI→后台 | `{t: "bye"}`，主动断开 |

Kotlin 侧的常量集中在 `core/bridge/BridgeProtocol.kt`；`test/bridge_test.dart` 用一个
按帧协议实现的假服务端守住 UI 侧，写错字段这条测试会先红。

### 4.7 三步施工

**第一步：Kotlin 只做「连接 + 认证」（已完成）**

- 新增 `LiantSocket.kt` + `P256.kt`，从状态文件读身份，完成 `auth.response`。
- JVM 单测用 openssl 向量守住 DER/SPKI/base64 的互操作（`P256Test.kt`，CI 跑
  `testDebugUnitTest`）。
- 当时与 Dart 后台并存的两条连接，已在第二步消失。

**第二步：桥接搬到 Kotlin，切流量（已完成）**

- `BridgeServer.kt` 实现同样的帧协议，写 `liant_bridge.json`（端口/令牌/心跳）。
- `MessageService` 删掉 `FlutterEngine` 与 `backgroundMain`；Dart 删除
  `runtime*.dart`、`bridge_server.dart`、`core/net/client.dart`。
- 降级改为「后台未就绪」，不再直连（`UnavailableClient`）。

**第三步：清理与收尾（待做）**

- `socket*.dart` 与 `p256.dart` **暂不能删**：配对仍走 Dart 的一次性 WebSocket 与密钥生成；
  要删得先把配对搬到 Kotlin。
- 补 JVM 单测：桥接帧固定 JSON 往返（与 `bridge_test.dart` 的假服务端对齐）。
- 真机过一遍 §3 清单，记迁移前后的 `:background` PSS。

### 4.8 风险与现状

1. **签名互操作**：JVM 单测已用 openssl 向量验过（同一组向量 Dart 侧也过）；
   真机 `auth.ok` 仍待确认。
2. **自签证书**：已用 trust-all + 指纹校验，二者缺一不可。
3. **桥接协议**：Dart 侧假服务端测试 + Kotlin 常量集中，改字段必须两边同时改。
4. **两条连接并存**：已随第二步消失。
5. **降级路径**：已按建议改成「后台未就绪」，不直连。
6. **通知职责**：Kotlin 读共享文件，`foreground=true` 只有在桥接 `hasPeer` 为真时才可信。

### 4.9 验证手段与完成标准

- CI 验证**编译**与 **JVM 单测**；认证与桥接是否真的通，必须真机看。
- 三个诊断位：常驻通知「已收到 N 条」、设置「后台服务」（心跳）、「最近的错误」。
- 内存对比：开发者选项 → 正在运行的服务 → `:background` 的 PSS，迁移前后各记一次。
- 完成标准：三个诊断位全部正常 + 杀 UI 进程后仍能收通知 + 内存降到 20 MB 级 +
  §3 的真机验证清单全过。

## 5. 待办

1. 后台进程真机验证：划掉任务、强停、锁屏、重启、OEM 省电（§3 清单）。
2. v108 通知文案与「后台服务」心跳行的实际表现。
3. 真机确认引用显示；`quote_ref` 需要 Service v1.0.0+ 与 Manager v1.0.0。
4. 扫码相机权限与配对流程。
5. 禁言/踢人权限（踢人接口要白名单/内邀）与失败提示。
6. 多图、文字 + 图片、文件发送。
7. SQLite v3→v4、PostgreSQL v3→v4 升级测试。
8. 安全日志分页：`audit.list` 已带 cursor，Manager 仍只加载最近 20 条。
9. 常驻通知没有「停用」入口，要不要在设置页给开关需要产品拍板。
10. `pubspec.lock` 比 `pubspec.yaml` 旧一个版本（lock 里还是 flutter_lints 4），
    找台有 Flutter 的机器跑 `flutter pub get` 后提交。
11. §4 真机验证：装 v108，确认设置页「消息连接」= 后台进程常驻、通知/免打扰正常、
    杀 UI 进程后仍能收消息，并记一次 `:background` 的 PSS。
12. §4 第三步：桥接帧的 JVM 往返单测；配对搬到 Kotlin 后再删 `socket*.dart` / `p256.dart`。
13. UI 试水评估：装 v112，`adb shell am start -n com.bailensn.liant/.NativeSettingsActivity` 打开原生设置页，
    与 Flutter 版截图逐像素对比（`python3 manager/tool/ui_diff.py flutter.png native.png diff.png`）；
    决定是否全迁（全迁要先定 Hive 历史数据 / 服务端会话列表，否则会话列表装完是空的）。

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

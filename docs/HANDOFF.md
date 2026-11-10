# 交接说明（给下一个会话）

仓库：`/storage/emulated/0/LianT`，分支 `main`。
Manager 版本：`1.0.0+101`，versionCode `101`。
稳定版已发布：Service `v1.0.0`、Manager `manager-v1.0.0`（正式签名、仅 arm64）。
最近两个测试包（arm64，`.github/workflows/ci.yml` 的 android job 自动发 pre-release）：

- `manager-v1.0.0-100`：独立 `:background` 进程（两进程各一条连接）
- `manager-v1.0.0-101`：前台服务改 `specialUse` + 后台独占 WebSocket、本地桥接转发

先看 `git status`。工作区可能有一处未提交的
`manager/android/gradle.properties`，是 Flutter migrator 自动加的
`android.builtInKotlin=false`、`android.newDsl=false`，不要盲目提交。

本机没有 Flutter/JDK/Gradle，**不能本地构建**：改完等 CI（push 触发
`.github/workflows/ci.yml`）出包，从 Release 下载 APK。推送方式见 §五。

---

## 一、后台常驻：独立进程 + 单一连接（已实现，待真机验证）

用户要求：**独立进程**，UI 进程被杀后仍保持 WebSocket；而且**全 App 只有后台进程
那一条连接**，事件实时转发回 UI 进程。

### 结构

```
UI 进程                                :background 进程（MessageService）
  MainActivity                          startForeground（specialUse）+ 常驻通知
  FlutterEngine（main）                 FlutterEngine（backgroundMain）
  BridgeMessageClient                   BridgeServer ── LiantClient ── WSS
        │  TCP 127.0.0.1 + 令牌                ▲
        └────────── JSON 行协议 ──────────────┘
  写 liant_background_state.json         读同一个文件（前台/免打扰/标题/备注）
```

- 清单：`MessageService` 带 `android:process=":background"`、`stopWithTask="false"`、
  `foregroundServiceType="specialUse"`（类型说明见 §一.4）。
- Dart 入口 `backgroundMain` **放在 `lib/main.dart`**，不能挪走：AOT 按函数名在
  **根库**里找入口，放在没人 import 的文件里会被摇树删掉。后台逻辑在
  `lib/core/background_runtime_io.dart`。
- 桥接：`lib/core/bridge_server_io.dart`（后台）/ `bridge_client_io.dart`（UI），
  协议在 `bridge_protocol.dart`，端口与令牌在 `liant/background_state` 通道
  给出的 `liant_bridge.json` 里。UI 的请求（bot.list / msg.send / state.snapshot …）
  全部经桥接转给后台进程，再由那条唯一的 WebSocket 发出去。

### 1. 只有一条 WebSocket

- 后台进程 `LiantClient` 收到的 `msg.inbound` / `event.*` / `auth.ok` 全部
  通过桥接 `broadcastEvent` 实时转给 UI；连接状态用 `broadcastState`。
- UI 进程的 `AppState` 拿到的是 `MessageClient` 接口（`client_api.dart`），
  实现可以是直连 `LiantClient`，也可以是 `BridgeMessageClient`。
- 降级：Android 上 `createMessageClient()` 先等桥接最多 6 秒，等不到就直连，
  并在设置页「消息连接」显示「主进程直连（后台服务未就绪）」。回到前台时如果
  后台服务已经活了（桥接文件心跳 90 秒内），会切回桥接（最多 5 分钟试一次）。
- 直连降级期间如果后台也在跑，服务端会看到两条连接 —— 这是唯一会出现双连接的情况。

### 2. 跨进程状态

`shared_preferences` 每个进程各一份内存缓存，`reload()` 也不回读磁盘，所以改成
共享文件 `liant_background_state.json`（UI 写、后台读，先写 `.tmp` 再 rename，
写操作串行化）。内容是身份 + 前台标志 + 免打扰 + 会话标题 + 备注；文件存在即
代表「已配对」，`RestartReceiver` 靠它决定开机要不要拉起服务。

### 3. 通知去重与分工

- 两边算同一个通知 ID：`notificationIdFor(msgId)`（protocol.dart，FNV-1a，
  并避开常驻通知的 1001）。
- UI 在前台时后台进程不弹（读共享状态里的 `foreground`）；UI 不在前台时后台弹。
- 通知都带 `setOnlyAlertOnce(true)`，同一条消息最多响一次。

### 4. 前台服务类型：`specialUse`

`dataSync` 在 **targetSdk 35 的 Android 15** 上有每天 6 小时上限，会把这个
「一直在收消息」的服务掐掉，所以换成 `specialUse`：

- 权限 `FOREGROUND_SERVICE_SPECIAL_USE`；
- `<property android:name="android.app.PROPERTY_SPECIAL_USE_FGS_SUBTYPE">`
  必须声明用途，否则 Android 14+ `startForeground` 抛异常；
- 没有时长限制；代价是上架 Google Play 时要申报理由（本项目只侧载，不涉及）；
- 运行时只在 `SDK_INT >= 34` 传显式类型，29~33 用两参版本按清单类型提交。

### 5. 生命周期

- `START_STICKY`；系统重启服务时 `onStartCommand` 的 intent 可能为 null，
  引擎在 `onCreate` 里恢复。
- `onTaskRemoved` 排一个 `setAndAllowWhileIdle` 闹钟（60 秒后）兜底拉起，
  失败只记日志（Android 12+ 后台启动前台服务会被系统拒绝）。
- `RestartReceiver`：开机、闹钟、常驻通知被划掉三个来源共用。
- UI 侧**不能**在 `detached`/`dispose` 里停服务，只有 `unpair()` 才停。

### 6. 已知限制

- 桥接是 127.0.0.1 的 TCP，任何 App 都能连端口，所以握手必须带后台随机令牌
  （写在应用私有目录，别的 App 读不到），端口每次启动都换。
- 后台进程被 OEM 杀掉后，UI 会先在 6 秒后降级直连，回前台再尝试切回。
- Android 12+ 从后台拉起前台服务受限；省电白名单只能引导用户加，不能承诺永不杀。
- 身份（含私钥）明文存在应用私有目录的共享文件里，和 shared_preferences 一个级别。

### 真机验证清单

```bash
# 装包后：确认有两个进程
adb shell ps -A | grep bailensn        # 期望看到 :background 独立 PID

# 设置页「消息连接」应显示「后台进程常驻」；常驻通知应显示「已连接」

# 验证「UI 进程被杀后仍在线」
adb shell am kill com.bailensn.liant        # 只杀可回收进程
# 或只杀 UI 进程的 PID：
adb shell ps -A | grep bailensn | grep -v ':background'
adb shell kill <PID>
# 然后从别的号给 Bot 发消息：应收到通知（界面重新打开后能看到这些消息）

# 验证开机恢复
adb reboot   # 开机后不打开 App，直接发消息
```

## 二、已经完成的功能

- 协议 v1：WSS 握手、P-256 自证、消息、媒体、设备、审计
- 引用消息 `quote_ref` 全链路：
  QQ `message_scene.ext.msg_idx` → `quote_ref` → store schema v4 → 协议 →
  Manager → QQ `message_reference.message_id`
- 群成员禁言/解除/踢人（QQ 群管理接口）：
  - `/v2/groups/{group_openid}/restrict_chat_setting`
  - `/v2/groups/{group_openid}/batch_remove_members`
- 扫码配对：`mobile_scanner 5.2.3`，相机权限，扫码/粘贴自动解析
- 假 @ 已删除（QQ 群发送接口没有 mention 字段，不再发纯文本冒充 @）
- 发送队列：文本 + 多图/多文件逐条发送；输入框立即清空，失败不回填
- 图片消息不套蓝气泡；文件消息显示为文件卡片
- 群聊/私聊标识
- 通知：UI 与 `:background` 进程同一通知 ID 去重；系统默认通知音
- 独立后台进程 + 单一 WebSocket + 本地桥接（§一）；前台服务类型 `specialUse`
- 性能：去掉实时模糊、MaterialApp 全局重建、会话列表懒构建、tab 隔离重绘
- 保留策略：不主动限制消息时间/媒体总量（默认 100 年 / 1<<62 字节）
- 本地聊天记录：Hive 持久化，去掉 2000 条/2.5MB 上限；首次启动会迁移旧 shared_preferences 消息
- JKS 正式签名 + `Release` 稳定版 workflow

---

## 三、发布与签名

GitHub Secrets：

- `ANDROID_KEYSTORE_BASE64`（JKS 的 base64）
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

稳定版：手动运行 `.github/workflows/release.yml`，输入 `1.0.0`。
会发：

- `v1.0.0`：Service linux amd64 / arm64
- `manager-v1.0.0`：正式签名 arm64 APK

日常 CI：`.github/workflows/ci.yml`，只发 pre-release。
`workflow_dispatch` 会跑全部 job；路径检测已修成多行。

**本地不要直接在 `/storage/emulated/0` 跑 `flutter build apk`**：
`android/gradlew` 没有执行权限，Flutter 会报
`lacked sufficient permissions to execute`。而且这台机器上没有 Flutter/JDK，
构建只能走 CI。

---

## 四、还没验证/待办

1. 后台进程真机验证（§一末尾清单）：划掉任务、强停、锁屏、重启、OEM 省电
2. 真机引用是否显示；`quote_ref` 需要 Service v1.0.0+ 和 Manager v1.0.0
3. 扫码相机权限、配对
4. 禁言/踢人权限：踢人接口需要白名单/内邀；失败提示是否正确
5. 多图、文字+图片、文件发送
6. SQLite v3→v4、PostgreSQL v3→v4 升级测试
7. 安全日志分页：目前仍只加载最近 20 条，`audit.list` 已加 cursor 字段但
   Manager/SecurityLogPage 还没接完
8. x86_64 已不支持，只发 arm64，发布说明要写清楚
9. 后台进程的常驻通知目前没有「停用」入口；要不要在设置页给个开关需要产品拍板

---

## 五、重要约束

- 中文交流，中文注释，中文提交信息
- 不留假数据/假按钮；平台不支持就明确写不支持
- 改完至少 `flutter analyze`；涉及 Service 要跑 `go vet ./...`、`go test -race ./...`
  （本机没有 Flutter，靠 CI 的 `client-check` 兜）
- 不要重新引入 Spring/Java 后端，Service 是 Go
- 不要提交 `key.properties`、`*.jks`、token、keystore
- 发布前确认 CI 全绿；不要用 debug 签名发稳定版
- 推送到 GitHub：这台机器没有 ssh 客户端，用 HTTP + token
  （token 在 `~/newtoken.txt`，不要写进仓库、不要贴进提交信息）：
  ```bash
  TOKEN=$(tr -d '\r\n' < ~/newtoken.txt)
  git -c user.name=Bailensn \
      -c user.email=186718762+Bailensn@users.noreply.github.com \
      commit -m "..." 
  git push "https://${TOKEN}@github.com/Bailensn/LianT.git" HEAD:main
  ```

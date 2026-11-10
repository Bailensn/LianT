# 交接说明（给下一个会话）

仓库：`/storage/emulated/0/LianT`，分支 `main`。
Manager 版本：`1.0.0+100`，versionCode `100`。
稳定版已发布：Service `v1.0.0`、Manager `manager-v1.0.0`（正式签名、仅 arm64）。

先看 `git status`。工作区可能有一处未提交的
`manager/android/gradle.properties`，是 Flutter migrator 自动加的
`android.builtInKotlin=false`、`android.newDsl=false`，不要盲目提交。

本机没有 Flutter/JDK/Gradle，**不能本地构建**：改完等 CI（push 触发
`.github/workflows/ci.yml`）出包，从 Release 下载 APK。推送方式见 §五。

---

## 一、后台常驻：独立进程（已实现，待真机验证）

用户要求的是**独立进程**：UI 进程被系统杀掉后仍能保持 WebSocket、收消息、发通知。

### 现在的结构

```
UI 进程                          :background 进程（MessageService）
  MainActivity                     startForeground + 常驻通知
  FlutterEngine（跑 main）          FlutterEngine（跑 backgroundMain）
  LiantClient（给界面）             LiantClient（只负责收消息/发通知）
  写 liant_background_state.json   读 liant_background_state.json
```

- 清单里 `MessageService` 带 `android:process=":background"`、
  `stopWithTask="false"`、`foregroundServiceType="dataSync"`。
- Dart 入口 `backgroundMain` **放在 `lib/main.dart`**，不能挪走：
  AOT 运行时按函数名在**根库**（main.dart 所在库）里找入口；放在没人 import
  的文件里会被摇树删掉。函数上的 `@pragma('vm:entry-point')` 必须保留。
- 后台逻辑在 `lib/core/background_runtime_io.dart`（Web 用 `_stub.dart`，条件导出）。

### 跨进程状态（关键坑，已经踩过）

`shared_preferences` 在每个进程各有一份内存缓存，`reload()` 也不会真回读磁盘，
后台进程读到的经常是旧值。所以改成共享文件：

- 路径：`filesDir/liant_background_state.json`，由原生 `liant/background_state`
  通道的 `statePath` 提供（MainActivity 与 MessageService 都注册，两个进程
  拿到的是同一个文件）。
- UI 进程写（`lib/core/background_state_io.dart`）：内容 = 身份 + 前台标志 +
  免打扰 + 会话标题 + 备注；先写 `.tmp` 再 rename，写操作串行化。
- 后台进程读：每条消息前重读一次，免打扰/前台开关立刻生效；每分钟对一次，
  识别解除配对、重新配对。
- 文件存在即代表「已配对」，`RestartReceiver` 靠它决定开机要不要拉起服务。

### 通知去重

- 两边算同一个通知 ID：`notificationIdFor(msgId)`（protocol.dart，FNV-1a，
  不用 `String.hashCode`，避免跨进程/跨版本不一致）。
- 通知都带 `setOnlyAlertOnce(true)`，所以同一条消息最多响一次。
- 分工：UI 在前台时后台进程不弹（读共享状态里的 `foreground`）；
  UI 不在前台时两边都可能弹，靠上面的 ID 合并。

### 生命周期

- `START_STICKY`；系统重启服务时 `onStartCommand` 的 intent 可能为 null，引擎在
  `onCreate` 里恢复。
- `onTaskRemoved` 排一个 `setAndAllowWhileIdle` 闹钟（60 秒后）兜底拉起，
  拉起失败只记日志（Android 12+ 后台启动前台服务会被系统拒绝）。
- `RestartReceiver`：开机（BOOT_COMPLETED / QUICKBOOT_POWERON）、闹钟、
  常驻通知被划掉后的重发，三个来源共用。
- **UI 侧不要再在 `detached`/`dispose` 里停服务**：那样划掉任务就没后台了。
  只有 `unpair()` 才停服务 + 删共享状态。

### 已知限制（要跟用户说清楚）

- 两个进程各一条 WebSocket，服务端会看到同一设备两条连接（协议没有单会话限制，
  `Broadcast` 会都发）。内存比单进程多一个 FlutterEngine 的量级。
- Android 12+ 从后台拉起前台服务受限；OEM 省电策略下仍可能被杀，只能引导用户
  加白名单，不能承诺「永不杀」。
- `dataSync` 类型在 **targetSdk 35** 的 Android 15 上有每天 6 小时上限；本项目
  现在 targetSdk 仍是 Flutter 3.24 默认的 34，升级 targetSdk 前必须先重新设计
  前台服务类型。
- 证书/身份都是明文写在共享文件里（和 shared_preferences 一样在应用私有目录）。

### 真机验证清单

```bash
# 装包后：确认有两个进程
adb shell ps -A | grep bailensn        # 期望看到 :background 独立 PID
adb shell dumpsys activity services com.bailensn.liant | grep -A3 MessageService
adb shell dumpsys notification --noredact | grep -A5 liant

# 验证「UI 进程被杀后仍在线」
#   am kill 只杀可回收进程：UI 退到后台后会被杀，带前台服务的 :background 不会被杀
adb shell am kill com.bailensn.liant
#   更精确的做法：只杀 UI 进程的 PID
adb shell ps -A | grep bailensn | grep -v ':background'   # 取 PID
adb shell kill <刚才那个 PID>
#   或者：在最近任务里划掉 App
# 然后从另一台设备给 Bot 发消息：应收到通知
# 常驻通知文案会显示「已连接 · 正在后台接收消息」/「连接中断，正在重连…」

# 验证开机恢复
adb reboot   # 开机后不打开 App，直接发消息
```

---

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
- 独立后台进程（本页 §一）
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

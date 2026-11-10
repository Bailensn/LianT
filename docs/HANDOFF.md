# 交接说明（给下一个会话）

仓库：`/storage/emulated/0/LianT`，分支 `main`。
当前最新代码提交：`7d118fc`（只修 CI dispatch 路径检测）。
Manager 版本：`1.0.0+98`，versionCode `98`。
稳定版已发布：Service `v1.0.0`、Manager `manager-v1.0.0`（正式签名、仅 arm64）。

先看 `git status`。工作区可能有一处未提交的
`manager/android/gradle.properties`，是 Flutter migrator 自动加的
`android.builtInKotlin=false`、`android.newDsl=false`，不要盲目提交。

---

## 一、当前任务：真正的后台常驻进程

用户明确要求：**独立进程**，不是 UI 进程里的线程，也不是只挂一个通知。

目标行为：

- App 退到后台，UI 进程被系统杀掉后，仍能独立保持 WebSocket、收消息、发通知
- 不是依赖 Activity 或 UI isolate
- 前台服务通知保留；Android 不允许无通知的永久后台进程

建议实现：

1. `AndroidManifest.xml` 里 `MessageService` 增加
   `android:process=":background"`、`android:stopWithTask="false"`、
   `android:exported="false"`。
2. `MessageService` 独立进程里：
   - `startForeground` + 常驻通知
   - 初始化 `FlutterLoader`
   - 创建自己的 `FlutterEngine`
   - `GeneratedPluginRegistrant` / 插件注册
   - `MethodChannel("liant/background_notify")` 接收 Dart 通知请求
   - `DartExecutor.DartEntrypoint("lib/background_main.dart", "backgroundMain")`
3. `lib/background_main.dart` 独立连接 `LiantClient`：
   - 读设备身份
   - 监听 `msg.inbound`
   - 判断免打扰/前台状态
   - 调 MethodChannel 发系统通知
4. 生命周期：
   - `START_STICKY`
   - `onTaskRemoved` 里用 AlarmManager 定时尝试拉起
   - `BOOT_COMPLETED` receiver 开机恢复
   - Android 14 注意 FGS 类型与启动限制
5. 进程间状态（**关键坑**）：
   - `shared_preferences` 默认按进程缓存，独立进程读不到 UI 进程刚写的值
   - 不要在独立进程里直接依赖 `SharedPreferences.getInstance()` 的缓存
   - 方案 A：通过原生 `SharedPreferences("FlutterSharedPreferences", MODE_MULTI_PROCESS)`
     读取 `flutter.liant.identity` / `flutter.liant.foreground` / `flutter.liant.muted`
   - 方案 B：把身份、免打扰、前台标志改成原生可读的文件/DataStore，Dart 走 MethodChannel
   - 先确认 `shared_preferences` 实际写入文件名和 key 前缀，不要猜
6. 去重：
   - UI 和后台进程都收到同一条消息时，用同一个通知 ID
   - 通知加 `setOnlyAlertOnce(true)`，避免响两次
7. 通知权限：
   - `POST_NOTIFICATIONS` 需要用户授权
   - 通知渠道：`liant.notifications.v2`，系统默认通知音
8. OEM：
   - 只能引导电池优化白名单/自启动，不能承诺永不杀
   - 真机要测：划掉任务、锁屏、息屏、重启、强停

需要改的文件：

- `manager/android/app/src/main/kotlin/com/bailensn/liant/MessageService.kt`
- `manager/android/app/src/main/kotlin/com/bailensn/liant/MainActivity.kt`
- `manager/android/app/src/main/AndroidManifest.xml`
- `manager/lib/background_main.dart`
- `manager/lib/core/app_state.dart`
- `manager/lib/core/client.dart`
- 可能新增 `BootReceiver.kt`、`NotificationHelper.kt`

验证：

```bash
# 构建
cd manager
flutter analyze
flutter build apk --release --target-platform android-arm64

# 安装后检查进程
adb shell ps -A | grep bailensn
adb shell dumpsys activity services com.bailensn.liant
adb shell dumpsys notification --noredact | grep -A5 liant
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
- 通知：UI 与后台同 ID 兜底；系统默认通知音
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
`lacked sufficient permissions to execute`。要本地编就把仓库复制到
`$HOME/LianT` 之类的可执行分区，或直接用 CI。

---

## 四、还没验证/待办

1. 真机引用是否显示；`quote_ref` 需要 Service v1.0.0+ 和 Manager v1.0.0
2. 扫码相机权限、配对
3. 禁言/踢人权限：踢人接口需要白名单/内邀；失败提示是否正确
4. 后台通知、声音、划掉任务、重启、OEM 省电
5. 多图、文字+图片、文件发送
6. SQLite v3→v4、PostgreSQL v3→v4 升级测试
7. 安全日志分页：目前仍只加载最近 20 条，`audit.list` 已加 cursor 字段但
   Manager/SecurityLogPage 还没接完
8. x86_64 已不支持，只发 arm64，发布说明要写清楚
9. 本地构建只用 CI 或内部存储，不要用 sdcard 路径

---

## 五、重要约束

- 中文交流，中文注释，中文提交信息
- 不留假数据/假按钮；平台不支持就明确写不支持
- 改完至少 `flutter analyze`；涉及 Service 要跑 `go vet ./...`、`go test -race ./...`
- 不要重新引入 Spring/Java 后端，Service 是 Go
- 不要提交 `key.properties`、`*.jks`、token、keystore
- 发布前确认 CI 全绿；不要用 debug 签名发稳定版

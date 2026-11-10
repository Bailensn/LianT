# 构建与发布

## 工具链（CI 与本地都要对齐）

| 项 | 版本 | 备注 |
| --- | --- | --- |
| Flutter / Dart | 3.47.9 / 3.13.7 | CI 钉死；`pubspec.lock` 与之对齐 |
| AGP | 9.1.0 | Flutter 3.47.9 模板值 |
| Gradle | 9.3.1 | 用 wrapper |
| Kotlin | 2.4.0 | app 里用 `kotlin.compilerOptions` 定 jvmTarget |
| Java | 17 | `compileOptions` / jvmTarget |
| compileSdk / targetSdk | 36 / 36 | 跟随 `flutter.*` |
| minSdk | 24 | Flutter 3.47 下限 |

构建脚本是 Kotlin DSL（`android/build.gradle.kts`、`settings.gradle.kts`、`app/build.gradle.kts`）。

## 两个必须知道的坑

- **只打 arm64**：`packaging.jniLibs.excludes` 剔掉 `lib/x86*/`、`lib/armeabi-v7a/**`。
  `ndk.abiFilters` **无效**——`FlutterPlugin.configureAbiWithoutSplits()` 会在 afterEvaluate
  里把它重置成 Flutter 支持的全部 ABI。
- **插件 compileSdk**：老插件写的是 34，而 AndroidX 要求 36，AGP 9 的 AAR metadata 检查会失败。
  根 `build.gradle.kts` 统一把 `LibraryExtension.compileSdk` 覆盖成 36；这个 `afterEvaluate`
  必须放在 `evaluationDependsOn(":app")` **之前**。

## 命令

```bash
cd manager
flutter pub get
flutter analyze
flutter test
flutter build apk --release --target-platform android-arm64
# 产物：build/app/outputs/flutter-apk/app-release.apk
```

后台服务用 `foregroundServiceType="specialUse"`（`dataSync` 在 targetSdk 35+ 的 Android 15
上有每天 6 小时上限），清单里带 `PROPERTY_SPECIAL_USE_FGS_SUBTYPE`。

## CI 与发版

- `.github/workflows/ci.yml`：路径变更检测 → `service-check`（Go vet/test）、
  `client-check`（analyze + test）、`android`（出 arm64 APK）。push 时自动发 pre-release
  `manager-v<version>-<code>`。
- `.github/workflows/release.yml`：手动跑，输入版本号，发 Service 二进制与正式签名 APK。
- 签名 secrets：`ANDROID_KEYSTORE_BASE64`、`ANDROID_KEYSTORE_PASSWORD`、
  `ANDROID_KEY_ALIAS`、`ANDROID_KEY_PASSWORD`。没有 keystore 时 release 走 debug 签名，
  只能测试用。
- 版本号：`manager/pubspec.yaml` 的 `+build` 与 `app/build.gradle.kts` 的
  `val liantVersionCode` 必须一致，每次发版 +1。

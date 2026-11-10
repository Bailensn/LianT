# 部署与构建

## 1. 服务端前置

一台能出公网的 Linux 机器（x86_64 或 arm64），跑一个 LianT Bot Engine 进程。
不用 Docker：二进制放 `/opt/liant`，用 systemd 常驻。服务端只有 API，客户端是 `manager/` 的 Flutter 应用。

- 固定端口 **8443/tcp**：`/v1/ws` 与 `/v1/media/...` 共用；
- 磁盘：消息库很小，媒体按单 Bot 3 GB 规划；
- 时间同步：认证要求时差 ≤ 60 秒。

二进制从 `Service` workflow 产物下载：`LianT-linux-amd64` 或 `LianT-linux-arm64`，附 `SHA256SUMS.txt`。

## 2. 安装

```bash
useradd -r -s /sbin/nologin liant || true
mkdir -p /opt/liant && cd /opt/liant
mv /path/to/LianT-linux-amd64 ./LianT && chmod +x ./LianT
sha256sum -c SHA256SUMS.txt --ignore-missing

./LianT init
./LianT config set connect.listen :8443
```

或者用仓库脚本（交互式会问目录、端口、用户，回车用默认值）：

```bash
sudo bash scripts/install.sh
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | sudo bash -s -- -y
sudo bash scripts/install.sh --dir /srv/liant --port 9443 --user liant
sudo bash scripts/install.sh --beta        # 默认只装稳定版，Service 目前只发 pre-release
```

开机自启：

```bash
cp /opt/liant/deploy/liant.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now liant
journalctl -u liant -f
```

## 3. 登记 Bot

Bot 管理全在客户端完成，命令行没有 `bot` 子命令：

1. 配对成功后打开「机器人」页 → 右上角「添加」；
2. 填 AppID 与 AppSecret（来自 QQ 开放平台机器人后台），可选备注名；
3. 保存时向 QQ 开放平台校验凭据并读取昵称，失败直接提示原因；
4. 回列表点「启动」，状态变「运行中」即可收消息。

凭据由适配器校验后加密入库，接口不回显，客户端也不会再读出来。服务重启会自动认领已登记的 Bot。

## 4. 配对手机

```bash
./LianT pair                              # 自动探测本机 IPv4，适合同一网络
./LianT pair --host 1.2.3.4               # 指定手机可达的地址
./LianT pair --host liant.example.com     # 有域名/反代时
./LianT pair --host liant.example.com --port 443
```

二维码的 `host` 按「`--host` > `config.connect.address` > 本机非回环 IPv4 > 主机名」取值；
自动探测只适合局域网，跨网必须显式给 `--host` 或先 `config set connect.address …`。
配对成功（`pair` 不会退出，会继续以服务模式运行，Ctrl+C 停止）。

终端打印配对码、节点指纹与配对信息 JSON，并渲染二维码（`data/pair-qr.png` 也留一份）。
客户端「连接 LianT」页可以**扫码**，也可以把 JSON 粘贴进输入框。
手机连上后终端要求输入 `y` 确认，成功即把设备公钥写入 `devices` 表；
配对码 60 秒有效、一次性，默认最多 8 台设备。

Android 端允许自签证书（TLS 只做传输，身份由 `auth.challenge` 的指纹与签名校验）；
Web 端仍要系统信任证书，生产环境建议用反代换正式证书。

## 5. 客户端（Android 唯一交付目标）

工具链（CI 与本地都要对齐）：

| 项 | 版本 |
| --- | --- |
| Flutter / Dart | 3.47.9 / 3.13.7 |
| AGP / Gradle | 9.1.0 / 9.3.1（用 wrapper） |
| Kotlin | 2.4.0（app 里用 `kotlin.compilerOptions` 定 jvmTarget） |
| Java | 17 |
| compileSdk / targetSdk / minSdk | 36 / 36 / 24 |

构建脚本是 Kotlin DSL。命令：

```bash
cd manager
flutter pub get && flutter analyze && flutter test
flutter build apk --release --target-platform android-arm64
# 产物：build/app/outputs/flutter-apk/app-release.apk
```

**两个必须知道的坑**：

- **只打 arm64**：用 `packaging.jniLibs.excludes` 剔掉 `lib/x86*/`、`lib/armeabi-v7a/**`。
  `ndk.abiFilters` **无效**——`FlutterPlugin.configureAbiWithoutSplits()` 会在 afterEvaluate 里
  把它重置成 Flutter 支持的全部 ABI。
- **插件 compileSdk**：老插件写的是 34，而 AndroidX 要求 36，AGP 9 的 AAR metadata 检查会失败。
  根 `build.gradle.kts` 统一把 `LibraryExtension.compileSdk` 覆盖成 36，且这个 `afterEvaluate`
  必须放在 `evaluationDependsOn(":app")` **之前**。

另外 AGP 9 默认 `extractNativeLibs=false`，APK 会大一倍，用 `packaging.jniLibs.useLegacyPackaging = true` 换回压缩。
应用 ID `com.bailensn.liant`；Web 只用于本地预览（`flutter build web --release` 后起个本地静态服务），
不对外发布；不做桌面端。

## 6. CI 与发版

- `.github/workflows/ci.yml`：路径变更检测 → `service-check`（Go vet/test）、`client-check`
  （analyze + test）、`android`（arm64 APK）。push 时自动发 pre-release `manager-v<version>-<code>`。
- `.github/workflows/release.yml`：手动跑，输入版本号，发 Service 二进制与正式签名 APK
  （`v<version>` + `manager-v<version>`）。
- 签名 secrets：`ANDROID_KEYSTORE_BASE64`、`ANDROID_KEYSTORE_PASSWORD`、
  `ANDROID_KEY_ALIAS`、`ANDROID_KEY_PASSWORD`。没有 keystore 时 release 走 debug 签名，只能测试用。
- 版本号：`manager/pubspec.yaml` 的 `+build` 与 `app/build.gradle.kts` 的 `val liantVersionCode`
  必须一致，每次发版 +1。

## 7. 常驻运行

```bash
./LianT serve        # 前台
./LianT serve -d     # 后台：日志 data/liant.log，pid data/liant.pid
./LianT stop
```

启动时认领状态为 `running/starting` 的 Bot，失败按 1s→30s 退避重试；退出（SIGINT/SIGTERM）停掉全部 Bot。
systemd 示例见 `deploy/liant.service`。

## 8. 保留与清理

- 消息与媒体默认保留 **7 天**，配置下限也是 7 天（只能调大）；
- 单 Bot 媒体超 **3 GB** 时按消息时间从旧到新淘汰；
- `serve` 每小时清理一次：删消息、标媒体过期/淘汰，并**真正删除**磁盘文件；
- 客户端本地不限条数，是否清理由用户决定（解除配对会清空）。

## 9. 升级与回滚

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
sudo bash scripts/upgrade.sh --dir /srv/liant     # 自定义安装目录
sudo bash scripts/upgrade.sh --beta               # 测试版
```

脚本会停服务、校验 SHA256、备份旧二进制、重启；旧版本留在 `<安装目录>/LianT.bak`，启动失败自动回滚。

- 数据库有 `PRAGMA user_version` 迁移，启动时自动升级；升级前先备份 `data/`；
- 节点密钥与证书在 `data/node/`：证书剩余不足 30 天会自动用同一私钥重签，指纹不变；
- 回滚到旧版本前，先确认旧版本能读新 schema，否则用备份恢复。

## 10. 卸载

```bash
sudo bash scripts/uninstall.sh                 # 连配置、消息、媒体一起删
sudo bash scripts/uninstall.sh --keep-data     # 只删服务与二进制
```

卸载会停止并移除 systemd 单元，默认删除整个安装目录（含 `config.enc` 与 `data/`）。
脚本拒绝 `/`、`/opt`、`/home/<user>` 这类危险路径；自定义目录用 `--dir` 指定，
或让脚本从 systemd 单元读取。

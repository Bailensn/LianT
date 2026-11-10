# 部署与构建

## 1. 服务端前置

一台能出公网（或至少有公网入口）的 Linux 机器，跑一个 LianT Bot Engine 进程。
不用 Docker：二进制放进 `/opt/liant`，用 systemd 常驻。服务端只提供 API，客户端是 `manager/` 的 Flutter 应用。

| 项 | 要求 |
| --- | --- |
| 系统 | Linux x86_64 或 arm64 |
| 端口 | 固定 **8443/tcp**，`/v1/ws` 与 `/v1/media/...` 共用（`connect.listen` 可改） |
| 磁盘 | 消息库很小；媒体按**单 Bot 3 GB** 规划（3 个 Bot 最多 9 GB） |
| 时间 | 认证要求时差 ≤ 60 秒，装 chrony/systemd-timesyncd |

二进制从正式 Release 下载：`LianT-linux-amd64` 或 `LianT-linux-arm64`，附 `SHA256SUMS.txt`。
**正式版是一个 Release**（tag `v<version>`），里面同时有 Service 二进制和 Manager APK。

## 2. 数据库

两种，`storage.driver` 决定：

| 驱动 | 说明 |
| --- | --- |
| `sqlite`（`LianT init` 默认） | 单文件 `data/bots.db`，零依赖，适合单机自托管 |
| `postgres`（`scripts/install.sh` 默认） | 需要角色与库；多实例或想用外部备份时选它 |

PostgreSQL 相关配置键：`storage.pg_host / pg_port / pg_user / pg_password / pg_database / pg_sslmode`。
库要自己建：

```sql
CREATE ROLE "liant" LOGIN PASSWORD '…';
CREATE DATABASE liant OWNER "liant";
```

两者都有 `user_version` 迁移，启动时自动升级。

## 3. 安装

手动：

```bash
useradd -r -s /sbin/nologin liant || true
mkdir -p /opt/liant && cd /opt/liant
mv /path/to/LianT-linux-amd64 ./LianT && chmod +x ./LianT
sha256sum -c SHA256SUMS.txt --ignore-missing

./LianT init                              # 生成 config 与密钥，建库
./LianT config set connect.listen :8443
```

`LianT init` 首次运行写入默认配置（sqlite、`data/bots.db`、`data/key/master.key`、监听 `:8443`），
并跑一次数据库迁移；重复执行是安全的。

脚本安装（交互式会问目录、端口、用户，回车用默认值）：

```bash
sudo bash scripts/install.sh
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | sudo bash -s -- -y
```

脚本参数（也可用同名环境变量 `LIANT_*`）：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--dir PATH` | `/opt/liant` | 安装目录 |
| `--port PORT` | `8443` | 服务端口 |
| `--user USER` | `liant` | 运行服务的用户 |
| `--url URL` | 空 | 从指定地址下载二进制 |
| `--beta` | 关 | 接收测试版（pre-release）；默认只装稳定版 |
| `--db TYPE` | `postgres` | `postgres` 或 `sqlite` |
| `--db-host/--db-port/--db-name/--db-user/--db-pass` | `127.0.0.1`/`5432`/`liant`/`liant`/空 | PostgreSQL 连接信息 |

安装脚本会写好 systemd 单元、拉起服务并打印状态。`Service` 目前只发 pre-release，
所以装测试版必须显式 `--beta`。

**开机自启**（脚本已做；手动安装时）：

```bash
cp /opt/liant/deploy/liant.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now liant
journalctl -u liant -f
```

`deploy/liant.service` 的内容与两个关键点：

```ini
[Service]
Type=simple
WorkingDirectory=/opt/liant
ExecStart=/opt/liant/LianT serve
Restart=always
RestartSec=3
User=root                      # 建议改成 liant
Environment=LIANT_CONFIRM=auto
Environment=TZ=Asia/Shanghai
```

- `User=` 改成独立用户，并让 `data/` 归它所有。
- `LIANT_CONFIRM=auto` 表示**无终端时自动批准配对**并写审计日志。
  配对码只可能由 `LianT pair` 生成并显示在终端，拿着它来配对本身已意味着操作者确认；
  想强制在终端敲 `y` 就删掉这行（`LIANT_CONFIRM=terminal`）。

## 4. 登记 Bot

Bot 管理全在客户端完成，命令行没有 `bot` 子命令：

1. 配对成功后打开「机器人」页 → 右上角「添加」；
2. 填 AppID 与 AppSecret（来自 QQ 开放平台机器人后台），可选备注名；
3. 保存时向 QQ 开放平台校验凭据并读取机器人昵称，失败直接提示原因；
4. 回列表点「启动」，状态变「运行中」即可收消息。

凭据由适配器校验后加密入库（`data/key/master.key`），接口不回显，客户端也不会再读出来。
`serve` 启动时会认领状态为 `running/starting` 的 Bot，不需要人工启动。

## 5. 配对手机

```bash
./LianT pair                              # 自动探测本机 IPv4，适合手机与服务器同一网络
./LianT pair --host 1.2.3.4               # 指定手机可达的地址
./LianT pair --host liant.example.com     # 有域名/反代时
./LianT pair --host liant.example.com --port 443   # 反代对外是 443
```

二维码/JSON 里的 `host` 取值优先级：
**`--host` > `config.connect.address` > 本机非回环 IPv4 > 主机名**。
自动探测只适合局域网；跨网必须显式给 `--host`，或先 `config set connect.address …`。

终端会打印配对码、节点指纹与配对信息 JSON，并渲染二维码（`data/pair-qr.png` 也留一份）。
客户端「连接 LianT」页可以**扫码**，也可以把 JSON 粘贴进输入框。
手机连上后服务端按 §3 的 `LIANT_CONFIRM` 策略确认，成功即把设备公钥写入 `devices` 表。

- 配对码 **60 秒有效、一次性**；默认最多 **8 台**设备；过期或错码重新 `LianT pair`。
- `pair` **不会退出**，会继续以服务模式运行（Bot 也被认领），所以之后不必再单独 `serve`；
  停止按 Ctrl+C。
- 可以限制允许配对的设备名（comma 分隔 glob，留空不限制）：`config set pair.allow "iPhone*,Pixel*"`。
- Android 端允许自签证书（TLS 只做传输，身份由握手时 `auth.challenge` 的指纹与签名校验）；
  Web 端仍要系统信任证书，生产环境建议用反代换正式证书。

## 6. 客户端（Android 唯一交付目标）

工具链（CI 与本地都要对齐）：

| 项 | 版本 | 备注 |
| --- | --- | --- |
| Flutter / Dart | 3.47.9 / 3.13.7 | CI 钉死，`pubspec.lock` 与之对齐 |
| AGP | 9.1.0 | Flutter 3.47.9 模板值 |
| Gradle | 9.3.1 | 用 wrapper |
| Kotlin | 2.4.0 | app 里用 `kotlin.compilerOptions` 定 jvmTarget |
| Java | 17 | `compileOptions` / jvmTarget |
| compileSdk / targetSdk | 36 / 36 | 跟随 `flutter.*` |
| minSdk | 24 | Flutter 3.47 的下限 |

构建脚本是 Kotlin DSL（`android/build.gradle.kts`、`settings.gradle.kts`、`app/build.gradle.kts`）。

```bash
cd manager
flutter pub get
flutter analyze
flutter test
flutter build apk --release --target-platform android-arm64
# 产物：build/app/outputs/flutter-apk/app-release.apk
```

**三个必须知道的坑**：

1. **只打 arm64**：用 `packaging.jniLibs.excludes` 剔掉 `lib/x86*/`、`lib/armeabi-v7a/**`。
   `ndk.abiFilters` **无效**——`FlutterPlugin.configureAbiWithoutSplits()` 会在 afterEvaluate
   里把它重置成 Flutter 支持的全部 ABI。不裁的话 ML Kit、CameraX、DataStore、dartjni 各带一套。
2. **插件 compileSdk**：老插件写 34，而 AndroidX 要求 36，AGP 9 的 AAR metadata 检查直接失败。
   根 `build.gradle.kts` 统一把 `LibraryExtension.compileSdk` 覆盖成 36，且这个 `afterEvaluate`
   必须放在 `evaluationDependsOn(":app")` **之前**，否则报
   「Cannot run Project.afterEvaluate when the project is already evaluated」。
3. **.so 压缩**：AGP 9 默认 `extractNativeLibs=false`（不压缩），APK 会大一倍；
   `packaging.jniLibs.useLegacyPackaging = true` 换回压缩（12 MB 级），侧载分发更在意下载体积。

其它：应用 ID `com.bailensn.liant`，已带 `INTERNET` 权限；签名从 `key.properties` 读
（`storeFile / storePassword / keyAlias / keyPassword`），没有就走 debug 签名、只能测试用。
不要提交 `key.properties` 与 `*.jks`。

**本地别在 `/storage/emulated/0` 跑 `flutter build apk`**：`android/gradlew` 在该分区没有执行权限，
Flutter 会报 `lacked sufficient permissions to execute`。构建走 CI。

**Web 只用于本地预览，不对外发布**：

```bash
cd manager && flutter build web --release
cd build/web && python3 -m http.server 8081 --bind 127.0.0.1
```

没有 gh-pages 或任何公开地址；网页版连自签服务端同样受证书信任限制。不做桌面端。

## 7. CI 与发版

| workflow | 触发 | 做什么 |
| --- | --- | --- |
| `ci.yml` | push（`docs/**`、`**/*.md` 以外的改动）、PR、手动 | 路径检测 → `service-check`（协议漂移检查 + `go vet` + `go test -race`）、`client-check`（`flutter analyze` + `test`）、`android`（arm64 APK）。push 时自动发 pre-release（只有 APK，测试用） |
| `release.yml` | 手动，输入版本号 | 发**一个** Release `v<version>`：`LianT-linux-amd64`、`LianT-linux-arm64`、`LianT-android-arm64.apk`、`SHA256SUMS.txt` |

**一个正式版 = 一个 Release。** 不要再把 Service 与 Manager 分成两个 tag 发：
用户装服务端和客户端应该来自同一次发布，版本号天然对齐。

- **pre-release 去重**：`ci.yml` 用 `manager-v<version>-<code>` 当 tag，已存在就跳过，
  所以同一个 versionCode 重复 push 不会产生重复 Release。
- 签名 secrets：`ANDROID_KEYSTORE_BASE64`（JKS 的 base64）、`ANDROID_KEYSTORE_PASSWORD`、
  `ANDROID_KEY_ALIAS`、`ANDROID_KEY_PASSWORD`。
- **版本号两处必须一致**：`manager/pubspec.yaml` 的 `version: 1.0.0+106` 与
  `manager/android/app/build.gradle.kts` 的 `val liantVersionCode = 106`；每次发版 +1。
  `release.yml` 的第一步就会校验这两处与输入的版本号，不一致直接失败，
  所以「改一处就发版」不会混过去。
- 正式发版步骤：确认 CI 全绿 → 改两处版本号（前缀一起改，例如 1.0.1 / 107）并提交 →
  手动跑 `release.yml` 输入同一个版本号 → 检查那个 Release 里四个产物都在、
  签名不是 debug（`release.yml` 在缺 keystore 时会直接失败）。
- 只发 arm64：x86_64 与 universal 已不再提供，发布说明要写清楚。

## 8. 常驻运行

```bash
./LianT serve        # 前台
./LianT serve -d     # 后台：日志 data/liant.log，pid data/liant.pid
./LianT stop         # 停止后台服务
```

- 启动时认领 `running/starting` 的 Bot，失败按 1s→30s 退避重试；
- 退出（SIGINT/SIGTERM）时停掉全部 Bot；
- `serve --no-tls`（或 `LIANT_NO_TLS=1`）用于反代终端 TLS 的部署，见 [PROTOCOL.md](PROTOCOL.md) §11。

## 9. 保留与清理

- 消息与媒体默认保留 **7 天**，配置下限也是 7 天（只能调大）；
- 单 Bot 媒体超 **3 GB** 时按消息时间从旧到新淘汰；
- `serve` 每小时清理一次：删消息、标媒体过期/淘汰，并**真正删除**磁盘文件；
- 客户端本地不限条数，是否清理由用户决定（解除配对会清空本地）。
- 备份：DB（或 `data/bots.db`）+ `data/key/master.key` + `data/node/`；媒体可选。
  **主密钥丢了等于所有凭据作废**，要一起备。

## 10. 升级与回滚

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
sudo bash scripts/upgrade.sh --dir /srv/liant     # 自定义安装目录
sudo bash scripts/upgrade.sh --beta               # 接受测试版
sudo bash scripts/upgrade.sh ./LianT-linux-amd64  # 直接给二进制
```

脚本会停服务、校验 SHA256、备份旧二进制、重启；旧版本留在 `<安装目录>/LianT.bak`，
启动失败自动回滚。默认从 systemd 单元的 `WorkingDirectory` 找安装目录。

- 数据库启动时自动迁移；**升级前先备份 `data/`**。
- 节点密钥与证书在 `data/node/`：证书剩余不足 30 天会自动用同一私钥重签，**指纹不变**，
  客户端不用重新配对。
- 回滚到旧版本前，先确认旧版本能读新 schema，否则用备份恢复。

## 11. 卸载

```bash
sudo bash scripts/uninstall.sh                 # 连配置、消息、媒体一起删
sudo bash scripts/uninstall.sh --keep-data     # 只删服务与二进制
sudo bash scripts/uninstall.sh --keep-user     # 保留服务用户
```

卸载会停止并移除 systemd 单元，默认删除整个安装目录（含 `config.enc` 与 `data/`）与服务用户。
脚本拒绝 `/`、`/opt`、`/home/<user>` 这类危险路径；自定义目录用 `--dir` 指定，
或让脚本从 systemd 单元读取。PostgreSQL 的库与角色不会被自动删除，需要的话自己 `psql`。

## 12. 排障

| 现象 | 先查什么 |
| --- | --- |
| 客户端连不上 | 端口是否放行（8443/tcp）、`connect.listen`、反代是否转发 WS Upgrade |
| 认证总失败 | 两端时间差是否 ≤60 秒；指纹是否在配对后被换过（换密钥 = 换身份） |
| 浏览器报证书错 | 浏览器不认自签证书；换正式证书或反代，客户端能连不代表浏览器能连 |
| 启用了 Basic 认证后手机连不上 | 客户端握手不带 `Authorization`；改用反代层做访问控制 |
| Bot 启动失败 | 看 `data/liant.log` 与客户端「机器人」页报错；QQ 凭据、白名单、群权限 |
| 图片打不开 | 可能已过 7 天（`media_expired`）或超 3 GB 被淘汰（`media_evicted`），不是 bug |
| 配对码无效 | 60 秒过期或已用过；重新 `LianT pair` |

日志：`serve -d` 写 `data/liant.log`；systemd 部署看 `journalctl -u liant -f`。

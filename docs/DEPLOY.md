# 部署 Service

面向自托管：一台能出公网（或至少有公网入口）的 Linux 机器，跑一个 LianT Bot Engine 进程。

不用 Docker：二进制放进 `/opt/liant`，用 systemd 常驻。服务端只提供 API，
客户端是 `manager/` 里的 Flutter 应用。

## 1. 前置

- Linux x86_64 或 arm64，一个能跑二进制的用户；
- 固定端口 **8443/tcp**：`/v1/ws` 与 `/v1/media/...` 共用；
- 磁盘：消息库很小，媒体按单 Bot 3 GB 规划；
- 时间同步：认证要求时差 ≤ 60 秒。

二进制从 CI 的 `Service binaries` 产物里下：`LianT-linux-amd64` 或 `LianT-linux-arm64`，
附 `SHA256SUMS.txt`。

## 2. 安装

```bash
useradd -r -s /sbin/nologin liant || true
mkdir -p /opt/liant && cd /opt/liant
mv /path/to/LianT-linux-amd64 ./LianT && chmod +x ./LianT
sha256sum -c SHA256SUMS.txt --ignore-missing

./LianT init
./LianT config set connect.listen :8443
```

也可以直接跑仓库里的安装脚本：

```bash
sudo bash scripts/install.sh /path/to/LianT-linux-amd64
```

开机自启用仓库里的 `deploy/liant.service`：

```bash
cp /opt/liant/deploy/liant.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now liant
journalctl -u liant -f
```

## 3. 登记 Bot

机器人管理全部在 Flutter 客户端里完成，命令行不再提供 `bot` 子命令：

1. 客户端配对成功后，打开「机器人」页；
2. 点右上角「添加」，填 AppID 与 AppSecret，可选一个备注名；
3. 保存时会向 QQ 开放平台校验凭据并读取机器人昵称，失败会直接提示原因；
4. 回到列表点「启动」，状态变成「运行中」即可收消息。

凭据由适配器校验后加密入库，接口不回显，客户端也不会再把它读出来。
QQ 的 `app_id/app_secret` 来自 QQ 开放平台机器人后台。

服务重启后会自动认领已登记的 Bot，不需要人工启动。

## 4. 配对手机

```bash
./LianT pair                    # 自动探测本机 IPv4，适合手机和服务器同一网络
./LianT pair --host 1.2.3.4     # 指定手机可达的地址
./LianT pair --host liant.example.com   # 有域名/反代时
```

配对成功后 `pair` **不会退出**，会继续以服务模式运行（Bot 也会被认领），
所以之后不需要再单独 `serve`；要停止按 Ctrl+C。

二维码的 `host` 按「`--host` > `config.connect.address` > 本机非回环 IPv4 > 主机名」取值；
自动探测只适合局域网，跨网必须显式给 `--host` 或先 `LianT config set connect.address …`。

终端会打印配对码、节点指纹和**配对信息 JSON**，并渲染成二维码
（`data/pair-qr.png` 也保留一份）。在客户端「连接 LianT」页：

- 把配对信息粘贴到输入框，点「用配对码连接」；或
- 扫码后把内容粘进去（客户端 v1 还没有内置扫码器）。

手机连上后服务端终端会要求输入 `y` 确认，成功即把设备公钥写入 `devices` 表。
配对码 60 秒有效、一次性；默认最多 8 台设备。

> 客户端第一次连接时要信任服务端证书。自签证书在 Android 端需要装进系统信任库，
> 浏览器则要先用 https 访问一次并手动信任；生产环境建议用反代换成正式证书。

## 5. 客户端

**Android 是交付目标：**

- 本地构建：`cd manager && flutter build apk --release`；
- CI 产物拆成三个（release，签名仍是 debug key）：
  `LianT-android-arm64`、`LianT-android-x86_64`、`LianT-android-universal`；
- 应用 ID `com.bailensn.liant`，已经带 `INTERNET` 权限；
- v1 还没有正式分发的签名配置，正式上架前需要接 signingConfig。

**Web 只用于本地预览，不对外发布：**

```bash
cd manager
flutter build web --release
cd build/web && python3 -m http.server 8081 --bind 127.0.0.1
```

没有 gh-pages 或任何公开地址；网页版连接自签服务端时同样受证书信任限制。

不做 Linux/Windows/macOS 桌面端：v1 的交付物只有 Android APK。

客户端本地保存消息（上限 2000 条）、备注、已读与主动消息开关；服务端只保留 7 天。
「设置」页可以看已配对设备、吊销设备、安全日志与服务端能力。
服务端根路径现在只返回一行纯文本说明，不再有网页管理端。

## 6. 常驻运行

```bash
./LianT serve        # 前台
./LianT serve -d     # 后台：日志 data/liant.log，pid data/liant.pid
./LianT stop         # 停止后台服务
```

启动时会认领状态为 `running/starting` 的 Bot，失败按 1s→30s 退避重试；
退出（SIGINT/SIGTERM）时停掉全部 Bot。

systemd 示例见 `deploy/liant.service`。

## 7. 保留与清理

- 消息与媒体默认保留 **7 天**，配置下限也是 7 天（只能调大）；
- 单 Bot 媒体超过 **3 GB** 时按消息时间从旧到新淘汰；
- `serve` 每小时执行一次清理：删消息、标媒体过期/淘汰，并**真正删除**磁盘文件；
- 客户端本地另有上限（消息 2000 条），是否清理由用户在客户端决定。

## 8. 升级与回滚

- 数据库有 `PRAGMA user_version` 迁移，启动时自动升级；升级前先备份 `data/`；
- 节点密钥与证书在 `data/node/`：证书剩余不足 30 天会自动用同一私钥重签，指纹不变；
- 回滚到旧版本前，先确认旧版本能读新 schema，否则用备份恢复。

# 部署 Service

面向自托管：一台能出公网（或至少有公网入口）的 Linux 机器，跑一个 `LianT` 进程。

## 1. 前置

- Go 1.27+（或用 CI 产物 `LianT-Service-<os>-<arch>`）；
- 固定端口 **8443/tcp**：`/v1/ws`（客户端 WSS）与 `/v1/media/...`（媒体下载）共用；
- 磁盘：消息库很小，媒体按 **单 Bot 3 GB** 规划（3 个 Bot 约 9 GB 上限）；
- 时间同步：认证要求客户端与服务端时差 ≤ 60 秒。

## 2. 首次初始化

```bash
./LianT init          # 生成 config.enc 与 data/key/master.key
./LianT config get    # 确认 storage.database / connect.listen
```

`data/key/master.key` 是数据解密密钥：**丢了就解不开凭据与配置**。
备份只包含 `config.enc`、`data/`（数据库 + 主密钥）；`data/media/` 可选。

## 3. 登记 Bot

```bash
./LianT bot add --platform qqbot \
  --cred app_id=123456789 --cred app_secret=xxxx \
  --label "主号"
./LianT bot list
```

凭据由适配器校验后加密入库，接口不回显。QQ 的 `app_id/app_secret` 来自 QQ 开放平台机器人后台。

登记后 Bot 状态是 `starting`：`serve` 启动时会自动认领。如果 `serve` 已经在跑，
在 App 的 Bot 页点「启动」即可（走 `bot.start`，不用重启服务）。

## 4. 配对手机

```bash
./LianT pair                    # 自动探测本机 IPv4，适合手机和服务器同一网络
./LianT pair --host 1.2.3.4     # 指定手机可达的地址
./LianT pair --host liant.example.com   # 有域名/反代时
```

二维码里的 `host` 按「`--host` > `config.connect.address` > 本机非回环 IPv4 > 主机名」取值；
自动探测只适合局域网，跨网必须显式给 `--host` 或先 `LianT config set connect.address …`。

终端会打印配对码、节点指纹和一段二维码 JSON；在 Manager 里粘贴 JSON（或以后扫码）。
手机连上后终端要求输入 `y` 确认，成功即把设备公钥写入 `devices` 表并退出。
配对码 60 秒有效、一次性；默认最多 8 台设备。

> 指纹用于客户端钉证书。**换节点密钥等于换身份**，所有手机都要重新配对。

## 5. 常驻运行

```bash
./LianT serve
```

启动时会认领状态为 `running/starting` 的 Bot，失败按 1s→30s 退避重试；
退出（SIGINT/SIGTERM）时停掉全部 Bot。

systemd 示例：

```ini
[Unit]
Description=LianT Service
After=network-online.target

[Service]
Type=simple
User=liant
WorkingDirectory=/opt/liant
ExecStart=/opt/liant/LianT serve
Restart=on-failure
RestartSec=5
# 只让数据目录可写
ProtectSystem=strict
ReadWritePaths=/opt/liant/data /opt/liant/config.enc

[Install]
WantedBy=multi-user.target
```

## 6. 保留与清理

- 消息与媒体默认保留 **7 天**，配置下限也是 7 天（只能调大）；
- 单 Bot 媒体超过 **3 GB** 时按消息时间从旧到新淘汰；
- `serve` 每小时执行一次清理：删消息、标媒体过期/淘汰，并**真正删除**磁盘文件；
- 客户端本地不设上限，是否清理由用户在 App 里决定。

## 7. 升级与回滚

- 数据库有 `PRAGMA user_version` 迁移，启动时自动升级；升级前先备份 `data/`；
- 节点密钥与证书在 `data/node/`：证书剩余不足 30 天会自动用同一私钥重签，指纹不变；
- 回滚到旧版本前，先确认旧版本能读新 schema，否则用备份恢复。

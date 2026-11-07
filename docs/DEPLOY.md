# 部署 Service

面向自托管：一台能出公网（或至少有公网入口）的 Linux 机器，跑一个 `LianT` 容器。

最快的方式是 [DOCKER.md](DOCKER.md) 里的一键脚本；这篇讲不用 Docker 的手动部署。

## 1. 前置

- Docker（推荐），或本机装 Go 1.27+ 自己 `go build ./cmd/liant`；
- 固定端口 **8443/tcp**：`/v1/ws`（客户端 WSS）与 `/v1/media/...`（媒体下载）共用；
- 磁盘：消息库很小，媒体按 **单 Bot 3 GB** 规划（3 个 Bot 约 9 GB 上限）；
- 时间同步：认证要求客户端与服务端时差 ≤ 60 秒；
- 确认版本：`./LianT version`（发布产物由 tag 注入，本地 `go build` 显示 `dev`）。

## 2. 首次初始化

```bash
./LianT init          # 生成 config.enc 与 data/key/master.key
./LianT config get    # 确认 storage.database / connect.listen
```

`data/key/master.key` 是数据解密密钥：**丢了就解不开凭据与配置**。
备份只包含 `config.enc`、`data/`（数据库 + 主密钥）；`data/media/` 可选。

## 3. 登记 Bot

机器人管理全部在网页端完成，命令行不再提供 `bot` 子命令：

1. 打开管理页，进「机器人」；
2. 点右上角「添加」，填 AppID 与 AppSecret，可选一个备注名；
3. 保存时会向 QQ 开放平台校验凭据并读取机器人昵称，失败会直接提示原因；
4. 回到列表点「启动」，状态变成「运行中」即可收消息。

凭据由适配器校验后加密入库，接口不回显，网页端也不会再把它读出来。
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

二维码里的 `host` 按「`--host` > `config.connect.address` > 本机非回环 IPv4 > 主机名」取值；
自动探测只适合局域网，跨网必须显式给 `--host` 或先 `LianT config set connect.address …`。

终端会打印配对码、节点指纹、管理页链接和二维码。手机扫二维码直接用浏览器打开
`https://<host>:8443/#pair=…`，页面会自动填好配对信息，点「连接」即可；
首次打开要手动信任一次自签证书。也可以把链接或二维码 JSON 粘到配对页里。

手机连上后终端要求输入 `y` 确认，成功即把设备公钥写入 `devices` 表。
配对码 60 秒有效、一次性；默认最多 8 台设备。

> 浏览器不能做证书钉扎，网页端的安全模型是「首次信任 + 同源」：
> 页面和 WSS 走同一个端口、同一张证书，必须先信任证书才能打开页面。
> 换节点密钥等于换身份，所有设备都要重新配对。

## 5. 管理页

管理页内嵌在 Service 二进制里，不需要单独部署：

```
https://<host>:8443/
```

- 会话、聊天、机器人、设置四个页面；
- 设备私钥由浏览器 Web Crypto 生成，不可导出，只存在本机 IndexedDB；
- 断线自动重连，未读与历史留在本机；
- 想给页面换域名/端口，用反代转发到 8443 即可，注意 WebSocket 也要转发。

## 6. 常驻运行

```bash
./LianT serve        # 前台
./LianT serve -d     # 后台：日志 data/liant.log，pid data/liant.pid
./LianT stop         # 停止后台服务
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

## 7. 保留与清理

- 消息与媒体默认保留 **7 天**，配置下限也是 7 天（只能调大）；
- 单 Bot 媒体超过 **3 GB** 时按消息时间从旧到新淘汰；
- `serve` 每小时执行一次清理：删消息、标媒体过期/淘汰，并**真正删除**磁盘文件；
- 客户端本地不设上限，是否清理由用户在 App 里决定。

## 8. 升级与回滚

- 数据库有 `PRAGMA user_version` 迁移，启动时自动升级；升级前先备份 `data/`；
- 节点密钥与证书在 `data/node/`：证书剩余不足 30 天会自动用同一私钥重签，指纹不变；
- 回滚到旧版本前，先确认旧版本能读新 schema，否则用备份恢复。

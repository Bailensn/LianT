# LianT

简体中文 | [English](README.md)

一款自托管的 Bot 运营客户端与服务端（首发 QQ，Telegram 随后）。

- **服务端**（`service/`）：Go，WSS 协议 v1，SQLite，消息保留 7 天。
  只提供 `/v1/ws` 与签名媒体链接，不再内嵌网页管理端。
- **客户端**（`manager/`）：Flutter。**Android 是唯一交付目标**；Web 只在本地用于预览，
  不对外发布；不做桌面端。会话、机器人、设备与安全日志都走同一份协议。

## 文档

- 文档索引：[docs/README.md](docs/README.md)
- 设计文档与决策记录：[DESIGN.md](docs/DESIGN.md)
- 通信协议：[PROTOCOL.md](docs/PROTOCOL.md)
- 界面规范（Flutter）：[UI-DESIGN.md](docs/UI-DESIGN.md)
- 部署说明：[DEPLOY.md](docs/DEPLOY.md)
- 安全模型：[SECURITY.md](docs/SECURITY.md)
- 发布规则：[RELEASING.md](docs/RELEASING.md)
- 交接与当前状态：[HANDOFF.md](docs/HANDOFF.md)

## 仓库结构

```text
LianT/
├── service/   Go 服务端（只提供 API）
├── manager/   Flutter 客户端
├── docs/      设计、协议与运维文档
├── deploy/    systemd 单元
└── scripts/   安装脚本与协议漂移检查
```

## 许可证

本项目整体采用 **GNU Affero General Public License v3.0 (AGPL-3.0)** 授权，
覆盖 `service/`、`manager/` 与其余全部文件。

修改后通过网络提供服务，同样需要按 AGPL-3.0 公开对应源码。
完整条款见根目录 [LICENSE](LICENSE)。

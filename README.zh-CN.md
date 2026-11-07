# LianT

简体中文 | [English](README.md)

一款自托管的 Bot 运营客户端与服务端（首发 QQ，Telegram 随后）。

- 设计文档与决策记录：[DESIGN.md](docs/DESIGN.md)
- 通信协议（草案）：[PROTOCOL.md](docs/PROTOCOL.md)
- 界面规范（网页端）：[UI-DESIGN.md](docs/UI-DESIGN.md)
- 部署说明：[DEPLOY.md](docs/DEPLOY.md)
- Docker 部署：[DOCKER.md](docs/DOCKER.md)
- 安全模型：[SECURITY.md](docs/SECURITY.md)
- 发布规则：[RELEASING.md](docs/RELEASING.md)

## 许可证

本项目整体采用 **GNU Affero General Public License v3.0 (AGPL-3.0)** 授权，
覆盖 `service/`、`web/` 与其余全部文件。

这与项目形态一致：管理端已经合并进 Service 二进制，由 Service 通过网页提供，
不再是可独立分发的宽松许可客户端。修改后通过网络提供服务，同样需要按 AGPL-3.0
公开对应源码。

完整条款见根目录 [LICENSE](LICENSE)。

# 文档索引

| 文件 | 记录什么 | 什么时候看 |
| --- | --- | --- |
| [DESIGN.md](DESIGN.md) | 产品决策（D1–D12）与边界，含历史现状审计 | 改产品方向、动核心模型之前 |
| [PROTOCOL.md](PROTOCOL.md) | 线上协议 v1：传输、握手、信封、方法与错误码 | 改服务端接口或客户端协议层 |
| [UI-DESIGN.md](UI-DESIGN.md) | Flutter 客户端的视觉与交互硬门禁 | 加页面或改组件之前 |
| [DEPLOY.md](DEPLOY.md) | 二进制 + systemd 部署、配对与运行 | 装机、升级、排障 |
| [SECURITY.md](SECURITY.md) | 信任根、连接校验、公网暴露与已知取舍 | 改认证、密钥或网络边界 |
| [RELEASING.md](RELEASING.md) | 版本号、CI 产物与发布流程 | 发版、改 workflow |
| [HANDOFF.md](HANDOFF.md) | 当前实现状态、构建命令、待办与已知缺口 | 接手项目的第一份文件 |

约定：

- `docs/` 只记录「是什么」与「为什么」；代码注释解释实现取舍，不复制文档结论。
- 决策变更先加一条记录（D13、D14…），再改实现，不直接改写旧结论。
- 文档里的命令都以仓库根目录为工作目录；构建 Flutter 前先看 `HANDOFF.md` 的 PATH 说明。

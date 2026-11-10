# 文档索引

术语：**Service** = 服务端（Bot Engine，`service/`）；**Manager** = Flutter 客户端（`manager/`）。

| 文件 | 记录什么 | 什么时候看 |
| --- | --- | --- |
| [DESIGN.md](DESIGN.md) | 产品决策（D1–D12）与边界，含历史现状审计 | 改产品方向、动核心模型之前 |
| [PROTOCOL.md](PROTOCOL.md) | 线上协议 v1：传输、握手、信封、方法与错误码 | 改服务端接口或客户端协议层 |
| [UI-DESIGN.md](UI-DESIGN.md) | Flutter 客户端的视觉与交互硬门禁 | 加页面或改组件之前 |
| [DEPLOY.md](DEPLOY.md) | 二进制 + systemd 部署、配对与运行 | 装机、升级、排障 |
| [SECURITY.md](SECURITY.md) | 信任根、连接校验、公网暴露与已知取舍 | 改认证、密钥或网络边界 |
| [RELEASING.md](RELEASING.md) | 版本号、CI 产物与发布流程 | 发版、改 workflow |
| [HANDOFF.md](HANDOFF.md) | 当前实现状态、构建工具链、待办与已知缺口 | 接手项目的第一份文件 |
| [NATIVE-CORE.md](NATIVE-CORE.md) | 迁移计划：Flutter 只做界面，网络/协议/签名/存储/通知搬到 Kotlin | 动后台进程或想把核心搬出 Dart 之前 |

约定：

- **只写现状**，越短越好；实现变了就直接改，不保留历史决策记录、不「冻结」。
- 重复率高的内容只留一份：协议细节看 `PROTOCOL.md`，构建与状态看 `HANDOFF.md`。
- 命令都以仓库根目录为工作目录；构建 Flutter 前先看 `HANDOFF.md` 的工具链说明。

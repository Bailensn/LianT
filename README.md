# LianT

简体中文 | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT 是自托管 **Bot Engine**，当前支持 QQ，Telegram 接入在规划中；配套 Flutter 客户端用于日常运营。

- **Service**（`service/`）：Go，WSS 协议 v1，SQLite 或 PostgreSQL，消息保留 7 天；仅提供 `/v1/ws` 与签名媒体链接，无内嵌网页管理端。
- **Manager**（`manager/`）：Flutter。**Android 为唯一交付目标**；Web 仅用于本地预览，不对外发布，无桌面端。会话、机器人、设备与安全日志统一走同一协议。

## 安装

Service 为单个 Linux 二进制，由 systemd 托管。环境要求：

- 带 systemd 的 Linux、root 权限、`curl`
- `python3` 或 `jq`（用于选择最新 release；也可直接传入 `--url`）
- PostgreSQL 可选；也可直接使用 SQLite

按部署环境设置以下变量；命令均通过变量引用：

```bash
INSTALL_DIR=/opt/liant
RUN_USER=liant
PORT=8443
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=your_database
DB_USER=your_db_user
DB_PASSWORD=your_password
```

交互安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

脚本将依次询问安装目录、端口、运行用户和数据库类型。默认使用 PostgreSQL；选择 `sqlite` 时无需 PostgreSQL。
非交互安装 SQLite：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

安装器不创建数据库角色或数据库，也不需要 PostgreSQL 超级用户权限。请先创建：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

未加引号的 SQL 标识符会转为小写。`--db-user` 必须与实际角色的大小写完全一致；例如使用 `your_db_user`，不要写成 `Your_Db_User`。如需混合大小写，请在 SQL 中使用双引号，并在安装时传入相同的大小写。

用变量安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

也可导出 `LIANT_DB_HOST`、`LIANT_DB_PORT`、`LIANT_DB_NAME`、`LIANT_DB_USER`、`LIANT_DB_PASSWORD`，配合 `sudo -E` 使用，避免密码出现在命令行中。

默认仅安装稳定版。Service 当前仅发布 pre-release（Alpha），需显式添加 `--beta`（或设置 `LIANT_BETA=1`）。

参数：`--dir PATH`、`--port PORT`、`--user USER`、`--url URL`、`--beta`、
`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

安装完成后，在 Android Manager 中填入 `LianT pair` 输出的配对信息，并在服务器终端确认：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <手机可达地址>
```

服务端仅提供 `/v1/ws` 与 `/v1/media/`，无网页管理端。

原地升级（停服务、校验 SHA256、备份旧二进制、重启）：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

测试版同样需添加 `--beta`。`upgrade.sh` 要求已有安装，不会重建目录或配置。

卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

默认停止并移除 systemd 单元；若 `LianT` 与 `config.enc` 仍存在，先清理 LianT 表，再删除安装目录与服务用户。支持清理不完整安装：缺失步骤将跳过，不会中止。使用 `--keep-data` 保留配置和数据，`--keep-user` 保留系统用户。数据库角色与数据库不会被自动删除；如需删除，请使用 `psql`。

## 文档

- 文档索引：[docs/README.md](docs/README.md)
- 设计文档与决策记录：[DESIGN.md](docs/DESIGN.md)
- 通信协议：[PROTOCOL.md](docs/PROTOCOL.md)
- 界面规范（Flutter）：[UI-DESIGN.md](docs/UI-DESIGN.md)
- 部署说明：[DEPLOY.md](docs/DEPLOY.md)
- 安全模型：[SECURITY.md](docs/SECURITY.md)
- 第三方声明：[NOTICE](NOTICE)
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

第三方组件声明见 [NOTICE](NOTICE)。

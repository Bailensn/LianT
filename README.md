# LianT

简体中文 | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT 是一个自托管的 **Bot Engine**（Bot 引擎），首发 QQ、Telegram 随后，
并带一个 Flutter 客户端用来运营这些 Bot。

- **Service（服务端/引擎）**（`service/`）：Go，WSS 协议 v1，SQLite 或 PostgreSQL，消息保留 7 天。
  只提供 `/v1/ws` 与签名媒体链接，不再内嵌网页管理端。
- **Manager（客户端）**（`manager/`）：Flutter。**Android 是唯一交付目标**；Web 只在本地用于预览，
  不对外发布；不做桌面端。会话、机器人、设备与安全日志都走同一份协议。

## 一键安装

Service 是单个 Linux 二进制，由 systemd 托管。需要：

- 带 systemd 的 Linux、root 权限、`curl`
- `python3` 或 `jq`（用来选最新 release；也可以直接传 `--url`）
- 可选：PostgreSQL；不想用就用 SQLite

先按自己的环境设置变量。下面的命令都引用变量，不写死数据库名或账号：

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

脚本会询问安装目录、端口、运行用户和数据库类型。默认 PostgreSQL，选 `sqlite` 就不需要 PG。
非交互装 SQLite：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

安装器**不会**创建角色和数据库，也不需要 PostgreSQL 超级用户权限。先用自己的名字建好：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

SQL 里不加引号的标识符会自动转小写。`--db-user` 必须和实际角色大小写完全一致
（例如 `your_db_user`，不要写成 `Your_Db_User`）。确实需要大小写混合时，建角色用双引号包住，
安装时也传完全一样的大小写。

用变量安装：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

也可以 export `LIANT_DB_HOST`、`LIANT_DB_PORT`、`LIANT_DB_NAME`、`LIANT_DB_USER`、
`LIANT_DB_PASSWORD`，再用 `sudo -E`，这样密码不用重复写在命令里。

默认只装稳定版。Service 目前只发 pre-release（Alpha），需要加 `--beta`（或 `LIANT_BETA=1`）。

参数：`--dir PATH`、`--port PORT`、`--user USER`、`--url URL`、`--beta`、
`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

装完后打开 Android Manager，把 `LianT pair` 打印的配对信息填进去，并在服务器终端确认：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <手机可达地址>
```

服务端只提供 `/v1/ws` 与 `/v1/media/`，没有网页管理端。

原地升级（停服务、校验 SHA256、备份旧二进制、重启）：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

测试版同样加 `--beta`。`upgrade.sh` 要求已有安装，不会替你重建目录或配置。

卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

默认会停/删 systemd 单元；如果 `LianT` 和 `config.enc` 还在，先清 LianT 的表；然后删安装目录、
删服务用户。不完整的安装也能清：缺哪一步就跳过哪一步，不会因此中止。要保留配置和数据加
`--keep-data`，要保留系统用户加 `--keep-user`。数据库角色和 database 不会被自动 DROP，
需要的话自己用 `psql` 处理。

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

# LianT

[简体中文](README.md) | 繁體中文 | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT 是自託管的 **Bot Engine**（聊天機器人引擎），首發 QQ、Telegram 隨後，
並附帶一個 Flutter 用戶端。

- **Service**（`service/`）：Go、WSS 協議 v1、SQLite 或 PostgreSQL、訊息保留 7 天。只提供 `/v1/ws` 與簽名媒體連結，沒有網頁管理端。
- **Manager**（`manager/`）：Flutter。**Android 是唯一交付目標**；Web 僅供本機預覽，不發布；不做桌面端。

## 快速開始

需求：Linux + systemd、root、`curl`；`python3` 或 `jq`（選擇最新 release，或改用 `--url`）；PostgreSQL 可選。

先改成自己的值，以下指令都只引用變數，不寫死資料庫名稱或帳號：

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

互動安裝：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

腳本會詢問安裝目錄、連接埠、執行使用者與資料庫類型。預設 PostgreSQL，選 `sqlite` 可跳過。

非互動 SQLite：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

安裝器不會建立角色或資料庫，也不需要超級使用者權限。先用自己的名稱建立：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

未加引號的 SQL 識別字會轉小寫；`--db-user` 必須與實際角色大小寫一致。需要混合大小寫時，在 SQL 中用雙引號，安裝時傳完全相同的大小寫。

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

預設只裝穩定版；目前只有 pre-release（Alpha），所以加 `--beta`。也可 export `LIANT_DB_HOST`、`LIANT_DB_PORT`、`LIANT_DB_NAME`、`LIANT_DB_USER`、`LIANT_DB_PASSWORD`，再用 `sudo -E`。

參數：`--dir PATH`、`--port PORT`、`--user USER`、`--url URL`、`--beta`、`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

安裝後在 Android Manager 輸入 `LianT pair` 印出的配對資訊，並在伺服器終端確認：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <手機可達位址>
```

升級：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

測試版加 `--beta`。卸載：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

預設會刪 systemd 單元、LianT 表、安裝目錄與服務使用者；不完整的安裝也能清。`--keep-data` 保留設定與資料，`--keep-user` 保留使用者。資料庫角色與 database 不會自動 DROP。

## 文件

- [文件索引](docs/README.md)
- [設計與決策](docs/DESIGN.md)
- [通訊協議](docs/PROTOCOL.md)
- [介面規範](docs/UI-DESIGN.md)
- [部署](docs/DEPLOY.md)
- [安全模型](docs/SECURITY.md)

## 授權

AGPL-3.0，詳見 [LICENSE](LICENSE)。

# LianT

[简体中文](README.md) | 繁體中文 | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

自託管 **Bot Engine**，目前支援 QQ，Telegram 在規劃中；附 Flutter 用戶端供日常操作。

- **Service**（`service/`）：Go、WSS 協議 v1、SQLite 或 PostgreSQL、訊息保留 7 天；僅提供 `/v1/ws` 與簽名媒體連結，無內嵌網頁主控台。
- **Manager**（`manager/`）：Flutter。**Android 為唯一交付目標**；Web 僅供本機預覽，無桌面端。

## 安裝

需求：Linux + systemd、root、`curl`，以及 `python3` 或 `jq`（用於選擇 release，`--url` 可略過）。PostgreSQL 為選用。

依部署環境設定以下變數；所有指令均透過變數引用：

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

依序詢問安裝目錄、連接埠、執行使用者與資料庫類型。預設 PostgreSQL；選擇 `sqlite` 使用 SQLite。非互動 SQLite：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

安裝器不建立角色或資料庫，也不需要超級使用者權限。請先建立：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

未加引號的 SQL 識別字會轉為小寫。`--db-user` 必須與實際角色完全一致；混合大小寫時，SQL 使用雙引號，安裝時傳入相同大小寫。

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

亦可匯出 `LIANT_DB_HOST`、`LIANT_DB_PORT`、`LIANT_DB_NAME`、`LIANT_DB_USER`、`LIANT_DB_PASSWORD`，搭配 `sudo -E` 使用。

預設僅安裝穩定版。Service 目前僅發布 pre-release（Alpha），需加上 `--beta`（或 `LIANT_BETA=1`）。

參數：`--dir`、`--port`、`--user`、`--url`、`--beta`、`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

## 配對

安裝完成後，在 Android Manager 輸入 `LianT pair` 輸出，並於伺服器終端確認：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <手機可達位址>
```

## 升級與卸載

原地升級（停止、驗證 SHA256、備份、重啟）：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

測試版加上 `--beta`。需已有安裝。

卸載：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

卸載器會移除 systemd 單元、LianT 表（`LianT` 與 `config.enc` 存在時）、安裝目錄與服務使用者；支援不完整安裝。`--keep-data` 保留設定與資料，`--keep-user` 保留服務使用者。資料庫角色與資料庫不會自動刪除。

## 文件

- [文件索引](docs/README.md)
- [設計與決策](docs/DESIGN.md)
- [通訊協議](docs/PROTOCOL.md)
- [介面規範](docs/UI-DESIGN.md)
- [部署](docs/DEPLOY.md)
- [安全模型](docs/SECURITY.md)

## 授權

AGPL-3.0，詳見 [LICENSE](LICENSE)。

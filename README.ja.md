# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | 日本語 | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

セルフホスト型 **Bot Engine**。現在は QQ に対応し、Telegram は計画中です。日常運用には Flutter クライアントを使用します。

- **Service**（`service/`）：Go、WSS プロトコル v1、SQLite または PostgreSQL、メッセージを 7 日間保持。`/v1/ws` と署名付きメディアリンクのみを提供し、Web 管理画面はありません。
- **Manager**（`manager/`）：Flutter。**配布対象は Android のみ**。Web はローカルプレビュー用で、デスクトップ版はありません。

## インストール

要件：Linux + systemd、root、`curl`、`python3` または `jq`（release 選択用。`--url` で省略可）。PostgreSQL は任意です。

デプロイ環境に合わせて以下の変数を設定してください。コマンドはすべてこれらの変数を参照します：

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

対話インストール：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s --
```

インストール先、ポート、実行ユーザー、データベース種別を順に確認します。既定は PostgreSQL で、`sqlite` を選ぶと SQLite を使用します。非対話 SQLite：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

インストーラーはロールもデータベースも作成せず、スーパーユーザー権限も要求しません。事前に作成してください：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

引用符のない SQL 識別子は小文字に変換されます。`--db-user` は実際のロールと完全に一致させてください。大文字小文字を混在させる場合は SQL で二重引用符を使用し、同じ表記をインストーラーに渡します。

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

`LIANT_DB_HOST`、`LIANT_DB_PORT`、`LIANT_DB_NAME`、`LIANT_DB_USER`、`LIANT_DB_PASSWORD` を export し、`sudo -E` を使用することもできます。

既定では最新の安定版をインストールします。安定版の Release には Service のバイナリと Manager の APK が両方含まれます。pre-release を入れたい場合のみ `--beta`（または `LIANT_BETA=1`）を追加してください。
同じ Release にクライアントの `LianT-android-arm64.apk` も含まれます。Android 端末にインストールしてください。

オプション：`--dir`、`--port`、`--user`、`--url`、`--beta`、`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

## ペアリング

インストール後、Android Manager に `LianT pair` の出力を入力し、サーバー端末で確認します：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <スマホから到達できるホスト>
```

## アップグレードとアンインストール

インプレース更新（停止、SHA256 検証、バックアップ、再起動）：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

pre-release は `--beta` を追加。既存のインストールが必要です。

アンインストール：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

systemd ユニット、LianT テーブル（`LianT` と `config.enc` が存在する場合）、インストール先、サービスユーザーを削除します。不完全なインストールにも対応します。`--keep-data` で設定とデータ、`--keep-user` でサービスユーザーを保持します。データベースのロールとデータベースは自動削除しません。

## ドキュメント

- [ドキュメント一覧](docs/README.md)
- [設計と決定](docs/DESIGN.md)
- [プロトコル](docs/PROTOCOL.md)
- [デプロイ](docs/DEPLOY.md)

## ライセンス

AGPL-3.0。[LICENSE](LICENSE) を参照。

サードパーティ通知は [NOTICE](NOTICE) を参照してください。

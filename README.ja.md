# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | 日本語 | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT はセルフホスト型の **Bot Engine** です。まず QQ、次に Telegram に対応し、
Flutter 製クライアントで運用します。

- **Service**（`service/`）：Go、WSS プロトコル v1、SQLite または PostgreSQL、メッセージは 7 日保持。`/v1/ws` と署名付きメディアリンクのみを提供し、Web 管理画面はありません。
- **Manager**（`manager/`）：Flutter。**配布対象は Android のみ**。Web はローカルプレビュー用で、デスクトップ版はありません。

## クイックスタート

必要環境：Linux + systemd、root、`curl`、`python3` または `jq`（最新 release の選択に使用。`--url` でも可）。PostgreSQL は任意です。

まず自分の値に変更してください。以下のコマンドは変数を参照し、DB 名やユーザー名を固定しません：

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
  | sudo bash -s -- --beta
```

インストール先、ポート、実行ユーザー、DB 種別を聞かれます。既定は PostgreSQL で、`sqlite` を選べば不要です。

SQLite の非対話インストール：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

インストーラーはロールも DB も作成せず、スーパーユーザー権限も要求しません。先に自分の名前で作成します：

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

引用符なしの SQL 識別子は小文字に折り畳まれます。`--db-user` は実際のロールと同じ大文字小文字にしてください。混在させる場合は SQL で二重引用符を使い、インストーラーにも同じ表記を渡します。

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

既定では安定版のみ。現在は pre-release（Alpha）のみなので `--beta` を付けます。`LIANT_DB_*` を export して `sudo -E` を使うこともできます。

オプション：`--dir`、`--port`、`--user`、`--url`、`--beta`、`--db postgres|sqlite`、`--db-host`、`--db-port`、`--db-name`、`--db-user`、`--db-pass`、`-y`。

インストール後、Android Manager で `LianT pair` の情報を入力し、サーバー側で確認します：

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <スマホから到達できるホスト>
```

アップグレード：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

テスト版は `--beta`。アンインストール：

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

既定で systemd ユニット、LianT のテーブル、インストール先、サービスユーザーを削除します。不完全なインストールも処理可能です。`--keep-data` で設定とデータ、`--keep-user` でユーザーを保持します。DB のロールと database は自動削除しません。

## ドキュメント

- [ドキュメント一覧](docs/README.md)
- [設計と決定](docs/DESIGN.md)
- [プロトコル](docs/PROTOCOL.md)
- [UI 規約](docs/UI-DESIGN.md)
- [デプロイ](docs/DEPLOY.md)
- [セキュリティ](docs/SECURITY.md)

## ライセンス

AGPL-3.0。[LICENSE](LICENSE) を参照。

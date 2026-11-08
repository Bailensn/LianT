# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | English | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT is a self-hosted **Bot Engine** for chat-platform bots (QQ first; Telegram next),
with a Flutter client for operating it.

- **Service** (the engine, `service/`): Go, WSS protocol v1, SQLite or PostgreSQL, 7-day message buffer. It serves
  `/v1/ws` and signed media links only — no embedded web console.
- **Manager** (the Flutter client, `manager/`): **Android is the only shipped target**; the web build is
  a local preview surface only and is not published. No desktop builds.
  Conversations, bots, devices and audit logs are part of the same protocol surface.

## Quick start

The Service is a single Linux binary managed by systemd. Requirements:

- Linux with systemd, root access, `curl`
- `python3` or `jq` (used to pick the newest release; or pass `--url`)
- Optional: PostgreSQL if you do not want SQLite

Set your own values first. Everything below references these variables; no database or account
name is fixed:

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

Interactive install:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

It asks for the install directory, port, run user and database type. PostgreSQL is the default;
choose `sqlite` to skip PostgreSQL. Non-interactive SQLite install using the variables above:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

The installer does **not** create the role or database and does not need superuser credentials.
Create them first with your own names:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

SQL identifiers without quotes are folded to lowercase. Use the same exact name for `--db-user`
(for example `your_db_user`, not `Your_Db_User`). If you need mixed case, quote it in SQL and pass
the exact same case to the installer.

Install with the variables:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Or export `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER`,
`LIANT_DB_PASSWORD` and use `sudo -E`, so the password is not repeated in the command line.

Only stable releases are used by default. The Service currently publishes pre-release (Alpha)
builds only, so add `--beta` (or `LIANT_BETA=1`) to install one.

Flags: `--dir PATH`, `--port PORT`, `--user USER`, `--url URL`, `--beta`, `--db postgres|sqlite`,
`--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

After installation, open the Android Manager, enter the pairing info printed by `LianT pair`, and
confirm on the server terminal:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <phone-reachable-host>
```

The Service serves only `/v1/ws` and `/v1/media/`; there is no web console.

Upgrade in place (stops the service, verifies SHA256, backs up the old binary, restarts):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Use `--beta` for pre-release, same as install. Upgrade requires an existing install; it does not
bootstrap a missing directory or configuration.

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

By default it stops/removes the unit, cleans LianT tables when `LianT` and `config.enc` are still
present, removes the install directory and deletes the service user. It also handles incomplete
installs: missing pieces are skipped instead of aborting. Use `--keep-data` to preserve
`config.enc` and `data/`, and `--keep-user` to keep the system user. Database roles/databases are
never dropped automatically; use `psql` if you want to remove them.

## Documents

- Index: [docs/README.md](docs/README.md)
- Design and decisions: [DESIGN.md](docs/DESIGN.md)
- Wire protocol: [PROTOCOL.md](docs/PROTOCOL.md)
- UI rules (Flutter): [UI-DESIGN.md](docs/UI-DESIGN.md)
- Deployment: [DEPLOY.md](docs/DEPLOY.md)
- Security model: [SECURITY.md](docs/SECURITY.md)
- Release rules: [RELEASING.md](docs/RELEASING.md)
- Handoff / current status: [HANDOFF.md](docs/HANDOFF.md)

## Repository layout

```text
LianT/
├── service/   Go service (API only)
├── manager/   Flutter client
├── docs/      design, protocol and operational docs
├── deploy/    systemd unit
└── scripts/   install and protocol-drift checks
```

## License

This project is licensed as a whole under the **GNU Affero General Public License v3.0 (AGPL-3.0)**,
covering `service/`, `manager/`, and every other file in the repository.

Offering a modified version over a network therefore requires publishing the corresponding
source, as AGPL-3.0 states. The full terms are in the root [LICENSE](LICENSE).

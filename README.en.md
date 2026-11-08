# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | English | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

Self-hosted **Bot Engine** for chat platforms. QQ is supported today; Telegram is planned.
A Flutter client is provided for day-to-day operation.

- **Service** (`service/`): Go, WSS protocol v1, SQLite or PostgreSQL, 7-day message retention.
  Serves only `/v1/ws` and signed media links; no embedded web console.
- **Manager** (`manager/`): Flutter. **Android is the only shipped target.** The web build is a
  local preview; no desktop build.

## Install

Requirements: Linux with systemd, root access, `curl`, and `python3` or `jq` (release selection;
`--url` bypasses it). PostgreSQL is optional.

Set deployment values; commands reference variables only:

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

Interactive installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

The script asks for the install directory, port, run user and database type. PostgreSQL is the
default; select `sqlite` to use SQLite. Non-interactive SQLite:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

The installer does not create roles or databases and does not require superuser access. Create
them first:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

Unquoted SQL identifiers are folded to lowercase. `--db-user` must match the actual role exactly.
For mixed case, quote the identifier in SQL and pass the same case to the installer.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Alternatively, export `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` and
`LIANT_DB_PASSWORD`, then use `sudo -E`.

Only stable releases are installed by default. The Service currently publishes pre-release
(Alpha) builds only; add `--beta` (or `LIANT_BETA=1`).

Flags: `--dir PATH`, `--port PORT`, `--user USER`, `--url URL`, `--beta`, `--db postgres|sqlite`,
`--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

## Pairing

After installation, enter the `LianT pair` output in the Android Manager and confirm on the
server terminal:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <phone-reachable-host>
```

## Upgrade and uninstall

Upgrade in place (stop, verify SHA256, back up, restart):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Add `--beta` for pre-release. An existing installation is required.

Uninstall:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

The uninstaller removes the unit, LianT tables (when `LianT` and `config.enc` are present), the
install directory and the service user. Incomplete installations are supported. Use
`--keep-data` to retain configuration and data, or `--keep-user` to retain the service user.
Database roles and databases are never dropped automatically.

## Documentation

- [Index](docs/README.md)
- [Design and decisions](docs/DESIGN.md)
- [Wire protocol](docs/PROTOCOL.md)
- [UI rules](docs/UI-DESIGN.md)
- [Deployment](docs/DEPLOY.md)
- [Security](docs/SECURITY.md)

## License

AGPL-3.0. See [LICENSE](LICENSE).

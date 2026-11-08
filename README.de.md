# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | Deutsch

LianT ist eine selbst gehostete **Bot Engine** für Chat-Plattform-Bots (zuerst QQ, dann Telegram)
mit einem Flutter-Client zur Bedienung.

- **Service** (`service/`): Go, WSS-Protokoll v1, SQLite oder PostgreSQL, 7 Tage Nachrichtenaufbewahrung. Serviert nur `/v1/ws` und signierte Medienlinks; keine Weboberfläche.
- **Manager** (`manager/`): Flutter. **Android ist das einzige veröffentlichte Ziel**; der Web-Build ist nur lokale Vorschau. Kein Desktop.

## Schnellstart

Voraussetzungen: Linux mit systemd, root, `curl` und `python3` oder `jq` für die neueste Release (oder `--url`). PostgreSQL ist optional.

Eigene Werte setzen; die Befehle verwenden Variablen und legen weder Datenbank- noch Kontonamen fest:

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

Interaktive Installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

Gefragt werden Installationsverzeichnis, Port, Laufzeitbenutzer und Datenbanktyp. PostgreSQL ist Standard; `sqlite` überspringt es.

SQLite ohne Interaktion:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

Der Installer legt **weder Rolle noch Datenbank** an und braucht keine Superuser-Rechte. Vorher mit eigenen Namen erstellen:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

SQL-Bezeichner ohne Anführungszeichen werden kleingeschrieben. `--db-user` muss exakt dem tatsächlichen Rollennamen entsprechen (z. B. `your_db_user`, nicht `Your_Db_User`). Bei gemischter Schreibweise in SQL Anführungszeichen verwenden und dem Installer exakt dieselbe Schreibweise übergeben.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Alternativ `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` und `LIANT_DB_PASSWORD` exportieren und `sudo -E` verwenden.

Standardmäßig werden nur stabile Releases installiert. Aktuell gibt es nur Pre-Releases (Alpha), daher `--beta` (oder `LIANT_BETA=1`) ergänzen.

Optionen: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

Nach der Installation im Android-Manager die `LianT pair`-Daten eingeben und im Server-Terminal bestätigen:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <vom Telefon erreichbarer Host>
```

Upgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Für Pre-Releases `--beta` ergänzen. Deinstallation:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Standardmäßig werden systemd-Unit, LianT-Tabellen, Installationsverzeichnis und Dienstbenutzer entfernt; unvollständige Installationen werden ebenfalls bereinigt. `--keep-data` behält Konfiguration und Daten, `--keep-user` den Benutzer. Datenbankrollen und -datenbanken werden nicht automatisch gelöscht.

## Dokumentation

- [Index](docs/README.md)
- [Design und Entscheidungen](docs/DESIGN.md)
- [Protokoll](docs/PROTOCOL.md)
- [UI-Regeln](docs/UI-DESIGN.md)
- [Deployment](docs/DEPLOY.md)
- [Sicherheit](docs/SECURITY.md)

## Lizenz

AGPL-3.0; siehe [LICENSE](LICENSE).

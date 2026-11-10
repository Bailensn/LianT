# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | Deutsch

Selbst gehostete **Bot Engine** für Chat-Plattformen. QQ wird unterstützt; Telegram ist geplant.
Ein Flutter-Client steht für den laufenden Betrieb bereit.

- **Service** (`service/`): Go, WSS-Protokoll v1, SQLite oder PostgreSQL, 7 Tage Nachrichtenaufbewahrung. Serviert nur `/v1/ws` und signierte Medienlinks; keine Weboberfläche.
- **Manager** (`manager/`): Flutter. **Android ist das einzige veröffentlichte Ziel.** Der Web-Build ist eine lokale Vorschau; keine Desktop-Version.

## Installation

Voraussetzungen: Linux mit systemd, root, `curl` und `python3` oder `jq` (Release-Auswahl; `--url` umgeht sie). PostgreSQL ist optional.

Legen Sie die folgenden Variablen für Ihre Deployment-Umgebung fest; alle Befehle referenzieren sie:

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
  | sudo bash -s --
```

Abfrage von Installationsverzeichnis, Port, Laufzeitbenutzer und Datenbanktyp. PostgreSQL ist
Standard; `sqlite` wählt SQLite. Nicht interaktiv mit SQLite:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

Der Installer erstellt weder Rollen noch Datenbanken und benötigt keine Superuser-Rechte. Zuerst anlegen:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

SQL-Bezeichner ohne Anführungszeichen werden kleingeschrieben. `--db-user` muss exakt dem
tatsächlichen Rollennamen entsprechen. Bei gemischter Schreibweise den Bezeichner in SQL quoten
und dieselbe Schreibweise an den Installer übergeben.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Alternativ `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` und
`LIANT_DB_PASSWORD` exportieren und `sudo -E` verwenden.

Standardmäßig wird das neueste stabile Release installiert; es enthält sowohl die
Service-Binaries als auch die Manager-APK. `--beta` (oder `LIANT_BETA=1`) nur für Pre-Releases.
Dasselbe Release enthält `LianT-android-arm64.apk` (den Client) für dein Android-Gerät.

Optionen: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`,
`--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

## Kopplung

Nach der Installation die Ausgabe von `LianT pair` im Android-Manager eingeben und im
Server-Terminal bestätigen:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <vom Telefon erreichbarer Host>
```

## Upgrade und Deinstallation

In-Place-Upgrade (stoppen, SHA256 prüfen, sichern, neu starten):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Für Pre-Releases `--beta` ergänzen. Eine bestehende Installation ist erforderlich.

Deinstallation:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Entfernt systemd-Unit, LianT-Tabellen (wenn `LianT` und `config.enc` vorhanden sind),
Installationsverzeichnis und Dienstbenutzer. Unvollständige Installationen werden unterstützt.
`--keep-data` behält Konfiguration und Daten, `--keep-user` den Dienstbenutzer. Datenbankrollen
und -datenbanken werden nie automatisch gelöscht.

## Dokumentation

- [Index](docs/README.md)
- [Design und Entscheidungen](docs/DESIGN.md)
- [Protokoll](docs/PROTOCOL.md)
- [Deployment](docs/DEPLOY.md)

## Lizenz

AGPL-3.0. Siehe [LICENSE](LICENSE).

Hinweise zu Drittanbieterkomponenten stehen in [NOTICE](NOTICE).

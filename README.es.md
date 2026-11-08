# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | Español | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT es un **Bot Engine** autoalojado para bots de plataformas de chat (QQ primero; Telegram después),
con un cliente Flutter para operarlo.

- **Service** (`service/`): Go, protocolo WSS v1, SQLite o PostgreSQL, retención de mensajes de 7 días. Solo sirve `/v1/ws` y enlaces de medios firmados; no tiene consola web.
- **Manager** (`manager/`): Flutter. **Android es el único objetivo publicado**; la versión web es solo vista previa local. Sin versión de escritorio.

## Inicio rápido

Requisitos: Linux con systemd, root, `curl`, y `python3` o `jq` para elegir la release más reciente (o usar `--url`). PostgreSQL es opcional.

Define tus propios valores; los comandos usan variables y no fijan nombres de base de datos ni de cuenta:

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

Instalación interactiva:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

Pregunta el directorio, el puerto, el usuario de ejecución y el tipo de base de datos. PostgreSQL es el valor predeterminado; elige `sqlite` para omitirlo.

SQLite sin interacción:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

El instalador **no** crea el rol ni la base de datos y no necesita superusuario. Créalos antes con tus propios nombres:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

Los identificadores SQL sin comillas se convierten a minúsculas. Usa el mismo nombre exacto en `--db-user` (por ejemplo, `your_db_user`, no `Your_Db_User`). Si necesitas mayúsculas, cita el nombre en SQL y pasa exactamente el mismo texto al instalador.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

También puedes exportar `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` y `LIANT_DB_PASSWORD` y usar `sudo -E`.

Por defecto solo se instalan versiones estables. Actualmente solo hay pre-release (Alpha), así que añade `--beta` (o `LIANT_BETA=1`).

Opciones: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

Tras instalar, abre el Manager de Android, introduce la información de `LianT pair` y confirma en la terminal del servidor:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <host accesible desde el móvil>
```

Actualizar:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Usa `--beta` para pre-release. Desinstalar:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Por defecto elimina la unidad systemd, las tablas de LianT, el directorio y el usuario del servicio; también limpia instalaciones incompletas. `--keep-data` conserva configuración y datos; `--keep-user` conserva el usuario. Los roles y bases de datos no se eliminan automáticamente.

## Documentación

- [Índice](docs/README.md)
- [Diseño y decisiones](docs/DESIGN.md)
- [Protocolo](docs/PROTOCOL.md)
- [Reglas de UI](docs/UI-DESIGN.md)
- [Despliegue](docs/DEPLOY.md)
- [Seguridad](docs/SECURITY.md)

## Licencia

AGPL-3.0; consulta [LICENSE](LICENSE).

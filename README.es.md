# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | Español | [Français](README.fr.md) | [Deutsch](README.de.md)

**Bot Engine** autoalojado para plataformas de chat. QQ está disponible; Telegram está previsto.
Se incluye un cliente Flutter para la operación diaria.

- **Service** (`service/`): Go, protocolo WSS v1, SQLite o PostgreSQL, retención de mensajes de 7 días. Solo sirve `/v1/ws` y enlaces de medios firmados; sin consola web.
- **Manager** (`manager/`): Flutter. **Android es el único objetivo publicado.** La versión web es una vista previa local; no hay versión de escritorio.

## Instalación

Requisitos: Linux con systemd, root, `curl` y `python3` o `jq` (selección de release; `--url` lo omite). PostgreSQL es opcional.

Defina las siguientes variables según su entorno de despliegue; todos los comandos las referencian:

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

Solicita el directorio, el puerto, el usuario de ejecución y el tipo de base de datos. PostgreSQL es el valor predeterminado; seleccione `sqlite` para usar SQLite. SQLite sin interacción:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

El instalador no crea roles ni bases de datos y no requiere superusuario. Créelos primero:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

Los identificadores SQL sin comillas se convierten a minúsculas. `--db-user` debe coincidir exactamente con el rol real. Para mayúsculas y minúsculas, cite el identificador en SQL y pase la misma grafía al instalador.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

También puede exportar `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` y `LIANT_DB_PASSWORD`, y usar `sudo -E`.

Solo se instalan versiones estables de forma predeterminada. El Service solo publica pre-release (Alpha); añada `--beta` (o `LIANT_BETA=1`).

Opciones: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

## Emparejamiento

Tras instalar, introduzca la salida de `LianT pair` en Android Manager y confirme en la terminal del servidor:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <host accesible desde el móvil>
```

## Actualización y desinstalación

Actualización in situ (detener, verificar SHA256, respaldar, reiniciar):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Añada `--beta` para pre-release. Requiere una instalación existente.

Desinstalación:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Elimina la unidad, las tablas de LianT (si `LianT` y `config.enc` existen), el directorio y el usuario del servicio. Admite instalaciones incompletas. `--keep-data` conserva configuración y datos; `--keep-user` conserva el usuario. Los roles y bases de datos no se eliminan automáticamente.

## Documentación

- [Índice](docs/README.md)
- [Diseño y decisiones](docs/DESIGN.md)
- [Protocolo](docs/PROTOCOL.md)
- [Reglas de UI](docs/UI-DESIGN.md)
- [Despliegue](docs/DEPLOY.md)
- [Seguridad](docs/SECURITY.md)

## Licencia

AGPL-3.0. Consulte [LICENSE](LICENSE).

Los avisos de terceros están en [NOTICE](NOTICE).

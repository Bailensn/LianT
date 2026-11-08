# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | Français | [Deutsch](README.de.md)

LianT est un **Bot Engine** auto-hébergé pour les bots de plateformes de chat (QQ d'abord ; Telegram ensuite),
avec un client Flutter pour l'exploiter.

- **Service** (`service/`) : Go, protocole WSS v1, SQLite ou PostgreSQL, messages conservés 7 jours. Il ne sert que `/v1/ws` et des liens média signés ; pas de console web.
- **Manager** (`manager/`) : Flutter. **Android est la seule cible publiée** ; la version web sert uniquement d'aperçu local. Pas de version bureau.

## Démarrage rapide

Prérequis : Linux avec systemd, root, `curl`, et `python3` ou `jq` pour choisir la dernière release (ou `--url`). PostgreSQL est facultatif.

Définissez vos propres valeurs ; les commandes utilisent des variables et ne figent ni le nom de la base ni celui du compte :

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

Installation interactive :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

Le script demande le répertoire, le port, l'utilisateur d'exécution et le type de base. PostgreSQL est la valeur par défaut ; choisissez `sqlite` pour l'éviter.

SQLite sans interaction :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

L'installateur ne crée **ni le rôle ni la base** et ne demande pas de superutilisateur. Créez-les avant avec vos propres noms :

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

Les identifiants SQL sans guillemets sont convertis en minuscules. Utilisez exactement le même nom pour `--db-user` (par exemple `your_db_user`, pas `Your_Db_User`). Si vous voulez des majuscules, citez le nom en SQL et passez exactement la même casse à l'installateur.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Vous pouvez aussi exporter `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER` et `LIANT_DB_PASSWORD`, puis utiliser `sudo -E`.

Par défaut, seules les versions stables sont installées. Le Service ne publie actuellement que des pre-releases (Alpha) : ajoutez `--beta` (ou `LIANT_BETA=1`).

Options : `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

Après l'installation, ouvrez le Manager Android, saisissez les informations de `LianT pair` et confirmez dans le terminal du serveur :

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <hôte joignable depuis le téléphone>
```

Mise à jour :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Ajoutez `--beta` pour une pre-release. Désinstallation :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Par défaut, supprime l'unité systemd, les tables LianT, le répertoire et l'utilisateur du service ; gère aussi les installations incomplètes. `--keep-data` conserve configuration et données ; `--keep-user` conserve l'utilisateur. Les rôles et bases de données ne sont pas supprimés automatiquement.

## Documentation

- [Index](docs/README.md)
- [Conception et décisions](docs/DESIGN.md)
- [Protocole](docs/PROTOCOL.md)
- [Règles d'interface](docs/UI-DESIGN.md)
- [Déploiement](docs/DEPLOY.md)
- [Sécurité](docs/SECURITY.md)

## Licence

AGPL-3.0 ; voir [LICENSE](LICENSE).

# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | Français | [Deutsch](README.de.md)

**Bot Engine** auto-hébergé pour plateformes de chat. QQ est pris en charge ; Telegram est prévu.
Un client Flutter est fourni pour l'exploitation quotidienne.

- **Service** (`service/`) : Go, protocole WSS v1, SQLite ou PostgreSQL, conservation des messages 7 jours. Sert uniquement `/v1/ws` et des liens média signés ; pas de console web.
- **Manager** (`manager/`) : Flutter. **Android est la seule cible publiée.** La version web est un aperçu local ; pas de version bureau.

## Installation

Prérequis : Linux avec systemd, root, `curl` et `python3` ou `jq` (sélection de release ; `--url` l'évite). PostgreSQL est facultatif.

Définissez les variables suivantes selon votre environnement de déploiement ; toutes les commandes y font référence :

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

Le script demande le répertoire, le port, l'utilisateur d'exécution et le type de base. PostgreSQL
est la valeur par défaut ; choisissez `sqlite` pour SQLite. SQLite sans interaction :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

L'installateur ne crée ni rôle ni base de données et ne requiert pas de superutilisateur. Créez-les d'abord :

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

Les identifiants SQL sans guillemets sont convertis en minuscules. `--db-user` doit correspondre
exactement au rôle réel. Pour une casse mixte, citez l'identifiant dans SQL et transmettez la même
graphie à l'installateur.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

Vous pouvez aussi exporter `LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER`
et `LIANT_DB_PASSWORD`, puis utiliser `sudo -E`.

Seules les versions stables sont installées par défaut. Le Service ne publie que des pre-releases
(Alpha) ; ajoutez `--beta` (ou `LIANT_BETA=1`).

Options : `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`,
`--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

## Appairage

Après l'installation, saisissez la sortie de `LianT pair` dans Android Manager et confirmez dans
le terminal du serveur :

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <hôte joignable depuis le téléphone>
```

## Mise à jour et désinstallation

Mise à jour sur place (arrêt, vérification SHA256, sauvegarde, redémarrage) :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

Ajoutez `--beta` pour une pre-release. Une installation existante est requise.

Désinstallation :

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

Supprime l'unité, les tables LianT (si `LianT` et `config.enc` existent), le répertoire et
l'utilisateur du service. Les installations incomplètes sont prises en charge. `--keep-data`
conserve configuration et données ; `--keep-user` conserve l'utilisateur. Les rôles et bases de
données ne sont jamais supprimés automatiquement.

## Documentation

- [Index](docs/README.md)
- [Conception et décisions](docs/DESIGN.md)
- [Protocole](docs/PROTOCOL.md)
- [Déploiement](docs/DEPLOY.md)

## Licence

AGPL-3.0. Voir [LICENSE](LICENSE).

Les mentions des composants tiers figurent dans [NOTICE](NOTICE).

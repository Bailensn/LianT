# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | 한국어 | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

LianT는 셀프 호스팅 **Bot Engine**입니다. QQ를 먼저 지원하고 Telegram이 뒤따르며,
Flutter 클라이언트로 운영합니다.

- **Service**(`service/`): Go, WSS 프로토콜 v1, SQLite 또는 PostgreSQL, 메시지 7일 보관. `/v1/ws`와 서명된 미디어 링크만 제공하며 웹 관리 화면은 없습니다.
- **Manager**(`manager/`): Flutter. **Android만 배포**하며 Web은 로컬 미리보기용이고 데스크톱은 없습니다.

## 빠른 시작

필요 환경: Linux + systemd, root, `curl`, 최신 release 선택용 `python3` 또는 `jq`(또는 `--url`). PostgreSQL은 선택 사항입니다.

먼저 값을 자신의 환경에 맞게 바꾸세요. 아래 명령은 변수만 참조하며 DB 이름이나 계정을 고정하지 않습니다:

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

대화형 설치:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- --beta
```

설치 경로, 포트, 실행 사용자, DB 종류를 묻습니다. 기본은 PostgreSQL이며 `sqlite`를 선택하면 필요 없습니다.

비대화형 SQLite:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

설치 스크립트는 role과 database를 만들지 않으며 슈퍼유저 권한도 필요로 하지 않습니다. 먼저 원하는 이름으로 만드세요:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

따옴표 없는 SQL 식별자는 소문자로 변환됩니다. `--db-user`는 실제 role과 대소문자가 같아야 합니다. 대소문자를 섞어야 하면 SQL에서 큰따옴표로 감싸고 설치할 때도 같은 표기를 넘기세요.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

기본은 안정 버전만 사용합니다. 현재는 pre-release(Alpha)만 있으므로 `--beta`를 붙입니다. `LIANT_DB_*`를 export하고 `sudo -E`를 써도 됩니다.

옵션: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

설치 후 Android Manager에서 `LianT pair` 정보를 입력하고 서버 터미널에서 확인합니다:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <휴대폰에서 접근 가능한 호스트>
```

업그레이드:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

테스트 버전은 `--beta`. 제거:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

기본으로 systemd 유닛, LianT 테이블, 설치 디렉터리, 서비스 사용자를 제거합니다. 불완전한 설치도 정리합니다. `--keep-data`는 설정/데이터, `--keep-user`는 사용자를 보존합니다. DB role과 database는 자동으로 DROP하지 않습니다.

## 문서

- [문서 색인](docs/README.md)
- [설계와 결정](docs/DESIGN.md)
- [프로토콜](docs/PROTOCOL.md)
- [UI 규칙](docs/UI-DESIGN.md)
- [배포](docs/DEPLOY.md)
- [보안](docs/SECURITY.md)

## 라이선스

AGPL-3.0. [LICENSE](LICENSE) 참고.

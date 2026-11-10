# LianT

[简体中文](README.md) | [繁體中文](README.zh-TW.md) | [English](README.en.md) | [日本語](README.ja.md) | 한국어 | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md)

셀프 호스팅 **Bot Engine**입니다. 현재 QQ를 지원하며 Telegram은 계획 중입니다. 일상 운영에는 Flutter 클라이언트를 사용합니다.

- **Service**(`service/`): Go, WSS 프로토콜 v1, SQLite 또는 PostgreSQL, 메시지 7일 보관. `/v1/ws`와 서명된 미디어 링크만 제공하며 웹 관리 화면은 없습니다.
- **Manager**(`manager/`): Flutter. **Android만 배포 대상**입니다. Web은 로컬 미리보기용이며 데스크톱 버전은 없습니다.

## 설치

요구 사항: Linux + systemd, root, `curl`, `python3` 또는 `jq`(release 선택용, `--url`로 생략 가능). PostgreSQL은 선택 사항입니다.

배포 환경에 맞게 다음 변수를 설정하세요. 모든 명령은 이 변수를 참조합니다:

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

설치 경로, 포트, 실행 사용자, 데이터베이스 종류를 순서대로 묻습니다. 기본값은 PostgreSQL이며 `sqlite`를 선택하면 SQLite를 사용합니다. 비대화형 SQLite:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --dir "$INSTALL_DIR" --port "$PORT" --user "$RUN_USER" --db sqlite
```

### PostgreSQL

설치 스크립트는 role과 database를 생성하지 않으며 슈퍼유저 권한도 요구하지 않습니다. 먼저 생성하세요:

```sql
CREATE ROLE your_db_user LOGIN PASSWORD 'your_db_password';
CREATE DATABASE your_db_name OWNER your_db_user;
```

따옴표 없는 SQL 식별자는 소문자로 변환됩니다. `--db-user`는 실제 role과 정확히 일치해야 합니다. 대소문자를 혼용하려면 SQL에서 큰따옴표를 사용하고 동일한 표기를 설치 스크립트에 전달하세요.

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh \
  | sudo bash -s -- -y --beta \
      --db postgres --db-host "$DB_HOST" --db-port "$DB_PORT" \
      --db-name "$DB_NAME" --db-user "$DB_USER" --db-pass "$DB_PASSWORD"
```

`LIANT_DB_HOST`, `LIANT_DB_PORT`, `LIANT_DB_NAME`, `LIANT_DB_USER`, `LIANT_DB_PASSWORD`를 export하고 `sudo -E`를 사용할 수도 있습니다.

기본적으로 안정 버전만 설치합니다. Service는 현재 pre-release(Alpha)만 배포하므로 `--beta`(또는 `LIANT_BETA=1`)를 추가하세요.

옵션: `--dir`, `--port`, `--user`, `--url`, `--beta`, `--db postgres|sqlite`, `--db-host`, `--db-port`, `--db-name`, `--db-user`, `--db-pass`, `-y`.

## 페어링

설치 후 Android Manager에 `LianT pair` 출력을 입력하고 서버 터미널에서 확인합니다:

```bash
cd "$INSTALL_DIR" && sudo -u "$RUN_USER" ./LianT pair --host <휴대폰에서 접근 가능한 호스트>
```

## 업그레이드 및 제거

제자리 업그레이드(중지, SHA256 검증, 백업, 재시작):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
```

pre-release는 `--beta`를 추가합니다. 기존 설치가 필요합니다.

제거:

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/uninstall.sh \
  | sudo bash -s -- -y
```

systemd 유닛, LianT 테이블(`LianT`와 `config.enc`가 있을 때), 설치 디렉터리, 서비스 사용자를 제거합니다. 불완전한 설치도 지원합니다. `--keep-data`는 설정과 데이터, `--keep-user`는 서비스 사용자를 유지합니다. 데이터베이스 role과 database는 자동으로 삭제하지 않습니다.

## 문서

- [문서 색인](docs/README.md)
- [설계와 결정](docs/DESIGN.md)
- [프로토콜](docs/PROTOCOL.md)
- [배포](docs/DEPLOY.md)

## 라이선스

AGPL-3.0. [LICENSE](LICENSE) 참고.

서드파티 고지는 [NOTICE](NOTICE)를 참조하세요.

#!/usr/bin/env bash
# LianT 一键部署。
#
#   curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | bash
#
# 可选环境变量：
#   LIANT_DIR      安装目录          默认 $HOME/liant
#   LIANT_PORT     对外端口          默认 8443
#   LIANT_DOMAIN   域名；自动起 Caddy 申请证书
#   LIANT_PROXY    external = 已有 Nginx/Caddy，只监听 127.0.0.1
#   LIANT_USER     账号              默认 admin
#   LIANT_PASSWORD 密码              留空则随机生成
#   LIANT_IMAGE    镜像              默认 ghcr.io/bailensn/liant:latest
set -euo pipefail

DIR="${LIANT_DIR:-$HOME/liant}"
PORT="${LIANT_PORT:-8443}"
DOMAIN="${LIANT_DOMAIN:-}"
PROXY="${LIANT_PROXY:-}"
BASIC_USER="${LIANT_USER:-admin}"
BASIC_PASS="${LIANT_PASSWORD:-}"
IMAGE="${LIANT_IMAGE:-ghcr.io/bailensn/liant:latest}"

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "没有找到 docker，先安装 Docker：https://docs.docker.com/engine/install/"

COMPOSE=""
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1 && docker-compose version 2>/dev/null | head -1 | grep -q 'v2\.'; then
  COMPOSE="docker-compose"
fi
[ -n "$COMPOSE" ] || die "需要 Docker Compose v2。Debian/Ubuntu：apt install docker-compose-v2；其它见 https://docs.docker.com/compose/install/"

[ -n "$BASIC_PASS" ] || BASIC_PASS=$(LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 16)

say "安装目录：$DIR"
mkdir -p "$DIR"
cd "$DIR"

TZ_VALUE="${TZ:-Asia/Shanghai}"

# $1 = ports/expose 片段，$2 = 额外环境变量，$3 = 是否额外起 Caddy
write_compose() {
  {
    echo "services:"
    echo "  liant:"
    echo "    image: $IMAGE"
    echo "    container_name: liant"
    echo "    restart: unless-stopped"
    printf '%s\n' "$1"
    echo "    volumes:"
    echo "      - liant_data:/app"
    echo "    environment:"
    echo "      TZ: $TZ_VALUE"
    echo "      LIANT_CONFIRM: auto"
    echo "      LIANT_BASIC_AUTH: \"$BASIC_USER:$BASIC_PASS\""
    printf '%s\n' "$2"
    echo "    security_opt:"
    echo "      - no-new-privileges:true"
    echo "    cap_drop:"
    echo "      - ALL"
    echo "    read_only: true"
    echo "    tmpfs:"
    echo "      - /tmp"

    if [ "$3" = "caddy" ]; then
      cat <<'YAML'

  caddy:
    image: caddy:2-alpine
    container_name: liant-caddy
    restart: unless-stopped
    depends_on:
      - liant
    ports:
      - "80:80"
      - "443:443"
    security_opt:
      - no-new-privileges:true
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
YAML
    fi

    echo
    echo "volumes:"
    echo "  liant_data:"
    if [ "$3" = "caddy" ]; then
      echo "  caddy_data:"
      echo "  caddy_config:"
    fi
  } > docker-compose.yml
}

if [ "$PROXY" = "external" ]; then
  say "已有反代模式：只监听 127.0.0.1:$PORT"
  write_compose "    ports:
      - \"127.0.0.1:$PORT:8443\"" '      LIANT_NO_TLS: "1"' external
elif [ -n "$DOMAIN" ]; then
  say "域名模式：$DOMAIN，Caddy 自动申请证书"
  cat > Caddyfile <<CADDY
$DOMAIN {
	encode zstd gzip
	reverse_proxy liant:8443
}
CADDY
  write_compose '    expose:
      - "8443"' '      LIANT_NO_TLS: "1"' caddy
else
  say "单容器模式：对外 https://<本机地址>:$PORT"
  write_compose "    ports:
      - \"$PORT:8443\"" "" plain
fi

say "拉取镜像"
$COMPOSE pull -q

say "启动"
$COMPOSE up -d

if [ -n "$DOMAIN" ]; then
  PUBLIC="https://$DOMAIN"
elif [ "$PROXY" = "external" ]; then
  PUBLIC="https://你的域名"
else
  IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
  PUBLIC="https://${IP:-<本机地址>}:$PORT"
fi

cat <<INFO

部署完成。

  管理页：$PUBLIC
  账号：  $BASIC_USER
  密码：  $BASIC_PASS
  配置：  $DIR/docker-compose.yml

INFO

if [ "$PROXY" = "external" ]; then
  cat <<NGINX
反代指向（二选一）：

  location / {
      proxy_pass http://127.0.0.1:$PORT;
      proxy_http_version 1.1;
      proxy_set_header Upgrade \$http_upgrade;
      proxy_set_header Connection "upgrade";
      proxy_set_header Host \$host;
  }

  caddy: reverse_proxy 127.0.0.1:$PORT

NGINX
fi

cat <<DONE
浏览器打开上面的地址，登录后点「直接进入」即可，不需要扫码或配对码。

常用命令：

  cd $DIR
  $COMPOSE logs -f liant
  $COMPOSE restart liant
  $COMPOSE down

DONE

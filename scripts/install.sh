#!/usr/bin/env bash
# LianT 一键部署脚本。
#
#   curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | bash
#
# 可选环境变量：
#   LIANT_DIR     安装目录         默认 $HOME/liant
#   LIANT_PORT    对外端口         默认 8443
#   LIANT_DOMAIN  域名；给了就自动起 Caddy 申请证书
#   LIANT_IMAGE   镜像             默认 ghcr.io/bailensn/liant:latest
set -euo pipefail

DIR="${LIANT_DIR:-$HOME/liant}"
PORT="${LIANT_PORT:-8443}"
DOMAIN="${LIANT_DOMAIN:-}"
BASIC_USER="${LIANT_USER:-admin}"
BASIC_PASS="${LIANT_PASSWORD:-}"
IMAGE="${LIANT_IMAGE:-ghcr.io/bailensn/liant:latest}"

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "没有找到 docker，先安装 Docker：https://docs.docker.com/engine/install/"

# 兼容两种安装方式：docker compose 插件，或独立的 docker-compose v2 二进制
COMPOSE=""
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif command -v docker-compose >/dev/null 2>&1 && docker-compose version 2>/dev/null | head -1 | grep -q 'v2\.'; then
  COMPOSE="docker-compose"
fi
if [ -z "$COMPOSE" ]; then
  die "需要 Docker Compose v2。Debian/Ubuntu：apt install docker-compose-v2 ；其它系统见 https://docs.docker.com/compose/install/"
fi

if [ -z "$BASIC_PASS" ]; then
  BASIC_PASS=$(LC_ALL=C tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 16 || true)
fi

say "安装目录：$DIR"
mkdir -p "$DIR"
cd "$DIR"

if [ -n "$DOMAIN" ]; then
  say "域名模式：$DOMAIN，Caddy 会自动申请证书"
  cat > Caddyfile <<EOF
$DOMAIN {
	encode zstd gzip
	reverse_proxy liant:8443
}
EOF
  cat > docker-compose.yml <<EOF
services:
  liant:
    image: $IMAGE
    container_name: liant
    restart: unless-stopped
    expose:
      - "8443"
    volumes:
      - liant_data:/app
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      LIANT_CONFIRM: auto
      LIANT_BASIC_AUTH: "$BASIC_USER:$BASIC_PASS"
      LIANT_NO_TLS: "1"
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    read_only: true
    tmpfs:
      - /tmp

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

volumes:
  liant_data:
  caddy_data:
  caddy_config:
EOF
else
  say "单容器模式：对外 https://<本机地址>:$PORT"
  cat > docker-compose.yml <<EOF
services:
  liant:
    image: $IMAGE
    container_name: liant
    restart: unless-stopped
    ports:
      - "$PORT:8443"
    volumes:
      - liant_data:/app
    environment:
      TZ: ${TZ:-Asia/Shanghai}
      LIANT_CONFIRM: auto
      LIANT_BASIC_AUTH: "$BASIC_USER:$BASIC_PASS"
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    read_only: true
    tmpfs:
      - /tmp

volumes:
  liant_data:
EOF
fi

say "拉取镜像"
$COMPOSE pull -q

say "启动"
$COMPOSE up -d

PUBLIC=""
if [ -n "$DOMAIN" ]; then
  PUBLIC="https://$DOMAIN"
else
  IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
  [ -n "$IP" ] || IP="<本机地址>"
  PUBLIC="https://$IP:$PORT"
fi

PAIR_ARGS="${PUBLIC:+--public-url $PUBLIC}"
[ -n "$DOMAIN" ] && PAIR_ARGS="--public-url $PUBLIC"

cat <<EOF

部署完成。

  管理页：$PUBLIC
  账号：  $BASIC_USER
  密码：  $BASIC_PASS
  数据卷：liant_data
  配置：  $DIR/docker-compose.yml

下一步配对（终端会打印二维码，手机扫码即可）：

  cd $DIR && $COMPOSE run --rm liant pair $PAIR_ARGS

常用命令：

  cd $DIR
  $COMPOSE logs -f liant     # 看日志
  $COMPOSE restart liant     # 重启
  $COMPOSE down              # 停止

登录凭据存在 $DIR/docker-compose.yml 的 LIANT_BASIC_AUTH 里，忘了就改那里再 $COMPOSE up -d。

EOF

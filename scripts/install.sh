#!/usr/bin/env bash
set -euo pipefail

DIR="${LIANT_DIR:-/opt/liant}"
PORT="${LIANT_PORT:-8443}"
USER_NAME="${LIANT_USER:-liant}"
ASSUME_YES="${LIANT_YES:-0}"
BETA="${LIANT_BETA:-0}"
DB_DRIVER="${LIANT_DB_DRIVER:-postgres}"
DB_HOST="${LIANT_DB_HOST:-127.0.0.1}"
DB_PORT="${LIANT_DB_PORT:-5432}"
DB_NAME="${LIANT_DB_NAME:-}"
DB_USER="${LIANT_DB_USER:-}"
DB_PASS="${LIANT_DB_PASSWORD:-}"
REPO="${LIANT_REPO:-Bailensn/LianT}"
SRC=""

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

latest_release_asset() {
  local mode="$1" asset="$2" json="$3"
  if command -v python3 >/dev/null 2>&1; then
    printf '%s' "$json" | python3 -c '
import json, sys
mode, asset = sys.argv[1], sys.argv[2]
data = json.load(sys.stdin)
best = None
for r in data:
    if mode == "stable" and r.get("prerelease"):
        continue
    assets = {a.get("name"): a.get("browser_download_url") for a in r.get("assets", [])}
    if asset not in assets:
        continue
    stamp = r.get("published_at") or r.get("created_at") or ""
    if best is None or stamp > best[0]:
        best = (stamp, r.get("tag_name", ""), assets[asset], assets.get("SHA256SUMS.txt", ""))
if best:
    print("\t".join(best[1:]))
' "$mode" "$asset"
  elif command -v jq >/dev/null 2>&1; then
    printf '%s' "$json" | jq -r --arg mode "$mode" --arg asset "$asset" '
      [ .[] | select(($mode == "beta") or (.prerelease | not))
        | . as $r
        | ($r.assets | map({(.name): .browser_download_url}) | add) as $a
        | select($a[$asset] != null)
        | {stamp: ($r.published_at // $r.created_at // ""), tag: $r.tag_name,
           bin: $a[$asset], sum: ($a["SHA256SUMS.txt"] // "")} ]
      | sort_by(.stamp)
      | if length == 0 then empty else .[-1] | [.tag, .bin, .sum] | @tsv end
    '
  else
    die "需要 python3 或 jq 来选择最新 release"
  fi
}

usage() {
  cat <<'USAGE'
用法：install.sh [选项] [二进制路径]

  --dir PATH     安装目录（默认 /opt/liant）
  --port PORT    服务端口（默认 8443）
  --user USER    运行服务的用户（默认 liant）
  --url URL      从指定地址下载二进制
  --beta         接收测试版（pre-release）；默认只装稳定版
  --db TYPE      数据库：postgres（默认）或 sqlite
  --db-host H    PostgreSQL 主机（默认 127.0.0.1）
  --db-port P    PostgreSQL 端口（默认 5432）
  --db-name N    PostgreSQL 数据库名（默认 liant）
  --db-user U    PostgreSQL 用户（默认 liant）
  --db-pass P    PostgreSQL 密码
  -y, --yes      全部使用默认值，不询问
  -h, --help     显示帮助

不传二进制路径时，按发布时间取最新 release；默认稳定版，加 --beta 才接收 pre-release。
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir) [ "$#" -ge 2 ] || die "--dir 缺少值"; DIR="$2"; shift 2 ;;
    --port) [ "$#" -ge 2 ] || die "--port 缺少值"; PORT="$2"; shift 2 ;;
    --user) [ "$#" -ge 2 ] || die "--user 缺少值"; USER_NAME="$2"; shift 2 ;;
    --url) [ "$#" -ge 2 ] || die "--url 缺少值"; LIANT_URL="$2"; shift 2 ;;
    --beta) BETA=1; shift ;;
    --db) [ "$#" -ge 2 ] || die "--db 缺少值"; DB_DRIVER="$2"; shift 2 ;;
    --db-host) [ "$#" -ge 2 ] || die "--db-host 缺少值"; DB_HOST="$2"; shift 2 ;;
    --db-port) [ "$#" -ge 2 ] || die "--db-port 缺少值"; DB_PORT="$2"; shift 2 ;;
    --db-name) [ "$#" -ge 2 ] || die "--db-name 缺少值"; DB_NAME="$2"; shift 2 ;;
    --db-user) [ "$#" -ge 2 ] || die "--db-user 缺少值"; DB_USER="$2"; shift 2 ;;
    --db-pass) [ "$#" -ge 2 ] || die "--db-pass 缺少值"; DB_PASS="$2"; shift 2 ;;
    -y|--yes) ASSUME_YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*) die "未知选项：$1（用 --help 查看用法）" ;;
    *) [ -z "$SRC" ] || die "只能指定一个二进制路径"; SRC="$1"; shift ;;
  esac
done

[ "$(id -u)" = "0" ] || die "需要 root 运行：sudo bash $0"

# 从终端读输入，即使脚本本身是通过 curl | bash 喂进来的。
prompt() {
  local label="$1" default="$2" answer=""
  if [ "$ASSUME_YES" = "1" ]; then
    printf '%s' "$default"
    return 0
  fi
  if [ -r /dev/tty ]; then
    printf '%s [%s]: ' "$label" "$default" >&2
    read -r answer < /dev/tty || answer=""
  fi
  printf '%s' "${answer:-$default}"
}

prompt_secret() {
  local label="$1" default="$2" answer=""
  if [ "$ASSUME_YES" = "1" ] || [ ! -r /dev/tty ]; then
    [ -n "$default" ] || die "$label 不能为空"
    printf '%s' "$default"
    return 0
  fi
  printf '%s: ' "$label" >&2
  read -rs answer < /dev/tty || answer=""
  printf '\n' >&2
  [ -n "$answer" ] || die "$label 不能为空"
  printf '%s' "$answer"
}

DIR="$(prompt '安装目录' "$DIR")"
PORT="$(prompt '服务端口' "$PORT")"
USER_NAME="$(prompt '运行用户' "$USER_NAME")"
DB_DRIVER="$(prompt '数据库类型 (postgres/sqlite)' "$DB_DRIVER")"
case "$DB_DRIVER" in
  postgres|sqlite) ;;
  *) die "数据库类型只能是 postgres 或 sqlite" ;;
esac
if [ "$DB_DRIVER" = "postgres" ]; then
  DB_HOST="$(prompt 'PostgreSQL 主机' "$DB_HOST")"
  DB_PORT="$(prompt 'PostgreSQL 端口' "$DB_PORT")"
  DB_NAME="$(prompt 'PostgreSQL 数据库名' "$DB_NAME")"
  DB_USER="$(prompt 'PostgreSQL 用户' "$DB_USER")"
  [ -n "$DB_NAME" ] || die "PostgreSQL 数据库名不能为空"
  [ -n "$DB_USER" ] || die "PostgreSQL 用户不能为空"
  DB_PASS="$(prompt_secret 'PostgreSQL 密码' "$DB_PASS")"
fi

DIR="${DIR%/}"
[ -n "$DIR" ] || die "安装目录不能为空"
case "$DIR" in
  /*) ;;
  *) DIR="$(pwd)/$DIR" ;;
esac
[[ "$PORT" =~ ^[0-9]+$ ]] || die "端口必须是数字：$PORT"
[ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ] || die "端口超出范围：$PORT"
[ -n "$USER_NAME" ] || die "运行用户不能为空"

say "安装目录：$DIR"
say "服务端口：$PORT"
say "运行用户：$USER_NAME"

if [ -z "$SRC" ] && [ -z "${LIANT_URL:-}" ]; then
  case "$(uname -m)" in
    x86_64|amd64) asset="LianT-linux-amd64" ;;
    aarch64|arm64) asset="LianT-linux-arm64" ;;
    *) die "不支持的架构：$(uname -m)" ;;
  esac
  say "从 GitHub 取最新构建：$asset"
  SRC="$(mktemp)"
  releases=$(curl -fsSL --retry 3 "https://api.github.com/repos/${REPO}/releases?per_page=30") \
    || die "无法读取 release 列表"
  mode="stable"
  [ "$BETA" = "1" ] && mode="beta"
  selected=$(latest_release_asset "$mode" "$asset" "$releases") || true
  [ -n "$selected" ] || die "没有可用的 release；如需测试版请加 --beta，或手动指定 LIANT_URL"
  IFS=$'\t' read -r tag url _sum <<< "$selected"
  say "选中 release：$tag"
  curl -fSL --retry 3 -o "$SRC" "$url" || die "下载失败"
elif [ -n "${LIANT_URL:-}" ] && [ -z "$SRC" ]; then
  say "下载二进制：$LIANT_URL"
  SRC="$(mktemp)"
  curl -fSL --retry 3 -o "$SRC" "$LIANT_URL" || die "下载失败"
fi
[ -n "$SRC" ] && [ -f "$SRC" ] || die "找不到二进制：$SRC"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) want="x86-64" ;;
  aarch64|arm64) want="ARM aarch64" ;;
  *) die "不支持的架构：$ARCH" ;;
esac
if command -v file >/dev/null 2>&1; then
  desc=$(file -b "$SRC")
  case "$desc" in
    *"$want"*) ;;
    *) die "二进制架构不对：$desc，期望 $want" ;;
  esac
fi

if systemctl list-unit-files liant.service 2>/dev/null | grep -q '^liant\.service'; then
  say "停止已有服务"
  systemctl stop liant || true
fi
if pgrep -f "$DIR/LianT" >/dev/null 2>&1; then
  die "检测到 $DIR/LianT 仍在运行，请先手动停止"
fi

say "创建用户与目录"
id -u "$USER_NAME" >/dev/null 2>&1 || useradd -r -s /sbin/nologin "$USER_NAME"
mkdir -p "$DIR"
install -m 0755 "$SRC" "$DIR/LianT"

if [ ! -f "$DIR/config.enc" ]; then
  say "初始化配置"
  ( cd "$DIR" && ./LianT init )
fi
say "配置数据库：$DB_DRIVER"
( cd "$DIR" && ./LianT config set storage.driver "$DB_DRIVER" )
if [ "$DB_DRIVER" = "postgres" ]; then
  ( cd "$DIR" && ./LianT config set storage.pg_host "$DB_HOST" )
  ( cd "$DIR" && ./LianT config set storage.pg_port "$DB_PORT" )
  ( cd "$DIR" && ./LianT config set storage.pg_database "$DB_NAME" )
  ( cd "$DIR" && ./LianT config set storage.pg_user "$DB_USER" )
  ( cd "$DIR" && ./LianT config set storage.pg_password "$DB_PASS" )
  ( cd "$DIR" && ./LianT config set storage.pg_sslmode disable )
fi
say "校验数据库连接并迁移"
( cd "$DIR" && ./LianT init )
say "写入监听端口：$PORT"
( cd "$DIR" && ./LianT config set connect.listen ":$PORT" )
chown -R "$USER_NAME" "$DIR"

say "写入 systemd 单元"
cat > /etc/systemd/system/liant.service <<UNIT
[Unit]
Description=LianT Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$DIR
ExecStart=$DIR/LianT serve
Restart=always
RestartSec=3
User=$USER_NAME
Environment=LIANT_CONFIRM=auto
Environment=TZ=${TZ:-Asia/Shanghai}

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now liant

IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
PAIR_CMD="cd $DIR && sudo -u $USER_NAME ./LianT pair --host ${IP:-<公网地址>}"
[ "$USER_NAME" = "root" ] && PAIR_CMD="cd $DIR && ./LianT pair --host ${IP:-<公网地址>}"
cat <<INFO

安装完成。

  目录：$DIR
  用户：$USER_NAME
  端口：$PORT
  数据库：$DB_DRIVER
  地址：https://${IP:-<本机地址>}:$PORT
  状态：systemctl status liant
  日志：journalctl -u liant -f

下一步：

  1. 配对：$PAIR_CMD
  2. 反代：Nginx 指向 127.0.0.1:$PORT，并给服务加 LIANT_NO_TLS=1
  3. 机器人：打开 LianT Manager → 机器人 → 添加

INFO

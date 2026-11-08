#!/usr/bin/env bash
set -euo pipefail

DIR="${LIANT_DIR:-}"
SERVICE="liant"
ASSUME_YES="${LIANT_YES:-0}"
PURGE=1
REMOVE_USER=0

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m注意:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'USAGE'
用法：uninstall.sh [选项]

  --dir PATH       安装目录；默认从 systemd 单元读取，找不到用 /opt/liant
  --keep-data      保留配置与消息数据（config.enc、data/）
  --purge          删除配置与数据（默认行为，保留为兼容写法）
  --remove-user    删除安装时创建的服务用户（默认保留）
  -y, --yes        跳过确认（非交互环境必须加）
  -h, --help       显示帮助

默认删除安装目录下的全部内容（含配置、消息、媒体），只保留系统用户。
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir) [ "$#" -ge 2 ] || die "--dir 缺少值"; DIR="$2"; shift 2 ;;
    --purge) PURGE=1; shift ;;
    --keep-data) PURGE=0; shift ;;
    --remove-user) REMOVE_USER=1; shift ;;
    -y|--yes) ASSUME_YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知选项：$1（用 --help 查看用法）" ;;
  esac
done

[ "$(id -u)" = "0" ] || die "需要 root 运行：sudo bash $0"

UNIT_PATH="/etc/systemd/system/${SERVICE}.service"
UNIT_USER=""
if [ -f "$UNIT_PATH" ]; then
  if [ -z "$DIR" ]; then
    DIR="$(awk -F= '/^WorkingDirectory=/{print $2; exit}' "$UNIT_PATH")"
  fi
  UNIT_USER="$(awk -F= '/^User=/{print $2; exit}' "$UNIT_PATH")"
fi
DIR="${DIR:-/opt/liant}"
DIR="${DIR%/}"
[ -n "$DIR" ] || die "安装目录不能为空"
if command -v realpath >/dev/null 2>&1; then
  DIR="$(realpath -m "$DIR")"
fi
case "$DIR" in
  /|/bin|/boot|/dev|/etc|/home|/lib|/lib64|/opt|/proc|/root|/run|/sbin|/srv|/sys|/tmp|/usr|/usr/local|/var)
    die "拒绝操作系统目录：$DIR" ;;
esac
if [ "$DIR" = "${HOME:-/root}" ] || [ "$(dirname "$DIR")" = "/home" ]; then
  die "拒绝删除用户主目录：$DIR"
fi
case "$DIR" in
  /*/*) ;;
  *) die "安装目录层级太浅，拒绝删除：$DIR" ;;
esac
if [ ! -x "$DIR/LianT" ] && [ ! -f "$DIR/config.enc" ] && [ ! -d "$DIR/data" ]; then
  die "$DIR 看起来不是 LianT 安装目录（没有 LianT/config.enc/data）"
fi

say "安装目录：$DIR"
if [ -n "$UNIT_USER" ]; then
  say "服务用户：$UNIT_USER"
fi

if [ ! -f "$UNIT_PATH" ] && [ ! -x "$DIR/LianT" ]; then
  die "没有找到 LianT 的 systemd 单元或 $DIR/LianT"
fi

if [ "$ASSUME_YES" != "1" ]; then
  [ -r /dev/tty ] || die "非交互环境请加 -y"
  if [ "$PURGE" = "1" ]; then
    printf '将停止并移除服务，删除 %s 下全部内容（含配置、消息、媒体）。确认？[y/N] ' "$DIR" >&2
  else
    printf '将停止并移除服务，删除 %s/LianT，保留配置与数据。确认？[y/N] ' "$DIR" >&2
  fi
  answer=""
  read -r answer < /dev/tty || answer=""
  case "$answer" in
    y|Y|yes|YES) ;;
    *) say "已取消"; exit 0 ;;
  esac
fi

if systemctl list-unit-files "${SERVICE}.service" 2>/dev/null | grep -q "^${SERVICE}\.service"; then
  say "停止并禁用 ${SERVICE}"
  systemctl stop "$SERVICE" || true
  systemctl disable "$SERVICE" || true
fi
if [ -f "$UNIT_PATH" ]; then
  say "移除 systemd 单元"
  rm -f "$UNIT_PATH"
  systemctl daemon-reload || true
fi

if pgrep -f "$DIR/LianT" >/dev/null 2>&1; then
  warn "仍有 $DIR/LianT 进程在运行（可能是前台 pair/serve），请手动结束"
fi

if [ "$PURGE" = "1" ] && [ -x "$DIR/LianT" ] && [ -f "$DIR/config.enc" ]; then
  say "清理数据库"
  ( cd "$DIR" && ./LianT db wipe ) || warn "数据库清理失败，请手动处理"
fi
if [ -x "$DIR/LianT" ]; then
  say "删除二进制"
  rm -f "$DIR/LianT"
fi
rm -f "$DIR/LianT.bak"

if [ "$PURGE" = "1" ]; then
  if [ -d "$DIR" ]; then
    say "删除安装目录与数据：$DIR"
    rm -rf "$DIR"
  fi
else
  say "已保留配置与数据：$DIR/config.enc、$DIR/data"
fi

if [ "$REMOVE_USER" = "1" ] && [ -n "$UNIT_USER" ] && [ "$UNIT_USER" != "root" ]; then
  say "删除服务用户：$UNIT_USER"
  userdel "$UNIT_USER" 2>/dev/null || warn "用户 $UNIT_USER 删除失败（可能还有进程或文件），请手动处理"
fi

say "完成"

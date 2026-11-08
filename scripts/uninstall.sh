#!/usr/bin/env bash
set -uo pipefail

DIR="${LIANT_DIR:-}"
SERVICE="liant"
ASSUME_YES="${LIANT_YES:-0}"
PURGE=1
REMOVE_USER=1

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m注意:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'USAGE'
用法：uninstall.sh [选项]

  --dir PATH       安装目录；默认从 systemd 单元读取，找不到用 /opt/liant
  --keep-data      保留配置与消息数据（config.enc、data/）
  --purge          删除配置与数据（默认行为）
  --keep-user      保留服务用户（默认删除）
  --remove-user    删除服务用户（默认）
  -y, --yes        跳过确认（非交互环境必须加）
  -h, --help       显示帮助

不要求目录完整：有 systemd 单元就停/删单元，有进程就结束，有二进制和配置就清库，
最后删掉安装目录。缺哪一步就跳过哪一步，不会因此退出。
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir) [ "$#" -ge 2 ] || die "--dir 缺少值"; DIR="$2"; shift 2 ;;
    --purge) PURGE=1; shift ;;
    --keep-data) PURGE=0; shift ;;
    --keep-user) REMOVE_USER=0; shift ;;
    --remove-user) REMOVE_USER=1; shift ;;
    -y|--yes) ASSUME_YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知选项：$1（用 --help 查看用法）" ;;
  esac
done

[ "$(id -u)" = "0" ] || die "需要 root 运行：sudo bash $0"

UNIT_PATH="/etc/systemd/system/${SERVICE}.service"
UNIT_USER=""
UNIT_DIR=""
if [ -f "$UNIT_PATH" ]; then
  UNIT_DIR="$(awk -F= '/^WorkingDirectory=/{print $2; exit}' "$UNIT_PATH" 2>/dev/null || true)"
  UNIT_USER="$(awk -F= '/^User=/{print $2; exit}' "$UNIT_PATH" 2>/dev/null || true)"
fi
if [ -z "$DIR" ]; then
  DIR="${UNIT_DIR:-/opt/liant}"
fi
DIR="${DIR%/}"
[ -n "$DIR" ] || die "安装目录不能为空"
if command -v realpath >/dev/null 2>&1; then
  DIR="$(realpath -m "$DIR" 2>/dev/null || printf '%s' "$DIR")"
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

# 先收集现状，不因为缺东西就退出。
found=0
[ -f "$UNIT_PATH" ] && found=1
[ -e "$DIR" ] && found=1
[ -n "$UNIT_USER" ] && found=1
if [ "$found" = "0" ]; then
  say "没有发现 LianT 的 systemd 单元或安装目录，视为已卸载"
  exit 0
fi

say "安装目录：$DIR"
[ -n "$UNIT_USER" ] && say "服务用户：$UNIT_USER"
[ -d "$DIR" ] || warn "$DIR 不存在，仍会清理 systemd 单元和用户"

if [ "$ASSUME_YES" != "1" ]; then
  if [ -r /dev/tty ]; then
    if [ "$PURGE" = "1" ]; then
      printf '将停止服务、清库并删除 %s 下全部内容。确认？[y/N] ' "$DIR" >&2
    else
      printf '将停止服务并删除 %s/LianT，保留配置与数据。确认？[y/N] ' "$DIR" >&2
    fi
    answer=""
    read -r answer < /dev/tty || answer=""
    case "$answer" in
      y|Y|yes|YES) ;;
      *) say "已取消"; exit 0 ;;
    esac
  else
    die "非交互环境请加 -y"
  fi
fi

if systemctl list-unit-files "${SERVICE}.service" >/dev/null 2>&1; then
  say "停止并禁用 ${SERVICE}"
  systemctl stop "$SERVICE" >/dev/null 2>&1 || true
  systemctl disable "$SERVICE" >/dev/null 2>&1 || true
fi
if [ -f "$UNIT_PATH" ]; then
  say "移除 systemd 单元"
  rm -f "$UNIT_PATH" || warn "无法删除 $UNIT_PATH"
  systemctl daemon-reload >/dev/null 2>&1 || true
fi

if pgrep -f -- "$DIR/LianT" >/dev/null 2>&1; then
  say "结束残留的 LianT 进程"
  pkill -TERM -f -- "$DIR/LianT" >/dev/null 2>&1 || true
  sleep 1
  pkill -KILL -f -- "$DIR/LianT" >/dev/null 2>&1 || true
fi

if [ "$PURGE" = "1" ] && [ -x "$DIR/LianT" ] && [ -f "$DIR/config.enc" ]; then
  say "清理数据库"
  ( cd "$DIR" && ./LianT db wipe ) || warn "数据库清理失败（角色/密码可能已变），继续删除本地目录"
elif [ "$PURGE" = "1" ]; then
  warn "缺少 LianT 或 config.enc，无法清理数据库；PostgreSQL 里的表可能仍在"
fi

if [ -x "$DIR/LianT" ]; then
  say "删除二进制"
  rm -f "$DIR/LianT" || warn "无法删除 $DIR/LianT"
fi
rm -f "$DIR/LianT.bak" 2>/dev/null || true

if [ "$PURGE" = "1" ]; then
  if [ -e "$DIR" ]; then
    say "删除安装目录：$DIR"
    rm -rf -- "$DIR" || warn "删除 $DIR 失败，请手动检查"
  fi
else
  say "已保留配置与数据：$DIR/config.enc、$DIR/data"
fi

if [ "$REMOVE_USER" = "1" ] && [ -n "$UNIT_USER" ] && [ "$UNIT_USER" != "root" ]; then
  if id "$UNIT_USER" >/dev/null 2>&1; then
    say "删除服务用户：$UNIT_USER"
    userdel "$UNIT_USER" >/dev/null 2>&1 || warn "用户 $UNIT_USER 删除失败（可能还有进程），请手动处理"
  fi
fi

say "完成"

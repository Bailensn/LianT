#!/usr/bin/env bash
# LianT Service 升级脚本：停服务、备份旧二进制、下载并校验最新 pre-release、替换、重启。
#
# 一键升级（在服务器上执行）：
#   curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/upgrade.sh | sudo bash
#
# 也可以指定二进制或地址：
#   sudo bash upgrade.sh /path/to/LianT-linux-amd64
#   sudo LIANT_URL=https://example.com/LianT-linux-amd64 bash upgrade.sh
set -euo pipefail

DIR="${LIANT_DIR:-}"
REPO="${LIANT_REPO:-Bailensn/LianT}"
SERVICE="liant"
SRC=""

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'USAGE'
用法：upgrade.sh [选项] [二进制路径]

  --dir PATH     安装目录；默认从 systemd 单元的 WorkingDirectory 读取
  --url URL      从指定地址下载二进制
  -y, --yes      不询问（当前无交互，保留给脚本调用）
  -h, --help     显示帮助
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dir) [ "$#" -ge 2 ] || die "--dir 缺少值"; DIR="$2"; shift 2 ;;
    --url) [ "$#" -ge 2 ] || die "--url 缺少值"; LIANT_URL="$2"; shift 2 ;;
    -y|--yes) shift ;;
    -h|--help) usage; exit 0 ;;
    -*) die "未知选项：$1（用 --help 查看用法）" ;;
    *) [ -z "$SRC" ] || die "只能指定一个二进制路径"; SRC="$1"; shift ;;
  esac
done

[ "$(id -u)" = "0" ] || die "需要 root 运行：sudo bash $0"

if [ -z "$DIR" ] && systemctl list-unit-files "$SERVICE.service" 2>/dev/null | grep -q "^${SERVICE}\.service"; then
  DIR="$(systemctl show -p WorkingDirectory --value "$SERVICE" 2>/dev/null || true)"
  [ "$DIR" = "/" ] && DIR=""
fi
DIR="${DIR:-/opt/liant}"
DIR="${DIR%/}"
[ -n "$DIR" ] || die "安装目录不能为空"
say "安装目录：$DIR"
[ -x "$DIR/LianT" ] || die "$DIR/LianT 不存在，请先用 install.sh 安装"

case "$(uname -m)" in
  x86_64|amd64) asset="LianT-linux-amd64" ;;
  aarch64|arm64) asset="LianT-linux-arm64" ;;
  *) die "不支持的架构：$(uname -m)" ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

has_checksum=0
if [ -n "$SRC" ]; then
  say "使用本地二进制：$SRC"
  [ -f "$SRC" ] || die "找不到文件：$SRC"
  cp "$SRC" "$tmp/$asset"
elif [ -n "${LIANT_URL:-}" ]; then
  say "下载：$LIANT_URL"
  curl -fSL --retry 3 -o "$tmp/$asset" "$LIANT_URL" || die "下载失败"
else
  say "从 GitHub 取最新构建：$asset"
  releases=$(curl -fsSL --retry 3 "https://api.github.com/repos/${REPO}/releases?per_page=5") || die "无法读取 release 列表"
  bin_url=$(printf '%s' "$releases" | grep -o "https://[^\"]*/${asset}" | head -1) || true
  sum_url=$(printf '%s' "$releases" | grep -o "https://[^\"]*/SHA256SUMS.txt" | head -1) || true
  [ -n "$bin_url" ] || die "没有可用的 release，请检查网络或手动指定 LIANT_URL"
  curl -fSL --retry 3 -o "$tmp/$asset" "$bin_url" || die "下载二进制失败"
  if [ -n "$sum_url" ]; then
    curl -fSL --retry 3 -o "$tmp/SHA256SUMS.txt" "$sum_url" || true
    if [ -f "$tmp/SHA256SUMS.txt" ]; then
      has_checksum=1
    fi
  fi
fi

if [ "$has_checksum" = 1 ]; then
  say "校验 SHA256"
  ( cd "$tmp" && sha256sum -c SHA256SUMS.txt --ignore-missing ) || die "校验失败，已中止"
else
  say "没有校验文件，跳过 SHA256 校验"
fi

has_unit=0
if systemctl list-unit-files "$SERVICE.service" 2>/dev/null | grep -q "^${SERVICE}\.service"; then
  has_unit=1
fi
was_active=0
if [ "$has_unit" = 1 ]; then
  if systemctl is-active --quiet "$SERVICE"; then
    was_active=1
    say "停止 $SERVICE"
    systemctl stop "$SERVICE"
  fi
elif pgrep -f "$DIR/LianT" >/dev/null 2>&1; then
  die "检测到 LianT 进程仍在运行，且没有 systemd 单元；请先手动停止再升级"
fi

backup="$DIR/LianT.bak"
say "备份旧二进制到 $backup"
cp -a "$DIR/LianT" "$backup"

say "替换二进制"
install -m 0755 "$tmp/$asset" "$DIR/LianT"

restart_ok=1
if [ "$has_unit" = 1 ] && [ "$was_active" = 1 ]; then
  say "启动 $SERVICE"
  if ! systemctl start "$SERVICE"; then
    restart_ok=0
  fi
fi

if [ "$restart_ok" = 0 ]; then
  printf '\033[1;31m服务启动失败，回滚到旧版本\033[0m\n' >&2
  cp -a "$backup" "$DIR/LianT"
  systemctl start "$SERVICE" || true
  journalctl -u "$SERVICE" -n 30 --no-pager || true
  exit 1
fi

say "完成"
"$DIR/LianT" version 2>/dev/null || true
if [ "$was_active" = 1 ]; then
  say "服务已重新启动：systemctl status $SERVICE"
else
  say "服务原本未运行；需要时手动 systemctl start $SERVICE"
fi

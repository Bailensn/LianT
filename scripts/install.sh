#!/usr/bin/env bash
# LianT Service 安装脚本：二进制放进 /opt/liant，用 systemd 常驻。
#
#   sudo bash install.sh /path/to/LianT-linux-amd64
#
# 也可以直接给地址：
#   sudo LIANT_URL=https://example.com/LianT-linux-amd64 bash install.sh
set -euo pipefail

DIR="${LIANT_DIR:-/opt/liant}"
PORT="${LIANT_PORT:-8443}"
USER_NAME="${LIANT_USER:-liant}"

say() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m错误:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = "0" ] || die "需要 root 运行：sudo bash $0"

SRC="${1:-}"
REPO="${LIANT_REPO:-Bailensn/LianT}"
if [ -z "$SRC" ] && [ -z "${LIANT_URL:-}" ]; then
  case "$(uname -m)" in
    x86_64|amd64) asset="LianT-linux-amd64" ;;
    aarch64|arm64) asset="LianT-linux-arm64" ;;
    *) die "不支持的架构：$(uname -m)" ;;
  esac
  say "从 GitHub 取最新构建：$asset"
  SRC=$(mktemp)
  url=$(curl -fsSL --retry 3 "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -o "https://[^\"]*/${asset}" | head -1) || true
  [ -n "$url" ] || die "没有可用的 release，请检查网络或手动指定 LIANT_URL"
  curl -fSL --retry 3 -o "$SRC" "$url" || die "下载失败"
elif [ -z "$SRC" ]; then
  say "下载二进制"
  SRC=$(mktemp)
  curl -fSL --retry 3 -o "$SRC" "$LIANT_URL" || die "下载失败"
fi
[ -n "$SRC" ] && [ -f "$SRC" ] || die "用法：install.sh 直接运行即可，或 <二进制路径>/LIANT_URL 覆盖"

ARCH=$(uname -m)
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

say "创建用户与目录"
id -u "$USER_NAME" >/dev/null 2>&1 || useradd -r -s /sbin/nologin "$USER_NAME"
mkdir -p "$DIR"
install -m 0755 "$SRC" "$DIR/LianT"

if [ ! -f "$DIR/config.enc" ]; then
  say "初始化配置"
  ( cd "$DIR" && ./LianT init )
fi

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
User=root
Environment=LIANT_CONFIRM=auto
Environment=TZ=${TZ:-Asia/Shanghai}

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now liant

IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
cat <<INFO

安装完成。

  目录：$DIR
  地址：https://${IP:-<本机地址>}:$PORT
  状态：systemctl status liant
  日志：journalctl -u liant -f

下一步：

  1. 配对：cd $DIR && ./LianT pair --host ${IP:-<公网地址>}
  2. 反代：Nginx 指向 127.0.0.1:$PORT，并给服务加 LIANT_NO_TLS=1
  3. 机器人：打开管理页 → 机器人 → 添加

INFO

#!/usr/bin/env bash
# 构建网页管理端，并把产物放进 Go 的内嵌目录。
#
# 之所以复制到临时目录再构建：Termux 的 /storage 是 FUSE，
# npm 装包需要符号链接，直接在仓库里 install 会失败。
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
work="${LIANT_WEB_BUILD_DIR:-${TMPDIR:-/tmp}/liant-web-build}"

rm -rf "$work"
mkdir -p "$work"
tar -C "$root/web" --exclude=node_modules --exclude=dist -cf - . | tar -C "$work" -xf -

cd "$work"
if [ -f package-lock.json ]; then npm ci --silent; else npm install --silent; fi
npm run build

rm -rf "$root/service/internal/webui/dist"
mkdir -p "$root/service/internal/webui/dist"
cp -r "$work/dist/." "$root/service/internal/webui/dist/"
rm -rf "$root/web/dist"
cp -r "$work/dist" "$root/web/dist"
echo "已生成 service/internal/webui/dist 与 web/dist"

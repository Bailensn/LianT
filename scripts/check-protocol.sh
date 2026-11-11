#!/usr/bin/env bash
set -euo pipefail

# 协议对照：
#   1) 服务端每个消息类型都必须在 docs/PROTOCOL.md 里有记录（硬性，防止悄悄加类型不写文档）；
#   2) 客户端（纯 Kotlin）当前覆盖了哪些类型只做汇报 —— UI 迁移期间还没搬完的会被列出来，
#      等全部搬完再把第 2 条改成硬性检查。
root=$(cd "$(dirname "$0")/.." && pwd)
missing=0
unused=0
total=0

while IFS= read -r typ; do
  total=$((total + 1))
  if ! grep -q "\`$typ\`" "$root/docs/PROTOCOL.md"; then
    echo "PROTOCOL.md 没有记录协议类型：$typ" >&2
    missing=1
  fi
  if ! grep -rqE "[\"']$typ[\"']" "$root/manager/app/src/main/kotlin"; then
    unused=$((unused + 1))
  fi
done < <(grep -rhoE '^\s*Type[A-Za-z]+ += "[a-z]+\.[a-z_.]+"' "$root/service/internal/protocol" \
  | sed 's/.*= "//; s/"//' | sort -u)

if [ "$missing" -ne 0 ]; then
  echo "协议对照失败：服务端有未写入 PROTOCOL.md 的消息类型" >&2
  exit 1
fi

echo "协议对照通过：文档覆盖 $total 个类型；客户端已接入 $((total - unused)) 个（迁移中，未接入 $unused 个）"

#!/usr/bin/env bash
# 协议漂移检查：Go 里声明的每个消息类型，Flutter 客户端都必须提到过。
#
# 不是类型级生成，但能挡住「服务端加了方法、客户端完全不知道」这类漂移，
# 成本很低，CI 里跑几百毫秒。
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
missing=0

while IFS= read -r typ; do
  if ! grep -rqE "['\"]$typ['\"]" "$root/manager/lib"; then
    echo "Flutter 客户端没有使用协议类型：$typ" >&2
    missing=1
  fi
done < <(grep -rhoE '^\s*Type[A-Za-z]+ += "[a-z]+\.[a-z_.]+"' "$root/service/internal/protocol" \
  | sed 's/.*= "//; s/"//' | sort -u)

# 握手与心跳只在底层实现里出现，不要求界面层引用
if [ "$missing" -ne 0 ]; then
  echo "协议对照失败：服务端新增了消息类型，Flutter 客户端需要跟进" >&2
  exit 1
fi

echo "协议对照通过"

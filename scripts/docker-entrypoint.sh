#!/bin/sh
# 容器入口：首次启动自动初始化，然后交给 LianT。
set -e

cd "${LIANT_HOME:-/app}"

if [ ! -f data/config.enc ] && [ ! -f config.enc ]; then
  echo "首次启动，正在初始化数据目录…"
  LianT init
fi

exec LianT "$@"

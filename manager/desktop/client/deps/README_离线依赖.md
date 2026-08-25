# 离线依赖目录 (offline deps)

把预下载的 wheel / 源码包放进本目录，launcher 启动时会**优先从本地安装**，避免慢速联网下载。

## 放什么
下载好的 `.whl`、`.tar.gz`、`.zip` 包，例如：
```
PySide6-6.6.3-cp312-cp312-win_amd64.whl
aiohttp-3.9.5-py3-none-any.whl
```

## 怎么批量拉齐 (在联网机器上)
针对当前客户端依赖 (`client/pyproject.toml`) 与目标平台 / Python 版本：

```bash
# Windows amd64 / Python 3.12 示例
pip download \
  -r <(python3 - <<'EOF'
import tomllib
with open("client/pyproject.toml","rb") as f:
    print("\n".join(tomllib.load(f)["project"]["dependencies"]))
EOF
) \
  -d client/deps \
  --only-binary=:all: \
  --platform win_amd64 --python-version 312
```

> 不同平台 (win_amd64 / macosx_* / manylinux*_x86_64) 包名不同，需分别拉取，或只维护你们真正分发的那一个平台。

## 失效 / 回退
- 本地无匹配版本 → 自动回退联网安装
- 目录为空或不存在 → 完全走联网，行为同以前

## 注意
本目录内容不进 git 提交（体积大、二进制备份），生产用 CI 或发布脚本动态生成即可。
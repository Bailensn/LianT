# 发布规则

## 版本号

格式：`v<主>.<次>.<修>-<通道>-<7 位哈希>`，通道取 `Alpha` / `Beta` / `RCn`：

```
v1.0.0-Alpha-eeeeeee
v1.0.0-Beta-eeeeeee
v1.0.0-RC1-eeeeeee
```

- **基线默认 `v1.0.0`**，通道默认 `Alpha`；只在有意义的里程碑才抬主/次/修或换通道，
  日常构建沿用同一基线、只换哈希（7 位短 SHA）。
- 哈希取当前提交的短 SHA：`git rev-parse --short=7 HEAD`。

## 产物

只有一个多架构镜像：

```
ghcr.io/bailensn/liant:v1.0.0-Alpha-<7位哈希>
ghcr.io/bailensn/liant:latest
```

- `linux/amd64` 与 `linux/arm64` 同时构建；
- 网页管理端在构建镜像时就内嵌进二进制，不单独发布；
- 不再产出独立二进制，也不再创建 GitHub Release。

## 触发方式

**每次 push 到 `main` 都会自动构建并推送镜像**，标签是 `v1.0.0-Alpha-<7 位哈希>`，
同时把 `latest` 指过去。也可以在 Actions 里手动跑 `Publish image`。

## 本地构建

```bash
docker build -t liant:dev .
docker run --rm -p 8443:8443 -v liant_data:/app liant:dev serve
```

想改版本号：

```bash
docker build --build-arg VERSION=v1.0.0-Alpha-local -t liant:dev .
```

`web/` 的 `node_modules` 与 `dist` 都不入库；镜像构建时在容器里现装现编。

## 不做什么

- 不发正式版，只有带哈希的预发布标签。
- 不保留历史镜像以外的产物；`latest` 永远指向 `main` 最新一次构建。

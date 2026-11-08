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
- 版本号只体现在构建产物与 `LianT version` 输出里；仓库不打 tag、不改 `pubspec.yaml`
  的版本（客户端的展示版本由构建时写入）。

## 产物

不再打包 Docker，也不推镜像。发布由两个 workflow 各自产出：

| Workflow | 触发 | 产物 |
| --- | --- | --- |
| `Service binaries` | `service/**` 变更或手动 | GitHub pre-release：`LianT-linux-amd64`、`LianT-linux-arm64`、`SHA256SUMS.txt` |
| `Flutter` | `manager/**` 变更或手动 | Android debug APK、Linux 桌面 bundle；Web 只做编译校验，不发布 |
| `CI` | 所有 push / PR | 协议漂移检查、`go vet`、`go test -race`（不产物） |

- 服务端二进制用 `CGO_ENABLED=0` 构建，直接拷到 `/opt/liant` 就能跑；`install.sh`
  从最新的 pre-release 取资产（注意 GitHub 的 `/releases/latest` 会跳过 pre-release，
  脚本用的是 `/releases?per_page=5`）。
- 服务端二进制不再内嵌网页管理端，只提供 `/v1/ws`、`/v1/media/` 与根路径说明。
- Flutter 的 APK 与桌面产物作为 workflow artifact 上传，不建 Release；
  **Web 构建仅供本地预览，不发布到 gh-pages 或任何公开地址**。

## 本地构建

服务端：

```bash
cd service
VERSION="v1.0.0-Alpha-$(git rev-parse --short=7 HEAD)"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
  -o ../dist/LianT-linux-amd64 ./cmd/liant
```

客户端（Flutter SDK 版本见 `manager/pubspec.yaml` 与 `HANDOFF.md`）：

```bash
cd manager
flutter build web --release --base-href /LianT/
flutter build apk --debug
flutter build linux --release
```

## 不做什么

- 不发正式版，只有带哈希的 pre-release 标签。
- 不恢复 Docker/Compose/GHCR 流程；部署以 systemd + 二进制为准。
- 不单独发布客户端安装包；APK 与桌面产物只作为 CI artifact。
- 不发布 Web 构建（gh-pages 已停用）。
- 不保留历史产物；需要旧版本时从对应提交重新构建。

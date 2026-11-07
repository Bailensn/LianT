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
- **所有发布都是 pre-release**，不覆盖稳定版语义。
- 发布流水线会校验版本格式，不匹配直接失败。

## 产物

| 组件 | 产物 |
| --- | --- |
| Service | `LianT-Service-linux-amd64` / `-linux-arm64` / `-windows-amd64.exe` / `-darwin-arm64` / `-android-arm64` + `SHA256SUMS.txt` |

网页管理端（`web/`）不再单独发产物：构建时由 `scripts/build-web.sh` 打进
`service/internal/webui/dist`，随 Service 二进制一起内嵌。打开 `https://<host>:8443/`
就是管理页，`pair` 命令输出的二维码直接指向它。

`android-arm64` 用 **cgo** 编译（走系统 netd 解析 DNS），因此是动态链接；其余平台静态。
二进制内嵌版本：`./LianT version` 会打印完整 tag 版本（含哈希）。

## 触发方式

**每次 push 到 `main` 都会自动发一个 pre-release**，版本固定为
`v1.0.0-Alpha-<7 位哈希>`（基线不变，只换哈希）。不会发正式 release。

也可以手动重发：`workflow_dispatch` 传已有 tag，或按下面的约定打 tag 推送。

## Tag

统一一个 tag，不带组件前缀：

```
v1.0.0-Alpha-eeeeeee
```

打 tag 同样触发 `Release (pre-release)`；也可以之后用 `workflow_dispatch` 传已有 tag 补发。

```bash
git tag -a v1.0.0-Alpha-$(git rev-parse --short=7 HEAD) -m "LianT v1.0.0-Alpha"
git push origin --tags
```

## 本地构建

```bash
scripts/build-web.sh          # 生成网页产物并放进内嵌目录
cd service && go build ./cmd/liant -ldflags "-X main.version=v1.0.0-Alpha-local"
```

`web/` 的 `node_modules` 与 `dist` 都不入库；不跑 `build-web.sh` 时
`service/internal/webui/dist` 只有占位文件，服务仍能编译，但管理页会返回 404。

## 不做什么

- 不发正式 release，只发 pre-release。
- 不删除历史 pre-release 的产物（如要清理，先确认没有人在用）。

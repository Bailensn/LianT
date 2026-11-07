# 发布规则

## 版本号

格式：`v<主>.<次>.<修>-<通道>-<6 位哈希>`，通道取 `Alpha` / `Beta` / `RCn`：

```
v1.0.0-Alpha-eeeeee
v1.0.0-Beta-eeeeee
v1.0.0-RC1-eeeeee
```

- **基线默认 `v1.0.0`**，通道默认 `Alpha`；只在有意义的里程碑才抬主/次/修或换通道，
  日常构建沿用同一基线、只换哈希。
- 哈希取当前提交的短 SHA：`git rev-parse --short=6 HEAD`（7 位也可以）。
- **所有发布都是 pre-release**，不覆盖稳定版语义。
- 发布流水线会校验版本格式，不匹配直接失败。

## Tag

每个组件独立打 tag，前缀区分：

```
service-v1.0.0-Alpha-eeeeee
manager-v1.0.0-Alpha-eeeeee
```

打 tag 会触发对应发布流水线（`Release Service` / `Release Manager`），构建产物并挂到
pre-release 上；也可以之后用 `workflow_dispatch` 传已有 tag 补发。

```bash
git tag -a service-v1.0.0-Alpha-$(git rev-parse --short=6 HEAD) -m "LianT Service v1.0.0-Alpha"
git push origin --tags
```

## 产物

| 组件 | 产物 |
| --- | --- |
| Service | `LianT-Service-linux-amd64` / `-linux-arm64` / `-windows-amd64.exe` / `-darwin-arm64` / `-android-arm64` + `SHA256SUMS.txt` |
| Manager | `LianT-manager-v1.0.0-<通道>-<hash>.apk`（debug 签名） |

`android-arm64` 用 **cgo** 编译（走系统 netd 解析 DNS），因此是动态链接；其余平台静态。
二进制内嵌版本：`./LianT version` 会打印完整 tag 版本（含哈希）。

## 不做什么

- 不自动在每次 push 上发版；只在需要时打 tag。
- 不删除历史 pre-release 的产物（如要清理，先确认没有人在用）。

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
  （`v1.0.0-Alpha-661681` 这类 6 位的是首个 pre-release，保留不动。）
- **所有发布都是 pre-release**，不覆盖稳定版语义。
- 发布流水线会校验版本格式，不匹配直接失败。

## versionCode

计数从 **1** 开始（Android 强制要求正整数，0 会被 Gradle 拒绝），每构建一次 +1：

```
versionCode = total_runs - version-code-base.txt 里的基线   （最小 1）
```

- 基线在 `manager/android/version-code-base.txt`，是启用该方案时的运行数；
- 之后任何工作流跑一次，`total_runs` +1，`versionCode` 就 +1；
- 本地 `assembleDebug` 不查询接口，退回 `1`（仅开发用）；
- 之前用「Unix 秒」生成的包（`1794054171` 那种）如果装过，需要卸载重装。

`versionName` 仍然保留完整的 `v1.0.0-Alpha-<hash>`。

## 触发方式

**每次 push 到 `main` 都会自动发一个 pre-release**，版本固定为
`v1.0.0-Alpha-<7 位哈希>`（基线不变，只换哈希）。Service 二进制和 Manager APK
**放在同一个 release 里**，不再分开发。不会发正式 release。

也可以手动重发：`workflow_dispatch` 传已有 tag，或按下面的约定打 tag 推送。

## Tag

统一一个 tag，不再带组件前缀：

```
v1.0.0-Alpha-eeeeeee
```

打 tag 会触发对应发布流水线（`Release Service` / `Release Manager`），构建产物并挂到
pre-release 上；也可以之后用 `workflow_dispatch` 传已有 tag 补发。

```bash
git tag -a v1.0.0-Alpha-$(git rev-parse --short=7 HEAD) -m "LianT v1.0.0-Alpha"
git push origin --tags
```

## 产物

| 组件 | 产物 |
| --- | --- |
| Service | `LianT-Service-linux-amd64` / `-linux-arm64` / `-windows-amd64.exe` / `-darwin-arm64` / `-android-arm64` + `SHA256SUMS.txt` |
| Manager | `LianT-manager-v1.0.0-<通道>-<7位hash>.apk`（debug 签名） |

`android-arm64` 用 **cgo** 编译（走系统 netd 解析 DNS），因此是动态链接；其余平台静态。
二进制内嵌版本：`./LianT version` 会打印完整 tag 版本（含哈希）。

## 不做什么

- 不自动在每次 push 上发版；只在需要时打 tag。
- 不删除历史 pre-release 的产物（如要清理，先确认没有人在用）。

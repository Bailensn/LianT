# 发布规则

## 版本号

Service 正式版：`v<主>.<次>.<修>`，例如 `v1.0.0`。
CI 日常测试版：`v1.0.0-Alpha-<7 位提交哈希>`。

Manager 展示版本固定 `1.0.0`，构建号（versionCode）从 82 起每次发布 +1：

- `pubspec.yaml`：`version: 1.0.0+<code>`
- `android/app/build.gradle`：`def liantVersionCode = <code>`

两个数字必须同步。当前为 `98`。

## 稳定版发布

手动运行 `.github/workflows/release.yml`，输入 `1.0.0`：

- Service：发布 `v1.0.0`，资产 `LianT-linux-amd64`、`LianT-linux-arm64`、`SHA256SUMS.txt`
- Manager：发布 `manager-v1.0.0`，资产 `LianT-arm64.apk`（正式签名）

安装/升级脚本只选 `prerelease=false` 的 Service release；没有稳定版时会提示加 `--beta`。

## Android 签名（JKS）

GitHub Actions 不能直接上传二进制 JKS，所以用 base64 放进 secrets：

```bash
# Linux
base64 -w0 release.jks
# macOS
base64 -i release.jks
```

仓库 Secrets：

- `ANDROID_KEYSTORE_BASE64`：上面输出的 base64
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

Release workflow 会把 JKS 解码到 `manager/android/app/upload-keystore.jks`，生成
`manager/android/key.properties`，再用正式签名构建。缺少任一 keystore secret 时稳定版直接失败。

本地构建时把 JKS 放到 `manager/android/app/`，新建 `manager/android/key.properties`：

```properties
storeFile=release.jks
storePassword=你的密码
keyAlias=你的别名
keyPassword=你的密码
```

`key.properties` 和 `*.jks` 已被 `.gitignore` 忽略，不要提交。

## 日常 CI

| 变更 | 产物 |
| --- | --- |
| `service/**` | pre-release：linux amd64 / arm64 |
| `manager/**` | arm64 APK（有签名 secrets 时正式签名，否则 debug） |
| `docs/**`、`*.md` | 跳过 |

Manager 只发 arm64；x86_64 设备不在支持范围。

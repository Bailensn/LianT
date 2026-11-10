# 迁移计划：Flutter 只做界面，核心逻辑搬到 Kotlin

状态：**未开工**，本文件是给下一个会话的施工说明。
背景：后台进程现在是「第二个 FlutterEngine」跑 Dart 版协议客户端，内存约 50 MB，
低内存机型容易被杀。目标是把 WebSocket / 协议 / 签名 / 身份 / 存储 / 通知全部
搬到 Kotlin，Flutter 只剩界面。

---

## 一、目标结构

```
UI 进程（Flutter）                      :background 进程（纯 Kotlin）
  pages/  ui/  l10n/                      LiantSocket     OkHttp WS + 握手 + 重连
  AppState（展示状态，不搬）               Auth            P-256 签名/验签
  BridgeMessageClient ──本地桥接──▶        BridgeServer    ServerSocket + JSON 行
                                          StateStore      读 liant_background_state.json
                                          Notifier        渠道/去重/免打扰/前台判断
```

关键决定：

- **沿用现有 TCP 桥接**（127.0.0.1 + 随机令牌 + JSON 行），不改 Binder。
  Dart 侧 `BridgeMessageClient` 一行都不用动，只把对面的 `BridgeServer` 换成 Kotlin。
- **`AppState` 留在 Dart**。它是「把事件加工成页面展示状态」，搬过去只会让每屏都
  改成订阅数据流，省不了内存（后台进程已经不依赖 Flutter）。
- 后台进程不再创建 `FlutterEngine`，`MessageService` 只做 `startForeground` + Kotlin 核心。

内存预期：后台进程 50 MB → **15–20 MB**。

---

## 二、要搬的东西（按文件）

| Dart 现状 | 行数 | Kotlin 对应 |
| --- | --- | --- |
| `core/net/client.dart` LiantClient | 441 | `net/LiantSocket.kt`：OkHttp WS、信封、`reply_to` 关联、30s 心跳、1→30s 退避重连、4001/4002 不重连 |
| `core/identity/p256.dart` | 477 | `crypto/P256.kt`：`Signature("SHA256withECDSA")`，32 字节标量 → `ECPrivateKey`，SPKI 解析与验签 |
| `core/background/bridge_server.dart` | 206 | `bridge/BridgeServer.kt`：accept 一条连接、令牌握手、`req/send/ev/state` 帧 |
| `core/background/runtime_io.dart` | 190 | `BackgroundRuntime.kt`：读共享状态、免打扰/前台判断、FNV-1a 通知 ID |
| `core/identity/identity.dart` | 230 | `State.kt` 的解析部分（身份 JSON 已在共享文件里） |
| 本地消息存储（Hive） | — | 第二阶段再决定：可留 Dart（省事）或搬 SQLite |

**不搬**：`app_state.dart`、`pages/`、`ui/`、`l10n/`、`BridgeMessageClient`。

---

## 三、协议细节（照着 Dart 实现抄）

1. 连接：`wss://host:port/v1/ws`，自签证书要放行（OkHttp 自定义 `X509TrustManager` +
   `HostnameVerifier`）。真正的信任是下一步的指纹校验。
2. 服务端发 `auth.challenge`：`{nonce, ts, service_pub, service_sig}`。
3. 客户端验 `service_sig`：被签字节是 `"$nonce\n$fingerprint\n$ts"` 的 UTF-8，
   `service_pub` 是 base64(PKIX DER)，ECDSA-SHA256。**指纹以配对时钉下的为准**，
   服务端说不出自己是谁就拒绝连接。
4. 客户端回 `auth.response`：`{manager_id: deviceId, sig: base64(ECDSA_SHA256(同一串))}`，
   外层信封带 `reply_to: challenge.id`。
5. 收到 `auth.ok`：置在线，开始 30s `ping`；服务端 `ping` 要回 `pong`。
6. 收到 `msg.inbound` / `event.msg`：解析 `msg_id / bot_id / peer_id / peer_kind /
   sender_id / sender_name / msg_type / body.text`，走通知逻辑；**同时原样转发给桥接**。
7. 其它事件（`event.bot` / `event.peer` / `event.audit` / `auth.ok`）也必须转发，
   UI 的会话标题、Bot 状态、审计都靠它们。
8. 请求-响应：桥接的 `req` 帧 → 用新信封 id 发出 → 收到 `reply_to == id` 的响应再回
   `res` 帧；超时 15s 回 `code: "timeout"`。

字段名以 `manager/lib/core/net/` 下的代码为准，**不要凭记忆写**。

---

## 四、分三步，每步可回退

### 第一步：Kotlin 协议客户端（只连接 + 认证）

- 新增 `LiantSocket.kt` + `P256.kt`，从状态文件读身份，连上并完成 `auth.response`。
- 不碰现有 Dart 后台；成功与否只写 `liant_crash.log` 或一条调试日志（设置页能看）。
- 真机验证点：**认证通过**（日志里出现 `auth.ok`）。这是最容易翻车的地方——
  DER 编码、SPKI、base64 只要差一个字节就失败。
- 不加依赖以外的改动；OkHttp 需要加进 `app/build.gradle.kts`。

### 第二步：桥接搬到 Kotlin，切流量

- `BridgeServer.kt` 实现同样的帧协议，`liant_bridge.json`（端口/令牌/心跳）继续写。
- 删掉 `MessageService` 里的 `FlutterEngine` 和 `backgroundMain`。
- Dart 侧只保留 `BridgeMessageClient`；删除 `core/background/runtime*.dart`、
  `bridge_server.dart`、`core/net/client.dart`（直连降级也要改走桥接或保留一份）。
- 真机验证：设置页「消息连接」显示后台常驻；收发消息、通知、免打扰都正常。

### 第三步：清理与收尾

- 删 `main.dart` 的 `backgroundMain`、`core/net/socket*`、`p256.dart`（若不再用）。
- 补 JVM 单测：`P256` 用 `p256_test.dart` 里 openssl 生成的向量验签；
  桥接帧用固定 JSON 往返。
- 更新 `docs/HANDOFF.md` 的代码结构图与内存说明。

---

## 五、风险清单

1. **签名互操作**：Dart 用自实现的 P-256；Kotlin 用 JCA。被签字节、DER、base64 必须完全一致。
   先用 openssl 向量做 JVM 单测，再上真机。
2. **自签证书**：OkHttp 默认拒绝；要自定义 TrustManager，且不能丢掉指纹校验。
3. **桥接协议**：UI 侧不改，帧类型/字段错一个就断；建议先把 Dart 版的帧常量抄成 Kotlin 常量。
4. **两条连接并存期**（第一步）：服务端会看到同一设备两条连接，属预期，第二步后消失。
5. **降级路径**：现在桥接连不上时 UI 会直连；客户端换成 Kotlin 后这条降级要重新决定
   （建议：桥接不可用就直接显示「后台未就绪」，不再直连，避免双连接）。

---

## 六、验证手段

- CI 只能验证**编译**和 **JVM 单测**；认证是否通过必须真机看日志。
- 设置页已有的三个诊断位：常驻通知「已收到 N 条」、设置「后台服务」（心跳）、
  「最近的错误」（`liant_crash.log` 最后一条）。
- 内存对比：开发者选项 → 正在运行的服务 / 内存，看 `:background` 进程 PSS 前后变化。

// 后台进程 ↔ UI 进程的本地桥接协议（JSON 行 / TCP 127.0.0.1）。
//
// 为什么不用 shared_preferences / 广播：事件是逐个到达的，需要保序、低延迟，
// 而且 UI 侧还要能把请求发给服务端。一条本地 TCP 连接同时满足两件事。
//
// 安全：监听只绑 127.0.0.1，握手必须带后台进程随机生成的令牌（写在应用私有
// 目录里，别的 App 读不到）。端口号会变，令牌每次后台进程启动都重新生成。
import 'dart:convert';

const int kBridgeProtocolVersion = 1;

/// 客户端 → 服务端：`{"hello": token, "v": 1}`
const String kFrameHello = 'hello';

/// 服务端 → 客户端：握手通过。
const String kFrameReady = 'ready';

/// 服务端 → 客户端：连接状态变化（state 是 ConnState.name）。
const String kFrameState = 'state';

/// 服务端 → 客户端：协议事件信封。
const String kFrameEvent = 'ev';

/// 客户端 → 服务端：请求（id/method/payload）。
const String kFrameRequest = 'req';

/// 客户端 → 服务端：不需要响应的发送（id/method/payload）。
const String kFrameSend = 'send';

/// 服务端 → 客户端：请求结果（id/ok/payload 或 id/ok=false/code/message）。
const String kFrameResponse = 'res';

/// 客户端 → 服务端：主动断开。
const String kFrameBye = 'bye';

String encodeFrame(Map<String, dynamic> frame) => jsonEncode(frame);

Map<String, dynamic>? decodeFrame(String line) {
  if (line.isEmpty) return null;
  try {
    final decoded = jsonDecode(line);
    return decoded is Map<String, dynamic> ? decoded : null;
  } on FormatException {
    return null;
  }
}

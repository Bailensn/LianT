// 消息客户端的统一接口。
//
// 唯一的实现是 BridgeMessageClient（桥接）：真正的 WebSocket 在 `:background`
// 进程的 Kotlin 运行时里，UI 进程通过本地桥接拿事件、发请求。
// 后台没就绪时用 UnavailableClient 占位，不再另开一条直连。
import '../identity/identity.dart';

/// 连接状态，界面按它渲染顶部说明条。
enum ConnState { idle, connecting, online, reconnecting, error }

/// 协议客户端该有的最小表面（AppState 只用到这些）。
abstract class MessageClient {
  Identity get identity;

  bool get online;

  void start();

  void stop();

  /// 发一条不需要响应的消息，返回信封 id。
  String send(String type, Map<String, dynamic> payload);

  /// 发一条请求并等待同 id 的响应；超时与 error 都会抛 ProtocolException。
  Future<Map<String, dynamic>> request(
    String type,
    Map<String, dynamic> payload, {
    Duration timeout = const Duration(seconds: 15),
  });

  /// 把服务端给的媒体相对路径拼成可加载的绝对地址。
  String resolveMediaUrl(String url);
}

/// 建客户端的结果：UI 需要知道当前是不是走后台进程的桥接。
///
/// `bridged == false` 说明后台服务不可用、已经在直连（这时后台若也在跑，
/// 服务端会看到两条连接，通知靠同一个 ID 去重）。
class MessageClientLaunch {
  const MessageClientLaunch(this.client, {required this.bridged});

  final MessageClient client;
  final bool bridged;
}

// 后台桥接不可用时的占位客户端。
//
// 核心下沉 Kotlin 之后 Dart 不再维护直连实现：桥接没起来就明确告诉界面
// 「后台未就绪」，不再自己另开一条 WebSocket（避免双连接与状态分叉）。
import '../identity/identity.dart';
import 'client_api.dart';
import 'protocol.dart';

class UnavailableClient implements MessageClient {
  UnavailableClient({
    required this.identity,
    required this.reason,
    required this.onState,
  });

  @override
  final Identity identity;

  /// 面向用户的说明，会出现在连接状态与设置页「最近状态」里。
  final String reason;

  final void Function(ConnState, String) onState;

  @override
  bool get online => false;

  @override
  void start() => onState(ConnState.error, reason);

  @override
  void stop() => onState(ConnState.idle, '');

  @override
  String send(String type, Map<String, dynamic> payload) {
    throw ProtocolException('offline', reason);
  }

  @override
  Future<Map<String, dynamic>> request(
    String type,
    Map<String, dynamic> payload, {
    Duration timeout = const Duration(seconds: 15),
  }) {
    return Future<Map<String, dynamic>>.error(
      ProtocolException('offline', reason),
    );
  }

  @override
  String resolveMediaUrl(String url) {
    if (url.startsWith('http://') || url.startsWith('https://')) return url;
    final base = identity.address.httpBase;
    return url.startsWith('/') ? '$base$url' : '$base/$url';
  }
}

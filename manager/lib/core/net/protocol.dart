// 协议 v1 的公共出口：信封、数据对象，以及几个跨进程共用的小工具。
//
// 实现拆在 envelope.dart / models.dart；外部继续 `import 'protocol.dart'` 即可。
export 'envelope.dart';
export 'models.dart';

String convKey(String botId, String peerId) => '$botId|$peerId';

/// 通知 ID：同一条消息在 UI 进程和 `:background` 进程里必须算出同一个值，
/// 两边才不会各弹一次。这里用固定的 FNV-1a，不依赖 String.hashCode 的实现。
int notificationIdFor(String msgId) {
  var hash = 0x811c9dc5;
  for (final unit in msgId.codeUnits) {
    hash ^= unit;
    hash = (hash * 0x01000193) & 0x7fffffff;
  }
  // 1001 是常驻通知的 ID（MessageService.NOTIFICATION_ID），撞上就顺延，
  // 否则消息通知会把「正在后台接收消息」那条盖掉。
  if (hash == 0 || hash == 1001) hash += 1;
  return hash;
}

/// openid 很长，标题只显示后 6 位。
String shortPeer(String peerId) {
  final raw = peerId.contains(':') ? peerId.split(':').skip(1).join(':') : peerId;
  if (raw.isEmpty) return '未知会话';
  if (raw.length > 8) return '用户 ${raw.substring(raw.length - 6)}';
  return '用户 $raw';
}

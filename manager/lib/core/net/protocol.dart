// 协议 v1 的公共出口：信封、数据对象，以及几个跨进程共用的小工具。
//
// 实现拆在 envelope.dart / models.dart；外部继续 `import 'protocol.dart'` 即可。
import 'dart:math';

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

/// 生成一个 UUID v4（不引第三方包；协议只要求全局唯一）。
String newId() {
  final random = Random.secure();
  final bytes = List<int>.generate(16, (_) => random.nextInt(256));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  final hex = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-'
      '${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
}

/// 配对失败原因的文案（与 web/src/lib/ws.ts 保持一致）。
String pairDeniedText(String reason) => switch (reason) {
      'no_ticket' => '服务端没有待处理的配对码',
      'expired' => '配对码已过期，请在服务端重新执行 pair',
      'used' => '配对码已经被使用',
      'mismatch' => '配对码或 nonce 不匹配',
      'devices_full' => '设备数量已达上限',
      'denied' => '服务端拒绝了这次配对',
      'invalid_pub' => '公钥格式不被接受',
      'invalid_sig' => '签名校验失败',
      'name_not_allowed' => '这台设备的名称不在服务端白名单里',
      'internal' => '服务端内部错误',
      _ => '配对失败：$reason',
    };

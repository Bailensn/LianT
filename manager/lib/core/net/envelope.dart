// 协议信封与错误：所有消息外面那层 `{v,id,type,ts,reply_to,payload}`。
import 'dart:convert';

/// 协议信封 `{v,id,type,ts,reply_to,payload}`。
class Envelope {
  Envelope({
    required this.id,
    required this.type,
    required this.ts,
    this.replyTo,
    this.payload = const <String, dynamic>{},
    this.v = 1,
  });

  final int v;
  final String id;
  final String type;
  final int ts;
  final String? replyTo;
  final Map<String, dynamic> payload;

  static Envelope? tryParse(Object? raw) {
    if (raw is! String) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return null;
      final type = decoded['type'];
      final id = decoded['id'];
      if (type is! String || id is! String) return null;
      final payload = decoded['payload'];
      return Envelope(
        v: decoded['v'] is int ? decoded['v'] as int : 1,
        id: id,
        type: type,
        ts: decoded['ts'] is int ? decoded['ts'] as int : 0,
        replyTo: decoded['reply_to'] is String ? decoded['reply_to'] as String : null,
        payload: payload is Map<String, dynamic> ? payload : const <String, dynamic>{},
      );
    } on FormatException {
      return null;
    }
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
        'v': v,
        'id': id,
        'type': type,
        'ts': ts,
        if (replyTo != null) 'reply_to': replyTo,
        'payload': payload,
      };

  String encode() => jsonEncode(toJson());
}

/// 服务端返回的结构化错误，按协议 §7 的 code 分类。
class ProtocolException implements Exception {
  const ProtocolException(this.code, this.message, {this.retryable = false});

  final String code;
  final String message;
  final bool retryable;

  /// 面向用户的中文说明；未知 code 退回服务端原文。
  String get friendly => switch (code) {
        'offline' => '还没有连上服务',
        'unauthorized' => '这台设备没有通过认证，请重新配对',
        'forbidden' => '没有这个机器人的权限',
        'not_found' => 'Bot 或消息不存在',
        'invalid_payload' => message.isEmpty ? '请求参数不合法' : message,
        'invalid_doc' => '消息格式不合法',
        'content_too_long' => '内容超过平台长度限制',
        'unsupported_capability' => '当前平台不支持这个能力',
        'reply_window_closed' => '回复窗口已过，只能改用主动消息',
        'mention_required' => '这个会话需要先 @ 机器人',
        'unsupported_version' => '客户端与服务端版本不兼容',
        'rate_limited' => '操作太频繁，稍后再试',
        'clock_skew' => '设备时间与服务端偏差过大',
        'bot_start_failed' => '机器人启动失败',
        'upstream_error' => '平台接口报错',
        'media_expired' => '媒体已超过保留期',
        'media_evicted' => '媒体因空间不足被清理',
        'internal' => '服务端出了点问题',
        'timeout' => '请求超时',
        _ => message.isEmpty ? '操作失败（$code）' : message,
      };

  @override
  String toString() => friendly;
}

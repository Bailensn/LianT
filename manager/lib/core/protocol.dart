// 协议 v1 的数据模型：信封、Bot、消息、设备、审计。
//
// 只放「线上形状」的解析，不放业务状态；业务状态在 app_state.dart。
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

/// Bot 的限制条件，界面只按这里渲染，不内置平台假设。
class BotConstraints {
  const BotConstraints({
    this.canInitiate = true,
    this.replyWindowS,
    this.maxRepliesPerInbound,
    this.requiresMention = false,
  });

  final bool canInitiate;
  final int? replyWindowS;
  final int? maxRepliesPerInbound;
  final bool requiresMention;

  factory BotConstraints.fromJson(Map<String, dynamic>? json) {
    if (json == null) return const BotConstraints();
    final canInitiate = json['can_initiate'];
    final replyWindow = json['reply_window_s'];
    final maxReplies = json['max_replies_per_inbound'];
    return BotConstraints(
      canInitiate: canInitiate is bool ? canInitiate : true,
      replyWindowS: replyWindow is num ? replyWindow.toInt() : null,
      maxRepliesPerInbound: maxReplies is num ? maxReplies.toInt() : null,
      requiresMention: json['requires_mention'] is bool ? json['requires_mention'] as bool : false,
    );
  }
}

/// bot.list / state.snapshot 里的 Bot。
class Bot {
  const Bot({
    required this.botId,
    required this.platform,
    required this.status,
    this.username = '',
    this.displayName = '',
    this.caps = const <String>[],
    this.constraints = const BotConstraints(),
    this.since,
  });

  final String botId;
  final String platform;
  final String status;
  final String username;
  final String displayName;
  final List<String> caps;
  final BotConstraints constraints;
  final int? since;

  bool get running => status == 'running';

  String get title {
    if (displayName.isNotEmpty) return displayName;
    if (username.isNotEmpty) return username;
    return botId;
  }

  String get initial => title.isEmpty ? '?' : title.characters;
}

extension on String {
  /// 取首字符（按 Unicode 码点，不拆代理对）。
  String get characters => isEmpty ? '?' : String.fromCharCode(runes.first);
}

Bot botFromJson(Map<String, dynamic> json) {
  final caps = json['caps'];
  return Bot(
    botId: json['bot_id'] as String? ?? '',
    platform: json['platform'] as String? ?? '',
    status: json['status'] as String? ?? 'stopped',
    username: json['username'] as String? ?? '',
    displayName: json['display_name'] as String? ?? '',
    caps: caps is List ? caps.whereType<String>().toList() : const <String>[],
    constraints: BotConstraints.fromJson(json['constraints'] as Map<String, dynamic>?),
    since: json['since'] is num ? (json['since'] as num).toInt() : null,
  );
}

/// 消息内容：plain 必填，doc 可选（v1 只认 plain，富文本按段落降级渲染）。
class MsgBody {
  const MsgBody({required this.plain, this.doc});

  final String plain;
  final Map<String, dynamic>? doc;

  factory MsgBody.fromJson(Map<String, dynamic>? json) {
    if (json == null) return const MsgBody(plain: '');
    final doc = json['doc'];
    return MsgBody(
      plain: json['plain'] as String? ?? '',
      doc: doc is Map<String, dynamic> ? doc : null,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
        'plain': plain,
        if (doc != null) 'doc': doc,
      };
}

/// 一条消息（入站事件、历史、乐观上屏共用）。
class ChatMessage {
  ChatMessage({
    required this.msgId,
    required this.botId,
    required this.peerId,
    required this.peerKind,
    required this.body,
    required this.ts,
    this.direction = 'in',
    this.msgType = 'text',
    this.senderId = '',
    this.senderName = '',
    this.senderRole = '',
    this.inReplyTo,
    this.pending = false,
    this.failed = false,
    this.active = false,
  });

  final String msgId;
  final String botId;
  final String peerId;
  final String peerKind;
  final MsgBody body;
  final int ts;
  final String direction;
  final String msgType;
  final String senderId;
  final String senderName;
  final String senderRole;
  final String? inReplyTo;
  final bool pending;
  final bool failed;
  final bool active;

  bool get outgoing => direction == 'out';
  bool get isMedia => msgType.isNotEmpty && msgType != 'text';

  ChatMessage copyWith({
    String? msgId,
    int? ts,
    String? inReplyTo,
    bool? pending,
    bool? failed,
  }) {
    return ChatMessage(
      msgId: msgId ?? this.msgId,
      botId: botId,
      peerId: peerId,
      peerKind: peerKind,
      body: body,
      ts: ts ?? this.ts,
      direction: direction,
      msgType: msgType,
      senderId: senderId,
      senderName: senderName,
      senderRole: senderRole,
      inReplyTo: inReplyTo ?? this.inReplyTo,
      pending: pending ?? this.pending,
      failed: failed ?? this.failed,
      active: active,
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
        'msg_id': msgId,
        'bot_id': botId,
        'peer_id': peerId,
        'peer_kind': peerKind,
        'body': body.toJson(),
        'ts': ts,
        'direction': direction,
        'msg_type': msgType,
        if (senderId.isNotEmpty) 'sender_id': senderId,
        if (senderName.isNotEmpty) 'sender_name': senderName,
        if (senderRole.isNotEmpty) 'sender_role': senderRole,
        if (inReplyTo != null) 'in_reply_to': inReplyTo,
      };

  factory ChatMessage.fromJson(Map<String, dynamic> json, {String? direction}) {
    return ChatMessage(
      msgId: json['msg_id'] as String? ?? '',
      botId: json['bot_id'] as String? ?? '',
      peerId: json['peer_id'] as String? ?? '',
      peerKind: json['peer_kind'] as String? ?? 'private',
      body: MsgBody.fromJson(json['body'] as Map<String, dynamic>?),
      ts: json['ts'] is num ? (json['ts'] as num).toInt() : 0,
      direction: direction ?? (json['direction'] as String? ?? 'in'),
      msgType: json['msg_type'] as String? ?? 'text',
      senderId: json['sender_id'] as String? ?? '',
      senderName: json['sender_name'] as String? ?? '',
      senderRole: json['sender_role'] as String? ?? '',
      inReplyTo: json['in_reply_to'] as String?,
    );
  }
}

/// 已配对设备。
class DeviceInfo {
  const DeviceInfo({
    required this.deviceId,
    required this.name,
    required this.createdAt,
    required this.lastSeenAt,
  });

  final String deviceId;
  final String name;
  final int createdAt;
  final int lastSeenAt;

  factory DeviceInfo.fromJson(Map<String, dynamic> json) => DeviceInfo(
        deviceId: json['device_id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        createdAt: json['created_at'] is num ? (json['created_at'] as num).toInt() : 0,
        lastSeenAt: json['last_seen_at'] is num ? (json['last_seen_at'] as num).toInt() : 0,
      );
}

/// 安全日志条目。
class AuditEntry {
  const AuditEntry({
    required this.ts,
    required this.kind,
    this.deviceId = '',
    this.name = '',
    this.ip = '',
    this.reason = '',
  });

  final int ts;
  final String kind;
  final String deviceId;
  final String name;
  final String ip;
  final String reason;

  factory AuditEntry.fromJson(Map<String, dynamic> json) => AuditEntry(
        ts: json['ts'] is num ? (json['ts'] as num).toInt() : 0,
        kind: json['kind'] as String? ?? '',
        deviceId: json['device_id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        ip: json['ip'] as String? ?? '',
        reason: json['reason'] as String? ?? '',
      );
}

/// auth.ok 下发的保留策略。
class Retention {
  const Retention({
    this.days = 7,
    this.textMaxBytes = 0,
    this.mediaMaxBytes = 0,
    this.cleanupIntervalS = 0,
    this.evicted = false,
  });

  final int days;
  final int textMaxBytes;
  final int mediaMaxBytes;
  final int cleanupIntervalS;
  final bool evicted;

  factory Retention.fromJson(Map<String, dynamic>? json) {
    if (json == null) return const Retention();
    int read(String key, int fallback) =>
        json[key] is num ? (json[key] as num).toInt() : fallback;
    return Retention(
      days: read('days', 7),
      textMaxBytes: read('text_max_bytes', 0),
      mediaMaxBytes: read('media_max_bytes', 0),
      cleanupIntervalS: read('cleanup_interval_s', 0),
      evicted: json['evicted'] == true,
    );
  }
}

/// 会话键：协议规定 peer_id 只在单个 bot 内唯一，所以永远带 bot。
String convKey(String botId, String peerId) => '$botId|$peerId';

/// openid 很长，标题只显示后 6 位。
String shortPeer(String peerId) {
  final raw = peerId.contains(':') ? peerId.split(':').skip(1).join(':') : peerId;
  if (raw.isEmpty) return '未知会话';
  if (raw.length > 8) return '用户 ${raw.substring(raw.length - 6)}';
  return '用户 $raw';
}

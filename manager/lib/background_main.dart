import 'dart:async';
import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'core/client.dart';
import 'core/identity.dart';
import 'core/protocol.dart';

const MethodChannel _notifyChannel = MethodChannel('liant/background_notify');

@pragma('vm:entry-point')
void backgroundMain() {
  WidgetsFlutterBinding.ensureInitialized();
  unawaited(_run());
}

Future<void> _run() async {
  final prefs = await SharedPreferences.getInstance();
  final raw = prefs.getString('liant.identity');
  if (raw == null) return;
  final decoded = jsonDecode(raw);
  if (decoded is! Map<String, dynamic>) return;
  final identity = Identity.fromJson(decoded);
  if (identity == null) return;
  final client = LiantClient(
    identity: identity,
    onState: (_, __) {},
    onEvent: (env) {
      if (env.type != 'msg.inbound' && env.type != 'event.msg') return;
      final msg = ChatMessage.fromJson(env.payload, direction: 'in');
      if (msg.msgId.isEmpty || msg.botId.isEmpty) return;
      unawaited(_notify(prefs, msg));
    },
  );
  client.start();
}

Future<void> _notify(SharedPreferences prefs, ChatMessage msg) async {
  await prefs.reload();
  if (prefs.getBool('liant.foreground') ?? false) return;
  final key = convKey(msg.botId, msg.peerId);
  final muted = _loadBoolMap(prefs.getString('liant.muted'));
  if (muted[key] == true) return;
  final titles = _loadStringMap(prefs.getString('liant.titles'));
  final remarks = _loadStringMap(prefs.getString('liant.remarks'));
  final isGroup = msg.peerKind == 'group';
  final sender = msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName;
  final title = _clip(
    isGroup
        ? (remarks[key] ?? titles[key] ?? (sender.isEmpty ? shortPeer(msg.peerId) : sender))
        : (msg.senderName.isEmpty
            ? (remarks[key] ?? titles[key] ?? shortPeer(msg.peerId))
            : msg.senderName),
    36,
  );
  final summary = _summary(msg);
  final body = _clip(isGroup ? '$sender: $summary' : summary, 90);
  await _notifyChannel.invokeMethod<void>('notify', <String, dynamic>{
    'id': msg.msgId.hashCode & 0x7fffffff,
    'title': title,
    'body': body,
  });
}

Map<String, bool> _loadBoolMap(String? raw) {
  if (raw == null || raw.isEmpty) return <String, bool>{};
  try {
    final value = jsonDecode(raw);
    if (value is! Map) return <String, bool>{};
    return <String, bool>{
      for (final entry in value.entries)
        if (entry.key is String && entry.value is bool)
          entry.key as String: entry.value as bool,
    };
  } catch (_) {
    return <String, bool>{};
  }
}

Map<String, String> _loadStringMap(String? raw) {
  if (raw == null || raw.isEmpty) return <String, String>{};
  try {
    final value = jsonDecode(raw);
    if (value is! Map) return <String, String>{};
    return <String, String>{
      for (final entry in value.entries)
        if (entry.key is String && entry.value is String)
          entry.key as String: entry.value as String,
    };
  } catch (_) {
    return <String, String>{};
  }
}

String _summary(ChatMessage msg) {
  switch (msg.msgType) {
    case 'image':
      return '[图片]';
    case 'video':
      return '[视频]';
    case 'audio':
      return '[语音]';
    case 'file':
      final name = msg.body.plain.trim();
      return name.isEmpty ? '[文件]' : '[文件]$name';
    default:
      final text = msg.body.plain.trim();
      return text.isEmpty ? '[消息]' : text;
  }
}

String _clip(String text, int max) {
  final flat = text.replaceAll(RegExp(r'\s+'), ' ').trim();
  if (flat.length <= max) return flat;
  return '${flat.substring(0, max)}...';
}

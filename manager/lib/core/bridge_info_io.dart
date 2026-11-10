// 桥接信息：后台进程把「本地端口 + 本次启动的令牌」写进这个文件，UI 进程读它接入。
//
// 文件路径由原生侧给出（liant/background_state 通道的 bridgePath），
// 两个进程的 filesDir 是同一个目录。
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

const MethodChannel _channel = MethodChannel('liant/background_state');

class BridgeInfo {
  const BridgeInfo({required this.port, required this.token, required this.at});

  final int port;
  final String token;

  /// 写入时间（毫秒）。UI 用新鲜度判断后台进程是否还活着。
  final int at;

  Map<String, dynamic> toJson() => <String, dynamic>{
        'port': port,
        'token': token,
        'at': at,
      };

  static BridgeInfo? fromJson(Map<String, dynamic> json) {
    final port = json['port'];
    final token = json['token'];
    if (port is! int || port <= 0 || port > 65535) return null;
    if (token is! String || token.isEmpty) return null;
    final at = json['at'];
    return BridgeInfo(
      port: port,
      token: token,
      at: at is int ? at : 0,
    );
  }
}

String? _cachedPath;

Future<String?> _infoPath() async {
  final cached = _cachedPath;
  if (cached != null) return cached;
  try {
    final path = await _channel.invokeMethod<String>('bridgePath');
    if (path == null || path.isEmpty) return null;
    _cachedPath = path;
    return path;
  } on MissingPluginException {
    return null;
  } catch (_) {
    return null;
  }
}

/// 后台进程调用：写下端口与令牌（先写临时文件再改名，避免读到半截）。
Future<void> writeBridgeInfo(BridgeInfo info) async {
  final path = await _infoPath();
  if (path == null) return;
  try {
    final temp = File('$path.tmp');
    await temp.writeAsString(jsonEncode(info.toJson()), flush: true);
    await temp.rename(path);
  } catch (_) {
    // 写不进去就是后台桥接不可用，UI 会退回直连。
  }
}

/// 后台进程调用：退出时删掉，免得 UI 去连一个已经不存在的端口。
Future<void> clearBridgeInfo() async {
  final path = await _infoPath();
  if (path == null) return;
  try {
    final file = File(path);
    if (await file.exists()) await file.delete();
  } catch (_) {}
}

/// UI 进程调用：读桥接信息；没有文件或内容损坏时返回 null。
Future<BridgeInfo?> readBridgeInfo() async {
  final path = await _infoPath();
  if (path == null) return null;
  try {
    final raw = await File(path).readAsString();
    final decoded = jsonDecode(raw);
    if (decoded is! Map<String, dynamic>) return null;
    return BridgeInfo.fromJson(decoded);
  } catch (_) {
    return null;
  }
}

/// 后台进程是否还活着：桥接信息足够新。
///
/// 后台进程每 30 秒刷新一次 `at`，所以 90 秒没更新就当作已退出。
Future<bool> backgroundAlive({
  Duration within = const Duration(seconds: 90),
}) async {
  final info = await readBridgeInfo();
  if (info == null || info.at <= 0) return false;
  final age = DateTime.now().millisecondsSinceEpoch - info.at;
  return age >= 0 && age <= within.inMilliseconds;
}

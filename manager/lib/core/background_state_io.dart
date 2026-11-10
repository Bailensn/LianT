// 后台进程与 UI 进程共享的通知状态。
//
// 为什么不用 shared_preferences：Android 的 SharedPreferences 在每个进程里各有
// 一份内存缓存，`reload()` 也不会真的回读磁盘，跨进程读到的经常是旧值。
// 所以这里写一个只读共享的 JSON 文件：UI 进程写，`:background` 进程读。
//
// 文件路径由原生侧给出（两个进程的 filesDir 是同一个目录）。
import 'dart:convert';
import 'dart:io';

import 'package:flutter/services.dart';

import 'identity.dart';

/// 原生侧用来回答「状态文件在哪」的通道；MainActivity 与 MessageService 都注册。
const MethodChannel _stateChannel = MethodChannel('liant/background_state');

/// 状态文件版本号；结构变了就加一，旧后台进程读到不认识的版本会直接退出。
const int _kStateVersion = 1;

class BackgroundState {
  const BackgroundState({
    required this.identity,
    required this.foreground,
    required this.muted,
    required this.titles,
    required this.remarks,
  });

  final Identity identity;

  /// UI 是否在前台；为 true 时后台进程不弹通知，交给 UI 进程。
  final bool foreground;

  /// 会话 key（`$botId|$peerId`）→ 是否免打扰。
  final Map<String, bool> muted;

  /// 会话 key → 服务端给的名称。
  final Map<String, String> titles;

  /// 会话 key → 本地备注。
  final Map<String, String> remarks;

  Map<String, dynamic> toJson() => <String, dynamic>{
        'v': _kStateVersion,
        'identity': identity.toJson(),
        'foreground': foreground,
        'muted': muted,
        'titles': titles,
        'remarks': remarks,
      };

  static BackgroundState? fromJson(Map<String, dynamic> json) {
    if (json['v'] != _kStateVersion) return null;
    final rawIdentity = json['identity'];
    if (rawIdentity is! Map<String, dynamic>) return null;
    final identity = Identity.fromJson(rawIdentity);
    if (identity == null || identity.host.isEmpty) return null;
    return BackgroundState(
      identity: identity,
      foreground: json['foreground'] == true,
      muted: _boolMap(json['muted']),
      titles: _stringMap(json['titles']),
      remarks: _stringMap(json['remarks']),
    );
  }
}

Map<String, bool> _boolMap(Object? raw) {
  if (raw is! Map) return <String, bool>{};
  return <String, bool>{
    for (final entry in raw.entries)
      if (entry.key is String && entry.value is bool)
        entry.key as String: entry.value as bool,
  };
}

Map<String, String> _stringMap(Object? raw) {
  if (raw is! Map) return <String, String>{};
  return <String, String>{
    for (final entry in raw.entries)
      if (entry.key is String && entry.value is String)
        entry.key as String: entry.value as String,
  };
}

String? _cachedPath;

/// 状态文件绝对路径；非 Android 平台（没有原生通道）返回 null。
Future<String?> _statePath() async {
  final cached = _cachedPath;
  if (cached != null) return cached;
  try {
    final path = await _stateChannel.invokeMethod<String>('statePath');
    if (path == null || path.isEmpty) return null;
    _cachedPath = path;
    return path;
  } on MissingPluginException {
    return null;
  } catch (_) {
    return null;
  }
}

/// 写操作串行化：前后台状态会连续变化，两个写并发用同一个临时文件会互相截断。
Future<void> _writeSeq = Future<void>.value();

/// UI 进程调用：把最新状态写进共享文件。
///
/// 先写临时文件再改名，避免后台进程读到写了一半的 JSON。
Future<void> writeBackgroundState(BackgroundState state) {
  final payload = jsonEncode(state.toJson());
  _writeSeq = _writeSeq.then((_) => _writeNow(payload));
  return _writeSeq;
}

/// UI 进程调用：解除配对时删掉共享状态，后台进程下次启动就不会再连。
Future<void> clearBackgroundState() {
  _writeSeq = _writeSeq.then((_) => _clearNow());
  return _writeSeq;
}

Future<void> _writeNow(String payload) async {
  final path = await _statePath();
  if (path == null) return;
  try {
    final temp = File('$path.tmp');
    await temp.writeAsString(payload, flush: true);
    await temp.rename(path);
  } on FileSystemException {
    // 写不进去就算了：下一轮状态变化会再写一次。
  } catch (_) {}
}

Future<void> _clearNow() async {
  final path = await _statePath();
  if (path == null) return;
  try {
    final file = File(path);
    if (await file.exists()) await file.delete();
    final temp = File('$path.tmp');
    if (await temp.exists()) await temp.delete();
  } on FileSystemException {
    // 忽略：文件本来就不在。
  } catch (_) {}
}

/// 后台进程调用：读共享状态；没有文件或内容损坏时返回 null。
Future<BackgroundState?> readBackgroundState() async {
  final path = await _statePath();
  if (path == null) return null;
  try {
    final raw = await File(path).readAsString();
    final decoded = jsonDecode(raw);
    if (decoded is! Map<String, dynamic>) return null;
    return BackgroundState.fromJson(decoded);
  } on FileSystemException {
    return null;
  } on FormatException {
    return null;
  } catch (_) {
    return null;
  }
}

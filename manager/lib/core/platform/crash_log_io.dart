// 崩溃/未捕获异常落盘。
//
// UI 进程与 :background 进程都会写同一个文件，只保留最后一段，避免无限增长。
import 'dart:io';

import 'package:flutter/services.dart';

const MethodChannel _channel = MethodChannel('liant/background_state');

/// 日志上限：超过就整体截断重写，只留最近的部分。
const int _maxBytes = 16 * 1024;

String? _cachedPath;

Future<String?> _logPath() async {
  final cached = _cachedPath;
  if (cached != null) return cached;
  try {
    final path = await _channel.invokeMethod<String>('logPath');
    if (path == null || path.isEmpty) return null;
    _cachedPath = path;
    return path;
  } on MissingPluginException {
    return null;
  } catch (_) {
    return null;
  }
}

/// 记一条错误：时间 + 类型 + 消息 + 前几行堆栈。
Future<void> recordError(Object error, StackTrace? stack) async {
  final path = await _logPath();
  if (path == null) return;
  final now = DateTime.now().toIso8601String();
  final head = '$now ${error.runtimeType}: $error';
  final trace = (stack ?? StackTrace.empty)
      .toString()
      .split('\n')
      .take(6)
      .join('\n');
  final line = '$head\n$trace\n\n';
  try {
    final file = File(path);
    final exists = await file.exists();
    var length = exists ? await file.length() : 0;
    if (length > _maxBytes) {
      await file.writeAsString(line, flush: true);
      return;
    }
    await file.writeAsString(line, mode: FileMode.append, flush: true);
  } catch (_) {
    // 写不进去就算了，不能让记日志本身把进程搞挂。
  }
}

/// 读最近一条错误（第一段），设置页用来展示。
Future<String?> readLastError() async {
  final path = await _logPath();
  if (path == null) return null;
  try {
    final file = File(path);
    if (!await file.exists()) return null;
    final text = await file.readAsString();
    final trimmed = text.trim();
    if (trimmed.isEmpty) return null;
    final blocks = trimmed.split('\n\n');
    return blocks.last.trim();
  } catch (_) {
    return null;
  }
}

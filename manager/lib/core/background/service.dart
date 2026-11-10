import 'package:flutter/services.dart';

const MethodChannel _channel = MethodChannel('liant/background');

Future<void> startMessageService() async {
  try {
    await _channel.invokeMethod<void>('start');
  } on MissingPluginException {
    // 非 Android（Web/桌面）没有这个通道。
  } catch (_) {}
}

Future<void> stopMessageService() async {
  try {
    await _channel.invokeMethod<void>('stop');
  } on MissingPluginException {
    // 非 Android（Web/桌面）没有这个通道。
  } catch (_) {}
}

Future<void> requestNotificationPermission() async {
  try {
    await _channel.invokeMethod<void>('requestNotifications');
  } on MissingPluginException {
    // 非 Android 没有这个通道。
  } catch (_) {}
}

Future<void> showMessageNotification({
  required int id,
  required String title,
  required String body,
}) async {
  try {
    await _channel.invokeMethod<void>('notify', <String, dynamic>{
      'id': id,
      'title': title,
      'body': body,
    });
  } on MissingPluginException {
    // 非 Android 没有这个通道。
  } catch (_) {}
}

Future<void> openExternalUrl(String url) async {
  if (url.isEmpty) return;
  try {
    await _channel.invokeMethod<void>('openUrl', <String, dynamic>{'url': url});
  } on MissingPluginException {
    // Web/桌面没有原生通道；预览环境就留在这里。
  } catch (_) {}
}

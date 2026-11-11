// 选客户端：Android 上只走 `:background` 进程的本地桥接。
//
// 核心下沉 Kotlin 之后 Dart 已经没有直连实现：桥接没起来就返回一个
// 「后台未就绪」的占位客户端，界面明确显示未就绪，等后台进程恢复后
// 再切回桥接（见 AppState._maybeUpgradeToBridge）。
import 'dart:io';

import '../background/bridge_client.dart';
import '../background/bridge_info.dart';
import '../identity/identity.dart';
import 'client_api.dart';
import 'protocol.dart';
import 'unavailable_client.dart';

/// 建连接客户端。
///
/// Android：先等后台服务把桥接端口写出来（最多 6 秒），接上就用桥接；
/// 等不到返回 [UnavailableClient]，[MessageClientLaunch.bridged] 为 false。
Future<MessageClientLaunch> createMessageClient({
  required Identity identity,
  required void Function(ConnState, String) onState,
  required void Function(Envelope) onEvent,
}) async {
  if (Platform.isAndroid) {
    final deadline = DateTime.now().add(const Duration(seconds: 6));
    while (DateTime.now().isBefore(deadline)) {
      final info = await readBridgeInfo();
      if (info != null) {
        final client = await BridgeMessageClient.connect(
          identity: identity,
          info: info,
          onState: onState,
          onEvent: onEvent,
        );
        if (client != null) return MessageClientLaunch(client, bridged: true);
      }
      await Future<void>.delayed(const Duration(milliseconds: 300));
    }
    return MessageClientLaunch(
      UnavailableClient(
        identity: identity,
        reason: '后台服务未就绪，正在等它启动',
        onState: onState,
      ),
      bridged: false,
    );
  }
  return MessageClientLaunch(
    UnavailableClient(
      identity: identity,
      reason: '当前平台不支持直连，Android 才能连服务',
      onState: onState,
    ),
    bridged: false,
  );
}

/// 后台进程是否还活着（桥接信息的新鲜度）。
Future<bool> backgroundServiceAlive() async {
  if (!Platform.isAndroid) return false;
  return backgroundAlive();
}

/// 后台进程最后一次心跳距今多少毫秒；null 表示没有心跳（没跑起来或已停）。
Future<int?> backgroundServiceAge() async {
  if (!Platform.isAndroid) return null;
  return backgroundAgeMs();
}

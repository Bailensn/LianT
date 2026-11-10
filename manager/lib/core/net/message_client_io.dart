// 选客户端：Android 上优先走 `:background` 进程的本地桥接，
// 让全 App 只有后台进程那一条 WebSocket；桥接连不上才降级直连。
import 'dart:io';

import '../background/bridge_client.dart';
import '../background/bridge_info.dart';
import 'client.dart';
import '../identity/identity.dart';
import 'protocol.dart';

/// 建连接客户端。
///
/// Android：先等后台服务把桥接端口写出来（最多 6 秒），接上就用桥接；
/// 接不上就直连降级，界面会通过 [MessageClientLaunch.bridged] 知道这一点。
/// 其他平台（桌面 / 测试）：直接连。
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
  }
  return MessageClientLaunch(
    LiantClient(identity: identity, onState: onState, onEvent: onEvent),
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

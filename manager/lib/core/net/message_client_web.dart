// Web：没有后台进程，也没有直连实现（Web 只是本地预览，不对外发布）。
import '../identity/identity.dart';
import 'client_api.dart';
import 'protocol.dart';
import 'unavailable_client.dart';

Future<MessageClientLaunch> createMessageClient({
  required Identity identity,
  required void Function(ConnState, String) onState,
  required void Function(Envelope) onEvent,
}) async {
  return MessageClientLaunch(
    UnavailableClient(
      identity: identity,
      reason: 'Web 预览不支持连接服务',
      onState: onState,
    ),
    bridged: false,
  );
}

Future<bool> backgroundServiceAlive() async => false;

Future<int?> backgroundServiceAge() async => null;

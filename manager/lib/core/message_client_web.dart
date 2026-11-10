// Web：没有后台进程，只能直连（浏览器也不允许自签证书绕过，见 socket_web.dart）。
import 'client.dart';
import 'identity.dart';
import 'protocol.dart';

Future<MessageClientLaunch> createMessageClient({
  required Identity identity,
  required void Function(ConnState, String) onState,
  required void Function(Envelope) onEvent,
}) async {
  return MessageClientLaunch(
    LiantClient(identity: identity, onState: onState, onEvent: onEvent),
    bridged: false,
  );
}

Future<bool> backgroundServiceAlive() async => false;

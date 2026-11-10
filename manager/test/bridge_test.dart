// 本地桥接的单元测试：后台进程（BridgeServer）与 UI 进程（BridgeMessageClient）
// 真的走一条 TCP 连接，验证握手、请求转发、事件/状态转发、断线失败。
import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:liant/core/bridge_client_io.dart';
import 'package:liant/core/bridge_info_io.dart';
import 'package:liant/core/bridge_server_io.dart';
import 'package:liant/core/client_api.dart';
import 'package:liant/core/identity.dart';
import 'package:liant/core/p256.dart';
import 'package:liant/core/protocol.dart';

Identity _identity() {
  final key = p256GenerateKey();
  return Identity(
    deviceId: 'dev-1',
    serviceName: 'LianT Service',
    host: '10.0.0.2',
    port: 8443,
    fingerprint: 'aa:bb:cc',
    secure: true,
    privateKey: key.d,
    publicPoint: key.pub,
  );
}

class _FakeClient implements MessageClient {
  _FakeClient(this.identity);

  @override
  final Identity identity;

  final List<String> requests = <String>[];
  final List<String> sends = <String>[];
  Completer<Map<String, dynamic>>? gate;

  @override
  bool get online => true;

  @override
  void start() {}

  @override
  void stop() {}

  @override
  String send(String type, Map<String, dynamic> payload) {
    sends.add(type);
    return 'env-$type';
  }

  @override
  Future<Map<String, dynamic>> request(
    String type,
    Map<String, dynamic> payload, {
    Duration timeout = const Duration(seconds: 15),
  }) {
    requests.add(type);
    final g = gate;
    if (g != null) return g.future;
    return Future<Map<String, dynamic>>.value(<String, dynamic>{
      'echo': type,
      'payload': payload,
    });
  }

  @override
  String resolveMediaUrl(String url) => url;
}

void main() {
  test('桥接：握手 + 请求 + 发送 + 事件 + 状态', () async {
    final fake = _FakeClient(_identity());
    final server = BridgeServer(
      clientOf: () => fake,
      connectionOf: () => (ConnState.online, ''),
    );
    await server.start();
    addTearDown(server.stop);

    final states = <ConnState>[];
    final events = <Envelope>[];
    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port!, token: server.token!, at: 0),
      onState: (state, _) => states.add(state),
      onEvent: events.add,
    );
    expect(client, isNotNull);
    client!.start();
    addTearDown(client.stop);

    // 接上时服务端会先补一条当前状态。
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(states, contains(ConnState.online));

    final res = await client.request('bot.list', <String, dynamic>{'x': 1});
    expect(res['echo'], 'bot.list');
    expect(fake.requests, contains('bot.list'));

    client.send('msg.ack', <String, dynamic>{'msg_ids': <String>['m1']});
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(fake.sends, contains('msg.ack'));

    server.broadcastEvent(
      Envelope(
        id: 'e1',
        type: 'msg.inbound',
        ts: 1,
        payload: <String, dynamic>{'msg_id': 'm1', 'bot_id': 'b1'},
      ),
    );
    server.broadcastState(ConnState.reconnecting, '第 2 次重连');
    await Future<void>.delayed(const Duration(milliseconds: 100));
    expect(events.length, 1);
    expect(events.first.type, 'msg.inbound');
    expect(events.first.payload['msg_id'], 'm1');
    expect(states.last, ConnState.reconnecting);
    expect(client.online, isFalse);
  });

  test('桥接：令牌不对连不上', () async {
    final fake = _FakeClient(_identity());
    final server = BridgeServer(
      clientOf: () => fake,
      connectionOf: () => (ConnState.online, ''),
    );
    await server.start();
    addTearDown(server.stop);

    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port!, token: 'wrong-token', at: 0),
      onState: (_, _) {},
      onEvent: (_) {},
    );
    expect(client, isNull);
  });

  test('桥接：后台断开时挂起的请求失败', () async {
    final fake = _FakeClient(_identity())..gate = Completer<Map<String, dynamic>>();
    final server = BridgeServer(
      clientOf: () => fake,
      connectionOf: () => (ConnState.online, ''),
    );
    await server.start();

    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port!, token: server.token!, at: 0),
      onState: (_, _) {},
      onEvent: (_) {},
    );
    expect(client, isNotNull);
    client!.start();
    await Future<void>.delayed(const Duration(milliseconds: 50));
    // 先挂断言再断开：否则错误在没人监听时完成，会被当成未捕获异常。
    final pending = client.request('slow', const <String, dynamic>{});
    final failed = expectLater(pending, throwsA(isA<ProtocolException>()));
    await Future<void>.delayed(const Duration(milliseconds: 50));
    await server.stop();
    await failed;
  });
}

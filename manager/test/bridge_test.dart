// 本地桥接的单元测试：UI 侧 BridgeMessageClient 对着一台按帧协议实现的假服务端
// 真的走一条 TCP 连接，验证握手、请求转发、发送、事件/状态转发、断线失败。
//
// 后台侧已经换成 Kotlin 的 BridgeServer，这里用一个最小的假服务端守住帧协议：
// 字段一旦改动，这个测试会先红，避免真机上「界面整条连不上」。
import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:liant/core/background/bridge_client.dart';
import 'package:liant/core/background/bridge_info.dart';
import 'package:liant/core/net/client_api.dart';
import 'package:liant/core/identity/identity.dart';
import 'package:liant/core/identity/p256.dart';
import 'package:liant/core/net/protocol.dart';

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

/// 按 `bridge_protocol.dart` 的帧格式实现的假后台，只覆盖测试用得到的部分。
class _FakeBridgeServer {
  _FakeBridgeServer(this.token);

  final String token;
  final List<String> requests = <String>[];
  final List<String> sends = <String>[];

  /// 为 true 时收到 req 先不回复，用来测「断开时挂起的请求失败」。
  bool hold = false;

  ServerSocket? _server;
  Socket? _socket;
  StreamSubscription<String>? _lines;

  int get port => _server!.port;

  Future<void> start() async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    _server = server;
    server.listen((socket) {
      _socket = socket;
      var ready = false;
      _lines = socket
          .cast<List<int>>()
          .transform(utf8.decoder)
          .transform(const LineSplitter())
          .listen(
        (line) {
          final frame = jsonDecode(line) as Map<String, dynamic>;
          if (!ready) {
            if (frame['hello'] != token) {
              socket.destroy();
              return;
            }
            ready = true;
            socket.write('${jsonEncode(<String, dynamic>{'t': 'ready', 'v': 1})}\n');
            socket.write(
              '${jsonEncode(<String, dynamic>{'t': 'state', 'state': 'online', 'detail': ''})}\n',
            );
            return;
          }
          switch (frame['t']) {
            case 'req':
              requests.add(frame['method'] as String);
              if (hold) return;
              socket.write(
                '${jsonEncode(<String, dynamic>{
                  't': 'res',
                  'id': frame['id'],
                  'ok': true,
                  'payload': <String, dynamic>{'echo': frame['method']},
                })}\n',
              );
              break;
            case 'send':
              sends.add(frame['method'] as String);
              socket.write(
                '${jsonEncode(<String, dynamic>{
                  't': 'res',
                  'id': frame['id'],
                  'ok': true,
                  'payload': <String, dynamic>{'id': 'env-${frame['method']}'},
                })}\n',
              );
              break;
            case 'bye':
              socket.destroy();
              break;
          }
        },
        onDone: () => _socket = null,
        onError: (Object _) {},
      );
    });
  }

  void pushEvent(Map<String, dynamic> envelope) {
    _socket?.write('${jsonEncode(<String, dynamic>{'t': 'ev', 'env': envelope})}\n');
  }

  void pushState(String name, String detail) {
    _socket?.write(
      '${jsonEncode(<String, dynamic>{'t': 'state', 'state': name, 'detail': detail})}\n',
    );
  }

  Future<void> stop() async {
    await _lines?.cancel();
    _socket?.destroy();
    _socket = null;
    await _server?.close();
    _server = null;
  }
}

void main() {
  test('桥接：握手 + 请求 + 发送 + 事件 + 状态', () async {
    final server = _FakeBridgeServer('token-1');
    await server.start();
    addTearDown(server.stop);

    final states = <ConnState>[];
    final events = <Envelope>[];
    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port, token: server.token, at: 0),
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
    expect(server.requests, contains('bot.list'));

    client.send('msg.ack', <String, dynamic>{
      'msg_ids': <String>['m1'],
    });
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(server.sends, contains('msg.ack'));

    server.pushEvent(<String, dynamic>{
      'v': 1,
      'id': 'e1',
      'type': 'msg.inbound',
      'ts': 1,
      'payload': <String, dynamic>{'msg_id': 'm1', 'bot_id': 'b1'},
    });
    server.pushState('reconnecting', '第 2 次重连');
    await Future<void>.delayed(const Duration(milliseconds: 100));
    expect(events.length, 1);
    expect(events.first.type, 'msg.inbound');
    expect(events.first.payload['msg_id'], 'm1');
    expect(states.last, ConnState.reconnecting);
    expect(client.online, isFalse);
  });

  test('桥接：令牌不对连不上', () async {
    final server = _FakeBridgeServer('token-1');
    await server.start();
    addTearDown(server.stop);

    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port, token: 'wrong-token', at: 0),
      onState: (_, _) {},
      onEvent: (_) {},
    );
    expect(client, isNull);
  });

  test('桥接：后台断开时挂起的请求失败', () async {
    final server = _FakeBridgeServer('token-1')..hold = true;
    await server.start();

    final client = await BridgeMessageClient.connect(
      identity: _identity(),
      info: BridgeInfo(port: server.port, token: server.token, at: 0),
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

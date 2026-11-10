// 用 dart:io 的 WebSocket 跑一遍 mock 服务的配对与握手，验证协议实现。
import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:liant/core/identity/p256.dart';

Future<void> main(List<String> args) async {
  final uri = Uri.parse(args.isNotEmpty ? args[0] : 'ws://127.0.0.1:8099/v1/ws');
  final ws = await WebSocket.connect(uri.toString());
  final key = p256GenerateKey();
  final pub = p256EncodeSpkiBase64(key.pub);
  final done = Completer<void>();
  String deviceId = '';

  ws.listen((raw) {
    final env = jsonDecode(raw as String) as Map<String, dynamic>;
    final type = env['type'] as String? ?? '';
    stdout.writeln('<- $type');
    if (type == 'auth.challenge') {
      final p = env['payload'] as Map<String, dynamic>;
      final nonce = p['nonce'] as String;
      final ts = p['ts'] as int;
      final fingerprint = p256Fingerprint(base64Decode(p['service_pub'] as String));
      p256VerifyServiceProof(
        servicePubBase64: p['service_pub'] as String,
        expectedFingerprint: fingerprint,
        message: utf8.encode('$nonce\n$fingerprint\n$ts'),
        signatureBase64: p['service_sig'] as String,
      );
      stdout.writeln('challenge verified, fp=$fingerprint');
      ws.add(jsonEncode(<String, dynamic>{
        'v': 1,
        'id': 'pair-1',
        'type': 'pair.request',
        'ts': DateTime.now().millisecondsSinceEpoch,
        'payload': <String, dynamic>{
          'ticket': '',
          'manager_name': 'smoke',
          'manager_pub': pub,
          'sig': '',
        },
      }));
    } else if (type == 'pair.ok') {
      final p = env['payload'] as Map<String, dynamic>;
      deviceId = p['device_id'] as String;
      stdout.writeln('paired device=$deviceId');
      ws.add(jsonEncode(<String, dynamic>{
        'v': 1,
        'id': 'auth-1',
        'type': 'auth.response',
        'ts': DateTime.now().millisecondsSinceEpoch,
        'payload': <String, dynamic>{'manager_id': deviceId, 'sig': ''},
      }));
    } else if (type == 'auth.ok') {
      ws.add(jsonEncode(<String, dynamic>{
        'v': 1,
        'id': 'snap-1',
        'type': 'state.snapshot',
        'ts': DateTime.now().millisecondsSinceEpoch,
        'payload': <String, dynamic>{},
      }));
    } else if (type == 'state.snapshot') {
      final p = env['payload'] as Map<String, dynamic>;
      stdout.writeln(
        'snapshot bots=${(p['bots'] as List).length} devices=${(p['devices'] as List).length}',
      );
      ws.add(jsonEncode(<String, dynamic>{
        'v': 1,
        'id': 'list-1',
        'type': 'msg.list',
        'ts': DateTime.now().millisecondsSinceEpoch,
        'payload': <String, dynamic>{
          'bot_id': 'qqbot:1905703456',
          'peer_id': 'qqbot:group_778899',
          'limit': 100,
        },
      }));
    } else if (type == 'msg.list') {
      final p = env['payload'] as Map<String, dynamic>;
      final messages = p['messages'] as List;
      stdout.writeln('history messages=${messages.length}');
      ws.add(jsonEncode(<String, dynamic>{
        'v': 1,
        'id': 'send-1',
        'type': 'msg.send',
        'ts': DateTime.now().millisecondsSinceEpoch,
        'payload': <String, dynamic>{
          'bot_id': 'qqbot:1905703456',
          'peer_id': 'qqbot:group_778899',
          'body': <String, dynamic>{'plain': 'smoke test'},
          'active': true,
        },
      }));
    } else if (type == 'msg.send') {
      final p = env['payload'] as Map<String, dynamic>;
      stdout.writeln('sent msg_id=${p['msg_id']} ts=${p['ts']}');
      if (!done.isCompleted) done.complete();
    }
  }, onError: (Object e) => stderr.writeln('ws error $e'));

  await done.future.timeout(const Duration(seconds: 10));
  await ws.close();
  exit(0);
}

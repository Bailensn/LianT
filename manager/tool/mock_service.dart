// 本地假服务端：只用于截图自检与界面联调，不参与打包。
//
// 它按协议 v1 的握手流程回 auth.challenge / pair.ok / auth.ok，
// 并提供 state.snapshot、msg.list、msg.send 等方法。数据全在内存里，
// 进程退出即清空；App 端没有任何内置的演示数据。
//
// 用法：
//   dart run tool/mock_service.dart [端口]
// 然后在配对页把服务地址填成 127.0.0.1:8099，点「直接进入」。
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:liant/core/p256.dart';

int _seq = 0;
final Random _random = Random.secure();

String _nowId() => 'mock-${DateTime.now().microsecondsSinceEpoch}-${++_seq}';

String _hex(int length) =>
    List<String>.generate(length, (_) => _random.nextInt(256).toRadixString(16).padLeft(2, '0')).join();

class _Peer {
  _Peer(this.id, this.kind, this.title);

  final String id;
  final String kind;
  final String title;
}

final List<_Peer> _peers = <_Peer>[
  _Peer('qqbot:group_778899', 'group', '产品群'),
  _Peer('qqbot:u_10002', 'private', '张三'),
  _Peer('qqbot:u_10001', 'private', '林可'),
];

final Map<String, List<Map<String, dynamic>>> _history = <String, List<Map<String, dynamic>>>{};

Map<String, dynamic> _message({
  required _Peer peer,
  required String text,
  required int ts,
  String direction = 'in',
  String senderId = '',
  String senderName = '',
  String msgType = 'text',
}) {
  return <String, dynamic>{
    'bot_id': 'qqbot:1905703456',
    'peer_id': peer.id,
    'peer_kind': peer.kind,
    'sender_id': senderId,
    'sender_name': senderName,
    'msg_type': msgType,
    'body': <String, dynamic>{'plain': text},
    'msg_id': 'msg-${_hex(8)}',
    'ts': ts,
    'direction': direction,
  };
}

void _seed() {
  final now = DateTime.now();
  final today = DateTime(now.year, now.month, now.day);
  final group = _peers[0];
  final zhang = _peers[1];
  final lin = _peers[2];
  final groupList = <Map<String, dynamic>>[
    _message(peer: group, text: '今晚打个包，明天上午发', ts: today.add(const Duration(hours: 9, minutes: 12)).millisecondsSinceEpoch, senderId: 'qqbot:u_10001', senderName: '林可'),
    _message(peer: group, text: '好，我盯着 CI，Web 和 APK 一起出', ts: today.add(const Duration(hours: 9, minutes: 15)).millisecondsSinceEpoch, direction: 'out'),
    _message(peer: group, text: '记得把 Flutter 的截图也发群里', ts: now.subtract(const Duration(minutes: 6)).millisecondsSinceEpoch, senderId: 'qqbot:u_10003', senderName: '阿哲'),
  ];
  final zhangList = <Map<String, dynamic>>[
    _message(peer: zhang, text: '新版本什么时候发？', ts: now.subtract(const Duration(hours: 2)).millisecondsSinceEpoch, senderId: 'qqbot:u_10002', senderName: '张三'),
    _message(peer: zhang, text: '今晚打个包，明天上午发', ts: now.subtract(const Duration(hours: 1, minutes: 50)).millisecondsSinceEpoch, direction: 'out'),
    _message(peer: zhang, text: '收到，辛苦', ts: now.subtract(const Duration(hours: 1, minutes: 30)).millisecondsSinceEpoch, senderId: 'qqbot:u_10002', senderName: '张三'),
  ];
  final linList = <Map<String, dynamic>>[
    _message(peer: lin, text: '机器人怎么不回了', ts: DateTime(now.year, now.month, now.day - 1, 22, 10).millisecondsSinceEpoch, senderId: 'qqbot:u_10001', senderName: '林可'),
  ];
  _history[group.id] = groupList;
  _history[zhang.id] = zhangList;
  _history[lin.id] = linList;
}

Map<String, dynamic> _bot() => <String, dynamic>{
      'bot_id': 'qqbot:1905703456',
      'platform': 'qqbot',
      'username': 'liant_demo_bot',
      'display_name': 'LianT 客服号',
      'status': 'running',
      'caps': <String>['text', 'image', 'markdown_template'],
      'limits': <String, dynamic>{'max_text_len': 1000, 'rich_text': true},
      'constraints': <String, dynamic>{
        'can_initiate': true,
        'reply_window_s': 300,
        'max_replies_per_inbound': 5,
        'requires_mention': false,
      },
      'since': DateTime.now().subtract(const Duration(days: 3)).millisecondsSinceEpoch ~/ 1000,
    };

Map<String, dynamic> _deviceSelf() => <String, dynamic>{
      'device_id': 'dev-preview',
      'name': '手机端',
      'created_at': DateTime.now().subtract(const Duration(days: 3)).millisecondsSinceEpoch,
      'last_seen_at': DateTime.now().millisecondsSinceEpoch,
    };

Map<String, dynamic> _deviceOther() => <String, dynamic>{
      'device_id': 'dev-web',
      'name': '网页端',
      'created_at': DateTime.now().subtract(const Duration(days: 7)).millisecondsSinceEpoch,
      'last_seen_at': DateTime.now().subtract(const Duration(hours: 5)).millisecondsSinceEpoch,
    };

List<Map<String, dynamic>> _audit() => <Map<String, dynamic>>[
      <String, dynamic>{
        'ts': DateTime.now().millisecondsSinceEpoch,
        'kind': 'auth.ok',
        'device_id': 'dev-preview',
        'name': '手机端',
        'ip': '100.64.0.12',
      },
      <String, dynamic>{
        'ts': DateTime.now().subtract(const Duration(hours: 5)).millisecondsSinceEpoch,
        'kind': 'auth.ok',
        'device_id': 'dev-web',
        'name': '网页端',
        'ip': '100.64.0.8',
      },
      <String, dynamic>{
        'ts': DateTime.now().subtract(const Duration(days: 1)).millisecondsSinceEpoch,
        'kind': 'auth.fail',
        'reason': '签名校验失败',
        'ip': '198.51.100.7',
      },
      <String, dynamic>{
        'ts': DateTime.now().subtract(const Duration(days: 3)).millisecondsSinceEpoch,
        'kind': 'pair.ok',
        'device_id': 'dev-preview',
        'name': '手机端',
        'ip': '100.64.0.12',
      },
    ];

Map<String, dynamic> _envelope(
  String type, {
  String? replyTo,
  Map<String, dynamic>? payload,
}) {
  return <String, dynamic>{
    'v': 1,
    'id': _nowId(),
    'type': type,
    'ts': DateTime.now().millisecondsSinceEpoch,
    'reply_to': ?replyTo,
    'payload': payload ?? <String, dynamic>{},
  };
}

Future<void> main(List<String> args) async {
  final port = args.isNotEmpty ? int.tryParse(args[0]) ?? 8099 : 8099;
  _seed();

  final serviceKey = p256GenerateKey();
  final servicePub = p256EncodeSpkiBase64(serviceKey.pub);
  final fingerprint = p256Fingerprint(p256EncodeSpki(serviceKey.pub));

  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, port);
  stdout.writeln('mock service: ws://127.0.0.1:$port/v1/ws');
  stdout.writeln('fingerprint: $fingerprint');

  await for (final request in server) {
    if (request.uri.path != '/v1/ws') {
      request.response.statusCode = HttpStatus.notFound;
      await request.response.close();
      continue;
    }
    final socket = await WebSocketTransformer.upgrade(request);
    _serve(socket, serviceKey, servicePub, fingerprint);
  }
}

void _serve(
  WebSocket socket,
  EcKeyPair serviceKey,
  String servicePub,
  String fingerprint,
) {
  final nonce = base64Url.encode(
    List<int>.generate(16, (_) => _random.nextInt(256)),
  ).replaceAll('=', '');
  final ts = DateTime.now().millisecondsSinceEpoch;
  final proofMessage = utf8.encode('$nonce\n$fingerprint\n$ts');
  final proof = p256Sign(serviceKey, proofMessage);

  socket.add(jsonEncode(_envelope('auth.challenge', payload: <String, dynamic>{
    'nonce': nonce,
    'ts': ts,
    'auto_pair': true,
    'service_pub': servicePub,
    'service_sig': base64Encode(proof),
  })));

  socket.listen(
    (raw) {
      final decoded = jsonDecode(raw is String ? raw : utf8.decode(raw as List<int>));
      if (decoded is! Map<String, dynamic>) return;
      final type = decoded['type'] as String? ?? '';
      final id = decoded['id'] as String? ?? '';
      final payload = decoded['payload'] is Map<String, dynamic>
          ? decoded['payload'] as Map<String, dynamic>
          : <String, dynamic>{};
      switch (type) {
        case 'pair.request':
          socket.add(jsonEncode(_envelope('pair.ok', replyTo: id, payload: <String, dynamic>{
            'service_id': fingerprint,
            'service_name': 'LianT Service（本地预览）',
            'devices_count': 2,
            'device_id': 'dev-preview',
          })));
        case 'auth.response':
          socket.add(jsonEncode(_envelope('auth.ok', replyTo: id, payload: <String, dynamic>{
            'version': 'mock-1',
            'service_time': DateTime.now().millisecondsSinceEpoch,
            'caps': <String>['msg.list', 'bot.control', 'media.recv', 'rich_text', 'multi_bot'],
            'retention': <String, dynamic>{
              'days': 7,
              'text_max_bytes': 104857600,
              'media_max_bytes': 3221225472,
              'cleanup_interval_s': 3600,
              'evicted': false,
            },
          })));
          _pushSeeds(socket);
        case 'state.snapshot':
          socket.add(jsonEncode(_envelope('state.snapshot', replyTo: id, payload: <String, dynamic>{
            'caps': <String>['msg.list', 'bot.control', 'media.recv', 'rich_text', 'multi_bot'],
            'bots': <Map<String, dynamic>>[_bot()],
            'devices': <Map<String, dynamic>>[_deviceSelf(), _deviceOther()],
            'audit': _audit(),
          })));
        case 'bot.list':
          socket.add(jsonEncode(_envelope('bot.list', replyTo: id, payload: <String, dynamic>{
            'bots': <Map<String, dynamic>>[_bot()],
          })));
        case 'bot.status':
          socket.add(jsonEncode(_envelope('bot.status', replyTo: id, payload: <String, dynamic>{
            'status': 'running',
            'since': DateTime.now().millisecondsSinceEpoch ~/ 1000,
          })));
        case 'bot.start' || 'bot.stop' || 'bot.add' || 'bot.remove':
          socket.add(jsonEncode(_envelope('bot.list', replyTo: id, payload: <String, dynamic>{
            'bots': <Map<String, dynamic>>[_bot()],
          })));
        case 'device.list':
          socket.add(jsonEncode(_envelope('device.list', replyTo: id, payload: <String, dynamic>{
            'devices': <Map<String, dynamic>>[_deviceSelf(), _deviceOther()],
          })));
        case 'device.revoke':
          socket.add(jsonEncode(_envelope('device.list', replyTo: id, payload: <String, dynamic>{
            'devices': <Map<String, dynamic>>[_deviceSelf()],
          })));
        case 'audit.list':
          socket.add(jsonEncode(_envelope('audit.list', replyTo: id, payload: <String, dynamic>{
            'entries': _audit(),
          })));
        case 'msg.list':
          final peerId = payload['peer_id'] as String? ?? '';
          final list = <Map<String, dynamic>>[...(_history[peerId] ?? <Map<String, dynamic>>[])]
            ..sort((a, b) => (a['ts'] as int).compareTo(b['ts'] as int));
          socket.add(jsonEncode(_envelope('msg.list', replyTo: id, payload: <String, dynamic>{
            'messages': list,
            'has_more': false,
          })));
        case 'msg.send':
          final peerId = payload['peer_id'] as String? ?? '';
          final body = payload['body'] as Map<String, dynamic>?;
          final text = body?['plain'] as String? ?? '';
          final peer = _peers.firstWhere(
            (p) => p.id == peerId,
            orElse: () => _Peer(peerId, 'private', peerId),
          );
          final message = _message(
            peer: peer,
            text: text,
            ts: DateTime.now().millisecondsSinceEpoch,
            direction: 'out',
          );
          _history.putIfAbsent(peerId, () => <Map<String, dynamic>>[]).add(message);
          socket.add(jsonEncode(_envelope('msg.send', replyTo: id, payload: <String, dynamic>{
            'msg_id': message['msg_id'],
            'ts': message['ts'],
          })));
        case 'msg.ack' || 'ping':
          socket.add(jsonEncode(_envelope('pong', replyTo: id)));
        case 'pong':
          break;
        default:
          socket.add(jsonEncode(_envelope('error', replyTo: id, payload: <String, dynamic>{
            'code': 'unknown_type',
            'message': 'mock 不认识 $type',
            'retryable': false,
          })));
      }
    },
    onError: (Object _) => socket.close(),
    onDone: () => socket.close(),
    cancelOnError: true,
  );
}

/// 认证后推三条入站消息，模拟「刚打开客户端时收到的推送」。
void _pushSeeds(WebSocket socket) {
  for (final peer in _peers) {
    final list = _history[peer.id] ?? <Map<String, dynamic>>[];
    if (list.isEmpty) continue;
    final latest = list.last;
    socket.add(jsonEncode(_envelope('event.peer', payload: <String, dynamic>{
      'bot_id': 'qqbot:1905703456',
      'peer_id': peer.id,
      'title': peer.title,
    })));
    socket.add(jsonEncode(_envelope('event.msg', payload: latest)));
  }
}

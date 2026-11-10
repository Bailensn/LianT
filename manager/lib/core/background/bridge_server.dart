// 后台进程里的本地桥接服务。
//
// 真正的 WebSocket 在 `:background` 进程里，这里把它收到的状态与事件
// 实时转给 UI 进程，并把 UI 的请求/发送转发给服务端。
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'bridge_protocol.dart';
import '../net/client_api.dart';
import '../net/protocol.dart';

/// 一条本地 TCP 桥接，JSON 行协议。
class BridgeServer {
  BridgeServer({required this.clientOf, required this.connectionOf});

  /// 当前在用的客户端；身份变化时会换实例，所以传 getter 而不是实例。
  final MessageClient? Function() clientOf;

  /// 当前连接状态；UI 刚接上时先补一条，界面才不会停在「连接中」。
  final (ConnState, String) Function() connectionOf;

  ServerSocket? _server;
  String? _token;
  Socket? _peer;
  StreamSubscription<String>? _peerLines;
  bool _peerReady = false;

  /// UI 进程是否接着桥接。断开说明 UI 进程已经没了。
  bool get hasPeer => _peer != null && _peerReady;

  int? get port => _server?.port;

  String? get token => _token;

  Future<void> start() async {
    _token = _newToken();
    // 只绑回环地址：外网/局域网连不进来。
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    _server = server;
    server.listen(_accept, onError: (Object _) {});
  }

  Future<void> stop() async {
    await _peerLines?.cancel();
    _peerLines = null;
    _peer?.destroy();
    _peer = null;
    _peerReady = false;
    await _server?.close();
    _server = null;
  }

  void broadcastState(ConnState state, String detail) {
    _write(<String, dynamic>{
      't': kFrameState,
      'state': state.name,
      'detail': detail,
    });
  }

  void broadcastEvent(Envelope env) {
    _write(<String, dynamic>{'t': kFrameEvent, 'env': env.toJson()});
  }

  // -------------------------------------------------------------------------

  void _accept(Socket socket) {
    // 只保留最新一条：UI 进程崩溃后旧连接可能还挂着。
    _peerLines?.cancel();
    _peer?.destroy();
    _peer = socket;
    _peerReady = false;

    final handshake = Timer(const Duration(seconds: 3), () {
      if (!_peerReady) _dropPeer(socket);
    });
    final lines = socket
        .cast<List<int>>()
        .transform(utf8.decoder)
        .transform(const LineSplitter());
    _peerLines = lines.listen(
      (line) {
        final frame = decodeFrame(line);
        if (frame == null) {
          _dropPeer(socket);
          return;
        }
        if (!_peerReady) {
          final hello = frame[kFrameHello];
          if (hello is! String || hello != _token) {
            _dropPeer(socket);
            return;
          }
          handshake.cancel();
          _peerReady = true;
          _write(<String, dynamic>{
            't': kFrameReady,
            'v': kBridgeProtocolVersion,
          });
          final (state, detail) = connectionOf();
          broadcastState(state, detail);
          return;
        }
        _handleFrame(frame);
      },
      onError: (Object _) => _dropPeer(socket),
      onDone: () => _dropPeer(socket),
      cancelOnError: false,
    );
  }

  void _dropPeer(Socket socket) {
    if (!identical(_peer, socket)) return;
    _peerLines?.cancel();
    _peerLines = null;
    _peer?.destroy();
    _peer = null;
    _peerReady = false;
  }

  void _handleFrame(Map<String, dynamic> frame) {
    final type = frame['t'];
    if (type == kFrameBye) {
      final peer = _peer;
      if (peer != null) _dropPeer(peer);
      return;
    }
    if (type == kFrameRequest || type == kFrameSend) {
      final id = frame['id'];
      final method = frame['method'];
      final rawPayload = frame['payload'];
      final payload = rawPayload is Map<String, dynamic>
          ? rawPayload
          : const <String, dynamic>{};
      if (id is! String || method is! String) return;
      final client = clientOf();
      if (client == null) {
        _writeError(id, 'offline', '后台连接未就绪');
        return;
      }
      if (type == kFrameSend) {
        try {
          final envelopeId = client.send(method, payload);
          _write(<String, dynamic>{
            't': kFrameResponse,
            'id': id,
            'ok': true,
            'payload': <String, dynamic>{'id': envelopeId},
          });
        } on ProtocolException catch (err) {
          _writeError(id, err.code, err.message);
        } catch (err) {
          _writeError(id, 'internal', '$err');
        }
        return;
      }
      unawaited(client.request(method, payload).then(
        (res) => _write(<String, dynamic>{
          't': kFrameResponse,
          'id': id,
          'ok': true,
          'payload': res,
        }),
        onError: (Object err) {
          if (err is ProtocolException) {
            _write(<String, dynamic>{
              't': kFrameResponse,
              'id': id,
              'ok': false,
              'code': err.code,
              'message': err.message,
              'retryable': err.retryable,
            });
          } else {
            _writeError(id, 'internal', '$err');
          }
        },
      ));
    }
  }

  void _writeError(String id, String code, String message) {
    _write(<String, dynamic>{
      't': kFrameResponse,
      'id': id,
      'ok': false,
      'code': code,
      'message': message,
    });
  }

  void _write(Map<String, dynamic> frame) {
    final peer = _peer;
    if (peer == null || !_peerReady) return;
    try {
      peer.write('${encodeFrame(frame)}\n');
    } catch (_) {
      // 对端已经断了，交给 onDone 清理。
    }
  }

  static String _newToken() {
    final random = Random.secure();
    final bytes = List<int>.generate(24, (_) => random.nextInt(256));
    return base64UrlEncode(bytes).replaceAll('=', '');
  }
}

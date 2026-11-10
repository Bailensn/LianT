// UI 进程侧的桥接客户端。
//
// 真正的 WebSocket 在 `:background` 进程（BridgeServer + LiantClient），
// 这里只是把它转发出来的状态/事件喂给界面，并把请求转回去。
import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'bridge_info.dart';
import 'bridge_protocol.dart';
import '../net/client_api.dart';
import '../identity/identity.dart';
import '../net/protocol.dart';

class _PendingBridge {
  _PendingBridge(this.completer, this.timer);

  final Completer<Map<String, dynamic>> completer;
  final Timer timer;
}

class BridgeMessageClient implements MessageClient {
  BridgeMessageClient._(
    this.identity,
    this._socket,
    this._onState,
    this._onEvent,
  ) {
    _lines = _socket
        .cast<List<int>>()
        .transform(utf8.decoder)
        .transform(const LineSplitter())
        .listen(
          _onLine,
          onError: (Object _) => _onClosed(),
          onDone: _onClosed,
          cancelOnError: false,
        );
  }

  @override
  final Identity identity;

  final Socket _socket;
  final void Function(ConnState, String) _onState;
  final void Function(Envelope) _onEvent;

  final Map<String, _PendingBridge> _pending = <String, _PendingBridge>{};
  final Completer<bool> _authed = Completer<bool>();
  final List<Map<String, dynamic>> _buffered = <Map<String, dynamic>>[];

  late final StreamSubscription<String> _lines;
  bool _ready = false;
  bool _started = false;
  bool _closed = false;
  bool _online = false;
  int _seq = 0;

  @override
  bool get online => _online;

  /// 连上后台进程的桥接（握手通过才算成功）。
  ///
  /// 返回 null 表示后台服务还没起来/端口已变，调用方应该降级直连。
  static Future<BridgeMessageClient?> connect({
    required Identity identity,
    required BridgeInfo info,
    required void Function(ConnState, String) onState,
    required void Function(Envelope) onEvent,
    Duration timeout = const Duration(seconds: 3),
  }) async {
    Socket socket;
    try {
      socket = await Socket.connect(
        InternetAddress.loopbackIPv4,
        info.port,
        timeout: timeout,
      );
    } catch (_) {
      return null;
    }
    final client = BridgeMessageClient._(identity, socket, onState, onEvent);
    try {
      socket.write('${encodeFrame(<String, dynamic>{
        kFrameHello: info.token,
        'v': kBridgeProtocolVersion,
      })}\n');
      final ok = await client._authed.future.timeout(
        timeout,
        onTimeout: () => false,
      );
      if (!ok) {
        client._hardClose();
        return null;
      }
    } catch (_) {
      client._hardClose();
      return null;
    }
    return client;
  }

  @override
  void start() {
    if (_started || _closed) return;
    _started = true;
    final buffered = List<Map<String, dynamic>>.of(_buffered);
    _buffered.clear();
    for (final frame in buffered) {
      _handle(frame);
    }
  }

  @override
  void stop() {
    if (_closed) return;
    try {
      _socket.write('${encodeFrame(<String, dynamic>{'t': kFrameBye})}\n');
    } catch (_) {}
    _hardClose();
    _onState(ConnState.idle, '');
  }

  @override
  Future<Map<String, dynamic>> request(
    String type,
    Map<String, dynamic> payload, {
    Duration timeout = const Duration(seconds: 20),
  }) {
    if (_closed || !_online) {
      return Future<Map<String, dynamic>>.error(
        const ProtocolException('offline', '连接未就绪'),
      );
    }
    final id = '${_seq++}';
    final completer = Completer<Map<String, dynamic>>();
    final timer = Timer(timeout, () {
      final entry = _pending.remove(id);
      if (entry != null) {
        entry.completer.completeError(
          const ProtocolException('timeout', '请求超时', retryable: true),
        );
      }
    });
    _pending[id] = _PendingBridge(completer, timer);
    _write(<String, dynamic>{
      't': kFrameRequest,
      'id': id,
      'method': type,
      'payload': payload,
    });
    return completer.future;
  }

  @override
  String send(String type, Map<String, dynamic> payload) {
    if (_closed || !_online) {
      throw const ProtocolException('offline', '连接未就绪');
    }
    final id = 'send-${_seq++}';
    _write(<String, dynamic>{
      't': kFrameSend,
      'id': id,
      'method': type,
      'payload': payload,
    });
    return id;
  }

  @override
  String resolveMediaUrl(String url) {
    if (url.startsWith('http://') || url.startsWith('https://')) return url;
    final base = identity.address.httpBase;
    return url.startsWith('/') ? '$base$url' : '$base/$url';
  }

  // -------------------------------------------------------------------------

  void _onLine(String line) {
    final frame = decodeFrame(line);
    if (frame == null) return;
    if (!_ready) {
      if (frame['t'] == kFrameReady) {
        _ready = true;
        if (!_authed.isCompleted) _authed.complete(true);
      }
      return;
    }
    if (!_started) {
      _buffered.add(frame);
      return;
    }
    _handle(frame);
  }

  void _handle(Map<String, dynamic> frame) {
    switch (frame['t']) {
      case kFrameState:
        final state = _stateFromName(frame['state']);
        final detail = frame['detail'] is String ? frame['detail'] as String : '';
        _online = state == ConnState.online;
        _onState(state, detail);
      case kFrameEvent:
        final raw = frame['env'];
        if (raw is Map<String, dynamic>) {
          final env = Envelope.tryParse(jsonEncode(raw));
          if (env != null) _onEvent(env);
        }
      case kFrameResponse:
        final id = frame['id'];
        if (id is! String) return;
        final entry = _pending.remove(id);
        if (entry == null) return;
        entry.timer.cancel();
        if (frame['ok'] == true) {
          final payload = frame['payload'];
          entry.completer.complete(
            payload is Map<String, dynamic> ? payload : const <String, dynamic>{},
          );
        } else {
          entry.completer.completeError(
            ProtocolException(
              frame['code'] is String ? frame['code'] as String : 'internal',
              frame['message'] is String ? frame['message'] as String : '',
              retryable: frame['retryable'] == true,
            ),
          );
        }
    }
  }

  void _write(Map<String, dynamic> frame) {
    if (_closed) return;
    try {
      _socket.write('${encodeFrame(frame)}\n');
    } catch (_) {
      _onClosed();
    }
  }

  void _onClosed() {
    if (_closed) return;
    _hardClose();
    _online = false;
    if (!_authed.isCompleted) _authed.complete(false);
    for (final entry in _pending.values) {
      entry.timer.cancel();
      if (!entry.completer.isCompleted) {
        entry.completer.completeError(
          const ProtocolException('offline', '后台服务连接已断开', retryable: true),
        );
      }
    }
    _pending.clear();
    _onState(ConnState.idle, '后台服务连接已断开');
  }

  void _hardClose() {
    _closed = true;
    _lines.cancel();
    _socket.destroy();
  }

  static ConnState _stateFromName(Object? name) {
    for (final state in ConnState.values) {
      if (state.name == name) return state;
    }
    return ConnState.connecting;
  }
}

// 协议 v1 的 WebSocket 客户端。
//
// 负责：签名握手、请求-响应关联、30 秒心跳、指数退避重连。
// 不负责业务状态（见 app_state.dart）。
import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:web_socket_channel/web_socket_channel.dart';

import 'identity.dart';
import 'p256.dart';
import 'protocol.dart';

/// 连接状态，界面按它渲染顶部说明条。
enum ConnState { idle, connecting, online, reconnecting, error }

class _Pending {
  _Pending(this.completer, this.timer);

  final Completer<Map<String, dynamic>> completer;
  final Timer timer;
}

/// 一台已配对设备的连接。
class LiantClient {
  LiantClient({
    required this.identity,
    required this.onState,
    required this.onEvent,
  });

  final Identity identity;
  final void Function(ConnState state, String detail) onState;
  final void Function(Envelope event) onEvent;

  static const Duration _pingInterval = Duration(seconds: 30);
  static const Duration _requestTimeout = Duration(seconds: 15);
  static const Duration _connectTimeout = Duration(seconds: 15);
  static const Duration _authTimeout = Duration(seconds: 20);
  static const Duration _maxRetryDelay = Duration(seconds: 30);

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;
  final Map<String, _Pending> _pending = <String, _Pending>{};
  final Random _random = Random();

  Completer<void>? _auth;
  Timer? _pingTimer;
  Timer? _retryTimer;
  int _generation = 0;
  int _closedGeneration = -1;
  int _retryCount = 0;
  bool _closed = true;
  bool _online = false;
  bool _fatal = false;

  bool get online => _online;

  void start() {
    if (!_closed) return;
    _closed = false;
    _fatal = false;
    _retryCount = 0;
    unawaited(_attempt());
  }

  void stop() {
    _closed = true;
    _online = false;
    _generation++;
    _clearTimers();
    _failAll(const ProtocolException('offline', '连接已关闭'));
    final auth = _auth;
    if (auth != null && !auth.isCompleted) {
      auth.completeError(const ProtocolException('offline', '连接已关闭'));
    }
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close();
    _channel = null;
    onState(ConnState.idle, '');
  }

  /// 发一条不需要响应的消息，返回信封 id。
  String send(String type, Map<String, dynamic> payload) {
    if (!_online || _channel == null) {
      throw const ProtocolException('offline', '连接未就绪');
    }
    return _write(type, payload);
  }

  /// 发一条请求并等待同 id 的响应；超时与 error 都会抛 [ProtocolException]。
  Future<Map<String, dynamic>> request(
    String type,
    Map<String, dynamic> payload, {
    Duration timeout = _requestTimeout,
  }) {
    if (!_online || _channel == null) {
      return Future<Map<String, dynamic>>.error(
        const ProtocolException('offline', '连接未就绪'),
      );
    }
    final id = newId();
    final completer = Completer<Map<String, dynamic>>();
    final timer = Timer(timeout, () {
      final entry = _pending.remove(id);
      if (entry != null) {
        entry.completer.completeError(
          const ProtocolException('timeout', '请求超时', retryable: true),
        );
      }
    });
    _pending[id] = _Pending(completer, timer);
    try {
      _write(type, payload, id: id);
    } catch (err) {
      _pending.remove(id);
      timer.cancel();
      rethrow;
    }
    return completer.future;
  }

  /// 把服务端给的媒体相对路径拼成可加载的绝对地址。
  String resolveMediaUrl(String url) {
    if (url.startsWith('http://') || url.startsWith('https://')) return url;
    final base = identity.address.httpBase;
    return url.startsWith('/') ? '$base$url' : '$base/$url';
  }

  // -------------------------------------------------------------------------
  // 连接与握手
  // -------------------------------------------------------------------------

  Future<void> _attempt() async {
    if (_closed) return;
    final generation = ++_generation;
    _online = false;
    _auth = Completer<void>();
    onState(
      _retryCount == 0 ? ConnState.connecting : ConnState.reconnecting,
      _retryCount == 0 ? '' : '第 $_retryCount 次重连',
    );

    final WebSocketChannel channel;
    try {
      channel = WebSocketChannel.connect(Uri.parse(identity.address.wsUrl));
    } catch (err) {
      _scheduleRetry('无法连接：$err');
      return;
    }
    _channel = channel;
    _subscription = channel.stream.listen(
      (raw) => _handleRaw(generation, raw),
      onError: (Object err) => _handleClosed(generation, _describe(err)),
      onDone: () => _handleClosed(generation),
      cancelOnError: false,
    );

    try {
      await channel.ready.timeout(_connectTimeout);
      await _auth!.future.timeout(_authTimeout);
    } on TimeoutException {
      _handleClosed(generation, '连接超时');
    } on ProtocolException catch (err) {
      _handleClosed(generation, err.friendly);
    } on StateError catch (err) {
      _handleClosed(generation, err.message);
    } catch (err) {
      _handleClosed(generation, _describe(err));
    }
  }

  void _handleRaw(int generation, Object? raw) {
    if (generation != _generation) return;
    final env = Envelope.tryParse(raw);
    if (env == null) return;

    if (env.type == 'auth.challenge') {
      unawaited(_answerChallenge(env));
      return;
    }
    if (env.type == 'auth.ok') {
      _online = true;
      _retryCount = 0;
      _startPing();
      final auth = _auth;
      if (auth != null && !auth.isCompleted) auth.complete();
      onState(ConnState.online, '');
    }
    _dispatch(env);
  }

  Future<void> _answerChallenge(Envelope env) async {
    final payload = env.payload;
    final nonce = payload['nonce'] as String? ?? '';
    final ts = payload['ts'] is num ? (payload['ts'] as num).toInt() : 0;
    if (nonce.isEmpty || ts == 0) {
      _failFatally('服务端挑战格式不正确');
      return;
    }
    // 待签字节固定为 nonce \n 指纹 \n 十进制 ts（协议 §4）。
    final message = utf8.encode('$nonce\n${identity.fingerprint}\n$ts');
    final servicePub = payload['service_pub'];
    final serviceSig = payload['service_sig'];
    try {
      if (servicePub is String && serviceSig is String) {
        p256VerifyServiceProof(
          servicePubBase64: servicePub,
          expectedFingerprint: identity.fingerprint,
          message: message,
          signatureBase64: serviceSig,
        );
      } else if (identity.fingerprint.isNotEmpty) {
        // 配对时钉过指纹，服务端却说不出自己是谁：拒绝连接，不给「忽略」开关。
        throw StateError('服务端没有提供身份证明，拒绝连接');
      }
      final signature = p256Sign(identity.keyPair, message);
      final channel = _channel;
      if (channel == null) return;
      _sendRaw(
        channel,
        Envelope(
          id: newId(),
          type: 'auth.response',
          ts: DateTime.now().millisecondsSinceEpoch,
          replyTo: env.id,
          payload: <String, dynamic>{
            'manager_id': identity.deviceId,
            'sig': base64Encode(signature),
          },
        ),
      );
    } on StateError catch (err) {
      _failFatally(err.message);
    }
  }

  void _failFatally(String reason) {
    _fatal = true;
    _online = false;
    onState(ConnState.error, reason);
    final auth = _auth;
    if (auth != null && !auth.isCompleted) {
      auth.completeError(ProtocolException('unauthorized', reason));
    }
    _closeSocket();
  }

  void _handleClosed(int generation, [String? reason]) {
    // 同一次连接只处理一次：onDone 与 _attempt 的 catch 可能同时到达。
    if (generation != _generation || _closedGeneration == generation) return;
    _closedGeneration = generation;
    final closeCode = _channel?.closeCode;
    _online = false;
    _clearTimers();
    _failAll(const ProtocolException('offline', '连接已断开'));
    final auth = _auth;
    if (auth != null && !auth.isCompleted) {
      auth.completeError(ProtocolException('offline', reason ?? '连接已断开'));
    }
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close();
    _channel = null;
    if (_closed || _fatal) return;
    // 协议 §9：4001 认证失败、4002 版本不兼容不重连。
    if (closeCode == 4001 || closeCode == 4002) {
      _fatal = true;
      onState(
        ConnState.error,
        closeCode == 4001 ? '认证失败：设备可能已被服务端吊销' : '客户端与服务端版本不兼容',
      );
      return;
    }
    _scheduleRetry(reason ?? '连接已断开');
  }

  void _scheduleRetry(String reason) {
    if (_closed || _fatal) return;
    _retryCount++;
    final base = min(1 << (_retryCount - 1), _maxRetryDelay.inSeconds);
    // ±20% 抖动，避免多台设备在同一秒重连。
    final jitter = 0.8 + _random.nextDouble() * 0.4;
    final delay = Duration(milliseconds: (base * 1000 * jitter).round());
    onState(
      ConnState.reconnecting,
      '${reason.isEmpty ? '连接中断' : reason} · ${(delay.inMilliseconds / 1000).toStringAsFixed(0)} 秒后重试',
    );
    _retryTimer?.cancel();
    _retryTimer = Timer(delay, () => unawaited(_attempt()));
  }

  void _closeSocket() {
    _generation++;
    _subscription?.cancel();
    _subscription = null;
    _channel?.sink.close();
    _channel = null;
    _clearTimers();
    _failAll(const ProtocolException('offline', '连接已关闭'));
  }

  void _clearTimers() {
    _pingTimer?.cancel();
    _pingTimer = null;
    _retryTimer?.cancel();
    _retryTimer = null;
  }

  void _startPing() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(_pingInterval, (_) {
      if (!_online || _channel == null) return;
      try {
        send('ping', const <String, dynamic>{});
      } catch (_) {
        // 下一轮 onDone/onError 会触发重连。
      }
    });
  }

  // -------------------------------------------------------------------------
  // 收发
  // -------------------------------------------------------------------------

  String _write(String type, Map<String, dynamic> payload, {String? id}) {
    final channel = _channel;
    if (channel == null) {
      throw const ProtocolException('offline', '连接未就绪');
    }
    final envelope = Envelope(
      id: id ?? newId(),
      type: type,
      ts: DateTime.now().millisecondsSinceEpoch,
      payload: payload,
    );
    _sendRaw(channel, envelope);
    return envelope.id;
  }

  void _sendRaw(WebSocketChannel channel, Envelope envelope) {
    try {
      channel.sink.add(envelope.encode());
    } on StateError {
      // 通道已关闭；由 onDone 负责重连。
    }
  }

  void _dispatch(Envelope env) {
    final replyTo = env.replyTo;
    if (replyTo != null) {
      final pending = _pending.remove(replyTo);
      if (pending != null) {
        pending.timer.cancel();
        if (env.type == 'error') {
          pending.completer.completeError(_errorFrom(env));
        } else {
          pending.completer.complete(env.payload);
        }
        return;
      }
    }
    if (env.type == 'ping') {
      try {
        _write('pong', const <String, dynamic>{}, id: newId());
      } catch (_) {
        // 忽略：连接已断。
      }
      return;
    }
    if (env.type == 'pong') return;
    onEvent(env);
  }

  ProtocolException _errorFrom(Envelope env) {
    final payload = env.payload;
    return ProtocolException(
      payload['code'] as String? ?? 'internal',
      payload['message'] as String? ?? '',
      retryable: payload['retryable'] == true,
    );
  }

  void _failAll(ProtocolException error) {
    for (final pending in _pending.values) {
      pending.timer.cancel();
      if (!pending.completer.isCompleted) {
        pending.completer.completeError(error);
      }
    }
    _pending.clear();
  }

  String _describe(Object error) {
    if (error is ProtocolException) return error.friendly;
    if (error is StateError) return error.message;
    final text = error.toString();
    if (text.contains('WebSocketChannelException')) return '无法连接到服务';
    if (text.length > 120) return text.substring(0, 120);
    return text;
  }
}

/// 生成一个 UUID v4（不引第三方包；协议只要求全局唯一）。
String newId() {
  final random = Random.secure();
  final bytes = List<int>.generate(16, (_) => random.nextInt(256));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  final hex = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
  return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-'
      '${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
}

/// 配对失败原因的文案（与 web/src/lib/ws.ts 保持一致）。
String pairDeniedText(String reason) => switch (reason) {
      'no_ticket' => '服务端没有待处理的配对码',
      'expired' => '配对码已过期，请在服务端重新执行 pair',
      'used' => '配对码已经被使用',
      'mismatch' => '配对码或 nonce 不匹配',
      'devices_full' => '设备数量已达上限',
      'denied' => '服务端拒绝了这次配对',
      'invalid_pub' => '公钥格式不被接受',
      'invalid_sig' => '签名校验失败',
      'name_not_allowed' => '这台设备的名称不在服务端白名单里',
      'internal' => '服务端内部错误',
      _ => '配对失败：$reason',
    };

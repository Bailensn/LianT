// 独立后台进程里的消息循环。
//
// 由 MessageService 在自己的 FlutterEngine 里执行 `backgroundMain` 进入；
// 这里不含任何 UI，只做四件事：
//   1. 连服务端（全 App 唯一一条 WebSocket）；
//   2. 把状态与事件实时转给 UI 进程（本地桥接）；
//   3. 转发 UI 发来的请求；
//   4. UI 不在前台时弹系统通知。
import 'dart:async';

import 'package:flutter/services.dart';

import 'background_state.dart';
import 'bridge_info_io.dart';
import 'bridge_server_io.dart';
import 'client.dart';
import 'identity.dart';
import 'protocol.dart';

/// 后台进程自己的原生通道（MessageService 注册）。
const MethodChannel _notifyChannel = MethodChannel('liant/background_notify');

BackgroundState? _state;
LiantClient? _client;
BridgeServer? _bridge;
Timer? _retryTimer;
ConnState _lastState = ConnState.idle;
String _lastDetail = '';
bool _running = false;

/// 后台进程入口主体：起桥接、读共享状态、连接服务端，然后常驻。
Future<void> startBackgroundRuntime() async {
  if (_running) return;
  _running = true;
  await _startBridge();
  // 每分钟对一次共享状态：识别解除配对、重新配对这类运行期变化。
  Timer.periodic(const Duration(seconds: 60), (_) {
    unawaited(_refreshState());
  });
  final state = await readBackgroundState();
  if (state == null) {
    // 没有配对信息（例如用户刚解除配对），先不连接，等状态文件出现。
    await _setStatus('未配对');
    return;
  }
  _state = state;
  _connect();
}

// ---------------------------------------------------------------------------
// 本地桥接
// ---------------------------------------------------------------------------

Future<void> _startBridge() async {
  try {
    final bridge = BridgeServer(
      clientOf: () => _client,
      connectionOf: () => (_lastState, _lastDetail),
    );
    await bridge.start();
    _bridge = bridge;
    await _touchBridge();
    // 心跳：UI 靠这个时间戳判断后台进程是否还活着。
    Timer.periodic(const Duration(seconds: 30), (_) {
      unawaited(_touchBridge());
    });
  } catch (_) {
    // 桥接起不来就只做通知；UI 会自动降级直连。
    _bridge = null;
  }
}

Future<void> _touchBridge() async {
  final bridge = _bridge;
  final port = bridge?.port;
  final token = bridge?.token;
  if (port == null || token == null) return;
  await writeBridgeInfo(
    BridgeInfo(
      port: port,
      token: token,
      at: DateTime.now().millisecondsSinceEpoch,
    ),
  );
}

// ---------------------------------------------------------------------------
// 连接
// ---------------------------------------------------------------------------

void _connect() {
  final state = _state;
  if (state == null) return;
  _client?.stop();
  final client = LiantClient(
    identity: state.identity,
    onState: _onConnState,
    onEvent: (env) {
      // 先转给 UI 进程：界面上的会话、Bot 状态、审计都靠这些事件。
      _bridge?.broadcastEvent(env);
      if (env.type != 'msg.inbound' && env.type != 'event.msg') return;
      unawaited(_onInbound(env));
    },
  );
  _client = client;
  client.start();
}

void _onConnState(ConnState state, String detail) {
  _lastState = state;
  _lastDetail = detail;
  _bridge?.broadcastState(state, detail);
  final text = switch (state) {
    ConnState.online => '已连接 · 正在后台接收消息',
    ConnState.connecting => '正在连接…',
    ConnState.reconnecting => '连接中断，正在重连…',
    ConnState.error => detail.isEmpty ? '连接失败' : detail,
    ConnState.idle => '未连接',
  };
  unawaited(_setStatus(text));
  if (state == ConnState.online) {
    _retryTimer?.cancel();
    _retryTimer = null;
    return;
  }
  // 认证失败之类是「不重连」的终态；隔几分钟再试着连一次，
  // 这样设备被重新授权后不用等用户手动打开 App。
  if (state == ConnState.error) {
    _retryTimer?.cancel();
    _retryTimer = Timer(const Duration(minutes: 5), () {
      _retryTimer = null;
      if (_state != null) {
        _connect();
      } else {
        unawaited(_refreshState());
      }
    });
  }
}

Future<void> _refreshState() async {
  final next = await readBackgroundState();
  if (next == null) {
    _client?.stop();
    _client = null;
    _state = null;
    _lastState = ConnState.idle;
    _lastDetail = '';
    _bridge?.broadcastState(_lastState, _lastDetail);
    await _setStatus('未配对');
    return;
  }
  final current = _client?.identity;
  _state = next;
  if (current == null || !_sameTarget(current, next.identity)) {
    _connect();
  }
}

bool _sameTarget(Identity a, Identity b) =>
    a.deviceId == b.deviceId &&
    a.host == b.host &&
    a.port == b.port &&
    a.secure == b.secure;

// ---------------------------------------------------------------------------
// 通知
// ---------------------------------------------------------------------------

Future<void> _onInbound(Envelope env) async {
  final msg = ChatMessage.fromJson(env.payload, direction: 'in');
  if (msg.msgId.isEmpty || msg.botId.isEmpty) return;
  // 每次通知都重新读一遍共享状态，免打扰/前台开关立刻生效。
  final state = await readBackgroundState() ?? _state;
  if (state == null) return;
  _state = state;
  if (state.foreground) return;
  final key = convKey(msg.botId, msg.peerId);
  if (state.muted[key] == true) return;

  final isGroup = msg.peerKind == 'group';
  final fallback =
      state.remarks[key] ?? state.titles[key] ?? shortPeer(msg.peerId);
  final sender =
      msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName;
  final title = _clip(
    isGroup ? fallback : (msg.senderName.isEmpty ? fallback : msg.senderName),
    36,
  );
  final summary = _summary(msg);
  final body = _clip(isGroup ? '$sender: $summary' : summary, 90);
  await _notify(msg.msgId, title, body);
}

Future<void> _notify(String msgId, String title, String body) async {
  try {
    await _notifyChannel.invokeMethod<void>('notify', <String, dynamic>{
      'id': notificationIdFor(msgId),
      'title': title,
      'body': body,
    });
  } on MissingPluginException {
    // 原生侧没起来；下一条消息再试。
  } catch (_) {}
}

Future<void> _setStatus(String text) async {
  try {
    await _notifyChannel.invokeMethod<void>('status', <String, dynamic>{
      'text': text,
    });
  } on MissingPluginException {
    // 原生侧没起来。
  } catch (_) {}
}

String _summary(ChatMessage msg) {
  switch (msg.msgType) {
    case 'image':
      return '[图片]';
    case 'video':
      return '[视频]';
    case 'audio':
      return '[语音]';
    case 'file':
      final name = msg.body.plain.trim();
      return name.isEmpty ? '[文件]' : '[文件]$name';
    default:
      final text = msg.body.plain.trim();
      return text.isEmpty ? '[消息]' : text;
  }
}

String _clip(String text, int max) {
  final flat = text.replaceAll(RegExp(r'\s+'), ' ').trim();
  if (flat.length <= max) return flat;
  return '${flat.substring(0, max)}...';
}

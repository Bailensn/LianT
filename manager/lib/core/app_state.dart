// 客户端状态：连接、Bot、消息、设备、审计，以及本地备注。
//
// 只在这里改状态并 notifyListeners；页面通过 AppScope 订阅。
import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../l10n/l10n.dart';
import 'background_service.dart';
import 'background_state.dart';
import 'client.dart';
import 'identity.dart';
import 'p256.dart';
import 'protocol.dart';
import 'socket.dart';

/// 媒体加载状态。
class MediaState {
  const MediaState._({this.url, this.mime = '', this.error, this.loading = false});

  const MediaState.loading() : this._(loading: true);

  const MediaState.ready(String url, {String mime = ''}) : this._(url: url, mime: mime);

  const MediaState.failed(String error) : this._(error: error);

  final String? url;
  final String mime;
  final String? error;
  final bool loading;
}

/// 会话摘要（列表页用）。
class Conversation {
  Conversation({
    required this.key,
    required this.botId,
    required this.peerId,
    required this.peerKind,
    required this.title,
    this.last,
    this.unread = 0,
  });

  final String key;
  final String botId;
  final String peerId;
  String peerKind;
  String title;
  ChatMessage? last;
  int unread;
}

/// 应用状态。
class AppState extends ChangeNotifier {
  AppState(this._prefs, {Box<String>? messageBox}) : _messageBox = messageBox;

  final SharedPreferences _prefs;
  final Box<String>? _messageBox;

  static const String _kIdentity = 'liant.identity';
  static const String _kRemarks = 'liant.remarks';
  static const String _kTitles = 'liant.titles';
  static const String _kPeerHints = 'liant.peerHints';
  static const String _kRead = 'liant.read';
  static const String _kMuted = 'liant.muted';
  static const String _kAcceptBeta = 'liant.acceptBeta';
  static const String _kLanguage = 'liant.language';
  static const String _kMessages = 'liant.messages';

  LiantClient? _client;
  bool _booted = false;
  bool _syncing = false;
  Timer? _saveTimer;
  Timer? _bgStateTimer;
  AppLifecycleListener? _lifecycle;

  Identity? identity;
  ConnState conn = ConnState.idle;
  String detail = '';
  String error = '';
  List<Bot> bots = <Bot>[];
  List<DeviceInfo> devices = <DeviceInfo>[];
  List<AuditEntry> audit = <AuditEntry>[];
  List<ChatMessage> messages = <ChatMessage>[];
  final Map<String, String> remarks = <String, String>{};
  final Map<String, String> titles = <String, String>{};
  final Map<String, String> peerHints = <String, String>{};
  final Map<String, bool> muted = <String, bool>{};
  final Map<String, int> readAt = <String, int>{};
  final Map<String, MediaState> media = <String, MediaState>{};
  final Map<String, Uint8List> localMedia = <String, Uint8List>{};
  Set<String> caps = <String>{};
  Retention retention = const Retention();
  String? openConversation;
  String serviceVersion = '';
  String languageSetting = 'system';
  bool appInForeground = true;
  bool acceptBeta = false;

  bool get booted => _booted;
  bool get online => conn == ConnState.online;

  Locale? get locale {
    switch (languageSetting) {
      case 'zh_Hant':
        return const Locale.fromSubtags(languageCode: 'zh', scriptCode: 'Hant');
      case 'zh':
      case 'en':
      case 'ja':
      case 'ko':
        return Locale(languageSetting);
      default:
        return null;
    }
  }

  void setLanguage(String setting) {
    languageSetting = setting;
    L10n.setSetting(setting);
    unawaited(_prefs.setString(_kLanguage, setting));
    notifyListeners();
  }

  // -------------------------------------------------------------------------
  // 启动与配对
  // -------------------------------------------------------------------------

  /// 从本地恢复身份与偏好；有身份就直接连。
  void boot() {
    if (_booted) return;
    final rawIdentity = _prefs.getString(_kIdentity);
    if (rawIdentity != null) {
      try {
        final decoded = jsonDecode(rawIdentity);
        if (decoded is Map<String, dynamic>) {
          identity = Identity.fromJson(decoded);
        }
      } on FormatException {
        identity = null;
      }
    }
    _loadMap(_kRemarks, remarks);
    _loadMap(_kTitles, titles);
    _loadMap(_kPeerHints, peerHints);
    final read = <String, dynamic>{};
    _loadMap(_kRead, read);
    for (final entry in read.entries) {
      if (entry.value is num) readAt[entry.key] = (entry.value as num).toInt();
    }
    _loadMessages();
    _lifecycle = AppLifecycleListener(
      onStateChange: (state) {
        if (state == AppLifecycleState.resumed) {
          _setForeground(true);
          return;
        }
        _setForeground(false);
        // 注意：detached（Activity 销毁、划掉任务）时**不能**停服务，
        // 后台进程存在的意义就是 UI 进程没了它还继续收消息。
        if (state != AppLifecycleState.detached && identity != null) {
          unawaited(startMessageService());
        }
      },
    );
    _booted = true;
    _syncBackgroundState(immediate: true);
    notifyListeners();
    if (identity != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        unawaited(startMessageService());
        unawaited(requestNotificationPermission());
      });
      _connect();
    }
  }

  /// 扫码/粘贴配对码配对。
  Future<void> pairWithCode(String input, String deviceName) async {
    final payload = parsePairInput(input);
    if (payload == null) {
      throw const ProtocolException('invalid_payload', '这段内容不是有效的配对信息');
    }
    if (payload.expired) {
      throw const ProtocolException('expired', '配对码已过期，请在服务端重新执行 pair');
    }
    if (payload.nonce.isEmpty) {
      throw const ProtocolException('invalid_payload', '配对信息缺少 nonce，无法签名');
    }
    final result = await _pair(
      address: payload.address,
      ticket: payload.ticket,
      nonce: payload.nonce,
      deviceName: deviceName,
    );
    await _adoptIdentity(result, expectedFingerprint: payload.fingerprint);
  }

  /// 免配对码自登记（服务端开启 Basic 认证时可用）。
  Future<void> pairAuto(ServerAddress address, String deviceName) async {
    final result = await _pair(
      address: address,
      ticket: '',
      nonce: '',
      deviceName: deviceName,
    );
    await _adoptIdentity(result, expectedFingerprint: '');
  }

  Future<void> _adoptIdentity(
    Identity id, {
    required String expectedFingerprint,
  }) async {
    if (expectedFingerprint.isNotEmpty &&
        id.fingerprint.isNotEmpty &&
        expectedFingerprint != id.fingerprint) {
      throw const ProtocolException(
        'unauthorized',
        '服务端指纹与配对信息不一致，已拒绝登记',
      );
    }
    identity = id;
    await _prefs.setString(_kIdentity, jsonEncode(id.toJson()));
    _syncBackgroundState(immediate: true);
    error = '';
    notifyListeners();
    unawaited(startMessageService());
    unawaited(requestNotificationPermission());
    _connect();
  }

  /// 一次性配对：连服务端、发 pair.request、等 pair.ok。
  Future<Identity> _pair({
    required ServerAddress address,
    required String ticket,
    required String nonce,
    required String deviceName,
  }) async {
    final keyPair = p256GenerateKey();
    final WebSocketChannel channel;
    try {
      channel = openWebSocket(Uri.parse(address.wsUrl));
    } catch (err) {
      throw ProtocolException('offline', '无法连接到服务：$err');
    }
    final completer = Completer<Identity>();
    StreamSubscription<dynamic>? subscription;
    Timer? timer;

    void fail(Object error) {
      if (completer.isCompleted) return;
      completer.completeError(error);
    }

    void submit() {
      final payload = <String, dynamic>{
        'ticket': ticket,
        'manager_name':
            deviceName.trim().isEmpty ? 'LianT 客户端' : deviceName.trim(),
        'manager_pub': keyPair.publicKeyBase64,
        // 免码登记没有 ticket，服务端不需要 nonce 签名。
        'sig': ticket.isEmpty
            ? ''
            : base64Encode(p256Sign(keyPair, utf8.encode(nonce))),
      };
      channel.sink.add(
        Envelope(
          id: newId(),
          type: 'pair.request',
          ts: DateTime.now().millisecondsSinceEpoch,
          payload: payload,
        ).encode(),
      );
    }

    subscription = channel.stream.listen(
      (raw) {
        final env = Envelope.tryParse(raw);
        if (env == null) return;
        if (env.type == 'auth.challenge') {
          final autoAllowed = env.payload['auto_pair'] == true;
          if (ticket.isEmpty && !autoAllowed) {
            fail(const ProtocolException(
              'no_ticket',
              '服务端没有开启免配对码接入：请用 pair 命令生成的配对码连接',
            ));
            return;
          }
          submit();
          return;
        }
        if (env.type == 'pair.ok') {
          final payload = env.payload;
          final serviceId = payload['service_id'] as String? ?? '';
          final deviceId = payload['device_id'] as String? ?? '';
          if (deviceId.isEmpty) {
            fail(const ProtocolException('internal', '服务端没有返回设备标识'));
            return;
          }
          if (!completer.isCompleted) {
            completer.complete(
              Identity(
                deviceId: deviceId,
                serviceName: payload['service_name'] as String? ?? 'LianT Service',
                host: address.host,
                port: address.port,
                fingerprint: serviceId,
                secure: address.secure,
                privateKey: keyPair.d,
                publicPoint: keyPair.pub,
              ),
            );
          }
          return;
        }
        if (env.type == 'pair.denied') {
          final reason = env.payload['reason'] as String? ?? 'denied';
          fail(ProtocolException(reason, pairDeniedText(reason)));
          return;
        }
        if (env.type == 'error') {
          fail(ProtocolException(
            env.payload['code'] as String? ?? 'internal',
            env.payload['message'] as String? ?? '配对失败',
          ));
        }
      },
      onError: (Object err) =>
          fail(ProtocolException('offline', '无法连接到服务：$err')),
      onDone: () {
        if (!completer.isCompleted) {
          fail(const ProtocolException('offline', '连接被关闭，配对没有完成'));
        }
      },
    );

    timer = Timer(const Duration(seconds: 90), () {
      fail(const ProtocolException('timeout', '配对超时，请确认服务端仍在等待确认'));
    });

    try {
      await channel.ready.timeout(const Duration(seconds: 15));
      return await completer.future;
    } on TimeoutException {
      throw const ProtocolException('timeout', '连接服务超时，请检查地址与端口');
    } finally {
      timer.cancel();
      await subscription.cancel();
      await channel.sink.close();
    }
  }

  Future<void> unpair() async {
    _saveTimer?.cancel();
    _client?.stop();
    _client = null;
    identity = null;
    conn = ConnState.idle;
    detail = '';
    error = '';
    bots = <Bot>[];
    devices = <DeviceInfo>[];
    audit = <AuditEntry>[];
    messages = <ChatMessage>[];
    caps = <String>{};
    media.clear();
    await _prefs.remove(_kIdentity);
    await _prefs.remove(_kMessages);
    await _messageBox?.clear();
    // 后台进程属于「已配对」状态：停服务、删共享状态，避免它继续用旧身份连。
    unawaited(stopMessageService());
    await clearBackgroundState();
    notifyListeners();
  }

  /// 立刻重连（界面上的「立即重试」）。
  void reconnect() {
    if (identity == null) return;
    _connect();
  }

  void _connect() {
    final id = identity;
    if (id == null) return;
    _client?.stop();
    final client = LiantClient(
      identity: id,
      onState: _onConnState,
      onEvent: _onEvent,
    );
    _client = client;
    client.start();
  }

  void _onConnState(ConnState state, String reason) {
    conn = state;
    detail = reason;
    if (state == ConnState.online) {
      error = '';
      unawaited(_syncSnapshot());
    }
    notifyListeners();
  }

  Future<void> _syncSnapshot() async {
    final client = _client;
    if (client == null || _syncing) return;
    _syncing = true;
    try {
      final snapshot =
          await client.request('state.snapshot', const <String, dynamic>{});
      final rawBots = snapshot['bots'];
      bots = rawBots is List
          ? rawBots.whereType<Map<String, dynamic>>().map(botFromJson).toList()
          : <Bot>[];
      final rawDevices = snapshot['devices'];
      devices = rawDevices is List
          ? rawDevices
              .whereType<Map<String, dynamic>>()
              .map(DeviceInfo.fromJson)
              .toList()
          : <DeviceInfo>[];
      final rawAudit = snapshot['audit'];
      audit = rawAudit is List
          ? rawAudit
              .whereType<Map<String, dynamic>>()
              .map(AuditEntry.fromJson)
              .toList()
          : <AuditEntry>[];
      final rawCaps = snapshot['caps'];
      if (rawCaps is List) caps = rawCaps.whereType<String>().toSet();
    } on ProtocolException catch (err) {
      if (err.code == 'unknown_type' || err.code == 'invalid_payload') {
        // 老服务端没有 state.snapshot：退回逐个拉取。
        await _fallbackSnapshot(client);
      } else {
        error = err.friendly;
      }
    } catch (err) {
      error = '加载失败：$err';
    } finally {
      _syncing = false;
      notifyListeners();
    }
  }

  Future<void> _fallbackSnapshot(LiantClient client) async {
    try {
      final botList = await client.request('bot.list', const <String, dynamic>{});
      final rawBots = botList['bots'];
      bots = rawBots is List
          ? rawBots.whereType<Map<String, dynamic>>().map(botFromJson).toList()
          : <Bot>[];
    } catch (_) {
      // 拉不到就保持空列表，顶部连接条会说明状态。
    }
    try {
      await loadDevices();
    } catch (_) {}
    try {
      await loadAudit();
    } catch (_) {}
  }

  // -------------------------------------------------------------------------
  // 事件
  // -------------------------------------------------------------------------

  void _onEvent(Envelope env) {
    switch (env.type) {
      case 'auth.ok':
        final version = env.payload['version'];
        if (version is String) serviceVersion = version;
        final rawCaps = env.payload['caps'];
        if (rawCaps is List) caps = rawCaps.whereType<String>().toSet();
        retention =
            Retention.fromJson(env.payload['retention'] as Map<String, dynamic>?);
      case 'msg.inbound' || 'event.msg':
        final msg = ChatMessage.fromJson(env.payload, direction: 'in');
        if (msg.msgId.isEmpty || msg.botId.isEmpty) break;
        if (messages.any((m) => m.msgId == msg.msgId)) break;
        _append(msg);
        _ack(msg.msgId);
        final key = convKey(msg.botId, msg.peerId);
        if (openConversation == key && appInForeground) {
          readAt[key] = DateTime.now().millisecondsSinceEpoch;
          unawaited(_persistMap(_kRead, readAt));
        }
        _notifyInbound(msg);
      case 'event.bot':
        final botId = env.payload['bot_id'] as String?;
        final status = env.payload['status'] as String?;
        if (botId != null && status != null) {
          bots = bots
              .map((b) => b.botId == botId ? _botWithStatus(b, status) : b)
              .toList();
        }
      case 'event.peer':
        final botId = env.payload['bot_id'] as String?;
        final peerId = env.payload['peer_id'] as String?;
        if (botId == null || peerId == null) break;
        final key = convKey(botId, peerId);
        final title = env.payload['title'];
        if (title is String && title.isNotEmpty) {
          titles[key] = title;
          unawaited(_persistMap(_kTitles, titles));
          _syncBackgroundState();
        }
        final hint = env.payload['hint'];
        if (hint is String && hint.isNotEmpty) {
          peerHints[key] = hint;
          unawaited(_persistMap(_kPeerHints, peerHints));
        }
      case 'event.audit':
        final entry = AuditEntry.fromJson(env.payload);
        audit = <AuditEntry>[entry, ...audit].take(50).toList();
        if (entry.kind == 'device.revoke' || entry.kind == 'pair.ok') {
          unawaited(loadDevices());
        }
      case 'error':
        final err = env.payload;
        error = err['message'] as String? ?? '服务端返回错误';
    }
    notifyListeners();
  }

  Bot _botWithStatus(Bot bot, String status) => Bot(
        botId: bot.botId,
        platform: bot.platform,
        status: status,
        username: bot.username,
        displayName: bot.displayName,
        caps: bot.caps,
        constraints: bot.constraints,
        since: bot.since,
      );

  void _ack(String msgId) {
    final client = _client;
    if (client == null) return;
    unawaited(client
        .request('msg.ack', <String, dynamic>{'msg_id': msgId})
        .then<void>((_) {}, onError: (Object _) {}));
  }

  void _append(ChatMessage msg) {
    if (messages.any((m) => m.msgId == msg.msgId)) return;
    messages = <ChatMessage>[...messages, msg]..sort(_byTime);
    unawaited(_persistMessage(msg));
    notifyListeners();
  }

  void _replace(String localId, ChatMessage next) {
    final bytes = localMedia.remove(localId);
    if (bytes != null && next.msgId.isNotEmpty) localMedia[next.msgId] = bytes;
    messages = messages.map((m) => m.msgId == localId ? next : m).toList()
      ..sort(_byTime);
    unawaited(_replacePersistedMessage(localId, next));
    notifyListeners();
  }

  static int _byTime(ChatMessage a, ChatMessage b) {
    final diff = a.ts - b.ts;
    return diff != 0 ? diff : a.msgId.compareTo(b.msgId);
  }

  // -------------------------------------------------------------------------
  // 查询
  // -------------------------------------------------------------------------

  Bot? botById(String botId) {
    for (final bot in bots) {
      if (bot.botId == botId) return bot;
    }
    return null;
  }

  List<ChatMessage> messagesFor(String botId, String peerId) => messages
      .where((m) => m.botId == botId && m.peerId == peerId)
      .toList();

  /// 会话列表：按最后一条消息倒序。
  List<Conversation> conversations() {
    final map = <String, Conversation>{};
    for (final msg in messages) {
      final key = convKey(msg.botId, msg.peerId);
      final conv = map[key] ??
          Conversation(
            key: key,
            botId: msg.botId,
            peerId: msg.peerId,
            peerKind: msg.peerKind,
            title: _titleFor(msg.botId, msg.peerId, msg.senderName),
          );
      final last = conv.last;
      if (last == null || msg.ts >= last.ts) conv.last = msg;
      conv.title = _titleFor(msg.botId, msg.peerId, conv.last?.senderName ?? '');
      if (!mutedFor(key) && msg.direction == 'in' && msg.ts > (readAt[key] ?? 0)) conv.unread++;
      map[key] = conv;
    }
    return map.values.toList()
      ..sort((a, b) => (b.last?.ts ?? 0) - (a.last?.ts ?? 0));
  }

  void _setForeground(bool value) {
    appInForeground = value;
    // 立即写：后台进程靠这个标志决定要不要弹通知，慢一拍就会重复弹。
    _syncBackgroundState(immediate: true);
  }

  // -------------------------------------------------------------------------
  // 后台进程共享状态
  // -------------------------------------------------------------------------

  /// 把「后台进程弹通知要用到的状态」写进共享文件。
  ///
  /// 跨进程读不了 shared_preferences（每个进程一份缓存），所以这里走文件；
  /// 写操作合并到 200ms 一次，避免连续操作时反复落盘。
  void _syncBackgroundState({bool immediate = false}) {
    _bgStateTimer?.cancel();
    if (immediate) {
      unawaited(_writeBackgroundState());
      return;
    }
    _bgStateTimer = Timer(const Duration(milliseconds: 200), () {
      unawaited(_writeBackgroundState());
    });
  }

  Future<void> _writeBackgroundState() async {
    final id = identity;
    if (id == null) {
      await clearBackgroundState();
      return;
    }
    await writeBackgroundState(
      BackgroundState(
        identity: id,
        foreground: appInForeground,
        muted: Map<String, bool>.of(muted),
        titles: Map<String, String>.of(titles),
        remarks: Map<String, String>.of(remarks),
      ),
    );
  }

  void _notifyInbound(ChatMessage msg) {
    // UI 进程照旧弹一次；:background 进程也会弹（UI 不在前台时），两边用
    // 同一个通知 ID + setOnlyAlertOnce，所以同一条消息最多响一次。
    final key = convKey(msg.botId, msg.peerId);
    if (mutedFor(key)) return;
    if (openConversation == key) return;
    final list = messagesFor(msg.botId, msg.peerId);
    final convTitle = conversationTitle(msg.botId, msg.peerId, list);
    final sender = msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName;
    final isGroup = msg.peerKind == 'group';
    final title = _clipText(
      isGroup ? convTitle : (msg.senderName.isEmpty ? convTitle : msg.senderName),
      36,
    );
    final summary = _notificationSummary(msg);
    final body = _clipText(
      isGroup ? '$sender: $summary' : summary,
      90,
    );
    final id = notificationIdFor(msg.msgId);
    unawaited(showMessageNotification(id: id, title: title, body: body));
  }

  String _notificationSummary(ChatMessage msg) {
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

  String _clipText(String text, int max) {
    final flat = text.replaceAll(RegExp(r'\s+'), ' ').trim();
    if (flat.length <= max) return flat;
    return '${flat.substring(0, max)}...';
  }

  String _titleFor(String botId, String peerId, String senderName) {
    final key = convKey(botId, peerId);
    final remark = remarks[key];
    if (remark != null && remark.isNotEmpty) return remark;
    final title = titles[key];
    if (title != null && title.isNotEmpty) return title;
    if (senderName.isNotEmpty) return senderName;
    return shortPeer(peerId);
  }

  String conversationTitle(String botId, String peerId, List<ChatMessage> list) {
    final known = _titleFor(botId, peerId, '');
    if (known != shortPeer(peerId)) return known;
    for (final msg in list) {
      if (msg.senderName.isNotEmpty) return msg.senderName;
    }
    return known;
  }

  // -------------------------------------------------------------------------
  // 动作
  // -------------------------------------------------------------------------

  Future<void> refreshBots() async {
    final client = _client;
    if (client == null) return;
    final res = await client.request('bot.list', const <String, dynamic>{});
    final raw = res['bots'];
    bots = raw is List
        ? raw.whereType<Map<String, dynamic>>().map(botFromJson).toList()
        : <Bot>[];
    notifyListeners();
  }

  Future<void> botStatus(String botId) async {
    final client = _client;
    if (client == null) return;
    final res =
        await client.request('bot.status', <String, dynamic>{'bot_id': botId});
    final status = res['status'] as String?;
    if (status != null) {
      bots = bots
          .map((b) => b.botId == botId ? _botWithStatus(b, status) : b)
          .toList();
      notifyListeners();
    }
  }

  Future<String> addBot({
    required String platform,
    required Map<String, String> credentials,
    String label = '',
  }) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    final res = await client.request('bot.add', <String, dynamic>{
      'platform': platform,
      'credentials': credentials,
      if (label.trim().isNotEmpty) 'label': label.trim(),
    });
    await refreshBots();
    return res['bot_id'] as String? ?? '';
  }

  Future<void> startBot(String botId) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('bot.start', <String, dynamic>{'bot_id': botId});
    await botStatus(botId);
    await refreshBots();
  }

  Future<void> stopBot(String botId) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('bot.stop', <String, dynamic>{'bot_id': botId});
    await botStatus(botId);
    await refreshBots();
  }

  Future<void> removeBot(String botId) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('bot.remove', <String, dynamic>{'bot_id': botId});
    await refreshBots();
  }

  Future<void> loadHistory(String botId, String peerId) async {
    final client = _client;
    // 离线时没有历史可拉；聊天页会自己显示「连接恢复后加载」。
    if (client == null) return;
    final seen = messages.map((m) => m.msgId).toSet();
    final added = <ChatMessage>[];
    Map<String, dynamic>? before;
    while (true) {
      final res = await client.request('msg.list', <String, dynamic>{
        'bot_id': botId,
        'peer_id': peerId,
        'limit': 100,
        if (before != null) 'before': before,
      });
      final raw = res['messages'];
      final page = <ChatMessage>[];
      if (raw is List) {
        for (final item in raw) {
          if (item is Map<String, dynamic>) {
            final msg = ChatMessage.fromJson(item);
            if (msg.msgId.isNotEmpty) {
              page.add(msg);
              if (seen.add(msg.msgId)) added.add(msg);
            }
          }
        }
      }
      if (page.isEmpty || res['has_more'] != true) break;
      final oldest = page.first;
      before = <String, dynamic>{
        'ts': oldest.ts,
        'msg_id': oldest.msgId,
      };
    }
    if (added.isNotEmpty) {
      messages = <ChatMessage>[...messages, ...added]..sort(_byTime);
      for (final msg in added) {
        unawaited(_persistMessage(msg));
      }
      notifyListeners();
    }
  }

  Future<void> send({
    required Bot bot,
    required String peerId,
    required String peerKind,
    required String text,
    String? inReplyTo,
    String? quoteRef,
    bool active = false,
    String? senderName,
  }) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    final localId = 'local-${newId()}';
    final optimistic = ChatMessage(
      msgId: localId,
      botId: bot.botId,
      peerId: peerId,
      peerKind: peerKind,
      body: MsgBody(plain: text),
      ts: DateTime.now().millisecondsSinceEpoch,
      direction: 'out',
      inReplyTo: active ? null : inReplyTo,
      quoteRef: active ? '' : (quoteRef ?? ''),
      pending: true,
      active: active,
      senderName: senderName ?? '',
    );
    _append(optimistic);
    try {
      final res = await client.request('msg.send', <String, dynamic>{
        'bot_id': bot.botId,
        'peer_id': peerId,
        'body': <String, dynamic>{'plain': text},
        if (!active && inReplyTo != null) 'in_reply_to': inReplyTo,
        if (!active && quoteRef != null && quoteRef.isNotEmpty)
          'quote_ref': quoteRef,
        if (active) 'active': true,
      });
      final msgId = res['msg_id'] as String? ?? localId;
      final ts = res['ts'] is num ? (res['ts'] as num).toInt() : optimistic.ts;
      _replace(localId, optimistic.copyWith(msgId: msgId, ts: ts, pending: false));
    } catch (err) {
      _replace(localId, optimistic.copyWith(pending: false, failed: true));
      rethrow;
    }
  }

  Future<void> sendMedia({
    required Bot bot,
    required String peerId,
    required String peerKind,
    required Uint8List data,
    required String name,
    required String kind,
    String? inReplyTo,
    String? quoteRef,
    bool active = false,
  }) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    final localId = 'local-${newId()}';
    final optimistic = ChatMessage(
      msgId: localId,
      botId: bot.botId,
      peerId: peerId,
      peerKind: peerKind,
      body: MsgBody(plain: name),
      ts: DateTime.now().millisecondsSinceEpoch,
      direction: 'out',
      msgType: kind,
      inReplyTo: active ? null : inReplyTo,
      quoteRef: active ? '' : (quoteRef ?? ''),
      pending: true,
      active: active,
    );
    localMedia[localId] = data;
    _append(optimistic);
    try {
      final res = await client.request('msg.send', <String, dynamic>{
        'bot_id': bot.botId,
        'peer_id': peerId,
        'body': <String, dynamic>{'plain': name},
        'media': <String, dynamic>{
          'kind': kind,
          'name': name,
          'data': base64Encode(data),
        },
        if (!active && inReplyTo != null) 'in_reply_to': inReplyTo,
        if (!active && quoteRef != null && quoteRef.isNotEmpty)
          'quote_ref': quoteRef,
        if (active) 'active': true,
      });
      final msgId = res['msg_id'] as String? ?? localId;
      final ts = res['ts'] is num ? (res['ts'] as num).toInt() : optimistic.ts;
      _replace(localId, optimistic.copyWith(msgId: msgId, ts: ts, pending: false));
    } catch (err) {
      _replace(localId, optimistic.copyWith(pending: false, failed: true));
      rethrow;
    }
  }

  Future<void> muteMember({
    required String botId,
    required String peerId,
    required String memberId,
    required int seconds,
  }) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('member.mute', <String, dynamic>{
      'bot_id': botId,
      'peer_id': peerId,
      'member_id': memberId,
      'seconds': seconds,
    });
  }

  Future<void> kickMember({
    required String botId,
    required String peerId,
    required String memberId,
    bool blacklist = false,
  }) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('member.kick', <String, dynamic>{
      'bot_id': botId,
      'peer_id': peerId,
      'member_id': memberId,
      'blacklist': blacklist,
    });
  }

  Future<void> retrySend(ChatMessage msg, Bot bot) async {
    messages = messages.where((m) => m.msgId != msg.msgId).toList();
    unawaited(_deletePersistedMessage(msg.msgId));
    notifyListeners();
    await send(
      bot: bot,
      peerId: msg.peerId,
      peerKind: msg.peerKind,
      text: msg.body.plain,
      inReplyTo: msg.inReplyTo,
      quoteRef: msg.quoteRef,
      active: msg.active,
    );
  }

  Future<void> fetchMedia(String botId, String msgId) async {
    final key = '$botId/$msgId';
    if (media.containsKey(key)) return;
    final client = _client;
    if (client == null) return;
    media[key] = const MediaState.loading();
    notifyListeners();
    try {
      final res = await client.request('media.fetch', <String, dynamic>{
        'bot_id': botId,
        'msg_id': msgId,
      });
      final url = res['url'] as String? ?? '';
      final mime = res['mime'] as String? ?? '';
      media[key] = url.isEmpty
          ? const MediaState.failed('媒体地址为空')
          : MediaState.ready(client.resolveMediaUrl(url), mime: mime);
    } on ProtocolException catch (err) {
      media[key] = MediaState.failed(err.friendly);
    } catch (_) {
      media[key] = const MediaState.failed('媒体加载失败');
    }
    notifyListeners();
  }

  Future<void> loadDevices() async {
    final client = _client;
    if (client == null) return;
    final res = await client.request('device.list', const <String, dynamic>{});
    final raw = res['devices'];
    devices = raw is List
        ? raw.whereType<Map<String, dynamic>>().map(DeviceInfo.fromJson).toList()
        : <DeviceInfo>[];
    notifyListeners();
  }

  Future<void> revokeDevice(String deviceId) async {
    final client = _client;
    if (client == null) throw const ProtocolException('offline', '连接未就绪');
    await client.request('device.revoke', <String, dynamic>{'device_id': deviceId});
    await loadDevices();
    await loadAudit();
  }

  Future<void> loadAudit() async {
    final client = _client;
    if (client == null) return;
    final res = await client.request('audit.list', <String, dynamic>{'limit': 20});
    final raw = res['entries'];
    audit = raw is List
        ? raw.whereType<Map<String, dynamic>>().map(AuditEntry.fromJson).toList()
        : <AuditEntry>[];
    notifyListeners();
  }

  void markRead(String key) {
    readAt[key] = DateTime.now().millisecondsSinceEpoch;
    unawaited(_persistMap(_kRead, readAt));
    notifyListeners();
  }

  bool mutedFor(String key) => muted[key] ?? false;

  void setAcceptBeta(bool value) {
    acceptBeta = value;
    unawaited(_prefs.setBool(_kAcceptBeta, value));
    notifyListeners();
  }

  void setMuted(String key, bool on) {
    if (on) {
      muted[key] = true;
    } else {
      muted.remove(key);
    }
    unawaited(_persistMap(_kMuted, muted));
    _syncBackgroundState();
    notifyListeners();
  }

  void setRemark(String key, String name) {
    final trimmed = name.trim();
    if (trimmed.isEmpty) {
      remarks.remove(key);
    } else {
      remarks[key] = trimmed;
    }
    unawaited(_persistMap(_kRemarks, remarks));
    _syncBackgroundState();
    notifyListeners();
  }

  void setOpenConversation(String? key) {
    openConversation = key;
  }

  void clearError() {
    error = '';
    notifyListeners();
  }

  @override
  void dispose() {
    _saveTimer?.cancel();
    _bgStateTimer?.cancel();
    _lifecycle?.dispose();
    // 同样不停后台服务：只有解除配对才真正停。
    _client?.stop();
    super.dispose();
  }

  // -------------------------------------------------------------------------
  // 本地存储
  // -------------------------------------------------------------------------

  /// 会话列表没有服务端索引，历史只能靠本地留存；与网页端一样按 msg_id 去重。
  void _loadMessages() {
    final loaded = <ChatMessage>[];
    final box = _messageBox;
    if (box != null) {
      for (final raw in box.values) {
        try {
          final decoded = jsonDecode(raw);
          if (decoded is Map<String, dynamic>) {
            final msg = ChatMessage.fromJson(decoded);
            if (msg.msgId.isNotEmpty) loaded.add(msg);
          }
        } on FormatException {
          // 单条坏数据忽略。
        }
      }
    }
    if (loaded.isEmpty) {
      // 迁移旧版 shared_preferences 里的消息，迁移成功后只保留 Hive。
      final raw = _prefs.getString(_kMessages);
      if (raw != null) {
        try {
          final decoded = jsonDecode(raw);
          if (decoded is List) {
            for (final item in decoded) {
              if (item is Map<String, dynamic>) {
                final msg = ChatMessage.fromJson(item);
                if (msg.msgId.isNotEmpty) loaded.add(msg);
              }
            }
          }
        } on FormatException {
          // 坏数据忽略。
        }
        if (loaded.isNotEmpty) {
          for (final msg in loaded) {
            unawaited(_persistMessage(msg));
          }
        }
        unawaited(_prefs.remove(_kMessages));
      }
    }
    messages = loaded..sort(_byTime);
  }

  Future<void> _persistMessage(ChatMessage msg) async {
    final box = _messageBox;
    if (box == null) return;
    try {
      await box.put(msg.msgId, jsonEncode(msg.toJson()));
    } catch (_) {
      // 本地存储失败不影响当前会话。
    }
  }

  Future<void> _replacePersistedMessage(
    String localId,
    ChatMessage next,
  ) async {
    final box = _messageBox;
    if (box == null) return;
    try {
      await box.delete(localId);
      await box.put(next.msgId, jsonEncode(next.toJson()));
    } catch (_) {
      // 忽略本地失败。
    }
  }

  Future<void> _deletePersistedMessage(String msgId) async {
    final box = _messageBox;
    if (box == null) return;
    try {
      await box.delete(msgId);
    } catch (_) {}
  }

  void _loadMap<T>(String key, Map<String, T> target) {
    final raw = _prefs.getString(key);
    if (raw == null) return;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return;
      decoded.forEach((k, v) {
        if (v is T) target[k] = v;
      });
    } on FormatException {
      // 本地数据损坏时忽略，不阻塞启动。
    }
  }

  Future<void> _persistMap(String key, Map<String, dynamic> value) async {
    try {
      await _prefs.setString(key, jsonEncode(value));
    } catch (_) {
      // 存储失败不影响当前会话。
    }
  }
}

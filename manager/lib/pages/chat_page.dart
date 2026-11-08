// 聊天页：玻璃头部随滚动折叠，群聊显示发送者，支持被动回复与本地备注。
import 'dart:async';
import 'dart:ui' as ui;

import 'package:file_picker/file_picker.dart';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../core/app_state.dart';
import '../core/protocol.dart';
import '../ui/format.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';
import '../l10n/l10n.dart';
import '../l10n/ltext.dart';

class ChatPage extends StatefulWidget {
  const ChatPage({super.key, required this.botId, required this.peerId});

  final String botId;
  final String peerId;

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  final TextEditingController _input = TextEditingController();
  final ScrollController _scroll = ScrollController();
  final Set<String> _mediaRequested = <String>{};

  AppState? _state;
  Timer? _ticker;
  bool _bootstrapped = false;
  bool _historyLoading = false;
  bool _historyLoaded = false;
  bool _collapsed = false;
  bool _sending = false;
  String _historyError = '';
  String _sendError = '';
  ChatMessage? _replyTo;
  final FocusNode _inputFocus = FocusNode();
  bool _mentioning = false;
  String _mentionName = '';
  bool _panelOpen = false;
  int _panelPage = 0;
  Uint8List? _pendingBytes;
  String _pendingName = '';
  String _pendingKind = '';
  final PageController _panelController = PageController();
  int _lastCount = 0;

  String get _key => convKey(widget.botId, widget.peerId);

  @override
  void initState() {
    super.initState();
    _scroll.addListener(_onScroll);
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      final bot = _state?.botById(widget.botId);
      if (bot?.constraints.replyWindowS != null) setState(() {});
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final state = AppScope.of(context);
    _state = state;
    final list = state.messagesFor(widget.botId, widget.peerId);
    final nearBottom = !_scroll.hasClients ||
        _scroll.position.maxScrollExtent - _scroll.offset < 160;

    if (!_bootstrapped) {
      _bootstrapped = true;
      _lastCount = list.length;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final current = AppScope.of(context);
        current.markRead(_key);
        current.setOpenConversation(_key);
        _maybeLoadHistory(current);
        _scrollToBottom(jump: true);
      });
    } else if (list.length != _lastCount) {
      final grew = list.length > _lastCount;
      _lastCount = list.length;
      if (grew) {
        // 新消息进来时只在接近底部才自动跟随，避免抢用户正在翻的历史。
        if (nearBottom) _scrollToBottom(jump: false);
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) AppScope.of(context).markRead(_key);
        });
      }
    }

    if (state.online && !_historyLoaded) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _maybeLoadHistory(AppScope.of(context));
      });
    }
    _fetchMedia(state, list);
  }

  @override
  void dispose() {
    _ticker?.cancel();
    _scroll.removeListener(_onScroll);
    _scroll.dispose();
    _input.dispose();
    _inputFocus.dispose();
    _panelController.dispose();
    final state = _state;
    if (state?.openConversation == _key) state?.setOpenConversation(null);
    super.dispose();
  }

  void _onScroll() {
    if (!_scroll.hasClients) return;
    final collapsed = _scroll.offset > 24;
    if (collapsed != _collapsed) setState(() => _collapsed = collapsed);
  }

  void _maybeLoadHistory(AppState state) {
    if (!state.online || _historyLoaded || _historyLoading) return;
    unawaited(_loadHistory(state));
  }

  Future<void> _loadHistory(AppState state) async {
    if (_historyLoading || !state.online) return;
    setState(() {
      _historyLoading = true;
      _historyError = '';
    });
    try {
      await state.loadHistory(widget.botId, widget.peerId);
      _historyLoaded = true;
      _scrollToBottom(jump: false);
    } on ProtocolException catch (err) {
      if (mounted) setState(() => _historyError = err.friendly);
    } catch (err) {
      if (mounted) setState(() => _historyError = '历史消息拉取失败：$err');
    } finally {
      if (mounted) setState(() => _historyLoading = false);
    }
  }

  void _fetchMedia(AppState state, List<ChatMessage> list) {
    for (final msg in list) {
      if (!msg.isMedia) continue;
      final id = '$_key|${msg.msgId}';
      if (_mediaRequested.contains(id)) continue;
      _mediaRequested.add(id);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(state.fetchMedia(msg.botId, msg.msgId));
      });
    }
  }

  void _scrollToBottom({bool jump = false}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients) return;
      final target = _scroll.position.maxScrollExtent;
      if (jump) {
        _scroll.jumpTo(target);
      } else {
        _scroll.animateTo(
          target,
          duration: const Duration(milliseconds: 220),
          curve: Curves.easeOut,
        );
      }
    });
  }

  ChatMessage? _lastInbound(List<ChatMessage> list) {
    for (var i = list.length - 1; i >= 0; i--) {
      if (list[i].direction == 'in') return list[i];
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    final bot = state.botById(widget.botId);
    final list = state.messagesFor(widget.botId, widget.peerId);
    final title = state.conversationTitle(widget.botId, widget.peerId, list);
    final lastIn = _lastInbound(list);
    final windowSeconds = bot?.constraints.replyWindowS;
    int? windowLeft;
    if (windowSeconds != null && lastIn != null) {
      final deadline = lastIn.ts + windowSeconds * 1000;
      windowLeft = ((deadline - DateTime.now().millisecondsSinceEpoch) / 1000).ceil();
    }
    final topInset = MediaQuery.paddingOf(context).top;
    final headerHeight = _collapsed ? 56.0 : 76.0;

    return Scaffold(
      resizeToAvoidBottomInset: true,
      backgroundColor: colors.bg,
      body: Stack(
        children: <Widget>[
          Positioned.fill(
            child: ListView(
              controller: _scroll,
              padding: EdgeInsets.fromLTRB(
                16,
                topInset + headerHeight + 8,
                16,
                128 +
                    (_replyTo != null ? 54 : 0) +
                    (_panelOpen ? _attachPanelHeight(_panelPage) : 0),
              ),
              children: _messageList(context, state, list, bot),
            ),
          ),
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            child: _header(
              context,
              state,
              bot,
              title,
              windowLeft,
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: _composer(context, state, bot),
          ),
        ],
      ),
    );
  }

  List<Widget> _messageList(
    BuildContext context,
    AppState state,
    List<ChatMessage> list,
    Bot? bot,
  ) {
    final colors = context.colors;
    final widgets = <Widget>[];
    if (list.isEmpty) {
      if (_historyLoading) {
        widgets.add(const Padding(
          padding: EdgeInsets.symmetric(vertical: 40),
          child: Center(
            child: SizedBox(
              width: 20,
              height: 20,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        ));
        widgets.add(LText(
          '正在加载历史…',
          textAlign: TextAlign.center,
          style: context.texts.bodySmall?.copyWith(color: colors.text3),
        ));
      } else if (_historyError.isNotEmpty) {
        widgets.add(NoticeCard(
          title: '历史消息拉取失败',
          message: _historyError,
          actionLabel: '重试',
          onAction: () => unawaited(_loadHistory(state)),
        ));
      } else if (!state.online) {
        widgets.add(Padding(
          padding: const EdgeInsets.only(top: 56),
          child: LText(
            '连接恢复后会自动加载历史',
            textAlign: TextAlign.center,
            style: context.texts.bodySmall?.copyWith(color: colors.text3),
          ),
        ));
      } else {
        widgets.add(EmptyState(
          icon: Icons.chat_outlined,
          title: '还没有消息',
          hint: state.bots.isEmpty
              ? '这台服务上还没有配置机器人'
              : '对方发来消息后会出现在这里',
        ));
      }
      return widgets;
    }

    if (_historyError.isNotEmpty) {
      widgets.add(NoticeCard(
        title: '历史消息拉取失败',
        message: _historyError,
        actionLabel: '重试',
        onAction: () => unawaited(_loadHistory(state)),
      ));
    }

    // 末尾的历史提示放在列表顶部，避免遮挡正在阅读的最后一条。
    final byId = <String, ChatMessage>{
      for (final m in list)
        if (m.msgId.isNotEmpty) m.msgId: m,
    };
    for (var i = 0; i < list.length; i++) {
      final msg = list[i];
      final prev = i > 0 ? list[i - 1] : null;
      final gap = prev == null || msg.ts - prev.ts > 5 * 60 * 1000;
      if (gap) {
        widgets.add(Padding(
          padding: const EdgeInsets.symmetric(vertical: 12),
          child: LText(
            formatMessageTime(msg.ts),
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 11, color: colors.text3),
          ),
        ));
      }
      final isGroup = msg.peerKind == 'group';
      if (isGroup && !msg.outgoing) {
        final sender = msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName;
        final role = msg.senderRole;
        final roleLabel = role == 'owner' ? '群主' : (role == 'admin' ? '管理员' : '');
        final roleColor = role == 'owner'
            ? const Color(0xFFFFB300)
            : role == 'admin'
                ? const Color(0xFF2EAD55)
                : colors.text2;
        widgets.add(Padding(
          padding: const EdgeInsets.only(left: 8, bottom: 3),
          child: Semantics(
            button: true,
            label: '查看 $sender 的操作',
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => unawaited(_showUserActions(msg)),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  LText(
                    sender,
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: roleColor,
                    ),
                  ),
                  if (roleLabel.isNotEmpty) ...<Widget>[
                    const SizedBox(width: 4),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                      decoration: BoxDecoration(
                        color: roleColor.withOpacity(0.14),
                        borderRadius: BorderRadius.circular(5),
                      ),
                      child: LText(
                        roleLabel,
                        style: TextStyle(
                          fontSize: 9,
                          fontWeight: FontWeight.w600,
                          color: roleColor,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ));
      }
      widgets.add(Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Align(
          alignment: msg.outgoing ? Alignment.centerRight : Alignment.centerLeft,
          child: _bubble(
            context,
            state,
            msg,
            bot,
            quoted: msg.inReplyTo == null ? null : byId[msg.inReplyTo],
          ),
        ),
      ));
    }
    widgets.add(const SizedBox(height: 8));
    return widgets;
  }

  Widget _imageBubble(BuildContext context, ImageProvider provider) {
    return GestureDetector(
      onTap: () => _openFullscreenImage(context, provider),
      child: Stack(
        children: <Widget>[
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: Image(
              image: provider,
              width: double.infinity,
              height: 220,
              fit: BoxFit.cover,
              errorBuilder: (_, __, ___) => Container(
                width: double.infinity,
                height: 160,
                alignment: Alignment.center,
                child: LText(
                  '媒体加载失败',
                  style: TextStyle(fontSize: 14, color: context.colors.text2),
                ),
              ),
            ),
          ),
          Positioned(
            top: 6,
            right: 6,
            child: Container(
              padding: const EdgeInsets.all(4),
              decoration: BoxDecoration(
                color: Colors.black54,
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Icon(
                Icons.fullscreen_rounded,
                size: 18,
                color: Colors.white,
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _openFullscreenImage(BuildContext context, ImageProvider provider) {
    Navigator.of(context).push(
      PageRouteBuilder<void>(
        pageBuilder: (context, animation, secondaryAnimation) => Scaffold(
          backgroundColor: Colors.black,
          body: Stack(
            children: <Widget>[
              Positioned.fill(
                child: InteractiveViewer(
                  child: Center(
                    child: Image(image: provider, fit: BoxFit.contain),
                  ),
                ),
              ),
              SafeArea(
                child: Align(
                  alignment: Alignment.topRight,
                  child: IconButton(
                    onPressed: () => Navigator.of(context).maybePop(),
                    icon: const Icon(Icons.close_rounded, color: Colors.white),
                  ),
                ),
              ),
            ],
          ),
        ),
        transitionsBuilder: (context, animation, secondaryAnimation, child) =>
            FadeTransition(opacity: animation, child: child),
      ),
    );
  }

  Widget _bubble(
    BuildContext context,
    AppState state,
    ChatMessage msg,
    Bot? bot, {
    ChatMessage? quoted,
  }) {
    final colors = context.colors;
    final outgoing = msg.outgoing;
    final mediaKey = '${msg.botId}/${msg.msgId}';
    final media = state.media[mediaKey];
    final maxWidth = MediaQuery.sizeOf(context).width * 0.78;

    Widget content;
    final localBytes = state.localMedia[msg.msgId];
    if (msg.isMedia && msg.msgType == 'image' && localBytes != null) {
      content = _imageBubble(context, MemoryImage(localBytes));
    } else if (msg.isMedia && msg.msgType == 'file') {
      final name = msg.body.plain.trim();
      content = LText(
        name.isEmpty ? '[文件]' : '[文件] $name',
        style: TextStyle(fontSize: 15, color: outgoing ? Colors.white : colors.text),
      );
    } else if (msg.isMedia && msg.msgType == 'video') {
      content = LText(
        '[视频]',
        style: TextStyle(fontSize: 15, color: outgoing ? Colors.white : colors.text),
      );
    } else if (msg.isMedia && msg.msgType == 'audio') {
      content = LText(
        '[语音]',
        style: TextStyle(fontSize: 15, color: outgoing ? Colors.white : colors.text),
      );
    } else if (msg.isMedia) {
      if (media == null || media.loading) {
        content = LText(
          '正在加载媒体…',
          style: TextStyle(fontSize: 15, color: (outgoing ? Colors.white : colors.text).withOpacity(0.8)),
        );
      } else if (media.url != null) {
        content = _imageBubble(context, NetworkImage(media.url!));
      } else {
        content = LText(
          media.error ?? msg.body.plain,
          style: TextStyle(fontSize: 14, color: (outgoing ? Colors.white : colors.text2)),
        );
      }
    } else {
      content = RichBody(
        body: msg.body,
        color: outgoing ? Colors.white : colors.text,
        accentColor: outgoing ? Colors.white : colors.accent,
      );
    }

    final bubble = Container(
      constraints: BoxConstraints(maxWidth: maxWidth),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: outgoing ? colors.accent : colors.elevated,
        borderRadius: BorderRadius.only(
          topLeft: const Radius.circular(18),
          topRight: const Radius.circular(18),
          bottomLeft: Radius.circular(outgoing ? 18 : 6),
          bottomRight: Radius.circular(outgoing ? 6 : 18),
        ),
        border: msg.failed ? Border.all(color: colors.danger, width: 1) : null,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          if (msg.inReplyTo != null && msg.inReplyTo!.isNotEmpty)
            Container(
              constraints: const BoxConstraints(maxWidth: 260),
              margin: const EdgeInsets.only(bottom: 6),
              padding: const EdgeInsets.fromLTRB(8, 5, 8, 5),
              decoration: BoxDecoration(
                color: outgoing ? Colors.white.withOpacity(0.16) : colors.bg,
                borderRadius: BorderRadius.circular(8),
                border: Border(
                  left: BorderSide(
                    color: outgoing ? Colors.white70 : colors.accent,
                    width: 2,
                  ),
                ),
              ),
              child: LText(
                quoted == null
                    ? '引用了一条消息'
                    : '${quoted.outgoing ? '我' : (quoted.senderName.isEmpty ? '对方' : quoted.senderName)}：'
                        '${quoted.body.plain.isEmpty ? '媒体消息' : quoted.body.plain}',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11,
                  color: outgoing ? Colors.white : colors.text2,
                ),
              ),
            ),
          if (msg.failed)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: LText(
                '发送失败 · 点击重试',
                style: TextStyle(fontSize: 12, color: outgoing ? Colors.white : colors.danger),
              ),
            ),
          Row(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.center,
            children: <Widget>[
              Flexible(child: content),
              if (msg.pending) ...<Widget>[
                const SizedBox(width: 8),
                SizedBox(
                  width: 13,
                  height: 13,
                  child: CircularProgressIndicator(
                    strokeWidth: 1.6,
                    color: (outgoing ? Colors.white : colors.text2).withOpacity(0.8),
                  ),
                ),
              ],
            ],
          ),
        ],
      ),
    );

    return GestureDetector(
      onTap: () {
        if (msg.failed && bot != null) {
          unawaited(state.retrySend(msg, bot));
        } else {
          unawaited(_showActions(state, msg));
        }
      },
      onLongPress: () => unawaited(_showActions(state, msg)),
      child: bubble,
    );
  }

  Widget _header(
    BuildContext context,
    AppState state,
    Bot? bot,
    String title,
    int? windowLeft,
  ) {
    final colors = context.colors;
    final hint = state.peerHints[_key];
    final subtitle = hint == 'whitelist' ? '群名需 QQ 白名单授权' : widget.peerId;
    final botLine = <String>[
      bot?.title ?? widget.botId,
      if (windowLeft != null)
        windowLeft > 0
            ? '回复窗口 ${formatCountdown(windowLeft)}'
            : '回复窗口已过',
    ].join(' · ');

    return ColoredBox(
      color: colors.bg,
      child: SafeArea(
        bottom: false,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
        height: _collapsed ? 56 : 76,
        child: ClipRect(
          child: BackdropFilter(
            filter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
            child: Container(
              decoration: BoxDecoration(
                color: colors.elevated.withOpacity(0.72),
                border: Border(
                  bottom: BorderSide(
                    color: _collapsed ? colors.sep : Colors.transparent,
                    width: 0.5,
                  ),
                ),
              ),
              child: Row(
                children: <Widget>[
                  SizedBox(
                    width: 76,
                    height: 44,
                    child: InkWell(
                      onTap: () => Navigator.of(context).maybePop(),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: <Widget>[
                          Icon(Icons.chevron_left_rounded, size: 24, color: colors.accent),
                          LText(
                            '返回',
                            style: TextStyle(fontSize: 15, color: colors.accent),
                          ),
                        ],
                      ),
                    ),
                  ),
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: <Widget>[
                        LText(
                          title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: _collapsed ? 15 : 17,
                            fontWeight: FontWeight.w600,
                            color: colors.text,
                          ),
                        ),
                        if (!_collapsed) ...<Widget>[
                          const SizedBox(height: 1),
                          LText(
                            subtitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11, color: colors.text3),
                          ),
                          LText(
                            botLine,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11, color: colors.text2),
                          ),
                        ],
                      ],
                    ),
                  ),
                  SizedBox(
                    width: 60,
                    height: 44,
                    child: InkWell(
                      onTap: () => unawaited(_openSettings(state)),
                      child: Center(
                        child: Icon(
                          Icons.more_horiz_rounded,
                          size: 24,
                          color: colors.accent,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
          ),
        ),
      ),
    );
  }

  Widget _composer(BuildContext context, AppState state, Bot? bot) {
    final colors = context.colors;
    final enabled = state.online && bot != null;
    return ClipRect(
      child: BackdropFilter(
        filter: ui.ImageFilter.blur(sigmaX: 20, sigmaY: 20),
        child: Container(
          color: colors.elevated.withOpacity(0.82),
          child: SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  if (_pendingBytes != null) _pendingPreview(context),
                  if (_replyTo != null) _replyPreview(context),
                  if (_mentioning) _mentionPreview(context),
                  if (_sendError.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 6, left: 4, right: 4),
                      child: Align(
                        alignment: Alignment.centerLeft,
                        child: LText(
                          _sendError,
                          style: TextStyle(fontSize: 12, color: colors.danger),
                        ),
                      ),
                    ),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: <Widget>[
                      Semantics(
                        button: true,
                        label: _panelOpen ? '收起更多功能' : '更多功能',
                        child: GestureDetector(
                          onTap: _togglePanel,
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 150),
                            width: 44,
                            height: 44,
                            decoration: BoxDecoration(
                              color: _panelOpen ? colors.accent : colors.bg,
                              shape: BoxShape.circle,
                              border: Border.all(color: colors.sep, width: 0.5),
                            ),
                            child: Icon(
                              _panelOpen ? Icons.close_rounded : Icons.add_rounded,
                              size: 22,
                              color: _panelOpen ? Colors.white : colors.text2,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Container(
                          constraints: const BoxConstraints(minHeight: 44, maxHeight: 132),
                          padding: const EdgeInsets.symmetric(horizontal: 14),
                          decoration: BoxDecoration(
                            color: colors.bg,
                            borderRadius: BorderRadius.circular(22),
                            border: Border.all(color: colors.sep, width: 0.5),
                          ),
                          // 不要用 Center 包 TextField：Align 会吃掉 maxHeight，
                          // 单行输入框会被撑到 132。让文本域自己按内容决定高度。
                          child: TextField(
                            controller: _input,
                            focusNode: _inputFocus,
                            onTapOutside: (_) {},
                            minLines: 1,
                            maxLines: 5,
                            keyboardType: TextInputType.multiline,
                            textInputAction: TextInputAction.newline,
                            textAlignVertical: TextAlignVertical.center,
                            style: TextStyle(fontSize: 16, height: 1.35, color: colors.text),
                            decoration: InputDecoration(
                              hintText: L10n.tr(state.online ? '发消息…' : '连接中…'),
                              contentPadding: const EdgeInsets.symmetric(vertical: 11),
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Semantics(
                        button: true,
                        label: '发送',
                        child: GestureDetector(
                          onTap: enabled && !_sending ? () => unawaited(_submit(state, bot)) : null,
                          child: AnimatedContainer(
                            duration: const Duration(milliseconds: 150),
                            width: 44,
                            height: 44,
                            decoration: BoxDecoration(
                              color: enabled ? colors.accent : colors.sunken,
                              shape: BoxShape.circle,
                            ),
                            child: Icon(
                              Icons.arrow_upward_rounded,
                              size: 20,
                              color: enabled ? Colors.white : colors.text3,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (_panelOpen) _attachPanel(context),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  List<_AttachAction> get _attachActions => <_AttachAction>[
        _AttachAction(
          icon: Icons.perm_media_outlined,
          label: '媒体',
          onTap: () => unawaited(_pickAttachment(media: true)),
        ),
        _AttachAction(
          icon: Icons.insert_drive_file_outlined,
          label: '文件',
          onTap: () => unawaited(_pickAttachment(media: false)),
        ),
      ];

  int get _attachPageCount {
    final count = _attachActions.length;
    return count == 0 ? 1 : (count + 7) ~/ 8;
  }

  List<_AttachAction> _pageActions(int page) =>
      _attachActions.skip(page * 8).take(8).toList();

  int _attachRows(int page) {
    final count = _pageActions(page).length;
    return ((count + 3) ~/ 4).clamp(1, 2).toInt();
  }

  double _attachPanelHeight(int page) {
    final safePage = page.clamp(0, _attachPageCount - 1).toInt();
    final rows = _attachRows(safePage);
    final dots = _attachPageCount > 1 ? 18.0 : 0.0;
    return 16 + rows * 72.0 + (rows - 1) * 8.0 + dots;
  }

  void _togglePanel() {
    setState(() => _panelOpen = !_panelOpen);
    if (_panelOpen) {
      _scrollToBottom();
    } else {
      _focusInput();
    }
  }

  Widget _attachPanel(BuildContext context) {
    final colors = context.colors;
    final pages = _attachPageCount;
    final current = _panelPage.clamp(0, pages - 1).toInt();
    return AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      height: _attachPanelHeight(current),
      margin: const EdgeInsets.only(top: 8),
      decoration: BoxDecoration(
        color: colors.bg.withOpacity(0.55),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Column(
        children: <Widget>[
          Expanded(
            child: PageView.builder(
              controller: _panelController,
              itemCount: pages,
              onPageChanged: (page) => setState(() => _panelPage = page),
              itemBuilder: (context, page) {
                final items = _pageActions(page);
                return GridView.builder(
                  padding: const EdgeInsets.all(8),
                  physics: const NeverScrollableScrollPhysics(),
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 4,
                    mainAxisSpacing: 8,
                    crossAxisSpacing: 8,
                    mainAxisExtent: 72,
                  ),
                  itemCount: items.length,
                  itemBuilder: (context, index) => _attachButton(context, items[index]),
                );
              },
            ),
          ),
          if (pages > 1)
            SizedBox(
              height: 18,
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: List<Widget>.generate(pages, (page) {
                  final on = page == current;
                  return AnimatedContainer(
                    duration: const Duration(milliseconds: 150),
                    width: on ? 14 : 6,
                    height: 6,
                    margin: const EdgeInsets.symmetric(horizontal: 3),
                    decoration: BoxDecoration(
                      color: on ? colors.accent : colors.text3.withOpacity(0.35),
                      borderRadius: BorderRadius.circular(3),
                    ),
                  );
                }),
              ),
            ),
        ],
      ),
    );
  }

  Widget _attachButton(BuildContext context, _AttachAction action) {
    final colors = context.colors;
    return InkWell(
      borderRadius: BorderRadius.circular(14),
      onTap: action.onTap,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: <Widget>[
          Container(
            width: 46,
            height: 46,
            decoration: BoxDecoration(
              color: colors.elevated,
              borderRadius: BorderRadius.circular(14),
              border: Border.all(color: colors.sep, width: 0.5),
            ),
            child: Icon(action.icon, size: 22, color: colors.accent),
          ),
          const SizedBox(height: 4),
          LText(
            action.label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 11, color: colors.text2),
          ),
        ],
      ),
    );
  }

  Future<void> _pickAttachment({required bool media}) async {
    try {
      final result = await FilePicker.platform.pickFiles(
        type: media ? FileType.media : FileType.any,
        withData: true,
        allowMultiple: false,
      );
      final files = result?.files ?? const <PlatformFile>[];
      if (files.isEmpty) return;
      final file = files.first;
      final bytes = file.bytes;
      if (bytes == null) {
        if (mounted) setState(() => _sendError = '读取文件失败');
        return;
      }
      if (bytes.length > 5 << 20) {
        if (mounted) setState(() => _sendError = '文件超过 5 MiB，暂时不能发');
        return;
      }
      final kind = _kindForName(file.name, media: media);
      if (!mounted) return;
      setState(() {
        _pendingBytes = bytes;
        _pendingName = file.name;
        _pendingKind = kind;
        _sendError = '';
      });
      _focusInput();
    } catch (err) {
      if (mounted) setState(() => _sendError = '选择文件失败：$err');
    }
  }

  String _kindForName(String name, {required bool media}) {
    final ext = name.contains('.') ? name.split('.').last.toLowerCase() : '';
    if (media) {
      if (const <String>{'jpg', 'jpeg', 'png', 'gif', 'webp', 'heic', 'heif', 'bmp'}.contains(ext)) {
        return 'image';
      }
      if (const <String>{'mp4', 'mov', 'm4v', 'webm', 'mkv', 'avi'}.contains(ext)) {
        return 'video';
      }
      if (const <String>{'mp3', 'wav', 'silk', 'm4a', 'ogg'}.contains(ext)) {
        return 'audio';
      }
      return 'file';
    }
    if (const <String>{'mp3', 'wav', 'silk', 'm4a', 'ogg'}.contains(ext)) return 'audio';
    return 'file';
  }

  Widget _pendingPreview(BuildContext context) {
    final colors = context.colors;
    final isImage = _pendingKind == 'image';
    final label = switch (_pendingKind) {
      'image' => '图片',
      'video' => '视频',
      'audio' => '语音',
      _ => '文件',
    };
    final icon = switch (_pendingKind) {
      'image' => Icons.image_outlined,
      'video' => Icons.videocam_outlined,
      'audio' => Icons.mic_none_rounded,
      _ => Icons.insert_drive_file_outlined,
    };
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.all(6),
      decoration: BoxDecoration(
        color: colors.bg,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: <Widget>[
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: isImage && _pendingBytes != null
                ? Image.memory(_pendingBytes!, width: 42, height: 42, fit: BoxFit.cover)
                : Container(
                    width: 42,
                    height: 42,
                    color: colors.elevated,
                    child: Icon(icon, size: 20, color: colors.accent),
                  ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                LText(
                  _pendingName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: colors.text),
                ),
                LText(
                  '$label · ${_formatBytes(_pendingBytes?.length ?? 0)}',
                  style: TextStyle(fontSize: 11, color: colors.text3),
                ),
              ],
            ),
          ),
          IconAction(
            icon: Icons.close_rounded,
            size: 18,
            onTap: () => setState(() {
              _pendingBytes = null;
              _pendingName = '';
              _pendingKind = '';
            }),
          ),
        ],
      ),
    );
  }

  String _formatBytes(int n) {
    if (n < 1024) return '$n B';
    if (n < 1024 * 1024) return '${(n / 1024).toStringAsFixed(0)} KB';
    return '${(n / 1024 / 1024).toStringAsFixed(1)} MB';
  }

  Widget _replyPreview(BuildContext context) {
    final colors = context.colors;
    final msg = _replyTo!;
    final who = msg.outgoing ? '我' : (msg.senderName.isEmpty ? '对方' : msg.senderName);
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.fromLTRB(10, 6, 4, 6),
      decoration: BoxDecoration(
        color: colors.bg,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: <Widget>[
          Container(
            width: 2,
            height: 30,
            color: colors.accent,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                LText(
                  '回复 $who',
                  style: TextStyle(fontSize: 11, color: colors.text2),
                ),
                LText(
                  msg.body.plain.isEmpty ? '媒体消息' : msg.body.plain,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: colors.text3),
                ),
              ],
            ),
          ),
          IconAction(
            icon: Icons.close_rounded,
            size: 18,
            onTap: () => setState(() => _replyTo = null),
          ),
        ],
      ),
    );
  }

  Widget _mentionPreview(BuildContext context) {
    final colors = context.colors;
    return Container(
      margin: const EdgeInsets.only(bottom: 6),
      padding: const EdgeInsets.fromLTRB(10, 5, 4, 5),
      decoration: BoxDecoration(
        color: colors.bg,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        children: <Widget>[
          LText(
            '@$_mentionName',
            style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: colors.accent),
          ),
          const Spacer(),
          IconAction(
            icon: Icons.close_rounded,
            size: 18,
            onTap: () => setState(() {
              _mentioning = false;
              _mentionName = '';
            }),
          ),
        ],
      ),
    );
  }

  Future<void> _submit(AppState state, Bot? bot) async {
    final text = _input.text.trim();
    final hasMedia = _pendingBytes != null;
    if ((!hasMedia && text.isEmpty) || bot == null || _sending) return;
    final list = state.messagesFor(widget.botId, widget.peerId);
    final lastIn = _lastInbound(list);
    final canInitiate = bot.constraints.canInitiate;
    final passive = _replyTo != null || _mentioning;
    final active = canInitiate && !passive;
    final target = _replyTo?.msgId ?? lastIn?.msgId;
    if (_replyTo != null && _replyTo!.outgoing) {
      setState(() => _sendError = '只能引用对方发来的消息');
      _focusInput();
      return;
    }
    if (!active && target == null) {
      setState(() {
        _sendError = _mentioning
            ? '还没有可 @ 的消息：等对方发一条再试'
            : '还没有可回复的消息：等对方发一条';
      });
      _focusInput();
      return;
    }
    setState(() {
      _sending = true;
      _sendError = '';
    });
    _input.clear();
    try {
      if (hasMedia) {
        await state.sendMedia(
          bot: bot,
          peerId: widget.peerId,
          peerKind: lastIn?.peerKind ?? 'private',
          data: _pendingBytes!,
          name: _pendingName,
          kind: _pendingKind,
          inReplyTo: passive ? target : null,
          active: active,
        );
      } else {
        await state.send(
          bot: bot,
          peerId: widget.peerId,
          peerKind: lastIn?.peerKind ?? 'private',
          text: text,
          inReplyTo: passive ? target : null,
          active: active,
        );
      }
      if (mounted) {
        setState(() {
          _replyTo = null;
          _mentioning = false;
          _mentionName = '';
          _pendingBytes = null;
          _pendingName = '';
          _pendingKind = '';
          _panelOpen = false;
          _sending = false;
        });
      }
      _focusInput();
      _scrollToBottom();
    } on ProtocolException catch (err) {
      if (mounted) {
        setState(() {
          if (!hasMedia) {
            _input.text = text;
            _input.selection = TextSelection.collapsed(offset: text.length);
          }
          _sendError = err.friendly;
          _sending = false;
        });
      }
      _focusInput();
    } catch (err) {
      if (mounted) {
        setState(() {
          if (!hasMedia) {
            _input.text = text;
            _input.selection = TextSelection.collapsed(offset: text.length);
          }
          _sendError = '发送失败：$err';
          _sending = false;
        });
      }
      _focusInput();
    }
  }

  void _focusInput() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _inputFocus.requestFocus();
    });
  }

  Future<void> _showUserActions(ChatMessage msg) async {
    final colors = context.colors;
    final name = msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName;
    final id = msg.senderId;
    final initial = name.isEmpty ? '?' : name.substring(0, 1);
    await showLiantSheet<void>(
      context: context,
      title: '用户操作',
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colors.bg,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Row(
                children: <Widget>[
                  CircleAvatar(
                    radius: 20,
                    backgroundColor: colors.accent.withOpacity(0.12),
                    child: LText(
                      initial,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: colors.accent,
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        LText(
                          name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w600,
                            color: colors.text,
                          ),
                        ),
                        if (id.isNotEmpty) ...<Widget>[
                          const SizedBox(height: 2),
                          LText(
                            id,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11, color: colors.text3),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
            PrimaryButton(
              label: '@ 提到 $name',
              onTap: () {
                _insertMention(name);
                Navigator.of(sheetContext).pop();
              },
            ),
            const SizedBox(height: 8),
            SecondaryButton(
              label: '引用这条消息',
              onTap: () {
                setState(() {
                  _replyTo = msg;
                  _sendError = '';
                });
                Navigator.of(sheetContext).pop();
                _focusInput();
              },
            ),
            if (id.isNotEmpty) ...<Widget>[
              const SizedBox(height: 8),
              SecondaryButton(
                label: '复制用户 ID',
                onTap: () async {
                  await Clipboard.setData(ClipboardData(text: id));
                  if (sheetContext.mounted) Navigator.of(sheetContext).pop();
                },
              ),
            ],
            const SizedBox(height: 8),
            SecondaryButton(
              label: '复制昵称',
              onTap: () async {
                await Clipboard.setData(ClipboardData(text: name));
                if (sheetContext.mounted) Navigator.of(sheetContext).pop();
              },
            ),
            const SizedBox(height: 12),
          ],
        ),
      ),
    );
  }

  Future<void> _showActions(AppState state, ChatMessage msg) async {
    final colors = context.colors;
    final list = state.messagesFor(widget.botId, widget.peerId);
    final mentionName = msg.outgoing
        ? state.conversationTitle(widget.botId, widget.peerId, list)
        : (msg.senderName.isNotEmpty ? msg.senderName : shortPeer(msg.senderId));
    await showLiantSheet<void>(
      context: context,
      title: '消息操作',
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colors.bg,
                borderRadius: BorderRadius.circular(12),
              ),
              child: LText(
                msg.body.plain.isEmpty ? '媒体消息' : msg.body.plain,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: context.texts.bodySmall?.copyWith(color: colors.text2),
              ),
            ),
            const SizedBox(height: 12),
            if (!msg.outgoing) ...<Widget>[
              PrimaryButton(
                label: '引用这条',
                onTap: () {
                  setState(() {
                    _replyTo = msg;
                    _sendError = '';
                  });
                  Navigator.of(sheetContext).pop();
                  _focusInput();
                },
              ),
              const SizedBox(height: 8),
            ],
            SecondaryButton(
              label: '@ ${mentionName.isEmpty ? '对方' : mentionName}',
              onTap: () {
                _insertMention(mentionName.isEmpty ? '对方' : mentionName);
                Navigator.of(sheetContext).pop();
              },
            ),
            const SizedBox(height: 8),
            SecondaryButton(
              label: '复制文本',
              onTap: () async {
                await Clipboard.setData(ClipboardData(text: msg.body.plain));
                if (sheetContext.mounted) Navigator.of(sheetContext).pop();
              },
            ),
            const SizedBox(height: 12),
          ],
        ),
      ),
    );
  }

  void _insertMention(String name) {
    final mention = '@$name ';
    final value = _input.value;
    final selection = value.selection;
    final start = selection.isValid ? selection.start : value.text.length;
    final end = selection.isValid ? selection.end : start;
    final next = value.text.replaceRange(start, end, mention);
    _input.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: start + mention.length),
    );
    setState(() {
      _mentioning = true;
      _mentionName = name;
      _sendError = '';
    });
    _focusInput();
  }

  Future<void> _openSettings(AppState state) async {
    final key = _key;
    final list = state.messagesFor(widget.botId, widget.peerId);
    final isGroup = list.isNotEmpty && list.last.peerKind == 'group';
    final controller = TextEditingController(text: state.remarks[key] ?? '');
    var muted = state.mutedFor(key);
    await showLiantSheet<void>(
      context: context,
      title: '会话设置',
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheet) {
          final colors = context.colors;
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 14),
                  height: 52,
                  decoration: BoxDecoration(
                    color: colors.bg,
                    borderRadius: BorderRadius.circular(14),
                  ),
                  child: Row(
                    children: <Widget>[
                      LText(
                        '备注名',
                        style: context.texts.bodyLarge?.copyWith(
                          fontSize: 16,
                          color: colors.text,
                        ),
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: TextField(
                          controller: controller,
                          textAlign: TextAlign.right,
                          onChanged: (value) => state.setRemark(key, value),
                          style: TextStyle(fontSize: 16, color: colors.text),
                          decoration: InputDecoration(
                            hintText: state.conversationTitle(
                              widget.botId,
                              widget.peerId,
                              list,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              ...<Widget>[
                const SizedBox(height: 12),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Container(
                    padding: const EdgeInsets.fromLTRB(14, 8, 8, 8),
                    decoration: BoxDecoration(
                      color: colors.elevated,
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(color: colors.sep, width: 0.5),
                    ),
                    child: Row(
                      children: <Widget>[
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: <Widget>[
                              LText(
                                '消息免打扰',
                                style: context.texts.titleSmall?.copyWith(color: colors.text),
                              ),
                              const SizedBox(height: 2),
                              LText(
                                isGroup
                                    ? '群消息不再提醒，仍会正常接收。'
                                    : '这个人的消息不再提醒，仍会正常接收。',
                                style: context.texts.labelSmall?.copyWith(
                                  color: colors.text3,
                                  height: 1.5,
                                ),
                              ),
                            ],
                          ),
                        ),
                        Switch.adaptive(
                          value: muted,
                          onChanged: (value) {
                            state.setMuted(key, value);
                            setSheet(() => muted = value);
                          },
                        ),
                      ],
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 16),
            ],
          );
        },
      ),
    );
    controller.dispose();
  }

}

class _AttachAction {
  const _AttachAction({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback onTap;
}

// 聊天页：玻璃头部随滚动折叠，群聊显示发送者，支持被动回复与本地备注。
import 'dart:async';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../core/app_state.dart';
import '../core/protocol.dart';
import '../ui/format.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';

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
    final active = bot != null && state.activeFor(bot, widget.peerId);
    final topInset = MediaQuery.paddingOf(context).top;
    final headerHeight = _collapsed ? 58.0 : 96.0;

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
                128 + (_replyTo != null ? 54 : 0),
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
            child: _composer(context, state, bot, active),
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
        widgets.add(Text(
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
          child: Text(
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
    for (var i = 0; i < list.length; i++) {
      final msg = list[i];
      final prev = i > 0 ? list[i - 1] : null;
      final gap = prev == null || msg.ts - prev.ts > 5 * 60 * 1000;
      if (gap) {
        widgets.add(Padding(
          padding: const EdgeInsets.symmetric(vertical: 12),
          child: Text(
            formatMessageTime(msg.ts),
            textAlign: TextAlign.center,
            style: TextStyle(fontSize: 11, color: colors.text3),
          ),
        ));
      }
      final isGroup = msg.peerKind == 'group';
      if (isGroup && !msg.outgoing) {
        widgets.add(Padding(
          padding: const EdgeInsets.only(left: 8, bottom: 3),
          child: Text(
            msg.senderName.isEmpty ? shortPeer(msg.senderId) : msg.senderName,
            style: TextStyle(fontSize: 11, color: colors.text3),
          ),
        ));
      }
      widgets.add(Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Align(
          alignment: msg.outgoing ? Alignment.centerRight : Alignment.centerLeft,
          child: _bubble(context, state, msg, bot),
        ),
      ));
    }
    widgets.add(const SizedBox(height: 8));
    return widgets;
  }

  Widget _bubble(BuildContext context, AppState state, ChatMessage msg, Bot? bot) {
    final colors = context.colors;
    final outgoing = msg.outgoing;
    final mediaKey = '${msg.botId}/${msg.msgId}';
    final media = state.media[mediaKey];
    final maxWidth = MediaQuery.sizeOf(context).width * 0.78;

    Widget content;
    if (msg.isMedia) {
      if (media == null || media.loading) {
        content = Text(
          '正在加载媒体…',
          style: TextStyle(fontSize: 15, color: (outgoing ? Colors.white : colors.text).withOpacity(0.8)),
        );
      } else if (media.url != null) {
        content = ClipRRect(
          borderRadius: BorderRadius.circular(12),
          child: Image.network(
            media.url!,
            fit: BoxFit.cover,
            errorBuilder: (_, __, ___) => Text(
              '媒体加载失败',
              style: TextStyle(fontSize: 14, color: outgoing ? Colors.white : colors.text2),
            ),
          ),
        );
      } else {
        content = Text(
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
          if (msg.failed)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Text(
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
          if (msg.inReplyTo != null && msg.inReplyTo!.isNotEmpty && !outgoing)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                '回复了一条消息',
                style: TextStyle(fontSize: 11, color: colors.text3),
              ),
            ),
        ],
      ),
    );

    return GestureDetector(
      onTap: () {
        if (msg.failed && bot != null) {
          unawaited(state.retrySend(msg, bot));
        } else {
          unawaited(_showActions(msg));
        }
      },
      onLongPress: () => unawaited(_showActions(msg)),
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

    return SafeArea(
      bottom: false,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        height: _collapsed ? 58 : 96,
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
                          Text(
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
                        Text(
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
                          Text(
                            subtitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11, color: colors.text3),
                          ),
                          Text(
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
                      onTap: () => unawaited(_openSettings(state, bot)),
                      child: Center(
                        child: Text(
                          '备注',
                          style: TextStyle(fontSize: 15, color: colors.accent),
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
    );
  }

  Widget _composer(BuildContext context, AppState state, Bot? bot, bool active) {
    final colors = context.colors;
    final enabled = state.online && bot != null;
    final canInitiate = bot?.constraints.canInitiate ?? false;
    final modeLabel = active
        ? '主动消息：不受回复窗口限制，不带 @ 与引用'
        : '被动回复：带上 @ 与引用，受回复窗口限制';
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
                  if (_replyTo != null) _replyPreview(context),
                  if (_sendError.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 6, left: 4, right: 4),
                      child: Align(
                        alignment: Alignment.centerLeft,
                        child: Text(
                          _sendError,
                          style: TextStyle(fontSize: 12, color: colors.danger),
                        ),
                      ),
                    ),
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: <Widget>[
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
                            minLines: 1,
                            maxLines: 5,
                            keyboardType: TextInputType.multiline,
                            textInputAction: TextInputAction.newline,
                            textAlignVertical: TextAlignVertical.center,
                            style: TextStyle(fontSize: 16, height: 1.35, color: colors.text),
                            decoration: InputDecoration(
                              hintText: state.online ? '发消息…' : '连接中…',
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
                  const SizedBox(height: 4),
                  Row(
                    children: <Widget>[
                      Expanded(
                        child: Text(
                          modeLabel,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(fontSize: 11, color: colors.text3),
                        ),
                      ),
                      if (!canInitiate)
                        Text(
                          '仅被动回复',
                          style: TextStyle(fontSize: 11, color: colors.text3),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
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
                Text(
                  '回复 $who',
                  style: TextStyle(fontSize: 11, color: colors.text2),
                ),
                Text(
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

  Future<void> _submit(AppState state, Bot? bot) async {
    final text = _input.text.trim();
    if (text.isEmpty || bot == null || _sending) return;
    final list = state.messagesFor(widget.botId, widget.peerId);
    final lastIn = _lastInbound(list);
    final active = state.activeFor(bot, widget.peerId) && _replyTo == null;
    final target = _replyTo?.msgId ?? lastIn?.msgId;
    if (!active && target == null) {
      setState(() {
        _sendError = '还没有可回复的消息：等对方发一条，或在「备注」里打开主动消息';
      });
      return;
    }
    setState(() {
      _sending = true;
      _sendError = '';
    });
    _input.clear();
    try {
      await state.send(
        bot: bot,
        peerId: widget.peerId,
        peerKind: lastIn?.peerKind ?? 'private',
        text: text,
        inReplyTo: target,
        active: active,
      );
      if (mounted) {
        setState(() {
          _replyTo = null;
          _sending = false;
        });
      }
      _scrollToBottom();
    } on ProtocolException catch (err) {
      if (mounted) {
        setState(() {
          _sendError = err.friendly;
          _sending = false;
        });
      }
    } catch (err) {
      if (mounted) {
        setState(() {
          _sendError = '发送失败：$err';
          _sending = false;
        });
      }
    }
  }

  Future<void> _showActions(ChatMessage msg) async {
    final colors = context.colors;
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
              child: Text(
                msg.body.plain.isEmpty ? '媒体消息' : msg.body.plain,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: context.texts.bodySmall?.copyWith(color: colors.text2),
              ),
            ),
            const SizedBox(height: 12),
            PrimaryButton(
              label: '回复这条',
              onTap: () {
                setState(() => _replyTo = msg);
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

  Future<void> _openSettings(AppState state, Bot? bot) async {
    final key = _key;
    final controller = TextEditingController(text: state.remarks[key] ?? '');
    var active = state.activeFor(bot ?? _placeholderBot(), widget.peerId);
    final canInitiate = bot?.constraints.canInitiate ?? false;
    if (!canInitiate) active = false;

    await showLiantSheet<void>(
      context: context,
      title: '会话设置',
      footer: PrimaryButton(
        label: '保存备注',
        onTap: () {
          state.setRemark(key, controller.text);
          Navigator.of(context).pop();
        },
      ),
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
                      Text(
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
                          style: TextStyle(fontSize: 16, color: colors.text),
                          decoration: InputDecoration(
                            hintText: state.conversationTitle(
                              widget.botId,
                              widget.peerId,
                              state.messagesFor(widget.botId, widget.peerId),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 8),
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
                            Text(
                              '主动消息',
                              style: context.texts.titleSmall?.copyWith(color: colors.text),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              canInitiate
                                  ? '默认开启，不受回复窗口限制；关掉则按被动回复，带 @ 与引用。'
                                  : '该平台不支持主动发起消息，只能回复入站消息。',
                              style: context.texts.labelSmall?.copyWith(
                                color: colors.text3,
                                height: 1.5,
                              ),
                            ),
                          ],
                        ),
                      ),
                      Switch.adaptive(
                        value: active,
                        onChanged: canInitiate
                            ? (value) {
                                state.setActiveMode(key, value);
                                setSheet(() => active = value);
                              }
                            : null,
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 16),
            ],
          );
        },
      ),
    );
    controller.dispose();
  }

  Bot _placeholderBot() => Bot(
        botId: widget.botId,
        platform: 'qqbot',
        status: 'unknown',
        constraints: const BotConstraints(canInitiate: false),
      );
}

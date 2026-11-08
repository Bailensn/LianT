// 会话列表：大标题、搜索、按天分组卡片、悬浮玻璃底栏之上留白。
import 'package:flutter/material.dart';

import '../core/app_state.dart';
import '../core/client.dart';
import '../ui/format.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';
import 'chat_page.dart';
import '../l10n/ltext.dart';

class ChatsPage extends StatefulWidget {
  const ChatsPage({super.key, required this.onOpenBots});

  final VoidCallback onOpenBots;

  @override
  State<ChatsPage> createState() => _ChatsPageState();
}

class _ChatsPageState extends State<ChatsPage> {
  final TextEditingController _search = TextEditingController();
  String _query = '';

  @override
  void initState() {
    super.initState();
    _search.addListener(() {
      if (_search.text == _query) return;
      setState(() => _query = _search.text);
    });
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    final conversations = state.conversations();
    final query = _query.trim().toLowerCase();
    final filtered = query.isEmpty
        ? conversations
        : conversations
            .where((c) =>
                c.title.toLowerCase().contains(query) ||
                (c.last?.body.plain ?? '').toLowerCase().contains(query))
            .toList();

    final groups = <String, List<Conversation>>{};
    for (final conv in filtered) {
      final bucket = dayBucket(conv.last?.ts ?? DateTime.now().millisecondsSinceEpoch);
      groups.putIfAbsent(bucket, () => <Conversation>[]).add(conv);
    }

    final online = state.online;
    final running = state.bots.where((b) => b.running).length;
    final status = _connectionStatus(state);

    final children = <Widget>[
      SearchField(controller: _search, hint: '搜索会话'),
      const SizedBox(height: 16),
      if (!online)
        NoticeCard(
          title: '与服务的连接中断',
          message: state.detail.isEmpty
              ? '正在尝试重新连接，收到的新消息会在恢复后补齐。'
              : state.detail,
          actionLabel: '立即重试',
          onAction: state.reconnect,
        ),
      if (state.error.isNotEmpty)
        NoticeCard(
          title: state.error,
          danger: true,
          actionLabel: '知道了',
          onAction: state.clearError,
        ),
      if (state.bots.isEmpty && state.identity != null)
        NoticeCard(
          title: '还没有机器人',
          message: '添加一个 QQ 机器人后，收到消息就会出现在这里。',
          actionLabel: '去添加',
          onAction: widget.onOpenBots,
        ),
      if (conversations.isEmpty)
        EmptyState(
          icon: Icons.forum_outlined,
          title: '暂无会话',
          hint: state.bots.isEmpty
              ? '还没有配置机器人，先去「机器人」页面添加一个'
              : running > 0
                  ? '机器人已在线，等待第一条消息'
                  : '机器人尚未运行，去「机器人」页面启动它',
          action: state.bots.isEmpty
              ? SizedBox(
                  width: 160,
                  child: SecondaryButton(
                    label: '添加机器人',
                    onTap: widget.onOpenBots,
                  ),
                )
              : null,
        )
      else
        for (final entry in groups.entries) ...<Widget>[
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: LText(
              entry.key,
              style: context.texts.bodySmall?.copyWith(
                color: colors.text2,
                fontWeight: FontWeight.w500,
                letterSpacing: 0.2,
              ),
            ),
          ),
          GroupCard(
            margin: const EdgeInsets.only(bottom: 24),
            children: <Widget>[
              for (final conv in entry.value)
                _ConversationTile(
                  conversation: conv,
                  botName: state.botById(conv.botId)?.title,
                  onTap: () => _openChat(conv),
                ),
            ],
          ),
        ],
      const SizedBox(height: 8),
    ];

    return LargeTitlePage(
      title: '会话',
      trailing: ToneBadge(label: status.$1, tone: status.$2),
      children: children,
    );
  }

  (String, BadgeTone) _connectionStatus(AppState state) {
    return switch (state.conn) {
      ConnState.online => ('已连接', BadgeTone.ok),
      ConnState.connecting => ('连接中', BadgeTone.muted),
      ConnState.reconnecting => ('重连中', BadgeTone.warn),
      ConnState.error => ('连接失败', BadgeTone.warn),
      ConnState.idle => ('未连接', BadgeTone.muted),
    };
  }

  void _openChat(Conversation conv) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => ChatPage(botId: conv.botId, peerId: conv.peerId),
      ),
    );
  }
}

class _ConversationTile extends StatelessWidget {
  const _ConversationTile({
    required this.conversation,
    required this.botName,
    required this.onTap,
  });

  final Conversation conversation;
  final String? botName;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final last = conversation.last;
    final preview = last == null
        ? '还没有消息'
        : '${last.outgoing ? '我：' : ''}${last.body.plain.isEmpty ? '空消息' : last.body.plain}';
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 64),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            child: Row(
              children: <Widget>[
                InitialAvatar(text: conversation.title, accent: true),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Row(
                        children: <Widget>[
                          Expanded(
                            child: LText(
                              conversation.title,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: context.texts.titleMedium?.copyWith(
                                fontSize: 16,
                                color: colors.text,
                              ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          LText(
                            last == null ? '' : formatListTime(last.ts),
                            style: context.texts.labelSmall?.copyWith(color: colors.text3),
                          ),
                        ],
                      ),
                      const SizedBox(height: 3),
                      Row(
                        children: <Widget>[
                          Expanded(
                            child: LText(
                              preview,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: context.texts.bodySmall?.copyWith(color: colors.text2),
                            ),
                          ),
                          if (conversation.unread > 0) ...<Widget>[
                            const SizedBox(width: 8),
                            Container(
                              height: 18,
                              constraints: const BoxConstraints(minWidth: 18),
                              padding: const EdgeInsets.symmetric(horizontal: 5),
                              alignment: Alignment.center,
                              decoration: BoxDecoration(
                                color: colors.accent,
                                borderRadius: BorderRadius.circular(9),
                              ),
                              child: LText(
                                conversation.unread > 99 ? '99+' : '${conversation.unread}',
                                style: const TextStyle(
                                  fontSize: 11,
                                  fontWeight: FontWeight.w600,
                                  color: Colors.white,
                                ),
                              ),
                            ),
                          ],
                        ],
                      ),
                      if (botName != null && (botName?.isNotEmpty ?? false))
                        Padding(
                          padding: const EdgeInsets.only(top: 2),
                          child: LText(
                            botName!,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(fontSize: 11, color: colors.text3),
                          ),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

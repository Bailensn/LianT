// 机器人页：列表、状态、启停、添加与移除。
import 'package:flutter/material.dart';

import '../core/protocol.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';

class BotsPage extends StatefulWidget {
  const BotsPage({super.key});

  @override
  State<BotsPage> createState() => _BotsPageState();
}

class _BotsPageState extends State<BotsPage> {
  bool _busy = false;
  String _pendingId = '';
  String _error = '';

  Future<void> _act(String botId, Future<void> Function(String) action) async {
    setState(() {
      _pendingId = botId;
      _error = '';
    });
    try {
      await action(botId);
    } on ProtocolException catch (err) {
      if (mounted) setState(() => _error = err.friendly);
    } catch (err) {
      if (mounted) setState(() => _error = '操作失败：$err');
    } finally {
      if (mounted) setState(() => _pendingId = '');
    }
  }

  Future<void> _openAddSheet() async {
    final state = AppScope.of(context);
    final appId = TextEditingController();
    final secret = TextEditingController();
    final label = TextEditingController();
    final colors = context.colors;
    _busy = false;
    _error = '';

    await showLiantSheet<void>(
      context: context,
      title: '添加 QQ 机器人',
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheet) {
          Future<void> submit() async {
            if (appId.text.trim().isEmpty || secret.text.trim().isEmpty) {
              setSheet(() => _error = 'AppID 与 AppSecret 都要填');
              return;
            }
            setSheet(() {
              _busy = true;
              _error = '';
            });
            try {
              await state.addBot(
                platform: 'qqbot',
                credentials: <String, String>{
                  'app_id': appId.text.trim(),
                  'app_secret': secret.text.trim(),
                },
                label: label.text,
              );
              if (sheetContext.mounted) Navigator.of(sheetContext).pop();
            } on ProtocolException catch (err) {
              setSheet(() => _error = err.friendly);
            } catch (err) {
              setSheet(() => _error = '保存失败：$err');
            } finally {
              setSheet(() => _busy = false);
            }
          }

          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Row(
                  children: <Widget>[
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: colors.accent.withOpacity(0.12),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        'QQ 机器人',
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                          color: colors.accent,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: colors.sunken,
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        'Telegram · 即将支持',
                        style: TextStyle(fontSize: 13, color: colors.text3),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(LTokens.cardRadius),
                  child: ColoredBox(
                    color: colors.bg,
                    child: Column(
                      children: <Widget>[
                        FieldRow(
                          label: 'AppID',
                          controller: appId,
                          hint: '1020xxxxx',
                          keyboardType: TextInputType.number,
                          monospace: true,
                        ),
                        const Hairline(indent: 16),
                        FieldRow(
                          label: 'AppSecret',
                          controller: secret,
                          hint: '平台密钥',
                          obscure: true,
                          monospace: true,
                        ),
                        const Hairline(indent: 16),
                        FieldRow(
                          label: '备注名',
                          controller: label,
                          hint: '可留空',
                        ),
                      ],
                    ),
                  ),
                ),
              ),
              if (_error.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
                  child: Text(
                    _error,
                    style: TextStyle(fontSize: 13, color: colors.danger, height: 1.5),
                  ),
                ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                child: PrimaryButton(
                  label: _busy ? '验证中…' : '保存并启动',
                  busy: _busy,
                  onTap: _busy ? null : submit,
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
                child: Text(
                  '凭据在 QQ 开放平台「机器人 → 开发设置」里；保存时会实时校验并读取昵称。AppSecret 只存在服务端，加密入库，客户端不会再读出来。',
                  style: context.texts.labelSmall?.copyWith(
                    color: colors.text3,
                    height: 1.6,
                  ),
                ),
              ),
            ],
          );
        },
      ),
    );

    appId.dispose();
    secret.dispose();
    label.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    final running = state.bots.where((b) => b.running).length;

    final children = <Widget>[
      if (!state.online)
        NoticeCard(
          title: '与服务的连接中断',
          message: state.detail.isEmpty ? '连接恢复后才能启停机器人。' : state.detail,
          actionLabel: '立即重试',
          onAction: state.reconnect,
        ),
      if (_error.isNotEmpty)
        NoticeCard(
          title: _error,
          danger: true,
          actionLabel: '知道了',
          onAction: () => setState(() => _error = ''),
        ),
      if (state.error.isNotEmpty)
        NoticeCard(
          title: state.error,
          danger: true,
          actionLabel: '知道了',
          onAction: state.clearError,
        ),
      if (state.bots.isEmpty)
        EmptyState(
          icon: Icons.smart_toy_outlined,
          title: '还没有机器人',
          hint: '点右上角「添加」，填入 QQ 开放平台的 AppID 与 AppSecret',
          action: SizedBox(
            width: 160,
            child: SecondaryButton(
              label: '添加机器人',
              onTap: _openAddSheet,
            ),
          ),
        )
      else
        GroupCard(
          title: '已配置',
          footer: '状态由服务端推送；停止后不再接收新消息，历史仍保留。',
          children: <Widget>[
            for (final bot in state.bots)
              _BotRow(
                bot: bot,
                busy: _pendingId == bot.botId,
                online: state.online,
                onToggle: () => _act(
                  bot.botId,
                  bot.running ? state.stopBot : state.startBot,
                ),
                onRemove: () async {
                  final ok = await confirmAction(
                    context,
                    title: '移除 ${bot.title}？',
                    message: '移除后服务端不再保留这个机器人的凭据，历史消息仍按保留期存在。',
                    confirmLabel: '移除',
                    danger: true,
                  );
                  if (!ok) return;
                  await _act(bot.botId, state.removeBot);
                },
              ),
          ],
        ),
      if (running > 0)
        Padding(
          padding: const EdgeInsets.only(left: 16),
          child: Text(
            '$running 个机器人正在运行',
            style: context.texts.labelSmall?.copyWith(color: colors.text3),
          ),
        ),
      const SizedBox(height: 8),
    ];

    return LargeTitlePage(
      title: '机器人',
      trailing: _AddChip(onTap: _openAddSheet),
      children: children,
    );
  }
}

class _AddChip extends StatelessWidget {
  const _AddChip({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Material(
      color: colors.elevated,
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        borderRadius: BorderRadius.circular(20),
        onTap: onTap,
        child: SizedBox(
          height: 36,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                Icon(Icons.add_rounded, size: 17, color: colors.accent),
                const SizedBox(width: 2),
                Text(
                  '添加',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: colors.accent,
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

class _BotRow extends StatelessWidget {
  const _BotRow({
    required this.bot,
    required this.busy,
    required this.online,
    required this.onToggle,
    required this.onRemove,
  });

  final Bot bot;
  final bool busy;
  final bool online;
  final VoidCallback onToggle;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final statusLabel = bot.running
        ? '运行中'
        : bot.status == 'error'
            ? '异常'
            : '已停止';
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 12, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              InitialAvatar(text: bot.title, accent: true),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Row(
                      children: <Widget>[
                        Flexible(
                          child: Text(
                            bot.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: context.texts.titleMedium?.copyWith(
                              fontSize: 16,
                              color: colors.text,
                            ),
                          ),
                        ),
                        const SizedBox(width: 6),
                        ToneBadge(
                          label: statusLabel,
                          tone: bot.running
                              ? BadgeTone.ok
                              : bot.status == 'error'
                                  ? BadgeTone.warn
                                  : BadgeTone.muted,
                        ),
                      ],
                    ),
                    const SizedBox(height: 2),
                    Text(
                      bot.botId,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        fontFamily: 'monospace',
                        color: colors.text3,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 8),
              if (busy)
                const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16),
                  child: SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                )
              else
                SizedBox(
                  height: 40,
                  child: FilledButton(
                    onPressed: online ? onToggle : null,
                    style: FilledButton.styleFrom(
                      backgroundColor:
                          bot.running ? colors.sunken : colors.accent,
                      foregroundColor:
                          bot.running ? colors.text : Colors.white,
                      disabledBackgroundColor: colors.sunken,
                      disabledForegroundColor: colors.text3,
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      minimumSize: const Size(64, 40),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(20),
                      ),
                    ),
                    child: Text(
                      bot.running ? '停止' : '启动',
                      style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          Padding(
            padding: const EdgeInsets.only(left: 56),
            child: Row(
              children: <Widget>[
                SizedBox(
                  height: 40,
                  child: TextButton(
                    onPressed: onRemove,
                    style: TextButton.styleFrom(
                      foregroundColor: colors.danger,
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      minimumSize: const Size(44, 40),
                    ),
                    child: const Text('移除', style: TextStyle(fontSize: 12)),
                  ),
                ),
                const SizedBox(width: 8),
                if (bot.caps.isNotEmpty)
                  Expanded(
                    child: Text(
                      '支持 ${bot.caps.take(3).join(' · ')}',
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
    );
  }
}

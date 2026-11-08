// 设置页：连接信息、已配对设备、安全日志、解除配对。
import 'package:flutter/material.dart';

import '../core/client.dart';
import '../core/protocol.dart';
import '../ui/format.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';
import '../l10n/l10n.dart';
import 'language_page.dart';
import 'security_log_page.dart';
import 'update_page.dart';
import '../l10n/ltext.dart';



class SettingsPage extends StatelessWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final identity = state.identity;
    final colors = context.colors;
    if (identity == null) return const SizedBox.shrink();
    final online = state.online;
    final running = state.bots.where((b) => b.running).length;

    final children = <Widget>[
      GroupCard(
        title: '服务',
        children: <Widget>[
          _SettingsRow(
            title: identity.serviceName.isEmpty ? 'LianT Service' : identity.serviceName,
            subtitle: '${identity.host}:${identity.port}',
            trailing: ToneBadge(
              label: online
                  ? '在线'
                  : state.conn == ConnState.reconnecting
                      ? '重连中'
                      : '离线',
              tone: online
                  ? BadgeTone.ok
                  : state.conn == ConnState.reconnecting
                      ? BadgeTone.warn
                      : BadgeTone.muted,
            ),
          ),
          if (!online)
            _SettingsRow(
              title: '重新连接',
              subtitle: '断开时点这里立刻重试',
              onTap: state.reconnect,
            ),
          _SettingsRow(
            title: '服务指纹',
            subtitle: identity.fingerprint.isEmpty ? '未知' : identity.fingerprint,
            selectable: true,
          ),
          _SettingsRow(
            title: '本机设备标识',
            subtitle: identity.deviceId,
            selectable: true,
            monospace: true,
          ),
          if (state.detail.isNotEmpty)
            _SettingsRow(title: '最近状态', subtitle: state.detail),
        ],
      ),
      GroupCard(
        title: '已配对设备',
        footer: '吊销后该设备立刻失去连接权限，需要在它上面重新配对。',
        children: <Widget>[
          if (state.devices.isEmpty)
            _SettingsRow(
              title: '暂无设备',
              subtitle: online ? '列表为空' : '连接后才能读取',
            )
          else
            for (final device in state.devices)
              _SettingsRow(
                title: device.name.isEmpty ? '未命名设备' : device.name,
                subtitle:
                    '最近活动 ${formatDateTime(device.lastSeenAt)} · ${device.deviceId}',
                trailing: device.deviceId == identity.deviceId
                    ? const ToneBadge(label: '本机', tone: BadgeTone.ok)
                    : SizedBox(
                        height: 36,
                        child: TextButton(
                          onPressed: () async {
                            final ok = await confirmAction(
                              context,
                              title: '吊销「${device.name.isEmpty ? device.deviceId : device.name}」？',
                              message: '吊销后这台设备需要重新配对才能连接。',
                              confirmLabel: '吊销',
                              danger: true,
                            );
                            if (!ok) return;
                            try {
                              await state.revokeDevice(device.deviceId);
                            } on ProtocolException catch (err) {
                              if (context.mounted) {
                                ScaffoldMessenger.of(context)
                                    .showSnackBar(SnackBar(content: LText(err.friendly)));
                              }
                            }
                          },
                          style: TextButton.styleFrom(
                            foregroundColor: colors.danger,
                            minimumSize: const Size(44, 36),
                            padding: const EdgeInsets.symmetric(horizontal: 10),
                          ),
                          child: const LText('吊销', style: TextStyle(fontSize: 13)),
                        ),
                      ),
              ),
        ],
      ),
      GroupCard(
        title: '机器人',
        footer: '多机器人同时在线，事件按 bot_id 区分；通知与提醒由你自己决定看哪一个。',
        children: <Widget>[
          _SettingsRow(
            title: '已配置',
            subtitle: '${state.bots.length} 个，运行中 $running 个',
          ),
          _SettingsRow(
            title: '服务端能力',
            subtitle: state.caps.isEmpty ? '未握手' : state.caps.join(' · '),
          ),
        ],
      ),
      GroupCard(
        title: '安全',
        footer: '私钥从不离开本机；服务端只保存公钥，撤销设备即删除公钥。',
        children: <Widget>[
          _SettingsRow(
            title: '安全日志',
            subtitle: '最近 20 条登录、配对、吊销记录',
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const SecurityLogPage()),
            ),
          ),
          const _SettingsRow(
            title: '设备密钥',
            subtitle: 'P-256 私钥在本机生成，仅存设备本地，不随消息上传。',
          ),
          _SettingsRow(
            title: '解除配对',
            subtitle: '清除本机身份，需要重新配对才能连接',
            danger: true,
            onTap: () async {
              final ok = await confirmAction(
                context,
                title: '解除配对？',
                message: '清除后需要重新扫码或输入配对码才能连回这台服务。',
                confirmLabel: '解除',
                danger: true,
              );
              if (ok) await state.unpair();
            },
          ),
        ],
      ),
      GroupCard(
        title: '通用',
        children: <Widget>[
          _SettingsRow(
            title: '语言',
            subtitle: state.languageSetting == 'system'
                ? L10n.label(L10n.code)
                : L10n.label(state.languageSetting),
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const LanguagePage()),
            ),
          ),
        ],
      ),
      GroupCard(
        title: '更新',
        children: <Widget>[
          _SettingsRow(
            title: '检查更新',
            subtitle: 'Manager 与 Service',
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const UpdatePage()),
            ),
          ),
        ],
      ),
      const GroupCard(
        title: '关于',
        children: <Widget>[
          _SettingsRow(title: '协议', subtitle: 'v1 · 端到端签名握手'),
          _SettingsRow(
            title: '聊天记录',
            subtitle: '不落盘：打开会话时按需从服务端拉取最近 100 条',
          ),
          _SettingsRow(title: '客户端', subtitle: 'Flutter · 与网页端同协议'),
        ],
      ),
      const SizedBox(height: 8),
    ];

    return LargeTitlePage(
      title: '设置',
      trailing: IconAction(
        icon: Icons.refresh_rounded,
        tooltip: '刷新设备与日志',
        size: 20,
        onTap: () async {
          try {
            await state.refreshBots();
            await state.loadDevices();
            await state.loadAudit();
          } on ProtocolException catch (err) {
            if (context.mounted) {
              ScaffoldMessenger.of(context)
                  .showSnackBar(SnackBar(content: LText(err.friendly)));
            }
          }
        },
      ),
      children: children,
    );
  }
}

class _SettingsRow extends StatelessWidget {
  const _SettingsRow({
    required this.title,
    this.subtitle,
    this.trailing,
    this.onTap,
    this.danger = false,
    this.selectable = false,
    this.monospace = false,
  });

  final String title;
  final String? subtitle;
  final Widget? trailing;
  final VoidCallback? onTap;
  final bool danger;
  final bool selectable;
  final bool monospace;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final subtitleText = subtitle ?? '';
    final content = Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                LText(
                  title,
                  style: context.texts.bodyLarge?.copyWith(
                    fontSize: 16,
                    color: danger ? colors.danger : colors.text,
                  ),
                ),
                if (subtitleText.isNotEmpty) ...<Widget>[
                  const SizedBox(height: 3),
                  selectable
                      ? SelectableText(
                          subtitleText,
                          style: TextStyle(
                            fontSize: 12,
                            height: 1.5,
                            color: danger ? colors.danger : colors.text3,
                            fontFamily: monospace ? 'monospace' : null,
                          ),
                        )
                      : LText(
                          subtitleText,
                          style: TextStyle(
                            fontSize: 12,
                            height: 1.5,
                            color: danger ? colors.danger : colors.text3,
                            fontFamily: monospace ? 'monospace' : null,
                          ),
                        ),
                ],
              ],
            ),
          ),
          if (trailing != null) ...<Widget>[
            const SizedBox(width: 12),
            trailing!,
          ],
        ],
      ),
    );
    if (onTap == null) return content;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 52),
          child: content,
        ),
      ),
    );
  }
}

import 'dart:async';

import 'package:flutter/material.dart';

import '../core/protocol.dart';
import '../l10n/ltext.dart';
import '../ui/format.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';

const Map<String, String> _auditKindText = <String, String>{
  'auth.ok': '登录成功',
  'auth.fail': '登录失败',
  'pair.ok': '配对成功',
  'pair.denied': '配对被拒',
  'device.revoke': '设备吊销',
};

class SecurityLogPage extends StatefulWidget {
  const SecurityLogPage({super.key});

  @override
  State<SecurityLogPage> createState() => _SecurityLogPageState();
}

class _SecurityLogPageState extends State<SecurityLogPage> {
  bool _booted = false;
  bool _loading = false;
  String _error = '';

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_booted) return;
    _booted = true;
    unawaited(_load());
  }

  Future<void> _load() async {
    if (_loading) return;
    setState(() {
      _loading = true;
      _error = '';
    });
    try {
      await AppScope.of(context).loadAudit();
    } on ProtocolException catch (err) {
      if (mounted) setState(() => _error = err.friendly);
    } catch (err) {
      if (mounted) setState(() => _error = '$err');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    return DetailScaffold(
      title: '安全日志',
      trailing: IconAction(
        icon: Icons.refresh_rounded,
        tooltip: '刷新',
        size: 20,
        onTap: _loading ? null : () => unawaited(_load()),
      ),
      children: <Widget>[
        if (_error.isNotEmpty)
          NoticeCard(
            title: '读取失败',
            message: _error,
            actionLabel: '重试',
            onAction: () => unawaited(_load()),
          ),
        if (_loading && state.audit.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 48),
            child: Center(
              child: SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
          )
        else if (state.audit.isEmpty)
          const EmptyState(
            icon: Icons.shield_outlined,
            title: '暂无记录',
            hint: '登录、配对与吊销都会写进这里',
          )
        else
          GroupCard(
            footer: '最近 20 条，成功与失败都会记录来源 IP。',
            children: <Widget>[
              for (final entry in state.audit)
                _AuditRow(
                  title: _auditKindText[entry.kind] ?? entry.kind,
                  subtitle: <String>[
                    if (entry.name.isNotEmpty) entry.name,
                    if (entry.reason.isNotEmpty) entry.reason,
                    if (entry.ip.isNotEmpty) entry.ip,
                  ].join(' · '),
                  time: formatDateTime(entry.ts),
                  color: entry.kind == 'auth.fail' || entry.kind == 'pair.denied'
                      ? colors.danger
                      : colors.text2,
                ),
            ],
          ),
      ],
    );
  }
}

class _AuditRow extends StatelessWidget {
  const _AuditRow({
    required this.title,
    required this.subtitle,
    required this.time,
    required this.color,
  });

  final String title;
  final String subtitle;
  final String time;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                LText(
                  title,
                  style: context.texts.bodyLarge?.copyWith(color: color),
                ),
                if (subtitle.isNotEmpty) ...<Widget>[
                  const SizedBox(height: 2),
                  LText(
                    subtitle,
                    style: context.texts.labelSmall?.copyWith(color: colors.text3),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(width: 12),
          LText(
            time,
            style: TextStyle(fontSize: 12, color: colors.text3),
          ),
        ],
      ),
    );
  }
}

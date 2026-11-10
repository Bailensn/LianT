import 'package:flutter/material.dart';

import '../core/background/service.dart';
import '../core/platform/update_service.dart';
import '../core/platform/version.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';
import '../l10n/ltext.dart';

class UpdatePage extends StatefulWidget {
  const UpdatePage({super.key});

  @override
  State<UpdatePage> createState() => _UpdatePageState();
}

class _UpdatePageState extends State<UpdatePage> {
  bool _managerBusy = false;
  bool _serviceBusy = false;

  bool get _checking => _managerBusy || _serviceBusy;

  Future<void> _check({required bool manager}) async {
    if (_checking) return;
    final state = AppScope.of(context);
    final serviceVersion = state.serviceVersion;
    final beta = state.acceptBeta;
    setState(() {
      if (manager) {
        _managerBusy = true;
      } else {
        _serviceBusy = true;
      }
    });
    try {
      final releases = await fetchReleases();
      final current = manager ? managerVersion : serviceVersion;
      ReleaseInfo? latest;
      if (manager) {
        for (final release in releases) {
          if (!release.tag.startsWith('manager-v')) continue;
          if (!beta && release.prerelease) continue;
          if (latest == null ||
              compareVersions(
                    release.tag.substring('manager-v'.length),
                    latest.tag.substring('manager-v'.length),
                  ) >
                  0) {
            latest = release;
          }
        }
        final latestVersion = latest == null
            ? current
            : latest.tag.substring('manager-v'.length);
        final isNew = latest != null && compareVersions(latestVersion, current) > 0;
        if (!mounted) return;
        await _showResult(
          isNew: isNew,
          latest: latest,
          current: current,
          label: 'Manager',
        );
      } else {
        for (final release in releases) {
          if (!release.tag.startsWith('v') ||
              release.tag.startsWith('manager-v')) {
            continue;
          }
          if (!beta && release.prerelease) continue;
          if (latest == null ||
              release.publishedAt.isAfter(latest.publishedAt)) {
            latest = release;
          }
        }
        final isNew =
            latest != null && (current.isEmpty || latest.tag != current);
        if (!mounted) return;
        await _showResult(
          isNew: isNew,
          latest: latest,
          current: current.isEmpty ? '未知' : current,
          label: 'Service',
        );
      }
    } catch (err) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: LText('检查更新失败：$err')),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          if (manager) {
            _managerBusy = false;
          } else {
            _serviceBusy = false;
          }
        });
      }
    }
  }

  Future<void> _showResult({
    required bool isNew,
    required ReleaseInfo? latest,
    required String current,
    required String label,
  }) async {
    await showLiantSheet<void>(
      context: context,
      title: '$label 更新',
      builder: (sheetContext) => Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            LText(
              isNew ? '发现新版本：${latest?.tag ?? ''}' : '当前已是最新版',
              style: context.texts.titleMedium,
            ),
            const SizedBox(height: 6),
            LText(
              '当前版本：$current',
              style: context.texts.bodySmall?.copyWith(color: context.colors.text2),
            ),
            const SizedBox(height: 16),
            if (isNew)
              Row(
                children: <Widget>[
                  Expanded(
                    child: SecondaryButton(
                      label: '取消',
                      onTap: () => Navigator.of(sheetContext).pop(),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: PrimaryButton(
                      label: '前往更新',
                      onTap: () {
                        Navigator.of(sheetContext).pop();
                        if (latest != null) openExternalUrl(latest.url);
                      },
                    ),
                  ),
                ],
              )
            else
              PrimaryButton(
                label: '确定',
                onTap: () => Navigator.of(sheetContext).pop(),
              ),
            const SizedBox(height: 12),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final serviceCurrent = state.serviceVersion.isEmpty
        ? '未握手'
        : state.serviceVersion;
    return DetailScaffold(
      title: '检查更新',
      children: <Widget>[
        GroupCard(
          title: '更新选项',
          children: <Widget>[
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 12, 8),
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        LText(
                          '是否接收测试版更新',
                          style: context.texts.titleSmall,
                        ),
                        const SizedBox(height: 2),
                        LText(
                          '默认关闭；开启后会检查 pre-release。',
                          style: context.texts.labelSmall?.copyWith(
                            color: context.colors.text3,
                            height: 1.5,
                          ),
                        ),
                      ],
                    ),
                  ),
                  Switch.adaptive(
                    value: state.acceptBeta,
                    onChanged: _checking
                        ? null
                        : (value) {
                            state.setAcceptBeta(value);
                            setState(() {});
                          },
                  ),
                ],
              ),
            ),
          ],
        ),
        GroupCard(
          title: '检查',
          children: <Widget>[
            _UpdateRow(
              title: '检查 Manager',
              subtitle: '当前版本 $managerVersion',
              busy: _managerBusy,
              onTap: _checking ? null : () => _check(manager: true),
            ),
            _UpdateRow(
              title: '检查 Service',
              subtitle: '当前版本 $serviceCurrent',
              busy: _serviceBusy,
              onTap: _checking ? null : () => _check(manager: false),
            ),
          ],
        ),
      ],
    );
  }
}

class _UpdateRow extends StatelessWidget {
  const _UpdateRow({
    required this.title,
    required this.subtitle,
    required this.busy,
    this.onTap,
  });

  final String title;
  final String subtitle;
  final bool busy;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
        child: Row(
          children: <Widget>[
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  LText(title, style: context.texts.bodyLarge),
                  const SizedBox(height: 2),
                  LText(
                    subtitle,
                    style: context.texts.labelSmall?.copyWith(color: colors.text3),
                  ),
                ],
              ),
            ),
            if (busy)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Icon(Icons.chevron_right_rounded, color: colors.text3),
          ],
        ),
      ),
    );
  }
}

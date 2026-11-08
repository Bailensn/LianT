// 共用组件：大标题页、分组卡片、玻璃底栏、气泡、抽屉、按钮。
import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import '../core/app_state.dart';
import '../core/protocol.dart';
import 'tokens.dart';
import '../l10n/l10n.dart';
import '../l10n/ltext.dart';

/// 把 AppState 挂到树上；`AppScope.of(context)` 拿状态并自动重建。
class AppScope extends InheritedNotifier<AppState> {
  const AppScope({super.key, required AppState state, required super.child})
      : super(notifier: state);

  static AppState of(BuildContext context) {
    final scope = context.dependOnInheritedWidgetOfExactType<AppScope>();
    assert(scope != null, 'AppScope 没有挂在树上');
    return scope!.notifier!;
  }
}

/// 1px（逻辑像素 0.5）半透明分隔线。
class Hairline extends StatelessWidget {
  const Hairline({super.key, this.indent = 0, this.endIndent = 0});

  final double indent;
  final double endIndent;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 0.5,
      color: context.colors.sep,
      margin: EdgeInsets.only(left: indent, right: endIndent),
    );
  }
}

/// 分组卡片：可选标题与脚注，子项之间自动插分隔线。
class GroupCard extends StatelessWidget {
  const GroupCard({
    super.key,
    this.title,
    this.footer,
    this.dividerIndent = 68,
    this.margin = const EdgeInsets.only(bottom: 24),
    required this.children,
  });

  final String? title;
  final String? footer;
  final double dividerIndent;
  final EdgeInsets margin;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final rows = <Widget>[];
    for (var i = 0; i < children.length; i++) {
      if (i > 0) rows.add(Hairline(indent: dividerIndent));
      rows.add(children[i]);
    }
    return Padding(
      padding: margin,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          if (title != null)
            Padding(
              padding: const EdgeInsets.only(left: 16, right: 16, bottom: 8),
              child: LText(
                title!,
                style: context.texts.bodySmall?.copyWith(
                  color: colors.text2,
                  fontWeight: FontWeight.w500,
                  letterSpacing: 0.2,
                ),
              ),
            ),
          ClipRRect(
            borderRadius: BorderRadius.circular(LTokens.cardRadius),
            child: ColoredBox(
              color: colors.elevated,
              child: Column(children: rows),
            ),
          ),
          if (footer != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
              child: LText(
                footer!,
                style: context.texts.labelSmall?.copyWith(
                  color: colors.text3,
                  height: 1.5,
                ),
              ),
            ),
        ],
      ),
    );
  }
}

enum BadgeTone { ok, warn, muted, accent }

class ToneBadge extends StatelessWidget {
  const ToneBadge({super.key, required this.label, this.tone = BadgeTone.muted});

  final String label;
  final BadgeTone tone;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final color = switch (tone) {
      BadgeTone.ok => colors.ok,
      BadgeTone.warn => colors.danger,
      BadgeTone.accent => colors.accent,
      BadgeTone.muted => colors.text2,
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
      decoration: BoxDecoration(
        color: color.withOpacity(0.12),
        borderRadius: BorderRadius.circular(20),
      ),
      child: LText(
        label,
        style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: color),
      ),
    );
  }
}

/// 主 CTA：全宽、48 高、圆角 16。
class PrimaryButton extends StatelessWidget {
  const PrimaryButton({
    super.key,
    required this.label,
    this.onTap,
    this.busy = false,
  });

  final String label;
  final VoidCallback? onTap;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final enabled = onTap != null && !busy;
    return Material(
      color: enabled ? colors.accent : colors.sunken,
      borderRadius: BorderRadius.circular(LTokens.controlRadius),
      child: InkWell(
        borderRadius: BorderRadius.circular(LTokens.controlRadius),
        onTap: enabled ? onTap : null,
        child: SizedBox(
          height: 48,
          width: double.infinity,
          child: Center(
            child: busy
                ? SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: enabled ? Colors.white : colors.text3,
                    ),
                  )
                : LText(
                    label,
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: enabled ? Colors.white : colors.text3,
                    ),
                  ),
          ),
        ),
      ),
    );
  }
}

/// 次级按钮：默认 44 高。
class SecondaryButton extends StatelessWidget {
  const SecondaryButton({
    super.key,
    required this.label,
    this.onTap,
    this.height = 44,
  });

  final String label;
  final VoidCallback? onTap;
  final double height;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final enabled = onTap != null;
    return Material(
      color: colors.sunken,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        borderRadius: BorderRadius.circular(14),
        onTap: enabled ? onTap : null,
        child: SizedBox(
          height: height,
          width: double.infinity,
          child: Center(
            child: LText(
              label,
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w500,
                color: enabled ? colors.text : colors.text3,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 44×44 图标按钮，触控热区不缩水。
class IconAction extends StatelessWidget {
  const IconAction({
    super.key,
    required this.icon,
    required this.onTap,
    this.color,
    this.tooltip,
    this.size = 22,
  });

  final IconData icon;
  final VoidCallback? onTap;
  final Color? color;
  final String? tooltip;
  final double size;

  @override
  Widget build(BuildContext context) {
    final button = SizedBox(
      width: LTokens.touchTarget,
      height: LTokens.touchTarget,
      child: IconButton(
        padding: EdgeInsets.zero,
        constraints: const BoxConstraints(
          minWidth: LTokens.touchTarget,
          minHeight: LTokens.touchTarget,
        ),
        splashRadius: 22,
        iconSize: size,
        tooltip: tooltip,
        onPressed: onTap,
        icon: Icon(icon, color: color ?? context.colors.text2),
      ),
    );
    return button;
  }
}

/// 空态：标题 + 一句可执行的提示。
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.hint,
    this.action,
  });

  final IconData icon;
  final String title;
  final String hint;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 56),
      child: Column(
        children: <Widget>[
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              color: colors.sunken,
              borderRadius: BorderRadius.circular(18),
            ),
            child: Icon(icon, color: colors.text3, size: 26),
          ),
          const SizedBox(height: 16),
          LText(
            title,
            textAlign: TextAlign.center,
            style: context.texts.titleMedium?.copyWith(color: colors.text),
          ),
          const SizedBox(height: 6),
          LText(
            hint,
            textAlign: TextAlign.center,
            style: context.texts.bodySmall?.copyWith(color: colors.text3, height: 1.6),
          ),
          if (action != null) ...<Widget>[const SizedBox(height: 20), action!],
        ],
      ),
    );
  }
}

/// 搜索框：44 高，背景是卡片色而不是描边。
class SearchField extends StatelessWidget {
  const SearchField({
    super.key,
    required this.controller,
    this.hint = '搜索会话',
    this.onChanged,
  });

  final TextEditingController controller;
  final String hint;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Container(
      height: 44,
      padding: const EdgeInsets.symmetric(horizontal: 12),
      decoration: BoxDecoration(
        color: colors.elevated,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: <Widget>[
          Icon(Icons.search, size: 18, color: colors.text3),
          const SizedBox(width: 8),
          Expanded(
            child: TextField(
              controller: controller,
              onChanged: onChanged,
              textInputAction: TextInputAction.search,
              textAlignVertical: TextAlignVertical.center,
              cursorColor: colors.accent,
              style: TextStyle(fontSize: 15, color: colors.text),
              decoration: InputDecoration(
                hintText: L10n.tr(hint),
                contentPadding: EdgeInsets.zero,
              ),
            ),
          ),
          if (controller.text.isNotEmpty)
            GestureDetector(
              onTap: () {
                controller.clear();
                onChanged?.call('');
              },
              child: Icon(Icons.cancel, size: 16, color: colors.text3),
            ),
        ],
      ),
    );
  }
}

/// 顶部 34px 大标题，随滚动收缩成 20px 并固定在顶部。
class LargeTitlePage extends StatelessWidget {
  const LargeTitlePage({
    super.key,
    required this.title,
    this.trailing,
    required this.children,
    this.padding = const EdgeInsets.fromLTRB(16, 0, 16, 130),
  });

  final String title;
  final Widget? trailing;
  final List<Widget> children;
  final EdgeInsets padding;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return CustomScrollView(
      slivers: <Widget>[
        SliverPersistentHeader(
          pinned: true,
          delegate: _LargeTitleDelegate(
            title: title,
            trailing: trailing,
            colors: colors,
            hasTrailing: trailing != null,
            rebuildKey: title,
          ),
        ),
        SliverPadding(
          padding: padding,
          sliver: SliverList(
            delegate: SliverChildListDelegate(children),
          ),
        ),
      ],
    );
  }
}

class _LargeTitleDelegate extends SliverPersistentHeaderDelegate {
  _LargeTitleDelegate({
    required this.title,
    required this.trailing,
    required this.colors,
    required this.hasTrailing,
    required this.rebuildKey,
  });

  final String title;
  final Widget? trailing;
  final LColors colors;
  final bool hasTrailing;
  final String rebuildKey;

  double get _maxExtent => 104;
  double get _minExtent => 58;

  @override
  double get maxExtent => _maxExtent;

  @override
  double get minExtent => _minExtent;

  @override
  Widget build(BuildContext context, double shrinkOffset, bool overlapsContent) {
    final t = (shrinkOffset / (maxExtent - minExtent)).clamp(0.0, 1.0);
    final fontSize = LTokens.largeTitle + (LTokens.compactTitle - LTokens.largeTitle) * t;
    return Container(
      color: colors.bg,
      child: Stack(
        children: <Widget>[
          Positioned(
            left: 16,
            right: 16,
            bottom: 8,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: <Widget>[
                Expanded(
                  child: LText(
                    title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: fontSize,
                      fontWeight: FontWeight.w700,
                      letterSpacing: -0.8 + 0.6 * t,
                      height: 1.15,
                      color: colors.text,
                    ),
                  ),
                ),
                if (hasTrailing && trailing != null)
                  Padding(
                    padding: const EdgeInsets.only(left: 12),
                    child: trailing,
                  ),
              ],
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: Opacity(
              opacity: t,
              child: Container(height: 0.5, color: colors.sep),
            ),
          ),
        ],
      ),
    );
  }

  @override
  bool shouldRebuild(covariant _LargeTitleDelegate oldDelegate) =>
      oldDelegate.title != title ||
      oldDelegate.trailing != trailing ||
      oldDelegate.colors != colors;
}

/// 悬浮玻璃底栏：底下必须有滚动内容，玻璃才有意义。
class GlassBottomNav extends StatelessWidget {
  const GlassBottomNav({
    super.key,
    required this.index,
    required this.onTap,
  });

  final int index;
  final ValueChanged<int> onTap;

  static const List<(IconData, IconData, String)> _items = <(IconData, IconData, String)>[
    (Icons.chat_bubble_outline_rounded, Icons.chat_bubble_rounded, '会话'),
    (Icons.smart_toy_outlined, Icons.smart_toy_rounded, '机器人'),
    (Icons.settings_outlined, Icons.settings_rounded, '设置'),
  ];

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(28),
        boxShadow: <BoxShadow>[
          BoxShadow(
            color: Colors.black.withOpacity(0.14),
            blurRadius: 24,
            offset: const Offset(0, 8),
          ),
        ],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(28),
        child: BackdropFilter(
          filter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
          child: Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: colors.elevated.withOpacity(0.72),
              borderRadius: BorderRadius.circular(28),
              border: Border.all(color: colors.sep, width: 0.5),
            ),
            child: Row(
              children: <Widget>[
                for (var i = 0; i < _items.length; i++)
                  Expanded(
                    child: _NavItem(
                      icon: _items[i].$1,
                      selectedIcon: _items[i].$2,
                      label: _items[i].$3,
                      selected: i == index,
                      onTap: () => onTap(i),
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

class _NavItem extends StatelessWidget {
  const _NavItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final color = selected ? colors.accent : colors.text2;
    return Semantics(
      button: true,
      selected: selected,
      label: label,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          height: 52,
          decoration: BoxDecoration(
            color: selected ? colors.accent.withOpacity(0.12) : Colors.transparent,
            borderRadius: BorderRadius.circular(22),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: <Widget>[
              Icon(selected ? selectedIcon : icon, size: 21, color: color),
              const SizedBox(height: 2),
              LText(
                label,
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
                  color: color,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 连接中断 / 表单错误的说明条。
class NoticeCard extends StatelessWidget {
  const NoticeCard({
    super.key,
    required this.title,
    this.message = '',
    this.actionLabel,
    this.onAction,
    this.danger = false,
    this.margin = const EdgeInsets.only(bottom: 16),
  });

  final String title;
  final String message;
  final String? actionLabel;
  final VoidCallback? onAction;
  final bool danger;
  final EdgeInsets margin;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Container(
      margin: margin,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: danger ? colors.danger.withOpacity(0.10) : colors.elevated,
        borderRadius: BorderRadius.circular(LTokens.cardRadius),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          LText(
            title,
            style: context.texts.titleSmall?.copyWith(
              color: danger ? colors.danger : colors.text,
            ),
          ),
          if (message.isNotEmpty) ...<Widget>[
            const SizedBox(height: 4),
            LText(
              message,
              style: context.texts.bodySmall?.copyWith(
                color: danger ? colors.danger : colors.text2,
                height: 1.55,
              ),
            ),
          ],
          if (actionLabel != null && onAction != null) ...<Widget>[
            const SizedBox(height: 12),
            SizedBox(
              height: 44,
              child: OutlinedButton(
                onPressed: onAction,
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.accent,
                  side: BorderSide(color: colors.sep),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(14),
                  ),
                ),
                child: LText(actionLabel!),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// 表单行：左侧标签，右侧输入。
class FieldRow extends StatelessWidget {
  const FieldRow({
    super.key,
    required this.label,
    required this.controller,
    this.hint = '',
    this.obscure = false,
    this.keyboardType,
    this.monospace = false,
    this.onChanged,
  });

  final String label;
  final TextEditingController controller;
  final String hint;
  final bool obscure;
  final TextInputType? keyboardType;
  final bool monospace;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: SizedBox(
        height: 52,
        child: Row(
          children: <Widget>[
            LText(
              label,
              style: context.texts.bodyLarge?.copyWith(
                fontSize: 16,
                color: colors.text,
              ),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: TextField(
                controller: controller,
                obscureText: obscure,
                keyboardType: keyboardType,
                onChanged: onChanged,
                textAlign: TextAlign.right,
                textAlignVertical: TextAlignVertical.center,
                cursorColor: colors.accent,
                style: TextStyle(
                  fontSize: 16,
                  color: colors.text,
                  fontFamily: monospace ? 'monospace' : null,
                ),
                decoration: InputDecoration(
                  hintText: L10n.tr(hint),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 圆角头像：没有头像接口时用首字，别假装有图。
class InitialAvatar extends StatelessWidget {
  const InitialAvatar({
    super.key,
    required this.text,
    this.size = 44,
    this.accent = false,
  });

  final String text;
  final double size;
  final bool accent;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final initial = text.isEmpty ? '?' : String.fromCharCode(text.runes.first);
    return Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: accent ? colors.accent.withOpacity(0.12) : colors.sunken,
        borderRadius: BorderRadius.circular(size / 2.6),
      ),
      child: LText(
        initial,
        style: TextStyle(
          fontSize: size * 0.38,
          fontWeight: FontWeight.w600,
          color: accent ? colors.accent : colors.text2,
        ),
      ),
    );
  }
}

/// 底部抽屉：拖拽条 + 标题 + 独立滚动内容 + 可选页脚。
Future<T?> showLiantSheet<T>({
  required BuildContext context,
  required String title,
  required WidgetBuilder builder,
  bool dismissable = true,
  Widget? footer,
}) {
  final colors = context.colors;
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    isDismissible: dismissable,
    enableDrag: dismissable,
    useSafeArea: true,
    backgroundColor: Colors.transparent,
    builder: (sheetContext) {
      return Padding(
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: Container(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.85,
          ),
          decoration: BoxDecoration(
            color: colors.elevated,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(24)),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              const SizedBox(height: 8),
              Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: colors.sep,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 12, 8, 8),
                child: Row(
                  children: <Widget>[
                    Expanded(
                      child: LText(
                        title,
                        style: context.texts.titleMedium?.copyWith(
                          fontSize: 17,
                          color: colors.text,
                        ),
                      ),
                    ),
                    IconAction(
                      icon: Icons.close_rounded,
                      size: 20,
                      tooltip: '关闭',
                      onTap: () => Navigator.of(sheetContext).pop(),
                    ),
                  ],
                ),
              ),
              Flexible(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: builder(sheetContext),
                ),
              ),
              if (footer != null)
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    16,
                    4,
                    16,
                    MediaQuery.paddingOf(sheetContext).bottom + 16,
                  ),
                  child: footer,
                ),
            ],
          ),
        ),
      );
    },
  );
}

/// 确认弹窗（删除、吊销、解除配对）。
Future<bool> confirmAction(
  BuildContext context, {
  required String title,
  String message = '',
  String confirmLabel = '确定',
  bool danger = false,
}) async {
  final colors = context.colors;
  final result = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => AlertDialog(
      backgroundColor: colors.elevated,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      title: LText(
        title,
        style: context.texts.titleMedium?.copyWith(fontSize: 17, color: colors.text),
      ),
      content: message.isEmpty
          ? null
          : LText(
              message,
              style: context.texts.bodySmall?.copyWith(color: colors.text2, height: 1.55),
            ),
      actionsPadding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      actions: <Widget>[
        TextButton(
          onPressed: () => Navigator.of(dialogContext).pop(false),
          child: LText('取消', style: TextStyle(color: colors.text2)),
        ),
        TextButton(
          onPressed: () => Navigator.of(dialogContext).pop(true),
          child: LText(
            confirmLabel,
            style: TextStyle(
              color: danger ? colors.danger : colors.accent,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    ),
  );
  return result == true;
}

/// 渲染协议 Body.doc：v1 的 6 种 span / 3 种块，未知类型只取文本。
class RichBody extends StatelessWidget {
  const RichBody({
    super.key,
    required this.body,
    required this.color,
    this.accentColor,
  });

  final MsgBody body;
  final Color color;
  final Color? accentColor;

  @override
  Widget build(BuildContext context) {
    final doc = body.doc;
    final blocks = doc?['blocks'];
    if (blocks is! List || blocks.isEmpty) {
      return LText(
        body.plain.isEmpty ? '空消息' : body.plain,
        style: TextStyle(fontSize: 15, height: 1.45, color: color),
      );
    }
    final rendered = <Widget>[];
    for (final raw in blocks) {
      if (raw is! Map) continue;
      final type = raw['type'] as String? ?? 'p';
      final spans = raw['spans'];
      if (spans is! List) continue;
      final text = Text.rich(
        TextSpan(children: _spans(context, spans)),
        style: TextStyle(fontSize: 15, height: 1.5, color: color),
      );
      if (type == 'code') {
        rendered.add(
          Container(
            width: double.infinity,
            margin: const EdgeInsets.symmetric(vertical: 4),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: color.withOpacity(0.08),
              borderRadius: BorderRadius.circular(10),
            ),
            child: DefaultTextStyle.merge(
              style: const TextStyle(fontFamily: 'monospace', fontSize: 13.5),
              child: text,
            ),
          ),
        );
      } else if (type == 'quote') {
        rendered.add(
          Container(
            margin: const EdgeInsets.symmetric(vertical: 4),
            padding: const EdgeInsets.only(left: 10),
            decoration: BoxDecoration(
              border: Border(
                left: BorderSide(color: color.withOpacity(0.35), width: 2),
              ),
            ),
            child: text,
          ),
        );
      } else {
        rendered.add(Padding(
          padding: const EdgeInsets.symmetric(vertical: 2),
          child: text,
        ));
      }
    }
    if (rendered.isEmpty) {
      return LText(
        body.plain.isEmpty ? '空消息' : body.plain,
        style: TextStyle(fontSize: 15, height: 1.45, color: color),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: rendered,
    );
  }

  List<InlineSpan> _spans(BuildContext context, List<dynamic> spans) {
    final accent = accentColor ?? context.colors.accent;
    final out = <InlineSpan>[];
    for (final raw in spans) {
      if (raw is! Map) continue;
      final text = raw['text'] as String? ?? '';
      if (text.isEmpty) continue;
      switch (raw['t'] as String? ?? 'text') {
        case 'bold':
          out.add(TextSpan(text: text, style: const TextStyle(fontWeight: FontWeight.w700)));
        case 'italic':
          out.add(TextSpan(text: text, style: const TextStyle(fontStyle: FontStyle.italic)));
        case 'strike':
          out.add(TextSpan(
            text: text,
            style: const TextStyle(decoration: TextDecoration.lineThrough),
          ));
        case 'code':
          out.add(TextSpan(
            text: text,
            style: TextStyle(fontFamily: 'monospace', fontSize: 13.5, color: accent),
          ));
        case 'link':
          out.add(TextSpan(
            text: text,
            style: TextStyle(
              color: accent,
              decoration: TextDecoration.underline,
              decorationColor: accent,
            ),
          ));
        default:
          out.add(TextSpan(text: text));
      }
    }
    return out;
  }
}

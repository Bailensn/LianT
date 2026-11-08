// 共用组件：大标题页、分组卡片、玻璃底栏、气泡、抽屉、按钮。
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/physics.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../core/app_state.dart';
import '../core/protocol.dart';
import 'tokens.dart';
import '../l10n/l10n.dart';
import '../l10n/ltext.dart';
import 'squircle.dart';

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
          SquircleClip(
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
      decoration: SquircleBoxDecoration(
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
class PressScale extends StatefulWidget {
  const PressScale({super.key, required this.child, this.enabled = true});

  final Widget child;
  final bool enabled;

  @override
  State<PressScale> createState() => _PressScaleState();
}

class _PressScaleState extends State<PressScale> {
  bool _pressed = false;

  @override
  Widget build(BuildContext context) {
    return Listener(
      onPointerDown: widget.enabled
          ? (_) => setState(() => _pressed = true)
          : null,
      onPointerUp: widget.enabled
          ? (_) => setState(() => _pressed = false)
          : null,
      onPointerCancel: widget.enabled
          ? (_) => setState(() => _pressed = false)
          : null,
      child: AnimatedScale(
        scale: _pressed ? 0.975 : 1,
        duration: const Duration(milliseconds: 110),
        curve: Curves.easeOutCubic,
        child: widget.child,
      ),
    );
  }
}

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
    return PressScale(
      enabled: enabled,
      child: Material(
      color: enabled ? colors.accent : colors.sunken,
      shape: ContinuousRectangleBorder(
        borderRadius: BorderRadius.circular(LTokens.controlRadius),
      ),
      child: InkWell(
        customBorder: ContinuousRectangleBorder(
          borderRadius: BorderRadius.circular(LTokens.controlRadius),
        ),
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
    return PressScale(
      enabled: enabled,
      child: Material(
      color: colors.sunken,
      shape: ContinuousRectangleBorder(
        borderRadius: BorderRadius.circular(14),
      ),
      child: InkWell(
        customBorder: ContinuousRectangleBorder(
          borderRadius: BorderRadius.circular(14),
        ),
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
    return PressScale(enabled: onTap != null, child: button);
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
            decoration: SquircleBoxDecoration(
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
      decoration: SquircleBoxDecoration(
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
class GlassBottomNav extends StatefulWidget {
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
  State<GlassBottomNav> createState() => _GlassBottomNavState();
}

// 交互与弹簧参数参考 FlClash / prism_ui 的 NavigationDock（GPL-3.0），
// 这里只保留与 LianT 底栏相关的按下、跟手、甩动和回位逻辑。
class _NavSpring extends ChangeNotifier implements ValueListenable<double> {
  _NavSpring(TickerProvider vsync, double value) {
    _ticker = vsync.createTicker(_tick);
    _value = value;
  }

  static const double rest = 1e-3;
  late final Ticker _ticker;
  double _value = 0;
  double _target = 0;
  double _now = 0;
  double _start = 0;
  SpringSimulation? _simulation;

  @override
  double get value => _value;

  double get _velocity => _simulation?.dx(_now - _start) ?? 0;

  void springTo(double target, SpringDescription spring) {
    _simulation = SpringSimulation(
      spring,
      _value,
      target,
      _velocity,
      tolerance: Tolerance(
        distance: rest,
        velocity: rest * math.sqrt(spring.stiffness / spring.mass),
      ),
    );
    _target = target;
    if (_ticker.isActive) {
      _start = _now;
      return;
    }
    _now = _start = 0;
    _ticker.start();
  }

  void jumpTo(double value) {
    _simulation = null;
    _ticker.stop();
    _value = value;
    notifyListeners();
  }

  void _tick(Duration elapsed) {
    _now = elapsed.inMicroseconds / Duration.microsecondsPerSecond;
    final simulation = _simulation;
    if (simulation == null) return;
    final time = _now - _start;
    if (simulation.isDone(time)) {
      _value = _target;
      _simulation = null;
      _ticker.stop();
    } else {
      _value = simulation.x(time);
    }
    notifyListeners();
  }

  @override
  void dispose() {
    _ticker.dispose();
    super.dispose();
  }
}

class _GlassBottomNavState extends State<GlassBottomNav>
    with TickerProviderStateMixin {
  static final SpringDescription _followSpring =
      SpringDescription.withDampingRatio(mass: 1, stiffness: 1024, ratio: 0.9);
  // 越界后改用高阻尼弹簧，快速甩到边缘也不会再来回撞墙。
  static final SpringDescription _boundarySpring =
      SpringDescription.withDampingRatio(mass: 1, stiffness: 600, ratio: 2.0);
  static final SpringDescription _pressSpring =
      SpringDescription.withDampingRatio(mass: 1, stiffness: 311, ratio: 0.593);
  // 松手回位只留很轻的余量；ratio 越低回弹越大，0.85 已接近临界阻尼。
  static final SpringDescription _travelSpring =
      SpringDescription.withDampingRatio(
    mass: 1,
    stiffness: 300,
    ratio: 0.85,
  );

  late final _NavSpring _lens = _NavSpring(this, widget.index.toDouble());
  late final _NavSpring _press = _NavSpring(this, 0);
  int? _pointer;
  VelocityTracker? _tracker;
  double _downX = 0;
  double _itemWidth = 0;
  int? _previewIndex;
  bool _dragging = false;
  bool _pressed = false;

  int get _lastIndex => GlassBottomNav._items.length - 1;

  int _indexAt(double dx) {
    if (_itemWidth <= 0) return widget.index;
    return (dx / _itemWidth - 0.5).round().clamp(0, _lastIndex);
  }

  double _positionAt(double dx) {
    if (_itemWidth <= 0) return widget.index.toDouble();
    final raw = dx / _itemWidth - 0.5;
    const overshoot = 0.18;
    if (raw < 0) return math.max(raw * 0.18, -overshoot);
    if (raw > _lastIndex) {
      return math.min(_lastIndex + (raw - _lastIndex) * 0.18, _lastIndex + overshoot);
    }
    return raw;
  }

  void _onDown(PointerDownEvent event) {
    if (_pointer != null) return;
    _pointer = event.pointer;
    _tracker = VelocityTracker.withKind(event.kind)
      ..addPosition(event.timeStamp, event.localPosition);
    _downX = event.localPosition.dx;
    _dragging = false;
    _pressed = true;
    _previewIndex = _indexAt(_downX);
    _press.springTo(1, _pressSpring);
    _lens.springTo(_previewIndex!.toDouble(), _pressSpring);
    setState(() {});
  }

  void _onMove(PointerMoveEvent event) {
    if (event.pointer != _pointer) return;
    _tracker?.addPosition(event.timeStamp, event.localPosition);
    if (!_dragging && (event.localPosition.dx - _downX).abs() < kTouchSlop) {
      return;
    }
    _dragging = true;
    final raw = event.localPosition.dx / _itemWidth - 0.5;
    final outside = raw < 0 || raw > _lastIndex;
    final position = _positionAt(event.localPosition.dx);
    final index = position.round().clamp(0, _lastIndex);
    if (index != _previewIndex) {
      HapticFeedback.selectionClick();
      setState(() => _previewIndex = index);
    }
    // 保留继承速度；越界后用远高于临界的阻尼把它吃掉。
    _lens.springTo(position, outside ? _boundarySpring : _followSpring);
  }

  void _onUp(PointerUpEvent event) {
    if (event.pointer != _pointer) return;
    final tracker = _tracker;
    var target = _dragging
        ? _lens.value.round().clamp(0, _lastIndex)
        : (_previewIndex ?? widget.index);
    if (_dragging && tracker != null) {
      final velocity = tracker.getVelocity().pixelsPerSecond.dx;
      if (velocity.abs() > 500) {
        target = (target + (velocity > 0 ? 1 : -1)).clamp(0, _lastIndex);
      }
    }
    _finish(target);
  }

  void _onCancel(PointerCancelEvent event) {
    if (event.pointer != _pointer) return;
    _finish(widget.index);
  }

  void _finish(int target) {
    _pointer = null;
    _tracker = null;
    _dragging = false;
    _pressed = false;
    _previewIndex = null;
    _press.springTo(0, _pressSpring);
    _lens.springTo(target.toDouble(), _travelSpring);
    setState(() {});
    if (target != widget.index) {
      HapticFeedback.selectionClick();
      widget.onTap(target);
    }
  }

  @override
  void didUpdateWidget(covariant GlassBottomNav oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (_pointer == null && oldWidget.index != widget.index) {
      _lens.springTo(widget.index.toDouble(), _travelSpring);
    }
  }

  @override
  void dispose() {
    _lens.dispose();
    _press.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return AnimatedBuilder(
      animation: Listenable.merge(<Listenable>[_lens, _press]),
      builder: (context, _) {
        final selected = _previewIndex ?? widget.index;
        return Transform.scale(
          scale: 1 + _press.value * 0.012,
          child: Container(
            decoration: SquircleBoxDecoration(
              borderRadius: BorderRadius.circular(28),
              boxShadow: <BoxShadow>[
                BoxShadow(
                  color: Colors.black.withOpacity(0.14),
                  blurRadius: 24,
                  offset: const Offset(0, 8),
                ),
              ],
            ),
            child: SquircleClip(
              borderRadius: BorderRadius.circular(28),
              child: BackdropFilter(
                filter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
                child: Container(
                  padding: const EdgeInsets.all(6),
                  decoration: SquircleBoxDecoration(
                    color: colors.elevated.withOpacity(0.72),
                    borderRadius: BorderRadius.circular(28),
                    border: Border.all(color: colors.sep, width: 0.5),
                  ),
                  child: SizedBox(
                    height: 52,
                    child: LayoutBuilder(
                      builder: (context, constraints) {
                        _itemWidth = constraints.maxWidth /
                            GlassBottomNav._items.length;
                        return Listener(
                          behavior: HitTestBehavior.opaque,
                          onPointerDown: _onDown,
                          onPointerMove: _onMove,
                          onPointerUp: _onUp,
                          onPointerCancel: _onCancel,
                          child: Stack(
                            children: <Widget>[
                              Positioned(
                                left: _lens.value * _itemWidth,
                                top: 0,
                                width: _itemWidth,
                                height: 52,
                                child: Container(
                                  decoration: SquircleBoxDecoration(
                                    color: colors.accent.withOpacity(
                                      _pressed ? 0.18 : 0.12,
                                    ),
                                    borderRadius: BorderRadius.circular(22),
                                  ),
                                ),
                              ),
                              Row(
                                children: <Widget>[
                                  for (var i = 0;
                                      i < GlassBottomNav._items.length;
                                      i++)
                                    Expanded(
                                      child: _NavItem(
                                        icon: GlassBottomNav._items[i].$1,
                                        selectedIcon:
                                            GlassBottomNav._items[i].$2,
                                        label: GlassBottomNav._items[i].$3,
                                        selected: i == selected,
                                        onTap: () => widget.onTap(i),
                                      ),
                                    ),
                                ],
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
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
        child: SizedBox(
          height: 52,
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
      decoration: SquircleBoxDecoration(
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
                  shape: ContinuousRectangleBorder(
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
      decoration: SquircleBoxDecoration(
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
    // Flutter 3.24 的 AnimationStyle 构造函数不是 const。
    // ignore: prefer_const_constructors
    sheetAnimationStyle: AnimationStyle(
      duration: const Duration(milliseconds: 260),
      reverseDuration: const Duration(milliseconds: 200),
    ),
    builder: (sheetContext) {
      return TweenAnimationBuilder<double>(
        tween: Tween<double>(begin: 0, end: 1),
        duration: const Duration(milliseconds: 260),
        curve: Curves.easeOutCubic,
        child: Padding(
          padding: EdgeInsets.only(
            bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
          ),
          child: Container(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.85,
          ),
          decoration: SquircleBoxDecoration(
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
                decoration: SquircleBoxDecoration(
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
        ),
        builder: (context, value, child) {
          final t = Curves.easeOutCubic.transform(value);
          return Opacity(
            opacity: 0.72 + 0.28 * t,
            child: Transform.scale(
              scale: 0.96 + 0.04 * t,
              alignment: Alignment.bottomCenter,
              child: child,
            ),
          );
        },
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
  final result = await showGeneralDialog<bool>(
    context: context,
    barrierDismissible: true,
    barrierLabel: title,
    barrierColor: Colors.black.withOpacity(0.45),
    transitionDuration: const Duration(milliseconds: 180),
    transitionBuilder: (context, animation, secondaryAnimation, child) {
      final curved = CurvedAnimation(
        parent: animation,
        curve: Curves.easeOutCubic,
        reverseCurve: Curves.easeInCubic,
      );
      return FadeTransition(
        opacity: curved,
        child: ScaleTransition(
          scale: Tween<double>(begin: 0.92, end: 1).animate(curved),
          child: child,
        ),
      );
    },
    pageBuilder: (dialogContext, animation, secondaryAnimation) => AlertDialog(
      backgroundColor: colors.elevated,
      surfaceTintColor: Colors.transparent,
      shape: ContinuousRectangleBorder(borderRadius: BorderRadius.circular(20)),
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
            decoration: SquircleBoxDecoration(
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
            decoration: SquircleBoxDecoration(
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

/// 普通二级页：聊天页同款返回头 + 普通列表，不再用大标题。
class DetailScaffold extends StatelessWidget {
  const DetailScaffold({
    super.key,
    required this.title,
    required this.children,
    this.trailing,
  });

  final String title;
  final List<Widget> children;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return StatusBarTint(
      color: Color.alphaBlend(colors.elevated.withOpacity(0.92), colors.bg),
      child: Scaffold(
      backgroundColor: colors.bg,
      body: Column(
        children: <Widget>[
          SafeArea(
            bottom: false,
            child: Container(
              height: 56,
              decoration: SquircleBoxDecoration(
                color: colors.elevated.withOpacity(0.92),
                border: Border(
                  bottom: BorderSide(color: colors.sep, width: 0.5),
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
                    child: Center(
                      child: LText(
                        title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 17,
                          fontWeight: FontWeight.w600,
                          color: colors.text,
                        ),
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 76,
                    child: trailing == null
                        ? null
                        : Center(child: trailing),
                  ),
                ],
              ),
            ),
          ),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 40),
              children: children,
            ),
          ),
        ],
      ),
    ),
    );
  }
}

/// 把状态栏区域涂成标题栏同色；系统图标仍由全局 SystemUiOverlayStyle 控制。
class StatusBarTint extends StatelessWidget {
  const StatusBarTint({super.key, required this.color, required this.child});

  final Color color;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final top = MediaQuery.paddingOf(context).top;
    return Stack(
      fit: StackFit.expand,
      children: <Widget>[
        child,
        if (top > 0)
          Positioned(
            top: 0,
            left: 0,
            right: 0,
            height: top,
            child: IgnorePointer(
              child: ColoredBox(color: color),
            ),
          ),
      ],
    );
  }
}

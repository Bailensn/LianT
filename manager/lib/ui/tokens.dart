// 视觉 token 与主题。
//
// 与 docs/DESIGN.md §7 对齐：背景分层表达层级、单一品牌蓝、34px 大标题、
// 1px 半透明分隔线、深浅色跟随系统。
import 'package:flutter/material.dart';
import 'squircle.dart';

/// 一套颜色。用 ThemeExtension 挂到 ThemeData 上，页面里直接 `context.colors`。
@immutable
class LColors extends ThemeExtension<LColors> {
  const LColors({
    required this.bg,
    required this.elevated,
    required this.sunken,
    required this.text,
    required this.text2,
    required this.text3,
    required this.sep,
    required this.accent,
    required this.danger,
    required this.ok,
  });

  final Color bg;
  final Color elevated;
  final Color sunken;
  final Color text;
  final Color text2;
  final Color text3;
  final Color sep;
  final Color accent;
  final Color danger;
  final Color ok;

  static const LColors light = LColors(
    bg: Color(0xFFF4F4F7),
    elevated: Color(0xFFFFFFFF),
    sunken: Color(0xFFEAEAEF),
    text: Color(0xFF111114),
    text2: Color(0xFF616168),
    text3: Color(0xFF8A8A92),
    sep: Color(0x14000000),
    accent: Color(0xFF2F6BFF),
    danger: Color(0xFFD93A40),
    ok: Color(0xFF2E9E63),
  );

  /// 深色背景是纯黑（OLED 省电），卡片提亮一级。
  static const LColors dark = LColors(
    bg: Color(0xFF0E0E10),
    elevated: Color(0xFF1B1B1F),
    sunken: Color(0xFF141417),
    text: Color(0xFFF4F4F7),
    text2: Color(0xFFA6A6AE),
    text3: Color(0xFF7E7E87),
    sep: Color(0x1FFFFFFF),
    accent: Color(0xFF4C7DFF),
    danger: Color(0xFFFF6B6B),
    ok: Color(0xFF4CD07D),
  );

  static LColors of(BuildContext context) =>
      Theme.of(context).extension<LColors>() ?? light;

  @override
  LColors copyWith({
    Color? bg,
    Color? elevated,
    Color? sunken,
    Color? text,
    Color? text2,
    Color? text3,
    Color? sep,
    Color? accent,
    Color? danger,
    Color? ok,
  }) {
    return LColors(
      bg: bg ?? this.bg,
      elevated: elevated ?? this.elevated,
      sunken: sunken ?? this.sunken,
      text: text ?? this.text,
      text2: text2 ?? this.text2,
      text3: text3 ?? this.text3,
      sep: sep ?? this.sep,
      accent: accent ?? this.accent,
      danger: danger ?? this.danger,
      ok: ok ?? this.ok,
    );
  }

  @override
  LColors lerp(ThemeExtension<LColors>? other, double t) {
    if (other is! LColors) return this;
    return LColors(
      bg: Color.lerp(bg, other.bg, t)!,
      elevated: Color.lerp(elevated, other.elevated, t)!,
      sunken: Color.lerp(sunken, other.sunken, t)!,
      text: Color.lerp(text, other.text, t)!,
      text2: Color.lerp(text2, other.text2, t)!,
      text3: Color.lerp(text3, other.text3, t)!,
      sep: Color.lerp(sep, other.sep, t)!,
      accent: Color.lerp(accent, other.accent, t)!,
      danger: Color.lerp(danger, other.danger, t)!,
      ok: Color.lerp(ok, other.ok, t)!,
    );
  }
}

/// 颜色、字号与圆角之外的少量常量。
class LTokens {
  const LTokens._();

  /// 卡片圆角；内部按钮 12–16，气泡 18。
  static const double cardRadius = 22;
  static const double controlRadius = 18;

  /// iOS 大标题与折叠后的小标题。
  static const double largeTitle = 34;
  static const double compactTitle = 20;

  /// 触控热区下限。
  static const double touchTarget = 44;
}

class LiantTheme {
  const LiantTheme._();

  static ThemeData light() => _build(Brightness.light, LColors.light);

  static ThemeData dark() => _build(Brightness.dark, LColors.dark);

  static ThemeData _build(Brightness brightness, LColors colors) {
    final base = ThemeData(
      useMaterial3: true,
      brightness: brightness,
      // 显式声明平台默认字体；网页与 Android 都是 Roboto，测试里也用它加载中文字体。
      fontFamily: 'Roboto',
      colorScheme: ColorScheme.fromSeed(
        seedColor: colors.accent,
        brightness: brightness,
      ).copyWith(
        primary: colors.accent,
        surface: colors.elevated,
        error: colors.danger,
      ),
      scaffoldBackgroundColor: colors.bg,
    );

    // 显式给每个文字样式上色：自定义 TextTheme 不带 color 时，
    // Android 上部分控件（尤其 TextField）会落到默认白色，白底上直接看不见。
    final dark = brightness == Brightness.dark;
    FontWeight? w(FontWeight? weight) {
      if (!dark || weight == null) return weight;
      if (weight == FontWeight.w700) return FontWeight.w600;
      if (weight == FontWeight.w600) return FontWeight.w500;
      if (weight == FontWeight.w500) return FontWeight.w400;
      return weight;
    }

    final textTheme = TextTheme(
      displaySmall: TextStyle(
        fontSize: LTokens.largeTitle,
        fontWeight: w(FontWeight.w700),
        letterSpacing: -0.8,
        height: 1.12,
      ),
      headlineSmall: TextStyle(
        fontSize: 22,
        fontWeight: w(FontWeight.w700),
        letterSpacing: -0.3,
        height: 1.2,
      ),
      titleLarge: TextStyle(fontSize: 20, fontWeight: w(FontWeight.w700), height: 1.25),
      titleMedium: TextStyle(fontSize: 17, fontWeight: w(FontWeight.w600), height: 1.3),
      titleSmall: TextStyle(fontSize: 15, fontWeight: w(FontWeight.w600), height: 1.3),
      bodyLarge: const TextStyle(fontSize: 16, height: 1.4),
      bodyMedium: const TextStyle(fontSize: 15, height: 1.4),
      bodySmall: const TextStyle(fontSize: 13, height: 1.45),
      labelLarge: TextStyle(fontSize: 15, fontWeight: w(FontWeight.w600)),
      labelMedium: TextStyle(fontSize: 13, fontWeight: w(FontWeight.w500)),
      labelSmall: TextStyle(
        fontSize: 12,
        fontWeight: w(FontWeight.w500),
        letterSpacing: 0.2,
        fontFeatures: const <FontFeature>[FontFeature.tabularFigures()],
      ),
    ).apply(bodyColor: colors.text, displayColor: colors.text);

    return base.copyWith(
      splashFactory: InkRipple.splashFactory,
      extensions: <ThemeExtension<dynamic>>[colors],
      textTheme: textTheme,
      primaryTextTheme: textTheme,
      inputDecorationTheme: InputDecorationTheme(
        border: InputBorder.none,
        isDense: true,
        hintStyle: TextStyle(fontSize: 15, color: colors.text3),
        contentPadding: EdgeInsets.zero,
      ),
      dividerTheme: DividerThemeData(
        color: colors.sep,
        thickness: 0.5,
        space: 0.5,
      ),
      pageTransitionsTheme: const PageTransitionsTheme(
        builders: <TargetPlatform, PageTransitionsBuilder>{
          TargetPlatform.android: _SoftPageTransitionsBuilder(),
          TargetPlatform.iOS: _SoftPageTransitionsBuilder(),
          TargetPlatform.linux: _SoftPageTransitionsBuilder(),
          TargetPlatform.macOS: _SoftPageTransitionsBuilder(),
          TargetPlatform.windows: _SoftPageTransitionsBuilder(),
        },
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: colors.text,
        contentTextStyle: TextStyle(fontSize: 14, color: colors.bg),
        shape: ContinuousRectangleBorder(
          borderRadius: squircleRadius(BorderRadius.circular(16)),
        ),
      ),
    );
  }
}

/// 页面里少写一点 `Theme.of(context)`。
extension LBuildContext on BuildContext {
  LColors get colors => LColors.of(this);

  TextTheme get texts => Theme.of(this).textTheme;

  bool get isDesktop => MediaQuery.sizeOf(this).width >= 768;

  double get bottomInset => MediaQuery.paddingOf(this).bottom;
}


/// 克制的进入/退出动效：轻微上移 + 淡入，曲线只做 ease-out。
class _SoftPageTransitionsBuilder extends PageTransitionsBuilder {
  const _SoftPageTransitionsBuilder();

  @override
  Widget buildTransitions<T>(
    PageRoute<T> route,
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
    Widget child,
  ) {
    final curved = CurvedAnimation(
      parent: animation,
      curve: Curves.easeOutCubic,
      reverseCurve: Curves.easeInCubic,
    );
    return FadeTransition(
      opacity: curved,
      child: SlideTransition(
        position: Tween<Offset>(
          begin: const Offset(0.015, 0),
          end: Offset.zero,
        ).animate(curved),
        child: child,
      ),
    );
  }
}

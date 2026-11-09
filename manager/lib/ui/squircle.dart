import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

/// iOS 连续曲率圆角装饰：用 ContinuousRectangleBorder 画 squircle，
/// 非均匀边框（例如只有底边）回退到普通 BoxDecoration。
/// 连续曲率在同样数值下看起来比圆角更“紧”，统一放大一档。
const double _radiusScale = 1.6;

BorderRadiusGeometry squircleRadius(BorderRadiusGeometry? value) {
  if (value is BorderRadius) {
    Radius scale(Radius r) => Radius.elliptical(r.x * _radiusScale, r.y * _radiusScale);
    return BorderRadius.only(
      topLeft: scale(value.topLeft),
      topRight: scale(value.topRight),
      bottomLeft: scale(value.bottomLeft),
      bottomRight: scale(value.bottomRight),
    );
  }
  if (value is BorderRadiusDirectional) {
    Radius scale(Radius r) => Radius.elliptical(r.x * _radiusScale, r.y * _radiusScale);
    return BorderRadiusDirectional.only(
      topStart: scale(value.topStart),
      topEnd: scale(value.topEnd),
      bottomStart: scale(value.bottomStart),
      bottomEnd: scale(value.bottomEnd),
    );
  }
  return value ?? BorderRadius.zero;
}

class SquircleBoxDecoration implements Decoration {
  const SquircleBoxDecoration({
    this.color,
    this.borderRadius,
    this.border,
    this.boxShadow,
    this.gradient,
    this.shape = BoxShape.rectangle,
  });

  final Color? color;
  final BorderRadiusGeometry? borderRadius;
  final BoxBorder? border;
  final List<BoxShadow>? boxShadow;
  final Gradient? gradient;
  final BoxShape shape;

  bool get _uniformBorder {
    final b = border;
    if (b == null) return true;
    if (b is! Border) return false;
    final top = b.top;
    return top == b.right && top == b.bottom && top == b.left;
  }

  Decoration get _inner {
    if (shape == BoxShape.circle) {
      return ShapeDecoration(
        shape: const CircleBorder(),
        color: color,
        gradient: gradient,
        shadows: boxShadow,
      );
    }
    if (!_uniformBorder) {
      return BoxDecoration(
        color: color,
        borderRadius: squircleRadius(borderRadius),
        border: border,
        boxShadow: boxShadow,
        gradient: gradient,
        shape: shape,
      );
    }
    final side = border is Border ? (border as Border).top : BorderSide.none;
    return ShapeDecoration(
      shape: ContinuousRectangleBorder(
        borderRadius: squircleRadius(borderRadius),
        side: side,
      ),
      color: color,
      gradient: gradient,
      shadows: boxShadow,
    );
  }

  @override
  BoxPainter createBoxPainter([VoidCallback? onChanged]) =>
      _inner.createBoxPainter(onChanged ?? () {});

  @override
  Path getClipPath(Rect rect, TextDirection textDirection) =>
      _inner.getClipPath(rect, textDirection);

  @override
  bool hitTest(Size size, Offset position, {TextDirection? textDirection}) =>
      _inner.hitTest(size, position, textDirection: textDirection);

  @override
  bool get isComplex => _inner.isComplex;

  @override
  EdgeInsetsGeometry get padding => _inner.padding;

  @override
  bool debugAssertIsValid() => true;

  @override
  Decoration? lerpFrom(Decoration? a, double t) => t < 0.5 ? a : this;

  @override
  Decoration? lerpTo(Decoration? b, double t) => t < 0.5 ? this : b;

  @override
  String toStringShort() => 'SquircleBoxDecoration';

  @override
  String toString({DiagnosticLevel minLevel = DiagnosticLevel.info}) =>
      'SquircleBoxDecoration';

  @override
  void debugFillProperties(DiagnosticPropertiesBuilder properties) =>
      _inner.debugFillProperties(properties);

  @override
  DiagnosticsNode toDiagnosticsNode({
    String? name,
    DiagnosticsTreeStyle? style,
  }) =>
      _inner.toDiagnosticsNode(name: name, style: style);
}

/// 用连续曲率路径裁剪内容（图片、玻璃层等）。
class SquircleClip extends StatelessWidget {
  const SquircleClip({
    super.key,
    required this.borderRadius,
    required this.child,
  });

  final BorderRadius borderRadius;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return ClipPath(
      clipper: ShapeBorderClipper(
        shape: ContinuousRectangleBorder(
          borderRadius: squircleRadius(borderRadius),
        ),
      ),
      child: child,
    );
  }
}

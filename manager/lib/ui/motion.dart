import 'package:flutter/material.dart';
import 'package:flutter/physics.dart';

/// 松手后带初速度的弹簧：基础阻尼比 1.1，按速度和边界距离动态调整。
/// 越快越活、越靠近边界越稳，系统关闭动画时直接加重阻尼。
double springDampingRatio(double velocity, double distance) {
  if (WidgetsBinding
      .instance.platformDispatcher.accessibilityFeatures.disableAnimations) {
    return 1.35;
  }
  final speed = velocity.abs();
  if (speed > 2600) return 0.9;
  if (speed > 1200) return 1.0;
  if (distance < 48) return 1.2;
  return 1.1;
}

/// 滚动到边界时用 SpringSimulation 带回弹；普通滚动仍走 BouncingScrollPhysics。
class SpringScrollPhysics extends BouncingScrollPhysics {
  const SpringScrollPhysics({super.parent});

  @override
  SpringScrollPhysics applyTo(ScrollPhysics? ancestor) =>
      SpringScrollPhysics(parent: buildParent(ancestor));

  @override
  Simulation? createBallisticSimulation(
    ScrollMetrics position,
    double velocity,
  ) {
    if (position.outOfRange) {
      final target = position.pixels
          .clamp(position.minScrollExtent, position.maxScrollExtent)
          .toDouble();
      final distance = (target - position.pixels).abs();
      final spring = SpringDescription.withDampingRatio(
        mass: 1,
        stiffness: 240,
        ratio: springDampingRatio(velocity, distance),
      );
      return SpringSimulation(
        spring,
        position.pixels,
        target,
        velocity,
        tolerance: const Tolerance(velocity: 0.08, distance: 0.08),
      );
    }
    return super.createBallisticSimulation(position, velocity);
  }
}

// Web/桌面：没有独立后台进程，这里全是空实现。
//
// 与 background_state_io.dart 保持同一套签名，调用方不用关心平台。
import '../identity/identity.dart';

class BackgroundState {
  const BackgroundState({
    required this.identity,
    required this.foreground,
    required this.muted,
    required this.titles,
    required this.remarks,
  });

  final Identity identity;
  final bool foreground;
  final Map<String, bool> muted;
  final Map<String, String> titles;
  final Map<String, String> remarks;

  static BackgroundState? fromJson(Map<String, dynamic> json) => null;
}

Future<void> writeBackgroundState(BackgroundState state) async {}

Future<void> clearBackgroundState() async {}

Future<BackgroundState?> readBackgroundState() async => null;

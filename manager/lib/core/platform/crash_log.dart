// 崩溃/未捕获异常落盘，方便下次在设置页里看到「上次为什么没了」。
export 'crash_log_io.dart'
    if (dart.library.js_interop) 'crash_log_stub.dart';

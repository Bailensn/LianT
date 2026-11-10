// 条件导出：Web 没有后台进程，只有直连。
export 'message_client_io.dart'
    if (dart.library.js_interop) 'message_client_web.dart';

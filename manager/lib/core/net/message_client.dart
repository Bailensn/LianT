// 条件导出：Web 与 Android 各自决定怎么建客户端。
export 'message_client_io.dart'
    if (dart.library.js_interop) 'message_client_web.dart';

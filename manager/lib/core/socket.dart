// 条件导出：原生端可以处理自签证书，Web 端只能交给浏览器。
export 'socket_io.dart' if (dart.library.js_interop) 'socket_web.dart';

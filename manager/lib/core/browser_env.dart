// 条件导出：网页版读地址栏，其它平台返回 null。
export 'browser_env_io.dart' if (dart.library.js_interop) 'browser_env_web.dart';

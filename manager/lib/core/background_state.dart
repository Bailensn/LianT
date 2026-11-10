// 条件导出：后台状态文件只有原生端需要，网页版是空实现。
export 'background_state_io.dart'
    if (dart.library.js_interop) 'background_state_stub.dart';

// 条件导出：后台消息循环只在原生端编译，网页版是空实现。
export 'runtime_io.dart'
    if (dart.library.js_interop) 'runtime_stub.dart';

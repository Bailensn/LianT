// Web：没有本地文件可写，吞掉。
Future<void> recordError(Object error, StackTrace? stack) async {}

Future<String?> readLastError() async => null;

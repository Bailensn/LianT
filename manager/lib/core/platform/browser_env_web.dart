// 网页版可以从地址栏读取预览参数（例如 ?host=127.0.0.1:8099）。
import 'package:web/web.dart' as web;

String? browserQueryParam(String name) {
  try {
    return Uri.parse(web.window.location.href).queryParameters[name];
  } on FormatException {
    return null;
  }
}

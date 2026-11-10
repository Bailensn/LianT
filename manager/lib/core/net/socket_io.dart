// Android/桌面的 WebSocket：允许自签证书。
//
// 自托管 Service 默认用自签证书，Android 系统信任库不会认；但真正的身份校验在
// auth.challenge：客户端比对 service_pub 的指纹并验签，TLS 不是唯一信任边界。
// 浏览器不允许绕过证书校验，所以 Web 端仍要求 Service 用受信任的证书。
import 'dart:io';

import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

WebSocketChannel openWebSocket(Uri uri) {
  final client = HttpClient()
    ..connectionTimeout = const Duration(seconds: 15)
    ..badCertificateCallback =
        (X509Certificate certificate, String host, int port) => true;
  return IOWebSocketChannel.connect(uri, customClient: client);
}

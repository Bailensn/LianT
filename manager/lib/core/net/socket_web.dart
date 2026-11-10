// Web：证书交给浏览器校验，客户端不能忽略错误。
import 'package:web_socket_channel/web_socket_channel.dart';

WebSocketChannel openWebSocket(Uri uri) => WebSocketChannel.connect(uri);

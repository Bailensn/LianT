// 设备身份与配对信息。
//
// 私钥是本机生成的 P-256 标量，只落在本地存储；服务端只保存公钥，
// 每次连接用挑战签名证明「还是这台设备」。
import 'dart:convert';
import 'dart:typed_data';

import 'p256.dart';

/// 一个服务地址（自动配对/扫码信息里都会用到）。
class ServerAddress {
  const ServerAddress({required this.host, required this.port, this.secure = true});

  final String host;
  final int port;
  final bool secure;

  String get label => '$host:$port';

  String get wsUrl => '${secure ? 'wss' : 'ws'}://$host:$port/v1/ws';

  String get httpBase => '${secure ? 'https' : 'http'}://$host:$port';

  /// 接受 `host:port`、`wss://host:port`、`https://host` 三种写法。
  static ServerAddress? parse(String input) {
    final text = input.trim();
    if (text.isEmpty) return null;
    if (text.contains('://')) {
      final uri = Uri.tryParse(text);
      if (uri == null || uri.host.isEmpty) return null;
      final secure = uri.scheme == 'wss' || uri.scheme == 'https';
      final port = uri.hasPort ? uri.port : (secure ? 443 : 80);
      return ServerAddress(host: uri.host, port: port, secure: secure);
    }
    final index = text.lastIndexOf(':');
    if (index > 0 && index < text.length - 1) {
      final host = text.substring(0, index);
      final port = int.tryParse(text.substring(index + 1));
      if (port == null || port <= 0 || port > 65535) return null;
      return ServerAddress(host: host, port: port);
    }
    if (text.contains(':')) return null;
    return ServerAddress(host: text, port: 8443);
  }
}

/// 服务端 `pair` 命令输出的配对信息（也可以从链接/二维码里来）。
class PairPayload {
  const PairPayload({
    required this.host,
    required this.port,
    required this.fingerprint,
    required this.ticket,
    required this.nonce,
    required this.expiresAt,
    this.secure = true,
  });

  final String host;
  final int port;
  final String fingerprint;
  final String ticket;
  final String nonce;
  final int expiresAt;
  final bool secure;

  ServerAddress get address =>
      ServerAddress(host: host, port: port, secure: secure);

  bool get expired =>
      expiresAt > 0 && expiresAt * 1000 < DateTime.now().millisecondsSinceEpoch;
}

/// 解析粘贴进来的配对信息：裸 JSON、base64url、或带 `#pair=`/`?pair=` 的链接。
PairPayload? parsePairInput(String input) {
  final raw = input.trim();
  if (raw.isEmpty) return null;

  final candidates = <String>[raw];
  final match = RegExp(r'[#?&]pair=([^&\s]+)').firstMatch(raw);
  if (match != null) {
    candidates.add(Uri.decodeComponent(match.group(1)!));
  }

  for (final candidate in candidates) {
    final text = candidate.trim();
    if (text.isEmpty) continue;
    final decoded = _decodeCandidate(text);
    if (decoded == null) continue;
    final payload = _fromJson(decoded);
    if (payload != null) return payload;
  }
  return null;
}

Map<String, dynamic>? _decodeCandidate(String text) {
  final direct = _tryJson(text);
  if (direct != null) return direct;
  try {
    final normalized = base64Url.normalize(text);
    final bytes = base64Decode(normalized);
    return _tryJson(utf8.decode(bytes));
  } on FormatException {
    return null;
  }
}

Map<String, dynamic>? _tryJson(String text) {
  if (!text.startsWith('{')) return null;
  try {
    final decoded = jsonDecode(text);
    return decoded is Map<String, dynamic> ? decoded : null;
  } on FormatException {
    return null;
  }
}

PairPayload? _fromJson(Map<String, dynamic> json) {
  final host = json['host'];
  final ticket = json['ticket'];
  if (host is! String || host.isEmpty || ticket is! String || ticket.isEmpty) {
    return null;
  }
  final port = json['port'] is num ? (json['port'] as num).toInt() : 8443;
  final exp = json['exp'] is num ? (json['exp'] as num).toInt() : 0;
  return PairPayload(
    host: host,
    port: port,
    fingerprint: json['fp'] as String? ?? '',
    ticket: ticket,
    nonce: json['nonce'] as String? ?? '',
    expiresAt: exp,
  );
}

/// 本机身份：配对成功后落盘，之后每次启动都靠它连服务。
class Identity {
  Identity({
    required this.deviceId,
    required this.serviceName,
    required this.host,
    required this.port,
    required this.fingerprint,
    required this.secure,
    required this.privateKey,
    required this.publicPoint,
  });

  final String deviceId;
  final String serviceName;
  final String host;
  final int port;
  final String fingerprint;
  final bool secure;
  final BigInt privateKey;
  final EcPoint publicPoint;

  ServerAddress get address =>
      ServerAddress(host: host, port: port, secure: secure);

  /// base64(PKIX DER)，协议里的 `manager_pub`。
  String get publicKeyBase64 => p256EncodeSpkiBase64(publicPoint);

  EcKeyPair get keyPair => EcKeyPair(privateKey, publicPoint);

  String get fingerprintShort {
    if (fingerprint.length <= 17) return fingerprint;
    return '${fingerprint.substring(0, 17)}…';
  }

  Identity copyWith({String? deviceId, String? serviceName}) => Identity(
        deviceId: deviceId ?? this.deviceId,
        serviceName: serviceName ?? this.serviceName,
        host: host,
        port: port,
        fingerprint: fingerprint,
        secure: secure,
        privateKey: privateKey,
        publicPoint: publicPoint,
      );

  Map<String, dynamic> toJson() => <String, dynamic>{
        'device_id': deviceId,
        'service_name': serviceName,
        'host': host,
        'port': port,
        'fingerprint': fingerprint,
        'secure': secure,
        'private_key': base64Encode(_toBytes(privateKey, 32)),
      };

  static Identity? fromJson(Map<String, dynamic> json) {
    try {
      final privateKey = _fromBytes(base64Decode(json['private_key'] as String));
      final pair = p256FromPrivateKey(privateKey);
      return Identity(
        deviceId: json['device_id'] as String? ?? '',
        serviceName: json['service_name'] as String? ?? 'LianT Service',
        host: json['host'] as String? ?? '',
        port: json['port'] is num ? (json['port'] as num).toInt() : 8443,
        fingerprint: json['fingerprint'] as String? ?? '',
        secure: json['secure'] is bool ? json['secure'] as bool : true,
        privateKey: privateKey,
        publicPoint: pair.pub,
      );
    } on FormatException {
      return null;
    } on ArgumentError {
      return null;
    }
  }
}

BigInt _fromBytes(List<int> bytes) {
  var value = BigInt.zero;
  for (final b in bytes) {
    value = (value << 8) | BigInt.from(b);
  }
  return value;
}

Uint8List _toBytes(BigInt value, int length) {
  final out = Uint8List(length);
  var v = value;
  for (var i = length - 1; i >= 0; i--) {
    out[i] = (v & BigInt.from(0xff)).toInt();
    v = v >> 8;
  }
  return out;
}

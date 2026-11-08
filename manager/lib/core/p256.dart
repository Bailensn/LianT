// P-256（secp256r1）上的 ECDSA：客户端用它证明设备身份、校验服务端自证。
//
// 为什么不用现成插件：
//   * 浏览器里 Web Crypto 的私钥不可导出，Flutter 统一 API 拿不到；
//   * Android Keystore 各厂商行为不一，v1 先保证 Web 与 Android 同一条代码路径；
//   * 签名算法在协议 v1 里已经冻结（ECDSA P-256 + SHA-256，ASN.1 DER 后 base64），
//     自己实现反而更容易与 Go 的 ecdsa.VerifyASN1 对齐。
//
// 这里的实现没有做常数时间优化：私钥只在本机内存里用来算一次签名，
// 时序侧信道需要攻击者已能在设备上高频测量，超出了 v1 的威胁模型。
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto;

/// 椭圆曲线上的点（仿射坐标）。
class EcPoint {
  const EcPoint(this.x, this.y);

  final BigInt x;
  final BigInt y;
}

/// 一对 P-256 密钥。私钥是 [d]，公钥是 [pub]。
class EcKeyPair {
  const EcKeyPair(this.d, this.pub);

  final BigInt d;
  final EcPoint pub;

  /// base64(PKIX DER SubjectPublicKeyInfo)，协议里 `manager_pub` 的形状。
  String get publicKeyBase64 => base64Encode(p256EncodeSpki(pub));
}

// ---------------------------------------------------------------------------
// 曲线参数（FIPS 186-4 / SEC 2）
// ---------------------------------------------------------------------------

final BigInt _p = BigInt.parse(
  'ffffffff00000001000000000000000000000000ffffffffffffffffffffffff',
  radix: 16,
);
final BigInt _n = BigInt.parse(
  'ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551',
  radix: 16,
);
final BigInt _a = _p - BigInt.from(3);
final BigInt _b = BigInt.parse(
  '5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b',
  radix: 16,
);
final BigInt _gx = BigInt.parse(
  '6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296',
  radix: 16,
);
final BigInt _gy = BigInt.parse(
  '4fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5',
  radix: 16,
);

final BigInt _zero = BigInt.zero;
final BigInt _one = BigInt.one;
final BigInt _two = BigInt.two;
final BigInt _three = BigInt.from(3);
final BigInt _four = BigInt.from(4);
final BigInt _eight = BigInt.from(8);

/// PKIX SubjectPublicKeyInfo 的固定前缀，止于 BIT STRING 的内容首字节之前。
const List<int> _spkiPrefix = <int>[
  0x30, 0x59, // SEQUENCE, 89 字节
  0x30, 0x13, // SEQUENCE, 19 字节
  0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, // OID 1.2.840.10045.2.1
  0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, // OID 1.2.840.10045.3.1.7
  0x03, 0x42, 0x00, // BIT STRING, 66 字节, 0 个未用位
];

const List<int> _oidEcPublicKey = <int>[
  0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01,
];
const List<int> _oidPrime256v1 = <int>[
  0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07,
];

// ---------------------------------------------------------------------------
// 公开接口
// ---------------------------------------------------------------------------

/// 生成一对新的 P-256 密钥。
EcKeyPair p256GenerateKey([Random? rng]) {
  final random = rng ?? Random.secure();
  final d = _randomScalar(random);
  final p = _toAffine(_multiply(d, _gx, _gy));
  if (p == null) {
    // 概率上不可能发生（d 的阶就是 n），保底递归一次。
    return p256GenerateKey(random);
  }
  return EcKeyPair(d, p);
}

/// 从私钥恢复密钥对（用于本地保存后重新加载）。
EcKeyPair p256FromPrivateKey(BigInt d) {
  if (d <= _zero || d >= _n) {
    throw ArgumentError('私钥超出 P-256 标量范围');
  }
  final p = _toAffine(_multiply(d, _gx, _gy));
  if (p == null) throw StateError('私钥无法推导公钥');
  return EcKeyPair(d, p);
}

/// ECDSA-SHA256 签名，输出 ASN.1 DER（与 Go 的 ecdsa.VerifyASN1 对齐）。
Uint8List p256Sign(EcKeyPair key, List<int> message, [Random? rng]) {
  final random = rng ?? Random.secure();
  final e = _hashToInt(message);
  while (true) {
    final k = _randomScalar(random);
    final point = _toAffine(_multiply(k, _gx, _gy));
    if (point == null) continue;
    final r = point.x % _n;
    if (r == _zero) continue;
    final s = _modN(k.modInverse(_n) * (e + r * key.d));
    if (s == _zero) continue;
    return _encodeSignature(r, s);
  }
}

/// 校验签名；任何格式问题都返回 false，不抛异常。
bool p256Verify(EcPoint pub, List<int> message, Uint8List signature) {
  final parsed = _decodeSignature(signature);
  if (parsed == null) return false;
  final (r, s) = parsed;
  if (r <= _zero || r >= _n || s <= _zero || s >= _n) return false;
  final e = _hashToInt(message);
  final w = s.modInverse(_n);
  final u1 = _modN(e * w);
  final u2 = _modN(r * w);
  final sum = _addJ(_multiply(u1, _gx, _gy), _multiply(u2, pub.x, pub.y));
  final point = _toAffine(sum);
  if (point == null) return false;
  return point.x % _n == r;
}

/// base64(PKIX DER)。
String p256EncodeSpkiBase64(EcPoint pub) => base64Encode(p256EncodeSpki(pub));

/// PKIX DER 字节。
Uint8List p256EncodeSpki(EcPoint pub) {
  final out = BytesBuilder(copy: false);
  out.add(_spkiPrefix);
  out.addByte(0x04); // 未压缩点
  out.add(_toBytes(pub.x, 32));
  out.add(_toBytes(pub.y, 32));
  return out.toBytes();
}

/// 解析 PKIX DER 里的公钥点；结构不合法或不在曲线上返回 null。
EcPoint? p256DecodeSpki(Uint8List der) {
  try {
    final r = _DerReader(der);
    r.expect(0x30);
    final outerLen = r.readLength();
    if (r.offset + outerLen != der.length) return null;

    r.expect(0x30);
    final algLen = r.readLength();
    final algEnd = r.offset + algLen;
    r.expect(0x06);
    if (!_listEquals(r.readValue(r.readLength()), _oidEcPublicKey)) return null;
    r.expect(0x06);
    if (!_listEquals(r.readValue(r.readLength()), _oidPrime256v1)) return null;
    if (r.offset != algEnd) return null;

    r.expect(0x03);
    final bits = r.readValue(r.readLength());
    if (bits.length != 66 || bits[0] != 0x00 || bits[1] != 0x04) return null;
    if (r.offset != der.length) return null;
    final x = _fromBytes(bits.sublist(2, 34));
    final y = _fromBytes(bits.sublist(34, 66));
    if (!_isOnCurve(x, y)) return null;
    return EcPoint(x, y);
  } on FormatException {
    return null;
  } on RangeError {
    return null;
  }
}

/// base32(SHA-256(DER))，每 4 位一组用连字符分开，与服务端指纹一致。
String p256Fingerprint(Uint8List spkiDer) =>
    _base32Groups(crypto.sha256.convert(spkiDer).bytes);

/// 校验服务端自证：指纹必须与配对时钉住的一致，签名必须由该公钥签出。
///
/// 抛出 [StateError]，调用方据此拒绝连接（不提供「忽略」开关，协议 §10.1）。
void p256VerifyServiceProof({
  required String servicePubBase64,
  required String expectedFingerprint,
  required List<int> message,
  required String signatureBase64,
}) {
  final Uint8List der;
  try {
    der = base64Decode(servicePubBase64);
  } on FormatException {
    throw StateError('服务端公钥不是合法的 base64');
  }
  final fp = p256Fingerprint(der);
  if (expectedFingerprint.isNotEmpty && fp != expectedFingerprint) {
    throw StateError('服务端身份指纹不匹配，连接可能被中间人劫持');
  }
  final pub = p256DecodeSpki(der);
  if (pub == null) throw StateError('服务端公钥格式不正确');
  final Uint8List sig;
  try {
    sig = base64Decode(signatureBase64);
  } on FormatException {
    throw StateError('服务端签名不是合法的 base64');
  }
  if (!p256Verify(pub, message, sig)) {
    throw StateError('服务端身份签名无效');
  }
}

// ---------------------------------------------------------------------------
// 曲线运算（Jacobian 坐标，减少一次标量乘里的求逆次数）
// ---------------------------------------------------------------------------

class _JPoint {
  const _JPoint(this.x, this.y, this.z);

  final BigInt x;
  final BigInt y;
  final BigInt z;

  bool get isInfinity => z == _zero;

  // BigInt 不是常量，只能声明成 final。
  static final _JPoint infinity = _JPoint(_one, _one, _zero);
}

BigInt _modP(BigInt v) => v % _p;

BigInt _modN(BigInt v) => v % _n;

BigInt _hashToInt(List<int> message) =>
    _fromBytes(crypto.sha256.convert(message).bytes);

_JPoint _doubleJ(_JPoint p) {
  if (p.isInfinity || p.y == _zero) return _JPoint.infinity;
  final y2 = _modP(p.y * p.y);
  final s = _modP(_four * p.x * y2);
  final z2 = _modP(p.z * p.z);
  final m = _modP(_three * p.x * p.x + _a * z2 * z2);
  final x3 = _modP(m * m - _two * s);
  final y4 = _modP(y2 * y2);
  final y3 = _modP(m * (s - x3) - _eight * y4);
  final z3 = _modP(_two * p.y * p.z);
  return _JPoint(x3, y3, z3);
}

_JPoint _addJ(_JPoint p, _JPoint q) {
  if (p.isInfinity) return q;
  if (q.isInfinity) return p;
  final z1z1 = _modP(p.z * p.z);
  final z2z2 = _modP(q.z * q.z);
  final u1 = _modP(p.x * z2z2);
  final u2 = _modP(q.x * z1z1);
  final s1 = _modP(p.y * q.z * z2z2);
  final s2 = _modP(q.y * p.z * z1z1);
  if (u1 == u2) {
    if (s1 != s2) return _JPoint.infinity;
    return _doubleJ(p);
  }
  final h = _modP(u2 - u1);
  final rr = _modP(s2 - s1);
  final h2 = _modP(h * h);
  final h3 = _modP(h2 * h);
  final u1h2 = _modP(u1 * h2);
  final x3 = _modP(rr * rr - h3 - _two * u1h2);
  final y3 = _modP(rr * (u1h2 - x3) - s1 * h3);
  final z3 = _modP(h * p.z * q.z);
  return _JPoint(x3, y3, z3);
}

/// k·(x, y)，返回 Jacobian 坐标。
_JPoint _multiply(BigInt k, BigInt x, BigInt y) {
  var result = _JPoint.infinity;
  var addend = _JPoint(x, y, _one);
  var e = k;
  while (e > _zero) {
    if (e.isOdd) result = _addJ(result, addend);
    addend = _doubleJ(addend);
    e = e >> 1;
  }
  return result;
}

EcPoint? _toAffine(_JPoint p) {
  if (p.isInfinity) return null;
  final zInv = p.z.modPow(_p - _two, _p);
  final zInv2 = _modP(zInv * zInv);
  final x = _modP(p.x * zInv2);
  final y = _modP(p.y * zInv2 * zInv);
  return EcPoint(x, y);
}

bool _isOnCurve(BigInt x, BigInt y) {
  if (x < _zero || x >= _p || y < _zero || y >= _p) return false;
  final left = _modP(y * y);
  final right = _modP(x * x * x + _a * x + _b);
  return left == right;
}

// ---------------------------------------------------------------------------
// 编码与解析
// ---------------------------------------------------------------------------

BigInt _randomScalar(Random random) {
  while (true) {
    final bytes = Uint8List(32);
    for (var i = 0; i < bytes.length; i++) {
      bytes[i] = random.nextInt(256);
    }
    final value = _fromBytes(bytes) % (_n - _one) + _one;
    if (value > _zero && value < _n) return value;
  }
}

BigInt _fromBytes(List<int> bytes) {
  var value = _zero;
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

Uint8List _minimalBytes(BigInt value) {
  if (value == _zero) return Uint8List(0);
  final bytes = <int>[];
  var v = value;
  while (v > _zero) {
    bytes.insert(0, (v & BigInt.from(0xff)).toInt());
    v = v >> 8;
  }
  return Uint8List.fromList(bytes);
}

Uint8List _encodeSignature(BigInt r, BigInt s) {
  final rEnc = _encodeDerInteger(r);
  final sEnc = _encodeDerInteger(s);
  final body = <int>[...rEnc, ...sEnc];
  return Uint8List.fromList(<int>[0x30, ..._encodeDerLength(body.length), ...body]);
}

Uint8List _encodeDerInteger(BigInt value) {
  var bytes = _minimalBytes(value);
  if (bytes.isEmpty) bytes = Uint8List.fromList(<int>[0]);
  if ((bytes[0] & 0x80) != 0) {
    bytes = Uint8List.fromList(<int>[0, ...bytes]);
  }
  return Uint8List.fromList(<int>[0x02, ..._encodeDerLength(bytes.length), ...bytes]);
}

List<int> _encodeDerLength(int length) {
  if (length < 0x80) return <int>[length];
  final out = <int>[];
  var v = length;
  while (v > 0) {
    out.insert(0, v & 0xff);
    v >>= 8;
  }
  return <int>[0x80 | out.length, ...out];
}

(BigInt, BigInt)? _decodeSignature(Uint8List der) {
  try {
    final r = _DerReader(der);
    r.expect(0x30);
    final len = r.readLength();
    if (r.offset + len != der.length) return null;
    r.expect(0x02);
    final rValue = _fromBytes(r.readValue(r.readLength()));
    r.expect(0x02);
    final sValue = _fromBytes(r.readValue(r.readLength()));
    if (r.offset != der.length) return null;
    return (rValue, sValue);
  } on FormatException {
    return null;
  } on RangeError {
    return null;
  }
}

class _DerReader {
  _DerReader(this.bytes);

  final Uint8List bytes;
  int offset = 0;

  int _byte() {
    if (offset >= bytes.length) throw const FormatException('DER 数据被截断');
    return bytes[offset++];
  }

  void expect(int tag) {
    if (_byte() != tag) throw const FormatException('DER 标签不匹配');
  }

  int readLength() {
    final first = _byte();
    if ((first & 0x80) == 0) return first;
    final count = first & 0x7f;
    if (count == 0 || count > 4) throw const FormatException('DER 长度不合法');
    var length = 0;
    for (var i = 0; i < count; i++) {
      length = (length << 8) | _byte();
    }
    if (length < 0 || offset + length > bytes.length) {
      throw const FormatException('DER 长度越界');
    }
    return length;
  }

  Uint8List readValue(int length) {
    if (offset + length > bytes.length) {
      throw const FormatException('DER 内容被截断');
    }
    final value = Uint8List.sublistView(bytes, offset, offset + length);
    offset += length;
    return Uint8List.fromList(value);
  }
}

bool _listEquals(List<int> a, List<int> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

/// RFC 4648 base32，无填充、大写、每 4 位插一个连字符。
String _base32Groups(List<int> bytes) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  var bits = 0;
  var value = 0;
  final out = StringBuffer();
  for (final b in bytes) {
    value = (value << 8) | b;
    bits += 8;
    while (bits >= 5) {
      out.write(alphabet[(value >> (bits - 5)) & 31]);
      bits -= 5;
    }
  }
  if (bits > 0) {
    out.write(alphabet[(value << (5 - bits)) & 31]);
  }
  final raw = out.toString();
  final grouped = <String>[];
  for (var i = 0; i < raw.length; i += 4) {
    grouped.add(raw.substring(i, i + 4 > raw.length ? raw.length : i + 4));
  }
  return grouped.join('-');
}



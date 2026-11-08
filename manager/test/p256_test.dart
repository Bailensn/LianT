// P-256 与配对信息的单元测试。签名向量由 openssl 生成，确保编码与 Go 侧一致。
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:liant/core/identity.dart';
import 'package:liant/core/p256.dart';

// openssl ecparam -name prime256v1 -genkey / dgst -sha256 -sign 生成。
const _opensslPubDer =
    'MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEOOifwDa/2e9az66MFEL5Vkupoye/SWCrvwGvvASDoatsL8JqbQNJTd3nzz8swF8dlg95TzJojOyTmGkoshLCOQ==';
const _opensslSig =
    'MEUCIEpXfIFy7LnzSQ4dAyfLsoJ8pMF2+40bqWgY+cSUiKjBAiEAssAxKL8qXY2aQYly1R51VVHMPG3kjWXjZwa4j8SWxlg=';
const _message = 'hello liant p256\n';

void main() {
  test('openssl 生成的 P-256 签名可以验过', () {
    final der = base64Decode(_opensslPubDer);
    final pub = p256DecodeSpki(der);
    expect(pub, isNotNull);
    expect(p256Verify(pub!, utf8.encode(_message), base64Decode(_opensslSig)), isTrue);
    expect(p256Verify(pub, utf8.encode('被篡改的内容'), base64Decode(_opensslSig)), isFalse);
  });

  test('自己的签名能验过，指纹格式稳定', () {
    final key = p256GenerateKey();
    final message = utf8.encode('nonce\nfp\n1793520000123');
    final signature = p256Sign(key, message);
    expect(p256Verify(key.pub, message, signature), isTrue);
    expect(p256Verify(key.pub, utf8.encode('other'), signature), isFalse);

    final der = p256EncodeSpki(key.pub);
    final fingerprint = p256Fingerprint(der);
    expect(RegExp(r'^([A-Z2-7]{4}-){12}[A-Z2-7]{4}$').hasMatch(fingerprint), isTrue);
    // 公钥 DER 是 91 字节的固定结构。
    expect(der.length, 91);
  });

  test('配对信息支持裸 JSON、base64 与链接片段', () {
    const json = '{"v":1,"host":"192.168.1.10","port":8443,"fp":"ABCD",'
        '"ticket":"123456","nonce":"n1","exp":0}';
    final direct = parsePairInput(json);
    expect(direct, isNotNull);
    expect(direct!.host, '192.168.1.10');
    expect(direct.ticket, '123456');

    final encoded = base64Encode(utf8.encode(json));
    expect(parsePairInput(encoded)?.nonce, 'n1');
    expect(parsePairInput('https://lt.example/#pair=$encoded')?.host, '192.168.1.10');
    expect(parsePairInput('这不是配对信息'), isNull);
  });

  test('服务地址支持主机:端口与 ws/wss 链接', () {
    expect(ServerAddress.parse('127.0.0.1:8099')?.wsUrl, 'wss://127.0.0.1:8099/v1/ws');
    final plain = ServerAddress.parse('ws://127.0.0.1:8099');
    expect(plain, isNotNull);
    expect(plain!.secure, isFalse);
    expect(plain.wsUrl, 'ws://127.0.0.1:8099/v1/ws');
    expect(ServerAddress.parse('lt.example')?.port, 8443);
  });
}

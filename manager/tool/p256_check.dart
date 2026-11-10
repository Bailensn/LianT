// P-256 实现的交叉校验脚本（不参与打包）。
//
// 用法：
//   dart run tool/p256_check.dart sign <message-file> <pub.der> <sig.bin>
//   dart run tool/p256_check.dart verify <pub.der> <sig.bin> <message-file>
//
// `sign` 用本仓库的实现签名并写出 SPKI DER 与 DER 签名，交给 openssl 验签；
// `verify` 校验 openssl 生成的签名，确认两边编码完全对齐。
import 'dart:convert';
import 'dart:io';

import 'package:liant/core/identity/p256.dart';

Future<void> main(List<String> args) async {
  if (args.length != 4) {
    stderr.writeln('参数不正确');
    exit(2);
  }
  final mode = args[0];
  if (mode == 'sign') {
    final message = await File(args[1]).readAsBytes();
    final key = p256GenerateKey();
    final signature = p256Sign(key, message);
    await File(args[2]).writeAsBytes(p256EncodeSpki(key.pub));
    await File(args[3]).writeAsBytes(signature);
    stdout.writeln('fingerprint: ${p256Fingerprint(p256EncodeSpki(key.pub))}');
    stdout.writeln('pub: ${key.publicKeyBase64}');
    stdout.writeln('sig: ${base64Encode(signature)}');
    return;
  }
  if (mode == 'verify') {
    final der = await File(args[1]).readAsBytes();
    final signature = await File(args[2]).readAsBytes();
    final message = await File(args[3]).readAsBytes();
    final pub = p256DecodeSpki(der);
    if (pub == null) {
      stderr.writeln('公钥解析失败');
      exit(1);
    }
    final ok = p256Verify(pub, message, signature);
    stdout.writeln(ok ? 'verify: ok' : 'verify: failed');
    exit(ok ? 0 : 1);
  }
  stderr.writeln('未知模式 $mode');
  exit(2);
}

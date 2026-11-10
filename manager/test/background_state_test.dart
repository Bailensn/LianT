// 后台进程共享状态与通知 ID 的单元测试。
//
// 这两块是跨进程正确性的关键：状态文件要能原样读回，通知 ID 在
// UI 进程和 :background 进程里必须算出同一个值（否则同一条消息会弹两次）。
import 'package:flutter_test/flutter_test.dart';
import 'package:liant/core/background/state.dart';
import 'package:liant/core/identity/identity.dart';
import 'package:liant/core/identity/p256.dart';
import 'package:liant/core/net/protocol.dart';

Identity _identity() {
  final key = p256GenerateKey();
  return Identity(
    deviceId: 'dev-1',
    serviceName: 'LianT Service',
    host: '10.0.0.2',
    port: 8443,
    fingerprint: 'aa:bb:cc',
    secure: true,
    privateKey: key.d,
    publicPoint: key.pub,
  );
}

void main() {
  test('后台状态 JSON 原样往返', () {
    final state = BackgroundState(
      identity: _identity(),
      foreground: false,
      muted: <String, bool>{'bot1|user1': true},
      titles: <String, String>{'bot1|user1': '运营群'},
      remarks: <String, String>{'bot1|user1': '我的备注'},
    );
    final restored = BackgroundState.fromJson(state.toJson());
    expect(restored, isNotNull);
    final back = restored!;
    expect(back.identity.deviceId, 'dev-1');
    expect(back.identity.host, '10.0.0.2');
    // 私钥也要能读回来，否则后台进程签不出 auth.response。
    expect(back.identity.privateKey, state.identity.privateKey);
    expect(back.foreground, isFalse);
    expect(back.muted['bot1|user1'], isTrue);
    expect(back.titles['bot1|user1'], '运营群');
    expect(back.remarks['bot1|user1'], '我的备注');
  });

  test('版本号不认识的旧状态直接丢弃', () {
    final json = BackgroundState(
      identity: _identity(),
      foreground: true,
      muted: const <String, bool>{},
      titles: const <String, String>{},
      remarks: const <String, String>{},
    ).toJson();
    json['v'] = 999;
    expect(BackgroundState.fromJson(json), isNull);
  });

  test('通知 ID 稳定且落在正数范围', () {
    const msgId = 'msg-0f9c8b7a-1';
    final id = notificationIdFor(msgId);
    expect(id, notificationIdFor(msgId));
    expect(id, greaterThan(0));
    expect(id, lessThan(0x80000000));
    expect(notificationIdFor('$msgId-x'), isNot(id));
  });
}

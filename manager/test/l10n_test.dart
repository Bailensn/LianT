// 翻译表结构 + 查表行为的回归测试。
//
// 表按语言拆成 strings/*.dart，任何一张表少一条都会在这里暴露。
import 'package:flutter_test/flutter_test.dart';
import 'package:liant/l10n/l10n.dart';
import 'package:liant/l10n/strings/en.dart';
import 'package:liant/l10n/strings/ja.dart';
import 'package:liant/l10n/strings/ko.dart';
import 'package:liant/l10n/strings/zh_hant.dart';

void main() {
  tearDown(() => L10n.setSetting('zh'));

  test('四种语言的词条与中文源一一对应', () {
    final keys = zhHantExact.keys.toSet();
    expect(keys.length, greaterThan(150));
    expect(enExact.keys.toSet(), keys);
    expect(jaExact.keys.toSet(), keys);
    expect(koExact.keys.toSet(), keys);

    final phraseKeys = zhHantPhrases.keys.toSet();
    expect(enPhrases.keys.toSet(), phraseKeys);
    expect(jaPhrases.keys.toSet(), phraseKeys);
    expect(koPhrases.keys.toSet(), phraseKeys);
  });

  test('整句查表', () {
    L10n.setSetting('en');
    expect(L10n.tr('设置'), 'Settings');
    L10n.setSetting('ja');
    expect(L10n.tr('设置'), '設定');
    L10n.setSetting('ko');
    expect(L10n.tr('设置'), '설정');
  });

  test('中文是原文，原样返回', () {
    L10n.setSetting('zh');
    expect(L10n.tr('设置'), '设置');
    expect(L10n.tr('这句没进翻译表'), '这句没进翻译表');
  });

  test('查不到整句时按片段替换', () {
    L10n.setSetting('en');
    expect(L10n.tr('3 个机器人正在运行'), contains('bots running'));
    expect(L10n.tr('3 个机器人正在运行'), contains('3'));
  });

  test('语言名用各自写法，未知值回落到「跟随系统」', () {
    expect(L10n.label('en'), 'English');
    expect(L10n.label('zh_Hant'), '繁體中文');
    L10n.setSetting('en');
    expect(L10n.label('system'), 'System');
    expect(L10n.label('unknown'), 'System');
  });

  test('resolve 对固定值原样返回，对 system 给出受支持语言', () {
    for (final code in <String>['zh', 'zh_Hant', 'en', 'ja', 'ko']) {
      expect(L10n.resolve(code), code);
    }
    expect(L10n.resolve('system'), isIn(L10n.settings));
  });
}

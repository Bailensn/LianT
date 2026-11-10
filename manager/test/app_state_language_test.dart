// 语言设置持久化：之前只写不读，重启后总是回到「跟随系统」。
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:liant/core/app_state.dart';
import 'package:liant/l10n/l10n.dart';
import 'package:shared_preferences/shared_preferences.dart';

Future<AppState> _boot(Map<String, Object> prefs) async {
  SharedPreferences.setMockInitialValues(prefs);
  final store = await SharedPreferences.getInstance();
  final state = AppState(store);
  state.boot();
  return state;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('启动时恢复已保存的语言', () async {
    final state = await _boot(<String, Object>{'liant.language': 'ja'});
    expect(state.languageSetting, 'ja');
    expect(state.locale, const Locale('ja'));
    expect(L10n.code, 'ja');
  });

  test('繁体中文的 BCP47 映射不丢', () async {
    final state = await _boot(<String, Object>{'liant.language': 'zh_Hant'});
    expect(state.languageSetting, 'zh_Hant');
    expect(state.locale, const Locale.fromSubtags(languageCode: 'zh', scriptCode: 'Hant'));
  });

  test('没有保存过就跟随系统', () async {
    final state = await _boot(<String, Object>{});
    expect(state.languageSetting, 'system');
  });

  test('不认识的语言值被忽略，回落到跟随系统', () async {
    final state = await _boot(<String, Object>{'liant.language': 'klingon'});
    expect(state.languageSetting, 'system');
  });

  test('beta 开关也一起恢复', () async {
    final state = await _boot(<String, Object>{'liant.acceptBeta': true});
    expect(state.acceptBeta, isTrue);
  });
}

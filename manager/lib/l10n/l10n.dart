import 'dart:ui' as ui;

import 'strings/en.dart';
import 'strings/ja.dart';
import 'strings/ko.dart';
import 'strings/zh_hant.dart';

/// 5 种语言 + 跟随系统。翻译表用中文原文做 key，找不到时回退中文。
///
/// 语言表在 `strings/` 下，一种语言一个文件（中文是原文，不需要表）。
/// 加语言：新建 `strings/xx.dart`，再在下面的 [_exact] / [_phrases] 注册。
class L10n {
  static const List<String> settings = <String>[
    'system',
    'zh',
    'zh_Hant',
    'en',
    'ja',
    'ko',
  ];

  /// 当前生效的语言代码（`zh` / `zh_Hant` / `en` / `ja` / `ko`）。
  static String code = 'zh';

  /// 把「设置里的选择」解析成实际语言；`system` 会读平台语言。
  static String resolve(String setting) {
    switch (setting) {
      case 'zh':
      case 'zh_Hant':
      case 'en':
      case 'ja':
      case 'ko':
        return setting;
    }
    final locale = ui.PlatformDispatcher.instance.locale;
    final lang = locale.languageCode;
    if (lang == 'zh') {
      final country = locale.countryCode;
      return (country == 'TW' || country == 'HK' || country == 'MO')
          ? 'zh_Hant'
          : 'zh';
    }
    if (lang == 'en' || lang == 'ja' || lang == 'ko') return lang;
    return 'zh';
  }

  static void setSetting(String setting) {
    code = resolve(setting);
  }

  /// 语言名（用该语言自己的写法），设置页与语言列表共用。
  static String label(String setting) {
    switch (setting) {
      case 'zh':
        return '简体中文';
      case 'zh_Hant':
        return '繁體中文';
      case 'en':
        return 'English';
      case 'ja':
        return '日本語';
      case 'ko':
        return '한국어';
      default:
        return tr('跟随系统');
    }
  }

  /// 把中文原文翻成当前语言；整句命中优先，否则按片段替换。
  static String tr(String text) {
    if (text.isEmpty) return text;
    final exact = _exact[code];
    if (exact == null) return text; // zh：原文就是译文
    final hit = exact[text];
    if (hit != null) return hit;
    var out = text;
    final phrases = _phrases[code];
    if (phrases == null) return out;
    for (final entry in phrases.entries) {
      if (entry.key.isEmpty) continue;
      out = out.replaceAll(entry.key, entry.value);
    }
    return out;
  }

  static const Map<String, Map<String, String>> _exact =
      <String, Map<String, String>>{
    'zh_Hant': zhHantExact,
    'en': enExact,
    'ja': jaExact,
    'ko': koExact,
  };

  static const Map<String, Map<String, String>> _phrases =
      <String, Map<String, String>>{
    'zh_Hant': zhHantPhrases,
    'en': enPhrases,
    'ja': jaPhrases,
    'ko': koPhrases,
  };
}

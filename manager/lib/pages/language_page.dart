import 'package:flutter/material.dart';

import '../l10n/l10n.dart';
import '../l10n/ltext.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';

class LanguagePage extends StatelessWidget {
  const LanguagePage({super.key});

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    final options = <String>[
      'system',
      'zh',
      'zh_Hant',
      'en',
      'ja',
      'ko',
    ];
    return LargeTitlePage(
      title: L10n.tr('语言'),
      trailing: IconAction(
        icon: Icons.close_rounded,
        tooltip: L10n.tr('返回'),
        onTap: () => Navigator.of(context).maybePop(),
      ),
      children: <Widget>[
        GroupCard(
          title: L10n.tr('选择语言'),
          children: <Widget>[
            for (final code in options)
              InkWell(
                onTap: () {
                  state.setLanguage(code);
                  Navigator.of(context).maybePop();
                },
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
                  child: Row(
                    children: <Widget>[
                      Expanded(
                        child: LText(
                          code == 'system' ? L10n.tr('跟随系统') : L10n.label(code),
                          style: context.texts.bodyLarge,
                        ),
                      ),
                      if (state.languageSetting == code)
                        Icon(Icons.check_rounded, size: 20, color: colors.accent),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ],
    );
  }
}

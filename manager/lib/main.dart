// LianT 客户端入口：配对页与三栏主壳。
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'core/app_state.dart';
import 'core/client.dart';
import 'pages/bots_page.dart';
import 'pages/chats_page.dart';
import 'pages/pair_page.dart';
import 'pages/settings_page.dart';
import 'ui/tokens.dart';
import 'ui/widgets.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // Web 版默认要用户点一下才开启语义树；这里直接打开，
  // 无障碍、AI 浏览器的 DOM 快照与自动化都依赖它。
  if (kIsWeb) {
    SemanticsBinding.instance.ensureSemantics();
  }
  // 本地身份很小，启动前同步读出来，避免先闪一下配对页。
  final prefs = await SharedPreferences.getInstance();
  final state = AppState(prefs)..boot();
  runApp(LiantApp(state: state));
}

class LiantApp extends StatelessWidget {
  const LiantApp({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return AppScope(
      state: state,
      child: MaterialApp(
        title: 'LianT',
        debugShowCheckedModeBanner: false,
        theme: LiantTheme.light(),
        darkTheme: LiantTheme.dark(),
        themeMode: ThemeMode.system,
        home: const _Root(),
      ),
    );
  }
}

class _Root extends StatelessWidget {
  const _Root();

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    if (state.identity == null) return const PairPage();
    return const Shell();
  }
}

/// 主壳：窄屏是悬浮玻璃底栏，宽屏是左侧 228px 导航。
class Shell extends StatefulWidget {
  const Shell({super.key});

  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  int _index = 0;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final pages = <Widget>[
      ChatsPage(onOpenBots: () => setState(() => _index = 1)),
      const BotsPage(),
      const SettingsPage(),
    ];

    return Scaffold(
      backgroundColor: colors.bg,
      body: LayoutBuilder(
        builder: (context, constraints) {
          if (constraints.maxWidth >= 768) {
            return SafeArea(
              child: Row(
                children: <Widget>[
                  _DesktopSidebar(
                    index: _index,
                    onTap: (value) => setState(() => _index = value),
                  ),
                  Container(width: 0.5, color: colors.sep),
                  Expanded(
                    child: Align(
                      alignment: Alignment.topCenter,
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 780),
                        child: IndexedStack(index: _index, children: pages),
                      ),
                    ),
                  ),
                ],
              ),
            );
          }
          return Stack(
            children: <Widget>[
              Positioned.fill(
                child: SafeArea(
                  bottom: false,
                  child: IndexedStack(index: _index, children: pages),
                ),
              ),
              Positioned(
                left: 16,
                right: 16,
                bottom: 16 + MediaQuery.paddingOf(context).bottom,
                child: GlassBottomNav(
                  index: _index,
                  onTap: (value) => setState(() => _index = value),
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _DesktopSidebar extends StatelessWidget {
  const _DesktopSidebar({required this.index, required this.onTap});

  final int index;
  final ValueChanged<int> onTap;

  static const List<(IconData, String)> _items = <(IconData, String)>[
    (Icons.chat_bubble_outline_rounded, '会话'),
    (Icons.smart_toy_outlined, '机器人'),
    (Icons.settings_outlined, '设置'),
  ];

  @override
  Widget build(BuildContext context) {
    final state = AppScope.of(context);
    final colors = context.colors;
    return SizedBox(
      width: 228,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 20, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Container(
                  width: 34,
                  height: 34,
                  alignment: Alignment.center,
                  decoration: BoxDecoration(
                    color: colors.accent,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: const Text(
                    'L',
                    style: TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.w600,
                      color: Colors.white,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                Text(
                  'LianT',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    letterSpacing: -0.3,
                    color: colors.text,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 28),
            for (var i = 0; i < _items.length; i++)
              Padding(
                padding: const EdgeInsets.only(bottom: 4),
                child: _SideItem(
                  icon: _items[i].$1,
                  label: _items[i].$2,
                  selected: i == index,
                  onTap: () => onTap(i),
                ),
              ),
            const Spacer(),
            Row(
              children: <Widget>[
                Container(
                  width: 7,
                  height: 7,
                  decoration: BoxDecoration(
                    color: switch (state.conn) {
                      ConnState.online => colors.ok,
                      ConnState.reconnecting || ConnState.error => colors.danger,
                      _ => colors.text3,
                    },
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    switch (state.conn) {
                      ConnState.online => '已连接',
                      ConnState.connecting => '连接中',
                      ConnState.reconnecting => '重连中',
                      ConnState.error => '连接失败',
                      ConnState.idle => '未连接',
                    },
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 12, color: colors.text3),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SideItem extends StatelessWidget {
  const _SideItem({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final color = selected ? colors.accent : colors.text2;
    return Material(
      color: selected ? colors.accent.withOpacity(0.12) : Colors.transparent,
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: onTap,
        child: SizedBox(
          height: 44,
          child: Row(
            children: <Widget>[
              const SizedBox(width: 12),
              Icon(icon, size: 20, color: color),
              const SizedBox(width: 10),
              Text(
                label,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: selected ? FontWeight.w600 : FontWeight.w500,
                  color: color,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

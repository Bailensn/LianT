// 配对页：填服务地址直接登记，或粘贴服务端 pair 命令输出的配对信息。
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import '../core/browser_env.dart';
import '../core/identity.dart';
import '../core/protocol.dart';
import '../ui/tokens.dart';
import '../ui/widgets.dart';
import '../l10n/l10n.dart';
import '../l10n/ltext.dart';
import '../ui/squircle.dart';
import 'qr_scan_page.dart';

class PairPage extends StatefulWidget {
  const PairPage({super.key});

  @override
  State<PairPage> createState() => _PairPageState();
}

class _PairPageState extends State<PairPage> {
  final TextEditingController _address = TextEditingController();
  final TextEditingController _pairText = TextEditingController();
  final TextEditingController _name = TextEditingController(text: '手机端');

  bool _autoBusy = false;
  bool _codeBusy = false;
  String _error = '';
  PairPayload? _parsed;

  @override
  void initState() {
    super.initState();
    _pairText.addListener(_reparse);
    // 网页预览时可以用 ?host=… 预填地址；自动连接只在
    // --dart-define=LIANT_PREVIEW=true 的预览构建里开启，
    // 避免一条链接就让正式客户端静默连上陌生服务。
    final preset = browserQueryParam('host');
    if (preset != null && preset.trim().isNotEmpty) {
      _address.text = preset.trim();
    }
    const preview = bool.fromEnvironment('LIANT_PREVIEW');
    if (preview && browserQueryParam('auto') == '1' && preset != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(_runAuto());
      });
    }
  }

  @override
  void dispose() {
    _address.dispose();
    _pairText.dispose();
    _name.dispose();
    super.dispose();
  }

  void _reparse() {
    final parsed = parsePairInput(_pairText.text);
    if (parsed == null && _parsed == null) return;
    setState(() => _parsed = parsed);
  }

  Future<void> _pastePair() async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    final text = data?.text;
    if (text == null || text.trim().isEmpty) {
      setState(() => _error = '剪贴板里没有可用的文本');
      return;
    }
    _pairText.text = text;
    _reparse();
    if (!mounted) return;
    if (_parsed == null) {
      setState(() => _error = '剪贴板内容不是有效的配对信息');
    } else {
      setState(() => _error = '');
    }
  }

  Future<void> _scanPair() async {
    final raw = await Navigator.of(context).push<String>(
      MaterialPageRoute<String>(builder: (_) => const QrScanPage()),
    );
    if (raw == null || raw.isEmpty) return;
    _pairText.text = raw;
    _reparse();
    if (!mounted) return;
    if (_parsed == null) {
      setState(() => _error = '二维码里不是有效的配对信息');
    } else {
      setState(() => _error = '');
    }
  }

  Future<void> _runAuto() async {
    final state = AppScope.of(context);
    final address = ServerAddress.parse(_address.text);
    if (address == null) {
      setState(() => _error = '服务地址格式不对，请填 主机:端口');
      return;
    }
    setState(() {
      _autoBusy = true;
      _error = '';
    });
    try {
      await state.pairAuto(address, _name.text);
    } on ProtocolException catch (err) {
      if (mounted) setState(() => _error = err.friendly);
    } catch (err) {
      if (mounted) setState(() => _error = '配对失败：$err');
    } finally {
      if (mounted) setState(() => _autoBusy = false);
    }
  }

  Future<void> _runCode() async {
    final state = AppScope.of(context);
    if (_parsed == null) {
      setState(() => _error = '这段内容不是有效的配对信息');
      return;
    }
    setState(() {
      _codeBusy = true;
      _error = '';
    });
    try {
      await state.pairWithCode(_pairText.text, _name.text);
    } on ProtocolException catch (err) {
      if (mounted) setState(() => _error = err.friendly);
    } catch (err) {
      if (mounted) setState(() => _error = '配对失败：$err');
    } finally {
      if (mounted) setState(() => _codeBusy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 520),
            child: ListView(
              padding: const EdgeInsets.fromLTRB(20, 32, 20, 32),
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Image.asset(
                      'assets/icon.png',
                      width: 56,
                      height: 56,
                      filterQuality: FilterQuality.high,
                    ),
                  ],
                ),
                const SizedBox(height: 20),
                LText(
                  '连接 LianT',
                  style: TextStyle(
                    fontSize: 32,
                    fontWeight: FontWeight.w700,
                    letterSpacing: -0.8,
                    height: 1.15,
                    color: colors.text,
                  ),
                ),
                const SizedBox(height: 8),
                LText(
                  '填服务地址直接登记，或在服务器上执行 LianT pair，把终端里的配对信息粘贴到下面。',
                  style: context.texts.bodyMedium?.copyWith(
                    color: colors.text2,
                    height: 1.6,
                  ),
                ),
                const SizedBox(height: 28),
                _AddressField(controller: _address),
                const SizedBox(height: 10),
                PrimaryButton(
                  label: _autoBusy ? '正在登记…' : '直接进入',
                  busy: _autoBusy,
                  onTap: _autoBusy ? null : _runAuto,
                ),
                const SizedBox(height: 8),
                LText(
                  '服务端开启登录认证时不用配对码：这台设备会直接登记。没有开启时请用下面的配对码。',
                  style: context.texts.labelSmall?.copyWith(
                    color: colors.text3,
                    height: 1.55,
                  ),
                ),
                const SizedBox(height: 28),
                Row(
                  children: <Widget>[
                    Expanded(child: Container(height: 0.5, color: colors.sep)),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      child: LText(
                        '或者用配对码',
                        style: context.texts.labelSmall?.copyWith(color: colors.text3),
                      ),
                    ),
                    Expanded(child: Container(height: 0.5, color: colors.sep)),
                  ],
                ),
                const SizedBox(height: 20),
                Container(
                  decoration: SquircleBoxDecoration(
                    color: colors.elevated,
                    borderRadius: BorderRadius.circular(LTokens.cardRadius),
                  ),
                  child: Column(
                    children: <Widget>[
                      Padding(
                        padding: const EdgeInsets.fromLTRB(14, 8, 14, 4),
                        child: TextField(
                          controller: _pairText,
                          maxLines: 5,
                          minLines: 4,
                          cursorColor: colors.accent,
                          style: TextStyle(
                            fontSize: 13,
                            height: 1.5,
                            fontFamily: 'monospace',
                            color: colors.text,
                          ),
                          decoration: InputDecoration(
                            hintText: L10n.tr('粘贴 LianT pair 打印的配对信息'),
                          ),
                        ),
                      ),
                      Container(height: 0.5, color: colors.sep),
                      Padding(
                        padding: const EdgeInsets.fromLTRB(14, 6, 6, 6),
                        child: Row(
                          children: <Widget>[
                            Expanded(
                              child: LText(
                                _parsed == null
                                    ? '等待配对信息'
                                    : '服务地址 ${_parsed!.host}:${_parsed!.port}',
                                style: context.texts.bodySmall?.copyWith(color: colors.text2),
                              ),
                            ),
                            if (!kIsWeb)
                              TextButton(
                                onPressed: _scanPair,
                                style: TextButton.styleFrom(
                                  minimumSize: const Size(44, 44),
                                  foregroundColor: colors.accent,
                                  padding: const EdgeInsets.symmetric(horizontal: 6),
                                ),
                                child: const LText('扫码'),
                              ),
                            TextButton(
                              onPressed: _pastePair,
                              style: TextButton.styleFrom(
                                minimumSize: const Size(44, 44),
                                foregroundColor: colors.accent,
                                padding: const EdgeInsets.symmetric(horizontal: 8),
                              ),
                              child: const LText('粘贴'),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 16),
                _DeviceNameField(controller: _name),
                const SizedBox(height: 20),
                if (_error.isNotEmpty) ...<Widget>[
                  NoticeCard(
                    title: _error,
                    danger: true,
                    margin: const EdgeInsets.only(bottom: 16),
                  ),
                ],
                SecondaryButton(
                  label: _codeBusy ? '等待服务端确认…' : '用配对码连接',
                  height: 46,
                  onTap: _parsed == null || _codeBusy ? null : _runCode,
                ),
                const SizedBox(height: 16),
                LText(
                  '配对码 60 秒内有效，服务端终端会要求本地确认。设备私钥只在本机生成，不会上传。',
                  style: context.texts.labelSmall?.copyWith(
                    color: colors.text3,
                    height: 1.6,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _AddressField extends StatelessWidget {
  const _AddressField({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Row(
      children: <Widget>[
        Expanded(
          child: Container(
            height: 48,
            padding: const EdgeInsets.symmetric(horizontal: 14),
            decoration: SquircleBoxDecoration(
              color: colors.elevated,
              borderRadius: BorderRadius.circular(LTokens.controlRadius),
            ),
            child: TextField(
              controller: controller,
              keyboardType: TextInputType.url,
              autocorrect: false,
              textAlignVertical: TextAlignVertical.center,
              cursorColor: colors.accent,
              style: TextStyle(fontSize: 16, color: colors.text),
              decoration: InputDecoration(
                hintText: L10n.tr('主机:端口'),
                contentPadding: EdgeInsets.zero,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _DeviceNameField extends StatelessWidget {
  const _DeviceNameField({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    return Container(
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: SquircleBoxDecoration(
        color: colors.elevated,
        borderRadius: BorderRadius.circular(LTokens.cardRadius),
      ),
      child: Row(
        children: <Widget>[
          LText(
            '设备名称',
            style: context.texts.bodyLarge?.copyWith(fontSize: 16, color: colors.text),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: TextField(
              controller: controller,
              textAlign: TextAlign.right,
              textAlignVertical: TextAlignVertical.center,
              cursorColor: colors.accent,
              style: TextStyle(fontSize: 16, color: colors.text),
              decoration: InputDecoration(
                hintText: L10n.tr('手机端'),
                contentPadding: EdgeInsets.zero,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

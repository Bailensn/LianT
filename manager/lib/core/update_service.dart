import 'dart:convert';

import 'package:http/http.dart' as http;

class ReleaseInfo {
  const ReleaseInfo({
    required this.tag,
    required this.name,
    required this.prerelease,
    required this.url,
    required this.publishedAt,
  });

  final String tag;
  final String name;
  final bool prerelease;
  final String url;
  final DateTime publishedAt;
}

Future<List<ReleaseInfo>> fetchReleases() async {
  final res = await http.get(
    Uri.parse('https://api.github.com/repos/Bailensn/LianT/releases?per_page=30'),
    headers: <String, String>{
      'Accept': 'application/vnd.github+json',
      'User-Agent': 'LianT-Manager',
    },
  );
  if (res.statusCode != 200) {
    throw Exception('GitHub 返回 ${res.statusCode}');
  }
  final raw = jsonDecode(res.body);
  if (raw is! List) throw Exception('release 列表格式不对');
  final out = <ReleaseInfo>[];
  for (final item in raw) {
    if (item is! Map<String, dynamic>) continue;
    final tag = item['tag_name'] as String? ?? '';
    if (tag.isEmpty) continue;
    out.add(
      ReleaseInfo(
        tag: tag,
        name: item['name'] as String? ?? tag,
        prerelease: item['prerelease'] as bool? ?? false,
        url: item['html_url'] as String? ?? '',
        publishedAt:
            DateTime.tryParse(item['published_at'] as String? ?? '') ??
                DateTime.fromMillisecondsSinceEpoch(0),
      ),
    );
  }
  return out;
}

List<int> versionParts(String version) {
  final matches = RegExp(r'\d+').allMatches(version);
  final parts = matches.map((m) => int.tryParse(m.group(0)!) ?? 0).toList();
  while (parts.length < 3) {
    parts.add(0);
  }
  return parts.take(3).toList();
}

int compareVersions(String a, String b) {
  final pa = versionParts(a);
  final pb = versionParts(b);
  for (var i = 0; i < 3; i++) {
    final diff = pa[i] - pb[i];
    if (diff != 0) return diff;
  }
  return 0;
}

// 时间格式：会话列表用短格式，消息流用时钟，安全日志用日期+分钟。
String _two(int v) => v.toString().padLeft(2, '0');

DateTime _dateOf(int ts) => DateTime.fromMillisecondsSinceEpoch(ts);

bool _sameDay(DateTime a, DateTime b) =>
    a.year == b.year && a.month == b.month && a.day == b.day;

/// 会话列表右上角的时间。
String formatListTime(int ts) {
  if (ts <= 0) return '';
  final date = _dateOf(ts);
  final now = DateTime.now();
  if (_sameDay(date, now)) return '${_two(date.hour)}:${_two(date.minute)}';
  final yesterday = now.subtract(const Duration(days: 1));
  if (_sameDay(date, yesterday)) return '昨天';
  if (date.year == now.year) return '${date.month}月${date.day}日';
  return '${date.year}/${date.month}/${date.day}';
}

/// 消息流里的时钟。
String formatClock(int ts) {
  if (ts <= 0) return '';
  final date = _dateOf(ts);
  return '${_two(date.hour)}:${_two(date.minute)}';
}

/// 消息流时间分隔条：今天只给时钟，其余给日期。
String formatMessageTime(int ts) {
  if (ts <= 0) return '';
  final date = _dateOf(ts);
  final now = DateTime.now();
  if (_sameDay(date, now)) return formatClock(ts);
  if (date.year == now.year) {
    return '${date.month}月${date.day}日 ${formatClock(ts)}';
  }
  return '${date.year}年${date.month}月${date.day}日 ${formatClock(ts)}';
}

/// 安全日志 / 设备活动时间。
String formatDateTime(int ts) {
  if (ts <= 0) return '从未';
  final date = _dateOf(ts);
  return '${date.month}月${date.day}日 ${_two(date.hour)}:${_two(date.minute)}';
}

/// 会话列表分组：今天 / 昨天 / 本周 / 更早。
String dayBucket(int ts) {
  final date = _dateOf(ts <= 0 ? DateTime.now().millisecondsSinceEpoch : ts);
  final now = DateTime.now();
  final startOfToday = DateTime(now.year, now.month, now.day);
  final startOfDate = DateTime(date.year, date.month, date.day);
  final diff = startOfToday.difference(startOfDate).inDays;
  if (diff <= 0) return '今天';
  if (diff == 1) return '昨天';
  if (diff <= 7) return '本周';
  return '更早';
}

/// 剩余秒数 → `m:ss`。
String formatCountdown(int seconds) {
  final safe = seconds < 0 ? 0 : seconds;
  return '${safe ~/ 60}:${_two(safe % 60)}';
}

// AppState 用到的展示模型：媒体加载状态、会话摘要。
import '../net/protocol.dart';

/// 媒体加载状态。
class MediaState {
  const MediaState._({this.url, this.mime = '', this.error, this.loading = false});

  const MediaState.loading() : this._(loading: true);

  const MediaState.ready(String url, {String mime = ''}) : this._(url: url, mime: mime);

  const MediaState.failed(String error) : this._(error: error);

  final String? url;
  final String mime;
  final String? error;
  final bool loading;
}

/// 会话摘要（列表页用）。
class Conversation {
  Conversation({
    required this.key,
    required this.botId,
    required this.peerId,
    required this.peerKind,
    required this.title,
    this.last,
    this.unread = 0,
  });

  final String key;
  final String botId;
  final String peerId;
  String peerKind;
  String title;
  ChatMessage? last;
  int unread;
}

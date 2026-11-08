import 'dart:ui' as ui;

/// 5 种语言 + 跟随系统。翻译表用中文原文做 key，找不到时回退中文。
class L10n {
  static const List<String> settings = <String>[
    'system',
    'zh',
    'zh_Hant',
    'en',
    'ja',
    'ko',
  ];

  static String code = 'zh';

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

  static int get _index {
    switch (code) {
      case 'zh_Hant':
        return 1;
      case 'en':
        return 2;
      case 'ja':
        return 3;
      case 'ko':
        return 4;
      default:
        return 0;
    }
  }

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
        return _exact['跟随系统']![_index];
    }
  }

  static String tr(String text) {
    if (text.isEmpty) return text;
    if (_index == 0) return text;
    final exact = _exact[text];
    if (exact != null) return exact[_index];
    var out = text;
    for (final entry in _phrases.entries) {
      if (entry.key.isEmpty) continue;
      out = out.replaceAll(entry.key, entry.value[_index]);
    }
    return out;
  }

  // [zh, zh_Hant, en, ja, ko]
  static const Map<String, List<String>> _exact = <String, List<String>>{
    '跟随系统': ['跟随系统', '跟隨系統', 'System', 'システム', '시스템'],
    '简体中文': ['简体中文', '简体中文', '简体中文', '简体中文', '중국어 간체'],
    '繁體中文': ['繁體中文', '繁體中文', '繁體中文', '繁體中文', '중국어 번체'],
    '日本語': ['日语', '日語', 'Japanese', '日本語', '일본어'],
    '한국어': ['韩语', '韓語', 'Korean', '韓国語', '한국어'],
    '语言': ['语言', '語言', 'Language', '言語', '언어'],
    '选择语言': ['选择语言', '選擇語言', 'Choose language', '言語を選択', '언어 선택'],
    '设置': ['设置', '設定', 'Settings', '設定', '설정'],
    '消息': ['消息', '訊息', 'Messages', 'メッセージ', '메시지'],
    '机器人': ['机器人', '機器人', 'Bots', 'ボット', '봇'],
    '服务': ['服务', '服務', 'Service', 'サービス', '서비스'],
    '通用': ['通用', '一般', 'General', '一般', '일반'],
    '更新': ['更新', '更新', 'Updates', '更新', '업데이트'],
    '关于': ['关于', '關於', 'About', 'このアプリについて', '정보'],
    '安全': ['安全', '安全', 'Security', 'セキュリティ', '보안'],
    '安全日志': ['安全日志', '安全日誌', 'Security log', 'セキュリティログ', '보안 로그'],
    '已配对设备': ['已配对设备', '已配對裝置', 'Paired devices', 'ペアリング済み端末', '페어링된 기기'],
    '重新连接': ['重新连接', '重新連線', 'Reconnect', '再接続', '다시 연결'],
    '服务指纹': ['服务指纹', '服務指紋', 'Service fingerprint', 'サービスの指紋', '서비스 지문'],
    '本机设备标识': ['本机设备标识', '本機裝置識別碼', 'This device ID', 'この端末の ID', '이 기기 ID'],
    '最近状态': ['最近状态', '最近狀態', 'Recent status', '最近の状態', '최근 상태'],
    '暂无设备': ['暂无设备', '尚無裝置', 'No devices', '端末なし', '기기 없음'],
    '吊销': ['吊销', '撤銷', 'Revoke', '取り消し', '해제'],
    '本机': ['本机', '本機', 'This device', 'この端末', '이 기기'],
    '已配置': ['已配置', '已設定', 'Configured', '設定済み', '설정됨'],
    '服务端能力': ['服务端能力', '伺服器能力', 'Server capabilities', 'サーバー機能', '서버 기능'],
    '暂无记录': ['暂无记录', '尚無紀錄', 'No records', '記録なし', '기록 없음'],
    '设备密钥': ['设备密钥', '裝置金鑰', 'Device key', '端末キー', '기기 키'],
    '解除配对': ['解除配对', '解除配對', 'Unpair', 'ペアリング解除', '페어링 해제'],
    '协议': ['协议', '協定', 'Protocol', 'プロトコル', '프로토콜'],
    '聊天记录': ['聊天记录', '聊天紀錄', 'Chat history', 'チャット履歴', '채팅 기록'],
    '客户端': ['客户端', '用戶端', 'Client', 'クライアント', '클라이언트'],
    '检查更新': ['检查更新', '檢查更新', 'Check for updates', 'アップデートを確認', '업데이트 확인'],
    '更新选项': ['更新选项', '更新選項', 'Update options', '更新オプション', '업데이트 옵션'],
    '是否接收测试版更新': [
      '是否接收测试版更新',
      '是否接收測試版更新',
      'Receive pre-release updates',
      'プレリリース更新を受け取る',
      '사전 출시 업데이트 받기',
    ],
    '默认关闭；开启后会检查 pre-release。': [
      '默认关闭；开启后会检查 pre-release。',
      '預設關閉；開啟後會檢查 pre-release。',
      'Off by default; enables pre-release checks.',
      '既定ではオフ。オンにすると pre-release を確認します。',
      '기본값은 꺼짐이며, 켜면 pre-release를 확인합니다.',
    ],
    '检查 Manager': ['检查 Manager', '檢查 Manager', 'Check Manager', 'Manager を確認', 'Manager 확인'],
    '检查 Service': ['检查 Service', '檢查 Service', 'Check Service', 'Service を確認', 'Service 확인'],
    '当前版本': ['当前版本', '目前版本', 'Current version', '現在のバージョン', '현재 버전'],
    '未握手': ['未握手', '未握手', 'Not connected', '未接続', '연결 안 됨'],
    '当前已是最新版': [
      '当前已是最新版',
      '目前是最新版本',
      'You are up to date',
      '最新バージョンです',
      '최신 버전입니다',
    ],
    '确定': ['确定', '確定', 'OK', 'OK', '확인'],
    '取消': ['取消', '取消', 'Cancel', 'キャンセル', '취소'],
    '前往更新': ['前往更新', '前往更新', 'Update', '更新へ', '업데이트로 이동'],
    '检查更新失败：': [
      '检查更新失败：',
      '檢查更新失敗：',
      'Update check failed: ',
      'アップデート確認に失敗：',
      '업데이트 확인 실패: ',
    ],
    '返回': ['返回', '返回', 'Back', '戻る', '뒤로'],
    '备注名': ['备注名', '備註名', 'Remark', 'メモ名', '메모 이름'],
    '消息免打扰': [
      '消息免打扰',
      '訊息勿擾',
      'Mute notifications',
      '通知をオフ',
      '알림 끄기',
    ],
    '群消息不再提醒，仍会正常接收。': [
      '群消息不再提醒，仍会正常接收。',
      '群組訊息不再提醒，仍會正常接收。',
      'Mute group notifications; messages are still received.',
      'グループ通知をオフにします。受信は継続します。',
      '그룹 알림을 끕니다. 메시지는 계속 수신됩니다.',
    ],
    '这个人的消息不再提醒，仍会正常接收。': [
      '这个人的消息不再提醒，仍会正常接收。',
      '這個人的訊息不再提醒，仍會正常接收。',
      'Mute this person; messages are still received.',
      'この人の通知をオフにします。受信は継続します。',
      '이 사람의 알림을 끕니다. 메시지는 계속 수신됩니다.',
    ],
    '保存备注': ['保存备注', '儲存備註', 'Save remark', 'メモを保存', '메모 저장'],
    '发消息…': ['发消息…', '傳送訊息…', 'Message…', 'メッセージ…', '메시지…'],
    '连接中…': ['连接中…', '連線中…', 'Connecting…', '接続中…', '연결 중…'],
    '发送': ['发送', '傳送', 'Send', '送信', '보내기'],
    '主动消息：普通发送': [
      '主动消息：普通发送',
      '主動訊息：一般傳送',
      'Active message: normal send',
      '主動メッセージ：通常送信',
      '능동 메시지: 일반 전송',
    ],
    '引用回复：被动窗口': [
      '引用回复：被动窗口',
      '引用回覆：被動視窗',
      'Quote reply: passive window',
      '引用返信：パッシブ枠',
      '인용 답장: 수동 창',
    ],
    '带 @：被动窗口': [
      '带 @：被动窗口',
      '帶 @：被動視窗',
      'Mention: passive window',
      '@ 付き：パッシブ枠',
      '@ 포함: 수동 창',
    ],
    '被动回复：受回复窗口限制': [
      '被动回复：受回复窗口限制',
      '被動回覆：受回覆視窗限制',
      'Passive reply: limited by reply window',
      'パッシブ返信：返信枠の制限あり',
      '수동 답장: 답장 창 제한',
    ],
    '仅被动回复': [
      '仅被动回复',
      '僅被動回覆',
      'Passive replies only',
      'パッシブ返信のみ',
      '수동 답장만',
    ],
    '消息操作': ['消息操作', '訊息操作', 'Message actions', 'メッセージ操作', '메시지 작업'],
    '引用这条': ['引用这条', '引用這則', 'Quote', '引用', '인용'],
    '复制文本': ['复制文本', '複製文字', 'Copy text', 'テキストをコピー', '텍스트 복사'],
    '用户操作': ['用户操作', '使用者操作', 'User actions', 'ユーザー操作', '사용자 작업'],
    '复制用户 ID': ['复制用户 ID', '複製使用者 ID', 'Copy user ID', 'ユーザー ID をコピー', '사용자 ID 복사'],
    '复制昵称': ['复制昵称', '複製暱稱', 'Copy nickname', 'ニックネームをコピー', '닉네임 복사'],
    '引用这条消息': ['引用这条消息', '引用這則訊息', 'Quote this message', 'このメッセージを引用', '이 메시지 인용'],
    '正在加载媒体…': [
      '正在加载媒体…',
      '正在載入媒體…',
      'Loading media…',
      'メディアを読み込み中…',
      '미디어 불러오는 중…',
    ],
    '媒体加载失败': ['媒体加载失败', '媒體載入失敗', 'Failed to load media', 'メディアの読み込みに失敗', '미디어 로드 실패'],
    '发送失败 · 点击重试': [
      '发送失败 · 点击重试',
      '傳送失敗 · 點擊重試',
      'Send failed · tap to retry',
      '送信失敗 · タップして再試行',
      '전송 실패 · 탭하여 재시도',
    ],
    '[图片]': ['[图片]', '[圖片]', '[Photo]', '[画像]', '[사진]'],
    '[视频]': ['[视频]', '[影片]', '[Video]', '[動画]', '[동영상]'],
    '[语音]': ['[语音]', '[語音]', '[Voice]', '[音声]', '[음성]'],
    '[文件]': ['[文件]', '[檔案]', '[File]', '[ファイル]', '[파일]'],
    '引用了一条消息': ['引用了一条消息', '引用了一則訊息', 'Quoted a message', 'メッセージを引用しました', '메시지를 인용했습니다'],
    '连接 LianT': ['连接 LianT', '連線 LianT', 'Connect to LianT', 'LianT に接続', 'LianT 연결'],
    '设备名称': ['设备名称', '裝置名稱', 'Device name', '端末名', '기기 이름'],
    '主机:端口': ['主机:端口', '主機:連接埠', 'host:port', 'ホスト:ポート', '호스트:포트'],
    '粘贴 LianT pair 打印的配对信息': [
      '粘贴 LianT pair 打印的配对信息',
      '貼上 LianT pair 印出的配對資訊',
      'Paste the pairing info printed by LianT pair',
      'LianT pair が出力したペアリング情報を貼り付け',
      'LianT pair가 출력한 페어링 정보를 붙여넣기',
    ],
    '连接': ['连接', '連線', 'Connect', '接続', '연결'],
    '配对码': ['配对码', '配對碼', 'Pairing code', 'ペアリングコード', '페어링 코드'],
    '或者用配对码': ['或者用配对码', '或者用配對碼', 'Or use a pairing code', 'またはペアリングコード', '또는 페어링 코드'],
    '取消配对': ['取消配对', '取消配對', 'Cancel pairing', 'ペアリングをキャンセル', '페어링 취소'],
    '服务端开启登录认证时不用配对码：这台设备会直接登记。没有开启时请用下面的配对码。': [
      '服务端开启登录认证时不用配对码：这台设备会直接登记。没有开启时请用下面的配对码。',
      '伺服器開啟登入驗證時不用配對碼：這台裝置會直接登記。沒有開啟時請用下面的配對碼。',
      'If the Service uses login auth, no code is needed; otherwise use the code below.',
      'サーバーがログイン認証を使う場合はコード不要です。それ以外は下のコードを使ってください。',
      '서버가 로그인 인증을 사용하면 코드가 필요 없습니다. 아니면 아래 코드를 사용하세요.',
    ],
    '填服务地址直接登记，或在服务器上执行 LianT pair，把终端里的配对信息粘贴到下面。': [
      '填服务地址直接登记，或在服务器上执行 LianT pair，把终端里的配对信息粘贴到下面。',
      '填服務位址直接登記，或在伺服器上執行 LianT pair，把終端機裡的配對資訊貼到下面。',
      'Enter the Service address to register, or run LianT pair on the server and paste the pairing info below.',
      'サービスアドレスを入力して登録するか、サーバーで LianT pair を実行し、表示された情報を下に貼り付けてください。',
      '서비스 주소를 입력해 등록하거나, 서버에서 LianT pair를 실행해 출력된 정보를 아래에 붙여넣으세요.',
    ],
    '配对码 60 秒内有效，服务端终端会要求本地确认。设备私钥只在本机生成，不会上传。': [
      '配对码 60 秒内有效，服务端终端会要求本地确认。设备私钥只在本机生成，不会上传。',
      '配對碼 60 秒內有效，伺服器終端機會要求本機確認。裝置私鑰只在本機產生，不會上傳。',
      'The code is valid for 60 seconds and must be confirmed on the Service terminal. The device key never leaves this device.',
      'コードは 60 秒間有効で、サーバー側の端末で確認が必要です。端末の秘密鍵はこの端末から出ません。',
      '코드는 60초 동안 유효하며 서버 터미널에서 확인해야 합니다. 기기 개인 키는 이 기기를 떠나지 않습니다.',
    ],
  };

  // 用于拼接字符串里的固定词。
  static const Map<String, List<String>> _phrases = <String, List<String>>{
    '当前版本：': ['当前版本：', '目前版本：', 'Current version: ', '現在のバージョン：', '현재 버전: '],
    '发现新版本：': ['发现新版本：', '發現新版本：', 'New version: ', '新しいバージョン：', '새 버전: '],
    '检查更新失败：': ['检查更新失败：', '檢查更新失敗：', 'Update check failed: ', 'アップデート確認に失敗：', '업데이트 확인 실패: '],
    '回复': ['回复', '回覆', 'Reply', '返信', '답장'],
    '个机器人正在运行': ['个机器人正在运行', '個機器人正在執行', 'bots running', '個のボットが稼働中', '개 봇 실행 중'],
    '最近活动': ['最近活动', '最近活動', 'Last active', '最終活動', '최근 활동'],
    '刚刚': ['刚刚', '剛剛', 'just now', 'たった今', '방금'],
    '分钟前': ['分钟前', '分鐘前', 'min ago', '分前', '분 전'],
    '小时前': ['小时前', '小時前', 'h ago', '時間前', '시간 전'],
    '天前': ['天前', '天前', 'd ago', '日前', '일 전'],
  };
}

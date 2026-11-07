package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.Conversation
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.overlay.OverlayBottomSheet
import top.yukonga.miuix.kmp.theme.MiuixTheme

/**
 * 会话信息底部抽屉，用 Miuix 的 OverlayBottomSheet：
 * 拖拽条、遮罩、直角与动画都由组件自己处理，不再手搓。
 */
@Composable
fun ChatDetailSheet(
    conversation: Conversation,
    onDismiss: () -> Unit,
) {
    OverlayBottomSheet(
        show = true,
        onDismissRequest = onDismiss,
        title = "会话信息",
    ) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 8.dp)) {
            DetailRow("类型", if (conversation.peerKind == "group") "群聊" else "私聊")
            DetailRow("会话 ID", conversation.peerId)
            DetailRow("Bot", conversation.botId)
            Spacer(Modifier.height(16.dp))
            Button(
                onClick = onDismiss,
                modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
            ) {
                Text("关闭")
            }
        }
    }
}

@Composable
private fun DetailRow(label: String, value: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
        Text(label, style = MiuixTheme.textStyles.footnote2)
        Text(value, style = MiuixTheme.textStyles.body2)
    }
}

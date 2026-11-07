package com.bailensn.liant.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.Conversation

/**
 * 会话信息底部抽屉。
 *
 * 规范：底部升起、36×4dp 拖拽条、内容占 60% 高度、底层不可滚动（ModalBottomSheet
 * 自带遮罩拦截）。这是信息类抽屉，允许点遮罩关闭；若以后放表单，必须改成只能点
 * 「取消/保存」关闭。
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatDetailSheet(
    conversation: Conversation,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        dragHandle = {
            Column(
                Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 4.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Spacer(
                    Modifier
                        .size(width = 36.dp, height = 4.dp)
                        .clip(RoundedCornerShape(2.dp))
                        .background(MaterialTheme.colorScheme.outlineVariant),
                )
            }
        },
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .fillMaxHeight(0.6f)
                .padding(horizontal = 20.dp, vertical = 8.dp),
        ) {
            Text(
                "会话信息",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.SemiBold,
            )
            Spacer(Modifier.padding(top = 12.dp))
            DetailRow("类型", if (conversation.peerKind == "group") "群聊" else "私聊")
            DetailRow("会话 ID", conversation.peerId)
            DetailRow("Bot", conversation.botId)
            Spacer(Modifier.weight(1f))
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
        Text(label, style = MaterialTheme.typography.labelMedium)
        Text(value, style = MaterialTheme.typography.bodyMedium)
    }
}

package com.bailensn.liant.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import top.yukonga.miuix.kmp.basic.HorizontalDivider
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.basic.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.data.Conversation
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import kotlin.math.absoluteValue

/** 会话列表：头像 + 标题 + 最近一条 + 时间 + 未读。 */
@Composable
fun ChatsScreen(
    connected: Boolean,
    onReconnect: () -> Unit,
    conversations: List<Conversation>,
    notificationsEnabled: Boolean,
    onOpenNotificationSettings: () -> Unit,
    onOpen: (Conversation) -> Unit,
    onRefresh: () -> Unit,
) {
    Column(Modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "消息",
                style = MiuixTheme.textStyles.title1,
                modifier = Modifier.weight(1f),
            )
            TextButton(
                text = "刷新",
                onClick = onRefresh,
                modifier = Modifier.heightIn(min = 48.dp),
            )
        }
        HorizontalDivider(thickness = 0.5.dp)
        if (!connected) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .background(MiuixTheme.colorScheme.secondaryContainer)
                    .padding(horizontal = 16.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "未连接，正在重试",
                    style = MiuixTheme.textStyles.footnote1,
                    color = MiuixTheme.colorScheme.onSecondaryContainer,
                    modifier = Modifier.weight(1f),
                )
                TextButton(text = "立即重连", onClick = onReconnect)
            }
        }
        if (!notificationsEnabled) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .background(MiuixTheme.colorScheme.secondaryContainer)
                    .padding(horizontal = 16.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "通知未开启，后台收不到新消息提醒",
                    style = MiuixTheme.textStyles.footnote1,
                    modifier = Modifier.weight(1f),
                )
                TextButton(
                    text = "去开启",
                    onClick = onOpenNotificationSettings,
                    modifier = Modifier.heightIn(min = 48.dp),
                )
            }
        }
        if (conversations.isEmpty()) {
            Column(
                Modifier.fillMaxWidth().padding(32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("还没有会话", style = MiuixTheme.textStyles.title4)
                Spacer(Modifier.size(8.dp))
                Text(
                    "在 QQ 群里 @ 一下 Bot，或等它收到消息，这里就会出现会话。",
                    style = MiuixTheme.textStyles.footnote1,
                    color = MiuixTheme.colorScheme.onSurfaceSecondary,
                )
            }
        }
        LazyColumn(Modifier.weight(1f)) {
            items(conversations, key = { "${it.botId}|${it.peerId}" }) { conversation ->
                ConversationRow(conversation, onOpen)
                HorizontalDivider(
                    thickness = 0.5.dp,
                    color = MiuixTheme.colorScheme.outline,
                    modifier = Modifier.padding(start = 72.dp),
                )
            }
        }
    }
}

@Composable
private fun ConversationRow(conversation: Conversation, onOpen: (Conversation) -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable { onOpen(conversation) }.padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Avatar(key = conversation.peerId, label = conversation.title)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    conversation.title,
                    style = MiuixTheme.textStyles.title4,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    formatChatTime(conversation.lastTs),
                    style = MiuixTheme.textStyles.footnote2,
                    color = MiuixTheme.colorScheme.onSurfaceSecondary,
                )
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    conversation.lastPlain.ifBlank { "[消息]" },
                    style = MiuixTheme.textStyles.body2,
                    color = MiuixTheme.colorScheme.onSurfaceSecondary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (conversation.unread > 0) {
                    Box(
                        Modifier
                            .padding(start = 8.dp)
                            .clip(CircleShape)
                            .background(MiuixTheme.colorScheme.error)
                            .padding(horizontal = 6.dp, vertical = 2.dp),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            conversation.unread.coerceAtMost(99).toString(),
                            color = MiuixTheme.colorScheme.onError,
                            fontSize = 11.sp,
                        )
                    }
                }
            }
        }
    }
}

/** 圆形头像：按 peer_id 取稳定色相，显示标题首字。 */
@Composable
fun Avatar(key: String, label: String, size: Dp = 44.dp) {
    val color = remember(key) {
        Color.hsl((key.hashCode().absoluteValue % 360).toFloat(), 0.45f, 0.55f)
    }
    Box(
        Modifier.size(size).clip(CircleShape).background(color),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label.take(1).uppercase(),
            color = Color.White,
            fontSize = (size.value * 0.4f).sp,
        )
    }
}

/** 今天显示 HH:mm，今年显示 MM-dd，更早显示 yyyy-MM-dd。 */
fun formatChatTime(ts: Long): String {
    if (ts <= 0) return ""
    val now = System.currentTimeMillis()
    val date = Date(ts)
    val sameDay = SimpleDateFormat("yyyyMMdd", Locale.getDefault())
        .format(date) == SimpleDateFormat("yyyyMMdd", Locale.getDefault()).format(Date(now))
    return when {
        sameDay -> SimpleDateFormat("HH:mm", Locale.getDefault()).format(date)
        SimpleDateFormat("yyyy", Locale.getDefault()).format(date) ==
            SimpleDateFormat("yyyy", Locale.getDefault()).format(Date(now)) ->
            SimpleDateFormat("MM-dd", Locale.getDefault()).format(date)
        else -> SimpleDateFormat("yyyy-MM-dd", Locale.getDefault()).format(date)
    }
}

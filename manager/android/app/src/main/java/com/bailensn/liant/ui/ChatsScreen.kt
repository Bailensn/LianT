package com.bailensn.liant.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.Conversation
import top.yukonga.miuix.kmp.basic.Card
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.basic.TextButton
import top.yukonga.miuix.kmp.theme.MiuixTheme
import com.bailensn.liant.ui.design.PageTitle

/** 消息列表：分组卡片 + 头像行，对齐 uitest 的列表样式。 */
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
    LazyColumn(
        Modifier.fillMaxWidth(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 8.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        item {
            PageTitle("消息", if (connected) "已连接" else "未连接，正在重试")
        }
        if (!connected) {
            item {
                Card(Modifier.fillMaxWidth()) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            "连接已断开",
                            style = MiuixTheme.textStyles.body1,
                            modifier = Modifier.weight(1f),
                        )
                        TextButton(text = "重连", onClick = onReconnect)
                    }
                }
            }
        }
        if (!notificationsEnabled) {
            item {
                Card(Modifier.fillMaxWidth()) {
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(
                            "通知未开启",
                            style = MiuixTheme.textStyles.body1,
                            modifier = Modifier.weight(1f),
                        )
                        TextButton(text = "去开启", onClick = onOpenNotificationSettings)
                    }
                }
            }
        }
        if (conversations.isEmpty()) {
            item {
                Card(Modifier.fillMaxWidth()) {
                    Column(Modifier.fillMaxWidth().padding(24.dp)) {
                        Text("还没有会话", style = MiuixTheme.textStyles.title4)
                        Text(
                            "在 QQ 群里 @ 一下 Bot，或等它收到消息",
                            style = MiuixTheme.textStyles.footnote2,
                            color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
                            modifier = Modifier.padding(top = 6.dp),
                        )
                    }
                }
            }
        }
        item {
            Card(Modifier.fillMaxWidth()) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "刷新",
                        style = MiuixTheme.textStyles.body1,
                        modifier = Modifier.weight(1f),
                    )
                    TextButton(text = "刷新", onClick = onRefresh)
                }
            }
        }
        items(conversations, key = { "${it.botId}|${it.peerId}" }) { conversation ->
            Card(Modifier.fillMaxWidth().clickable { onOpen(conversation) }) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier.size(44.dp).clip(CircleShape),
                    ) {
                        Card(Modifier.fillMaxWidth()) {
                            Box(Modifier.size(44.dp), contentAlignment = Alignment.Center) {
                                Text(
                                    conversation.title.take(1),
                                    style = MiuixTheme.textStyles.title4,
                                )
                            }
                        }
                    }
                    Column(Modifier.weight(1f).padding(start = 12.dp)) {
                        Text(conversation.title, style = MiuixTheme.textStyles.body1)
                        Text(
                            conversation.preview.ifEmpty { "暂无消息" },
                            style = MiuixTheme.textStyles.footnote2,
                            color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
                        )
                    }
                }
            }
        }
    }
}

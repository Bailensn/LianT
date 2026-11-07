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
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.WindowInsetsSides
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.only
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.ChatMessage
import com.bailensn.liant.data.Conversation

/** 单个会话：顶部栏 + 气泡列表 + 输入条，QQ/微信式布局。 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatScreen(
    conversation: Conversation,
    messages: List<ChatMessage>,
    sending: Boolean,
    onBack: () -> Unit,
    onSend: (String) -> Unit,
) {
    var showDetails by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()
    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty()) {
            listState.animateScrollToItem(messages.lastIndex)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column(Modifier.clickable { showDetails = true }) {
                        Text(
                            conversation.title,
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            if (conversation.peerKind == "group") "群聊" else "私聊",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                },
                navigationIcon = {
                    TextButton(
                        onClick = onBack,
                        modifier = Modifier.heightIn(min = 48.dp),
                    ) { Text("‹ 返回") }
                },
            )
        },
        bottomBar = {
            InputBar(sending = sending, onSend = onSend)
        },
    ) { padding ->
        Box(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .background(MaterialTheme.colorScheme.surfaceContainerLowest),
        ) {
            LazyColumn(
                state = listState,
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(
                    horizontal = 12.dp,
                    vertical = 8.dp,
                ),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                items(messages, key = { it.msgId }) { message ->
                    MessageBubble(message)
                }
            }
        }
    }

    if (showDetails) {
        ChatDetailSheet(conversation = conversation, onDismiss = { showDetails = false })
    }
}

@Composable
private fun MessageBubble(message: ChatMessage) {
    val outgoing = message.outgoing
    Row(
        Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = if (outgoing) Arrangement.End else Arrangement.Start,
    ) {
        Column(
            Modifier
                .widthIn(max = 280.dp)
                .clip(
                    RoundedCornerShape(
                        topStart = 12.dp,
                        topEnd = 12.dp,
                        bottomStart = if (outgoing) 12.dp else 2.dp,
                        bottomEnd = if (outgoing) 2.dp else 12.dp,
                    ),
                )
                .background(
                    if (outgoing) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.surfaceContainerHigh,
                )
                .padding(horizontal = 12.dp, vertical = 8.dp),
        ) {
            if (!outgoing && message.senderName.isNotBlank()) {
                Text(
                    message.senderName,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
                Spacer(Modifier.size(2.dp))
            }
            if (message.kind == "image") {
                MediaThumb(
                    ratio = MediaRatio.FourThree,
                    modifier = Modifier.fillMaxWidth(),
                    label = "图片·待下载",
                )
                if (message.plain.isNotBlank()) {
                    Spacer(Modifier.size(4.dp))
                    Text(
                        message.plain,
                        style = MaterialTheme.typography.bodyMedium,
                        color = if (outgoing) MaterialTheme.colorScheme.onPrimary
                        else MaterialTheme.colorScheme.onSurface,
                    )
                }
            } else {
                Text(
                    message.plain,
                    style = MaterialTheme.typography.bodyLarge,
                    color = if (outgoing) MaterialTheme.colorScheme.onPrimary
                    else MaterialTheme.colorScheme.onSurface,
                )
            }
            Spacer(Modifier.size(2.dp))
            Text(
                formatChatTime(message.ts),
                style = MaterialTheme.typography.labelSmall,
                color = if (outgoing) MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f)
                else MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.align(Alignment.End),
            )
        }
    }
}

@Composable
private fun InputBar(sending: Boolean, onSend: (String) -> Unit) {
    var text by remember { mutableStateOf("") }
    Row(
        Modifier
            .fillMaxWidth()
            // 键盘与底部手势条都算进安全区，不用 imePadding 叠 navigationBarsPadding。
            .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Bottom))
            .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        OutlinedTextField(
            value = text,
            onValueChange = { text = it },
            modifier = Modifier.weight(1f),
            placeholder = { Text("输入消息") },
            maxLines = 4,
        )
        Spacer(Modifier.size(8.dp))
        Button(
            onClick = {
                val value = text.trim()
                if (value.isNotEmpty()) {
                    onSend(value)
                    text = ""
                }
            },
            enabled = !sending && text.isNotBlank(),
            modifier = Modifier.heightIn(min = 48.dp),
        ) {
            Text(if (sending) "发送中" else "发送")
        }
    }
}
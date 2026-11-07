package com.bailensn.liant.ui

import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.aspectRatio
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
import top.yukonga.miuix.kmp.basic.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.basic.TextField
import top.yukonga.miuix.kmp.basic.Scaffold
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.basic.TextButton
import androidx.compose.foundation.text.input.TextFieldLineLimits
import androidx.compose.foundation.text.input.rememberTextFieldState
import top.yukonga.miuix.kmp.basic.SmallTopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.ui.design.formatChatTime
import com.bailensn.liant.data.ChatMessage
import com.bailensn.liant.data.Conversation

/** 单个会话：顶部栏 + 气泡列表 + 输入条，QQ/微信式布局。 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ChatScreen(
    conversation: Conversation,
    messages: List<ChatMessage>,
    media: Map<String, Bitmap>,
    mediaErrors: Map<String, String>,
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
            SmallTopAppBar(
                title = conversation.title,
                subtitle = if (conversation.peerKind == "group") "群聊" else "私聊",
                navigationIcon = {
                    TextButton(text = "返回", onClick = onBack)
                },
                actions = {
                    TextButton(text = "详情", onClick = { showDetails = true })
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
                .background(MiuixTheme.colorScheme.surfaceVariant),
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
                    MessageBubble(message, media, mediaErrors)
                }
            }
        }
    }

    if (showDetails) {
        ChatDetailSheet(conversation = conversation, onDismiss = { showDetails = false })
    }
}

@Composable
private fun MessageBubble(
    message: ChatMessage,
    media: Map<String, Bitmap>,
    mediaErrors: Map<String, String>,
) {
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
                    if (outgoing) MiuixTheme.colorScheme.primary
                    else MiuixTheme.colorScheme.surfaceVariant,
                )
                .padding(horizontal = 12.dp, vertical = 8.dp),
        ) {
            if (!outgoing && message.senderName.isNotBlank()) {
                Text(
                    message.senderName,
                    style = MiuixTheme.textStyles.footnote2,
                    color = MiuixTheme.colorScheme.primary,
                )
                Spacer(Modifier.size(2.dp))
            }
            if (message.kind == "image") {
                val bitmap = media[message.msgId]
                if (bitmap != null) {
                    Image(
                        bitmap = bitmap.asImageBitmap(),
                        contentDescription = "图片消息",
                        contentScale = ContentScale.Crop,
                        modifier = Modifier
                            .fillMaxWidth()
                            .aspectRatio(MediaRatio.FourThree.value)
                            .clip(RoundedCornerShape(8.dp)),
                    )
                } else {
                    MediaThumb(
                        ratio = MediaRatio.FourThree,
                        modifier = Modifier.fillMaxWidth(),
                        label = when (mediaErrors[message.msgId]) {
                            "media_expired" -> "图片已过期"
                            "media_evicted" -> "空间不足，图片已清理"
                            "failed" -> "图片加载失败"
                            else -> "图片·待下载"
                        },
                    )
                }
                if (message.plain.isNotBlank()) {
                    Spacer(Modifier.size(4.dp))
                    Text(
                        message.plain,
                        style = MiuixTheme.textStyles.body2,
                        color = if (outgoing) MiuixTheme.colorScheme.onPrimary
                        else MiuixTheme.colorScheme.onSurface,
                    )
                }
            } else {
                Text(
                    message.plain,
                    style = MiuixTheme.textStyles.body1,
                    color = if (outgoing) MiuixTheme.colorScheme.onPrimary
                    else MiuixTheme.colorScheme.onSurface,
                )
            }
            Spacer(Modifier.size(2.dp))
            Text(
                formatChatTime(message.ts),
                style = MiuixTheme.textStyles.footnote2,
                color = if (outgoing) MiuixTheme.colorScheme.onPrimary.copy(alpha = 0.7f)
                else MiuixTheme.colorScheme.onSurfaceSecondary,
                modifier = Modifier.align(Alignment.End),
            )
        }
    }
}

@Composable
private fun InputBar(sending: Boolean, onSend: (String) -> Unit) {
    val textState = rememberTextFieldState()
    Row(
        Modifier
            .fillMaxWidth()
            // 键盘与底部手势条都算进安全区，不用 imePadding 叠 navigationBarsPadding。
            .windowInsetsPadding(WindowInsets.safeDrawing.only(WindowInsetsSides.Bottom))
            .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        TextField(
            state = textState,
            modifier = Modifier.weight(1f),
            label = if (textState.text.isEmpty()) "输入消息" else "",
            useLabelAsPlaceholder = true,
            lineLimits = TextFieldLineLimits.MultiLine(minHeightInLines = 1, maxHeightInLines = 4),
        )
        Spacer(Modifier.size(8.dp))
        Button(
            onClick = {
                val value = textState.text.toString().trim()
                if (value.isNotEmpty()) {
                    onSend(value)
                    textState.setAll("")
                }
            },
            enabled = !sending && textState.text.isNotBlank(),
            modifier = Modifier.heightIn(min = 48.dp),
        ) {
            Text(if (sending) "发送中" else "发送")
        }
    }
}
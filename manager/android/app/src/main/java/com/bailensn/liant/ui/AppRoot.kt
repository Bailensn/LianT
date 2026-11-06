package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.ChatController
import com.bailensn.liant.data.Conversation
import com.bailensn.liant.data.LiantRepository
import kotlinx.coroutines.launch

/**
 * 客户端根：未配对显示配对面板，已配对显示会话列表与会话页。
 *
 * 配对二维码由 Service 的 `LianT pair` 打印；v1 先粘贴 JSON，
 * 相机扫码与 Room 留到下一步。
 */
@Composable
fun AppRoot() {
    val context = LocalContext.current
    val repository = remember { LiantRepository(context.applicationContext) }
    val controller = remember { ChatController(context.applicationContext, repository) }
    val scope = rememberCoroutineScope()

    val conversations by controller.conversations.collectAsState()
    val messages by controller.messages.collectAsState()
    val controllerError by controller.error.collectAsState()

    var paired by remember { mutableStateOf(repository.savedPairing() != null) }
    var current by remember { mutableStateOf<Conversation?>(null) }
    var busy by remember { mutableStateOf(false) }
    var sending by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(paired) {
        if (!paired) return@LaunchedEffect
        busy = true
        error = null
        try {
            if (!repository.isConnected()) repository.connect()
            controller.refresh()
        } catch (e: Exception) {
            error = e.message ?: e::class.java.simpleName
        } finally {
            busy = false
        }
        repository.events()?.collect { envelope -> controller.handleEvent(envelope) }
    }

    MaterialTheme {
        Box(Modifier.fillMaxSize()) {
            val openConversation = current
            when {
                !paired -> PairingRoute(
                    busy = busy,
                    onPair = { qr, name ->
                        scope.launch {
                            busy = true
                            error = null
                            try {
                                repository.pair(qr, name)
                                paired = true
                            } catch (e: Exception) {
                                error = e.message ?: e::class.java.simpleName
                            } finally {
                                busy = false
                            }
                        }
                    },
                )

                openConversation == null -> {
                    Column(Modifier.fillMaxSize()) {
                        if (busy) LinearProgressIndicator(Modifier.fillMaxWidth())
                        ChatsScreen(
                            conversations = conversations,
                            onOpen = { conversation ->
                                scope.launch {
                                    controller.open(conversation)
                                    current = conversation
                                }
                            },
                            onRefresh = {
                                scope.launch {
                                    try {
                                        controller.refresh()
                                    } catch (e: Exception) {
                                        error = e.message ?: e::class.java.simpleName
                                    }
                                }
                            },
                            onForget = {
                                repository.forget()
                                controller.closeChat()
                                current = null
                                paired = false
                            },
                        )
                    }
                }

                else -> ChatScreen(
                    conversation = openConversation,
                    messages = messages,
                    sending = sending,
                    onBack = {
                        controller.closeChat()
                        current = null
                    },
                    onSend = { text ->
                        scope.launch {
                            sending = true
                            controller.clearError()
                            controller.send(text)
                            sending = false
                        }
                    },
                )
            }

            val message = error ?: controllerError
            if (message != null) {
                Text(
                    "错误：$message",
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .padding(16.dp),
                )
            }
        }
    }
}

@Composable
private fun PairingRoute(
    busy: Boolean,
    onPair: (qr: String, name: String) -> Unit,
) {
    Column(
        Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("LianT", style = MaterialTheme.typography.headlineMedium)
        Text("在 Service 终端运行 LianT pair，粘贴打印出的二维码内容")
        var qr by remember { mutableStateOf("") }
        var name by remember { mutableStateOf("我的手机") }
        OutlinedTextField(
            value = qr,
            onValueChange = { qr = it },
            label = { Text("二维码 JSON") },
            modifier = Modifier.fillMaxWidth(),
            minLines = 3,
            maxLines = 8,
        )
        OutlinedTextField(
            value = name,
            onValueChange = { name = it },
            label = { Text("设备名") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
        )
        Button(
            onClick = { onPair(qr.trim(), name.trim().ifEmpty { "未命名设备" }) },
            enabled = !busy && qr.isNotBlank(),
        ) {
            Text(if (busy) "等待 Service 确认…" else "配对")
        }
    }
}

package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import com.bailensn.liant.data.BotSummary
import com.bailensn.liant.data.ChatController
import com.bailensn.liant.data.Conversation
import com.bailensn.liant.data.LiantRepository
import com.bailensn.liant.data.SavedPairing
import kotlinx.coroutines.launch

/**
 * 客户端根：未配对 → 配对页；已配对 → 消息 / Bot / 设置 三栏 + 底部 TabBar。
 *
 * 规范（Ludi mobile-design-system）：顶部避让状态栏、底部 TabBar 与输入条避让手势条，
 * 可点击项 ≥48dp，分割线 0.5dp，键盘弹起不遮挡输入。
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
    var saved by remember { mutableStateOf(repository.savedPairing()) }
    var current by remember { mutableStateOf<Conversation?>(null) }
    var tab by remember { mutableStateOf(0) }
    var bots by remember { mutableStateOf<List<BotSummary>>(emptyList()) }
    var botsBusy by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var sending by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    suspend fun loadBots() {
        botsBusy = true
        try {
            if (!repository.isConnected()) repository.connect()
            bots = repository.bots()
        } catch (e: Exception) {
            error = e.message ?: e::class.java.simpleName
        } finally {
            botsBusy = false
        }
    }

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

    LaunchedEffect(tab, paired) {
        if (paired && tab == 1) loadBots()
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
                                saved = repository.pair(qr, name)
                                paired = true
                            } catch (e: Exception) {
                                error = e.message ?: e::class.java.simpleName
                            } finally {
                                busy = false
                            }
                        }
                    },
                )

                openConversation != null -> ChatScreen(
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

                else -> Column(Modifier.fillMaxSize().statusBarsPadding()) {
                    Box(Modifier.weight(1f)) {
                        when (tab) {
                            0 -> ChatsScreen(
                                conversations = conversations,
                                onOpen = { conversation ->
                                    scope.launch {
                                        controller.open(conversation)
                                        current = conversation
                                    }
                                },
                                onRefresh = {
                                    scope.launch { controller.refresh() }
                                },
                            )

                            1 -> BotListPanel(
                                bots = bots,
                                busy = botsBusy,
                                onRefresh = { scope.launch { loadBots() } },
                            )

                            else -> SettingsPanel(
                                saved = saved,
                                onForget = {
                                    repository.forget()
                                    controller.closeChat()
                                    current = null
                                    saved = null
                                    bots = emptyList()
                                    paired = false
                                },
                            )
                        }
                        if (busy) {
                            LinearProgressIndicator(Modifier.fillMaxWidth().align(Alignment.TopCenter))
                        }
                    }
                    BottomTabBar(selected = tab, onSelect = { tab = it })
                }
            }

            val message = error ?: controllerError
            if (message != null) {
                Text(
                    "错误：$message",
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .navigationBarsPadding()
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
        Modifier
            .fillMaxSize()
            .statusBarsPadding()
            .navigationBarsPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
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
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
        ) {
            Text(if (busy) "等待 Service 确认…" else "配对")
        }
    }
}

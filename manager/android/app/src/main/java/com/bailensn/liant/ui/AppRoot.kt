package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.BotSummary
import com.bailensn.liant.data.LiantRepository
import kotlinx.coroutines.launch

/**
 * 客户端根：未配对显示配对面板，已配对显示 Bot 列表。
 *
 * 配对二维码由 Service 的 `LianT pair` 打印；v1 先支持粘贴 JSON，
 * 相机扫码后面再加（CameraX）。
 */
@Composable
fun AppRoot() {
    val context = LocalContext.current
    val repository = remember { LiantRepository(context.applicationContext) }
    val scope = rememberCoroutineScope()

    var paired by remember { mutableStateOf(repository.savedPairing() != null) }
    var busy by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var bots by remember { mutableStateOf<List<BotSummary>>(emptyList()) }

    suspend fun loadBots() {
        busy = true
        error = null
        try {
            if (!repository.isConnected()) repository.connect()
            bots = repository.bots()
        } catch (e: Exception) {
            error = e.message ?: e::class.java.simpleName
        } finally {
            busy = false
        }
    }

    LaunchedEffect(paired) {
        if (paired) loadBots()
    }

    MaterialTheme {
        Column(
            modifier = Modifier.fillMaxSize().padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("LianT", style = MaterialTheme.typography.headlineMedium)

            if (!paired) {
                Text("粘贴 Service 终端打印的二维码内容（JSON）")
                PairPanel(
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
            } else {
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Button(onClick = { scope.launch { loadBots() } }, enabled = !busy) {
                        Text("刷新")
                    }
                    Button(
                        onClick = {
                            repository.forget()
                            paired = false
                            bots = emptyList()
                        },
                        enabled = !busy,
                    ) {
                        Text("解除配对")
                    }
                }
                if (busy) CircularProgressIndicator()
                if (bots.isEmpty() && !busy) {
                    Text("还没有 Bot。先在 Service 上运行：LianT bot add --platform qqbot --cred app_id=... --cred app_secret=...")
                }
                LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(bots) { bot -> BotCard(bot) }
                }
            }

            error?.let { message ->
                Text("错误：$message", color = MaterialTheme.colorScheme.error)
            }
        }
    }
}

@Composable
private fun PairPanel(
    busy: Boolean,
    onPair: (qr: String, name: String) -> Unit,
) {
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

@Composable
private fun BotCard(bot: BotSummary) {
    Card {
        Column(Modifier.fillMaxWidth().padding(12.dp)) {
            Text(bot.displayName.ifEmpty { bot.botId }, style = MaterialTheme.typography.titleMedium)
            Text("${bot.platform} · ${bot.status}", style = MaterialTheme.typography.bodySmall)
            if (bot.caps.isNotEmpty()) {
                Text(bot.caps.joinToString(" / "), style = MaterialTheme.typography.bodySmall)
            }
        }
    }
}

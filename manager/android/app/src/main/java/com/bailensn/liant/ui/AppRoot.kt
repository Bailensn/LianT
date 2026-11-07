package com.bailensn.liant.ui

import android.content.ActivityNotFoundException
import android.content.Context
import android.graphics.BitmapFactory
import android.net.Uri
import android.content.Intent
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
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
import com.bailensn.liant.net.ConnectionClosedException
import com.bailensn.liant.net.ProtocolException
import com.bailensn.liant.notify.DeepLink
import com.bailensn.liant.notify.MessageNotifier
import com.bailensn.liant.service.ConnectionService
import com.google.zxing.BarcodeFormat
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.MultiFormatReader
import com.google.zxing.NotFoundException
import com.google.zxing.RGBLuminanceSource
import com.google.zxing.common.HybridBinarizer
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

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
    val media by controller.media.collectAsState()
    val mediaErrors by controller.mediaErrors.collectAsState()
    val appVersion = remember {
        runCatching {
            context.packageManager.getPackageInfo(context.packageName, 0).versionName
        }.getOrNull() ?: ""
    }
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
    var activeBotId by remember { mutableStateOf(repository.activeBotId()) }
    var notificationsEnabled by remember { mutableStateOf(MessageNotifier.isEnabled(context)) }

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

    // 连接 → 收事件 → 等断开 → 退避重连 → 补拉。整个循环随 paired 取消。
    LaunchedEffect(paired) {
        if (!paired) return@LaunchedEffect
        busy = true
        var attempt = 0
        while (true) {
            try {
                repository.connect()
                controller.refresh()
                controller.backfill()
                busy = false
                error = null
                attempt = 0
                coroutineScope {
                    val collector = launch {
                        repository.events()?.collect { envelope ->
                            controller.handleEvent(envelope)
                        }
                    }
                    repository.closed()?.first()
                    collector.cancel()
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                busy = false
                error = e.message ?: e::class.java.simpleName
                // 协议 §9：认证失败（4001）不重连，重试只会一直撞墙。
                if (e is ProtocolException && (e.code == "unauthorized" || e.code == "forbidden")) {
                    break
                }
                // 关闭码 4001（认证失败）/ 4002（版本不兼容）重连也没用。
                if (e is ConnectionClosedException && (e.code == 4001 || e.code == 4002)) {
                    break
                }
            }
            if (!isActive) break
            delay(backoffMillis(attempt))
            attempt++
        }
    }

    // 配对后挂前台服务；解除配对时停掉。
    LaunchedEffect(paired) {
        if (paired) {
            ConnectionService.start(context)
        } else {
            ConnectionService.stop(context)
        }
    }

    fun openNotificationSettings() {
        val intent = Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
            .putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        try {
            context.startActivity(intent)
        } catch (_: ActivityNotFoundException) {
            // 少数 ROM 没有这个设置页：静默失败，用户可自行去设置。
        }
    }

    LaunchedEffect(tab, paired) {
        if (paired && tab == 1) loadBots()
        if (tab == 2) notificationsEnabled = MessageNotifier.isEnabled(context)
    }

    // 通知点进来：找到对应会话就打开；会话还没补拉出来时等列表刷新后再试。
    val deepLink by DeepLink.pending
    LaunchedEffect(deepLink, conversations, paired) {
        val target = deepLink ?: return@LaunchedEffect
        if (!paired) return@LaunchedEffect
        val conversation = conversations.firstOrNull {
            it.botId == target.botId && it.peerId == target.peerId
        } ?: return@LaunchedEffect
        controller.open(conversation)
        current = conversation
        tab = 0
        DeepLink.consume()
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
                    media = media,
                    mediaErrors = mediaErrors,
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
                                notificationsEnabled = notificationsEnabled,
                                onOpenNotificationSettings = ::openNotificationSettings,
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
                                activeBotId = activeBotId,
                                onRefresh = { scope.launch { loadBots() } },
                                onStart = { botId ->
                                    scope.launch {
                                        try {
                                            repository.startBot(botId)
                                            loadBots()
                                        } catch (e: Exception) {
                                            error = e.message ?: e::class.java.simpleName
                                        }
                                    }
                                },
                                onStop = { botId ->
                                    scope.launch {
                                        try {
                                            repository.stopBot(botId)
                                            loadBots()
                                        } catch (e: Exception) {
                                            error = e.message ?: e::class.java.simpleName
                                        }
                                    }
                                },
                                onSelectActive = { botId ->
                                    repository.setActiveBotId(botId)
                                    activeBotId = botId
                                },
                            )

                            else -> SettingsPanel(
                                saved = saved,
                                appVersion = appVersion,
                                notificationsEnabled = notificationsEnabled,
                                onOpenNotificationSettings = ::openNotificationSettings,
                                onForget = {
                                    repository.forget()
                                    controller.closeChat()
                                    current = null
                                    saved = null
                                    bots = emptyList()
                                    activeBotId = null
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
        val scanner = rememberLauncherForActivityResult(ScanContract()) { result ->
            result.contents?.let { qr = it }
        }
        val context = LocalContext.current
        val scope = rememberCoroutineScope()
        var importError by remember { mutableStateOf<String?>(null) }
        val picker = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
            if (uri != null) {
                scope.launch {
                    val text = withContext(Dispatchers.IO) { decodeQrFromUri(context, uri) }
                    if (text != null) {
                        qr = text
                        importError = null
                    } else {
                        importError = "这张图里没找到二维码"
                    }
                }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(
                onClick = {
                    scanner.launch(
                        ScanOptions().apply {
                            setDesiredBarcodeFormats(ScanOptions.QR_CODE)
                            setPrompt("把 Service 终端上的配对二维码放进框内")
                            setBeepEnabled(false)
                            setOrientationLocked(false)
                        },
                    )
                },
                modifier = Modifier.weight(1f).heightIn(min = 48.dp),
            ) {
                Text("扫码")
            }
            Button(
                onClick = { picker.launch("image/*") },
                modifier = Modifier.weight(1f).heightIn(min = 48.dp),
            ) {
                Text("相册导入")
            }
        }
        importError?.let {
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        OutlinedTextField(
            value = qr,
            onValueChange = { qr = it },
            label = { Text("二维码 JSON，也可以手动粘贴") },
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

/** 1s → 2s → 4s … → 30s 退避（协议 §9）。 */
private fun backoffMillis(attempt: Int): Long =
    (1000L shl attempt.coerceAtMost(5)).coerceAtMost(30_000L)

private fun decodeQrFromUri(context: Context, uri: Uri): String? = runCatching {
    val bytes = context.contentResolver.openInputStream(uri)?.use { it.readBytes() } ?: return null
    decodeQr(bytes)
}.getOrNull()

private fun decodeQr(bytes: ByteArray): String? {
    val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
    BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
    if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null
    var sample = 1
    while (bounds.outWidth / (sample * 2) >= 2048 && bounds.outHeight / (sample * 2) >= 2048) {
        sample *= 2
    }
    val options = BitmapFactory.Options().apply { inSampleSize = sample }
    val bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options) ?: return null
    val pixels = IntArray(bitmap.width * bitmap.height)
    bitmap.getPixels(pixels, 0, bitmap.width, 0, 0, bitmap.width, bitmap.height)
    val source = RGBLuminanceSource(bitmap.width, bitmap.height, pixels)
    val image = BinaryBitmap(HybridBinarizer(source))
    val hints = mapOf<DecodeHintType, Any>(
        DecodeHintType.POSSIBLE_FORMATS to listOf(BarcodeFormat.QR_CODE),
        DecodeHintType.TRY_HARDER to true,
    )
    return try {
        MultiFormatReader().decode(image, hints).text
    } catch (_: NotFoundException) {
        null
    }
}

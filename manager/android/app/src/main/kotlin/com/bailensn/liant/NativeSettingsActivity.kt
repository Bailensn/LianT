package com.bailensn.liant

import android.app.Activity
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.unit.dp
import androidx.core.view.WindowCompat
import com.bailensn.liant.core.bridge.BridgeClient
import com.bailensn.liant.ui.shell.GlassBottomNav
import com.bailensn.liant.ui.shell.NativePlaceholderScreen
import com.bailensn.liant.ui.settings.SettingsDataLoader
import com.bailensn.liant.ui.settings.SettingsScreen
import com.bailensn.liant.ui.settings.SettingsUiState
import com.bailensn.liant.ui.theme.LiantTheme
import com.bailensn.liant.ui.theme.LocalLColors
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * 原生设置页（迁移试水）。
 *
 * 读本机文件拿身份/心跳，连本地桥接拿 `state.snapshot`（设备/机器人/能力）。
 * **会占用桥接的唯一连接**：从 Flutter 界面进来时 Flutter 那边的桥接会断，
 * 回 Flutter 后 AppState 会重连（resume 时补一次）。
 */
class NativeSettingsActivity : ComponentActivity() {
    private val ui = Handler(Looper.getMainLooper())

    /**
     * 取一次首屏快照。刚连上桥接时后台可能还没把 peer 切过来，
     * 失败就隔 400ms 重试，最多 5 次。
     */
    private suspend fun fetchSnapshot(client: BridgeClient): JSONObject? {
        repeat(5) { attempt ->
            val result = withContext(Dispatchers.IO) {
                runCatching { client.request("state.snapshot", JSONObject()) }
            }
            result.onSuccess { return it }
            Log.w(TAG, "state.snapshot 第 ${attempt + 1} 次失败：${result.exceptionOrNull()?.message}")
            delay(400)
        }
        return null
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            LiantTheme {
                val dark = isSystemInDarkTheme()
                val view = LocalView.current
                SideEffect {
                    val window = (view.context as? Activity)?.window ?: return@SideEffect
                    val controller = WindowCompat.getInsetsController(window, view)
                    controller.isAppearanceLightStatusBars = !dark
                    controller.isAppearanceLightNavigationBars = !dark
                }

                val context = LocalContext.current
                var state by remember { mutableStateOf(SettingsUiState()) }
                var bridge by remember { mutableStateOf<BridgeClient?>(null) }
                val scope = rememberCoroutineScope()

                val listener = remember {
                    object : BridgeClient.Listener {
                        override fun onState(stateName: String, detail: String) {
                            ui.post { state = state.copy(connection = stateName, detail = detail) }
                        }

                        override fun onEvent(envelope: JSONObject) = Unit

                        override fun onClosed() {
                            ui.post { state = state.copy(connection = "idle", detail = "后台连接已断开") }
                        }
                    }
                }

                fun reload() {
                    scope.launch {
                        state = withContext(Dispatchers.IO) { SettingsDataLoader.load(context) }
                        val current = bridge ?: return@launch
                        if (current.isClosed) return@launch
                        val snapshot = fetchSnapshot(current)
                        if (snapshot != null) state = state.withSnapshot(snapshot)
                    }
                }

                LaunchedEffect(Unit) {
                    state = withContext(Dispatchers.IO) { SettingsDataLoader.load(context) }
                    val connected = withContext(Dispatchers.IO) {
                        BridgeClient.connect(context, listener)
                    }
                    bridge = connected
                    if (connected == null) {
                        Log.w(TAG, "原生设置页：桥接连接失败（端口/令牌/后台未就绪）")
                        state = state.copy(connection = "idle")
                    } else {
                        Log.i(TAG, "原生设置页：桥接已就绪，拉取 state.snapshot")
                        val snapshot = fetchSnapshot(connected)
                        if (snapshot != null) {
                            val deviceCount = snapshot.optJSONArray("devices")?.length() ?: 0
                            Log.i(TAG, "原生设置页：snapshot 已应用（$deviceCount 台设备）")
                            state = state.withSnapshot(snapshot)
                        } else {
                            Log.w(TAG, "原生设置页：state.snapshot 多次失败")
                        }
                    }
                }

                DisposableEffect(Unit) {
                    onDispose { bridge?.close() }
                }

                var tab by remember { mutableStateOf(2) }
                val colors = LocalLColors.current
                Box(
                    Modifier
                        .fillMaxSize()
                        .background(colors.bg),
                ) {
                    when (tab) {
                        0 -> NativePlaceholderScreen(title = "会话")
                        1 -> NativePlaceholderScreen(title = "机器人")
                        else -> SettingsScreen(
                            state = state,
                            onRefresh = { reload() },
                            onRevoke = { deviceId ->
                                scope.launch {
                                    val current = bridge ?: return@launch
                                    withContext(Dispatchers.IO) {
                                        runCatching {
                                            current.request(
                                                "device.revoke",
                                                JSONObject().put("device_id", deviceId),
                                            )
                                        }
                                    }
                                    reload()
                                }
                            },
                        )
                    }
                    GlassBottomNav(
                        index = tab,
                        onSelect = { tab = it },
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp)
                            .padding(bottom = 16.dp)
                            .navigationBarsPadding(),
                    )
                }
            }
        }
    }

    private companion object {
        const val TAG = "LianT"
    }
}

package com.bailensn.liant

import android.app.Activity
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import com.bailensn.liant.core.bridge.BridgeClient
import com.bailensn.liant.ui.settings.SettingsDataLoader
import com.bailensn.liant.ui.settings.SettingsScreen
import com.bailensn.liant.ui.settings.SettingsUiState
import com.bailensn.liant.ui.theme.LiantTheme
import kotlinx.coroutines.Dispatchers
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
                        val current = bridge
                        if (current != null && !current.isClosed) {
                            val snapshot = withContext(Dispatchers.IO) {
                                runCatching {
                                    current.request("state.snapshot", JSONObject())
                                }.getOrNull()
                            }
                            if (snapshot != null) state = state.withSnapshot(snapshot)
                        }
                    }
                }

                LaunchedEffect(Unit) {
                    state = withContext(Dispatchers.IO) { SettingsDataLoader.load(context) }
                    val connected = withContext(Dispatchers.IO) {
                        BridgeClient.connect(context, listener)
                    }
                    bridge = connected
                    if (connected == null) {
                        state = state.copy(connection = "idle")
                    }
                    if (connected != null) {
                        val snapshot = withContext(Dispatchers.IO) {
                            runCatching {
                                connected.request("state.snapshot", JSONObject())
                            }.getOrNull()
                        }
                        if (snapshot != null) state = state.withSnapshot(snapshot)
                    }
                }

                DisposableEffect(Unit) {
                    onDispose { bridge?.close() }
                }

                SettingsScreen(
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
        }
    }
}

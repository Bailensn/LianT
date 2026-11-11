package com.bailensn.liant

import android.app.Activity
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
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
import com.bailensn.liant.ui.settings.SettingsDataLoader
import com.bailensn.liant.ui.settings.SettingsScreen
import com.bailensn.liant.ui.settings.SettingsUiState
import com.bailensn.liant.ui.theme.LiantTheme
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * 原生设置页（迁移试水）。
 *
 * 从 Flutter 设置页的临时入口进来，只读本机文件，不连桥接/WebSocket，
 * 所以不会影响 Flutter 侧现有的连接与通知。
 */
class NativeSettingsActivity : ComponentActivity() {
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
                val scope = rememberCoroutineScope()

                LaunchedEffect(Unit) {
                    state = withContext(Dispatchers.IO) { SettingsDataLoader.load(context) }
                }

                SettingsScreen(
                    state = state,
                    onRefresh = {
                        scope.launch {
                            state = withContext(Dispatchers.IO) { SettingsDataLoader.load(context) }
                        }
                    },
                )
            }
        }
    }
}

package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import com.bailensn.liant.platform.Platform
import com.bailensn.liant.platform.currentPlatform

@Composable
fun AppWindow(
    onExit: () -> Unit
) {
    when (currentPlatform) {
        Platform.Mac ->
            MacWindow(onExit = onExit)
        Platform.Windows,
        Platform.Linux ->
            DesktopWindow(onExit = onExit)
    }
}
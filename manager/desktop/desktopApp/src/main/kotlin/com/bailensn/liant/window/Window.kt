package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import com.bailensn.liant.platform.Platform
import com.bailensn.liant.platform.currentPlatform

/**
 * 应用窗口总入口：根据当前系统选择用哪套窗口。
 * Windows / Linux 用我们自绘的无边框窗口 + 小白条；
 * macOS 保留系统的原生标题栏。
 */
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
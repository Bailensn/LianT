package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import androidx.compose.ui.window.Window
import com.bailensn.liant.component.AppContent

/**
 * macOS 窗口：直接用系统的原生标题栏和红绿灯按钮，所以不需要小白条。
 */
@Composable
fun MacWindow(
    onExit: () -> Unit
) {
    Window(
        title = "LianT",
        onCloseRequest = onExit
    ) {
        AppContent()
    }
}
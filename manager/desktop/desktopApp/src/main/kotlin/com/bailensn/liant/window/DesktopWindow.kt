package com.bailensn.liant.window

import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.rememberWindowState
import com.bailensn.liant.component.AppContent
import com.bailensn.liant.titlebar.DesktopTitleBar

/**
 * Windows / Linux 使用的无边框窗口。
 * 隐藏系统标题栏（undecorated），改用我们自己的"小白条"当窗口控制按钮。
 * transparent 让窗口透明，配合后面的玻璃毛玻璃效果。
 */
@Composable
fun DesktopWindow(
    onExit: () -> Unit
) {
    // 用一个 WindowState 统一管理窗口的尺寸/位置/最大化/最小化，改它的属性窗口就会跟着变
    val windowState = rememberWindowState(
        width = 900.dp,
        height = 600.dp,
        position = WindowPosition.Aligned(
            Alignment.Center
        )
    )

    Window(
        undecorated = true,     // 不要系统的窗口边框，用我们自己的小白条
        transparent = true,     // 窗口背景透明，方便做玻璃效果
        state = windowState,
        onCloseRequest = onExit // 关窗口时退出应用
    ) {
        Column {
            DesktopTitleBar(
                isMaximized = windowState.isMaximized,
                onMinimize = {
                    // 点小白条的"最小化" → 窗口最小化
                    windowState.isMinimized = true
                },
                onToggleMaximize = {
                    // 点小白条的"最大化/还原" → 切换状态
                    windowState.isMaximized =
                        !windowState.isMaximized
                },
                onClose = onExit // 点小白条的"关闭" → 退出
            )
            AppContent()
        }
    }
}
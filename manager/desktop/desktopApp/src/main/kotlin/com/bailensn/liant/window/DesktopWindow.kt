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
 * 隐藏系统的标题栏（undecorated），改成我们自绘的透明仿系统标题栏。
 * transparent 让窗口透明，配合后面的玻璃毛玻璃效果。
 */
@Composable
fun DesktopWindow(
    onExit: () -> Unit
) {
    // 用一个 WindowState 统一管理窗口尺寸/位置，改它的属性窗口就会跟着变
    val windowState = rememberWindowState(
        width = 900.dp,
        height = 600.dp,
        position = WindowPosition.Aligned(
            Alignment.Center
        )
    )

    // Window { } 内部自带 WindowScope，所以能直接调用
    // WindowScope.DesktopTitleBar(...)（它就是那个作用域的扩展函数）
    Window(
        undecorated = true,   // 不要系统的标题栏，用我们自己的透明标题栏
        transparent = true,   // 窗口背景透明，方便做玻璃效果
        state = windowState,
        onCloseRequest = onExit // 关窗口时退出应用
    ) {
        Column {
            DesktopTitleBar(
                onClose = onExit // 点标题栏的"关闭" → 退出
            )
            AppContent()
        }
    }
}
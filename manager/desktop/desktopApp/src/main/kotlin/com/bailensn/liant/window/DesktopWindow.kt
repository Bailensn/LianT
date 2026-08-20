package com.bailensn.liant.window

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.rememberWindowState
import com.bailensn.liant.component.AppContent
import com.bailensn.liant.titlebar.DesktopTitleBar

/**
 * Windows / Linux 使用的无边框窗口。
 * 没有系统的标题栏（undecorated），只有一个"漂浮白条"悬浮在内容顶部。
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
        undecorated = true,   // 不要系统的标题栏
        transparent = true,   // 窗口背景透明，方便做玻璃效果
        state = windowState,
        onCloseRequest = onExit // 关窗口时退出应用
    ) {
        // 用 Box 叠加：内容铺满整个窗口，白条漂浮在内容顶部
        Box(modifier = Modifier.fillMaxSize()) {
            AppContent()

            DesktopTitleBar(
                modifier = Modifier
                    .align(Alignment.TopCenter)  // 白条在顶部居中
                    .padding(top = 10.dp)        // 离顶部留一点距离，看起来是"漂浮"
            )
        }
    }
}
package com.bailensn.liant.titlebar

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.WindowScope
import androidx.compose.foundation.window.WindowDraggableArea
import java.awt.Frame
import java.awt.event.WindowStateListener

// 标题栏高度
private val TitleBarHeight = 40.dp

/**
 * 仿系统 · 透明自定义标题栏（Windows/Linux 用）。
 *
 * 布局和系统标题栏一样：
 *   左侧：只显示程序名 "LianT"（不带图标）
 *   右侧：最小化 / 最大化 / 关闭 三件套
 *
 * 为什么这段能用系统窗口能力？因为它是 WindowScope 扩展函数，
 * Window 里的 window（就是系统底层 java.awt.Frame）在这里可以直接拿到，
 * 用它来最小化/最大化最可靠，不会遇到 Compose 版本 API 改动的问题。
 */
@Composable
fun WindowScope.DesktopTitleBar(
    modifier: Modifier = Modifier,
    onClose: () -> Unit
) {
    // 窗口本身（ComposeWindow 是 java.awt.Frame 的子类）
    val frame = window as Frame

    // 记录当前是否最大化，用来给最大化/还原按钮切换图标
    var isMaximized by remember { mutableStateOf(frame.isMaximizedNow) }

    // 监听系统窗口状态变化（比如用系统快捷键最大化后，这里的图标也要跟着变）
    DisposableEffect(frame) {
        val listener = WindowStateListener {
            isMaximized = frame.isMaximizedNow
        }
        frame.addWindowStateListener(listener)
        onDispose { frame.removeWindowStateListener(listener) }
    }

    // 这里包一层 WindowDraggableArea：鼠标按住标题栏（空白处）就能拖动窗口。
    // 三件套按钮会盖在上面，按钮能正常点击，不会被拖动干扰。
    WindowDraggableArea(
        modifier = modifier
            .fillMaxWidth()
            .height(TitleBarHeight)
    ) {
        Row(
            modifier = Modifier.fillMaxSize(),
            verticalAlignment = Alignment.CenterVertically,
            // LianT 靠左，三件套靠右
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            // 左侧：程序名（透明背景，不带图标）
            Text(
                text = "LianT",
                color = MaterialTheme.colorScheme.onSurface,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(start = 16.dp)
            )

            // 把三件套推到最右侧（Spacer 撑满中间空白）
            WindowControls(
                modifier = Modifier.padding(end = 8.dp),
                isMaximized = isMaximized,
                onMinimize = { frame.extendedState = Frame.ICONIFIED },        // 最小化到任务栏
                onToggleMaximize = {
                    // 最大化 ↔ 还原
                    frame.extendedState =
                        if (frame.isMaximizedNow) Frame.NORMAL
                        else Frame.MAXIMIZED_BOTH
                    isMaximized = frame.isMaximizedNow
                },
                onClose = onClose // 关闭 = 退出应用
            )
        }
    }
}

// 用系统状态判断窗口是不是最大化（不依赖 Compose 版本，稳定）
private val Frame.isMaximizedNow: Boolean
    get() = (extendedState and Frame.MAXIMIZED_BOTH) == Frame.MAXIMIZED_BOTH
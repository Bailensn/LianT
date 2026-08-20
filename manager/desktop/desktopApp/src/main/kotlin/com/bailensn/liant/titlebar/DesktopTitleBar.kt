package com.bailensn.liant.titlebar

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowDraggableArea

// 标题栏高度
private val TitleBarHeight = 40.dp

/**
 * Windows / Linux 的自绘标题栏。
 *
 * 作用：
 * 1. 这一条区域可以被鼠标按住拖动整个窗口（WindowDraggableArea）；
 * 2. 左上角放一个"小白条"（三个窗口控制按钮）。
 *
 * 所有参数都有默认值，所以就算不传也能编译，方便单独预览用。
 */
@Composable
fun DesktopTitleBar(
    modifier: Modifier = Modifier,
    isMaximized: Boolean = false,
    onMinimize: () -> Unit = {},
    onToggleMaximize: () -> Unit = {},
    onClose: () -> Unit = {}
) {
    // 包一层 WindowDraggableArea：鼠标按住这条就能拖窗口
    WindowDraggableArea(
        modifier = modifier
            .fillMaxWidth()
            .height(TitleBarHeight)
    ) {
        Box(
            modifier = Modifier.fillMaxSize()
        ) {
            WindowControls(
                modifier = Modifier
                    .align(Alignment.CenterStart)
                    .padding(start = 12.dp),
                isMaximized = isMaximized,
                onMinimize = onMinimize,
                onToggleMaximize = onToggleMaximize,
                onClose = onClose
            )
        }
    }
}
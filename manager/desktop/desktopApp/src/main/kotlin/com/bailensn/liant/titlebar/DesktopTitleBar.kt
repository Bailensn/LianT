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
import androidx.compose.foundation.window.WindowDraggableArea
import androidx.compose.ui.window.WindowScope

// 标题栏高度
private val TitleBarHeight = 40.dp

/**
 * Windows / Linux 的自绘标题栏。
 *
 * 特别注意：这个函数是 WindowScope 的扩展函数。
 * WindowDraggableArea 只有在这个作用域里才能用（它需要知道拖的是哪个窗口），
 * 所以函数前面要写 WindowScope.，调用它的地方（Window{ } 内部）天然就在这个作用域里。
 *
 * 作用：
 * 1. 这一条区域可以被鼠标按住拖动整个窗口（WindowDraggableArea）；
 * 2. 左上角放一个"小白条"（窗口控制按钮）。
 */
@Composable
fun WindowScope.DesktopTitleBar(
    modifier: Modifier = Modifier,
    onMinimize: () -> Unit,
    onClose: () -> Unit
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
                onMinimize = onMinimize,
                onClose = onClose
            )
        }
    }
}
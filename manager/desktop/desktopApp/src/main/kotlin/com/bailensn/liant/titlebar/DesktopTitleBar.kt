package com.bailensn.liant.titlebar

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.window.WindowDraggableArea
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.input.pointer.*
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowScope
import java.awt.Frame
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// 漂浮白条的尺寸（一个居中的小圆角条）
private val BarWidth = 170.dp
private val BarHeight = 30.dp

/**
 * 漂浮白条（Windows / Linux 用）。
 *
 * 它悬浮在 AppContent 顶部，不做标题栏，也不放任何按钮。
 * 交互全靠鼠标手势：
 *   左键按住拖动   → 拖动整个窗口
 *   右键单击      → 窗口最小化
 *   右键双击      → 窗口最大化 / 还原
 *
 * 窗口最小化/最大化直接用底层的 java.awt.Frame.extendedState，
 * 是长期稳定 API，和 Compose 版本无关，不会遇到 API 改名的问题。
 */
@Composable
fun WindowScope.DesktopTitleBar(
    modifier: Modifier = Modifier
) {
    // 窗口本体（ComposeWindow 是 java.awt.Frame 的子类）
    val frame = window as Frame

    // 外包一层 WindowDraggableArea：处理"左键按住拖动窗口"。
    // 内部再叠加自己的手势：单独监听右键单击/双击。
    WindowDraggableArea(
        modifier = modifier
            .width(BarWidth)
            .height(BarHeight)
            .clip(RoundedCornerShape(15.dp))
            .background(
                MaterialTheme.colorScheme.surface
                    .copy(alpha = 0.65f)   // 半透明白条
            )
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .pointerInput(frame) {
                    detectRightClicks(
                        onSingleClick = {
                            // 右键单击 → 最小化到任务栏
                            frame.extendedState = Frame.ICONIFIED
                        },
                        onDoubleClick = {
                            // 右键双击 → 最大化 / 还原
                            frame.extendedState =
                                if (frame.isMaximizedNow) Frame.NORMAL
                                else Frame.MAXIMIZED_BOTH
                        }
                    )
                }
        )
    }
}

/**
 * 识别右键的单击 / 双击。
 * 关键点：单击不能立即触发，要等地一下，
 * 看会不会很快来第二次（那样才算双击）。
 */
private suspend fun PointerInputScope.detectRightClicks(
    onSingleClick: () -> Unit,
    onDoubleClick: () -> Unit
) {
    // 两次点击间隔小于这个值就当作双击
    val doubleClickWindowMillis = 300L
    var lastClickAt = 0L
    var singleJob: Job? = null

    awaitEachGesture {
        // 等到右键按下，再等到右键松开（一次完整的点击）
        awaitSecondaryDown() ?: return@awaitEachGesture
        awaitSecondaryUp()

        val now = System.currentTimeMillis()
        if (lastClickAt != 0L && now - lastClickAt <= doubleClickWindowMillis) {
            // 第二次点击凑成了双击 → 取消之前挂起的单击，只触发双击
            singleJob?.cancel()
            singleJob = null
            lastClickAt = 0L
            onDoubleClick()
        } else {
            // 第一次点击 → 先挂起，等一会儿看是不是双击；不是的话再触发单击
            lastClickAt = now
            singleJob?.cancel()
            singleJob = launch {
                delay(doubleClickWindowMillis)
                onSingleClick()
            }
        }
    }
}

/** 一直等到出现"鼠标右键按下"的那一帧；若是其它键按下则继续等。 */
private suspend fun AwaitPointerEventScope.awaitSecondaryDown(): PointerEvent? {
    while (true) {
        val event = awaitPointerEvent(PointerEventPass.Main)
        if (event.buttons.isSecondaryPressed && event.changes.any { it.pressed }) {
            return event
        }
    }
}

/** 一直等到"鼠标右键松开"（一次点击结束）。 */
private suspend fun AwaitPointerEventScope.awaitSecondaryUp(): Boolean {
    while (true) {
        val event = awaitPointerEvent(PointerEventPass.Main)
        if (!event.buttons.isSecondaryPressed || event.changes.none { it.pressed }) {
            return true
        }
    }
}

/** 用系统状态判断窗口是不是最大化（不依赖 Compose 版本，稳定） */
private val Frame.isMaximizedNow: Boolean
    get() = (extendedState and Frame.MAXIMIZED_BOTH) == Frame.MAXIMIZED_BOTH
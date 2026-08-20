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
import androidx.compose.ui.input.pointer.AwaitPointerEventScope
import androidx.compose.ui.input.pointer.PointerEvent
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.PointerInputScope
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowScope
import java.awt.Frame
import kotlinx.coroutines.withTimeoutOrNull

// 漂浮白条的尺寸（一个居中的小圆角条）
private val BarWidth = 170.dp
private val BarHeight = 30.dp

/** 两次右键点击间隔小于这个值就当作双击 */
private const val DoubleClickWindowMillis = 300L

/**
 * 漂浮白条（Windows / Linux 用）。
 *
 * 它悬浮在 AppContent 顶部，不做标题栏，也不放任何按钮。
 * 交互全靠鼠标手势：
 *   左键按住拖动   → 拖动整个窗口
 *   右键单击      → 窗口最小化
 *   右键双击      → 窗口最大化 / 还原
 *
 * 左键拖动用官方 [WindowDraggableArea]；
 * 右键单击/双击用自定义手势（稳定 API：PointerInputScope + buttons）自己区分。
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
 *
 * 逻辑：一次右键完整点击（按下→松开）后，先等地一下（DoubleClickWindowMillis），
 * 如果这段时间内又来了一次"右键按下"，就说明是双击；否则就是单击。
 *
 * 全部用挂起等待实现（withTimeoutOrNull / awaitPointerEvent），
 * 不需要额外的 CoroutineScope，能安全地在 PointerInputScope 里运行。
 */
private suspend fun PointerInputScope.detectRightClicks(
    onSingleClick: () -> Unit,
    onDoubleClick: () -> Unit
) = awaitEachGesture {
    // 一次完整的右键点击 = 右键按下 → 右键松开
    awaitSecondaryDown()
    awaitSecondaryUp()

    // 抬起后再等地一下，看是否立刻又按下（⇢ 那是第二次点击 = 双击）
    val secondClickCame =
        withTimeoutOrNull(DoubleClickWindowMillis) {
            awaitSecondaryDown()
        } != null

    if (secondClickCame) {
        // 是双击：等第二次点击松开，然后触发双击
        awaitSecondaryUp()
        onDoubleClick()
    } else {
        // 是单击
        onSingleClick()
    }
}

/** 一直等到出现"鼠标右键按下"的那一帧。 */
private suspend fun AwaitPointerEventScope.awaitSecondaryDown(): PointerEvent {
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
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
import androidx.compose.ui.input.pointer.isSecondaryPressed
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowScope
import java.awt.Frame
import kotlinx.coroutines.withTimeoutOrNull

private val BarWidth = 170.dp
private val BarHeight = 17.dp

private const val DoubleClickWindowMillis = 300L

@Composable
fun WindowScope.DesktopTitleBar(
    modifier: Modifier = Modifier
) {
    val frame = window as Frame
    WindowDraggableArea(
        modifier = modifier
            .width(BarWidth)
            .height(BarHeight)
            .clip(RoundedCornerShape(15.dp))
            .background(
                MaterialTheme.colorScheme.surface
                    .copy(alpha = 0.5f)
            )
    ) {
        Box(
            modifier = Modifier
                .fillMaxSize()
                .pointerInput(frame) {
                    detectRightClicks(
                        onSingleClick = {
                            frame.extendedState = Frame.ICONIFIED
                        },
                        onDoubleClick = {
                            frame.extendedState =
                                if (frame.isMaximizedNow) Frame.NORMAL
                                else Frame.MAXIMIZED_BOTH
                        }
                    )
                }
        )
    }
}

private suspend fun PointerInputScope.detectRightClicks(
    onSingleClick: () -> Unit,
    onDoubleClick: () -> Unit
) = awaitEachGesture {
    awaitSecondaryDown()
    awaitSecondaryUp()
    val secondClickCame =
        withTimeoutOrNull(DoubleClickWindowMillis) {
            awaitSecondaryDown()
        } != null

    if (secondClickCame) {
        awaitSecondaryUp()
        onDoubleClick()
    } else {
        // 是单击
        onSingleClick()
    }
}

private suspend fun AwaitPointerEventScope.awaitSecondaryDown(): PointerEvent {
    while (true) {
        val event = awaitPointerEvent(PointerEventPass.Main)
        if (event.buttons.isSecondaryPressed && event.changes.any { it.pressed }) {
            return event
        }
    }
}

private suspend fun AwaitPointerEventScope.awaitSecondaryUp(): Boolean {
    while (true) {
        val event = awaitPointerEvent(PointerEventPass.Main)
        if (!event.buttons.isSecondaryPressed || event.changes.none { it.pressed }) {
            return true
        }
    }
}

private val Frame.isMaximizedNow: Boolean
    get() = (extendedState and Frame.MAXIMIZED_BOTH) == Frame.MAXIMIZED_BOTH
package com.bailensn.liant

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.WindowDraggableArea

@Composable
fun WindowRoot() {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .clip(
                RoundedCornerShape(
                    24.dp
                )
            )
    ) {
        LianTTheme {
            DesktopDragLayout {
                TheLayout()
            }
        }
    }
}


@Composable
fun DesktopDragLayout(
    content: @Composable () -> Unit
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
    ) {
        content()
        WindowDraggableArea {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(36.dp)
                    .background(
                        color = Color.White.copy(
                            alpha = 0.85f
                        ),
                        shape = RoundedCornerShape(
                            bottomStart = 16.dp,
                            bottomEnd = 16.dp
                        )
                    )
            )
        }
    }
}
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
import org.jetbrains.compose.resources.painterResource
import com.bailensn.liant.Res
import com.bailensn.liant.ic_launcher

@Composable
fun DesktopWindow(
    onExit: () -> Unit
) {
    val windowState = rememberWindowState(
        width = 900.dp,
        height = 600.dp,
        position = WindowPosition.Aligned(
            Alignment.Center
        )
    )

    Window(
        undecorated = true,
        transparent = true,
        state = windowState,
        onCloseRequest = onExit,
        title = "LianT",
        icon = painterResource(Res.drawable.ic_launcher)
    ) {
        Box(modifier = Modifier.fillMaxSize()) {
            AppContent()

            DesktopTitleBar(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(top = 13.dp)
            )
        }
    }
}
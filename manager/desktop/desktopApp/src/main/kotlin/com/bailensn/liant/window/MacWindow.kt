package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import androidx.compose.ui.window.Window
import com.bailensn.liant.component.AppContent

@Composable
fun MacWindow(
    onExit: () -> Unit
) {
    Window(
        title = "LianT",
        onCloseRequest = onExit
    ) {
        AppContent()
    }
}
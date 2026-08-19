package com.bailensn.liant

import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.*
import androidx.compose.ui.Alignment

fun main() = application {
    val windowState = rememberWindowState(
        width = 430.dp,
        height = 800.dp,
        position = WindowPosition.Aligned(
            Alignment.Center
        )
    )

    Window(
        onCloseRequest = ::exitApplication,
        title = "LianT",
        state = windowState,
        undecorated = true,
        transparent = true,
        resizable = true
    ) {
        WindowRoot()
    }
}
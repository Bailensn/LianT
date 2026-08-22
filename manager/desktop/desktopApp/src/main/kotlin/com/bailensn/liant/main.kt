package com.bailensn.liant

import androidx.compose.ui.window.application
import com.bailensn.liant.theme.LianTTheme
import com.bailensn.liant.window.AppWindow

fun main() {
    application {
        LianTTheme {
            AppWindow(
                onExit = { exitApplication() }
            )
        }
    }
}
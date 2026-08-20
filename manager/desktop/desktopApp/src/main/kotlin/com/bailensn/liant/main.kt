package com.bailensn.liant

import androidx.compose.ui.window.application
import androidx.compose.ui.window.exitApplication
import com.bailensn.liant.theme.LianTTheme
import com.bailensn.liant.window.AppWindow

fun main() {
    application {
        // 给整个应用套上统一主题
        LianTTheme {
            // onExit = 点击小白条上的"关闭"按钮时执行 → 退出程序
            AppWindow(
                onExit = { exitApplication() }
            )
        }
    }
}
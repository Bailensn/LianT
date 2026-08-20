package com.bailensn.liant

import androidx.compose.ui.window.application
import com.bailensn.liant.theme.LianTTheme
import com.bailensn.liant.window.AppWindow

fun main() {
    application {
        // 给整个应用套上统一主题
        LianTTheme {
            // onExit = 点击小白条上的"关闭"按钮时执行 → 退出程序
            // 注意：exitApplication 是 application { } 作用域自带的，不用、也不能 import
            AppWindow(
                onExit = { exitApplication() }
            )
        }
    }
}
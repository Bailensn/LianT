package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import androidx.compose.ui.window.*
import com.bailensn.liant.component.AppContent
import androidx.compose.ui.window.exitApplication

@Composable
fun MacWindow(){
    Window(
        onCloseRequest =
            ::exitApplication,
        title =
            "LianT"
    ){
        AppContent()
    }
}
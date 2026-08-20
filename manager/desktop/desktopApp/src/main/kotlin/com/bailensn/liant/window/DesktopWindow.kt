package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.window.*
import androidx.compose.ui.unit.*
import com.bailensn.liant.component.AppContent
import com.bailensn.liant.titlebar.DesktopTitleBar

@Composable
fun DesktopWindow(){
    Window(
        onCloseRequest =
            ::exitApplication,
        undecorated = true,
        transparent = true,
        state =
            WindowState(
                width = 900.dp,
                height = 600.dp,
                position =
                    WindowPosition.Aligned(
                        Alignment.Center
                    )
            )
    ){
        Column {
            DesktopTitleBar()
            AppContent()
        }
    }
}
package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import androidx.compose.ui.window.*
import androidx.compose.ui.unit.*
import com.bailensn.liant.component.AppContent
import com.bailensn.liant.titlebar.DesktopTitleBar

@Composable
fun DesktopWindow(){
    Window(
        onCloseRequest = ::exitApplication,
        undecorated = true,
        transparent = true
    ){
        DesktopDragLayout {
            Column {
                DesktopTitleBar()
                AppContent()
            }
        }
    }
}
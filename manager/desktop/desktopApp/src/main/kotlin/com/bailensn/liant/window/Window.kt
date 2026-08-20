package com.bailensn.liant.window

import androidx.compose.runtime.Composable
import com.bailensn.liant.platform.*

@Composable
fun AppWindow(){
    when(currentPlatform){
        Platform.Mac ->
            MacWindow()
        Platform.Windows,
        Platform.Linux ->
            DesktopWindow()
    }
}
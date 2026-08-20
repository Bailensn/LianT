package com.bailensn.liant.titlebar

import androidx.compose.runtime.Composable
import com.bailensn.liant.platform.Platform
import com.bailensn.liant.platform.currentPlatform

@Composable
fun TitleBar(){
    when(currentPlatform){
        Platform.Mac -> {
            MacTitleBar()
        }
        Platform.Windows,
        Platform.Linux -> {
            DesktopTitleBar()
        }
    }
}
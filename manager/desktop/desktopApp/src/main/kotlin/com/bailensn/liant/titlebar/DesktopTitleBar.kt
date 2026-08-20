package com.bailensn.liant.titlebar

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.unit.dp
import com.bailensn.liant.component.GlassPanel

@Composable
fun DesktopTitleBar(){
    var expand by remember {
        mutableStateOf(false)
    }
    Box(
        modifier =
            Modifier
                .fillMaxWidth()
                .height(36.dp)
                .background(
                    Color.White
                )
                .pointerInput(Unit){
                    awaitPointerEventScope {
                        while(true){
                            val event = awaitPointerEvent()
                            if(event.buttons.isSecondaryPressed){
                                expand = !expand
                            }
                        }
                    }
                }
    ){
        if(expand){
            GlassPanel(
                onFullScreen = {

                },
                onMinimize = {

                },
                onClose = {

                }
            )
        }
    }
}
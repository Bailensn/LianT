package com.bailensn.liant.component

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

@Composable
fun GlassPanel(
    onFullScreen:()->Unit,
    onMinimize:()->Unit,
    onClose:()->Unit
){
    Row(
        modifier =
            Modifier
                .background(
                    Color.White.copy(
                        alpha = 0.75f
                    ),
                    RoundedCornerShape(
                        18.dp
                    )
                )
                .padding(10.dp),
        horizontalArrangement =
            Arrangement.spacedBy(8.dp)
    ){
        Button(
            onClick = onFullScreen
        ){
            Text("⛶")
        }
        Button(
            onClick = onMinimize
        ){
            Text("—")
        }
        Button(
            onClick = onClose
        ){
            Text("×")
        }
    }
}
package com.bailensn.liant

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application

@Composable
fun App() {
    var visible by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { visible = true }

    MaterialTheme(
        colorScheme = lightColorScheme(primary = Color(0xFF3278DC)) // 你原来的蓝
    ) {
        AnimatedVisibility(visible, enter = fadeIn(tween(200))) {
            Surface(
                color = MaterialTheme.colorScheme.primary,
                shape = RoundedCornerShape(25.dp),
                modifier = Modifier.fillMaxSize().padding(8.dp)
            ) {
                Column(
                    Modifier.fillMaxSize().padding(24.dp),
                    verticalArrangement = Arrangement.spacedBy(80.dp)
                ) {
                    Text("联T", style = MaterialTheme.typography.headlineLarge, color = Color.White)
                    Button(
                        onClick = { exitApplication() },
                        colors = ButtonDefaults.buttonColors(
                            containerColor = Color.White,
                            contentColor = Color(0xFF3278DC)
                        )
                    ) { Text("退出") }
                }
            }
        }
    }
}

fun main() = application {
    Window(onCloseRequest = ::exitApplication, title = "LianT") {
        App()
    }
}

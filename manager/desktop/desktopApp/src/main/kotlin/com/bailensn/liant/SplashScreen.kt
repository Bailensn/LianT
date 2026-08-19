package com.bailensn.liant

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun LianTTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme =
        if (darkTheme) {
            darkColorScheme()
        } else {
            lightColorScheme()
        }
    MaterialTheme(
        colorScheme = colorScheme,
        shapes = Shapes(
            large = RoundedCornerShape(32.dp),
            extraLarge = RoundedCornerShape(36.dp)
        ),
        content = content
    )
}

@Composable
fun TheLayout() {
    val textColor =
        if (isSystemInDarkTheme())
            Color.White
        else
            Color.Black
    val iconSize = 76.dp
    Box(
        modifier = Modifier
            .fillMaxSize()
    ) {
        LiquidGlassContainer(
            modifier = Modifier.fillMaxSize(),
            background = {
                AnimatedGradientBackground(
                    modifier = Modifier.fillMaxSize()
                )
            }
        ) { backdrop ->
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(
                        horizontal = 24.dp
                    )
                    .padding(
                        top = 70.dp
                    ),
                verticalArrangement =
                    Arrangement.SpaceBetween
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth(),
                    horizontalAlignment =
                        Alignment.Start
                ) {
                    FloatingGlassIcon(
                        backdrop = backdrop,
                        size = iconSize,
                        iconRes = "ic_icon"
                    )
                    Spacer(
                        modifier =
                            Modifier.height(24.dp)
                    )
                    Text(
                        text = "欢迎回来",
                        fontSize = 30.sp,
                        fontWeight =
                            FontWeight.Bold,
                        color = textColor
                    )
                    Spacer(
                        modifier =
                            Modifier.height(8.dp)
                    )
                    Text(
                        text = "请选择要登录的 bot",
                        fontSize = 17.sp,
                        color =
                            textColor.copy(
                                alpha = 0.75f
                            )
                    )
                }
                GlassSheet(
                    backdrop = backdrop,
                    shape =
                        RoundedCornerShape(
                            32.dp
                        ),
                    blurRadius = 4.dp,
                    refractionAmount = 50.dp,
                    modifier =
                        Modifier
                            .fillMaxWidth()
                            .padding(
                                bottom = 32.dp
                            )
                ) {
                    Column(
                        modifier =
                            Modifier
                                .fillMaxWidth()
                                .padding(12.dp)
                    ) {
                        Spacer(
                            modifier =
                                Modifier.height(
                                    600.dp
                                )
                        )
                    }
                }
            }
        }
    }
}
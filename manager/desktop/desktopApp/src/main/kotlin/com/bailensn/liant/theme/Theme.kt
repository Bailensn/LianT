package com.bailensn.liant.theme

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

/** 应用统一主题：根据系统深浅色自动切换亮色/暗色配色 */
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
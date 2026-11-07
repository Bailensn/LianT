package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Chat
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SmartToy
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import top.yukonga.miuix.kmp.basic.FloatingNavigationBar
import top.yukonga.miuix.kmp.basic.FloatingNavigationBarItem
import top.yukonga.miuix.kmp.blur.LayerBackdrop
import top.yukonga.miuix.kmp.blur.ProgressiveBlur
import top.yukonga.miuix.kmp.blur.drawBackdrop
import top.yukonga.miuix.kmp.blur.progressiveBlur

/**
 * iOS 式悬浮底栏：浮动胶囊 + 背后的渐进模糊。
 *
 * 模糊来自 miuix-blur 的 backdrop 管线（API 33+，低版本自动退化为纯色），
 * 参考库自带 uitest 里的 FloatingNavigationBar 用法。
 */
@Composable
fun BottomTabBar(
    backdrop: LayerBackdrop,
    selected: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    val shape = RoundedCornerShape(28.dp)
    FloatingNavigationBar(
        modifier = modifier
            .padding(horizontal = 24.dp, vertical = 12.dp)
            .drawBackdrop(
                backdrop = backdrop,
                shape = { shape },
                effects = {
                    progressiveBlur(radiusX = 28f, radiusY = 28f, gradient = ProgressiveBlur.Bottom)
                },
                progressiveGradient = ProgressiveBlur.Bottom,
            ),
        color = Color.Transparent,
        cornerRadius = 28.dp,
        showDivider = false,
    ) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 8.dp),
            horizontalArrangement = Arrangement.SpaceEvenly,
        ) {
            FloatingNavigationBarItem(
                selected = selected == 0,
                onClick = { onSelect(0) },
                icon = Icons.Filled.Chat,
                label = "消息",
            )
            FloatingNavigationBarItem(
                selected = selected == 1,
                onClick = { onSelect(1) },
                icon = Icons.Filled.SmartToy,
                label = "Bot",
            )
            FloatingNavigationBarItem(
                selected = selected == 2,
                onClick = { onSelect(2) },
                icon = Icons.Filled.Settings,
                label = "设置",
            )
        }
    }
}

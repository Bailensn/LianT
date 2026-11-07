package com.bailensn.liant.ui

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Chat
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SmartToy
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.bailensn.liant.ui.liquid.IosLiquidGlassNavigationBar
import top.yukonga.miuix.kmp.basic.NavigationItem
import top.yukonga.miuix.kmp.blur.LayerBackdrop

/** iOS 液态玻璃底栏，直接移植 Miuix uitest 的 LiquidGlassNavigationBar。 */
@Composable
fun BottomTabBar(
    backdrop: LayerBackdrop?,
    selected: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    IosLiquidGlassNavigationBar(
        items = listOf(
            NavigationItem(label = "消息", icon = Icons.Filled.Chat),
            NavigationItem(label = "Bot", icon = Icons.Filled.SmartToy),
            NavigationItem(label = "设置", icon = Icons.Filled.Settings),
        ),
        selectedIndex = selected,
        onItemClick = onSelect,
        backdrop = backdrop,
        isBlurActive = backdrop != null,
        modifier = modifier,
    )
}

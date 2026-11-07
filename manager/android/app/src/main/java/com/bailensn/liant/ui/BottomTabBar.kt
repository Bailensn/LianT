package com.bailensn.liant.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Chat
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.SmartToy
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import top.yukonga.miuix.kmp.basic.Icon
import top.yukonga.miuix.kmp.basic.NavigationBar
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.theme.MiuixTheme

/** 3 槽位底部导航，直接用 Miuix NavigationBar，安全区由组件自己处理。 */
@Composable
fun BottomTabBar(selected: Int, onSelect: (Int) -> Unit) {
    NavigationBar {
        TabItem(this, 0, Icons.Filled.Chat, "消息", selected, onSelect)
        TabItem(this, 1, Icons.Filled.SmartToy, "Bot", selected, onSelect)
        TabItem(this, 2, Icons.Filled.Settings, "设置", selected, onSelect)
    }
}

@Composable
private fun RowScope.TabItem(
    scope: RowScope,
    index: Int,
    icon: ImageVector,
    label: String,
    selected: Int,
    onSelect: (Int) -> Unit,
) {
    val active = index == selected
    val tint = if (active) MiuixTheme.colorScheme.primary else MiuixTheme.colorScheme.onSurfaceSecondary
    Column(
        Modifier
            .weight(1f)
            .heightIn(min = 48.dp)
            .clickable { onSelect(index) }
            .padding(vertical = 6.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Icon(
            imageVector = icon,
            contentDescription = label,
            modifier = Modifier.size(22.dp),
            tint = tint,
        )
        Text(
            text = label,
            fontSize = 11.sp,
            color = tint,
        )
    }
}

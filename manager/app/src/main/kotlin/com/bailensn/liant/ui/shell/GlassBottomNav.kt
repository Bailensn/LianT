package com.bailensn.liant.ui.shell

import androidx.compose.animation.core.animateDpAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.SmartToy
import androidx.compose.material.icons.rounded.ChatBubble
import androidx.compose.material.icons.rounded.ChatBubbleOutline
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material.icons.rounded.SmartToy
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.theme.LTokens
import com.bailensn.liant.ui.theme.LocalLColors
import com.bailensn.liant.ui.theme.SquircleShape

/**
 * 玻璃底栏（其实是实心 elevated + 阴影 + 0.5dp 描边，Flutter 版没有用 BackdropFilter）。
 *
 * 几何照 `lib/ui/widgets.dart` 的 GlassBottomNav：
 * 外层 28 的连续曲率圆角（×1.6 后按短边钳制）、内边距 6、内容高 52；
 * 选中指示块 accent 12% 圆角 22、三个等宽 tab；图标 21、标签 11。
 */
@Composable
fun GlassBottomNav(
    index: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier,
) {
    val colors = LocalLColors.current
    val items = navItems()
    val shape = SquircleShape((28f * LTokens.SquircleScale).dp)
    val haptics = LocalHapticFeedback.current

    Box(
        modifier = modifier
            .shadow(
                elevation = 12.dp,
                shape = shape,
                clip = false,
                ambientColor = Color.Black.copy(alpha = 0.14f),
                spotColor = Color.Black.copy(alpha = 0.14f),
            )
            .clip(shape)
            .background(colors.elevated)
            .border(0.5.dp, colors.sep, shape)
            .padding(6.dp),
    ) {
        BoxWithConstraints {
            val itemWidth = maxWidth / items.size
            val offset by animateDpAsState(
                targetValue = itemWidth * index,
                label = "nav-indicator",
            )
            Box(
                Modifier
                    .offset(x = offset)
                    .width(itemWidth)
                    .height(52.dp)
                    .clip(SquircleShape((22f * LTokens.SquircleScale).dp))
                    .background(colors.accent.copy(alpha = 0.12f)),
            )
            Row(Modifier.height(52.dp)) {
                items.forEachIndexed { i, item ->
                    val selected = i == index
                    Column(
                        modifier = Modifier
                            .width(itemWidth)
                            .height(52.dp)
                            .selectable(
                                selected = selected,
                                onClick = {
                                    if (i != index) {
                                        haptics.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                        onSelect(i)
                                    }
                                },
                            ),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        // 上下居中：图标 21 + 2 间距 + 标签 11。
                        Box(Modifier.height(52.dp), contentAlignment = Alignment.Center) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(
                                    imageVector = if (selected) item.selectedIcon else item.icon,
                                    contentDescription = item.label,
                                    tint = if (selected) colors.accent else colors.text2,
                                    modifier = Modifier.size(21.dp),
                                )
                                Box(Modifier.height(2.dp))
                                Text(
                                    text = item.label,
                                    color = if (selected) colors.accent else colors.text2,
                                    fontSize = 11.sp,
                                    fontWeight = if (selected) {
                                        androidx.compose.ui.text.font.FontWeight.W600
                                    } else {
                                        androidx.compose.ui.text.font.FontWeight.W500
                                    },
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

private class NavItem(
    val icon: ImageVector,
    val selectedIcon: ImageVector,
    val label: String,
)

private fun navItems(): List<NavItem> = listOf(
    NavItem(Icons.Rounded.ChatBubbleOutline, Icons.Rounded.ChatBubble, "会话"),
    NavItem(Icons.Outlined.SmartToy, Icons.Rounded.SmartToy, "机器人"),
    NavItem(Icons.Outlined.Settings, Icons.Rounded.Settings, "设置"),
)

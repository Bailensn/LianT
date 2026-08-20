package com.bailensn.liant.titlebar

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.hoverable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsHoveredAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 仿系统标题栏 · 窗口控制三件套
 *
 * 右侧三个圆点按钮，和 Windows 系统标题栏一样：
 *   ─  最小化
 *   ▢  最大化 ▣（还原）
 *   ✕  关闭
 *
 * 它是**透明**的（没有背景），只画三个半透明圆点，鼠标悬停会变亮一点。
 * 它只通过回调告诉外面"点了哪个"，不关心窗口是谁，所以复用性强。
 */
@Composable
fun WindowControls(
    modifier: Modifier = Modifier,
    isMaximized: Boolean,
    onMinimize: () -> Unit,
    onToggleMaximize: () -> Unit,
    onClose: () -> Unit
) {
    // 就用主题里的背景色稍微加深一点点来做圆点，深色浅色主题都自然
    val dotColor = MaterialTheme.colorScheme.onSurface

    Row(
        modifier = modifier,
        verticalAlignment = Alignment.CenterVertically
    ) {
        // 最小化
        ControlDot(
            label = "─",
            color = dotColor.copy(alpha = 0.45f),
            hoverColor = dotColor.copy(alpha = 0.70f)
        ) { onMinimize() }

        // 最大化 / 还原（图标会随状态切换）
        ControlDot(
            label = if (isMaximized) "▣" else "▢",
            color = dotColor.copy(alpha = 0.45f),
            hoverColor = dotColor.copy(alpha = 0.70f)
        ) { onToggleMaximize() }

        // 关闭（红色，提醒这个是危险操作）
        ControlDot(
            label = "✕",
            color = Color(0x99E05050),
            hoverColor = Color(0xFFE05050)
        ) { onClose() }
    }
}

/** 一个带悬停高亮的半透明圆点按钮 */
@Composable
private fun ControlDot(
    label: String,
    color: Color,
    hoverColor: Color,
    onClick: () -> Unit
) {
    // 用于感知鼠标是否悬停在按钮上（记住它，避免每次重组都新建）
    val interaction = remember { MutableInteractionSource() }
    val isHovered by interaction.collectIsHoveredAsState()

    Box(
        modifier = Modifier
            .size(22.dp)
            .clip(CircleShape)
            .clickable(
                interactionSource = interaction,
                indication = null, // 点按不给水波纹，保持系统那种干净样式
                onClick = onClick
            )
            .hoverable(interaction)
            .background(if (isHovered) hoverColor else color),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = label,
            color = Color.White,
            fontSize = 10.sp,
            fontWeight = FontWeight.Bold
        )
    }
}
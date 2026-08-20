package com.bailensn.liant.titlebar

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 小白条 · 窗口控制条
 *
 * 一个圆角的小白条，装着三个圆形按钮：
 *   ─  最小化
 *   ▢  最大化 / 还原
 *   ✕  关闭
 *
 * 它自己不关心窗口是谁，只负责把"用户点了哪个按钮"用回调告诉外面。
 * 这样不管在哪个平台复用都很灵活，也方便测试。
 */
@Composable
fun WindowControls(
    modifier: Modifier = Modifier,
    isMaximized: Boolean = false,
    onMinimize: () -> Unit,
    onToggleMaximize: () -> Unit,
    onClose: () -> Unit
) {
    Row(
        modifier = modifier
            .clip(RoundedCornerShape(16.dp))       // 圆角外框
            .background(                            // 白色半透明小条
                MaterialTheme.colorScheme.surface
                    .copy(alpha = 0.85f)
            )
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(2.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        // 最小化
        ControlDot(
            label = "─",
            color = Color(0xFF9AA0A8)
        ) { onMinimize() }

        // 最大化 / 还原（图标会随状态切换）
        ControlDot(
            label = if (isMaximized) "▣" else "▢",
            color = Color(0xFF757B84)
        ) { onToggleMaximize() }

        // 关闭（红色，提醒这个是危险操作）
        ControlDot(
            label = "✕",
            color = Color(0xFFE14C4C)
        ) { onClose() }
    }
}

/** 小白条里的单个圆形按钮 */
@Composable
private fun ControlDot(
    label: String,
    color: Color,
    onClick: () -> Unit
) {
    Box(
        modifier = Modifier
            .size(22.dp)
            .clip(CircleShape)
            .background(color)
            .clickable(onClick = onClick),
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
package com.bailensn.liant.ui.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.theme.LTokens
import com.bailensn.liant.ui.theme.LocalLColors
import com.bailensn.liant.ui.theme.SquircleShape

/** 空状态，照 Flutter 的 `EmptyState`：56 方块图标 + 标题 + 提示。 */
@Composable
fun EmptyState(icon: ImageVector, title: String, hint: String) {
    val colors = LocalLColors.current
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 24.dp, vertical = 56.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier
                .size(56.dp)
                .clip(SquircleShape((18f * LTokens.SquircleScale).dp))
                .background(colors.sunken),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, contentDescription = null, tint = colors.text3, modifier = Modifier.size(26.dp))
        }
        Spacer(Modifier.height(16.dp))
        Text(text = title, color = colors.text, fontSize = 17.sp, textAlign = TextAlign.Center)
        Spacer(Modifier.height(6.dp))
        Text(
            text = hint,
            color = colors.text3,
            fontSize = 13.sp,
            lineHeight = 20.8.sp,
            textAlign = TextAlign.Center,
        )
    }
}

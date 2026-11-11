package com.bailensn.liant.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.theme.LColors
import com.bailensn.liant.ui.theme.LTokens
import com.bailensn.liant.ui.theme.LocalLColors

/**
 * 原生侧的通用组件，逐项对应 Flutter 的 `lib/ui/widgets.dart`：
 * GroupCard / Hairline / ToneBadge / LargeTitle 头部。
 */

/** 0.5dp 分隔线；卡片内的行之间用它，缩进对齐文字起点。 */
@Composable
fun Hairline(indent: Dp = 0.dp, color: Color? = null) {
    Spacer(
        Modifier
            .padding(start = indent)
            .fillMaxWidth()
            .height(0.5.dp)
            .background(color ?: LocalLColors.current.sep),
    )
}

enum class BadgeTone { Ok, Warn, Accent, Muted }

/** 小圆角标签：在线 / 重连中 / 本机。 */
@Composable
fun ToneBadge(label: String, tone: BadgeTone = BadgeTone.Muted) {
    val colors = LocalLColors.current
    val color = when (tone) {
        BadgeTone.Ok -> colors.ok
        BadgeTone.Warn -> colors.danger
        BadgeTone.Accent -> colors.accent
        BadgeTone.Muted -> colors.text2
    }
    Text(
        text = label,
        color = color,
        fontSize = 12.sp,
        fontWeight = FontWeight.W600,
        modifier = Modifier
            .clip(RoundedCornerShape(20.dp))
            .background(color.copy(alpha = 0.12f))
            .padding(horizontal = 9.dp, vertical = 3.dp),
    )
}

/**
 * 分组卡片：小标题 + 圆角卡片 + 行间 0.5dp 分隔线 + 可选的脚注。
 *
 * 圆角用放大的普通圆角近似 Flutter 的连续曲率（见 LTokens.SquircleScale）。
 */
@Composable
fun GroupCard(
    title: String? = null,
    footer: String? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = LocalLColors.current
    Column(Modifier.fillMaxWidth().padding(bottom = 24.dp)) {
        if (title != null) {
            Text(
                text = title,
                color = colors.text2,
                fontSize = 13.sp,
                fontWeight = FontWeight.W500,
                letterSpacing = 0.2.sp,
                modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 8.dp),
            )
        }
        Surface(
            color = colors.elevated,
            shape = RoundedCornerShape(LTokens.cardShapeRadius),
            modifier = Modifier.fillMaxWidth(),
        ) {
            // 行之间由调用方插 Hairline(dividerIndent)：Compose 没有「枚举子项」，
            // 这样比在 GroupCard 里猜哪两行之间要线更直白。
            Column(content = content)
        }
        if (footer != null) {
            Text(
                text = footer,
                color = colors.text3,
                fontSize = 12.sp,
                lineHeight = 18.sp,
                modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp),
            )
        }
    }
}

/** 设置页的一行：标题 + 可选副标题 + 右侧标签，可点击。 */
@Composable
fun SettingsRow(
    title: String,
    subtitle: String? = null,
    trailing: (@Composable () -> Unit)? = null,
    danger: Boolean = false,
    monospace: Boolean = false,
    onClick: (() -> Unit)? = null,
) {
    val colors = LocalLColors.current
    val titleColor = if (danger) colors.danger else colors.text
    val subtitleColor = if (danger) colors.danger else colors.text3
    val modifier = Modifier
        .fillMaxWidth()
        .defaultMinSize(minHeight = 52.dp)
        .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
        .padding(start = 16.dp, end = 16.dp, top = 12.dp, bottom = 12.dp)
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.Center) {
            Text(
                text = title,
                color = titleColor,
                fontSize = 16.sp,
                lineHeight = 22.sp,
            )
            if (!subtitle.isNullOrEmpty()) {
                Spacer(Modifier.height(3.dp))
                Text(
                    text = subtitle,
                    color = subtitleColor,
                    fontSize = 12.sp,
                    lineHeight = 18.sp,
                    fontFamily = if (monospace) FontFamily.Monospace else null,
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        if (trailing != null) {
            Spacer(Modifier.width(12.dp))
            trailing()
        }
    }
}

/** 大标题头：34sp、字重与字距对齐 Flutter 的 displaySmall。 */
@Composable
fun LargeTitleHeader(
    title: String,
    subtitle: String? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    val colors = LocalLColors.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 16.dp, end = 16.dp, top = 10.dp, bottom = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                text = title,
                style = MaterialTheme.typography.displaySmall,
                color = colors.text,
            )
            if (subtitle != null) {
                Spacer(Modifier.height(2.dp))
                Text(text = subtitle, color = colors.text3, fontSize = 13.sp)
            }
        }
        trailing?.invoke()
    }
}

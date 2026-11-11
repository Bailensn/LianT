package com.bailensn.liant.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 视觉 token，逐值对应 Flutter 的 `lib/ui/tokens.dart`。
 *
 * 这里是原生 UI 的唯一颜色来源；加颜色先改这里，再改 Flutter 那份，
 * 两边对齐后再删 Flutter。
 */
data class LColors(
    val bg: Color,
    val elevated: Color,
    val sunken: Color,
    val text: Color,
    val text2: Color,
    val text3: Color,
    val sep: Color,
    val accent: Color,
    val danger: Color,
    val ok: Color,
) {
    companion object {
        val Light = LColors(
            bg = Color(0xFFF4F4F7),
            elevated = Color(0xFFFFFFFF),
            sunken = Color(0xFFEAEAEF),
            text = Color(0xFF111114),
            text2 = Color(0xFF616168),
            text3 = Color(0xFF8A8A92),
            sep = Color(0x14000000),
            accent = Color(0xFF2F6BFF),
            danger = Color(0xFFD93A40),
            ok = Color(0xFF2E9E63),
        )

        /** 深色背景是纯黑（OLED 省电），卡片提亮一级。 */
        val Dark = LColors(
            bg = Color(0xFF0E0E10),
            elevated = Color(0xFF1B1B1F),
            sunken = Color(0xFF141417),
            text = Color(0xFFF4F4F7),
            text2 = Color(0xFFA6A6AE),
            text3 = Color(0xFF7E7E87),
            sep = Color(0x1FFFFFFF),
            accent = Color(0xFF4C7DFF),
            danger = Color(0xFFFF6B6B),
            ok = Color(0xFF4CD07D),
        )
    }
}

object LTokens {
    val cardRadius = 22.dp
    val controlRadius = 18.dp
    val largeTitle = 34.sp
    val compactTitle = 20.sp
    val touchTarget = 44.dp

    /**
     * squircle 的视觉补偿：连续曲率在同样数值下看起来比普通圆角更「紧」，
     * Flutter 的 `squircleRadius()` 统一乘了 1.6；这里照抄，保证两边数值一致。
     * 真正的连续曲率路径见 [SquircleShape]。
     */
    const val SquircleScale = 1.6f

    val cardShapeRadius = (cardRadius.value * SquircleScale).dp

    /** 小标签（ToneBadge）在 Flutter 里是 20 的圆角、同样乘 1.6。 */
    val badgeShapeRadius = (20f * SquircleScale).dp
}

val LocalLColors = staticCompositionLocalOf { LColors.Light }

@Composable
fun LiantTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val colors = if (darkTheme) LColors.Dark else LColors.Light
    val scheme = if (darkTheme) {
        darkColorScheme(
            primary = colors.accent,
            background = colors.bg,
            surface = colors.elevated,
            onSurface = colors.text,
            error = colors.danger,
        )
    } else {
        lightColorScheme(
            primary = colors.accent,
            background = colors.bg,
            surface = colors.elevated,
            onSurface = colors.text,
            error = colors.danger,
        )
    }
    CompositionLocalProvider(LocalLColors provides colors) {
        MaterialTheme(
            colorScheme = scheme,
            typography = liantTypography(darkTheme),
            content = content,
        )
    }
}

/**
 * 字号与 Flutter `TextTheme` 对齐；深色下字重降一档（Flutter 里也是这么做的）。
 */
private fun liantTypography(dark: Boolean): Typography {
    fun weight(light: FontWeight): FontWeight = when {
        !dark -> light
        light == FontWeight.W700 -> FontWeight.W600
        light == FontWeight.W600 -> FontWeight.W500
        light == FontWeight.W500 -> FontWeight.W400
        else -> light
    }

    return Typography(
        displaySmall = TextStyle(
            fontSize = LTokens.largeTitle,
            fontWeight = weight(FontWeight.W700),
            letterSpacing = (-0.8).sp,
            // Flutter 的大标题 height: 1.15 → 34 * 1.15 ≈ 39。
            lineHeight = 39.sp,
        ),
        headlineSmall = TextStyle(
            fontSize = 22.sp,
            fontWeight = weight(FontWeight.W700),
            letterSpacing = (-0.3).sp,
            lineHeight = 26.sp,
        ),
        titleLarge = TextStyle(
            fontSize = 20.sp,
            fontWeight = weight(FontWeight.W700),
            lineHeight = 25.sp,
        ),
        titleMedium = TextStyle(
            fontSize = 17.sp,
            fontWeight = weight(FontWeight.W600),
            lineHeight = 22.sp,
        ),
        titleSmall = TextStyle(
            fontSize = 15.sp,
            fontWeight = weight(FontWeight.W600),
            lineHeight = 20.sp,
        ),
        bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 22.sp),
        bodyMedium = TextStyle(fontSize = 15.sp, lineHeight = 21.sp),
        bodySmall = TextStyle(fontSize = 13.sp, lineHeight = 19.sp),
        labelLarge = TextStyle(fontSize = 15.sp, fontWeight = weight(FontWeight.W600)),
        labelMedium = TextStyle(fontSize = 13.sp, fontWeight = weight(FontWeight.W500)),
        labelSmall = TextStyle(fontSize = 12.sp, fontWeight = weight(FontWeight.W500)),
    )
}

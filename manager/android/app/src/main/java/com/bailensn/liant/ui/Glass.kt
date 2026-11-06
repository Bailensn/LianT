package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.drawscope.DrawScope
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin
import top.yukonga.miuix.kmp.blur.BlurDefaults
import top.yukonga.miuix.kmp.blur.LayerBackdrop
import top.yukonga.miuix.kmp.blur.highlight.Highlight
import top.yukonga.miuix.kmp.blur.rememberLayerBackdrop
import top.yukonga.miuix.kmp.blur.textureBlur
import top.yukonga.miuix.kmp.theme.MiuixTheme

/**
 * 录一层页面内容，给顶栏、底栏、悬浮按钮当模糊源。
 * minSdk 33，miuix-blur 可以无条件使用，不需要能力门控。
 */
@Composable
internal fun rememberGlassBackdrop(): LayerBackdrop {
    val background = MiuixTheme.colorScheme.background
    return rememberLayerBackdrop {
        drawRect(background)
        drawContent()
    }
}

/** 液态玻璃底板：底栏、浮动按钮共用。 */
@Composable
internal fun GlassBar(
    backdrop: LayerBackdrop,
    modifier: Modifier = Modifier,
    barShape: Shape = RectangleShape,
    content: @Composable () -> Unit,
) {
    Box(modifier) {
        Box(
            modifier = Modifier
                .matchParentSize()
                .textureBlur(
                    backdrop = backdrop,
                    shape = barShape,
                    blurRadius = BlurDefaults.BlurRadius,
                    highlight = Highlight.Default,
                ),
        )
        content()
    }
}

/**
 * 顶栏用的渐隐遮罩。横向滚动时整块模糊会掉帧，顶栏只是给标题垫底，
 * 一条渐变就够，不必上 textureBlur。
 */
internal data class EdgeScrim(
    val angle: Float = 90f,
    val fadeStart: Float = 0f,
    val fadeEnd: Float = 1f,
    val startAlpha: Float = 0.55f,
    val endAlpha: Float = 0f,
) {
    companion object {
        val Top: EdgeScrim = EdgeScrim()
        val Bottom: EdgeScrim = EdgeScrim(angle = 270f)
    }
}

@Composable
internal fun ScrimBar(
    modifier: Modifier = Modifier,
    scrim: EdgeScrim = EdgeScrim.Top,
    content: @Composable () -> Unit,
) {
    val surface = MiuixTheme.colorScheme.surface
    Box(modifier) {
        Box(
            modifier = Modifier
                .matchParentSize()
                .drawBehind {
                    drawRect(
                        axisBrush(
                            scrim.angle,
                            surface.copy(alpha = scrim.startAlpha),
                            surface.copy(alpha = scrim.endAlpha),
                            scrim.fadeStart,
                            scrim.fadeEnd,
                        ),
                    )
                },
        )
        content()
    }
}

/** 沿 [angle] 方向的线性渐变；90 度时就是竖直方向 0 → 1。 */
private fun DrawScope.axisBrush(
    angle: Float,
    startColor: Color,
    endColor: Color,
    fadeStart: Float = 0f,
    fadeEnd: Float = 1f,
): Brush {
    val radians = Math.toRadians(angle.toDouble())
    val dx = cos(radians).toFloat()
    val dy = sin(radians).toFloat()
    val center = Offset(size.width / 2f, size.height / 2f)
    val half = (abs(size.width * dx) + abs(size.height * dy)) / 2f
    val from = Offset(center.x - dx * half, center.y - dy * half)
    val to = Offset(center.x + dx * half, center.y + dy * half)
    val solidEnd = fadeEnd.coerceIn(fadeStart + 0.01f, 1f)
    return Brush.linearGradient(
        colorStops = arrayOf(
            0f to startColor,
            fadeStart.coerceIn(0f, solidEnd) to startColor,
            solidEnd to endColor,
            1f to endColor,
        ),
        start = from,
        end = to,
    )
}

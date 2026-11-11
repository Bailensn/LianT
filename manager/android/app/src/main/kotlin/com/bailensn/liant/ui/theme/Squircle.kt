package com.bailensn.liant.ui.theme

import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Outline
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.LayoutDirection
import kotlin.math.min

/**
 * 连续曲率圆角（squircle）。
 *
 * 与 Flutter 的 `ContinuousRectangleBorder` 用同一套路径：从角的两条直边端点出发，
 * 用三次贝塞尔把直边平滑接进角里，控制点落在矩形角点上。这样曲率是渐变的，
 * 没有普通圆角「直边突然变成圆弧」的折点——也就是 Flutter 版卡片看起来更「紧」
 * 的原因。逐段对应 `continuous_rectangle_border.dart` 的 `_getPath`。
 *
 * 半径按 Flutter 的 `squircleRadius()` 规则先乘 1.6（见 [LTokens.SquircleScale]），
 * 传进来的是已经放大过的值。
 */
class SquircleShape(private val radius: Dp) : Shape {
    override fun createOutline(
        size: Size,
        layoutDirection: LayoutDirection,
        density: Density,
    ): Outline {
        val width = size.width
        val height = size.height
        val limit = min(width, height) / 2f
        val r = min(with(density) { radius.toPx() }, limit).coerceAtLeast(0f)
        val path = Path().apply {
            moveTo(0f, r)
            cubicTo(0f, 0f, 0f, 0f, r, 0f)
            lineTo(width - r, 0f)
            cubicTo(width, 0f, width, 0f, width, r)
            lineTo(width, height - r)
            cubicTo(width, height, width, height, width - r, height)
            lineTo(r, height)
            cubicTo(0f, height, 0f, height, 0f, height - r)
            close()
        }
        return Outline.Generic(path)
    }
}

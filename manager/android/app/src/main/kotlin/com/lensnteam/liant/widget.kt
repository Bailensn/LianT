package com.lensnteam.liant

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeContentPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameMillis
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.kyant.backdrop.Backdrop
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import com.kyant.backdrop.drawBackdrop
import com.kyant.backdrop.effects.blur
import com.kyant.backdrop.effects.lens
import com.kyant.backdrop.effects.vibrancy
import kotlin.math.PI
import kotlin.math.sin
import kotlin.random.Random

/**
 * 统一管理 backdrop 的生命周期：
 * 1. 在这里创建 backdrop
 * 2. 自动把 [background] 打上 .layerBackdrop(backdrop)，录入内容
 * 3. 把 backdrop 传给 [content]，玻璃控件直接用，不用自己声明
 *
 * 调用方只需要"叠放"背景和玻璃内容，不用关心 backdrop 怎么建、怎么传。
 */
@Composable
fun LiquidGlassContainer(
    modifier: Modifier = Modifier,
    backdropFallbackColor: Color = Color.Black,
    background: @Composable BoxScope.() -> Unit,
    content: @Composable BoxScope.(backdrop: Backdrop) -> Unit
) {
    Box(modifier = modifier) {
        // backdrop 在容器内部统一创建。fallbackColor 是兜底色：
        // 背景内容里透明的部分会用它填充，按背景整体明暗选深/浅色。
        val backdrop = rememberLayerBackdrop {
            drawRect(backdropFallbackColor)
            drawContent()
        }

        // 背景统一打上 layerBackdrop，这样传进来的任何背景
        // （图片、渐变、甚至另一层可交互控件）都会被自动录入 backdrop
        Box(
            modifier = Modifier
            .matchParentSize()
            .layerBackdrop(backdrop)
        ) {
            background()
        }

        // 玻璃内容直接拿到已经建好的 backdrop 来用
        content(backdrop)
    }
}

/**
 * 玻璃卡片本体：拿到外部传入的 backdrop，采样并应用模糊、折射、鲜艳度效果。
 */
@Composable
fun GlassSheet(
    backdrop: Backdrop,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(42.dp),
    blurRadius: Dp = 3.dp,
    refractionHeight: Dp = 30.dp,
    refractionAmount: Dp = 40.dp,
    surfaceColor: Color = Color.Black.copy(alpha = 0.05f),
    content: @Composable ColumnScope.() -> Unit
) {
    Column(
        modifier = modifier
        .navigationBarsPadding()
        .fillMaxWidth()
        .wrapContentHeight()
        .drawBackdrop(
            backdrop = backdrop,
            shape = { shape },
            effects = {
                vibrancy()
                blur(blurRadius.toPx())
                lens(
                    refractionHeight = refractionHeight.toPx(),
                    refractionAmount = refractionAmount.toPx(),
                    depthEffect = true,
                    chromaticAberration = true
                )
            },
            onDrawSurface = {
                drawRect(surfaceColor)
            }
        ),
        content = content
    )
}

/**
 * 浮空的小玻璃板，当图标容器用：跟主玻璃板共用同一个 backdrop，
 * 效果强度按小尺寸调轻一些，中间放一张矢量图标（SVG 转出来的 VectorDrawable）。
 */
@Composable
fun FloatingGlassIcon(
    backdrop: Backdrop,
    modifier: Modifier = Modifier,
    size: Dp = 76.dp,
    shape: Shape = RoundedCornerShape(24.dp),
    iconRes: Int = R.drawable.ic_icon,
    surfaceColor: Color = Color.White.copy(alpha = 0.10f)
) {
    Box(
        modifier = modifier
        .size(size)
        .drawBackdrop(
            backdrop = backdrop,
            shape = { shape },
            effects = {
                vibrancy()
                blur(2.dp.toPx())
                lens(
                    refractionHeight = 10.dp.toPx(),
                    refractionAmount = 16.dp.toPx(),
                    depthEffect = true,
                    chromaticAberration = false
                )
            },
            onDrawSurface = {
                drawRect(surfaceColor)
            }
        ),
        contentAlignment = Alignment.Center
    ) {
        Image(
            painter = painterResource(id = iconRes),
            contentDescription = "Icon",
            modifier = Modifier
            .size(size)
            .padding(1.dp)
        )
    }
}

/**
 * 类 MiuiX 风格的动态渐变背景：3～5 团蓝色柔光光斑，中心贴着屏幕边缘外侧、
 * 在屏幕外 16～120dp 的范围内缓慢漂移（严格不进入屏幕，只有辐射范围会进屏幕），
 * 光斑半径为屏幕宽度的 2/3。
 *
 * 配色是常规逻辑：白天浅色底、夜间深色底，都避开纯白/纯黑，用灰色系过渡。
 *
 * 动画用持续递增的帧时钟时间驱动 sin()，而不是会周期性从头重放的
 * InfiniteTransition + tween：后者每轮循环结束会瞬间从终值跳回初始值，
 * 只要角速度和循环边界没对齐（比如多套一个非整数倍频率），就会看到画面"闪一下"。
 * 用持续递增、永不回绕的时间做输入，sin() 本身是严格连续的，从根上不会跳变。
 */
private enum class Edge { TOP, BOTTOM, LEFT, RIGHT }

private class BlobConfig(
    val edge: Edge,
    val breathePeriodMillis: Int,
    val driftPeriodMillis: Int,
    val phase: Float,
    val alongBase: Float
)

@Composable
fun AnimatedGradientBackground(modifier: Modifier = Modifier) {
    val darkTheme = isSystemInDarkTheme()

    val themeBlue = if (darkTheme) Color(0xFF5C93FF) else Color(0xFF3D74E8)
    val base = if (darkTheme) Color(0xFF1B1C20) else Color(0xFFEDEEF2)

    // 光斑配置只在第一次进入组合时随机一次，之后保持稳定，
    // 只靠时间驱动位置变化，不会因为重组重新洗牌导致跳变。
    val blobs = remember {
        List(Random.nextInt(3, 6)) {
            BlobConfig(
                edge = Edge.values().random(),
                breathePeriodMillis = Random.nextInt(12000, 22000),
                driftPeriodMillis = Random.nextInt(9000, 17000),
                phase = Random.nextFloat() * (2f * PI.toFloat()),
                alongBase = Random.nextFloat()
            )
        }
    }

    // 持续递增、永不重置的已运行时间（毫秒），作为所有 sin() 的输入
    var elapsedMillis by remember { mutableFloatStateOf(0f) }
    LaunchedEffect(Unit) {
        val startMillis = withFrameMillis { it }
        while (true) {
            withFrameMillis { frameMillis ->
                elapsedMillis = (frameMillis - startMillis).toFloat()
            }
        }
    }

    Canvas(modifier = modifier.fillMaxSize()) {
        drawRect(base)

        val w = size.width
        val h = size.height
        val blobRadius = w * (2f / 3f)
        val fullTurn = (2 * PI).toFloat()

        // 中心离屏幕边缘的距离范围：下限设成严格大于 0 的一个正值，
        // 保证中心点永远待在屏幕外，只有半径覆盖的辐射范围会伸进屏幕。
        val minOutsidePx = 16.dp.toPx()
        val maxOutsidePx = 120.dp.toPx()

        blobs.forEach { config ->
            val breathe = sin(elapsedMillis / config.breathePeriodMillis * fullTurn) * 0.5f + 0.5f
            val outside = minOutsidePx + breathe * (maxOutsidePx - minOutsidePx)

            val alongFraction = (
                config.alongBase +
                0.15f * sin(elapsedMillis / config.driftPeriodMillis * fullTurn + config.phase)
            ).coerceIn(0f, 1f)

            val center = when (config.edge) {
                Edge.TOP -> Offset(w * alongFraction, -outside)
                Edge.BOTTOM -> Offset(w * alongFraction, h + outside)
                Edge.LEFT -> Offset(-outside, h * alongFraction)
                Edge.RIGHT -> Offset(w + outside, h * alongFraction)
            }

            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(themeBlue.copy(alpha = 0.35f), themeBlue.copy(alpha = 0f)),
                    center = center,
                    radius = blobRadius
                ),
                radius = blobRadius,
                center = center
            )
        }
    }
}

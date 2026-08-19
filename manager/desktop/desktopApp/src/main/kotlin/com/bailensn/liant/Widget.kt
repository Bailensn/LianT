package com.bailensn.liant

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.*
import androidx.compose.ui.res.loadImageBitmap
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.*
import com.kyant.backdrop.*
import com.kyant.backdrop.backdrops.layerBackdrop
import com.kyant.backdrop.backdrops.rememberLayerBackdrop
import com.kyant.backdrop.effects.blur
import com.kyant.backdrop.effects.lens
import com.kyant.backdrop.effects.vibrancy
import kotlin.math.PI
import kotlin.math.sin
import kotlin.random.Random
import org.jetbrains.compose.resources.painterResource
import com.bailensn.liant.generated.resources.Res
import com.bailensn.liant.generated.resources.ic_icon

@Composable
fun LiquidGlassContainer(
    modifier: Modifier = Modifier,
    backdropFallbackColor: Color = Color.Black,
    background: @Composable BoxScope.() -> Unit,
    content: @Composable BoxScope.(Backdrop) -> Unit
) {
    Box(
        modifier = modifier
    ) {
        val backdrop =
            rememberLayerBackdrop {
                drawRect(
                    backdropFallbackColor
                )
                drawContent()
            }
        Box(
            modifier =
                Modifier
                    .matchParentSize()
                    .layerBackdrop(backdrop)
        ) {
            background()
        }
        content(backdrop)
    }
}

@Composable
fun GlassSheet(
    backdrop: Backdrop,
    modifier: Modifier = Modifier,
    shape: Shape = RoundedCornerShape(42.dp),
    blurRadius: Dp = 4.dp,
    refractionHeight: Dp = 30.dp,
    refractionAmount: Dp = 50.dp,
    surfaceColor: Color =
        Color.White.copy(alpha = 0.08f),
    content: @Composable ColumnScope.() -> Unit
) {
    Column(
        modifier =
            modifier
                .fillMaxWidth()
                .drawBackdrop(
                    backdrop = backdrop,
                    shape = {
                        shape
                    },
                    effects = {
                        vibrancy()
                        blur(
                            blurRadius.toPx()
                        )
                        lens(
                            refractionHeight =
                                refractionHeight.toPx(),
                            refractionAmount =
                                refractionAmount.toPx(),
                            depthEffect = true,
                            chromaticAberration = true
                        )
                    },
                    onDrawSurface = {
                        drawRect(
                            surfaceColor
                        )
                    }
                ),
        content = content
    )
}

@Composable
fun FloatingGlassIcon(
    backdrop: Backdrop,
    modifier: Modifier = Modifier,
    size: Dp = 76.dp,
    shape: Shape =
        RoundedCornerShape(24.dp),
) {
    Box(
        modifier =
            modifier
                .size(size)
                .drawBackdrop(
                    backdrop = backdrop,
                    shape = {
                        shape
                    },
                    effects = {
                        vibrancy()
                        blur(
                            2.dp.toPx()
                        )
                        lens(
                            refractionHeight =
                                10.dp.toPx(),
                            refractionAmount =
                                16.dp.toPx(),
                            depthEffect = true,
                            chromaticAberration = false
                        )
                    },
                    onDrawSurface = {
                        drawRect(
                            Color.White.copy(
                                alpha = 0.1f
                            )
                        )
                    }
                ),
        contentAlignment =
            Alignment.Center
    ) {
        Image(
            painter = painterResource(Res.drawable.ic_icon),
            contentDescription = "Icon"
        )
    }
}

private enum class Edge {
    TOP,
    BOTTOM,
    LEFT,
    RIGHT
}

private class BlobConfig(
    val edge: Edge,
    val breathePeriodMillis: Int,
    val driftPeriodMillis: Int,
    val phase: Float,
    val alongBase: Float
)

@Composable
fun AnimatedGradientBackground(
    modifier: Modifier = Modifier
) {
    val dark =
        isSystemInDarkTheme()
    val themeBlue =
        if (dark)
            Color(0xFF5C93FF)
        else
            Color(0xFF3D74E8)
    val base =
        if (dark)
            Color(0xFF1B1C20)
        else
            Color(0xFFEDEEF2)
    val blobs =
        remember {
            List(
                Random.nextInt(3,6)
            ){
                BlobConfig(
                    edge =
                        Edge.entries.random(),
                    breathePeriodMillis =
                        Random.nextInt(
                            12000,
                            22000
                        ),
                    driftPeriodMillis =
                        Random.nextInt(
                            9000,
                            17000
                        ),
                    phase =
                        Random.nextFloat()
                            *
                            (2f * PI.toFloat()),
                    alongBase =
                        Random.nextFloat()
                )
            }
        }
    var elapsed by remember {
        mutableFloatStateOf(0f)
    }
    LaunchedEffect(Unit){
        val start =
            withFrameMillis {
                it
            }
        while(true){
            withFrameMillis {
                elapsed =
                    (
                        it - start
                    ).toFloat()
            }
        }
    }
    Canvas(
        modifier =
            modifier.fillMaxSize()
    ){
        drawRect(base)
        val radius =
            size.width * 2f / 3f
        blobs.forEach {
            val breathe =
                sin(
                    elapsed /
                    it.breathePeriodMillis *
                    (2 * PI).toFloat()
                ) * 0.5f + 0.5f
            val outside =
                16.dp.toPx()
                +
                breathe *
                (
                    120.dp.toPx()
                    -
                    16.dp.toPx()
                )
            val fraction =
                (
                    it.alongBase
                    +
                    0.15f *
                    sin(
                        elapsed /
                        it.driftPeriodMillis *
                        (2 * PI).toFloat()
                        +
                        it.phase
                    )
                ).coerceIn(
                    0f,
                    1f
                )
            val center =
                when(it.edge){
                    Edge.TOP ->
                        Offset(
                            size.width * fraction,
                            -outside
                        )
                    Edge.BOTTOM ->
                        Offset(
                            size.width * fraction,
                            size.height + outside
                        )
                    Edge.LEFT ->
                        Offset(
                            -outside,
                            size.height * fraction
                        )
                    Edge.RIGHT ->
                        Offset(
                            size.width + outside,
                            size.height * fraction
                        )
                }
            drawCircle(
                brush =
                    Brush.radialGradient(
                        colors =
                            listOf(
                                themeBlue.copy(
                                    alpha = 0.35f
                                ),
                                themeBlue.copy(
                                    alpha = 0f
                                )
                            ),
                        center = center,
                        radius = radius
                    ),
                radius = radius,
                center = center
            )
        }
    }
}
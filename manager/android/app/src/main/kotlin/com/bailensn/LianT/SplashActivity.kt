package com.bailensn.LianT

import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

class SplashActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val darkTheme = isSystemInDarkTheme()
            DisposableEffect(darkTheme) {
                enableEdgeToEdge(
                    statusBarStyle = SystemBarStyle.auto(
                        android.graphics.Color.TRANSPARENT,
                        android.graphics.Color.TRANSPARENT
                    ) { darkTheme },
                    navigationBarStyle = SystemBarStyle.auto(
                        android.graphics.Color.TRANSPARENT,
                        android.graphics.Color.TRANSPARENT
                    ) { darkTheme }
                )
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                    window.isNavigationBarContrastEnforced = false
                }
                onDispose {}
            }
            LianTTheme { TheLayout() }
        }
    }
}

@OptIn(ExperimentalMaterial3ExpressiveApi::class)
@Composable
private fun LianTTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val context = LocalContext.current
    val colorScheme = when {
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            if (darkTheme) dynamicDarkColorScheme(context)
            else dynamicLightColorScheme(context)
        }
        darkTheme -> darkColorScheme()
        else -> expressiveLightColorScheme()
    }
    MaterialExpressiveTheme(
        colorScheme = colorScheme,
        shapes = Shapes(
            large = RoundedCornerShape(32.dp),
            extraLarge = RoundedCornerShape(36.dp)
        ),
        content = content
    )
}

@Preview
@Composable
fun MyPreview() {
    LianTTheme { TheLayout() }
}

@Composable
fun TheLayout() {
    val textColor = if (isSystemInDarkTheme()) Color.White else Color.Black
    val iconSize = 76.dp

    Box(modifier = Modifier.fillMaxSize()) {
        LiquidGlassContainer(
            modifier = Modifier.fillMaxSize(),
            background = {
                AnimatedGradientBackground(modifier = Modifier.fillMaxSize())
            }
        ) { backdrop ->
            // 主布局：使用 Column 将【上面的文字/图标】与【底部的玻璃板】分开处理
            Column(
                modifier = Modifier
                .fillMaxSize()
                .padding(horizontal = 24.dp),
                verticalArrangement = Arrangement.SpaceBetween
            ) {
                // --- 1. 上半部分：在背景空地上的浮空图标和文字 ---
                Column(
                    modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 80.dp), // 距离顶部的距离，可自行调整
                    horizontalAlignment = Alignment.Start
                ) {
                    // 浮空小玻璃板图标
                    FloatingGlassIcon(
                        backdrop = backdrop,
                        size = iconSize,
                        iconRes = R.drawable.ic_icon
                    )

                    Spacer(modifier = Modifier.height(24.dp))

                    // 位于空地上的标题
                    Text(
                        text = "欢迎回来",
                        fontSize = 30.sp,
                        fontWeight = FontWeight.Bold,
                        color = textColor
                    )

                    Spacer(modifier = Modifier.height(8.dp))

                    // 位于空地上的副标题
                    Text(
                        text = "请选择要登录的 bot",
                        fontSize = 17.sp,
                        color = textColor.copy(alpha = 0.75f)
                    )
                }

                // --- 2. 下半部分：底部玻璃板 ---
                GlassSheet(
                    backdrop = backdrop,
                    shape = RoundedCornerShape(32.dp),
                    blurRadius = 4.dp,
                    refractionAmount = 50.dp,
                    modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 32.dp) // 距离底部的边距
                ) {
                    // 这里是玻璃板内部的真正内容（如 Bot 列表、按钮等）
                    Column(
                        modifier = Modifier
                        .fillMaxWidth()
                        .padding(12.dp)
                    ) {
                        Spacer(modifier = Modifier.height(600.dp)) // 预留给后续控件的高
                    }
                }
            }
        }
    }
}

package com.bailensn.liant.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import top.yukonga.miuix.kmp.blur.layerBackdrop
import com.bailensn.liant.R
import top.yukonga.miuix.kmp.basic.Scaffold
import top.yukonga.miuix.kmp.basic.SmallTopAppBar
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.theme.MiuixTheme

/**
 * 未连接时的占位页。真正的多 Bot 收件箱在 P1 接入协议后替换这里。
 * 先用上 Miuix 的 Scaffold + 顶栏，把外壳和玻璃效果跑通。
 */
@Composable
fun HomeScreen() {
    val backdrop = rememberGlassBackdrop()
    Scaffold(
        modifier = Modifier.fillMaxSize(),
        topBar = {
            ScrimBar {
                SmallTopAppBar(
                    title = stringResource(R.string.app_name),
                    color = Color.Transparent,
                )
            }
        },
    ) { innerPadding: PaddingValues ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(MiuixTheme.colorScheme.background)
                .layerBackdrop(backdrop)
                .padding(innerPadding)
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = stringResource(R.string.home_title),
                color = MiuixTheme.colorScheme.primary,
            )
            Text(
                text = stringResource(R.string.home_empty_title),
                color = MiuixTheme.colorScheme.onSurfaceSecondarySummary,
            )
            Text(
                text = stringResource(R.string.home_empty_hint),
                color = MiuixTheme.colorScheme.onSurfaceSecondarySummary,
                style = MiuixTheme.textStyles.footnote1,
            )
        }
    }
}

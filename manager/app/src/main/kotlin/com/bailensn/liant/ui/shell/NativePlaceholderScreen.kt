package com.bailensn.liant.ui.shell

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.components.LargeTitleHeader
import com.bailensn.liant.ui.theme.LocalLColors

/**
 * 还没搬的原生页面占位。
 *
 * 说清楚现状，不放假数据、不做假按钮；要看得用 Flutter 版（返回即可）。
 */
@Composable
fun NativePlaceholderScreen(title: String) {
    val colors = LocalLColors.current
    Column(
        Modifier
            .fillMaxSize()
            .background(colors.bg)
            .statusBarsPadding(),
    ) {
        LargeTitleHeader(title = title)
        Box(
            Modifier
                .fillMaxWidth()
                .weight(1f),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = "原生页面还在迁移，先用 Flutter 版",
                color = colors.text3,
                fontSize = 13.sp,
            )
        }
    }
}

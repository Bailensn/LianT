package com.bailensn.liant.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.shape.RoundedCornerShape
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.basic.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp

/** 媒体比例只允许 1:1、4:3、16:9（设计规范的三段式裁切）。 */
enum class MediaRatio(val value: Float) {
    Square(1f),
    FourThree(4f / 3f),
    SixteenNine(16f / 9f),
}

/**
 * 媒体占位/缩略图。
 *
 * 先用固定比例占位，等 media.fetch 接上后把图片填进同一个盒子里，
 * 布局不会因为加载完成而跳动。
 */
@Composable
fun MediaThumb(
    ratio: MediaRatio,
    modifier: Modifier = Modifier,
    label: String = "图片",
) {
    Box(
        modifier
            .aspectRatio(ratio.value)
            .clip(RoundedCornerShape(8.dp))
            .background(MiuixTheme.colorScheme.surfaceVariant),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            label,
            style = MiuixTheme.textStyles.footnote2,
            color = MiuixTheme.colorScheme.onSurfaceSecondary,
        )
    }
}

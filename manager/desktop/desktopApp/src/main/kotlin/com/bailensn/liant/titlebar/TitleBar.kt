package com.bailensn.liant.titlebar

import androidx.compose.runtime.Composable

/**
 * 旧的标题栏分发入口（历史遗留，暂无调用方）。
 *
 * 注意：真正的 Desktop 标题栏已经改为 WindowScope 扩展函数
 * `WindowScope.DesktopTitleBar(...)`，由 window/DesktopWindow 里直接调用，
 * 放到这里仅供编译兼容。新代码请直接用 `WindowScope.DesktopTitleBar`。
 */
@Composable
fun TitleBar() {
    // 什么都不做：桌面窗口的小白条已由 Window { } 内部的 DesktopTitleBar 渲染。
    // 这个函数保留只是为了防止老项目的 TitleBar() 调用处编译报错。
}
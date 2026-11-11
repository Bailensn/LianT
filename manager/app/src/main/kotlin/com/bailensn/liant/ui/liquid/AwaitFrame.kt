package com.bailensn.liant.ui.liquid

import androidx.compose.runtime.withFrameNanos

/** 对应 catalog 里各平台的 awaitFrame（Android 用 Compose 的帧时钟）。 */
internal suspend fun awaitFrame() {
    withFrameNanos { }
}

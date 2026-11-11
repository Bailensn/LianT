package com.bailensn.liant.core

import android.content.Context
import java.io.File

/**
 * 进程间共享文件的路径。
 *
 * UI 进程与 `:background` 进程的 filesDir 是同一个目录，路径必须只有一处定义，
 * 否则两侧读写会各写各的（Dart 侧通过 `liant/background_state` 通道拿同一份路径）。
 */
object CoreFiles {
    /** UI 进程写、后台进程读的状态文件；存在即代表「已配对」。 */
    fun stateFile(context: Context): File =
        File(context.filesDir, "liant_background_state.json")

    /** 后台进程写给 UI 的桥接信息（本地端口 + 令牌 + 心跳时间）。 */
    fun bridgeFile(context: Context): File =
        File(context.filesDir, "liant_bridge.json")

    /** 未捕获异常与核心错误落盘的位置（UI 与 :background 进程共用）。 */
    fun logFile(context: Context): File =
        File(context.filesDir, "liant_crash.log")
}

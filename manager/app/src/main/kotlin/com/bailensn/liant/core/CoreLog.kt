package com.bailensn.liant.core

import android.content.Context
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.Executors

/**
 * Kotlin 核心的落盘日志。
 *
 * 与 Dart 的 `liant_crash.log` 共用同一个文件、同一个 16 KB 上限，设置页
 * 「最近的错误」能直接看到。成功路径只写 logcat，不往这里塞正常信息，
 * 否则「最近的错误」会变成一条无意义的状态行。
 */
object CoreLog {
    private const val MAX_BYTES = 16 * 1024

    private val writer = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "liant-core-log").apply { isDaemon = true }
    }

    fun record(context: Context, message: String) {
        val app = context.applicationContext
        val line = "${timestamp()} [Kotlin] $message\n\n"
        writer.execute {
            try {
                val file = CoreFiles.logFile(app)
                if (file.isFile && file.length() > MAX_BYTES) {
                    file.writeText(line)
                } else {
                    file.appendText(line)
                }
            } catch (_: Exception) {
                // 记日志本身不能把后台进程搞挂。
            }
        }
    }

    private fun timestamp(): String =
        SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS", Locale.US).format(Date())
}

package com.bailensn.liant.ui.settings

import android.content.Context
import android.content.pm.PackageManager
import com.bailensn.liant.core.CoreFiles
import com.bailensn.liant.core.bridge.BridgeInfo
import com.bailensn.liant.core.state.StateStore
import java.io.IOException

/**
 * 设置页（原生试水版）的展示状态。
 *
 * 数据全部来自本机文件：共享状态文件里的身份、桥接文件里的心跳与连接状态、
 * 崩溃日志里的最后一条错误。**不连桥接**，所以不会抢走 Flutter 那条连接。
 */
data class SettingsUiState(
    val loading: Boolean = true,
    val paired: Boolean = false,
    val serviceName: String = "",
    val address: String = "",
    val deviceId: String = "",
    val fingerprint: String = "",
    /** 与 Dart 的 ConnState.name 一致：idle / connecting / online / reconnecting / error。 */
    val connection: String = "idle",
    val detail: String = "",
    val backgroundAgeMs: Long? = null,
    val lastError: String? = null,
    val versionName: String = "",
) {
    /** 桥接心跳在 90 秒内算「后台还在」。 */
    val backgroundAlive: Boolean
        get() = backgroundAgeMs != null && backgroundAgeMs >= 0 && backgroundAgeMs <= 90_000

    val online: Boolean get() = connection == "online"
}

object SettingsDataLoader {
    fun load(context: Context): SettingsUiState {
        val state = StateStore.read(context)
        val bridge = BridgeInfo.read(context)
        val age = bridge?.at?.takeIf { it > 0 }?.let { System.currentTimeMillis() - it }
        return SettingsUiState(
            loading = false,
            paired = state != null,
            serviceName = state?.identity?.serviceName.orEmpty(),
            address = state?.identity?.let { "${it.host}:${it.port}" }.orEmpty(),
            deviceId = state?.identity?.deviceId.orEmpty(),
            fingerprint = state?.identity?.fingerprint.orEmpty(),
            connection = bridge?.state?.takeIf { it.isNotEmpty() } ?: "idle",
            detail = bridge?.detail.orEmpty(),
            backgroundAgeMs = age,
            lastError = readLastError(context),
            versionName = versionName(context),
        )
    }

    /** 只取最后一段的第一行，和设置页「最近的错误」一致。 */
    private fun readLastError(context: Context): String? {
        val file = CoreFiles.logFile(context)
        if (!file.isFile) return null
        val text = try {
            file.readText().trim()
        } catch (_: IOException) {
            return null
        }
        if (text.isEmpty()) return null
        return text.split("\n\n").last().trim().lineSequence().firstOrNull()
    }

    private fun versionName(context: Context): String = try {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName.orEmpty()
    } catch (_: PackageManager.NameNotFoundException) {
        ""
    }
}

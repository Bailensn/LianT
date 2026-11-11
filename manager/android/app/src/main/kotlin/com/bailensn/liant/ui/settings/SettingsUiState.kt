package com.bailensn.liant.ui.settings

import android.content.Context
import android.content.pm.PackageManager
import com.bailensn.liant.core.CoreFiles
import com.bailensn.liant.core.bridge.BridgeInfo
import com.bailensn.liant.core.state.StateStore
import org.json.JSONObject
import java.io.IOException
import java.util.Calendar
import java.util.Locale

/** 已配对设备的一行（吊销按钮由界面决定要不要画）。 */
data class DeviceRow(
    val deviceId: String,
    val name: String,
    val subtitle: String,
    val self: Boolean,
)

/**
 * 设置页（原生）的展示状态。
 *
 * 本机文件给身份/心跳/错误，桥接的 `state.snapshot` 给设备、机器人与会话能力。
 * 不连桥接时退化成 Flutter 的「连接后才能读取」文案。
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
    val language: String = "简体中文",
    val devices: List<DeviceRow> = emptyList(),
    val devicesLoaded: Boolean = false,
    /** `state.snapshot` 是否成功拿到过；没拿到就别显示成「列表为空」。 */
    val snapshotLoaded: Boolean = false,
    val botCount: Int = 0,
    val botsRunning: Int = 0,
    val caps: List<String> = emptyList(),
) {
    /** 桥接心跳在 90 秒内算「后台还在」。 */
    val backgroundAlive: Boolean
        get() = backgroundAgeMs != null && backgroundAgeMs >= 0 && backgroundAgeMs <= 90_000

    val online: Boolean get() = connection == "online"

    /** 把 `state.snapshot` 的结果并进来（设备/机器人/能力）。 */
    fun withSnapshot(snapshot: JSONObject): SettingsUiState {
        val devicesJson = snapshot.optJSONArray("devices")
        val devices = buildList {
            if (devicesJson != null) {
                for (index in 0 until devicesJson.length()) {
                    val raw = devicesJson.optJSONObject(index) ?: continue
                    val id = raw.optString("device_id")
                    val name = raw.optString("name")
                    add(
                        DeviceRow(
                            deviceId = id,
                            name = name.ifEmpty { "未命名设备" },
                            subtitle = "最近活动 ${SettingsDataLoader.formatDateTime(raw.optLong("last_seen_at", 0L))} · $id",
                            self = id == deviceId,
                        ),
                    )
                }
            }
        }
        val botsJson = snapshot.optJSONArray("bots")
        var bots = 0
        var running = 0
        if (botsJson != null) {
            for (index in 0 until botsJson.length()) {
                val raw = botsJson.optJSONObject(index) ?: continue
                bots++
                if (raw.optString("status") == "running") running++
            }
        }
        val capsJson = snapshot.optJSONArray("caps")
        val caps = buildList {
            if (capsJson != null) {
                for (index in 0 until capsJson.length()) {
                    val cap = capsJson.optString(index)
                    if (cap.isNotEmpty()) add(cap)
                }
            }
        }
        return copy(
            devices = devices,
            devicesLoaded = devicesJson != null,
            snapshotLoaded = true,
            botCount = bots,
            botsRunning = running,
            caps = caps,
        )
    }
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
            language = languageLabel(context),
        )
    }

    /** 与 Flutter `formatDateTime` 一致：`M月d日 HH:mm`，0 显示「从未」。 */
    fun formatDateTime(ts: Long): String {
        if (ts <= 0) return "从未"
        val calendar = Calendar.getInstance().apply { timeInMillis = ts }
        val month = calendar.get(Calendar.MONTH) + 1
        val day = calendar.get(Calendar.DAY_OF_MONTH)
        return "%d月%d日 %02d:%02d".format(
            Locale.US,
            month,
            day,
            calendar.get(Calendar.HOUR_OF_DAY),
            calendar.get(Calendar.MINUTE),
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

    /** 与 Flutter 的 `L10n.label` 同一套写法；`system` 按设备语言解析。 */
    private fun languageLabel(context: Context): String {
        val prefs = context.getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)
        val setting = prefs.getString("flutter.liant.language", "system") ?: "system"
        return when (setting) {
            "zh" -> "简体中文"
            "zh_Hant" -> "繁體中文"
            "en" -> "English"
            "ja" -> "日本語"
            "ko" -> "한국어"
            else -> {
                val locale = Locale.getDefault()
                when {
                    locale.language == "zh" &&
                        locale.country in setOf("TW", "HK", "MO") -> "繁體中文"
                    locale.language == "zh" -> "简体中文"
                    locale.language == "en" -> "English"
                    locale.language == "ja" -> "日本語"
                    locale.language == "ko" -> "한국어"
                    else -> "简体中文"
                }
            }
        }
    }
}

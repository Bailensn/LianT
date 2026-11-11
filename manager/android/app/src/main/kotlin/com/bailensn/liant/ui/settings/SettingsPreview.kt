package com.bailensn.liant.ui.settings

import androidx.compose.runtime.Composable
import androidx.compose.ui.tooling.preview.Preview
import com.bailensn.liant.ui.theme.LiantTheme

/**
 * 原生设置页的 `@Preview`。
 *
 * 没有电脑 / 没有 Android Studio 的话，这里只是给以后有 Studio 的环境用的；
 * 手机上验证请装 CI 出的 APK，从「设置 → 通用 → 界面试水」进入。
 */
private val sampleConnected = SettingsUiState(
    loading = false,
    paired = true,
    serviceName = "liant-node",
    address = "192.168.1.10:8443",
    deviceId = "d9f3c2a1-7b4e-4c1a-9e2f-1a2b3c4d5e6f",
    fingerprint = "AB12-CD34-EF56-GH78-JK90-LM12-NP34-QR56",
    connection = "online",
    detail = "",
    backgroundAgeMs = 12_000,
    lastError = null,
    versionName = "1.0.0",
    language = "简体中文",
    devices = listOf(
        DeviceRow(
            deviceId = "d9f3c2a1-7b4e-4c1a-9e2f-1a2b3c4d5e6f",
            name = "我的手机",
            subtitle = "最近活动 11月12日 02:40 · d9f3c2a1-7b4e-4c1a-9e2f-1a2b3c4d5e6f",
            self = true,
        ),
        DeviceRow(
            deviceId = "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
            name = "桌面端",
            subtitle = "最近活动 11月11日 21:08 · a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
            self = false,
        ),
    ),
    devicesLoaded = true,
    botCount = 2,
    botsRunning = 2,
    caps = listOf("msg.list", "bot.control", "media.recv", "rich_text", "multi_bot"),
)

private val sampleUnpaired = SettingsUiState(
    loading = false,
    paired = false,
    connection = "idle",
    backgroundAgeMs = null,
    versionName = "1.0.0",
)

@Preview(
    name = "设置 · 浅色",
    widthDp = 411,
    heightDp = 891,
    showBackground = true,
    backgroundColor = 0xFFF4F4F7,
)
@Composable
private fun SettingsLightPreview() {
    LiantTheme(darkTheme = false) {
        SettingsScreen(state = sampleConnected, onRefresh = {}, onRevoke = {})
    }
}

@Preview(
    name = "设置 · 深色",
    widthDp = 411,
    heightDp = 891,
    showBackground = true,
    backgroundColor = 0xFF0E0E10,
)
@Composable
private fun SettingsDarkPreview() {
    LiantTheme(darkTheme = true) {
        SettingsScreen(state = sampleConnected, onRefresh = {}, onRevoke = {})
    }
}

@Preview(
    name = "设置 · 未配对",
    widthDp = 411,
    heightDp = 891,
    showBackground = true,
    backgroundColor = 0xFFF4F4F7,
)
@Composable
private fun SettingsUnpairedPreview() {
    LiantTheme(darkTheme = false) {
        SettingsScreen(state = sampleUnpaired, onRefresh = {}, onRevoke = {})
    }
}

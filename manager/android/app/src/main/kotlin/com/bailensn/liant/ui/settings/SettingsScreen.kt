package com.bailensn.liant.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.bailensn.liant.ui.components.BadgeTone
import com.bailensn.liant.ui.components.GroupCard
import com.bailensn.liant.ui.components.Hairline
import com.bailensn.liant.ui.components.LargeTitleHeader
import com.bailensn.liant.ui.components.SettingsRow
import com.bailensn.liant.ui.components.ToneBadge
import com.bailensn.liant.ui.theme.LocalLColors
import kotlin.math.roundToInt

/**
 * 设置页的原生版（迁移试水）。
 *
 * 与 Flutter 版逐行对齐：服务卡片、本机卡片、关于卡片；数据来自本机文件，
 * 不建立 WebSocket / 桥接连接，所以打开它不会影响 Flutter 侧的那条连接。
 */
@Composable
fun SettingsScreen(state: SettingsUiState, onRefresh: () -> Unit) {
    val colors = LocalLColors.current
    Column(
        Modifier
            .fillMaxSize()
            .background(colors.bg)
            .statusBarsPadding(),
    ) {
        LargeTitleHeader(title = "设置")
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 40.dp),
        ) {
            item { ServiceCard(state, onRefresh) }
            item { MachineCard() }
            item { AboutCard(state) }
        }
    }
}

@Composable
private fun ServiceCard(state: SettingsUiState, onRefresh: () -> Unit) {
    GroupCard(title = "服务") {
        SettingsRow(
            title = state.serviceName.ifEmpty { "LianT Service" },
            subtitle = state.address.ifEmpty { "未连接" },
            trailing = {
                ToneBadge(
                    label = when {
                        state.online -> "在线"
                        state.connection == "reconnecting" || state.connection == "connecting" -> "重连中"
                        else -> "离线"
                    },
                    tone = when {
                        state.online -> BadgeTone.Ok
                        state.connection == "reconnecting" || state.connection == "connecting" -> BadgeTone.Warn
                        else -> BadgeTone.Muted
                    },
                )
            },
        )
        Hairline(indent = 68.dp)
        SettingsRow(title = "后台服务", subtitle = backgroundText(state))
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "最近的错误",
            subtitle = state.lastError ?: "无",
            onClick = onRefresh,
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "消息连接",
            subtitle = if (state.backgroundAlive) {
                "后台进程常驻 · 关掉界面也能收消息"
            } else {
                "后台服务未就绪 · 不直连，等它起来"
            },
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "服务指纹",
            subtitle = state.fingerprint.ifEmpty { "未知" },
            monospace = true,
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "本机设备标识",
            subtitle = state.deviceId.ifEmpty { "未配对" },
            monospace = true,
        )
        if (state.detail.isNotEmpty()) {
            Hairline(indent = 68.dp)
            SettingsRow(title = "最近状态", subtitle = state.detail)
        }
        Hairline(indent = 68.dp)
        SettingsRow(title = "刷新", subtitle = "重新读取身份与后台心跳", onClick = onRefresh)
    }
}

@Composable
private fun MachineCard() {
    GroupCard(title = "本机") {
        SettingsRow(
            title = "设备密钥",
            subtitle = "P-256 私钥在本机生成，仅存设备本地，不随消息上传。",
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "聊天记录",
            subtitle = "本机保存、不设上限；打开会话时再从服务端补拉最近 100 条",
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "协议",
            subtitle = "v1 · 每次连接用设备私钥签名，TLS 只做传输加密",
        )
    }
}

@Composable
private fun AboutCard(state: SettingsUiState) {
    GroupCard(title = "关于") {
        SettingsRow(title = "客户端", subtitle = "Jetpack Compose 试水 · 与 Flutter 并存")
        Hairline(indent = 68.dp)
        SettingsRow(title = "版本", subtitle = state.versionName.ifEmpty { "1.0.0" })
    }
}

/** 与 Flutter 版「后台服务」那一行同一套文案。 */
private fun backgroundText(state: SettingsUiState): String {
    if (state.loading) return "检查中…"
    val age = state.backgroundAgeMs ?: return "未运行"
    if (age < 0) return "运行中"
    if (age < 90_000) return "运行中 · 心跳 ${(age / 1000).roundToInt()} 秒前"
    return "可能已停止 · 上次心跳 ${age / 60_000} 分钟前"
}

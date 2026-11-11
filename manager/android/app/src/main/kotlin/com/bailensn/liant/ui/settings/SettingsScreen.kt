package com.bailensn.liant.ui.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.components.BadgeTone
import com.bailensn.liant.ui.components.GroupCard
import com.bailensn.liant.ui.components.Hairline
import com.bailensn.liant.ui.components.LargeTitleHeader
import com.bailensn.liant.ui.components.SettingsRow
import com.bailensn.liant.ui.components.ToneBadge
import com.bailensn.liant.ui.theme.LocalLColors
import kotlin.math.roundToInt

/**
 * 设置页的原生版，逐行对齐 Flutter 的 `pages/settings_page.dart`。
 *
 * 顺序、分组标题、脚注、行文案都照抄；数据来自本机文件 + 桥接的 `state.snapshot`。
 * 底部留 130dp 是给玻璃底栏的（外壳还没搬），先保持和 Flutter 一样的滚动高度。
 */
@Composable
fun SettingsScreen(
    state: SettingsUiState,
    onRefresh: () -> Unit,
    onRevoke: (String) -> Unit,
) {
    val colors = LocalLColors.current
    val listState = rememberLazyListState()
    val density = LocalDensity.current
    var pendingRevoke by remember { mutableStateOf<DeviceRow?>(null) }

    // 与 Flutter 的 LargeTitlePage 一致：前 46dp 滚动用于把标题从 34 折到 20。
    val collapse = remember {
        derivedStateOf {
            if (listState.firstVisibleItemIndex > 0) {
                1f
            } else {
                val range = with(density) { 46.dp.toPx() }
                (listState.firstVisibleItemScrollOffset / range).coerceIn(0f, 1f)
            }
        }
    }

    Column(
        Modifier
            .fillMaxSize()
            .background(colors.bg)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        LargeTitleHeader(
            title = "设置",
            collapse = collapse.value,
            trailing = {
                IconButton(
                    onClick = onRefresh,
                    modifier = Modifier.size(44.dp),
                ) {
                    Icon(
                        imageVector = Icons.Filled.Refresh,
                        contentDescription = "刷新",
                        tint = colors.text2,
                        modifier = Modifier.size(20.dp),
                    )
                }
            },
        )
        LazyColumn(
            state = listState,
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, top = 0.dp, bottom = 130.dp),
        ) {
            item { ServiceCard(state, onRefresh) }
            item { DevicesCard(state, onRequestRevoke = { pendingRevoke = it }) }
            item { BotsCard(state) }
            item { SecurityCard() }
            item { GeneralCard(state) }
            item { UpdateCard() }
            item { AboutCard() }
            item { Spacer(Modifier.height(8.dp)) }
        }
    }

    val device = pendingRevoke
    if (device != null) {
        AlertDialog(
            onDismissRequest = { pendingRevoke = null },
            title = { Text("吊销「${device.name}」？") },
            text = { Text("吊销后这台设备需要重新配对才能连接。") },
            confirmButton = {
                TextButton(
                    onClick = {
                        pendingRevoke = null
                        onRevoke(device.deviceId)
                    },
                ) {
                    Text("吊销", color = colors.danger)
                }
            },
            dismissButton = {
                TextButton(onClick = { pendingRevoke = null }) {
                    Text("取消")
                }
            },
        )
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
        if (!state.online) {
            Hairline(indent = 68.dp)
            SettingsRow(title = "重新连接", subtitle = "断开时点这里立刻重试", onClick = onRefresh)
        }
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
    }
}

@Composable
private fun DevicesCard(state: SettingsUiState, onRequestRevoke: (DeviceRow) -> Unit) {
    val colors = LocalLColors.current
    GroupCard(
        title = "已配对设备",
        footer = "吊销后该设备立刻失去连接权限，需要在它上面重新配对。",
    ) {
        if (state.devices.isEmpty()) {
            SettingsRow(
                title = "暂无设备",
                subtitle = if (state.snapshotLoaded) "列表为空" else "连接后才能读取",
            )
        } else {
            state.devices.forEachIndexed { index, device ->
                if (index > 0) Hairline(indent = 68.dp)
                SettingsRow(
                    title = device.name,
                    subtitle = device.subtitle,
                    trailing = {
                        if (device.self) {
                            ToneBadge(label = "本机", tone = BadgeTone.Ok)
                        } else {
                            TextButton(
                                onClick = { onRequestRevoke(device) },
                                modifier = Modifier.height(36.dp),
                                contentPadding = PaddingValues(horizontal = 10.dp),
                                colors = ButtonDefaults.textButtonColors(contentColor = colors.danger),
                            ) {
                                Text("吊销", fontSize = 13.sp)
                            }
                        }
                    },
                )
            }
        }
    }
}

@Composable
private fun BotsCard(state: SettingsUiState) {
    GroupCard(
        title = "机器人",
        footer = "多个机器人可同时在线，事件按 bot_id 区分；要静音就在对应会话里设免打扰。",
    ) {
        SettingsRow(
            title = "已配置",
            subtitle = if (state.snapshotLoaded) {
                "${state.botCount} 个，运行中 ${state.botsRunning} 个"
            } else {
                "连接后才能读取"
            },
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "服务端能力",
            subtitle = if (state.caps.isEmpty()) "连接后才能读取" else state.caps.joinToString(" · "),
        )
    }
}

@Composable
private fun SecurityCard() {
    GroupCard(
        title = "安全",
        footer = "私钥从不离开本机；服务端只保存公钥，撤销设备即删除公钥。",
    ) {
        SettingsRow(title = "安全日志", subtitle = "最近 20 条登录、配对、吊销记录")
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "设备密钥",
            subtitle = "P-256 私钥在本机生成，仅存设备本地，不随消息上传。",
        )
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "解除配对",
            subtitle = "清除本机身份，需要重新配对才能连接",
            danger = true,
        )
    }
}

@Composable
private fun GeneralCard(state: SettingsUiState) {
    GroupCard(title = "通用") {
        SettingsRow(title = "语言", subtitle = state.language)
    }
}

@Composable
private fun UpdateCard() {
    GroupCard(title = "更新") {
        SettingsRow(title = "检查更新", subtitle = "Manager 与 Service")
    }
}

@Composable
private fun AboutCard() {
    GroupCard(title = "关于") {
        SettingsRow(title = "协议", subtitle = "v1 · 每次连接用设备私钥签名，TLS 只做传输加密")
        Hairline(indent = 68.dp)
        SettingsRow(
            title = "聊天记录",
            subtitle = "本机保存、不设上限；打开会话时再从服务端补拉最近 100 条",
        )
        Hairline(indent = 68.dp)
        SettingsRow(title = "客户端", subtitle = "Flutter · 与网页端同协议")
    }
}

/** 与 Flutter 版「后台服务」那一行同一套文案。 */
private fun backgroundText(state: SettingsUiState): String {
    if (state.loading) return "检查中…"
    val age = state.backgroundAgeMs ?: return "未运行"
    if (age < 0) return "运行中"
    if (age < 90_000) return "运行中 · 心跳 ${(age / 1000.0).roundToInt()} 秒前"
    return "可能已停止 · 上次心跳 ${age / 60_000} 分钟前"
}

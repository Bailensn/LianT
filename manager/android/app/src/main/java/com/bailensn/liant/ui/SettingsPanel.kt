package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.SavedPairing
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.Card
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.theme.MiuixTheme

/** 设置：Miuix 分组卡片 + 56dp 行，参考库自带 uitest 的设置页。 */
@Composable
fun SettingsPanel(
    saved: SavedPairing?,
    appVersion: String,
    notificationsEnabled: Boolean,
    onOpenNotificationSettings: () -> Unit,
    onForget: () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Column(Modifier.padding(start = 4.dp, top = 20.dp, bottom = 4.dp)) {
            Text("设置", style = MiuixTheme.textStyles.title1)
            Text(
                "LianT Manager $appVersion",
                style = MiuixTheme.textStyles.footnote2,
                color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
            )
        }

        Section("配对") {
            if (saved == null) {
                InfoRow("状态", "尚未配对")
            } else {
                InfoRow("服务地址", "${saved.host}:${saved.port}")
                InfoRow("设备 ID", saved.deviceId)
                InfoRow("节点指纹", saved.fingerprint, small = true)
            }
        }

        Section("通知") {
            Row(
                Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    if (notificationsEnabled) "已开启，只有活跃 Bot 会响" else "未开启，收不到系统通知",
                    style = MiuixTheme.textStyles.body1,
                    modifier = Modifier.weight(1f),
                )
                if (!notificationsEnabled) {
                    Button(onClick = onOpenNotificationSettings) { Text("开启") }
                }
            }
        }

        Section("关于") {
            InfoRow("版本", appVersion)
        }

        Section("危险操作") {
            Column(Modifier.fillMaxWidth().padding(16.dp)) {
                Text("解除配对后本地凭据会被清除", style = MiuixTheme.textStyles.footnote2)
                Button(
                    onClick = onForget,
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp).heightIn(min = 48.dp),
                ) {
                    Text("解除配对")
                }
            }
        }
    }
}

@Composable
private fun Section(title: String, content: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text(
            title,
            style = MiuixTheme.textStyles.footnote2,
            color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
            modifier = Modifier.padding(start = 12.dp),
        )
        Card(Modifier.fillMaxWidth()) {
            Column(Modifier.fillMaxWidth()) { content() }
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String, small: Boolean = false) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = MiuixTheme.textStyles.body1, modifier = Modifier.weight(1f))
        Text(
            value,
            style = if (small) MiuixTheme.textStyles.footnote2 else MiuixTheme.textStyles.body2,
            color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
        )
    }
}

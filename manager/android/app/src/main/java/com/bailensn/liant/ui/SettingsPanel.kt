package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.HorizontalDivider
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.basic.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.SavedPairing

/** 设置：显示配对信息，提供解除配对（危险操作，单独按钮）。 */
@Composable
fun SettingsPanel(
    saved: SavedPairing?,
    appVersion: String,
    notificationsEnabled: Boolean,
    onOpenNotificationSettings: () -> Unit,
    onForget: () -> Unit,
) {
    Column(Modifier.fillMaxSize()) {
        Text(
            "设置",
            style = MiuixTheme.textStyles.title1,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 16.dp),
        )
        HorizontalDivider(thickness = 0.5.dp)
        Column(
            Modifier.fillMaxWidth().padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            if (saved == null) {
                Text("未配对")
            } else {
                Text("服务地址", style = MiuixTheme.textStyles.footnote2)
                Text("${saved.host}:${saved.port}", style = MiuixTheme.textStyles.body1)
                Text("节点指纹", style = MiuixTheme.textStyles.footnote2)
                Text(saved.fingerprint, style = MiuixTheme.textStyles.footnote1)
                Text("设备 ID", style = MiuixTheme.textStyles.footnote2)
                Text(saved.deviceId, style = MiuixTheme.textStyles.footnote1)
            }
            Text("通知", style = MiuixTheme.textStyles.footnote2)
            Text(
                if (notificationsEnabled) "已开启，只有活跃 Bot 会响" else "未开启，收不到系统通知",
                style = MiuixTheme.textStyles.footnote1,
            )
            if (!notificationsEnabled) {
                Button(
                    onClick = onOpenNotificationSettings,
                    modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
                ) {
                    Text("去系统设置开启")
                }
            }
            Text("关于", style = MiuixTheme.textStyles.footnote2)
            Text("LianT Manager $appVersion", style = MiuixTheme.textStyles.footnote1)
            Button(
                onClick = onForget,
                modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
            ) {
                Text("解除配对")
            }
        }
    }
}

package com.bailensn.liant.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.SavedPairing

/** 设置：显示配对信息，提供解除配对（危险操作，单独按钮）。 */
@Composable
fun SettingsPanel(
    saved: SavedPairing?,
    onForget: () -> Unit,
) {
    Column(Modifier.fillMaxSize()) {
        Text(
            "设置",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.SemiBold,
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
                Text("服务地址", style = MaterialTheme.typography.labelMedium)
                Text("${saved.host}:${saved.port}", style = MaterialTheme.typography.bodyLarge)
                Text("节点指纹", style = MaterialTheme.typography.labelMedium)
                Text(saved.fingerprint, style = MaterialTheme.typography.bodySmall)
                Text("设备 ID", style = MaterialTheme.typography.labelMedium)
                Text(saved.deviceId, style = MaterialTheme.typography.bodySmall)
            }
            Button(
                onClick = onForget,
                modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
            ) {
                Text("解除配对")
            }
        }
    }
}

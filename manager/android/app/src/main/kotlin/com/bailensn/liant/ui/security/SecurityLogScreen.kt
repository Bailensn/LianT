package com.bailensn.liant.ui.security

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.outlined.Shield
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.components.GroupCard
import com.bailensn.liant.ui.components.Hairline
import com.bailensn.liant.ui.settings.SettingsDataLoader
import com.bailensn.liant.ui.shell.DetailScaffold
import com.bailensn.liant.ui.shell.EmptyState
import com.bailensn.liant.ui.settings.AuditRow
import com.bailensn.liant.ui.theme.LocalLColors

private val auditKindText = mapOf(
    "auth.ok" to "登录成功",
    "auth.fail" to "登录失败",
    "pair.ok" to "配对成功",
    "pair.denied" to "配对被拒",
    "device.revoke" to "设备吊销",
)

/** 安全日志页，文案与布局照 Flutter 的 `security_log_page.dart`。 */
@Composable
fun SecurityLogScreen(
    entries: List<AuditRow>,
    loading: Boolean,
    onBack: () -> Unit,
    onRefresh: () -> Unit,
) {
    val colors = LocalLColors.current
    DetailScaffold(
        title = "安全日志",
        onBack = onBack,
        trailing = {
            IconButton(onClick = onRefresh, modifier = Modifier.size(44.dp)) {
                Icon(
                    imageVector = Icons.Filled.Refresh,
                    contentDescription = "刷新",
                    tint = colors.text2,
                    modifier = Modifier.size(20.dp),
                )
            }
        },
    ) {
        if (loading && entries.isEmpty()) {
            Box(
                Modifier
                    .fillMaxWidth()
                    .padding(vertical = 48.dp),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp)
            }
        } else if (entries.isEmpty()) {
            EmptyState(
                icon = Icons.Outlined.Shield,
                title = "暂无记录",
                hint = "登录、配对与吊销都会写进这里",
            )
        } else {
            GroupCard(footer = "最近 20 条，成功与失败都会记录来源 IP。") {
                entries.forEachIndexed { index, entry ->
                    if (index > 0) Hairline(indent = 68.dp)
                    AuditRowItem(entry)
                }
            }
        }
    }
}

@Composable
private fun AuditRowItem(entry: AuditRow) {
    val colors = LocalLColors.current
    val danger = entry.kind == "auth.fail" || entry.kind == "pair.denied"
    Row(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 12.dp),
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                text = auditKindText[entry.kind] ?: entry.kind,
                color = if (danger) colors.danger else colors.text,
                fontSize = 16.sp,
                lineHeight = 22.sp,
            )
            val subtitle = listOf(entry.name, entry.reason, entry.ip)
                .filter { it.isNotEmpty() }
                .joinToString(" · ")
            if (subtitle.isNotEmpty()) {
                Spacer(Modifier.height(2.dp))
                Text(
                    text = subtitle,
                    color = colors.text3,
                    fontSize = 12.sp,
                    lineHeight = 18.sp,
                )
            }
        }
        Spacer(Modifier.width(12.dp))
        Text(
            text = SettingsDataLoader.formatDateTime(entry.ts),
            color = colors.text3,
            fontSize = 12.sp,
        )
    }
}

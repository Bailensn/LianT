package com.bailensn.liant.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.bailensn.liant.data.BotSummary

/** Bot 列表：空态给出「刷新」出口。 */
@Composable
fun BotListPanel(
    bots: List<BotSummary>,
    busy: Boolean,
    activeBotId: String?,
    onRefresh: () -> Unit,
    onSelectActive: (String) -> Unit,
    onStart: (String) -> Unit,
    onStop: (String) -> Unit,
) {
    Column(Modifier.fillMaxSize()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "Bot",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.weight(1f),
            )
            Button(onClick = onRefresh, enabled = !busy, modifier = Modifier.heightIn(min = 48.dp)) {
                Text(if (busy) "刷新中" else "刷新")
            }
        }
        HorizontalDivider(thickness = 0.5.dp)
        Text(
            "点一个 Bot 设为当前；只有当前 Bot 的新消息会发系统通知。",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
        )
        if (bots.isEmpty()) {
            Column(
                Modifier.fillMaxWidth().padding(32.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text("还没有 Bot", style = MaterialTheme.typography.titleMedium)
                Text(
                    "在 Service 上运行：LianT bot add --platform qqbot --cred app_id=… --cred app_secret=…",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Button(onClick = onRefresh, enabled = !busy, modifier = Modifier.heightIn(min = 48.dp)) {
                    Text("重新加载")
                }
            }
        }
        LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(16.dp)) {
            items(bots, key = { it.botId }) { bot ->
                val current = bot.botId == activeBotId
                Card(
                    Modifier.fillMaxWidth().clickable { onSelectActive(bot.botId) },
                ) {
                    Row(
                        Modifier.fillMaxWidth().padding(16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                bot.displayName.ifEmpty { bot.botId },
                                style = MaterialTheme.typography.titleMedium,
                            )
                            Text(
                                "${bot.platform} · ${bot.status}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            Row(
                                horizontalArrangement = Arrangement.spacedBy(8.dp),
                                modifier = Modifier.padding(top = 8.dp),
                            ) {
                                if (bot.status == "running" || bot.status == "starting") {
                                    Button(
                                        onClick = { onStop(bot.botId) },
                                        enabled = !busy,
                                        modifier = Modifier.heightIn(min = 48.dp),
                                    ) { Text("停止") }
                                } else {
                                    Button(
                                        onClick = { onStart(bot.botId) },
                                        enabled = !busy,
                                        modifier = Modifier.heightIn(min = 48.dp),
                                    ) { Text("启动") }
                                }
                            }
                        }
                        if (current) {
                            Text(
                                "当前",
                                style = MaterialTheme.typography.labelMedium,
                                color = MaterialTheme.colorScheme.primary,
                            )
                        }
                    }
                }
            }
        }
    }
}

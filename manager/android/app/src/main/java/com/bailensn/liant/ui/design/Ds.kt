package com.bailensn.liant.ui.design

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import top.yukonga.miuix.kmp.basic.Text
import top.yukonga.miuix.kmp.theme.MiuixTheme

/** 页面大标题 + 可选副标题，对应 uitest 的 LargeTitle。 */
@Composable
fun PageTitle(title: String, subtitle: String? = null) {
    Column(Modifier.fillMaxWidth().padding(start = 4.dp, top = 20.dp, bottom = 4.dp)) {
        Text(title, style = MiuixTheme.textStyles.title1)
        if (subtitle != null) {
            Text(
                subtitle,
                style = MiuixTheme.textStyles.footnote2,
                color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
            )
        }
    }
}

/** 分组标题（卡片上方的小灰字）。 */
@Composable
fun SectionHeader(title: String) {
    Text(
        title,
        style = MiuixTheme.textStyles.footnote2,
        color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
        modifier = Modifier.padding(start = 12.dp),
    )
}

/** 56dp 标准行：左侧主文本，右侧可放控件。 */
@Composable
fun RowScope.ListRow(
    title: String,
    summary: String? = null,
    trailing: @Composable () -> Unit = {},
) {
    Column(
        Modifier.weight(1f).heightIn(min = 56.dp).padding(vertical = 8.dp),
    ) {
        Text(title, style = MiuixTheme.textStyles.body1)
        if (summary != null) {
            Text(
                summary,
                style = MiuixTheme.textStyles.footnote2,
                color = MiuixTheme.colorScheme.onSurfaceVariantSummary,
            )
        }
    }
    trailing()
}

/** 行容器：左右 16dp、最小 56dp、垂直居中。 */
@Composable
fun RowContainer(content: @Composable RowScope.() -> Unit) {
    androidx.compose.foundation.layout.Row(
        Modifier.fillMaxWidth().heightIn(min = 56.dp).padding(horizontal = 16.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        content = content,
    )
}

/** 聊天时间戳 → HH:mm。 */
internal fun formatChatTime(ts: Long): String {
    val cal = java.util.Calendar.getInstance().apply { timeInMillis = ts }
    return String.format("%02d:%02d", cal.get(java.util.Calendar.HOUR_OF_DAY), cal.get(java.util.Calendar.MINUTE))
}

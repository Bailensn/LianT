package com.bailensn.liant.ui.language

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Check
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.bailensn.liant.ui.components.GroupCard
import com.bailensn.liant.ui.components.Hairline
import com.bailensn.liant.ui.shell.DetailScaffold
import com.bailensn.liant.ui.theme.LocalLColors

private val languageOptions = listOf("system", "zh", "zh_Hant", "en", "ja", "ko")

private fun languageLabel(code: String): String = when (code) {
    "zh" -> "简体中文"
    "zh_Hant" -> "繁體中文"
    "en" -> "English"
    "ja" -> "日本語"
    "ko" -> "한국어"
    else -> "跟随系统"
}

/** 语言选择页，照 Flutter 的 `language_page.dart`；选中即写回并返回。 */
@Composable
fun LanguageScreen(
    current: String,
    onBack: () -> Unit,
    onSelect: (String) -> Unit,
) {
    val colors = LocalLColors.current
    DetailScaffold(title = "语言", onBack = onBack) {
        GroupCard(
            title = "选择语言",
            footer = "原生页面目前只渲染中文；选择会写回本地，切回 Flutter 版即生效。",
        ) {
            languageOptions.forEachIndexed { index, code ->
                if (index > 0) Hairline(indent = 68.dp)
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clickable { onSelect(code) }
                        .padding(horizontal = 16.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = languageLabel(code),
                        color = colors.text,
                        fontSize = 16.sp,
                        lineHeight = 22.sp,
                        modifier = Modifier.weight(1f),
                    )
                    if (code == current) {
                        Icon(
                            imageVector = Icons.Rounded.Check,
                            contentDescription = null,
                            tint = colors.accent,
                            modifier = Modifier.size(20.dp),
                        )
                    }
                }
            }
        }
    }
}

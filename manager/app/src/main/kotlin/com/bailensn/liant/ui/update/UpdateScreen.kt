package com.bailensn.liant.ui.update

import android.content.Context
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.bailensn.liant.ui.components.GroupCard
import com.bailensn.liant.ui.components.Hairline
import com.bailensn.liant.ui.components.SettingsRow
import com.bailensn.liant.ui.shell.DetailScaffold
import com.bailensn.liant.ui.theme.LocalLColors
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONArray

private data class ReleaseInfo(
    val tag: String,
    val prerelease: Boolean,
    val url: String,
    val publishedAt: Long,
)

private data class UpdateResult(val title: String, val message: String, val url: String?)

private object UpdateChecker {
    private val client = OkHttpClient()

    fun fetch(): List<ReleaseInfo> {
        val request = Request.Builder()
            .url("https://api.github.com/repos/Bailensn/LianT/releases?per_page=30")
            .header("Accept", "application/vnd.github+json")
            .header("User-Agent", "LianT-Manager")
            .build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) {
                throw IllegalStateException("GitHub 返回 ${response.code}")
            }
            val raw = JSONArray(response.body?.string().orEmpty().ifEmpty { "[]" })
            val out = mutableListOf<ReleaseInfo>()
            for (index in 0 until raw.length()) {
                val item = raw.optJSONObject(index) ?: continue
                val tag = item.optString("tag_name")
                if (tag.isEmpty()) continue
                out += ReleaseInfo(
                    tag = tag,
                    prerelease = item.optBoolean("prerelease", false),
                    url = item.optString("html_url"),
                    publishedAt = item.optString("published_at").let(::parseTime),
                )
            }
            return out
        }
    }

    fun compare(a: String, b: String): Int {
        val pa = parts(a)
        val pb = parts(b)
        for (i in 0 until 3) {
            val diff = pa[i] - pb[i]
            if (diff != 0) return diff
        }
        return 0
    }

    private fun parts(version: String): List<Int> {
        val numbers = Regex("\\d+").findAll(version).map { it.value.toIntOrNull() ?: 0 }.take(3).toList()
        return numbers + List(3 - numbers.size) { 0 }
    }

    /** minSdk 24 没有 java.time，用 SimpleDateFormat 解析 ISO-8601（UTC）。 */
    private fun parseTime(text: String): Long = try {
        java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss'Z'", java.util.Locale.US)
            .apply { timeZone = java.util.TimeZone.getTimeZone("UTC") }
            .parse(text)
            ?.time
            ?: 0L
    } catch (_: Exception) {
        0L
    }
}

/** 检查更新页（原生）：对比 GitHub Releases，结果用对话框给出发布页链接。 */
@Composable
fun UpdateScreen(
    managerVersion: String,
    serviceVersion: String,
    onBack: () -> Unit,
    onOpenUrl: (String) -> Unit,
) {
    val context = LocalContext.current
    val colors = LocalLColors.current
    val scope = rememberCoroutineScope()
    var busy by remember { mutableStateOf("") }
    var result by remember { mutableStateOf<UpdateResult?>(null) }
    val beta = remember { readBeta(context) }

    fun check(manager: Boolean) {
        if (busy.isNotEmpty()) return
        busy = if (manager) "Manager" else "Service"
        scope.launch {
            val outcome = withContext(Dispatchers.IO) {
                runCatching { evaluate(manager, beta, managerVersion, serviceVersion) }
            }
            busy = ""
            result = outcome.getOrElse {
                UpdateResult("检查更新失败", it.message ?: "$it", null)
            }
        }
    }

    DetailScaffold(title = "检查更新", onBack = onBack) {
        GroupCard(title = "Manager") {
            SettingsRow(title = "当前版本", subtitle = managerVersion)
            Hairline(indent = 68.dp)
            SettingsRow(
                title = "检查更新",
                subtitle = if (busy == "Manager") "检查中…" else "从 GitHub Releases 读取",
                onClick = { check(true) },
            )
        }
        GroupCard(title = "Service") {
            SettingsRow(
                title = "当前版本",
                subtitle = serviceVersion.ifEmpty { "未知（未连接）" },
                monospace = true,
            )
            Hairline(indent = 68.dp)
            SettingsRow(
                title = "检查更新",
                subtitle = if (busy == "Service") "检查中…" else "对比最新稳定版",
                onClick = { check(false) },
            )
        }
    }

    result?.let { outcome ->
        AlertDialog(
            onDismissRequest = { result = null },
            title = { Text(outcome.title) },
            text = { Text(outcome.message) },
            confirmButton = {
                val url = outcome.url
                if (url != null) {
                    TextButton(
                        onClick = {
                            result = null
                            onOpenUrl(url)
                        },
                    ) {
                        Text("打开发布页", color = colors.accent)
                    }
                }
            },
            dismissButton = {
                TextButton(onClick = { result = null }) { Text("关闭") }
            },
        )
    }
}

private fun evaluate(
    manager: Boolean,
    beta: Boolean,
    managerVersion: String,
    serviceVersion: String,
): UpdateResult {
    val releases = UpdateChecker.fetch()
    if (manager) {
        val current = managerVersion.substringBefore('-')
        val latest = releases
            .filter { it.tag.startsWith("manager-v") }
            .filter { beta || !it.prerelease }
            .reduceOrNull { a, b ->
                val va = a.tag.removePrefix("manager-v")
                val vb = b.tag.removePrefix("manager-v")
                if (UpdateChecker.compare(vb, va) > 0) b else a
            }
        val latestVersion = latest?.tag?.removePrefix("manager-v")
        return if (latest != null && latestVersion != null && UpdateChecker.compare(latestVersion, current) > 0) {
            UpdateResult("发现新版本", "Manager $latestVersion（当前 $current）", latest.url)
        } else {
            UpdateResult("已是最新", "Manager 当前 $current", null)
        }
    }
    val latest = releases
        .filter { it.tag.startsWith("v") && !it.tag.startsWith("manager-v") }
        .filter { beta || !it.prerelease }
        .maxByOrNull { it.publishedAt }
    return when {
        latest == null -> UpdateResult("没有找到发布", "GitHub 上还没有 Service 发布", null)
        serviceVersion.isEmpty() -> UpdateResult(
            "服务端版本未知",
            "先连上服务端再检查；目前最新发布是 ${latest.tag}",
            latest.url,
        )
        latest.tag != serviceVersion -> UpdateResult(
            "发现新版本",
            "Service ${latest.tag}（当前 $serviceVersion）",
            latest.url,
        )
        else -> UpdateResult("已是最新", "Service 当前 $serviceVersion", null)
    }
}

private fun readBeta(context: Context): Boolean = context
    .getSharedPreferences("FlutterSharedPreferences", Context.MODE_PRIVATE)
    .getBoolean("flutter.liant.acceptBeta", false)

package com.bailensn.liant.core.bridge

import android.content.Context
import com.bailensn.liant.core.CoreFiles
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.io.IOException

/**
 * 后台进程写给 UI 的桥接信息：本次启动的端口 + 令牌 + 心跳时间 + 当前连接状态。
 *
 * - UI 进程读它接入（`port` / `token`）；
 * - `at` 每 30 秒刷新一次，UI 用新鲜度（90 秒）判断后台还在不在；
 * - `state` / `detail` 在每次连接状态变化时刷新，给不连桥接的界面（例如原生设置页）
 *   一个「当前在线状态」的只读快照。
 *
 * 先写临时文件再改名，避免读到写了一半的 JSON。
 */
data class BridgeSnapshot(
    val port: Int,
    val token: String,
    val at: Long,
    val state: String,
    val detail: String,
    /** 服务端在 auth.ok 里报的版本，原生界面「检查更新」用来对比。 */
    val serviceVersion: String,
)

object BridgeInfo {
    fun write(
        context: Context,
        port: Int,
        token: String,
        state: String = "",
        detail: String = "",
        serviceVersion: String = "",
    ): Boolean {
        val payload = JSONObject()
            .put("port", port)
            .put("token", token)
            .put("at", System.currentTimeMillis())
            .put("state", state)
            .put("detail", detail)
            .put("service_version", serviceVersion)
        return writeAtomic(CoreFiles.bridgeFile(context), payload.toString())
    }

    fun read(context: Context): BridgeSnapshot? {
        val file = CoreFiles.bridgeFile(context)
        if (!file.isFile) return null
        val text = try {
            file.readText()
        } catch (_: IOException) {
            return null
        }
        val json = try {
            JSONObject(text)
        } catch (_: JSONException) {
            return null
        }
        val port = json.optInt("port", 0)
        val token = json.optString("token")
        if (port <= 0 || port > 65535 || token.isEmpty()) return null
        return BridgeSnapshot(
            port = port,
            token = token,
            at = json.optLong("at", 0L),
            state = json.optString("state"),
            detail = json.optString("detail"),
            serviceVersion = json.optString("service_version"),
        )
    }

    /** 心跳距今多少毫秒；null 表示没跑起来或已停。 */
    fun ageMs(context: Context): Long? {
        val snapshot = read(context) ?: return null
        if (snapshot.at <= 0) return null
        return System.currentTimeMillis() - snapshot.at
    }

    private fun writeAtomic(file: File, text: String): Boolean =
        try {
            val temp = File(file.parentFile, file.name + ".tmp")
            temp.writeText(text)
            if (!temp.renameTo(file)) {
                // rename 失败（极少数文件系统）就退化成直接写，至少内容是对的。
                file.writeText(text)
                temp.delete()
            }
            true
        } catch (_: IOException) {
            false
        }
}

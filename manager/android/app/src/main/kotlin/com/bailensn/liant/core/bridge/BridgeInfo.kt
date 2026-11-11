package com.bailensn.liant.core.bridge

import android.content.Context
import com.bailensn.liant.core.CoreFiles
import org.json.JSONObject
import java.io.File
import java.io.IOException

/**
 * 把「本次启动的桥接端口 + 令牌 + 心跳时间」写进 `liant_bridge.json`。
 *
 * UI 进程读它接入；`at` 每 30 秒刷新一次，UI 用新鲜度（90 秒）判断后台还在不在。
 * 先写临时文件再改名，避免 UI 读到写了一半的 JSON。
 */
object BridgeInfo {
    fun write(context: Context, port: Int, token: String): Boolean {
        val payload = JSONObject()
            .put("port", port)
            .put("token", token)
            .put("at", System.currentTimeMillis())
        return writeAtomic(CoreFiles.bridgeFile(context), payload.toString())
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

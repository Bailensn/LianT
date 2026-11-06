package com.bailensn.liant.net

import org.json.JSONObject
import java.util.UUID

/** 协议 v1 的单层 JSON 信封（docs/PROTOCOL.md §5）。 */
object Wire {

    const val VERSION = 1

    fun newId(): String = UUID.randomUUID().toString()

    fun envelope(type: String, payload: JSONObject = JSONObject()): JSONObject =
        JSONObject()
            .put("v", VERSION)
            .put("id", newId())
            .put("type", type)
            .put("ts", System.currentTimeMillis())
            .put("payload", payload)

    /**
     * 认证签名覆盖的字节，必须与服务端 node.AuthMessage 逐字节一致：
     * nonce + "\n" + fingerprint + "\n" + 十进制 ts。
     */
    fun authMessage(nonce: String, fingerprint: String, ts: Long): ByteArray =
        "$nonce\n$fingerprint\n$ts".toByteArray(Charsets.UTF_8)
}

package com.bailensn.liant.net

import org.json.JSONObject

/**
 * 扫码得到的一次性配对信息。
 *
 * 二维码内容就是 Service 终端打印的那段 JSON：
 * {v, host, port, fp, ticket, nonce, exp}。
 */
data class PairingTicket(
    val host: String,
    val port: Int,
    val fingerprint: String,
    val ticket: String,
    val nonce: String,
    val expiresAt: Long,
) {
    fun isExpired(nowMillis: Long = System.currentTimeMillis()): Boolean =
        expiresAt > 0 && nowMillis > expiresAt * 1000

    companion object {
        fun parse(raw: String): PairingTicket {
            val json = JSONObject(raw)
            fun required(name: String): String =
                json.optString(name).takeIf { it.isNotEmpty() }
                    ?: throw IllegalArgumentException("二维码缺少 $name")
            return PairingTicket(
                host = required("host"),
                port = json.optInt("port", 8443),
                fingerprint = required("fp"),
                ticket = required("ticket"),
                nonce = required("nonce"),
                expiresAt = json.optLong("exp", 0L),
            )
        }
    }
}

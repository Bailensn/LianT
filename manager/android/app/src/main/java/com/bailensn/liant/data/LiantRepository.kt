package com.bailensn.liant.data

import android.content.Context
import com.bailensn.liant.net.LiantClient
import com.bailensn.liant.net.ManagerIdentity
import com.bailensn.liant.net.PairingTicket
import org.json.JSONObject

/** 本机保存的配对结果；私钥在 Keystore，这里只存连接参数与 device_id。 */
data class SavedPairing(
    val host: String,
    val port: Int,
    val fingerprint: String,
    val deviceId: String,
)

/** bot.list 里的一行。 */
data class BotSummary(
    val botId: String,
    val platform: String,
    val displayName: String,
    val status: String,
    val caps: List<String>,
)

/**
 * 客户端的会话入口：配对、重连、拉 Bot、发消息。
 *
 * 一个客户端只持有一条连接（协议 multi_bot），切 Bot 不重连。
 */
class LiantRepository(context: Context) {

    private val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    private val identity: ManagerIdentity by lazy { ManagerIdentity() }

    private var client: LiantClient? = null

    fun savedPairing(): SavedPairing? {
        val host = prefs.getString(KEY_HOST, null) ?: return null
        val fingerprint = prefs.getString(KEY_FP, null) ?: return null
        val deviceId = prefs.getString(KEY_DEVICE, null) ?: return null
        return SavedPairing(host, prefs.getInt(KEY_PORT, 8443), fingerprint, deviceId)
    }

    /** 首次配对。成功后把连接参数落盘，并保留这条已认证的连接。 */
    suspend fun pair(qrText: String, managerName: String): SavedPairing {
        val ticket = PairingTicket.parse(qrText)
        require(!ticket.isExpired()) { "配对二维码已过期" }
        val connection = LiantClient(ticket.host, ticket.port, ticket.fingerprint, identity)
        val result = connection.pair(ticket, managerName)
        client = connection
        val saved = SavedPairing(ticket.host, ticket.port, ticket.fingerprint, result.deviceId)
        persist(saved)
        return saved
    }

    /** 用已保存的设备身份重连并认证；旧连接会被关掉。 */
    suspend fun connect(): SavedPairing {
        val saved = savedPairing() ?: error("尚未配对")
        client?.close()
        val connection = LiantClient(saved.host, saved.port, saved.fingerprint, identity)
        connection.connect(saved.deviceId)
        client = connection
        return saved
    }

    /** 当前活跃 Bot：只有它发系统通知（D9）。 */
    fun activeBotId(): String? = prefs.getString(KEY_ACTIVE_BOT, null)

    fun setActiveBotId(botId: String) {
        prefs.edit().putString(KEY_ACTIVE_BOT, botId).apply()
    }

    fun isConnected(): Boolean = client != null

    /** 服务端推送事件（msg.inbound / event.bot）；未连接时为空。 */
    fun events(): kotlinx.coroutines.flow.SharedFlow<JSONObject>? = client?.eventFlow

    /** 连接关闭/失败信号；未连接时为空。 */
    fun closed(): kotlinx.coroutines.flow.SharedFlow<Throwable?>? = client?.closed

    suspend fun listMessages(
        botId: String,
        peerId: String,
        before: JSONObject? = null,
        after: JSONObject? = null,
        limit: Int = 100,
    ): JSONObject {
        val connection = client ?: error("连接尚未建立")
        return connection.listMessages(botId, peerId, before = before, after = after, limit = limit)
    }

    suspend fun bots(): List<BotSummary> {
        val connection = client ?: error("连接尚未建立")
        val payload = connection.botList()
        val array = payload.optJSONArray("bots") ?: return emptyList()
        return buildList {
            for (i in 0 until array.length()) {
                val item = array.optJSONObject(i) ?: continue
                add(
                    BotSummary(
                        botId = item.optString("bot_id"),
                        platform = item.optString("platform"),
                        displayName = item.optString("display_name"),
                        status = item.optString("status"),
                        caps = item.optJSONArray("caps")?.let { caps ->
                            buildList { for (j in 0 until caps.length()) add(caps.optString(j)) }
                        } ?: emptyList(),
                    ),
                )
            }
        }
    }

    suspend fun sendText(botId: String, peerId: String, text: String, inReplyTo: String?): JSONObject {
        val connection = client ?: error("连接尚未建立")
        return connection.sendMessage(botId, peerId, text, inReplyTo)
    }

    fun disconnect() {
        client?.close()
        client = null
    }

    fun forget() {
        disconnect()
        prefs.edit().clear().apply()
    }

    private fun persist(saved: SavedPairing) {
        prefs.edit()
            .putString(KEY_HOST, saved.host)
            .putInt(KEY_PORT, saved.port)
            .putString(KEY_FP, saved.fingerprint)
            .putString(KEY_DEVICE, saved.deviceId)
            .apply()
    }

    private companion object {
        const val PREFS = "liant"
        const val KEY_HOST = "host"
        const val KEY_PORT = "port"
        const val KEY_FP = "fingerprint"
        const val KEY_DEVICE = "device_id"
        const val KEY_ACTIVE_BOT = "active_bot_id"
    }
}

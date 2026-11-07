package com.bailensn.liant.net

import android.util.Base64
import java.io.IOException
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.withTimeout
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject

/** 协议错误（服务端 error 帧或客户端校验失败）。 */
class ProtocolException(val code: String, message: String) : IOException(message)

/**
 * 一条到 Service 的 WSS 连接：TLS 指纹钉 + 挑战认证 + 请求/响应 + 事件流。
 *
 * 一个连接承载所有 Bot（协议 multi_bot），切换当前 Bot 只是本地状态，不重连。
 */
class LiantClient(
    private val host: String,
    private val port: Int,
    private val pinnedFingerprint: String,
    private val identity: ManagerIdentity,
) {

    data class Challenge(val nonce: String, val ts: Long)

    data class Reply(val type: String, val payload: JSONObject)

    data class PairResult(val deviceId: String, val auth: JSONObject)

    private val http: OkHttpClient = TlsPinning.client(pinnedFingerprint)
    private val pending = ConcurrentHashMap<String, CompletableDeferred<Reply>>()
    private val challenge = CompletableDeferred<Challenge>()
    private val events = MutableSharedFlow<JSONObject>(extraBufferCapacity = 128)
    private val _closed = MutableSharedFlow<Throwable?>(replay = 1, extraBufferCapacity = 1)

    private var socket: WebSocket? = null
    private var authInfo: JSONObject? = null

    /** 服务端事件（msg.inbound / event.bot）。 */
    val eventFlow: SharedFlow<JSONObject> get() = events

    /** 连接关闭/失败时发一次，供上层做退避重连。 */
    val closed: SharedFlow<Throwable?> get() = _closed

    var managerId: String = ""
        private set

    /** auth.ok 的 payload（caps / retention / service_time）。 */
    val auth: JSONObject? get() = authInfo

    fun open() {
        val request = Request.Builder().url("wss://$host:$port/v1/ws").build()
        socket = http.newWebSocket(request, listener)
    }

    /** 首次配对：用二维码里的 ticket + nonce 换取 device_id，然后立刻认证。 */
    suspend fun pair(ticket: PairingTicket, managerName: String): PairResult {
        open()
        val ch = withTimeout(HANDSHAKE_TIMEOUT_MS) { challenge.await() }

        val signature = identity.sign(ticket.nonce.toByteArray(Charsets.UTF_8))
        val reply = call(
            "pair.request",
            JSONObject()
                .put("ticket", ticket.ticket)
                .put("manager_name", managerName)
                .put("manager_pub", identity.publicKeyBase64())
                .put("sig", Base64.encodeToString(signature, Base64.NO_WRAP)),
            // 终端上要人工确认，给足时间。
            timeoutMs = PAIR_TIMEOUT_MS,
        )
        if (reply.type != "pair.ok") {
            throw ProtocolException(reply.type, reply.payload.optString("reason", "配对被拒绝"))
        }
        val deviceId = reply.payload.optString("device_id")
        if (deviceId.isEmpty()) {
            throw ProtocolException("pair.ok", "服务端没有返回 device_id")
        }
        val auth = authenticate(ch, deviceId)
        return PairResult(deviceId, auth)
    }

    /** 已配对设备重连并认证。 */
    suspend fun connect(deviceId: String): JSONObject {
        open()
        val ch = withTimeout(HANDSHAKE_TIMEOUT_MS) { challenge.await() }
        return authenticate(ch, deviceId)
    }

    suspend fun botList(): JSONObject = call("bot.list", JSONObject()).payload

    suspend fun sendMessage(
        botId: String,
        peerId: String,
        plain: String,
        inReplyTo: String?,
        operatorId: String? = null,
    ): JSONObject {
        val body = JSONObject().put("plain", plain)
        val payload = JSONObject()
            .put("bot_id", botId)
            .put("peer_id", peerId)
            .put("body", body)
        if (inReplyTo != null) payload.put("in_reply_to", inReplyTo)
        if (operatorId != null) payload.put("operator_id", operatorId)
        return call("msg.send", payload).payload
    }

    suspend fun listMessages(
        botId: String,
        peerId: String,
        before: JSONObject? = null,
        after: JSONObject? = null,
        limit: Int = 50,
    ): JSONObject {
        val payload = JSONObject()
            .put("bot_id", botId)
            .put("peer_id", peerId)
            .put("limit", limit)
        // 服务端只接受 before/after 之一。
        if (after != null) {
            payload.put("after", after)
        } else if (before != null) {
            payload.put("before", before)
        }
        return call("msg.list", payload).payload
    }

    /**
     * 取媒体字节：先 media.fetch 拿签名地址，再用同一条钉过指纹的连接下载。
     *
     * 地址可能是相对路径，按当前连接的主机拼接。
     */
    suspend fun fetchMedia(botId: String, msgId: String): ByteArray {
        val payload = call(
            "media.fetch",
            JSONObject().put("bot_id", botId).put("msg_id", msgId),
        ).payload
        var url = payload.optString("url")
        if (url.isEmpty()) throw ProtocolException("media.fetch", "服务端没有返回地址")
        if (url.startsWith("/")) url = "https://$host:$port$url"
        val request = Request.Builder().url(url).build()
        return withContext(Dispatchers.IO) {
            http.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    throw IOException("媒体下载失败（${response.code}）")
                }
                response.body?.bytes() ?: throw IOException("媒体内容为空")
            }
        }
    }

    fun close() {
        socket?.close(1000, null)
        failAll(IOException("连接已关闭"))
    }

    private suspend fun authenticate(challenge: Challenge, deviceId: String): JSONObject {
        val signature = identity.sign(
            Wire.authMessage(challenge.nonce, pinnedFingerprint, challenge.ts),
        )
        val reply = call(
            "auth.response",
            JSONObject()
                .put("manager_id", deviceId)
                .put("sig", Base64.encodeToString(signature, Base64.NO_WRAP)),
        )
        if (reply.type != "auth.ok") {
            throw ProtocolException(reply.type, "认证失败")
        }
        managerId = deviceId
        authInfo = reply.payload
        return reply.payload
    }

    private suspend fun call(
        type: String,
        payload: JSONObject,
        timeoutMs: Long = REQUEST_TIMEOUT_MS,
    ): Reply {
        val envelope = Wire.envelope(type, payload)
        val id = envelope.getString("id")
        val deferred = CompletableDeferred<Reply>()
        pending[id] = deferred
        try {
            val webSocket = socket ?: throw IOException("连接尚未建立")
            if (!webSocket.send(envelope.toString())) {
                throw IOException("发送失败：连接不可用")
            }
            return withTimeout(timeoutMs) { deferred.await() }
        } finally {
            pending.remove(id)
        }
    }

    private val listener = object : WebSocketListener() {
        override fun onMessage(webSocket: WebSocket, text: String) {
            try {
                handle(text)
            } catch (e: Exception) {
                failAll(e)
            }
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            failAll(t)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            failAll(IOException("连接已关闭（$code）"))
        }
    }

    private fun handle(text: String) {
        val envelope = JSONObject(text)
        val type = envelope.optString("type")
        val payload = envelope.optJSONObject("payload") ?: JSONObject()
        val replyTo = envelope.optString("reply_to").takeIf { it.isNotEmpty() }

        if (type == "auth.challenge") {
            challenge.complete(
                Challenge(payload.optString("nonce"), payload.optLong("ts")),
            )
            return
        }
        if (replyTo != null) {
            val deferred = pending.remove(replyTo) ?: return
            if (type == "error") {
                val code = payload.optString("code", "internal")
                deferred.completeExceptionally(
                    ProtocolException(code, payload.optString("message", "请求失败")),
                )
            } else {
                deferred.complete(Reply(type, payload))
            }
            return
        }
        events.tryEmit(envelope)
    }

    private fun failAll(cause: Throwable) {
        _closed.tryEmit(cause)
        challenge.completeExceptionally(cause)
        val iterator = pending.values.iterator()
        while (iterator.hasNext()) {
            iterator.next().completeExceptionally(cause)
        }
        pending.clear()
    }

    companion object {
        private const val HANDSHAKE_TIMEOUT_MS = 20_000L
        private const val REQUEST_TIMEOUT_MS = 15_000L
        private const val PAIR_TIMEOUT_MS = 300_000L
    }
}

package com.bailensn.liant.core.net

import android.util.Base64
import android.util.Log
import com.bailensn.liant.core.crypto.P256
import com.bailensn.liant.core.state.Identity
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONException
import org.json.JSONObject
import java.security.SecureRandom
import java.security.cert.X509Certificate
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLException
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager
import kotlin.math.min
import kotlin.math.roundToInt

/**
 * 协议 v1 的 WebSocket 客户端（Kotlin 版）。
 *
 * 与 Dart 的 `core/net/client.dart` 一一对应：
 * - 自签证书放行（真正的信任边界是 auth.challenge 里的指纹校验，不是 TLS）；
 * - `auth.challenge` → 验服务端自证 → 回 `auth.response`；
 * - 30 秒应用层 ping，收到 ping 回 pong；
 * - 1→30 秒指数退避重连，4001/4002 不重连；
 * - 请求-响应按 `reply_to` 关联，15 秒超时。
 *
 * 第一步只用来验证「连得上、认证得过」；桥接与通知在第二步接进来。
 */
class LiantSocket(
    val identity: Identity,
    private val listener: Listener,
) {
    /** 与 Dart 的 `ConnState` 同名，桥接帧里直接用它的小写名字。 */
    enum class State { idle, connecting, online, reconnecting, error }

    interface Listener {
        fun onState(state: State, detail: String)

        /** 协议事件原样转发（完整信封）；ping/pong/请求响应不会到这里。 */
        fun onEnvelope(envelope: JSONObject)
    }

    /** 协议层错误；`code` 取值见 PROTOCOL.md §7。 */
    class ProtocolException(
        val code: String,
        message: String,
        val retryable: Boolean = false,
    ) : Exception(message)

    private class Pending(
        val timeout: ScheduledFuture<*>,
        val callback: (Result<JSONObject>) -> Unit,
    )

    private val scheduler: ScheduledExecutorService =
        Executors.newSingleThreadScheduledExecutor { runnable ->
            Thread(runnable, "liant-socket").apply { isDaemon = true }
        }

    private val random = SecureRandom()

    private val http: OkHttpClient = buildHttpClient()

    private val pending = ConcurrentHashMap<String, Pending>()

    @Volatile
    private var webSocket: WebSocket? = null

    @Volatile
    private var closed = true

    @Volatile
    private var fatal = false

    @Volatile
    private var online = false

    @Volatile
    private var generation = 0

    @Volatile
    private var retryCount = 0

    @Volatile
    private var state: State = State.idle

    @Volatile
    private var detail: String = ""

    private var authTimeout: ScheduledFuture<*>? = null
    private var pingTask: ScheduledFuture<*>? = null
    private var retryTask: ScheduledFuture<*>? = null

    val isOnline: Boolean get() = online

    /** 当前连接状态；桥接刚接上时要补一条给 UI。 */
    fun connection(): Pair<State, String> = state to detail

    // -------------------------------------------------------------------------
    // 生命周期
    // -------------------------------------------------------------------------

    fun start() {
        synchronized(this) {
            if (!closed) return
            closed = false
            fatal = false
            retryCount = 0
        }
        attempt()
    }

    fun stop() {
        var socket: WebSocket? = null
        synchronized(this) {
            if (closed) return
            closed = true
            online = false
            generation++
            cancelTimers()
            socket = webSocket
            webSocket = null
        }
        failAll(ProtocolException("offline", "连接已关闭"))
        socket?.close(CLOSE_NORMAL, "client stop")
        setState(State.idle, "")
        scheduler.shutdown()
        http.dispatcher.executorService.shutdown()
        http.connectionPool.evictAll()
    }

    // -------------------------------------------------------------------------
    // 连接与握手
    // -------------------------------------------------------------------------

    private fun attempt() {
        if (closed || fatal) return
        val current = synchronized(this) {
            if (closed || fatal) return
            ++generation
        }
        online = false
        setState(
            if (retryCount == 0) State.connecting else State.reconnecting,
            if (retryCount == 0) "" else "第 $retryCount 次重连",
        )

        val request = try {
            Request.Builder().url(identity.wsUrl).build()
        } catch (_: IllegalArgumentException) {
            failFatally("服务地址不合法：${identity.wsUrl}")
            return
        }

        authTimeout?.cancel(false)
        authTimeout = scheduler.schedule(
            { handleClosed(current, "认证超时") },
            AUTH_TIMEOUT_MS,
            TimeUnit.MILLISECONDS,
        )

        val webSocketListener = object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                if (current != generation) {
                    webSocket.close(CLOSE_NORMAL, null)
                    return
                }
                Log.i(TAG, "核心：WebSocket 已打开 ${identity.wsUrl}")
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                if (current != generation) return
                handleText(text)
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                webSocket.close(CLOSE_NORMAL, null)
                handleClosed(current, closeReason(CLOSE_NORMAL, code, reason), code)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                handleClosed(current, closeReason(CLOSE_NORMAL, code, reason), code)
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                handleClosed(current, describe(t), response?.code)
            }
        }
        val socket = try {
            http.newWebSocket(request, webSocketListener)
        } catch (error: Exception) {
            handleClosed(current, describe(error))
            return
        }
        webSocket = socket
    }

    private fun handleText(text: String) {
        try {
            val envelope = parseEnvelope(text) ?: return
            when (envelope.type) {
                TYPE_AUTH_CHALLENGE -> {
                    answerChallenge(envelope)
                    return
                }
                TYPE_AUTH_OK -> onAuthOk()
            }
            dispatch(envelope)
        } catch (error: Exception) {
            // 单条消息处理失败不能把 :background 进程带走。
            Log.w(TAG, "核心：处理消息失败", error)
        }
    }

    private fun answerChallenge(envelope: Envelope) {
        val payload = envelope.payload
        val nonce = payload.optString("nonce")
        val ts = payload.optLong("ts", 0L)
        if (nonce.isEmpty() || ts == 0L) {
            failFatally("服务端挑战格式不正确")
            return
        }
        // 待签字节固定为 nonce \n 指纹 \n 十进制 ts（协议 §4）。
        val message = "$nonce\n${identity.fingerprint}\n$ts".toByteArray(Charsets.UTF_8)
        val servicePub = stringOrNull(payload, "service_pub")
        val serviceSig = stringOrNull(payload, "service_sig")
        try {
            if (servicePub != null && serviceSig != null) {
                P256.verifyServiceProof(
                    servicePubDer = decodeBase64(servicePub, "服务端公钥"),
                    expectedFingerprint = identity.fingerprint,
                    message = message,
                    signatureDer = decodeBase64(serviceSig, "服务端签名"),
                )
            } else if (identity.fingerprint.isNotEmpty()) {
                // 配对时钉过指纹，服务端却说不出自己是谁：拒绝连接，不给「忽略」开关。
                throw SecurityException("服务端没有提供身份证明，拒绝连接")
            }
        } catch (error: SecurityException) {
            failFatally(error.message ?: "服务端身份校验失败")
            return
        }

        val signature = try {
            P256.sign(identity.privateKey, message)
        } catch (error: Exception) {
            failFatally("设备签名失败：${error.message}")
            return
        }

        val socket = webSocket ?: return
        val response = envelope(
            id = newId(),
            type = TYPE_AUTH_RESPONSE,
            replyTo = envelope.id,
            payload = JSONObject()
                .put("manager_id", identity.deviceId)
                .put("sig", Base64.encodeToString(signature, Base64.NO_WRAP)),
        )
        if (!socket.send(response.toString())) {
            failFatally("连接已关闭，无法回复认证")
            return
        }
        Log.i(TAG, "核心：已回复 auth.response（${identity.deviceId}）")
    }

    private fun onAuthOk() {
        online = true
        retryCount = 0
        authTimeout?.cancel(false)
        authTimeout = null
        startPing()
        setState(State.online, "")
        Log.i(TAG, "核心：auth.ok（${identity.host}:${identity.port}）")
    }

    private fun handleClosed(closedGeneration: Int, reason: String, closeCode: Int? = null) {
        synchronized(this) {
            if (closedGeneration != generation || closed) return
            // 同一次连接只处理一次：onClosing/onClosed/onFailure 可能先后到达。
            generation++
            online = false
            cancelTimers()
            webSocket = null
        }
        failAll(ProtocolException("offline", "连接已断开"))
        if (fatal) return
        // 协议 §9：4001 认证失败、4002 版本不兼容不重连。
        if (closeCode == CLOSE_AUTH_FAILED || closeCode == CLOSE_VERSION) {
            fatal = true
            setState(
                State.error,
                if (closeCode == CLOSE_AUTH_FAILED) {
                    "认证失败：设备可能已被服务端吊销"
                } else {
                    "客户端与服务端版本不兼容"
                },
            )
            return
        }
        scheduleRetry(reason)
    }

    private fun scheduleRetry(reason: String) {
        if (closed || fatal) return
        retryCount++
        val exponent = (retryCount - 1).coerceIn(0, 5)
        val baseSeconds = min(1 shl exponent, MAX_RETRY_SECONDS)
        // ±20% 抖动，避免多台设备在同一秒重连。
        val jitter = 0.8 + random.nextDouble() * 0.4
        val delayMs = (baseSeconds * 1000 * jitter).toLong().coerceAtLeast(200)
        val text = reason.ifEmpty { "连接中断" }
        setState(State.reconnecting, "$text · ${(delayMs / 1000.0).roundToInt()} 秒后重试")
        retryTask?.cancel(false)
        retryTask = scheduler.schedule({ attempt() }, delayMs, TimeUnit.MILLISECONDS)
    }

    private fun failFatally(reason: String) {
        var socket: WebSocket? = null
        synchronized(this) {
            if (closed) return
            fatal = true
            online = false
            generation++
            cancelTimers()
            socket = webSocket
            webSocket = null
        }
        socket?.close(CLOSE_NORMAL, "fatal")
        failAll(ProtocolException("unauthorized", reason))
        setState(State.error, reason)
        Log.w(TAG, "核心：$reason")
    }

    private fun cancelTimers() {
        authTimeout?.cancel(false)
        authTimeout = null
        pingTask?.cancel(false)
        pingTask = null
        retryTask?.cancel(false)
        retryTask = null
    }

    private fun startPing() {
        pingTask?.cancel(false)
        pingTask = scheduler.scheduleWithFixedDelay(
            {
                if (!online || webSocket == null) return@scheduleWithFixedDelay
                try {
                    send(TYPE_PING, JSONObject())
                } catch (_: Exception) {
                    // 下一轮 onFailure/onClosed 会触发重连。
                }
            },
            PING_INTERVAL_MS,
            PING_INTERVAL_MS,
            TimeUnit.MILLISECONDS,
        )
    }

    // -------------------------------------------------------------------------
    // 收发
    // -------------------------------------------------------------------------

    /** 发一条不需要响应的消息，返回信封 id。 */
    fun send(type: String, payload: JSONObject): String {
        val socket = webSocket
        if (!online || socket == null) throw ProtocolException("offline", "连接未就绪")
        val id = newId()
        if (!socket.send(envelope(id, type, null, payload).toString())) {
            throw ProtocolException("offline", "连接已关闭")
        }
        return id
    }

    /** 发一条请求并等待同 id 的响应；超时与 error 都通过 callback 返回失败。 */
    fun request(
        type: String,
        payload: JSONObject,
        timeoutMs: Long = REQUEST_TIMEOUT_MS,
        callback: (Result<JSONObject>) -> Unit,
    ): String? {
        val socket = webSocket
        if (!online || socket == null) {
            callback(Result.failure(ProtocolException("offline", "连接未就绪")))
            return null
        }
        val id = newId()
        val timeout = scheduler.schedule(
            {
                val entry = pending.remove(id) ?: return@schedule
                entry.callback(Result.failure(ProtocolException("timeout", "请求超时", true)))
            },
            timeoutMs,
            TimeUnit.MILLISECONDS,
        )
        pending[id] = Pending(timeout, callback)
        if (!socket.send(envelope(id, type, null, payload).toString())) {
            pending.remove(id)?.let { it.timeout.cancel(false) }
            callback(Result.failure(ProtocolException("offline", "连接已关闭")))
            return null
        }
        return id
    }

    private fun dispatch(envelope: Envelope) {
        val replyTo = envelope.replyTo
        if (replyTo != null) {
            val entry = pending.remove(replyTo)
            if (entry != null) {
                entry.timeout.cancel(false)
                if (envelope.type == TYPE_ERROR) {
                    entry.callback(Result.failure(protocolError(envelope.payload)))
                } else {
                    entry.callback(Result.success(envelope.payload))
                }
                return
            }
        }
        when (envelope.type) {
            TYPE_PING -> {
                try {
                    send(TYPE_PONG, JSONObject())
                } catch (_: Exception) {
                    // 连接已断，重连由 onFailure/onClosed 负责。
                }
            }
            TYPE_PONG -> Unit
            else -> listener.onEnvelope(envelope.json)
        }
    }

    private fun failAll(error: ProtocolException) {
        val entries = pending.values.toList()
        pending.clear()
        for (entry in entries) {
            entry.timeout.cancel(false)
            entry.callback(Result.failure(error))
        }
    }

    private fun setState(next: State, nextDetail: String) {
        state = next
        detail = nextDetail
        listener.onState(next, nextDetail)
    }

    // -------------------------------------------------------------------------
    // 信封与工具
    // -------------------------------------------------------------------------

    private class Envelope(
        val id: String,
        val type: String,
        val replyTo: String?,
        val payload: JSONObject,
        val json: JSONObject,
    )

    private fun parseEnvelope(raw: String): Envelope? =
        try {
            val json = JSONObject(raw)
            val type = json.optString("type")
            val id = json.optString("id")
            if (type.isEmpty() || id.isEmpty()) {
                null
            } else {
                Envelope(
                    id = id,
                    type = type,
                    replyTo = stringOrNull(json, "reply_to"),
                    payload = json.optJSONObject("payload") ?: JSONObject(),
                    json = json,
                )
            }
        } catch (_: JSONException) {
            null
        }

    private fun envelope(
        id: String,
        type: String,
        replyTo: String?,
        payload: JSONObject,
    ): JSONObject = JSONObject()
        .put("v", PROTOCOL_VERSION)
        .put("id", id)
        .put("type", type)
        .put("ts", System.currentTimeMillis())
        .apply { if (replyTo != null) put("reply_to", replyTo) }
        .put("payload", payload)

    private fun protocolError(payload: JSONObject): ProtocolException = ProtocolException(
        code = payload.optString("code", "internal"),
        message = payload.optString("message"),
        retryable = payload.optBoolean("retryable", false),
    )

    private fun stringOrNull(json: JSONObject, key: String): String? {
        if (!json.has(key) || json.isNull(key)) return null
        return json.optString(key).ifEmpty { null }
    }

    private fun decodeBase64(text: String, label: String): ByteArray =
        try {
            Base64.decode(text, Base64.NO_WRAP)
        } catch (_: IllegalArgumentException) {
            throw SecurityException("$label 不是合法的 base64")
        }

    private fun newId(): String = UUID.randomUUID().toString()

    private fun closeReason(normal: Int, code: Int, reason: String): String = when {
        reason.isNotEmpty() -> reason
        code == normal -> "连接已断开"
        else -> "连接关闭（$code）"
    }

    private fun describe(error: Throwable): String {
        val text = error.message ?: error.javaClass.simpleName
        return when {
            error is ProtocolException -> text
            error is SSLException -> "TLS 握手失败：$text"
            text.contains("Failed to connect") || text.contains("Connection refused") -> "无法连接到服务"
            text.length > 120 -> text.take(120)
            else -> text
        }
    }

    private fun buildHttpClient(): OkHttpClient {
        // 自托管 Service 默认自签证书：TLS 不是信任边界，auth.challenge 的指纹校验才是。
        val trustManager = object : X509TrustManager {
            override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit

            override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit

            override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
        }
        val sslContext = SSLContext.getInstance("TLS")
        sslContext.init(null, arrayOf<TrustManager>(trustManager), SecureRandom())
        return OkHttpClient.Builder()
            .connectTimeout(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS)
            // 读/写超时交给应用层心跳与重连，不设死。
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .writeTimeout(0, TimeUnit.MILLISECONDS)
            .pingInterval(0, TimeUnit.MILLISECONDS)
            .sslSocketFactory(sslContext.socketFactory, trustManager)
            .hostnameVerifier { _, _ -> true }
            .build()
    }

    private companion object {
        const val TAG = "LianT"

        const val PROTOCOL_VERSION = 1

        const val PING_INTERVAL_MS = 30_000L
        const val AUTH_TIMEOUT_MS = 20_000L
        const val REQUEST_TIMEOUT_MS = 15_000L
        const val CONNECT_TIMEOUT_SECONDS = 15L
        const val MAX_RETRY_SECONDS = 30

        const val CLOSE_NORMAL = 1000
        const val CLOSE_AUTH_FAILED = 4001
        const val CLOSE_VERSION = 4002

        const val TYPE_AUTH_CHALLENGE = "auth.challenge"
        const val TYPE_AUTH_RESPONSE = "auth.response"
        const val TYPE_AUTH_OK = "auth.ok"
        const val TYPE_PING = "ping"
        const val TYPE_PONG = "pong"
        const val TYPE_ERROR = "error"
    }
}

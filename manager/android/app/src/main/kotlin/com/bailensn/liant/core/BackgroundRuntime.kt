package com.bailensn.liant.core

import android.content.Context
import android.util.Log
import com.bailensn.liant.core.bridge.BridgeInfo
import com.bailensn.liant.core.bridge.BridgeServer
import com.bailensn.liant.core.net.LiantSocket
import com.bailensn.liant.core.state.BackgroundState
import com.bailensn.liant.core.state.Identity
import com.bailensn.liant.core.state.StateStore
import org.json.JSONObject
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/**
 * `:background` 进程的全部工作，替代原来的 Android 后台 FlutterEngine：
 *
 * 1. 起本地桥接，把端口/令牌写进 `liant_bridge.json`（30 秒心跳）；
 * 2. 读共享状态文件拿身份，连服务端（全 App 唯一一条 WebSocket）；
 * 3. 把状态与事件实时转给 UI 进程；
 * 4. UI 不在前台时弹通知；
 * 5. 每分钟对一次共享状态：识别解除配对、重新配对。
 *
 * 这里不依赖 Flutter，所以后台进程的 PSS 从「第二个引擎」降到纯 Kotlin 运行时。
 */
class BackgroundRuntime(
    private val context: Context,
    /** 更新常驻通知文案；由 MessageService 切到主线程执行。 */
    private val status: (String) -> Unit,
) {
    @Volatile
    private var running = false

    @Volatile
    private var socket: LiantSocket? = null

    @Volatile
    private var state: BackgroundState? = null

    @Volatile
    private var lastState = LiantSocket.State.idle

    @Volatile
    private var lastDetail = ""

    @Volatile
    private var bridgePort = 0

    @Volatile
    private var serviceVersion = ""

    private val bridge = BridgeServer(
        clientOf = { socket },
        connectionOf = {
            val current = socket
            if (current == null) {
                LiantSocket.State.idle to ""
            } else {
                current.connection()
            }
        },
    )

    private val scheduler: ScheduledExecutorService =
        Executors.newSingleThreadScheduledExecutor { runnable ->
            Thread(runnable, "liant-runtime").apply { isDaemon = true }
        }

    private val notifier = Notifier(context)
    private val received = AtomicInteger(0)

    private var heartbeatTask: ScheduledFuture<*>? = null
    private var refreshTask: ScheduledFuture<*>? = null
    private var retryTask: ScheduledFuture<*>? = null

    fun start() {
        if (running) return
        running = true
        val thread = Thread({ run() }, "liant-runtime-start")
        thread.start()
    }

    fun stop() {
        running = false
        heartbeatTask?.cancel(false)
        refreshTask?.cancel(false)
        retryTask?.cancel(false)
        socket?.stop()
        socket = null
        bridge.stop()
        scheduler.shutdownNow()
    }

    // -------------------------------------------------------------------------
    // 启动
    // -------------------------------------------------------------------------

    private fun run() {
        try {
            bridge.start()
            val port = bridge.port
            if (port == null) {
                Log.w(TAG, "后台桥接启动失败")
                status("后台桥接启动失败")
                return
            }
            bridgePort = port
            touchBridge()
            heartbeatTask = scheduler.scheduleWithFixedDelay(
                { touchBridge() },
                HEARTBEAT_SECONDS,
                HEARTBEAT_SECONDS,
                TimeUnit.SECONDS,
            )
            // 每分钟对一次共享状态：解除配对、重新配对这类运行期变化。
            refreshTask = scheduler.scheduleWithFixedDelay(
                { refreshState() },
                REFRESH_SECONDS,
                REFRESH_SECONDS,
                TimeUnit.SECONDS,
            )
            // 配对后 UI 先写状态文件、再拉服务，后台可能抢在写盘前启动：
            // 3 秒后补一次，避免界面白等一分钟「后台未就绪」。
            scheduler.schedule({ refreshState() }, 3, TimeUnit.SECONDS)
            val loaded = StateStore.read(context)
            if (loaded == null) {
                // 没有配对信息（例如用户刚解除配对），先不连接，等状态文件出现。
                status("未配对")
                return
            }
            state = loaded
            connect(loaded)
        } catch (error: Exception) {
            Log.w(TAG, "后台运行时启动失败", error)
            CoreLog.record(context, "后台运行时启动失败：$error")
            status("后台服务启动失败")
        }
    }

    /** 刷新桥接文件：心跳时间 + 当前连接状态（原生界面靠它显示在线状态）。 */
    private fun touchBridge() {
        if (!running || bridgePort <= 0) return
        BridgeInfo.write(
            context = context,
            port = bridgePort,
            token = bridge.currentToken,
            state = lastState.wire,
            detail = lastDetail,
            serviceVersion = serviceVersion,
        )
    }

    // -------------------------------------------------------------------------
    // 连接
    // -------------------------------------------------------------------------

    private fun connect(loaded: BackgroundState) {
        socket?.stop()
        val client = LiantSocket(
            loaded.identity,
            object : LiantSocket.Listener {
                override fun onState(state: LiantSocket.State, detail: String) {
                    lastState = state
                    lastDetail = detail
                    bridge.broadcastState(state, detail)
                    touchBridge()
                    status(statusText(state, detail))
                    if (state == LiantSocket.State.online) {
                        retryTask?.cancel(false)
                        retryTask = null
                    } else if (state == LiantSocket.State.error) {
                        // 认证失败之类是「不重连」的终态；隔几分钟再试一次，
                        // 这样设备被重新授权后不用等用户手动打开 App。
                        scheduleRetry()
                    }
                }

                override fun onEnvelope(envelope: JSONObject) {
                    if (envelope.optString("type") == "auth.ok") {
                        val version = envelope.optJSONObject("payload")?.optString("version").orEmpty()
                        if (version.isNotEmpty()) {
                            serviceVersion = version
                            touchBridge()
                        }
                    }
                    // 先转给 UI：会话标题、Bot 状态、审计都靠这些事件。
                    bridge.broadcastEvent(envelope)
                    val type = envelope.optString("type")
                    if (type != TYPE_INBOUND && type != TYPE_EVENT_MSG) return
                    received.incrementAndGet()
                    if (lastState == LiantSocket.State.online) status(onlineText())
                    val payload = envelope.optJSONObject("payload") ?: return
                    onInbound(payload)
                }
            },
        )
        socket = client
        client.start()
    }

    private fun scheduleRetry() {
        if (!running) return
        retryTask?.cancel(false)
        retryTask = scheduler.schedule(
            {
                val next = StateStore.read(context)
                if (next != null && running) {
                    state = next
                    connect(next)
                }
            },
            RETRY_MINUTES,
            TimeUnit.MINUTES,
        )
    }

    private fun refreshState() {
        if (!running) return
        val next = StateStore.read(context)
        if (next == null) {
            socket?.stop()
            socket = null
            state = null
            lastState = LiantSocket.State.idle
            lastDetail = ""
            bridge.broadcastState(lastState, "")
            touchBridge()
            status("未配对")
            return
        }
        val current = socket?.identity
        state = next
        if (current == null || !sameTarget(current, next.identity)) {
            connect(next)
        }
    }

    private fun sameTarget(a: Identity, b: Identity): Boolean =
        a.deviceId == b.deviceId &&
            a.host == b.host &&
            a.port == b.port &&
            a.secure == b.secure

    // -------------------------------------------------------------------------
    // 通知
    // -------------------------------------------------------------------------

    private fun onInbound(payload: JSONObject) {
        val message = InboundMessage.from(payload) ?: return
        // 每次通知都重新读一遍共享状态，免打扰/前台开关立刻生效。
        val current = StateStore.read(context) ?: state ?: return
        state = current
        // foreground 由 UI 进程写，UI 进程被系统杀掉时会留下过期的 true。
        // 桥接上没有对端 = UI 已经没了，这时必须照常通知。
        if (current.foreground && bridge.hasPeer) return
        val key = "${message.botId}|${message.peerId}"
        if (current.muted[key] == true) return

        val isGroup = message.peerKind == "group"
        val fallback =
            current.remarks[key] ?: current.titles[key] ?: shortPeer(message.peerId)
        val sender = message.senderName.ifEmpty { shortPeer(message.senderId) }
        val title = clip(
            if (isGroup) fallback else message.senderName.ifEmpty { fallback },
            36,
        )
        val summary = summary(message)
        val body = clip(if (isGroup) "$sender: $summary" else summary, 90)
        notifier.notify(message.msgId, title, body)
    }

    private fun summary(message: InboundMessage): String = when (message.msgType) {
        "image" -> "[图片]"
        "video" -> "[视频]"
        "audio" -> "[语音]"
        "file" -> message.plain.trim().let { if (it.isEmpty()) "[文件]" else "[文件]$it" }
        else -> message.plain.trim().ifEmpty { "[消息]" }
    }

    // -------------------------------------------------------------------------
    // 文案
    // -------------------------------------------------------------------------

    private fun statusText(state: LiantSocket.State, detail: String): String = when (state) {
        LiantSocket.State.online -> onlineText()
        LiantSocket.State.connecting -> "正在连接…"
        LiantSocket.State.reconnecting -> "连接中断，正在重连…"
        LiantSocket.State.error -> detail.ifEmpty { "连接失败" }
        LiantSocket.State.idle -> "未连接"
    }

    private fun onlineText(): String {
        val count = received.get()
        return if (count == 0) "已连接 · 正在后台接收消息" else "已连接 · 已收到 $count 条"
    }

    private fun clip(text: String, max: Int): String {
        val flat = text.replace(Regex("\\s+"), " ").trim()
        return if (flat.length <= max) flat else flat.take(max) + "..."
    }

    /** openid 很长，标题只显示后 6 位（与 Dart 的 shortPeer 一致）。 */
    private fun shortPeer(peerId: String): String {
        val raw = if (peerId.contains(':')) peerId.split(':').drop(1).joinToString(":") else peerId
        if (raw.isEmpty()) return "未知会话"
        return if (raw.length > 8) "用户 ${raw.takeLast(6)}" else "用户 $raw"
    }

    private companion object {
        const val TAG = "LianT"
        const val TYPE_INBOUND = "msg.inbound"
        const val TYPE_EVENT_MSG = "event.msg"
        const val HEARTBEAT_SECONDS = 30L
        const val REFRESH_SECONDS = 60L
        const val RETRY_MINUTES = 5L
    }
}

/**
 * 通知只需要协议里这几个字段；其余留给 UI 的 AppState 加工。
 * 字段名与 `ChatMessage.fromJson` 完全一致。
 */
private class InboundMessage(
    val msgId: String,
    val botId: String,
    val peerId: String,
    val peerKind: String,
    val senderId: String,
    val senderName: String,
    val msgType: String,
    val plain: String,
) {
    companion object {
        fun from(payload: JSONObject): InboundMessage? {
            val msgId = payload.optString("msg_id")
            val botId = payload.optString("bot_id")
            if (msgId.isEmpty() || botId.isEmpty()) return null
            val body = payload.optJSONObject("body")
            return InboundMessage(
                msgId = msgId,
                botId = botId,
                peerId = payload.optString("peer_id"),
                peerKind = payload.optString("peer_kind"),
                senderId = payload.optString("sender_id"),
                senderName = payload.optString("sender_name"),
                msgType = payload.optString("msg_type", "text"),
                plain = body?.optString("plain") ?: "",
            )
        }
    }
}

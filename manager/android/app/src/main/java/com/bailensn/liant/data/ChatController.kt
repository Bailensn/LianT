package com.bailensn.liant.data

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.ProcessLifecycleOwner
import com.bailensn.liant.notify.MessageNotifier
import com.bailensn.liant.net.ProtocolException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * 会话与消息的本地状态：入站事件落库、打开会话、发送与错误。
 *
 * 所有数据库与网络调用都在 IO 线程；UI 只订阅 StateFlow。
 */
class ChatController(context: Context, private val repository: LiantRepository) {

    private val store = LocalStore(context.applicationContext)
    private val notifier = MessageNotifier(context.applicationContext)

    private val _conversations = MutableStateFlow<List<Conversation>>(emptyList())
    val conversations: StateFlow<List<Conversation>> = _conversations.asStateFlow()

    private val _messages = MutableStateFlow<List<ChatMessage>>(emptyList())
    val messages: StateFlow<List<ChatMessage>> = _messages.asStateFlow()

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    /** 已下载的图片，按 msg_id 缓存；过期/淘汰的不放进来，UI 继续显示占位。 */
    private val _media = MutableStateFlow<Map<String, Bitmap>>(emptyMap())
    val media: StateFlow<Map<String, Bitmap>> = _media.asStateFlow()

    /** 图片没拉下来的原因：media_expired / media_evicted / failed。 */
    private val _mediaErrors = MutableStateFlow<Map<String, String>>(emptyMap())
    val mediaErrors: StateFlow<Map<String, String>> = _mediaErrors.asStateFlow()

    private var active: Conversation? = null

    suspend fun refresh() = withContext(Dispatchers.IO) {
        _conversations.value = store.conversations()
    }

    suspend fun open(conversation: Conversation) = withContext(Dispatchers.IO) {
        active = conversation
        store.markRead(conversation.botId, conversation.peerId)
        _messages.value = store.messages(conversation.botId, conversation.peerId)
        _conversations.value = store.conversations()
        loadMedia(conversation.botId, conversation.peerId)
    }

    fun closeChat() {
        active = null
    }

    /** 处理服务端事件；只有 msg.inbound 会落库。 */
    suspend fun handleEvent(envelope: JSONObject) {
        if (envelope.optString("type") != "msg.inbound") return
        val payload = envelope.optJSONObject("payload") ?: return
        val botId = payload.optString("bot_id")
        val peerId = payload.optString("peer_id")
        if (botId.isEmpty() || peerId.isEmpty()) return

        val activeNow = active?.let { it.botId == botId && it.peerId == peerId } ?: false
        val plain = payload.optJSONObject("body")?.optString("plain") ?: ""
        val peerKind = payload.optString("peer_kind", "private")
        val senderName = payload.optString("sender_name")
        withContext(Dispatchers.IO) {
            store.insertInbound(
                botId = botId,
                peerId = peerId,
                peerKind = peerKind,
                msgId = payload.optString("msg_id"),
                senderName = senderName,
                plain = plain,
                kind = payload.optString("msg_type", "text"),
                ts = payload.optLong("ts", System.currentTimeMillis()),
                active = activeNow,
            )
            _conversations.value = store.conversations()
            if (activeNow) {
                _messages.value = store.messages(botId, peerId)
                loadMedia(botId, peerId)
            }
        }

        // D9：消息一律入库；只有「当前活跃 Bot + App 不在前台」才发通知。
        if (botId == repository.activeBotId() && !isAppForeground()) {
            val title = if (peerKind == "group") {
                "群聊 " + peerId.takeLast(6)
            } else {
                senderName.ifBlank { "私聊 " + peerId.takeLast(6) }
            }
            notifier.notify(title, plain, botId, peerId)
        }
    }

    /** App 是否在前台（ProcessLifecycleOwner 的 STARTED 及以上）。 */
    private fun isAppForeground(): Boolean =
        ProcessLifecycleOwner.get().lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)

    /**
     * 断线重连后的游标补拉：每个会话取 (last_ts, last_msg_id) 之后最多 100 条。
     *
     * 只拉一页；更长的历史留到打开会话时再翻（msg.list 的 before 已支持）。
     */
    suspend fun backfill() = withContext(Dispatchers.IO) {
        val cursors = store.conversationsForBackfill()
        for (cursor in cursors) {
            var after = JSONObject()
                .put("ts", cursor.lastTs)
                .put("msg_id", cursor.lastMsgId)
            var pages = 0
            try {
                while (pages < MAX_BACKFILL_PAGES) {
                    val payload = repository.listMessages(
                        botId = cursor.botId,
                        peerId = cursor.peerId,
                        after = after,
                        limit = BACKFILL_PAGE_SIZE,
                    )
                    val array = payload.optJSONArray("messages") ?: break
                    if (array.length() == 0) break
                    var lastTs = after.optLong("ts")
                    var lastMsgId = after.optString("msg_id")
                    for (i in 0 until array.length()) {
                        val item = array.optJSONObject(i) ?: continue
                        val msgId = item.optString("msg_id")
                        if (msgId.isEmpty()) continue
                        store.insertRemote(
                            botId = cursor.botId,
                            peerId = cursor.peerId,
                            msgId = msgId,
                            direction = item.optString("direction", "in").ifEmpty { "in" },
                            senderName = item.optString("sender_name"),
                            plain = item.optJSONObject("body")?.optString("plain") ?: "",
                            kind = item.optString("msg_type", "text"),
                            ts = item.optLong("ts", System.currentTimeMillis()),
                        )
                        lastTs = item.optLong("ts", lastTs)
                        lastMsgId = msgId
                    }
                    if (!payload.optBoolean("has_more", false) || lastMsgId.isEmpty()) break
                    after = JSONObject().put("ts", lastTs).put("msg_id", lastMsgId)
                    pages++
                }
            } catch (e: Exception) {
                _error.value = "补拉失败：" + (e.message ?: e::class.java.simpleName)
            }
        }
        _conversations.value = store.conversations()
        active?.let { _messages.value = store.messages(it.botId, it.peerId) }
    }

    /** 发送一条回复；QQ 只能被动回复，所以带上最近一条入站消息的 id。 */
    suspend fun send(text: String): Boolean {
        val conversation = active ?: return false
        if (text.isBlank()) return false
        return withContext(Dispatchers.IO) {
            try {
                val replyTo = store.lastInboundMsgId(conversation.botId, conversation.peerId)
                val payload = repository.sendText(
                    botId = conversation.botId,
                    peerId = conversation.peerId,
                    text = text,
                    inReplyTo = replyTo,
                )
                store.insertOutbound(
                    botId = conversation.botId,
                    peerId = conversation.peerId,
                    msgId = payload.optString("msg_id"),
                    plain = text,
                    ts = payload.optLong("ts", System.currentTimeMillis()),
                )
                _messages.value = store.messages(conversation.botId, conversation.peerId)
                _conversations.value = store.conversations()
                true
            } catch (e: Exception) {
                _error.value = e.message ?: e::class.java.simpleName
                false
            }
        }
    }

    /** 逐条拉图片；失败不报错，保持占位（服务端会区分 expired/evicted）。 */
    private suspend fun loadMedia(botId: String, peerId: String) {
        val images = store.messages(botId, peerId).filter { it.kind == "image" }
        for (message in images) {
            if (_media.value.containsKey(message.msgId)) continue
            try {
                val bytes = repository.fetchMedia(botId, message.msgId)
                if (bytes.size > MAX_IMAGE_BYTES) continue
                // 气泡只有 280dp 宽，没必要把 4000px 的原图整张解进内存。
                val bitmap = decodeScaled(bytes) ?: continue
                _media.value = capMedia(_media.value + (message.msgId to bitmap))
                _mediaErrors.value = _mediaErrors.value - message.msgId
            } catch (e: Exception) {
                val code = (e as? ProtocolException)?.code ?: "failed"
                _mediaErrors.value = _mediaErrors.value + (message.msgId to code)
            }
        }
    }

    /** 先读尺寸再按 2 的幂降采样，目标长边约 1080px。 */
    private fun decodeScaled(bytes: ByteArray): Bitmap? {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bounds)
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null
        var sample = 1
        var width = bounds.outWidth
        var height = bounds.outHeight
        while (width / 2 >= TARGET_IMAGE_EDGE && height / 2 >= TARGET_IMAGE_EDGE) {
            width /= 2
            height /= 2
            sample *= 2
        }
        val options = BitmapFactory.Options().apply { inSampleSize = sample }
        return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, options)
    }

    /** 简单 LRU-ish 上限：超出就丢掉最早插入的几张，避免多会话累积吃内存。 */
    private fun capMedia(current: Map<String, Bitmap>): Map<String, Bitmap> {
        if (current.size <= MAX_CACHED_BITMAPS) return current
        val keep = current.entries.drop(current.size - MAX_CACHED_BITMAPS)
        return keep.associate { it.key to it.value }
    }

    fun clearError() {
        _error.value = null
    }
}

private const val BACKFILL_PAGE_SIZE = 100
private const val MAX_BACKFILL_PAGES = 20

private const val MAX_IMAGE_BYTES = 5 * 1024 * 1024
private const val TARGET_IMAGE_EDGE = 1080
private const val MAX_CACHED_BITMAPS = 60

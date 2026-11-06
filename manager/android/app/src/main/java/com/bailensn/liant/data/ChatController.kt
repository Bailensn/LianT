package com.bailensn.liant.data

import android.content.Context
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

    private val _conversations = MutableStateFlow<List<Conversation>>(emptyList())
    val conversations: StateFlow<List<Conversation>> = _conversations.asStateFlow()

    private val _messages = MutableStateFlow<List<ChatMessage>>(emptyList())
    val messages: StateFlow<List<ChatMessage>> = _messages.asStateFlow()

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    private var active: Conversation? = null

    suspend fun refresh() = withContext(Dispatchers.IO) {
        _conversations.value = store.conversations()
    }

    suspend fun open(conversation: Conversation) = withContext(Dispatchers.IO) {
        active = conversation
        store.markRead(conversation.botId, conversation.peerId)
        _messages.value = store.messages(conversation.botId, conversation.peerId)
        _conversations.value = store.conversations()
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
        withContext(Dispatchers.IO) {
            store.insertInbound(
                botId = botId,
                peerId = peerId,
                peerKind = payload.optString("peer_kind", "private"),
                msgId = payload.optString("msg_id"),
                senderName = payload.optString("sender_name"),
                plain = plain,
                ts = payload.optLong("ts", System.currentTimeMillis()),
                active = activeNow,
            )
            _conversations.value = store.conversations()
            if (activeNow) {
                _messages.value = store.messages(botId, peerId)
            }
        }
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

    fun clearError() {
        _error.value = null
    }
}

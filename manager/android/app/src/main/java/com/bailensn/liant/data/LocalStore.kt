package com.bailensn.liant.data

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper

/** 会话列表里的一行。 */
data class Conversation(
    val botId: String,
    val peerId: String,
    val peerKind: String,
    val title: String,
    val lastPlain: String,
    val lastTs: Long,
    val unread: Int,
)

/** 气泡里的一条消息。 */
data class ChatMessage(
    val msgId: String,
    val direction: String,
    val senderName: String,
    val plain: String,
    val ts: Long,
) {
    val outgoing: Boolean get() = direction == "out"
}

/**
 * 本地消息库（sqlite，无第三方依赖）。
 *
 * D2：本地不设保留上限，消息进设备就永久保留；清理由用户触发。
 * 主键用 (bot_id, peer_id, msg_id)，入站重推不会产生重复。
 */
class LocalStore(context: Context) :
    SQLiteOpenHelper(context.applicationContext, DB_NAME, null, DB_VERSION) {

    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL(
            """
            CREATE TABLE conversations (
                bot_id TEXT NOT NULL,
                peer_id TEXT NOT NULL,
                peer_kind TEXT NOT NULL,
                title TEXT NOT NULL,
                last_plain TEXT NOT NULL,
                last_ts INTEGER NOT NULL,
                unread INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY (bot_id, peer_id)
            )
            """.trimIndent(),
        )
        db.execSQL(
            """
            CREATE TABLE messages (
                bot_id TEXT NOT NULL,
                peer_id TEXT NOT NULL,
                msg_id TEXT NOT NULL,
                direction TEXT NOT NULL,
                sender_name TEXT NOT NULL,
                plain TEXT NOT NULL,
                ts INTEGER NOT NULL,
                in_reply_to TEXT NOT NULL DEFAULT '',
                PRIMARY KEY (bot_id, peer_id, msg_id)
            )
            """.trimIndent(),
        )
        db.execSQL(
            "CREATE INDEX messages_peer_ts ON messages (bot_id, peer_id, ts)",
        )
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
        // v1 只有一张初始 schema；后续版本在这里迁移。
    }

    /** 入站消息落库；返回是否为新消息，并更新会话摘要。 */
    fun insertInbound(
        botId: String,
        peerId: String,
        peerKind: String,
        msgId: String,
        senderName: String,
        plain: String,
        ts: Long,
        active: Boolean,
    ): Boolean {
        val db = writableDatabase
        db.beginTransaction()
        try {
            val inserted = db.insertWithOnConflict(
                "messages",
                null,
                ContentValues().apply {
                    put("bot_id", botId)
                    put("peer_id", peerId)
                    put("msg_id", msgId)
                    put("direction", "in")
                    put("sender_name", senderName)
                    put("plain", plain)
                    put("ts", ts)
                },
                SQLiteDatabase.CONFLICT_IGNORE,
            ) != -1L
            if (inserted) {
                val title = if (peerKind == "group") {
                    "群聊 " + peerId.takeLast(6)
                } else {
                    senderName.ifBlank { "私聊 " + peerId.takeLast(6) }
                }
                db.execSQL(
                    """
                    INSERT INTO conversations (bot_id, peer_id, peer_kind, title, last_plain, last_ts, unread)
                    VALUES (?, ?, ?, ?, ?, ?, ?)
                    ON CONFLICT(bot_id, peer_id) DO UPDATE SET
                        peer_kind = excluded.peer_kind,
                        last_plain = excluded.last_plain,
                        last_ts = excluded.last_ts,
                        unread = conversations.unread + excluded.unread
                    """.trimIndent(),
                    arrayOf<Any?>(botId, peerId, peerKind, title, plain, ts, if (active) 0 else 1),
                )
            }
            db.setTransactionSuccessful()
            return inserted
        } finally {
            db.endTransaction()
        }
    }

    /** 自己发出的消息落库（服务端返回的 msg_id/ts）。 */
    fun insertOutbound(botId: String, peerId: String, msgId: String, plain: String, ts: Long) {
        val db = writableDatabase
        db.beginTransaction()
        try {
            db.insertWithOnConflict(
                "messages",
                null,
                ContentValues().apply {
                    put("bot_id", botId)
                    put("peer_id", peerId)
                    put("msg_id", msgId)
                    put("direction", "out")
                    put("sender_name", "")
                    put("plain", plain)
                    put("ts", ts)
                },
                SQLiteDatabase.CONFLICT_IGNORE,
            )
            db.execSQL(
                "UPDATE conversations SET last_plain = ?, last_ts = ? WHERE bot_id = ? AND peer_id = ?",
                arrayOf<Any?>(plain, ts, botId, peerId),
            )
            db.setTransactionSuccessful()
        } finally {
            db.endTransaction()
        }
    }

    fun conversations(): List<Conversation> {
        val out = mutableListOf<Conversation>()
        readableDatabase.query(
            "conversations",
            arrayOf("bot_id", "peer_id", "peer_kind", "title", "last_plain", "last_ts", "unread"),
            null, null, null, null, "last_ts DESC",
        ).use { cursor ->
            while (cursor.moveToNext()) {
                out += Conversation(
                    botId = cursor.getString(0),
                    peerId = cursor.getString(1),
                    peerKind = cursor.getString(2),
                    title = cursor.getString(3),
                    lastPlain = cursor.getString(4),
                    lastTs = cursor.getLong(5),
                    unread = cursor.getInt(6),
                )
            }
        }
        return out
    }

    fun messages(botId: String, peerId: String, limit: Int = 200): List<ChatMessage> {
        val out = mutableListOf<ChatMessage>()
        readableDatabase.query(
            "messages",
            arrayOf("msg_id", "direction", "sender_name", "plain", "ts"),
            "bot_id = ? AND peer_id = ?",
            arrayOf(botId, peerId),
            null, null, "ts ASC", limit.toString(),
        ).use { cursor ->
            while (cursor.moveToNext()) {
                out += ChatMessage(
                    msgId = cursor.getString(0),
                    direction = cursor.getString(1),
                    senderName = cursor.getString(2),
                    plain = cursor.getString(3),
                    ts = cursor.getLong(4),
                )
            }
        }
        return out
    }

    fun markRead(botId: String, peerId: String) {
        writableDatabase.execSQL(
            "UPDATE conversations SET unread = 0 WHERE bot_id = ? AND peer_id = ?",
            arrayOf(botId, peerId),
        )
    }

    /** 最近一条入站消息的 id，用作 QQ 被动回复的 in_reply_to。 */
    fun lastInboundMsgId(botId: String, peerId: String): String? {
        readableDatabase.query(
            "messages",
            arrayOf("msg_id"),
            "bot_id = ? AND peer_id = ? AND direction = 'in'",
            arrayOf(botId, peerId),
            null, null, "ts DESC", "1",
        ).use { cursor ->
            return if (cursor.moveToFirst()) cursor.getString(0) else null
        }
    }

    private companion object {
        const val DB_NAME = "liant.db"
        const val DB_VERSION = 1
    }
}

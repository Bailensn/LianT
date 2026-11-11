package com.bailensn.liant.core.bridge

import android.content.Context
import android.util.Log
import org.json.JSONObject
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.IOException
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.net.InetSocketAddress
import java.net.Socket
import java.util.concurrent.ArrayBlockingQueue
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * UI 进程侧的桥接客户端（原生版，替代 Dart 的 `BridgeMessageClient`）。
 *
 * 只做一件事：连 127.0.0.1 上 `:background` 的 BridgeServer，按 §4.6 的帧协议
 * 收状态/事件、发请求。连接信息从 `liant_bridge.json` 读（端口 + 本次令牌）。
 */
class BridgeClient private constructor(
    private val socket: Socket,
    private val listener: Listener,
) {
    interface Listener {
        fun onState(state: String, detail: String)

        fun onEvent(envelope: JSONObject)

        fun onClosed()
    }

    class ProtocolException(
        val code: String,
        message: String,
        val retryable: Boolean = false,
    ) : Exception(message)

    private val writer = BufferedWriter(OutputStreamWriter(socket.getOutputStream(), Charsets.UTF_8))
    private val pending = ConcurrentHashMap<String, ArrayBlockingQueue<Any>>()
    private val writeLock = Any()
    private val ready = CountDownLatch(1)

    @Volatile
    private var closed = false

    private var seq = 0

    val isClosed: Boolean get() = closed

    fun start() {
        Thread({ readLoop() }, "liant-bridge-client").apply {
            isDaemon = true
            start()
        }
    }

    fun awaitReady(timeoutMs: Long): Boolean =
        ready.await(timeoutMs, TimeUnit.MILLISECONDS) && !closed

    /** 同步请求；超时/错误抛 [ProtocolException]。必须在非主线程调用。 */
    fun request(type: String, payload: JSONObject, timeoutMs: Long = 15_000): JSONObject {
        if (closed) throw ProtocolException("offline", "后台连接已断开", true)
        val id = "native-${seq++}"
        val queue = ArrayBlockingQueue<Any>(1)
        pending[id] = queue
        writeFrame(
            JSONObject()
                .put("t", BridgeProtocol.FRAME_REQUEST)
                .put("id", id)
                .put("method", type)
                .put("payload", payload),
        )
        val result = try {
            queue.poll(timeoutMs, TimeUnit.MILLISECONDS)
        } catch (_: InterruptedException) {
            null
        }
        pending.remove(id)
        return when (result) {
            null -> throw ProtocolException("timeout", "请求超时", true)
            is ProtocolException -> throw result
            else -> result as JSONObject
        }
    }

    fun close() {
        closed = true
        ready.countDown()
        runCatching { writer.close() }
        runCatching { socket.close() }
    }

    private fun writeFrame(frame: JSONObject) {
        synchronized(writeLock) {
            try {
                writer.write(BridgeProtocol.encode(frame))
                writer.write("\n")
                writer.flush()
            } catch (_: IOException) {
                close()
            }
        }
    }

    private fun readLoop() {
        try {
            val reader = BufferedReader(InputStreamReader(socket.getInputStream(), Charsets.UTF_8))
            while (!closed) {
                val line = reader.readLine() ?: break
                val frame = BridgeProtocol.decode(line) ?: continue
                when (frame.optString("t")) {
                    BridgeProtocol.FRAME_READY -> ready.countDown()
                    BridgeProtocol.FRAME_STATE -> listener.onState(
                        frame.optString("state"),
                        frame.optString("detail"),
                    )
                    BridgeProtocol.FRAME_EVENT -> frame.optJSONObject("env")?.let(listener::onEvent)
                    BridgeProtocol.FRAME_RESPONSE -> deliver(frame)
                }
            }
        } catch (_: IOException) {
            // 对端断开，正常。
        } finally {
            closed = true
            ready.countDown()
            val error = ProtocolException("offline", "后台连接已断开", true)
            pending.values.forEach { it.offer(error) }
            pending.clear()
            listener.onClosed()
        }
    }

    private fun deliver(frame: JSONObject) {
        val queue = pending.remove(frame.optString("id")) ?: return
        if (frame.optBoolean("ok", false)) {
            queue.offer(frame.optJSONObject("payload") ?: JSONObject())
        } else {
            queue.offer(
                ProtocolException(
                    code = frame.optString("code", "internal"),
                    message = frame.optString("message"),
                    retryable = frame.optBoolean("retryable", false),
                ),
            )
        }
    }

    companion object {
        private const val TAG = "LianT"
        private const val LOOPBACK = "127.0.0.1"

        /** 连不上返回 null（后台没起来/端口已换），调用方按「后台未就绪」处理。 */
        fun connect(
            context: Context,
            listener: Listener,
            timeoutMs: Int = 3_000,
        ): BridgeClient? {
            val info = BridgeInfo.read(context) ?: return null
            val socket = Socket()
            try {
                socket.connect(InetSocketAddress(LOOPBACK, info.port), timeoutMs)
                socket.soTimeout = 0
            } catch (error: IOException) {
                runCatching { socket.close() }
                Log.i(TAG, "桥接连接失败：${error.message}")
                return null
            }
            val client = try {
                BridgeClient(socket, listener)
            } catch (_: IOException) {
                runCatching { socket.close() }
                return null
            }
            client.start()
            client.writeFrame(
                JSONObject()
                    .put(BridgeProtocol.FRAME_HELLO, info.token)
                    .put("v", BridgeProtocol.VERSION),
            )
            if (!client.awaitReady(timeoutMs.toLong())) {
                client.close()
                return null
            }
            return client
        }
    }
}

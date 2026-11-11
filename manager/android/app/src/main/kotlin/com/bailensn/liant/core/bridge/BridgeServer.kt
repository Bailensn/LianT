package com.bailensn.liant.core.bridge

import android.util.Base64
import android.util.Log
import com.bailensn.liant.core.net.LiantSocket
import org.json.JSONObject
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.IOException
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.security.SecureRandom

/**
 * 后台进程里的本地桥接服务。
 *
 * 真正的 WebSocket 在 `:background` 进程里，这里把它收到的状态与事件实时转给
 * UI 进程，并把 UI 的请求/发送转发给服务端。
 *
 * 安全：只绑 127.0.0.1；握手必须带本次启动随机生成的令牌（写在应用私有目录，
 * 别的 App 读不到）；端口每次启动都换。
 */
class BridgeServer(
    /** 当前在用的客户端；身份变化时会换实例，所以传 getter 而不是实例。 */
    private val clientOf: () -> LiantSocket?,
    /** 当前连接状态；UI 刚接上时先补一条，界面才不会停在「连接中」。 */
    private val connectionOf: () -> Pair<LiantSocket.State, String>,
) {
    private val lock = Any()

    private var serverSocket: ServerSocket? = null
    private var token: String = ""
    private var peer: Socket? = null
    private var writer: BufferedWriter? = null
    private var peerReady = false
    private var running = false

    private var acceptThread: Thread? = null
    private var readThread: Thread? = null

    val port: Int? get() = serverSocket?.localPort

    val currentToken: String get() = token

    /** UI 进程是否接着桥接。断开说明 UI 进程已经没了。 */
    val hasPeer: Boolean
        get() = synchronized(lock) { peerReady && peer != null }

    fun start() {
        val server = try {
            ServerSocket(0, 1, InetAddress.getByName(LOOPBACK))
        } catch (error: IOException) {
            Log.w(TAG, "桥接监听失败", error)
            return
        }
        token = newToken()
        serverSocket = server
        running = true
        acceptThread = Thread({ acceptLoop(server) }, "liant-bridge").apply {
            isDaemon = true
            start()
        }
    }

    fun stop() {
        running = false
        synchronized(lock) { closePeerLocked() }
        try {
            serverSocket?.close()
        } catch (_: IOException) {
        }
        serverSocket = null
        acceptThread?.interrupt()
        readThread?.interrupt()
        acceptThread = null
        readThread = null
    }

    fun broadcastState(state: LiantSocket.State, detail: String) {
        write(
            JSONObject()
                .put("t", BridgeProtocol.FRAME_STATE)
                .put("state", state.wire)
                .put("detail", detail),
        )
    }

    fun broadcastEvent(envelope: JSONObject) {
        write(
            JSONObject()
                .put("t", BridgeProtocol.FRAME_EVENT)
                .put("env", envelope),
        )
    }

    // -------------------------------------------------------------------------
    // 连接
    // -------------------------------------------------------------------------

    private fun acceptLoop(server: ServerSocket) {
        while (running) {
            val socket = try {
                server.accept()
            } catch (_: IOException) {
                break
            }
            attach(socket)
        }
    }

    private fun attach(socket: Socket) {
        synchronized(lock) {
            // 只保留最新一条：UI 进程崩溃后旧连接可能还挂着。
            closePeerLocked()
            peer = socket
            peerReady = false
        }
        readThread = Thread({ readLoop(socket) }, "liant-bridge-peer").apply {
            isDaemon = true
            start()
        }
    }

    private fun readLoop(socket: Socket) {
        try {
            socket.soTimeout = HANDSHAKE_TIMEOUT_MS
            val reader = BufferedReader(InputStreamReader(socket.getInputStream(), Charsets.UTF_8))
            val out = BufferedWriter(OutputStreamWriter(socket.getOutputStream(), Charsets.UTF_8))

            val first = reader.readLine() ?: return
            val hello = BridgeProtocol.decode(first) ?: return
            if (hello.optString(BridgeProtocol.FRAME_HELLO) != token) return

            synchronized(lock) {
                if (peer !== socket || !running) return
                writer = out
                peerReady = true
            }
            socket.soTimeout = 0

            write(JSONObject().put("t", BridgeProtocol.FRAME_READY).put("v", BridgeProtocol.VERSION))
            val (state, detail) = connectionOf()
            broadcastState(state, detail)

            while (running) {
                val line = reader.readLine() ?: break
                val frame = BridgeProtocol.decode(line) ?: break
                handleFrame(frame)
            }
        } catch (_: IOException) {
            // 对端断开，正常。
        } finally {
            drop(socket)
        }
    }

    private fun handleFrame(frame: JSONObject) {
        when (frame.optString("t")) {
            BridgeProtocol.FRAME_BYE -> {
                val current = synchronized(lock) { peer }
                if (current != null) drop(current)
            }
            BridgeProtocol.FRAME_REQUEST, BridgeProtocol.FRAME_SEND -> {
                val id = frame.optString("id")
                val method = frame.optString("method")
                if (id.isEmpty() || method.isEmpty()) return
                val payload = frame.optJSONObject("payload") ?: JSONObject()
                val client = clientOf()
                if (client == null) {
                    writeError(id, "offline", "后台连接未就绪")
                    return
                }
                if (frame.optString("t") == BridgeProtocol.FRAME_SEND) {
                    sendOnce(client, id, method, payload)
                } else {
                    request(client, id, method, payload)
                }
            }
        }
    }

    private fun sendOnce(client: LiantSocket, id: String, method: String, payload: JSONObject) {
        try {
            val envelopeId = client.send(method, payload)
            write(
                JSONObject()
                    .put("t", BridgeProtocol.FRAME_RESPONSE)
                    .put("id", id)
                    .put("ok", true)
                    .put("payload", JSONObject().put("id", envelopeId)),
            )
        } catch (error: LiantSocket.ProtocolException) {
            writeError(id, error.code, error.message ?: "")
        } catch (error: Exception) {
            writeError(id, "internal", error.toString())
        }
    }

    private fun request(client: LiantSocket, id: String, method: String, payload: JSONObject) {
        client.request(method, payload) { result ->
            result.onSuccess { response ->
                write(
                    JSONObject()
                        .put("t", BridgeProtocol.FRAME_RESPONSE)
                        .put("id", id)
                        .put("ok", true)
                        .put("payload", response),
                )
            }.onFailure { error ->
                if (error is LiantSocket.ProtocolException) {
                    write(
                        JSONObject()
                            .put("t", BridgeProtocol.FRAME_RESPONSE)
                            .put("id", id)
                            .put("ok", false)
                            .put("code", error.code)
                            .put("message", error.message ?: "")
                            .put("retryable", error.retryable),
                    )
                } else {
                    writeError(id, "internal", error.toString())
                }
            }
        }
    }

    private fun writeError(id: String, code: String, message: String) {
        write(
            JSONObject()
                .put("t", BridgeProtocol.FRAME_RESPONSE)
                .put("id", id)
                .put("ok", false)
                .put("code", code)
                .put("message", message),
        )
    }

    private fun write(frame: JSONObject) {
        synchronized(lock) {
            val out = writer ?: return
            if (!peerReady) return
            try {
                out.write(BridgeProtocol.encode(frame))
                out.write("\n")
                out.flush()
            } catch (_: IOException) {
                // 对端已经断了，交给 readLoop 清理。
            }
        }
    }

    private fun drop(socket: Socket) {
        synchronized(lock) {
            if (peer !== socket) return
            closePeerLocked()
        }
    }

    private fun closePeerLocked() {
        peerReady = false
        try {
            writer?.close()
        } catch (_: IOException) {
        }
        writer = null
        try {
            peer?.close()
        } catch (_: IOException) {
        }
        peer = null
    }

    private fun newToken(): String {
        val bytes = ByteArray(24)
        SecureRandom().nextBytes(bytes)
        return Base64.encodeToString(
            bytes,
            Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING,
        )
    }

    private companion object {
        const val TAG = "LianT"
        const val LOOPBACK = "127.0.0.1"
        const val HANDSHAKE_TIMEOUT_MS = 3_000
    }
}

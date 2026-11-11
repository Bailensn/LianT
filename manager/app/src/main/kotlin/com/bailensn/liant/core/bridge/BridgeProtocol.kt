package com.bailensn.liant.core.bridge

import org.json.JSONException
import org.json.JSONObject

/**
 * 后台进程 ↔ UI 进程的本地桥接协议（JSON 行 / TCP 127.0.0.1）。
 *
 * 与 Dart 的 `core/background/bridge_protocol.dart` 和 `bridge_client.dart`
 * 逐字段一致；改字段必须两边同时改，错一个 UI 就整条连不上。
 *
 * 帧：
 * - `{hello: token, v: 1}` UI→后台，握手；
 * - `{t: "ready", v: 1}` 后台→UI，握手通过；
 * - `{t: "state", state, detail}` 连接状态；
 * - `{t: "ev", env}` 协议事件原样转发（完整信封）；
 * - `{t: "req", id, method, payload}` UI→后台，需要响应；
 * - `{t: "send", id, method, payload}` UI→后台，不需要响应；
 * - `{t: "res", id, ok, payload}` / `{t: "res", id, ok: false, code, message}`；
 * - `{t: "bye"}` UI→后台，主动断开。
 */
object BridgeProtocol {
    const val VERSION = 1

    const val FRAME_HELLO = "hello"
    const val FRAME_READY = "ready"
    const val FRAME_STATE = "state"
    const val FRAME_EVENT = "ev"
    const val FRAME_REQUEST = "req"
    const val FRAME_SEND = "send"
    const val FRAME_RESPONSE = "res"
    const val FRAME_BYE = "bye"

    fun encode(frame: JSONObject): String = frame.toString()

    fun decode(line: String): JSONObject? {
        if (line.isEmpty()) return null
        return try {
            JSONObject(line)
        } catch (_: JSONException) {
            null
        }
    }
}

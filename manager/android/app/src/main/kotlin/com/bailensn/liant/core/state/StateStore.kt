package com.bailensn.liant.core.state

import android.content.Context
import android.util.Base64
import com.bailensn.liant.core.CoreFiles
import org.json.JSONException
import org.json.JSONObject
import java.io.IOException

/**
 * 一台已配对设备：服务地址 + 钉住的服务端指纹 + 私钥。
 *
 * 与 Dart 侧 `core/identity/identity.dart` 的 `Identity` 对应，字段名以共享
 * 状态文件里的 JSON 为准，不要凭记忆改名（改错一个字段后台就连不上）。
 */
class Identity(
    val deviceId: String,
    val serviceName: String,
    val host: String,
    val port: Int,
    val fingerprint: String,
    val secure: Boolean,
    /** P-256 私钥，32 字节大端标量。 */
    val privateKey: ByteArray,
) {
    val wsUrl: String
        get() = "${if (secure) "wss" else "ws"}://$host:$port/v1/ws"

    override fun toString(): String = "Identity($deviceId, $host:$port)"
}

/**
 * 共享状态文件的解析结果。
 *
 * 第一步只用到 [identity]；免打扰、标题、备注提前解析出来，搬家到 Kotlin 的
 * 第二步（BridgeServer + 通知）直接复用。
 */
class BackgroundState(
    val identity: Identity,
    /** UI 是否在前台；为 true 且桥接有对端时后台不弹通知。 */
    val foreground: Boolean,
    /** 会话 key（`botId|peerId`）→ 是否免打扰。 */
    val muted: Map<String, Boolean>,
    /** 会话 key → 服务端给的名称。 */
    val titles: Map<String, String>,
    /** 会话 key → 本地备注。 */
    val remarks: Map<String, String>,
)

/**
 * 读 UI 进程写的共享状态文件。
 *
 * 文件不存在 = 未配对（或刚解除配对），返回 null；
 * 内容损坏、版本不认识、身份字段不完整同样返回 null，调用方按「未配对」处理。
 */
object StateStore {
    /** 状态文件版本；结构变了就加一（与 Dart 侧 `_kStateVersion` 保持一致）。 */
    private const val VERSION = 1

    private const val DEFAULT_PORT = 8443

    fun read(context: Context): BackgroundState? {
        val file = CoreFiles.stateFile(context)
        if (!file.isFile) return null
        val text = try {
            file.readText()
        } catch (_: IOException) {
            return null
        }
        val json = try {
            JSONObject(text)
        } catch (_: JSONException) {
            return null
        }
        if (json.optInt("v", 0) != VERSION) return null
        val rawIdentity = json.optJSONObject("identity") ?: return null
        val identity = parseIdentity(rawIdentity) ?: return null
        return BackgroundState(
            identity = identity,
            foreground = json.optBoolean("foreground", false),
            muted = boolMap(json.optJSONObject("muted")),
            titles = stringMap(json.optJSONObject("titles")),
            remarks = stringMap(json.optJSONObject("remarks")),
        )
    }

    private fun parseIdentity(json: JSONObject): Identity? {
        val host = json.optString("host")
        if (host.isEmpty()) return null
        val privateKey = try {
            Base64.decode(json.optString("private_key"), Base64.NO_WRAP)
        } catch (_: IllegalArgumentException) {
            return null
        }
        if (privateKey.size != 32) return null
        return Identity(
            deviceId = json.optString("device_id"),
            serviceName = json.optString("service_name", "LianT Service"),
            host = host,
            port = json.optInt("port", DEFAULT_PORT),
            fingerprint = json.optString("fingerprint"),
            secure = json.optBoolean("secure", true),
            privateKey = privateKey,
        )
    }

    private fun boolMap(json: JSONObject?): Map<String, Boolean> {
        if (json == null) return emptyMap()
        val out = LinkedHashMap<String, Boolean>()
        val keys = json.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            json.opt(key)?.let { value ->
                if (value is Boolean) out[key] = value
            }
        }
        return out
    }

    private fun stringMap(json: JSONObject?): Map<String, String> {
        if (json == null) return emptyMap()
        val out = LinkedHashMap<String, String>()
        val keys = json.keys()
        while (keys.hasNext()) {
            val key = keys.next()
            val value = json.opt(key)
            if (value is String) out[key] = value
        }
        return out
    }
}

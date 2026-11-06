package com.bailensn.liant.net

import java.security.MessageDigest
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager
import okhttp3.OkHttpClient

/**
 * 证书指纹钉。
 *
 * 服务端是自签证书，客户端不校验域名，只比对「公钥 SHA-256 的 base32」——
 * 与 Service 打印的节点指纹同一套算法（docs/PROTOCOL.md §2/§10）。
 * 不提供「忽略校验」的开关。
 */
object TlsPinning {

    fun client(pinnedFingerprint: String): OkHttpClient {
        val expected = normalize(pinnedFingerprint)
        val trustManager = object : X509TrustManager {
            override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                throw CertificateException("服务端不校验客户端证书")
            }

            override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                val cert = chain?.firstOrNull() ?: throw CertificateException("空的证书链")
                val actual = normalize(fingerprint(cert.publicKey.encoded))
                if (!MessageDigest.isEqual(actual.toByteArray(), expected.toByteArray())) {
                    throw CertificateException("证书指纹与服务端不符")
                }
            }

            override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
        }
        val context = SSLContext.getInstance("TLS")
        context.init(null, arrayOf(trustManager), null)
        return OkHttpClient.Builder()
            .sslSocketFactory(context.socketFactory, trustManager)
            // 信任来自指纹，与主机名无关。
            .hostnameVerifier { _, _ -> true }
            .build()
    }

    /** 去掉分组横线并转大写，便于比较。 */
    fun normalize(fingerprint: String): String = fingerprint.replace("-", "").uppercase()

    /** base32(SHA-256(SPKI DER))，每 4 位一组，与服务端 node.fingerprint 一致。 */
    fun fingerprint(spkiDer: ByteArray): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(spkiDer)
        return base32(digest).chunked(4).joinToString("-")
    }

    private fun base32(data: ByteArray): String {
        val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
        val out = StringBuilder()
        var buffer = 0
        var bits = 0
        for (byte in data) {
            buffer = (buffer shl 8) or (byte.toInt() and 0xFF)
            bits += 8
            while (bits >= 5) {
                out.append(alphabet[(buffer shr (bits - 5)) and 0x1F])
                bits -= 5
            }
        }
        if (bits > 0) {
            out.append(alphabet[(buffer shl (5 - bits)) and 0x1F])
        }
        return out.toString()
    }
}

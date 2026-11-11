package com.bailensn.liant.core.crypto

import java.math.BigInteger
import java.security.AlgorithmParameters
import java.security.GeneralSecurityException
import java.security.KeyFactory
import java.security.MessageDigest
import java.security.PrivateKey
import java.security.PublicKey
import java.security.SecureRandom
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec
import java.security.spec.ECParameterSpec
import java.security.spec.ECPrivateKeySpec
import java.security.spec.X509EncodedKeySpec

/**
 * P-256（secp256r1）上的 ECDSA。
 *
 * 协议 v1 冻结的格式是「ECDSA P-256 + SHA-256，ASN.1 DER 后 base64」，必须与
 * Go 的 ecdsa.VerifyASN1、Dart 侧自实现的 p256.dart 逐字节一致。这里直接用
 * JCA（Android 上是 Conscrypt）：
 *
 * - 私钥是协议里的 32 字节大端标量，用 [ECPrivateKeySpec] 还原成 [PrivateKey]；
 * - `SHA256withECDSA` 的输入输出就是 DER，不需要自己做 ASN.1 编解码；
 * - 服务端公钥是 base64(PKIX DER)，用 [X509EncodedKeySpec] 解析。
 *
 * 本文件刻意不依赖任何 Android API：JVM 单测（P256Test）能直接跑同一份代码。
 */
object P256 {
    /** 曲线名：与 openssl 的 prime256v1 是同一组参数。 */
    private const val CURVE = "secp256r1"

    /** 签名算法：JCA 里 SHA256withECDSA 的签名结果就是 ASN.1 DER。 */
    private const val SIGNATURE_ALGORITHM = "SHA256withECDSA"

    private val params: ECParameterSpec by lazy {
        val parameters = AlgorithmParameters.getInstance("EC")
        parameters.init(ECGenParameterSpec(CURVE))
        parameters.getParameterSpec(ECParameterSpec::class.java)
    }

    private val keyFactory: KeyFactory by lazy { KeyFactory.getInstance("EC") }

    private val random = SecureRandom()

    /**
     * 把 32 字节标量还原成私钥对象。
     *
     * 私钥来自共享状态文件，坏了就说明身份不可用；直接抛异常让调用方断开，
     * 不要默默生成一把新密钥（那会变成另一台设备）。
     */
    fun privateKey(scalar: ByteArray): PrivateKey {
        require(scalar.size == 32) { "私钥必须是 32 字节" }
        val d = BigInteger(1, scalar)
        require(d.signum() > 0 && d < params.order) { "私钥超出 P-256 标量范围" }
        return keyFactory.generatePrivate(ECPrivateKeySpec(d, params))
    }

    /** ECDSA-SHA256 签名，返回 ASN.1 DER。 */
    fun sign(scalar: ByteArray, message: ByteArray): ByteArray {
        val signature = Signature.getInstance(SIGNATURE_ALGORITHM)
        signature.initSign(privateKey(scalar), random)
        signature.update(message)
        return signature.sign()
    }

    /** 校验 DER 签名；格式不对或验不过都返回 false，不抛异常。 */
    fun verify(publicKey: PublicKey, message: ByteArray, signatureDer: ByteArray): Boolean =
        try {
            val signature = Signature.getInstance(SIGNATURE_ALGORITHM)
            signature.initVerify(publicKey)
            signature.update(message)
            signature.verify(signatureDer)
        } catch (_: GeneralSecurityException) {
            false
        } catch (_: IllegalArgumentException) {
            false
        }

    /**
     * 解析 PKIX DER 公钥；只接受 P-256。
     *
     * 服务端要是拿别的曲线来冒充，[X509EncodedKeySpec] 本身也会解析成功，
     * 所以这里必须再比一次曲线参数。
     */
    fun parsePublicKey(spkiDer: ByteArray): PublicKey? =
        try {
            val key = keyFactory.generatePublic(X509EncodedKeySpec(spkiDer))
            if (key is ECPublicKey && isP256(key.params)) key else null
        } catch (_: GeneralSecurityException) {
            null
        } catch (_: IllegalArgumentException) {
            null
        }

    /**
     * base32(SHA-256(spkiDer))，每 4 位一组用连字符分开，与服务端指纹一致。
     *
     * 算法与 Dart 的 p256.dart 完全相同：RFC 4648 base32、无填充、大写。
     */
    fun fingerprint(spkiDer: ByteArray): String =
        base32Groups(MessageDigest.getInstance("SHA-256").digest(spkiDer))

    /**
     * 校验服务端自证：指纹必须与配对时钉住的一致，签名必须由该公钥签出。
     *
     * 被签字节由调用方拼好（`nonce \n 指纹 \n ts`），这里只负责密码学部分。
     * 任何校验失败都抛 [SecurityException]，调用方据此断开，不提供「忽略」开关。
     */
    fun verifyServiceProof(
        servicePubDer: ByteArray,
        expectedFingerprint: String,
        message: ByteArray,
        signatureDer: ByteArray,
    ): PublicKey {
        val actual = fingerprint(servicePubDer)
        if (expectedFingerprint.isNotEmpty() && actual != expectedFingerprint) {
            throw SecurityException("服务端身份指纹不匹配，连接可能被中间人劫持")
        }
        val publicKey = parsePublicKey(servicePubDer)
            ?: throw SecurityException("服务端公钥格式不正确")
        if (!verify(publicKey, message, signatureDer)) {
            throw SecurityException("服务端身份签名无效")
        }
        return publicKey
    }

    // -------------------------------------------------------------------------
    // 编码
    // -------------------------------------------------------------------------

    private fun isP256(other: ECParameterSpec): Boolean {
        val expected = params
        // 只比基点坐标，不依赖 ECPoint.equals 的实现细节。
        return other.curve.field.fieldSize == 256 &&
            other.order == expected.order &&
            other.generator.x == expected.generator.x &&
            other.generator.y == expected.generator.y
    }

    private fun base32Groups(bytes: ByteArray): String {
        val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
        val out = StringBuilder()
        var buffer = 0
        var bits = 0
        for (byte in bytes) {
            // 只用到低 12 位，Int 溢出不影响取位。
            buffer = (buffer shl 8) or (byte.toInt() and 0xff)
            bits += 8
            while (bits >= 5) {
                out.append(alphabet[(buffer shr (bits - 5)) and 31])
                bits -= 5
            }
        }
        if (bits > 0) {
            out.append(alphabet[(buffer shl (5 - bits)) and 31])
        }
        return out.chunked(4).joinToString("-")
    }
}

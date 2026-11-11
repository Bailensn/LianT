package com.bailensn.liant.core.crypto

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.security.KeyPairGenerator
import java.security.interfaces.ECPrivateKey
import java.security.spec.ECGenParameterSpec
import java.util.Base64

/**
 * P-256 与 openssl 的互操作测试。
 *
 * 向量与 Dart 的 `test/p256_test.dart` 完全相同：同一组 openssl 公钥/签名必须
 * 在 Kotlin（JCA）侧也验得过，否则迁移后设备认证会失败。P256.kt 不依赖 Android
 * API，所以这组用例可以跑在普通 JVM 上（CI 的 `testDebugUnitTest`）。
 */
class P256Test {
    // openssl ecparam -name prime256v1 -genkey / dgst -sha256 -sign 生成。
    private val opensslPubDer: ByteArray = Base64.getDecoder().decode(
        "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEOOifwDa/2e9az66MFEL5Vkupoye/" +
            "SWCrvwGvvASDoatsL8JqbQNJTd3nzz8swF8dlg95TzJojOyTmGkoshLCOQ==",
    )

    private val opensslSig: ByteArray = Base64.getDecoder().decode(
        "MEUCIEpXfIFy7LnzSQ4dAyfLsoJ8pMF2+40bqWgY+cSUiKjBAiEAssAxKL8qXY2aQYly" +
            "1R51VVHMPG3kjWXjZwa4j8SWxlg=",
    )

    private val message: ByteArray = "hello liant p256\n".toByteArray(Charsets.UTF_8)

    @Test
    fun opensslSignatureVerifies() {
        val publicKey = P256.parsePublicKey(opensslPubDer)
        assertNotNull(publicKey)
        assertTrue(P256.verify(publicKey!!, message, opensslSig))
        assertFalse(
            P256.verify(publicKey, "被篡改的内容".toByteArray(Charsets.UTF_8), opensslSig),
        )
    }

    @Test
    fun fingerprintHasProtocolShape() {
        val fingerprint = P256.fingerprint(opensslPubDer)
        // 32 字节 → 52 个 base32 字符，每 4 个一组加连字符，与服务端一致。
        assertTrue(
            "指纹格式不对：$fingerprint",
            Regex("^([A-Z2-7]{4}-){12}[A-Z2-7]{4}$").matches(fingerprint),
        )
        assertEquals(64, fingerprint.length)
        assertEquals(fingerprint, P256.fingerprint(opensslPubDer))
    }

    @Test
    fun serviceProofChecksFingerprintAndSignature() {
        val pair = newP256KeyPair()
        val der = pair.public.encoded
        val signed = "nonce\nfp\n1793520000123".toByteArray(Charsets.UTF_8)
        val signature = P256.sign(scalarOf(pair.private as ECPrivateKey), signed)
        val fingerprint = P256.fingerprint(der)

        assertNotNull(P256.verifyServiceProof(der, fingerprint, signed, signature))

        try {
            P256.verifyServiceProof(der, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", signed, signature)
            fail("指纹不一致时必须拒绝连接")
        } catch (error: SecurityException) {
            assertTrue(error.message!!.contains("指纹"))
        }

        try {
            P256.verifyServiceProof(der, fingerprint, "被篡改的内容".toByteArray(), signature)
            fail("消息被篡改时必须拒绝")
        } catch (_: SecurityException) {
            // 预期。
        }
    }

    @Test
    fun generatedSignatureRoundTrips() {
        val pair = newP256KeyPair()
        val scalar = scalarOf(pair.private as ECPrivateKey)
        val signature = P256.sign(scalar, message)

        // JCA 的 ECDSA 签名就是 ASN.1 DER，首字节必须是 SEQUENCE。
        assertEquals(0x30, signature[0].toInt() and 0xff)
        assertTrue(P256.verify(pair.public, message, signature))
        assertFalse(P256.verify(pair.public, message + 0x20.toByte(), signature))
        assertFalse(P256.verify(pair.public, message, byteArrayOf(1, 2, 3)))
    }

    @Test
    fun rejectsOtherCurves() {
        val generator = KeyPairGenerator.getInstance("EC")
        generator.initialize(ECGenParameterSpec("secp384r1"))
        val der = generator.generateKeyPair().public.encoded
        assertNull(P256.parsePublicKey(der))
    }

    @Test
    fun rejectsInvalidPrivateKey() {
        try {
            P256.privateKey(ByteArray(32))
            fail("全零私钥必须被拒绝")
        } catch (_: IllegalArgumentException) {
            // 预期。
        }
        try {
            P256.privateKey(ByteArray(31))
            fail("长度不是 32 字节必须被拒绝")
        } catch (_: IllegalArgumentException) {
            // 预期。
        }
    }

    private fun newP256KeyPair() = KeyPairGenerator.getInstance("EC").run {
        initialize(ECGenParameterSpec("secp256r1"))
        generateKeyPair()
    }

    /** JCA 的 toByteArray 可能带符号位 0x00 或不足 32 字节，统一成定长大端。 */
    private fun scalarOf(key: ECPrivateKey): ByteArray {
        val raw = key.s.toByteArray()
        return when {
            raw.size == 32 -> raw
            raw.size == 33 && raw[0] == 0.toByte() -> raw.copyOfRange(1, raw.size)
            raw.size < 32 -> ByteArray(32).also { raw.copyInto(it, 32 - raw.size) }
            else -> throw AssertionError("标量长度异常：${raw.size}")
        }
    }
}

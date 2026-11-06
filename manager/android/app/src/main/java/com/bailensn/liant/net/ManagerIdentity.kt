package com.bailensn.liant.net

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.PrivateKey
import java.security.Signature
import java.security.spec.ECGenParameterSpec

/**
 * Manager 的设备身份：P-256 私钥放在 Android Keystore，永不可导出。
 *
 * 签名输出是 ASN.1 DER，与服务端 node.Verify 的期望一致；
 * 公钥以 X.509 SubjectPublicKeyInfo DER 的 base64 提交（pair.request）。
 */
class ManagerIdentity(alias: String = DEFAULT_ALIAS) {

    private val alias: String = alias

    init {
        ensureKey()
    }

    private fun keyStore(): KeyStore =
        KeyStore.getInstance(PROVIDER).apply { load(null) }

    private fun ensureKey() {
        val store = keyStore()
        if (store.containsAlias(alias)) return
        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, PROVIDER)
        generator.initialize(
            KeyGenParameterSpec.Builder(
                alias,
                KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY,
            )
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256)
                .build(),
        )
        generator.generateKeyPair()
    }

    /** 公钥 DER 的 base64（NO_WRAP），用于 pair.request.manager_pub。 */
    fun publicKeyBase64(): String {
        val cert = keyStore().getCertificate(alias) ?: error("设备密钥缺失")
        return Base64.encodeToString(cert.publicKey.encoded, Base64.NO_WRAP)
    }

    /** 用设备私钥签名，返回 DER 编码的签名。 */
    fun sign(data: ByteArray): ByteArray {
        val key = keyStore().getKey(alias, null) as? PrivateKey ?: error("设备私钥缺失")
        val signature = Signature.getInstance("SHA256withECDSA")
        signature.initSign(key)
        signature.update(data)
        return signature.sign()
    }

    companion object {
        private const val PROVIDER = "AndroidKeyStore"
        private const val DEFAULT_ALIAS = "liant-manager"
    }
}

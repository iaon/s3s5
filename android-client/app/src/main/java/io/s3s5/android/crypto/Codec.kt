package io.s3s5.android.crypto

import org.json.JSONObject
import java.security.GeneralSecurityException
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

interface Codec {
    fun seal(objectType: String, sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray
    fun open(objectType: String, sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray
    fun enabled(): Boolean
}

object NoopCodec : Codec {
    override fun seal(objectType: String, sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray =
        plaintext.copyOf()

    override fun open(objectType: String, sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray =
        data.copyOf()

    override fun enabled(): Boolean = false
}

class PskCodec(psk: String) : Codec {
    private val pskBytes = psk.toByteArray(Charsets.UTF_8)
    private val random = SecureRandom()
    private val b64 = Base64.getEncoder()
    private val b64d = Base64.getDecoder()

    init {
        require(psk.length >= 16) { "S3S5_PSK must be at least 16 characters" }
    }

    override fun enabled(): Boolean = true

    override fun seal(
        objectType: String,
        sessionId: String,
        direction: String,
        seq: Long,
        plaintext: ByteArray,
    ): ByteArray {
        val nonce = ByteArray(12)
        random.nextBytes(nonce)
        val cipher = newCipher(Cipher.ENCRYPT_MODE, sessionId, direction, nonce)
        cipher.updateAAD(associatedData(objectType, sessionId, direction, seq))
        val ciphertext = cipher.doFinal(plaintext)
        return JSONObject()
            .put("v", 1)
            .put("alg", "AES-256-GCM")
            .put("nonce", b64.encodeToString(nonce))
            .put("ciphertext", b64.encodeToString(ciphertext))
            .toString()
            .toByteArray(Charsets.UTF_8)
    }

    override fun open(
        objectType: String,
        sessionId: String,
        direction: String,
        seq: Long,
        data: ByteArray,
    ): ByteArray {
        val env = JSONObject(String(data, Charsets.UTF_8))
        if (env.getInt("v") != 1 || env.getString("alg") != "AES-256-GCM") {
            throw GeneralSecurityException("unsupported crypto envelope")
        }
        val nonce = b64d.decode(env.getString("nonce"))
        if (nonce.size != 12) {
            throw GeneralSecurityException("invalid crypto nonce size")
        }
        val ciphertext = b64d.decode(env.getString("ciphertext"))
        val cipher = newCipher(Cipher.DECRYPT_MODE, sessionId, direction, nonce)
        cipher.updateAAD(associatedData(objectType, sessionId, direction, seq))
        return cipher.doFinal(ciphertext)
    }

    private fun newCipher(mode: Int, sessionId: String, direction: String, nonce: ByteArray): Cipher {
        val key = deriveKey(sessionId, direction)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(mode, SecretKeySpec(key, "AES"), GCMParameterSpec(128, nonce))
        return cipher
    }

    private fun deriveKey(sessionId: String, direction: String): ByteArray =
        hkdfSha256(
            secret = pskBytes,
            salt = "s3s5/v1/$sessionId".toByteArray(Charsets.UTF_8),
            info = "payload/$direction".toByteArray(Charsets.UTF_8),
            n = 32,
        )

    companion object {
        fun associatedData(objectType: String, sessionId: String, direction: String, seq: Long): ByteArray =
            "s3s5/v1|$objectType|$sessionId|$direction|${"%020d".format(seq)}".toByteArray(Charsets.UTF_8)

        fun hkdfSha256(secret: ByteArray, salt: ByteArray, info: ByteArray, n: Int): ByteArray {
            val prk = hmacSha256(salt, secret)
            val out = ArrayList<Byte>(n)
            var prev = ByteArray(0)
            var counter = 1
            while (out.size < n) {
                val mac = Mac.getInstance("HmacSHA256")
                mac.init(SecretKeySpec(prk, "HmacSHA256"))
                mac.update(prev)
                mac.update(info)
                mac.update(counter.toByte())
                prev = mac.doFinal()
                for (b in prev) {
                    if (out.size < n) {
                        out.add(b)
                    }
                }
                counter++
            }
            return ByteArray(out.size) { out[it] }
        }

        private fun hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
            val mac = Mac.getInstance("HmacSHA256")
            mac.init(SecretKeySpec(key, "HmacSHA256"))
            return mac.doFinal(data)
        }
    }
}

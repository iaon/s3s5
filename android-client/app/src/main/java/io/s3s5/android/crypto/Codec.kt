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
    fun sealData(sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray
    fun openData(sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray
    fun enabled(): Boolean
}

object NoopCodec : Codec {
    override fun seal(objectType: String, sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray =
        plaintext.copyOf()

    override fun open(objectType: String, sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray =
        data.copyOf()

    override fun sealData(sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray =
        plaintext.copyOf()

    override fun openData(sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray =
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
        val nonce = randomNonce()
        val ciphertext = sealRaw(objectType, sessionId, direction, seq, nonce, plaintext)
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
        return openRaw(objectType, sessionId, direction, seq, nonce, ciphertext)
    }

    override fun sealData(sessionId: String, direction: String, seq: Long, plaintext: ByteArray): ByteArray {
        val nonce = randomNonce()
        val ciphertext = sealRaw("data", sessionId, direction, seq, nonce, plaintext)
        require(ciphertext.size <= Int.MAX_VALUE - 24)
        val out = ByteArray(12 + nonce.size + ciphertext.size)
        out[0] = 'S'.code.toByte()
        out[1] = '5'.code.toByte()
        out[2] = 'D'.code.toByte()
        out[3] = '1'.code.toByte()
        out[4] = 1
        out[5] = 1
        out[6] = nonce.size.toByte()
        out[7] = 0
        writeU32(out, 8, ciphertext.size)
        nonce.copyInto(out, 12)
        ciphertext.copyInto(out, 12 + nonce.size)
        return out
    }

    override fun openData(sessionId: String, direction: String, seq: Long, data: ByteArray): ByteArray {
        if (data.size < 24) {
            throw GeneralSecurityException("truncated data crypto envelope")
        }
        if (data[0] != 'S'.code.toByte() || data[1] != '5'.code.toByte() || data[2] != 'D'.code.toByte() || data[3] != '1'.code.toByte()) {
            throw GeneralSecurityException("invalid data crypto envelope magic")
        }
        if (data[4].toInt() != 1 || data[5].toInt() != 1) {
            throw GeneralSecurityException("unsupported data crypto envelope")
        }
        val nonceLen = data[6].toInt() and 0xff
        if (nonceLen != 12) {
            throw GeneralSecurityException("invalid data crypto nonce size")
        }
        val ctLen = readU32(data, 8)
        if (ctLen < 16 || data.size != 12 + nonceLen + ctLen) {
            throw GeneralSecurityException("invalid data crypto envelope length")
        }
        val nonce = data.copyOfRange(12, 12 + nonceLen)
        val ciphertext = data.copyOfRange(12 + nonceLen, data.size)
        return openRaw("data", sessionId, direction, seq, nonce, ciphertext)
    }

    private fun randomNonce(): ByteArray {
        val nonce = ByteArray(12)
        random.nextBytes(nonce)
        return nonce
    }

    private fun sealRaw(objectType: String, sessionId: String, direction: String, seq: Long, nonce: ByteArray, plaintext: ByteArray): ByteArray {
        val cipher = newCipher(Cipher.ENCRYPT_MODE, sessionId, direction, nonce)
        cipher.updateAAD(associatedData(objectType, sessionId, direction, seq))
        return cipher.doFinal(plaintext)
    }

    private fun openRaw(objectType: String, sessionId: String, direction: String, seq: Long, nonce: ByteArray, ciphertext: ByteArray): ByteArray {
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

        private fun writeU32(out: ByteArray, offset: Int, value: Int) {
            out[offset] = ((value ushr 24) and 0xff).toByte()
            out[offset + 1] = ((value ushr 16) and 0xff).toByte()
            out[offset + 2] = ((value ushr 8) and 0xff).toByte()
            out[offset + 3] = (value and 0xff).toByte()
        }

        private fun readU32(data: ByteArray, offset: Int): Int =
            ((data[offset].toInt() and 0xff) shl 24) or
                ((data[offset + 1].toInt() and 0xff) shl 16) or
                ((data[offset + 2].toInt() and 0xff) shl 8) or
                (data[offset + 3].toInt() and 0xff)
    }
}

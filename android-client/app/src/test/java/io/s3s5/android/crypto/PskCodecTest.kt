package io.s3s5.android.crypto

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test
import java.security.GeneralSecurityException

class PskCodecTest {
    private val session = "0123456789abcdef0123456789abcdef"

    @Test
    fun associatedDataMatchesGoFormat() {
        assertEquals(
            "s3s5/v1|data|$session|c2s|00000000000000000042",
            String(PskCodec.associatedData("data", session, "c2s", 42), Charsets.UTF_8),
        )
    }

    @Test
    fun hkdfMatchesRfc5869Sha256CaseOne() {
        val ikm = ByteArray(22) { 0x0b }
        val salt = (0x00..0x0c).map { it.toByte() }.toByteArray()
        val info = (0xf0..0xf9).map { it.toByte() }.toByteArray()
        val okm = PskCodec.hkdfSha256(ikm, salt, info, 42)
        assertEquals(
            "3cb25f25faacd57a90434f64d0362f2a" +
                "2d2d0a90cf1a5a4c5db02d56ecc4c5bf" +
                "34007208d5b887185865",
            okm.joinToString("") { "%02x".format(it.toInt() and 0xff) },
        )
    }

    @Test
    fun encryptDecryptRoundTrip() {
        val codec = PskCodec("test-pre-shared-key")
        val plaintext = "hello over s3".toByteArray()
        val sealed = codec.seal("data", session, "c2s", 3, plaintext)
        assertArrayEquals(plaintext, codec.open("data", session, "c2s", 3, sealed))
    }

    @Test(expected = GeneralSecurityException::class)
    fun wrongAadFails() {
        val codec = PskCodec("test-pre-shared-key")
        val sealed = codec.seal("data", session, "c2s", 3, "hello".toByteArray())
        codec.open("data", session, "c2s", 4, sealed)
    }
}

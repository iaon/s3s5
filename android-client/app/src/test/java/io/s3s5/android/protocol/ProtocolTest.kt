package io.s3s5.android.protocol

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ProtocolTest {
    @Test
    fun keysMatchGoLayout() {
        val session = "0123456789abcdef0123456789abcdef"
        assertEquals("s3s5/v1/open/$session.json", Protocol.openKey("", session))
        assertEquals("p/v1/open-result/$session.json", Protocol.openResultKey("/p/", session))
        assertEquals("p/v1/data/c2s/$session/00000000000000000042.bin", Protocol.dataKey("p", "c2s", session, 42))
        assertEquals("p/v1/ack/s2c/$session.json", Protocol.ackKey("p", "s2c", session))
        assertEquals("p/v1/close/server/$session.json", Protocol.closeKey("p", "server", session))
        assertEquals("p/v1/open/", Protocol.openPrefix("p"))
    }

    @Test
    fun sequenceFormattingMatchesGo() {
        assertEquals("00000000000000000000", Protocol.formatSeq(0))
        assertEquals("00000000000000000007", Protocol.formatSeq(7))
        assertEquals(7, Protocol.parseSeq("s3s5/v1/data/c2s/id/00000000000000000007.bin"))
    }

    @Test
    fun sessionIdsAreThirtyTwoHexChars() {
        val id = Protocol.newSessionId()
        assertTrue(id.matches(Regex("[0-9a-f]{32}")))
    }

    @Test
    fun chunkLimitsAreValidated() {
        Protocol.validateChunkSize(MIN_CHUNK_SIZE)
        Protocol.validateChunkSize(MAX_CHUNK_SIZE)
        assertEquals(4096, Protocol.effectiveSendChunkSize(8192, 4096))
        assertEquals(4096, Protocol.effectiveSendChunkSize(4096, 8192))
    }
}

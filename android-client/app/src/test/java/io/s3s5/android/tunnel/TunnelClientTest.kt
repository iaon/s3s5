package io.s3s5.android.tunnel

import io.s3s5.android.crypto.NoopCodec
import io.s3s5.android.objectstore.MemoryObjectStore
import io.s3s5.android.objectstore.PutOptions
import io.s3s5.android.protocol.OpenResult
import io.s3s5.android.protocol.Protocol
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test

class TunnelClientTest {
    @Test
    fun openResultCanBeDecodedFromStore() = runTest {
        val store = MemoryObjectStore()
        val session = "0123456789abcdef0123456789abcdef"
        val result = OpenResult(sessionId = session, accepted = true, createdAt = "2026-01-01T00:00:00Z")
        store.putObject(
            Protocol.openResultKey("p", session),
            NoopCodec.seal("open-result", session, "control", 0, Protocol.run {
                org.json.JSONObject()
                    .put("version", result.version)
                    .put("session_id", result.sessionId)
                    .put("accepted", result.accepted)
                    .put("created_at", result.createdAt)
                    .toString()
                    .toByteArray()
            }),
            PutOptions(),
        )
        val loaded = Protocol.unmarshalOpenResult(store.getObject(Protocol.openResultKey("p", session)))
        assertEquals(true, loaded.accepted)
    }

    @Test
    fun memoryStoreSupportsClosePollingPattern() = runTest {
        val store = MemoryObjectStore()
        val key = Protocol.closeKey("p", SIDE_SERVER, "s")
        val job = launch {
            delay(10)
            store.putObject(key, byteArrayOf(1))
        }
        job.join()
        assertEquals(1, store.headObject(key).size)
    }
}

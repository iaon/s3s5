package io.s3s5.android.tunnel

import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong

data class TunnelStatsSnapshot(
    val activeSessions: Int,
    val bytesSent: Long,
    val bytesReceived: Long,
    val chunksSent: Long,
    val chunksReceived: Long,
)

class TunnelStats {
    private val activeSessions = AtomicInteger()
    private val bytesSent = AtomicLong()
    private val bytesReceived = AtomicLong()
    private val chunksSent = AtomicLong()
    private val chunksReceived = AtomicLong()

    fun incActive() {
        activeSessions.incrementAndGet()
    }

    fun decActive() {
        activeSessions.decrementAndGet()
    }

    fun incChunksSent(bytes: Int) {
        chunksSent.incrementAndGet()
        bytesSent.addAndGet(bytes.toLong())
    }

    fun incChunksReceived(bytes: Int) {
        chunksReceived.incrementAndGet()
        bytesReceived.addAndGet(bytes.toLong())
    }

    fun snapshot(): TunnelStatsSnapshot = TunnelStatsSnapshot(
        activeSessions = activeSessions.get(),
        bytesSent = bytesSent.get(),
        bytesReceived = bytesReceived.get(),
        chunksSent = chunksSent.get(),
        chunksReceived = chunksReceived.get(),
    )
}

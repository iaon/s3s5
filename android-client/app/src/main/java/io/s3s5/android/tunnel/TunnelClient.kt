package io.s3s5.android.tunnel

import io.s3s5.android.crypto.Codec
import io.s3s5.android.objectstore.ObjectNotFoundException
import io.s3s5.android.objectstore.ObjectStore
import io.s3s5.android.objectstore.PutOptions
import io.s3s5.android.protocol.Ack
import io.s3s5.android.protocol.Close
import io.s3s5.android.protocol.OpenRequest
import io.s3s5.android.protocol.Protocol
import io.s3s5.android.protocol.Target
import io.s3s5.android.socks5.REPLY_HOST_UNREACHABLE
import io.s3s5.android.socks5.REPLY_SUCCEEDED
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import java.io.EOFException
import java.io.InputStream
import java.io.OutputStream
import java.net.Socket
import java.time.Instant

const val DIRECTION_C2S = "c2s"
const val DIRECTION_S2C = "s2c"
const val SIDE_CLIENT = "client"
const val SIDE_SERVER = "server"

data class TunnelConfig(
    val store: ObjectStore,
    val codec: Codec,
    val stats: TunnelStats = TunnelStats(),
    val prefix: String = "s3s5",
    val chunkSize: Int = 64 * 1024,
    val pollMinMillis: Long = 50,
    val pollMaxMillis: Long = 2_000,
    val windowChunks: Long = 16,
    val idleTimeoutMillis: Long = 120_000,
)

class TunnelClient(private val cfg: TunnelConfig) {
    suspend fun handleSocks(
        target: Target,
        socket: Socket,
        reply: suspend (Byte) -> Unit,
    ) {
        val sessionId = Protocol.newSessionId()
        val request = OpenRequest(
            sessionId = sessionId,
            target = target,
            createdAt = Instant.now().toString(),
        )
        try {
            putJson(
                Protocol.openKey(cfg.prefix, sessionId),
                "open",
                sessionId,
                "control",
                Protocol.marshalOpenRequest(request),
            )
            val result = waitJson(
                Protocol.openResultKey(cfg.prefix, sessionId),
                "open-result",
                sessionId,
                "control",
            ) { Protocol.unmarshalOpenResult(it) }
            if (!result.accepted) {
                reply(REPLY_HOST_UNREACHABLE)
                throw IllegalStateException("server rejected target: ${result.error}")
            }
            reply(REPLY_SUCCEEDED)
            cfg.stats.incActive()
            try {
                coroutineScope {
                    val toStore = async {
                        streamToStore(sessionId, DIRECTION_C2S, SIDE_CLIENT, socket.getInputStream())
                    }
                    val fromStore = async {
                        try {
                            streamFromStore(sessionId, DIRECTION_S2C, SIDE_SERVER, socket.getOutputStream())
                        } finally {
                            runCatching { socket.shutdownOutput() }
                        }
                    }
                    val first = runCatching { toStore.await() }.exceptionOrNull()
                    if (first != null && first !is CancellationException) {
                        fromStore.cancel()
                        throw first
                    }
                    fromStore.await()
                }
            } finally {
                cfg.stats.decActive()
            }
        } catch (e: Exception) {
            if (e is CancellationException) {
                throw e
            }
            throw e
        }
    }

    private suspend fun streamToStore(
        sessionId: String,
        direction: String,
        side: String,
        input: InputStream,
    ) = withContext(Dispatchers.IO) {
        val buffer = ByteArray(cfg.chunkSize)
        var seq = 0L
        while (isActive) {
            val n = input.read(buffer)
            if (n == -1) {
                putClose(sessionId, side, "")
                return@withContext
            }
            if (n == 0) {
                continue
            }
            waitWindow(sessionId, direction, seq)
            val plain = buffer.copyOf(n)
            val sealed = cfg.codec.seal("data", sessionId, direction, seq, plain)
            cfg.store.putObject(
                Protocol.dataKey(cfg.prefix, direction, sessionId, seq),
                sealed,
                PutOptions(contentType = "application/octet-stream"),
            )
            cfg.stats.incChunksSent(n)
            seq++
        }
    }

    private suspend fun streamFromStore(
        sessionId: String,
        direction: String,
        peerSide: String,
        output: OutputStream,
    ) = withContext(Dispatchers.IO) {
        var seq = 0L
        var lastAck = 0L
        var currentDelay = cfg.pollMinMillis
        var deadline = System.currentTimeMillis() + cfg.idleTimeoutMillis
        val closeCheckEvery = 4
        var missesSinceCloseCheck = closeCheckEvery
        val ackEvery = ackInterval(cfg.windowChunks)
        while (isActive) {
            val key = Protocol.dataKey(cfg.prefix, direction, sessionId, seq)
            val data = try {
                cfg.store.getObject(key)
            } catch (e: ObjectNotFoundException) {
                null
            }
            if (data != null) {
                val plain = cfg.codec.open("data", sessionId, direction, seq, data)
                output.write(plain)
                output.flush()
                cfg.stats.incChunksReceived(plain.size)
                seq++
                if (seq - lastAck >= ackEvery) {
                    putAck(sessionId, direction, seq)
                    lastAck = seq
                }
                currentDelay = cfg.pollMinMillis
                deadline = System.currentTimeMillis() + cfg.idleTimeoutMillis
                missesSinceCloseCheck = closeCheckEvery
                continue
            }
            missesSinceCloseCheck++
            if (missesSinceCloseCheck >= closeCheckEvery) {
                missesSinceCloseCheck = 0
                if (closeExists(sessionId, peerSide)) {
                    if (seq > lastAck) {
                        putAck(sessionId, direction, seq)
                    }
                    return@withContext
                }
            }
            if (System.currentTimeMillis() > deadline) {
                throw EOFException("idle timeout waiting for $direction seq $seq")
            }
            delay(currentDelay)
            currentDelay = nextDelay(currentDelay)
        }
    }

    private suspend fun putJson(
        key: String,
        type: String,
        sessionId: String,
        direction: String,
        data: ByteArray,
    ) {
        val sealed = cfg.codec.seal(type, sessionId, direction, 0, data)
        cfg.store.putObject(key, sealed, PutOptions(contentType = "application/octet-stream"))
    }

    private suspend fun <T> waitJson(
        key: String,
        type: String,
        sessionId: String,
        direction: String,
        decode: (ByteArray) -> T,
    ): T = withTimeout(cfg.idleTimeoutMillis) {
        var currentDelay = cfg.pollMinMillis
        while (isActive) {
            val data = try {
                cfg.store.getObject(key)
            } catch (e: ObjectNotFoundException) {
                null
            }
            if (data != null) {
                return@withTimeout decode(cfg.codec.open(type, sessionId, direction, 0, data))
            }
            delay(currentDelay)
            currentDelay = nextDelay(currentDelay)
        }
        throw CancellationException("wait cancelled")
    }

    private suspend fun putAck(sessionId: String, direction: String, nextSeq: Long) {
        val ack = Ack(
            sessionId = sessionId,
            direction = direction,
            nextSeq = nextSeq,
            updatedAt = Instant.now().toString(),
        )
        putJson(
            Protocol.ackKey(cfg.prefix, direction, sessionId),
            "ack",
            sessionId,
            direction,
            Protocol.marshalAck(ack),
        )
    }

    private suspend fun getAck(sessionId: String, direction: String): Long =
        try {
            val data = cfg.store.getObject(Protocol.ackKey(cfg.prefix, direction, sessionId))
            Protocol.unmarshalAck(cfg.codec.open("ack", sessionId, direction, 0, data)).nextSeq
        } catch (e: ObjectNotFoundException) {
            0
        }

    private suspend fun waitWindow(sessionId: String, direction: String, seq: Long) {
        if (cfg.windowChunks == 0L || seq < cfg.windowChunks) {
            return
        }
        var currentDelay = cfg.pollMinMillis
        while (true) {
            val next = getAck(sessionId, direction)
            if (seq < next + cfg.windowChunks) {
                return
            }
            delay(currentDelay)
            currentDelay = nextDelay(currentDelay)
        }
    }

    private suspend fun putClose(sessionId: String, side: String, reason: String) {
        val close = Close(
            sessionId = sessionId,
            side = side,
            reason = reason,
            createdAt = Instant.now().toString(),
        )
        putJson(
            Protocol.closeKey(cfg.prefix, side, sessionId),
            "close",
            sessionId,
            side,
            Protocol.marshalClose(close),
        )
    }

    private suspend fun closeExists(sessionId: String, side: String): Boolean =
        try {
            cfg.store.headObject(Protocol.closeKey(cfg.prefix, side, sessionId))
            true
        } catch (e: ObjectNotFoundException) {
            false
        }

    private fun ackInterval(window: Long): Long = if (window <= 2) 1 else window / 2

    private fun nextDelay(current: Long): Long =
        (current * 2).coerceAtMost(cfg.pollMaxMillis).coerceAtLeast(cfg.pollMinMillis)
}

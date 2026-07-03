package io.s3s5.android.socks5

import io.s3s5.android.protocol.AddressType
import io.s3s5.android.protocol.Target
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.EOFException
import java.io.InputStream
import java.io.OutputStream
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.Socket

const val SOCKS_VERSION_5: Byte = 0x05
const val METHOD_NO_AUTH: Byte = 0x00
const val METHOD_NO_ACCEPTABLE: Byte = 0xff.toByte()
const val CMD_CONNECT: Byte = 0x01
const val CMD_BIND: Byte = 0x02
const val CMD_UDP_ASSOCIATE: Byte = 0x03
const val ATYP_IPV4: Byte = 0x01
const val ATYP_DOMAIN: Byte = 0x03
const val ATYP_IPV6: Byte = 0x04
const val REPLY_SUCCEEDED: Byte = 0x00
const val REPLY_GENERAL_FAILURE: Byte = 0x01
const val REPLY_HOST_UNREACHABLE: Byte = 0x04
const val REPLY_COMMAND_NOT_SUPPORTED: Byte = 0x07
const val REPLY_ADDRESS_TYPE_UNSUPPORTED: Byte = 0x08

data class SocksRequest(
    val command: Byte,
    val target: Target,
)

class Socks5Server(
    private val host: String,
    private val port: Int,
    private val scope: CoroutineScope,
    private val handler: suspend (Target, Socket, suspend (Byte) -> Unit) -> Unit,
) {
    private var serverSocket: ServerSocket? = null
    private var acceptJob: Job? = null

    suspend fun start() = withContext(Dispatchers.IO) {
        val socket = ServerSocket()
        socket.reuseAddress = true
        socket.bind(InetSocketAddress(InetAddress.getByName(host), port))
        serverSocket = socket
        acceptJob = scope.launch(Dispatchers.IO) {
            while (isActive && !socket.isClosed) {
                val client = try {
                    socket.accept()
                } catch (e: Exception) {
                    if (socket.isClosed) break else throw e
                }
                launch(Dispatchers.IO) {
                    handleClient(client)
                }
            }
        }
    }

    suspend fun stop() {
        serverSocket?.close()
        acceptJob?.cancelAndJoin()
        serverSocket = null
        acceptJob = null
    }

    private suspend fun handleClient(socket: Socket) {
        socket.use {
            try {
                val input = it.getInputStream()
                val output = it.getOutputStream()
                Socks5Protocol.negotiate(input, output)
                val request = Socks5Protocol.readRequest(input)
                if (request.command != CMD_CONNECT) {
                    Socks5Protocol.writeReply(output, REPLY_COMMAND_NOT_SUPPORTED)
                    return
                }
                var replied = false
                val reply: suspend (Byte) -> Unit = { code ->
                    if (!replied) {
                        replied = true
                        Socks5Protocol.writeReply(output, code)
                    }
                }
                handler(request.target, it, reply)
                if (!replied) {
                    reply(REPLY_GENERAL_FAILURE)
                }
            } catch (_: UnsupportedAddressTypeException) {
                runCatching { Socks5Protocol.writeReply(it.getOutputStream(), REPLY_ADDRESS_TYPE_UNSUPPORTED) }
            } catch (_: Exception) {
                runCatching { Socks5Protocol.writeReply(it.getOutputStream(), REPLY_GENERAL_FAILURE) }
            }
        }
    }
}

object Socks5Protocol {
    fun negotiate(input: InputStream, output: OutputStream) {
        val head = input.readN(2)
        if (head[0] != SOCKS_VERSION_5) {
            throw IllegalArgumentException("unsupported SOCKS version")
        }
        val methods = input.readN(head[1].toInt() and 0xff)
        if (methods.contains(METHOD_NO_AUTH)) {
            output.write(byteArrayOf(SOCKS_VERSION_5, METHOD_NO_AUTH))
            output.flush()
            return
        }
        output.write(byteArrayOf(SOCKS_VERSION_5, METHOD_NO_ACCEPTABLE))
        output.flush()
        throw IllegalArgumentException("no acceptable SOCKS5 auth method")
    }

    fun readRequest(input: InputStream): SocksRequest {
        val head = input.readN(4)
        if (head[0] != SOCKS_VERSION_5 || head[2] != 0.toByte()) {
            throw IllegalArgumentException("invalid SOCKS5 request")
        }
        val command = head[1]
        val atyp = head[3]
        val target = when (atyp) {
            ATYP_IPV4 -> Target(
                type = AddressType.IPV4,
                host = InetAddress.getByAddress(input.readN(4)).hostAddress.orEmpty(),
                port = input.readPort(),
            )
            ATYP_IPV6 -> Target(
                type = AddressType.IPV6,
                host = InetAddress.getByAddress(input.readN(16)).hostAddress.orEmpty(),
                port = input.readPort(),
            )
            ATYP_DOMAIN -> {
                val length = input.read()
                if (length < 0) throw EOFException()
                Target(
                    type = AddressType.DOMAIN,
                    host = String(input.readN(length), Charsets.UTF_8),
                    port = input.readPort(),
                )
            }
            else -> throw UnsupportedAddressTypeException()
        }
        return SocksRequest(command, target)
    }

    fun writeReply(output: OutputStream, code: Byte) {
        output.write(byteArrayOf(SOCKS_VERSION_5, code, 0, ATYP_IPV4, 0, 0, 0, 0, 0, 0))
        output.flush()
    }

    private fun InputStream.readPort(): Int {
        val port = readN(2)
        return ((port[0].toInt() and 0xff) shl 8) or (port[1].toInt() and 0xff)
    }

    private fun InputStream.readN(n: Int): ByteArray {
        val out = ByteArray(n)
        var offset = 0
        while (offset < n) {
            val read = read(out, offset, n - offset)
            if (read < 0) throw EOFException()
            offset += read
        }
        return out
    }
}

class UnsupportedAddressTypeException : Exception()

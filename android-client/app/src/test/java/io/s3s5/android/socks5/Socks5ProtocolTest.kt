package io.s3s5.android.socks5

import io.s3s5.android.protocol.AddressType
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream

class Socks5ProtocolTest {
    @Test
    fun negotiatesNoAuth() {
        val output = ByteArrayOutputStream()
        Socks5Protocol.negotiate(ByteArrayInputStream(byteArrayOf(0x05, 0x01, 0x00)), output)
        assertArrayEquals(byteArrayOf(0x05, 0x00), output.toByteArray())
    }

    @Test
    fun readsDomainConnectRequest() {
        val domain = "example.com".toByteArray()
        val input = ByteArrayInputStream(
            byteArrayOf(0x05, CMD_CONNECT, 0x00, ATYP_DOMAIN, domain.size.toByte()) +
                domain +
                byteArrayOf(0x01, 0xbb.toByte()),
        )
        val request = Socks5Protocol.readRequest(input)
        assertEquals(CMD_CONNECT, request.command)
        assertEquals(AddressType.DOMAIN, request.target.type)
        assertEquals("example.com", request.target.host)
        assertEquals(443, request.target.port)
    }

    @Test
    fun readsIpv4ConnectRequest() {
        val input = ByteArrayInputStream(
            byteArrayOf(0x05, CMD_CONNECT, 0x00, ATYP_IPV4, 127, 0, 0, 1, 0x04, 0x38),
        )
        val request = Socks5Protocol.readRequest(input)
        assertEquals(AddressType.IPV4, request.target.type)
        assertEquals("127.0.0.1", request.target.host)
        assertEquals(1080, request.target.port)
    }

    @Test
    fun replyFormatMatchesGo() {
        val output = ByteArrayOutputStream()
        Socks5Protocol.writeReply(output, REPLY_COMMAND_NOT_SUPPORTED)
        assertArrayEquals(
            byteArrayOf(0x05, REPLY_COMMAND_NOT_SUPPORTED, 0, ATYP_IPV4, 0, 0, 0, 0, 0, 0),
            output.toByteArray(),
        )
    }
}

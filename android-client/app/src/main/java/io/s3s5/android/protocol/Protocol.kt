package io.s3s5.android.protocol

import org.json.JSONObject
import java.security.SecureRandom
import java.time.Instant
import java.util.Locale

const val VERSION = 1

enum class AddressType(val wireName: String) {
    IPV4("ipv4"),
    IPV6("ipv6"),
    DOMAIN("domain"),
}

data class Target(
    val type: AddressType,
    val host: String,
    val port: Int,
) {
    fun address(): String = if (host.contains(":") && !host.startsWith("[")) {
        "[$host]:$port"
    } else {
        "$host:$port"
    }
}

data class OpenRequest(
    val version: Int = VERSION,
    val sessionId: String,
    val target: Target,
    val createdAt: String = Instant.now().toString(),
)

data class OpenResult(
    val version: Int = VERSION,
    val sessionId: String,
    val accepted: Boolean,
    val error: String = "",
    val createdAt: String = Instant.now().toString(),
)

data class Ack(
    val version: Int = VERSION,
    val sessionId: String,
    val direction: String,
    val nextSeq: Long,
    val updatedAt: String = Instant.now().toString(),
)

data class Close(
    val version: Int = VERSION,
    val sessionId: String,
    val side: String,
    val reason: String = "",
    val createdAt: String = Instant.now().toString(),
)

object Protocol {
    private val random = SecureRandom()

    fun newSessionId(): String {
        val bytes = ByteArray(16)
        random.nextBytes(bytes)
        return bytes.joinToString("") { "%02x".format(it.toInt() and 0xff) }
    }

    fun normalizePrefix(prefix: String?): String {
        val trimmed = prefix.orEmpty().trim('/')
        return trimmed.ifEmpty { "s3s5" }
    }

    fun openKey(prefix: String?, sessionId: String): String =
        key(prefix, "v1", "open", "$sessionId.json")

    fun openResultKey(prefix: String?, sessionId: String): String =
        key(prefix, "v1", "open-result", "$sessionId.json")

    fun dataKey(prefix: String?, direction: String, sessionId: String, seq: Long): String =
        key(prefix, "v1", "data", direction, sessionId, "${formatSeq(seq)}.bin")

    fun ackKey(prefix: String?, direction: String, sessionId: String): String =
        key(prefix, "v1", "ack", direction, "$sessionId.json")

    fun closeKey(prefix: String?, side: String, sessionId: String): String =
        key(prefix, "v1", "close", side, "$sessionId.json")

    fun heartbeatKey(prefix: String?, side: String, sessionId: String): String =
        key(prefix, "v1", "heartbeat", side, "$sessionId.json")

    fun openPrefix(prefix: String?): String = key(prefix, "v1", "open") + "/"

    fun formatSeq(seq: Long): String = String.format(Locale.US, "%020d", seq)

    fun parseSeq(keyOrFile: String): Long {
        val base = keyOrFile.substringAfterLast('/').removeSuffix(".bin")
        return base.toLong()
    }

    fun marshalOpenRequest(value: OpenRequest): ByteArray {
        val target = JSONObject()
            .put("type", value.target.type.wireName)
            .put("host", value.target.host)
            .put("port", value.target.port)
        return JSONObject()
            .put("version", value.version)
            .put("session_id", value.sessionId)
            .put("target", target)
            .put("created_at", value.createdAt)
            .toString()
            .toByteArray(Charsets.UTF_8)
    }

    fun unmarshalOpenResult(data: ByteArray): OpenResult {
        val json = JSONObject(String(data, Charsets.UTF_8))
        return OpenResult(
            version = json.getInt("version"),
            sessionId = json.getString("session_id"),
            accepted = json.getBoolean("accepted"),
            error = json.optString("error", ""),
            createdAt = json.getString("created_at"),
        )
    }

    fun marshalAck(value: Ack): ByteArray =
        JSONObject()
            .put("version", value.version)
            .put("session_id", value.sessionId)
            .put("direction", value.direction)
            .put("next_seq", value.nextSeq)
            .put("updated_at", value.updatedAt)
            .toString()
            .toByteArray(Charsets.UTF_8)

    fun unmarshalAck(data: ByteArray): Ack {
        val json = JSONObject(String(data, Charsets.UTF_8))
        return Ack(
            version = json.getInt("version"),
            sessionId = json.getString("session_id"),
            direction = json.getString("direction"),
            nextSeq = json.getLong("next_seq"),
            updatedAt = json.getString("updated_at"),
        )
    }

    fun marshalClose(value: Close): ByteArray {
        val json = JSONObject()
            .put("version", value.version)
            .put("session_id", value.sessionId)
            .put("side", value.side)
            .put("created_at", value.createdAt)
        if (value.reason.isNotEmpty()) {
            json.put("reason", value.reason)
        }
        return json.toString().toByteArray(Charsets.UTF_8)
    }

    private fun key(prefix: String?, vararg parts: String): String =
        (listOf(normalizePrefix(prefix)) + parts.asList())
            .map { it.trim('/') }
            .filter { it.isNotEmpty() }
            .joinToString("/")
}

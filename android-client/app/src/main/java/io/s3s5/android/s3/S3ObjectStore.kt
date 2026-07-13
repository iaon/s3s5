package io.s3s5.android.s3

import io.s3s5.android.objectstore.ListOptions
import io.s3s5.android.objectstore.ListPage
import io.s3s5.android.objectstore.ObjectInfo
import io.s3s5.android.objectstore.ObjectNotFoundException
import io.s3s5.android.objectstore.ObjectStore
import io.s3s5.android.objectstore.PutOptions
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.w3c.dom.Element
import java.io.ByteArrayInputStream
import java.net.URI
import java.net.URLEncoder
import java.security.MessageDigest
import java.time.Duration
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import javax.xml.parsers.DocumentBuilderFactory

class S3ObjectStore(
    config: S3Config,
    private val client: OkHttpClient = OkHttpClient.Builder()
        .callTimeout(Duration.ofSeconds(60))
        .build(),
) : ObjectStore {
    private val cfg = config.withProviderDefaults()

    init {
        require(cfg.bucket.isNotBlank()) { "bucket is required" }
        require(cfg.accessKeyId.isNotBlank() && cfg.secretAccessKey.isNotBlank()) {
            "S3 credentials are required"
        }
    }

    override suspend fun putObject(key: String, data: ByteArray, options: PutOptions): Unit = withContext(Dispatchers.IO) {
        val extraHeaders = buildMap {
            if (options.contentType.isNotBlank()) {
                put("content-type", options.contentType)
            }
            options.metadata.forEach { (k, v) -> put("x-amz-meta-${k.lowercase(Locale.US)}", v) }
        }
        val builder = signedRequestBuilder("PUT", key, emptyMap(), data, extraHeaders)
        val mediaType = options.contentType.takeIf { it.isNotBlank() }?.toMediaTypeOrNull()
        val response = client.newCall(builder.put(data.toRequestBody(mediaType)).build()).execute()
        response.use {
            if (it.code !in listOf(200, 201)) {
                throw statusError(it.code, it.message, it.body?.string())
            }
        }
    }

    override suspend fun getObject(key: String): ByteArray = withContext(Dispatchers.IO) {
        val response = client.newCall(signedRequestBuilder("GET", key).get().build()).execute()
        response.use {
            when (it.code) {
                200 -> it.body?.bytes() ?: ByteArray(0)
                404 -> throw ObjectNotFoundException(key)
                else -> throw statusError(it.code, it.message, it.body?.string())
            }
        }
    }

    override suspend fun headObject(key: String): ObjectInfo = withContext(Dispatchers.IO) {
        val response = client.newCall(signedRequestBuilder("HEAD", key).head().build()).execute()
        response.use {
            when (it.code) {
                200 -> ObjectInfo(key, it.header("Content-Length")?.toLongOrNull() ?: -1)
                404 -> throw ObjectNotFoundException(key)
                else -> throw statusError(it.code, it.message, it.body?.string())
            }
        }
    }

    override suspend fun listPrefix(prefix: String, options: ListOptions): List<String> =
        listPrefixPage(prefix, options).keys

    override suspend fun listPrefixPage(prefix: String, options: ListOptions): ListPage = withContext(Dispatchers.IO) {
        val query = buildMap {
            put("list-type", "2")
            put("prefix", prefix)
            if (options.maxKeys > 0) {
                put("max-keys", options.maxKeys.toString())
            }
            if (options.continuationToken.isNotBlank()) {
                put("continuation-token", options.continuationToken)
            }
        }
        val response = client.newCall(signedRequestBuilder("GET", "", query).get().build()).execute()
        response.use {
            if (it.code != 200) {
                throw statusError(it.code, it.message, it.body?.string())
            }
            parseListBucket(it.body?.bytes() ?: ByteArray(0))
        }
    }

    override suspend fun deleteObject(key: String): Unit = withContext(Dispatchers.IO) {
        val response = client.newCall(signedRequestBuilder("DELETE", key).delete().build()).execute()
        response.use {
            if (it.code !in listOf(200, 204, 404)) {
                throw statusError(it.code, it.message, it.body?.string())
            }
        }
    }

    private fun signedRequestBuilder(
        method: String,
        key: String,
        query: Map<String, String> = emptyMap(),
        body: ByteArray = ByteArray(0),
        extraHeaders: Map<String, String> = emptyMap(),
    ): Request.Builder {
        val url = objectUrl(key, query)
        val now = Instant.now()
        val amzDate = DateTimeFormatter.ofPattern("yyyyMMdd'T'HHmmss'Z'")
            .withZone(ZoneOffset.UTC)
            .format(now)
        val shortDate = DateTimeFormatter.ofPattern("yyyyMMdd")
            .withZone(ZoneOffset.UTC)
            .format(now)
        val payloadHash = sha256Hex(body)
        val headers = sortedMapOf<String, String>(
            "host" to URI(url.toString()).hostWithPort(),
            "x-amz-content-sha256" to payloadHash,
            "x-amz-date" to amzDate,
        )
        if (cfg.sessionToken.isNotBlank()) {
            headers["x-amz-security-token"] = cfg.sessionToken
        }
        extraHeaders.forEach { (k, v) -> headers[k.lowercase(Locale.US)] = v }
        val signedHeaders = headers.keys.joinToString(";")
        val canonicalHeaders = headers.entries.joinToString("") { "${it.key}:${it.value.trim()}\n" }
        val canonicalRequest = listOf(
            method,
            URI(url.toString()).rawPath?.ifEmpty { "/" } ?: "/",
            canonicalQuery(query),
            canonicalHeaders,
            signedHeaders,
            payloadHash,
        ).joinToString("\n")
        val scope = "$shortDate/${cfg.region}/s3/aws4_request"
        val stringToSign = listOf(
            "AWS4-HMAC-SHA256",
            amzDate,
            scope,
            sha256Hex(canonicalRequest.toByteArray(Charsets.UTF_8)),
        ).joinToString("\n")
        val signingKey = sigKey(cfg.secretAccessKey, shortDate, cfg.region, "s3")
        val signature = hmacSha256(signingKey, stringToSign.toByteArray(Charsets.UTF_8)).toHex()
        val authorization = "AWS4-HMAC-SHA256 Credential=${cfg.accessKeyId}/$scope, SignedHeaders=$signedHeaders, Signature=$signature"
        val builder = Request.Builder().url(url)
        headers.forEach { (k, v) -> builder.header(k, v) }
        builder.header("Authorization", authorization)
        return builder
    }

    private fun objectUrl(key: String, query: Map<String, String>): HttpUrl {
        val escapedKey = key.split('/').joinToString("/") { pathEncode(it) }
        val base = if (cfg.endpoint.isNotBlank()) {
            cfg.endpoint.toHttpUrl()
        } else {
            "https://${cfg.bucket}.s3.${cfg.region}.amazonaws.com".toHttpUrl()
        }
        val builder = base.newBuilder()
        if (cfg.endpoint.isNotBlank() && !cfg.forcePathStyle) {
            builder.host("${cfg.bucket}.${base.host}")
        }
        val rawPath = if (cfg.endpoint.isNotBlank() && cfg.forcePathStyle) {
            joinUrlPath(base.encodedPath, cfg.bucket, escapedKey)
        } else {
            joinUrlPath(base.encodedPath, escapedKey)
        }
        builder.encodedPath(rawPath)
        query.toSortedMap().forEach { (k, v) -> builder.addQueryParameter(k, v) }
        return builder.build()
    }

    private fun parseListBucket(data: ByteArray): ListPage {
        val doc = DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(ByteArrayInputStream(data))
        val nodes = doc.getElementsByTagName("Contents")
        val keys = mutableListOf<String>()
        for (i in 0 until nodes.length) {
            val contents = nodes.item(i) as? Element ?: continue
            val key = contents.getElementsByTagName("Key").item(0)?.textContent ?: continue
            keys.add(key)
        }
        val truncated = doc.getElementsByTagName("IsTruncated").item(0)?.textContent?.equals("true", ignoreCase = true) == true
        val next = doc.getElementsByTagName("NextContinuationToken").item(0)?.textContent.orEmpty()
        return ListPage(keys = keys.sorted(), isTruncated = truncated, nextContinuationToken = next)
    }

    private fun canonicalQuery(query: Map<String, String>): String =
        query.toSortedMap().entries.joinToString("&") {
            "${pathEncode(it.key)}=${pathEncode(it.value)}"
        }

    private fun statusError(code: Int, message: String, body: String?): Exception {
        val clean = body?.take(4096)?.trim().orEmpty().ifEmpty { message }
        return Exception("s3 status $code: $clean")
    }

    private fun URI.hostWithPort(): String =
        if (port == -1) host else "$host:$port"

    private fun joinUrlPath(vararg parts: String): String =
        "/" + parts
            .flatMap { it.split('/') }
            .filter { it.isNotEmpty() }
            .joinToString("/")

    private fun pathEncode(value: String): String =
        URLEncoder.encode(value, "UTF-8")
            .replace("+", "%20")
            .replace("%7E", "~")

    private fun sha256Hex(data: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(data).toHex()

    private fun sigKey(secret: String, date: String, region: String, service: String): ByteArray {
        val kDate = hmacSha256("AWS4$secret".toByteArray(Charsets.UTF_8), date.toByteArray(Charsets.UTF_8))
        val kRegion = hmacSha256(kDate, region.toByteArray(Charsets.UTF_8))
        val kService = hmacSha256(kRegion, service.toByteArray(Charsets.UTF_8))
        return hmacSha256(kService, "aws4_request".toByteArray(Charsets.UTF_8))
    }

    private fun hmacSha256(key: ByteArray, data: ByteArray): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(key, "HmacSHA256"))
        return mac.doFinal(data)
    }

    private fun ByteArray.toHex(): String = joinToString("") { "%02x".format(it.toInt() and 0xff) }
}

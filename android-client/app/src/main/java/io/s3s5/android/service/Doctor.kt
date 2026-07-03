package io.s3s5.android.service

import io.s3s5.android.config.AppConfig
import io.s3s5.android.config.AppSecrets
import io.s3s5.android.objectstore.PutOptions
import io.s3s5.android.protocol.Protocol
import io.s3s5.android.s3.S3ObjectStore
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.time.Instant
import kotlin.system.measureTimeMillis

object Doctor {
    suspend fun run(config: AppConfig, secrets: AppSecrets): Long = withContext(Dispatchers.IO) {
        config.validateForStart(secrets)
        val store = S3ObjectStore(config.s3Config(secrets))
        val key = "${Protocol.normalizePrefix(config.prefix)}/v1/doctor/android-${Instant.now().toEpochMilli()}.txt"
        measureTimeMillis {
            val payload = "s3s5 android doctor ${Instant.now()}".toByteArray(Charsets.UTF_8)
            store.putObject(key, payload, PutOptions(contentType = "text/plain"))
            val loaded = store.getObject(key)
            require(loaded.contentEquals(payload)) { "doctor object payload mismatch" }
            store.headObject(key)
            store.listPrefix(key.substringBeforeLast('/') + "/")
            store.deleteObject(key)
        }
    }
}

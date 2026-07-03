package io.s3s5.android.s3

import java.util.Locale

data class S3Config(
    val provider: String = "",
    val bucket: String = "",
    val region: String = "us-east-1",
    val endpoint: String = "",
    val forcePathStyle: Boolean = false,
    val accessKeyId: String = "",
    val secretAccessKey: String = "",
    val sessionToken: String = "",
) {
    fun withProviderDefaults(): S3Config {
        val inferred = provider.trim().lowercase(Locale.US).ifEmpty { inferProvider(region, endpoint) }
        return when (inferred) {
            "yandex", "yc", "yandex-cloud", "yandexcloud" -> copy(
                provider = "yandex",
                endpoint = endpoint.ifEmpty { "https://storage.yandexcloud.net" },
                region = if (region.isEmpty() || region == "us-east-1" || region.startsWith("ru-central1-")) {
                    "ru-central1"
                } else {
                    region
                },
                forcePathStyle = true,
            )
            "minio" -> copy(
                provider = "minio",
                endpoint = endpoint.ifEmpty { "http://127.0.0.1:9000" },
                region = region.ifEmpty { "us-east-1" },
                forcePathStyle = true,
            )
            "custom" -> copy(provider = "custom", region = region.ifEmpty { "us-east-1" })
            "aws" -> copy(provider = "aws", region = region.ifEmpty { "us-east-1" })
            else -> copy(provider = inferred, region = region.ifEmpty { "us-east-1" })
        }.let {
            if (it.bucket.contains(".") && it.endpoint.isNotEmpty()) {
                it.copy(forcePathStyle = true)
            } else {
                it
            }
        }
    }

    companion object {
        fun inferProvider(region: String, endpoint: String): String {
            val r = region.trim().lowercase(Locale.US)
            val e = endpoint.trim().lowercase(Locale.US)
            return when {
                e.contains("storage.yandexcloud.net") -> "yandex"
                r.startsWith("ru-central1") -> "yandex"
                e.contains("127.0.0.1") || e.contains("localhost") -> "minio"
                else -> "aws"
            }
        }
    }
}

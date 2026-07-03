package io.s3s5.android.s3

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class S3ConfigTest {
    @Test
    fun yandexDefaultsMatchGo() {
        val cfg = S3Config(provider = "yc", region = "us-east-1").withProviderDefaults()
        assertEquals("yandex", cfg.provider)
        assertEquals("https://storage.yandexcloud.net", cfg.endpoint)
        assertEquals("ru-central1", cfg.region)
        assertTrue(cfg.forcePathStyle)
    }

    @Test
    fun minioDefaultsMatchGo() {
        val cfg = S3Config(provider = "minio").withProviderDefaults()
        assertEquals("http://127.0.0.1:9000", cfg.endpoint)
        assertEquals("us-east-1", cfg.region)
        assertTrue(cfg.forcePathStyle)
    }
}

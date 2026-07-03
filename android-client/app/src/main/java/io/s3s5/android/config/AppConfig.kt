package io.s3s5.android.config

import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import io.s3s5.android.BuildConfig
import io.s3s5.android.s3.S3Config
import java.security.KeyStore
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

data class AppConfig(
    val provider: String = "aws",
    val bucket: String = "",
    val prefix: String = "s3s5",
    val region: String = "us-east-1",
    val endpoint: String = "",
    val forcePathStyle: Boolean = false,
    val listenHost: String = "127.0.0.1",
    val listenPort: Int = 1080,
    val chunkSize: Int = 64 * 1024,
    val pollMinMillis: Long = 50,
    val pollMaxMillis: Long = 2_000,
    val windowChunks: Long = 16,
    val idleTimeoutMillis: Long = 120_000,
    val allowLanListen: Boolean = false,
) {
    fun validateForStart(secrets: AppSecrets) {
        require(bucket.isNotBlank()) { "bucket is required" }
        require(secrets.accessKeyId.isNotBlank()) { "access key is required" }
        require(secrets.secretAccessKey.isNotBlank()) { "secret key is required" }
        require(secrets.psk.length >= 16) { "PSK must be at least 16 characters" }
        require(listenPort in 1..65535) { "listen port must be 1..65535" }
        require(listenHost == "127.0.0.1" || allowLanListen) {
            "LAN listen requires explicit opt-in"
        }
        require(BuildConfig.DEBUG || !endpoint.startsWith("http://", ignoreCase = true)) {
            "release builds require HTTPS endpoints"
        }
    }

    fun s3Config(secrets: AppSecrets): S3Config = S3Config(
        provider = provider,
        bucket = bucket,
        region = region,
        endpoint = endpoint,
        forcePathStyle = forcePathStyle,
        accessKeyId = secrets.accessKeyId,
        secretAccessKey = secrets.secretAccessKey,
        sessionToken = secrets.sessionToken,
    )
}

data class AppSecrets(
    val accessKeyId: String = "",
    val secretAccessKey: String = "",
    val sessionToken: String = "",
    val psk: String = "",
)

class ConfigStore(context: Context) {
    private val prefs: SharedPreferences = context.getSharedPreferences("s3s5-config", Context.MODE_PRIVATE)
    private val secrets = SecretStore(context)

    fun loadConfig(): AppConfig = AppConfig(
        provider = prefs.getString("provider", "aws").orEmpty(),
        bucket = prefs.getString("bucket", "").orEmpty(),
        prefix = prefs.getString("prefix", "s3s5").orEmpty(),
        region = prefs.getString("region", "us-east-1").orEmpty(),
        endpoint = prefs.getString("endpoint", "").orEmpty(),
        forcePathStyle = prefs.getBoolean("forcePathStyle", false),
        listenHost = prefs.getString("listenHost", "127.0.0.1").orEmpty(),
        listenPort = prefs.getInt("listenPort", 1080),
        chunkSize = prefs.getInt("chunkSize", 64 * 1024),
        pollMinMillis = prefs.getLong("pollMinMillis", 50),
        pollMaxMillis = prefs.getLong("pollMaxMillis", 2_000),
        windowChunks = prefs.getLong("windowChunks", 16),
        idleTimeoutMillis = prefs.getLong("idleTimeoutMillis", 120_000),
        allowLanListen = prefs.getBoolean("allowLanListen", false),
    )

    fun saveConfig(config: AppConfig) {
        prefs.edit()
            .putString("provider", config.provider)
            .putString("bucket", config.bucket)
            .putString("prefix", config.prefix)
            .putString("region", config.region)
            .putString("endpoint", config.endpoint)
            .putBoolean("forcePathStyle", config.forcePathStyle)
            .putString("listenHost", config.listenHost)
            .putInt("listenPort", config.listenPort)
            .putInt("chunkSize", config.chunkSize)
            .putLong("pollMinMillis", config.pollMinMillis)
            .putLong("pollMaxMillis", config.pollMaxMillis)
            .putLong("windowChunks", config.windowChunks)
            .putLong("idleTimeoutMillis", config.idleTimeoutMillis)
            .putBoolean("allowLanListen", config.allowLanListen)
            .apply()
    }

    fun loadSecrets(): AppSecrets = AppSecrets(
        accessKeyId = secrets.get("accessKeyId"),
        secretAccessKey = secrets.get("secretAccessKey"),
        sessionToken = secrets.get("sessionToken"),
        psk = secrets.get("psk"),
    )

    fun saveSecrets(value: AppSecrets) {
        secrets.put("accessKeyId", value.accessKeyId)
        secrets.put("secretAccessKey", value.secretAccessKey)
        secrets.put("sessionToken", value.sessionToken)
        secrets.put("psk", value.psk)
    }

    fun clearSecrets() {
        secrets.clear()
    }
}

private class SecretStore(context: Context) {
    private val prefs = context.getSharedPreferences("s3s5-secrets", Context.MODE_PRIVATE)
    private val keyAlias = "s3s5-secrets-v1"
    private val b64 = Base64.getEncoder()
    private val b64d = Base64.getDecoder()

    fun get(name: String): String {
        val encoded = prefs.getString(name, "").orEmpty()
        if (encoded.isEmpty()) {
            return ""
        }
        val packed = b64d.decode(encoded)
        if (packed.size < 13) {
            return ""
        }
        val nonce = packed.copyOfRange(0, 12)
        val ciphertext = packed.copyOfRange(12, packed.size)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, nonce))
        return String(cipher.doFinal(ciphertext), Charsets.UTF_8)
    }

    fun put(name: String, value: String) {
        if (value.isEmpty()) {
            prefs.edit().remove(name).apply()
            return
        }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val nonce = cipher.iv
        val ciphertext = cipher.doFinal(value.toByteArray(Charsets.UTF_8))
        prefs.edit().putString(name, b64.encodeToString(nonce + ciphertext)).apply()
    }

    fun clear() {
        prefs.edit().clear().apply()
    }

    private fun key(): SecretKey {
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (keyStore.getKey(keyAlias, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        val spec = KeyGenParameterSpec.Builder(
            keyAlias,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
        )
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setRandomizedEncryptionRequired(true)
            .build()
        generator.init(spec)
        return generator.generateKey()
    }
}

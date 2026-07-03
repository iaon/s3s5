package io.s3s5.android.ui

import android.Manifest
import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.text.InputType
import android.view.View
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import io.s3s5.android.config.AppConfig
import io.s3s5.android.config.AppSecrets
import io.s3s5.android.config.ConfigStore
import io.s3s5.android.service.Doctor
import io.s3s5.android.service.S3S5ForegroundService
import io.s3s5.android.service.ServiceStatus
import io.s3s5.android.service.ServiceStatusBus
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

class MainActivity : Activity() {
    private val scope = MainScope()
    private lateinit var configStore: ConfigStore
    private lateinit var fields: Fields
    private lateinit var statusText: TextView
    private lateinit var countersText: TextView
    private lateinit var logsText: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        configStore = ConfigStore(this)
        requestNotificationsIfNeeded()
        buildUi()
        loadValues()
        scope.launch {
            ServiceStatusBus.state.collectLatest { renderStatus(it) }
        }
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    private fun buildUi() {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(28, 28, 28, 28)
        }
        val scroll = ScrollView(this).apply { addView(root) }
        setContentView(scroll)

        val title = TextView(this).apply {
            text = "s3s5"
            textSize = 24f
        }
        root.addView(title)

        fields = Fields(
            provider = edit("Provider"),
            bucket = edit("Bucket"),
            prefix = edit("Prefix"),
            region = edit("Region"),
            endpoint = edit("Endpoint"),
            accessKey = edit("Access key"),
            secretKey = edit("Secret key", password = true),
            sessionToken = edit("Session token", password = true),
            psk = edit("PSK", password = true),
            listenHost = edit("Listen host"),
            listenPort = edit("Listen port", number = true),
            forcePathStyle = check("Path-style S3"),
            allowLanListen = check("Allow LAN listen"),
        )
        listOf(
            fields.provider,
            fields.bucket,
            fields.prefix,
            fields.region,
            fields.endpoint,
            fields.accessKey,
            fields.secretKey,
            fields.sessionToken,
            fields.psk,
            fields.listenHost,
            fields.listenPort,
        ).forEach { root.addView(it) }
        root.addView(fields.forcePathStyle)
        root.addView(fields.allowLanListen)

        val row1 = row()
        row1.addView(button("Save") { saveValues(showLog = true) })
        row1.addView(button("Start") { startTunnel() })
        row1.addView(button("Stop") { stopTunnel() })
        root.addView(row1)

        val row2 = row()
        row2.addView(button("Doctor") { runDoctor() })
        row2.addView(button("Copy SOCKS") { copySocksAddress() })
        row2.addView(button("Clear logs") { ServiceStatusBus.clearLogs() })
        root.addView(row2)

        val row3 = row()
        row3.addView(button("Clear secrets") {
            configStore.clearSecrets()
            fields.accessKey.setText("")
            fields.secretKey.setText("")
            fields.sessionToken.setText("")
            fields.psk.setText("")
            ServiceStatusBus.log("secrets cleared")
        })
        root.addView(row3)

        statusText = TextView(this).apply { textSize = 16f }
        countersText = TextView(this).apply { textSize = 14f }
        logsText = TextView(this).apply {
            textSize = 13f
            typeface = android.graphics.Typeface.MONOSPACE
        }
        root.addView(statusText)
        root.addView(countersText)
        root.addView(logsText)
    }

    private fun loadValues() {
        val config = configStore.loadConfig()
        val secrets = configStore.loadSecrets()
        fields.provider.setText(config.provider)
        fields.bucket.setText(config.bucket)
        fields.prefix.setText(config.prefix)
        fields.region.setText(config.region)
        fields.endpoint.setText(config.endpoint)
        fields.forcePathStyle.isChecked = config.forcePathStyle
        fields.listenHost.setText(config.listenHost)
        fields.listenPort.setText(config.listenPort.toString())
        fields.allowLanListen.isChecked = config.allowLanListen
        fields.accessKey.setText(secrets.accessKeyId)
        fields.secretKey.setText(secrets.secretAccessKey)
        fields.sessionToken.setText(secrets.sessionToken)
        fields.psk.setText(secrets.psk)
    }

    private fun saveValues(showLog: Boolean = false): Pair<AppConfig, AppSecrets> {
        val config = AppConfig(
            provider = fields.provider.textString(),
            bucket = fields.bucket.textString(),
            prefix = fields.prefix.textString().ifEmpty { "s3s5" },
            region = fields.region.textString().ifEmpty { "us-east-1" },
            endpoint = fields.endpoint.textString(),
            forcePathStyle = fields.forcePathStyle.isChecked,
            listenHost = fields.listenHost.textString().ifEmpty { "127.0.0.1" },
            listenPort = fields.listenPort.textString().toIntOrNull() ?: 1080,
            allowLanListen = fields.allowLanListen.isChecked,
        )
        val secrets = AppSecrets(
            accessKeyId = fields.accessKey.textString(),
            secretAccessKey = fields.secretKey.textString(),
            sessionToken = fields.sessionToken.textString(),
            psk = fields.psk.textString(),
        )
        configStore.saveConfig(config)
        configStore.saveSecrets(secrets)
        if (showLog) {
            ServiceStatusBus.log("configuration saved")
        }
        return config to secrets
    }

    private fun startTunnel() {
        saveValues()
        val intent = S3S5ForegroundService.startIntent(this)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun stopTunnel() {
        startService(S3S5ForegroundService.stopIntent(this))
    }

    private fun runDoctor() {
        val (config, secrets) = saveValues()
        ServiceStatusBus.log("doctor started")
        scope.launch {
            try {
                val latency = Doctor.run(config, secrets)
                ServiceStatusBus.update { it.copy(doctorLatencyMillis = latency, lastError = "") }
                ServiceStatusBus.log("doctor passed in ${latency}ms")
            } catch (e: Exception) {
                ServiceStatusBus.update { it.copy(lastError = e.message.orEmpty()) }
                ServiceStatusBus.log("doctor failed: ${e.message.orEmpty()}")
            }
        }
    }

    private fun copySocksAddress() {
        val address = "${fields.listenHost.textString().ifEmpty { "127.0.0.1" }}:${fields.listenPort.textString().ifEmpty { "1080" }}"
        val clipboard = getSystemService(ClipboardManager::class.java)
        clipboard.setPrimaryClip(ClipData.newPlainText("s3s5 SOCKS address", address))
        ServiceStatusBus.log("SOCKS address copied")
    }

    private fun renderStatus(status: ServiceStatus) {
        statusText.text = buildString {
            append("Status: ${status.state.name.lowercase()}\n")
            append("Listen: ${status.listenAddress}\n")
            if (status.lastError.isNotEmpty()) append("Last error: ${status.lastError}\n")
            if (status.doctorLatencyMillis >= 0) append("Doctor: ${status.doctorLatencyMillis}ms\n")
        }
        countersText.text = buildString {
            val t = status.tunnelStats
            val s = status.storeCounters
            append("Sessions: ${t.activeSessions}\n")
            append("Bytes sent/received: ${t.bytesSent}/${t.bytesReceived}\n")
            append("Chunks sent/received: ${t.chunksSent}/${t.chunksReceived}\n")
            append("S3 PUT/GET/HEAD/LIST/DELETE: ${s.put}/${s.get}/${s.head}/${s.list}/${s.delete}")
        }
        logsText.text = status.logs.joinToString("\n")
    }

    private fun edit(hint: String, password: Boolean = false, number: Boolean = false): EditText =
        EditText(this).apply {
            this.hint = hint
            singleLine = true
            inputType = when {
                password -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
                number -> InputType.TYPE_CLASS_NUMBER
                else -> InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            }
        }

    private fun check(text: String): CheckBox =
        CheckBox(this).apply { this.text = text }

    private fun button(text: String, action: (View) -> Unit): Button =
        Button(this).apply {
            this.text = text
            setOnClickListener { action(it) }
        }

    private fun row(): LinearLayout =
        LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }

    private fun EditText.textString(): String = text?.toString()?.trim().orEmpty()

    private fun requestNotificationsIfNeeded() {
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 100)
        }
    }

    private data class Fields(
        val provider: EditText,
        val bucket: EditText,
        val prefix: EditText,
        val region: EditText,
        val endpoint: EditText,
        val accessKey: EditText,
        val secretKey: EditText,
        val sessionToken: EditText,
        val psk: EditText,
        val listenHost: EditText,
        val listenPort: EditText,
        val forcePathStyle: CheckBox,
        val allowLanListen: CheckBox,
    )
}

package io.s3s5.android.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import io.s3s5.android.R
import io.s3s5.android.config.ConfigStore
import io.s3s5.android.crypto.PskCodec
import io.s3s5.android.objectstore.CountingObjectStore
import io.s3s5.android.s3.S3ObjectStore
import io.s3s5.android.socks5.REPLY_GENERAL_FAILURE
import io.s3s5.android.socks5.Socks5Server
import io.s3s5.android.tunnel.TunnelClient
import io.s3s5.android.tunnel.TunnelConfig
import io.s3s5.android.tunnel.TunnelStats
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

class S3S5ForegroundService : Service() {
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var server: Socks5Server? = null
    private var statsJob: Job? = null
    private var tunnelStats: TunnelStats? = null
    private var countingStore: CountingObjectStore? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        ensureChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopServiceWork()
            else -> startServiceWork()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopServiceWork()
        serviceScope.cancel()
        super.onDestroy()
    }

    private fun startServiceWork() {
        if (server != null) {
            return
        }
        ServiceStatusBus.update { it.copy(state = RunState.STARTING, lastError = "") }
        startForeground(NOTIFICATION_ID, notification("Starting", ServiceStatusBus.state.value.listenAddress))
        serviceScope.launch {
            try {
                val store = ConfigStore(this@S3S5ForegroundService)
                val config = store.loadConfig()
                val secrets = store.loadSecrets()
                config.validateForStart(secrets)
                val baseStore = S3ObjectStore(config.s3Config(secrets))
                val counted = CountingObjectStore(baseStore)
                val stats = TunnelStats()
                val client = TunnelClient(
                    TunnelConfig(
                        store = counted,
                        codec = PskCodec(secrets.psk),
                        stats = stats,
                        prefix = config.prefix,
                        chunkSize = config.chunkSize,
                        pollMinMillis = config.pollMinMillis,
                        pollMaxMillis = config.pollMaxMillis,
                        windowChunks = config.windowChunks,
                        idleTimeoutMillis = config.idleTimeoutMillis,
                    ),
                )
                val listenAddress = "${config.listenHost}:${config.listenPort}"
                val socks = Socks5Server(config.listenHost, config.listenPort, serviceScope) { target, socket, reply ->
                    try {
                        client.handleSocks(target, socket, reply)
                    } catch (e: Exception) {
                        reply(REPLY_GENERAL_FAILURE)
                        ServiceStatusBus.log("session failed: ${e.message.orEmpty()}")
                    }
                }
                socks.start()
                server = socks
                tunnelStats = stats
                countingStore = counted
                ServiceStatusBus.update {
                    it.copy(state = RunState.RUNNING, listenAddress = listenAddress, lastError = "")
                }
                updateNotification("Running", listenAddress)
                statsJob = serviceScope.launch {
                    while (true) {
                        ServiceStatusBus.update {
                            it.copy(
                                tunnelStats = stats.snapshot(),
                                storeCounters = counted.snapshot(),
                            )
                        }
                        delay(1_000)
                    }
                }
            } catch (e: Exception) {
                ServiceStatusBus.update { it.copy(state = RunState.ERROR, lastError = e.message.orEmpty()) }
                ServiceStatusBus.log("service failed: ${e.message.orEmpty()}")
                updateNotification("Error", e.message.orEmpty())
                stopSelf()
            }
        }
    }

    private fun stopServiceWork() {
        ServiceStatusBus.update { it.copy(state = RunState.STOPPING) }
        statsJob?.cancel()
        statsJob = null
        serviceScope.launch {
            server?.stop()
            server = null
            ServiceStatusBus.update { it.copy(state = RunState.STOPPED) }
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun notification(title: String, text: String): Notification {
        val stopIntent = Intent(this, S3S5ForegroundService::class.java).setAction(ACTION_STOP)
        val stopPendingIntent = PendingIntent.getService(
            this,
            1,
            stopIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("s3s5 $title")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_upload_done)
            .setOngoing(true)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Stop", stopPendingIntent)
            .build()
    }

    private fun updateNotification(title: String, text: String) {
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification(title, text))
    }

    private fun ensureChannel() {
        val manager = getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.notification_channel),
            NotificationManager.IMPORTANCE_LOW,
        )
        manager.createNotificationChannel(channel)
    }

    companion object {
        private const val CHANNEL_ID = "s3s5-tunnel"
        private const val NOTIFICATION_ID = 5305
        const val ACTION_STOP = "io.s3s5.android.STOP"

        fun startIntent(context: Context): Intent = Intent(context, S3S5ForegroundService::class.java)
        fun stopIntent(context: Context): Intent =
            Intent(context, S3S5ForegroundService::class.java).setAction(ACTION_STOP)
    }
}

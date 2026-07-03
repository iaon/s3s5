package io.s3s5.android.service

import io.s3s5.android.objectstore.StoreCounters
import io.s3s5.android.tunnel.TunnelStatsSnapshot
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

enum class RunState {
    STOPPED,
    STARTING,
    RUNNING,
    STOPPING,
    ERROR,
}

data class ServiceStatus(
    val state: RunState = RunState.STOPPED,
    val listenAddress: String = "127.0.0.1:1080",
    val lastError: String = "",
    val doctorLatencyMillis: Long = -1,
    val tunnelStats: TunnelStatsSnapshot = TunnelStatsSnapshot(0, 0, 0, 0, 0),
    val storeCounters: StoreCounters = StoreCounters(0, 0, 0, 0, 0),
    val logs: List<String> = emptyList(),
)

object ServiceStatusBus {
    private val mutable = MutableStateFlow(ServiceStatus())
    val state: StateFlow<ServiceStatus> = mutable

    fun update(transform: (ServiceStatus) -> ServiceStatus) {
        mutable.value = transform(mutable.value)
    }

    fun log(message: String) {
        update { current ->
            current.copy(logs = (current.logs + message).takeLast(100))
        }
    }

    fun clearLogs() {
        update { it.copy(logs = emptyList()) }
    }
}

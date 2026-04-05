package org.organicprogramming.gabriel.greeting.kotlincompose.controller

import io.grpc.BindableService
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.organicprogramming.gabriel.greeting.kotlincompose.model.AppPlatformCapabilities
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxServerTransport
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxSettingsSnapshot
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.DescribeRegistration
import org.organicprogramming.gabriel.greeting.kotlincompose.rpc.CoaxRpcService
import org.organicprogramming.gabriel.greeting.kotlincompose.rpc.GreetingAppRpcService
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.SettingsStore
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.coaxEnabledKey
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.coaxSettingsKey
import org.organicprogramming.holons.Serve

class CoaxController(
    private val greetingController: GreetingController,
    private val settingsStore: SettingsStore,
    private val capabilities: AppPlatformCapabilities = AppPlatformCapabilities.desktopCurrent(),
    dispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    private val scope = CoroutineScope(SupervisorJob() + dispatcher)
    private val _state = MutableStateFlow(
        CoaxUiState(
            isEnabled = settingsStore.readBool(coaxEnabledKey()),
            snapshot = loadSnapshot(settingsStore, capabilities),
            capabilities = capabilities,
        ),
    )

    val state: StateFlow<CoaxUiState> = _state.asStateFlow()

    private var runningServer: Serve.RunningServer? = null
    private var disposed = false
    private var startGeneration = 0

    suspend fun startIfEnabled() {
        if (!state.value.isEnabled) {
            return
        }
        reconfigureRuntime()
    }

    suspend fun setIsEnabled(value: Boolean) {
        if (state.value.isEnabled == value) {
            return
        }
        settingsStore.writeBool(coaxEnabledKey(), value)
        _state.update { it.copy(isEnabled = value) }
        reconfigureRuntime()
    }

    suspend fun setServerEnabled(value: Boolean) {
        if (state.value.serverEnabled == value) {
            return
        }
        updateSnapshot(state.value.snapshot.copy(serverEnabled = value))
        reconfigureRuntime()
    }

    suspend fun setServerTransport(value: CoaxServerTransport) {
        if (state.value.serverTransport == value) {
            return
        }
        val nextPortText = if (usesDefaultServerPort()) value.defaultPort.toString() else state.value.serverPortText
        updateSnapshot(
            state.value.snapshot.copy(
                serverTransport = value,
                serverPortText = nextPortText,
            ),
        )
        if (state.value.serverEnabled) {
            reconfigureRuntime()
        }
    }

    suspend fun setServerHost(value: String) {
        if (state.value.serverHost == value) {
            return
        }
        updateSnapshot(state.value.snapshot.copy(serverHost = value))
        if (state.value.serverEnabled) {
            reconfigureRuntime()
        }
    }

    suspend fun setServerPortText(value: String) {
        if (state.value.serverPortText == value) {
            return
        }
        updateSnapshot(state.value.snapshot.copy(serverPortText = value))
        if (state.value.serverEnabled) {
            reconfigureRuntime()
        }
    }

    suspend fun setServerUnixPath(value: String) {
        if (state.value.serverUnixPath == value) {
            return
        }
        updateSnapshot(state.value.snapshot.copy(serverUnixPath = value))
        if (state.value.serverEnabled) {
            reconfigureRuntime()
        }
    }

    fun disableAfterRpc() {
        scope.launch {
            delay(100)
            setIsEnabled(false)
        }
    }

    suspend fun shutdown() {
        disposed = true
        stopServer(clearStatus = true)
        scope.cancel()
    }

    private suspend fun reconfigureRuntime() {
        val generation = ++startGeneration
        if (!state.value.isEnabled || !state.value.serverEnabled) {
            stopServer(clearStatus = true)
            return
        }

        stopServer(clearStatus = false)
        startServer(generation)
    }

    private suspend fun startServer(generation: Int) {
        val appService: BindableService = GreetingAppRpcService(greetingController)
        val coaxService: BindableService = CoaxRpcService(greetingController, this)
        _state.update { it.copy(listenUri = null, statusDetail = null) }

        try {
            DescribeRegistration.ensureRegistered()
            val server = Serve.startWithOptions(
                runtimeListenUri(),
                listOf(coaxService, appService),
                Serve.Options(describe = true, logger = { }),
            )
            if (generation != startGeneration || !state.value.isEnabled || !state.value.serverEnabled) {
                server.stop()
                return
            }
            runningServer = server
            _state.update { it.copy(listenUri = server.publicUri, statusDetail = null) }
        } catch (error: Throwable) {
            if (generation != startGeneration) {
                return
            }
            _state.update {
                it.copy(
                    listenUri = null,
                    statusDetail = "Server surface failed to start: $error",
                )
            }
        }
    }

    private suspend fun stopServer(clearStatus: Boolean) {
        val server = runningServer
        runningServer = null
        _state.update {
            it.copy(
                listenUri = null,
                statusDetail = if (clearStatus) null else it.statusDetail,
            )
        }
        if (server == null) {
            return
        }
        delay(250)
        server.stop()
    }

    private fun runtimeListenUri(): String =
        when (state.value.serverTransport) {
            CoaxServerTransport.TCP -> {
                val host = state.value.serverHost.trim().ifEmpty { CoaxSettingsSnapshot.defaultHost }
                "tcp://$host:${state.value.serverPort}"
            }
            CoaxServerTransport.UNIX -> {
                val path = state.value.serverUnixPath.trim().ifEmpty { CoaxSettingsSnapshot.defaultUnixPath }
                "unix://$path"
            }
        }

    private fun usesDefaultServerPort(): Boolean {
        val trimmed = state.value.serverPortText.trim()
        if (state.value.serverTransport == CoaxServerTransport.UNIX) {
            return trimmed.isEmpty()
        }
        return trimmed.isEmpty() || trimmed.toIntOrNull() == state.value.serverTransport.defaultPort
    }

    private fun updateSnapshot(snapshot: CoaxSettingsSnapshot) {
        settingsStore.writeString(coaxSettingsKey(), snapshot.encode())
        _state.update { it.copy(snapshot = snapshot) }
    }

    companion object {
        private fun loadSnapshot(
            store: SettingsStore,
            capabilities: AppPlatformCapabilities,
        ): CoaxSettingsSnapshot {
            val rawValue = store.readString(coaxSettingsKey())
            val decoded = CoaxSettingsSnapshot.decode(rawValue)
            return if (!capabilities.supportsUnixSockets && decoded.serverTransport == CoaxServerTransport.UNIX) {
                CoaxSettingsSnapshot.defaults
            } else {
                decoded
            }
        }
    }
}

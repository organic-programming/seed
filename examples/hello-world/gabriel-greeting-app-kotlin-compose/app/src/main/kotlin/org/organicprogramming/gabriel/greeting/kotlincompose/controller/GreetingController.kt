package org.organicprogramming.gabriel.greeting.kotlincompose.controller

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import org.organicprogramming.gabriel.greeting.kotlincompose.model.AppPlatformCapabilities
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GreetingUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.model.LanguageOption
import org.organicprogramming.gabriel.greeting.kotlincompose.model.normalizedTransportSelection
import org.organicprogramming.gabriel.greeting.kotlincompose.model.preferredHolon
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.GreetingHolonConnection
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.HolonCatalog
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.HolonConnector
import java.util.concurrent.atomic.AtomicBoolean

class GreetingController(
    private val catalog: HolonCatalog,
    private val connector: HolonConnector,
    private val capabilities: AppPlatformCapabilities = AppPlatformCapabilities.desktopCurrent(),
    initialTransport: String? = System.getenv("OP_ASSEMBLY_TRANSPORT"),
    dispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    private val scope = CoroutineScope(SupervisorJob() + dispatcher)
    private val initialized = AtomicBoolean(false)
    private val _state = MutableStateFlow(
        GreetingUiState(
            transport = normalizedTransportSelection(initialTransport),
        ),
    )

    val state: StateFlow<GreetingUiState> = _state.asStateFlow()

    private var connection: GreetingHolonConnection? = null
    private var startFuture: kotlinx.coroutines.Deferred<Unit>? = null
    private var disposed = false
    private var connectionGeneration = 0
    private var loadGeneration = 0
    private var greetGeneration = 0

    suspend fun initialize() {
        if (!initialized.compareAndSet(false, true)) {
            return
        }
        refreshHolons()
        loadLanguages(greetAfterLoad = false)
        if (state.value.selectedLanguageCode.isNotBlank()) {
            greet()
        }
    }

    suspend fun refreshHolons() {
        val previousSelection = state.value.selectedHolon?.slug
        try {
            val discovered = catalog.discover()
            val selected = when {
                discovered.isEmpty() -> null
                previousSelection != null -> discovered.firstOrNull { it.slug == previousSelection }
                else -> null
            } ?: preferredHolon(discovered) ?: discovered.firstOrNull()

            _state.update {
                it.copy(
                    availableHolons = discovered,
                    selectedHolon = selected,
                    connectionError = null,
                )
            }
        } catch (error: Throwable) {
            _state.update {
                it.copy(
                    availableHolons = emptyList(),
                    selectedHolon = null,
                    connectionError = "Failed to discover Gabriel holons: $error",
                )
            }
        }
    }

    suspend fun selectHolonBySlug(slug: String, reload: Boolean = true) {
        val identity = state.value.availableHolons.firstOrNull { it.slug == slug }
            ?: throw IllegalStateException("Holon '$slug' not found")
        if (state.value.selectedHolon == identity) {
            if (reload) {
                loadLanguages()
            }
            return
        }
        _state.update { it.copy(selectedHolon = identity) }
        stop()
        if (reload) {
            loadLanguages()
        }
    }

    suspend fun setTransport(value: String, reload: Boolean = true) {
        val normalized = normalizedTransportSelection(value)
        require(capabilities.appTransports.contains(normalized)) {
            "Transport \"$normalized\" is not available on this platform"
        }
        if (normalized == state.value.transport) {
            return
        }
        _state.update { it.copy(transport = normalized) }
        stop()
        if (reload) {
            loadLanguages()
        }
    }

    suspend fun setSelectedLanguage(code: String, greetNow: Boolean = true) {
        if (code == state.value.selectedLanguageCode) {
            return
        }
        _state.update { it.copy(selectedLanguageCode = code) }
        if (greetNow) {
            greet()
        }
    }

    suspend fun setUserName(value: String, greetNow: Boolean = true) {
        if (value == state.value.userName) {
            return
        }
        _state.update { it.copy(userName = value) }
        if (greetNow && state.value.selectedLanguageCode.isNotBlank()) {
            greet()
        }
    }

    suspend fun loadLanguages(greetAfterLoad: Boolean = true) {
        val generation = ++loadGeneration
        _state.update {
            it.copy(
                isLoading = true,
                error = null,
                greeting = "",
                availableLanguages = emptyList(),
            )
        }

        try {
            ensureStarted()
        } catch (_: Throwable) {
            if (loadGeneration == generation) {
                _state.update {
                    it.copy(
                        isLoading = false,
                        error = "Failed to load languages: ${it.connectionError ?: "Holon did not become ready"}",
                    )
                }
            }
            return
        }

        if (!state.value.isRunning) {
            if (loadGeneration == generation) {
                _state.update {
                    it.copy(
                        isLoading = false,
                        error = "Failed to load languages: ${it.connectionError ?: "Holon did not become ready"}",
                    )
                }
            }
            return
        }

        val retryDelays = if (state.value.transport == "stdio") {
            listOf(0L, 80L, 180L)
        } else {
            listOf(120L, 300L, 600L)
        }

        retryDelays.forEachIndexed { index, delayMillis ->
            try {
                if (delayMillis > 0) {
                    delay(delayMillis)
                }
                val languages = requireNotNull(connection).listLanguages()
                if (loadGeneration != generation) {
                    return
                }
                val preferredCode = state.value.selectedLanguageCode
                val selectedCode = languages.firstOrNull { it.code == preferredCode }?.code
                    ?: languages.firstOrNull { it.code == "en" }?.code
                    ?: languages.firstOrNull()?.code
                    .orEmpty()
                _state.update {
                    it.copy(
                        availableLanguages = languages,
                        selectedLanguageCode = selectedCode,
                        error = null,
                        isLoading = false,
                    )
                }
                if (greetAfterLoad && selectedCode.isNotBlank()) {
                    greet()
                }
                return
            } catch (error: Throwable) {
                if (index == retryDelays.lastIndex && loadGeneration == generation) {
                    _state.update {
                        it.copy(
                            isLoading = false,
                            error = "Failed to load languages: ${it.connectionError ?: error}",
                        )
                    }
                }
            }
        }
    }

    suspend fun greet(name: String? = null, langCode: String? = null) {
        val resolvedCode = langCode ?: state.value.selectedLanguageCode
        if (resolvedCode.isBlank()) {
            return
        }

        val requestGeneration = ++greetGeneration
        _state.update { it.copy(isGreeting = true) }
        try {
            ensureStarted()
            val response = requireNotNull(connection).sayHello(
                name = name ?: state.value.userName,
                langCode = resolvedCode,
            )
            if (greetGeneration == requestGeneration) {
                _state.update {
                    it.copy(
                        greeting = response,
                        error = null,
                    )
                }
            }
        } catch (error: Throwable) {
            if (greetGeneration == requestGeneration) {
                _state.update {
                    it.copy(error = "Greeting failed: $error")
                }
            }
        } finally {
            if (greetGeneration == requestGeneration) {
                _state.update { it.copy(isGreeting = false) }
            }
        }
    }

    suspend fun ensureStarted() {
        connection?.let { return }
        startFuture?.let {
            it.await()
            connection?.let { return }
        }

        if (state.value.availableHolons.isEmpty()) {
            refreshHolons()
        }

        val holon = state.value.selectedHolon ?: preferredHolon(state.value.availableHolons)
        if (holon == null) {
            _state.update {
                it.copy(
                    isRunning = false,
                    connectionError = "No Gabriel holons found",
                )
            }
            throw IllegalStateException(state.value.connectionError)
        }

        _state.update { it.copy(selectedHolon = holon, connectionError = null) }
        val generation = ++connectionGeneration
        val future = scope.async {
            connect(generation, holon)
        }
        startFuture = future
        try {
            future.await()
        } finally {
            if (startFuture === future) {
                startFuture = null
            }
        }
    }

    private suspend fun connect(generation: Int, holon: GabrielHolonIdentity) {
        try {
            val nextConnection = connector.connect(holon, state.value.transport)
            if (generation != connectionGeneration || disposed) {
                nextConnection.close()
                return
            }
            connection = nextConnection
            _state.update {
                it.copy(
                    isRunning = true,
                    connectionError = null,
                )
            }
        } catch (error: Throwable) {
            if (generation != connectionGeneration || disposed) {
                return
            }
            connection = null
            _state.update {
                it.copy(
                    isRunning = false,
                    connectionError = "Failed to start Gabriel holon: $error",
                )
            }
            throw error
        }
    }

    suspend fun stop() {
        connectionGeneration += 1
        startFuture = null
        val currentConnection = connection
        connection = null
        _state.update { it.copy(isRunning = false) }
        if (currentConnection == null) {
            return
        }
        try {
            currentConnection.close()
        } catch (error: Throwable) {
            _state.update { it.copy(connectionError = "Failed to stop Gabriel holon connection: $error") }
        }
    }

    suspend fun shutdown() {
        disposed = true
        stop()
        scope.cancel()
    }
}

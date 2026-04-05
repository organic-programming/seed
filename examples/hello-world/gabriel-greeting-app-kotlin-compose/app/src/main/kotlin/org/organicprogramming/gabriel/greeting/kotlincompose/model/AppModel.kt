package org.organicprogramming.gabriel.greeting.kotlincompose.model

import com.google.gson.Gson
import com.google.gson.JsonSyntaxException
import org.organicprogramming.holons.HolonRef
import java.net.URI

private val gson = Gson()

data class LanguageOption(
    val code: String,
    val name: String,
    val nativeName: String,
)

enum class CoaxSurfaceState(val badgeTitle: String) {
    OFF("OFF"),
    SAVED("SAVED"),
    ANNOUNCED("ANNOUNCED"),
    LIVE("LIVE"),
    ERROR("ERROR"),
}

enum class CoaxServerTransport(val title: String, val defaultPort: Int) {
    TCP("TCP", 60000),
    UNIX("Unix socket", 0),
    ;

    companion object {
        fun fromRaw(value: String): CoaxServerTransport =
            when (value.trim().lowercase()) {
                "unix" -> UNIX
                else -> TCP
            }
    }
}

data class CoaxSettingsSnapshot(
    val serverEnabled: Boolean,
    val serverTransport: CoaxServerTransport,
    val serverHost: String,
    val serverPortText: String,
    val serverUnixPath: String,
) {
    fun encode(): String = gson.toJson(this)

    companion object {
        const val defaultHost = "127.0.0.1"
        const val defaultUnixPath = "/tmp/gabriel-greeting-coax.sock"

        val defaults = CoaxSettingsSnapshot(
            serverEnabled = true,
            serverTransport = CoaxServerTransport.TCP,
            serverHost = defaultHost,
            serverPortText = "60000",
            serverUnixPath = defaultUnixPath,
        )

        fun decode(value: String): CoaxSettingsSnapshot {
            if (value.trim().isEmpty()) {
                return defaults
            }
            return try {
                gson.fromJson(value, CoaxSettingsSnapshot::class.java) ?: defaults
            } catch (_: JsonSyntaxException) {
                defaults
            }
        }
    }
}

data class CoaxSurfaceStatus(
    val id: String,
    val title: String,
    val endpoint: String?,
    val state: CoaxSurfaceState,
)

data class GabrielHolonIdentity(
    val slug: String,
    val familyName: String,
    val binaryName: String,
    val buildRunner: String,
    val displayName: String,
    val sortRank: Int,
    val holonUuid: String,
    val born: String,
    val sourceKind: String,
    val discoveryPath: String,
    val hasSource: Boolean,
) {
    val variant: String
        get() = slug.removePrefix("gabriel-greeting-")

    companion object {
        private val excludedSlugs = setOf(
            "gabriel-greeting-app-swiftui",
            "gabriel-greeting-app-flutter",
            "gabriel-greeting-app-kotlin-compose",
        )

        private val sortOrder = mapOf(
            "gabriel-greeting-swift" to 0,
            "gabriel-greeting-go" to 1,
            "gabriel-greeting-rust" to 2,
            "gabriel-greeting-python" to 3,
            "gabriel-greeting-c" to 4,
            "gabriel-greeting-cpp" to 5,
            "gabriel-greeting-csharp" to 6,
            "gabriel-greeting-dart" to 7,
            "gabriel-greeting-java" to 8,
            "gabriel-greeting-kotlin" to 9,
            "gabriel-greeting-node" to 10,
            "gabriel-greeting-ruby" to 11,
        )

        fun fromDiscovered(ref: HolonRef): GabrielHolonIdentity? {
            val info = ref.info ?: return null
            val slug = info.slug.trim()
            if (!slug.startsWith("gabriel-greeting-") || slug in excludedSlugs) {
                return null
            }

            val entrypoint = info.entrypoint.trim()
            return GabrielHolonIdentity(
                slug = slug,
                familyName = info.identity.familyName.trim(),
                binaryName = entrypoint.substringAfterLast('/').substringAfterLast('\\').ifBlank { slug },
                buildRunner = info.runner.trim(),
                displayName = displayNameFor(slug),
                sortRank = sortRankFor(slug),
                holonUuid = info.uuid.trim(),
                born = "",
                sourceKind = sourceKindForUrl(ref.url),
                discoveryPath = discoveryPathFromUrl(ref.url),
                hasSource = info.hasSource,
            )
        }

        fun displayNameFor(slug: String): String =
            when (slug.removePrefix("gabriel-greeting-")) {
                "cpp" -> "Gabriel (C++)"
                "csharp" -> "Gabriel (C#)"
                "node" -> "Gabriel (Node.js)"
                else -> {
                    val variant = slug.removePrefix("gabriel-greeting-")
                        .split('-')
                        .filter { it.isNotBlank() }
                        .joinToString(" ") { token ->
                            token.replaceFirstChar { char -> char.uppercase() }
                        }
                    "Gabriel ($variant)"
                }
            }

        fun sortRankFor(slug: String): Int = sortOrder[slug] ?: 999

        private fun discoveryPathFromUrl(url: String): String =
            runCatching {
                val uri = URI.create(url)
                if (uri.scheme == "file") {
                    uri.path
                } else {
                    url
                }
            }.getOrDefault(url)

        private fun sourceKindForUrl(url: String): String {
            val path = discoveryPathFromUrl(url)
            return when {
                path.contains("/.op/build/") -> "built"
                path.contains("/Holons/") -> "siblings"
                else -> "source"
            }
        }
    }
}

data class AppPlatformCapabilities(
    val supportsUnixSockets: Boolean,
) {
    val appTransports: List<String>
        get() = if (supportsUnixSockets) listOf("stdio", "unix", "tcp") else listOf("stdio", "tcp")

    val coaxServerTransports: List<CoaxServerTransport>
        get() = if (supportsUnixSockets) CoaxServerTransport.entries else listOf(CoaxServerTransport.TCP)

    companion object {
        fun desktopCurrent(): AppPlatformCapabilities {
            val osName = System.getProperty("os.name", "").lowercase()
            return AppPlatformCapabilities(supportsUnixSockets = !osName.contains("win"))
        }
    }
}

data class GreetingUiState(
    val isRunning: Boolean = false,
    val isLoading: Boolean = true,
    val isGreeting: Boolean = false,
    val connectionError: String? = null,
    val error: String? = null,
    val greeting: String = "",
    val userName: String = "World",
    val selectedLanguageCode: String = "",
    val transport: String = "stdio",
    val availableLanguages: List<LanguageOption> = emptyList(),
    val availableHolons: List<GabrielHolonIdentity> = emptyList(),
    val selectedHolon: GabrielHolonIdentity? = null,
) {
    val statusTitle: String
        get() = when {
            isLoading -> "Starting holon..."
            isRunning -> "Ready"
            else -> "Offline"
        }
}

data class CoaxUiState(
    val isEnabled: Boolean,
    val snapshot: CoaxSettingsSnapshot,
    val capabilities: AppPlatformCapabilities,
    val listenUri: String? = null,
    val statusDetail: String? = null,
) {
    val serverEnabled: Boolean
        get() = snapshot.serverEnabled

    val serverTransport: CoaxServerTransport
        get() = snapshot.serverTransport

    val serverHost: String
        get() = snapshot.serverHost

    val serverPortText: String
        get() = snapshot.serverPortText

    val serverUnixPath: String
        get() = snapshot.serverUnixPath

    val serverPort: Int
        get() = sanitizedPort(serverPortText, serverTransport.defaultPort)

    val serverPortValidationMessage: String?
        get() = when {
            serverTransport != CoaxServerTransport.TCP -> null
            serverPortText.trim().isEmpty() -> "Empty port. Falling back to ${serverTransport.defaultPort}."
            serverPort !in 1..65535 -> "Invalid port. Falling back to ${serverTransport.defaultPort}."
            else -> null
        }

    val serverPreviewEndpoint: String
        get() = when (serverTransport) {
            CoaxServerTransport.TCP -> {
                val host = serverHost.trim().ifEmpty { CoaxSettingsSnapshot.defaultHost }
                "tcp://$host:$serverPort"
            }
            CoaxServerTransport.UNIX -> {
                val path = serverUnixPath.trim().ifEmpty { CoaxSettingsSnapshot.defaultUnixPath }
                "unix://$path"
            }
        }

    val serverStatus: CoaxSurfaceStatus
        get() = CoaxSurfaceStatus(
            id = "server",
            title = "Server",
            endpoint = if (serverEnabled) (listenUri ?: serverPreviewEndpoint) else serverPreviewEndpoint,
            state = when {
                !serverEnabled -> CoaxSurfaceState.OFF
                statusDetail != null && isEnabled -> CoaxSurfaceState.ERROR
                listenUri != null -> CoaxSurfaceState.LIVE
                !isEnabled -> CoaxSurfaceState.SAVED
                else -> CoaxSurfaceState.ANNOUNCED
            },
        )
}

fun normalizedTransportSelection(value: String?): String =
    when (value?.trim()?.lowercase().orEmpty()) {
        "", "auto", "stdio", "stdio://" -> "stdio"
        "unix", "unix://" -> "unix"
        "tcp", "tcp://" -> "tcp"
        else -> "stdio"
    }

fun transportTitle(value: String): String = normalizedTransportSelection(value)

fun sanitizedPort(value: String, fallback: Int): Int {
    val parsed = value.trim().toIntOrNull()
    return if (parsed == null || parsed !in 1..65535) fallback else parsed
}

fun preferredHolon(holons: Iterable<GabrielHolonIdentity>): GabrielHolonIdentity? =
    holons.minByOrNull { it.sortRank }

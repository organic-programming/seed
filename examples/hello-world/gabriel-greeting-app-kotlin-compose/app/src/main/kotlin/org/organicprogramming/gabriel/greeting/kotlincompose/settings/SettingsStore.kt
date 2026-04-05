package org.organicprogramming.gabriel.greeting.kotlincompose.settings

import com.google.gson.GsonBuilder
import com.google.gson.reflect.TypeToken
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxServerTransport
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxSettingsSnapshot
import java.io.File

private const val COAX_ENABLED_KEY = "coax.server.enabled"
private const val COAX_SETTINGS_KEY = "coax.server.settings"
private const val COAX_SERVER_ENABLED_ENV = "OP_COAX_SERVER_ENABLED"
private const val COAX_SERVER_LISTEN_URI_ENV = "OP_COAX_SERVER_LISTEN_URI"

private val prettyGson = GsonBuilder().setPrettyPrinting().create()
private val mapType = object : TypeToken<MutableMap<String, Any?>>() {}.type

interface SettingsStore {
    fun readBool(key: String, defaultValue: Boolean = false): Boolean
    fun readString(key: String, defaultValue: String = ""): String
    fun writeBool(key: String, value: Boolean)
    fun writeString(key: String, value: String)
}

class FileSettingsStore private constructor(
    private val file: File,
    private val values: MutableMap<String, Any?>,
) : SettingsStore {
    override fun readBool(key: String, defaultValue: Boolean): Boolean =
        values[key] as? Boolean ?: defaultValue

    override fun readString(key: String, defaultValue: String): String =
        values[key] as? String ?: defaultValue

    override fun writeBool(key: String, value: Boolean) {
        values[key] = value
        flush()
    }

    override fun writeString(key: String, value: String) {
        values[key] = value
        flush()
    }

    private fun flush() {
        file.writeText(prettyGson.toJson(values) + "\n")
    }

    companion object {
        fun create(): FileSettingsStore {
            val directory = File(settingsDirectoryPath())
            directory.mkdirs()
            val file = File(directory, "settings.json")
            val values = if (file.exists()) {
                runCatching {
                    @Suppress("UNCHECKED_CAST")
                    prettyGson.fromJson<MutableMap<String, Any?>>(file.readText(), mapType) ?: mutableMapOf()
                }.getOrElse { mutableMapOf() }
            } else {
                mutableMapOf()
            }
            return FileSettingsStore(file, values)
        }

        private fun settingsDirectoryPath(): String {
            val home = System.getenv("HOME").orEmpty().trim()
            val appData = System.getenv("APPDATA").orEmpty().trim()
            val xdgConfig = System.getenv("XDG_CONFIG_HOME").orEmpty().trim()
            val osName = System.getProperty("os.name", "").lowercase()

            return when {
                osName.contains("win") && appData.isNotEmpty() ->
                    "$appData/Organic Programming/Gabriel Greeting App Kotlin Compose"
                osName.contains("mac") && home.isNotEmpty() ->
                    "$home/Library/Application Support/Organic Programming/Gabriel Greeting App Kotlin Compose"
                osName.contains("linux") && xdgConfig.isNotEmpty() ->
                    "$xdgConfig/organic-programming/gabriel-greeting-app-kotlin-compose"
                osName.contains("linux") && home.isNotEmpty() ->
                    "$home/.config/organic-programming/gabriel-greeting-app-kotlin-compose"
                else -> "${System.getProperty("user.dir")}/.gabriel-greeting-app-kotlin-compose"
            }
        }
    }
}

class MemorySettingsStore : SettingsStore {
    private val values = mutableMapOf<String, Any?>()

    override fun readBool(key: String, defaultValue: Boolean): Boolean =
        values[key] as? Boolean ?: defaultValue

    override fun readString(key: String, defaultValue: String): String =
        values[key] as? String ?: defaultValue

    override fun writeBool(key: String, value: Boolean) {
        values[key] = value
    }

    override fun writeString(key: String, value: String) {
        values[key] = value
    }
}

fun applyLaunchEnvironmentOverrides(
    store: SettingsStore,
    environment: Map<String, String> = System.getenv(),
) {
    val listenUri = environment[COAX_SERVER_LISTEN_URI_ENV].orEmpty().trim()
    val enabled = parseBoolOverride(environment[COAX_SERVER_ENABLED_ENV])

    if (listenUri.isEmpty() && enabled == null) {
        return
    }

    if (listenUri.isNotEmpty()) {
        store.writeString(COAX_SETTINGS_KEY, snapshotFromListenUri(listenUri).encode())
    }

    when {
        enabled != null -> store.writeBool(COAX_ENABLED_KEY, enabled)
        listenUri.isNotEmpty() -> store.writeBool(COAX_ENABLED_KEY, true)
    }
}

fun coaxEnabledKey(): String = COAX_ENABLED_KEY

fun coaxSettingsKey(): String = COAX_SETTINGS_KEY

private fun parseBoolOverride(value: String?): Boolean? =
    when (value.orEmpty().trim().lowercase()) {
        "1", "true", "yes", "on" -> true
        "0", "false", "no", "off" -> false
        else -> null
    }

private fun snapshotFromListenUri(listenUri: String): CoaxSettingsSnapshot {
    val trimmed = listenUri.trim()
    return when {
        trimmed.startsWith("unix://") -> CoaxSettingsSnapshot(
            serverEnabled = true,
            serverTransport = CoaxServerTransport.UNIX,
            serverHost = CoaxSettingsSnapshot.defaultHost,
            serverPortText = CoaxSettingsSnapshot.defaults.serverPortText,
            serverUnixPath = trimmed.removePrefix("unix://"),
        )
        trimmed.startsWith("tcp://") -> {
            val uri = java.net.URI.create(trimmed)
            val host = uri.host?.trim().orEmpty().ifEmpty { CoaxSettingsSnapshot.defaultHost }
            val port = if (uri.port > 0) uri.port.toString() else CoaxSettingsSnapshot.defaults.serverPortText
            CoaxSettingsSnapshot(
                serverEnabled = true,
                serverTransport = CoaxServerTransport.TCP,
                serverHost = host,
                serverPortText = port,
                serverUnixPath = CoaxSettingsSnapshot.defaultUnixPath,
            )
        }
        else -> CoaxSettingsSnapshot.defaults
    }
}

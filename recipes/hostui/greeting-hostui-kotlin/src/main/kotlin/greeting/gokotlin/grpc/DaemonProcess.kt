package greeting.gokotlin.grpc

import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlinx.coroutines.runBlocking
import kotlin.io.path.absolutePathString
import kotlin.io.path.exists
import org.organicprogramming.holons.Connect

class DaemonProcess {
    companion object {
        private const val packagedHolonSlug = "greeting-daemon"
        private const val packagedHolonUUID = "6492d55a-55b8-4ecb-a406-2a2a401f7c01"
        private const val packagedFamilyName = "daemon"
        private const val packagedDaemonBinaryName = "gudule-greeting-daemon"
        private const val devDaemonBinaryName = "gudule-daemon-greeting-go"
    }

    private var client: GreetingClient? = null
    private var stageRoot: Path? = null

    fun client(): GreetingClient {
        if (client == null) {
            start()
        }
        return requireNotNull(client)
    }

    fun start() {
        if (client != null) {
            return
        }

        val overrideTarget = System.getenv("GUDULE_DAEMON_TARGET")?.trim().orEmpty()
        if (overrideTarget.isNotEmpty()) {
            val channel = runBlocking { Connect.connect(overrideTarget) }
            client = GreetingClient(channel)
            return
        }

        val daemonPath = resolveDaemonPath()
        val root = stageHolonRoot(daemonPath)
        stageRoot = root

        val previousUserDir = System.getProperty("user.dir")
        try {
            System.setProperty("user.dir", root.toString())
            val channel = runBlocking { Connect.connect(packagedHolonSlug) }
            client = GreetingClient(channel)
        } catch (t: Throwable) {
            stageRoot = null
            deleteStageRoot(root)
            throw t
        } finally {
            if (previousUserDir == null) {
                System.clearProperty("user.dir")
            } else {
                System.setProperty("user.dir", previousUserDir)
            }
        }
    }

    fun stop() {
        val root = stageRoot
        stageRoot = null
        val activeClient = client
        client = null

        try {
            activeClient?.close()
        } finally {
            if (root != null) {
                deleteStageRoot(root)
            }
        }
    }

    private fun resolveDaemonPath(): String {
        val candidates = listOf(
            Path.of(packagedDaemonBinaryName),
            Path.of("../../daemons/greeting-daemon-go/.op/build/bin/$devDaemonBinaryName"),
            Path.of("../../daemons/greeting-daemon-go/$devDaemonBinaryName"),
        )
        for (candidate in candidates) {
            if (candidate.exists()) {
                return candidate.normalize().absolutePathString()
            }
        }

        val resource = javaClass.classLoader.getResourceAsStream("embedded/$packagedDaemonBinaryName")
            ?: error("Embedded daemon resource is missing")

        val tempFile = Files.createTempFile("gudule-greeting-daemon", "")
        resource.use { input ->
            Files.copy(input, tempFile, java.nio.file.StandardCopyOption.REPLACE_EXISTING)
        }
        File(tempFile.toUri()).setExecutable(true)
        tempFile.toFile().deleteOnExit()
        return tempFile.absolutePathString()
    }

    private fun stageHolonRoot(binaryPath: String): Path {
        val root = Files.createTempDirectory("greeting-daemon-stage-")
        val holonDir = root.resolve("holons").resolve(packagedHolonSlug)
        Files.createDirectories(holonDir)
        Files.writeString(holonDir.resolve("holon.yaml"), manifestFor(binaryPath))
        return root
    }

    private fun manifestFor(binaryPath: String): String = """
schema: holon/v0
uuid: "$packagedHolonUUID"
given_name: greeting
family_name: "$packagedFamilyName"
motto: Packaged greeting daemon fallback.
composer: Codex
clade: deterministic/pure
status: draft
born: "2026-03-11"
generated_by: codex
kind: native
build:
  runner: recipe
artifacts:
  binary: "${yamlEscape(binaryPath)}"
""".trimIndent() + "\n"

    private fun yamlEscape(value: String): String =
        value.replace("\\", "\\\\").replace("\"", "\\\"")

    private fun deleteStageRoot(root: Path) {
        runCatching {
            val directory = root.toFile()
            if (directory.exists()) {
                directory.deleteRecursively()
            }
        }
    }
}

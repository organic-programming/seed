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
        private const val holonSlug = "greeting-daemon-greeting-gokotlin"
        private const val holonUUID = "1a409a1e-69e3-4846-9f9b-47b0a6f98f84"
        private const val familyName = "Greeting-Gokotlin"
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

        val daemonPath = resolveDaemonPath()
        val root = stageHolonRoot(daemonPath)
        stageRoot = root

        val previousUserDir = System.getProperty("user.dir")
        try {
            System.setProperty("user.dir", root.toString())
            val channel = runBlocking { Connect.connect(holonSlug) }
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
        val devBinary = Path.of("../greeting-daemon/gudule-daemon-greeting-gokotlin")
        if (devBinary.exists()) {
            return devBinary.normalize().absolutePathString()
        }

        val resource = javaClass.classLoader.getResourceAsStream("embedded/gudule-daemon-greeting-gokotlin")
            ?: error("Embedded daemon resource is missing")

        val tempFile = Files.createTempFile("gudule-daemon-gokotlin", "")
        resource.use { input ->
            Files.copy(input, tempFile, java.nio.file.StandardCopyOption.REPLACE_EXISTING)
        }
        File(tempFile.toUri()).setExecutable(true)
        tempFile.toFile().deleteOnExit()
        return tempFile.absolutePathString()
    }

    private fun stageHolonRoot(binaryPath: String): Path {
        val root = Files.createTempDirectory("greeting-gokotlin-holon-")
        val holonDir = root.resolve("holons").resolve(holonSlug)
        Files.createDirectories(holonDir)
        Files.writeString(holonDir.resolve("holon.yaml"), manifestFor(binaryPath))
        return root
    }

    private fun manifestFor(binaryPath: String): String = """
schema: holon/v0
uuid: "$holonUUID"
given_name: greeting-daemon
family_name: "$familyName"
motto: Greets users in 56 languages — a Gokotlin recipe example.
composer: B. ALTER
clade: deterministic/pure
status: draft
born: "2026-02-20"
generated_by: manual
kind: native
build:
  runner: go-module
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

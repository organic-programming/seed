package greeting.gokotlin.grpc

import java.io.BufferedReader
import java.io.File
import java.io.InputStream
import java.io.InputStreamReader
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.time.Duration
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.runBlocking
import kotlin.io.path.absolutePathString
import kotlin.io.path.exists
import kotlin.io.path.isRegularFile
import org.organicprogramming.holons.Connect
import org.organicprogramming.holons.ConnectOptions

private data class GreetingDaemonIdentity(
    val slug: String,
    val familyName: String,
    val binaryName: String,
    val buildRunner: String,
    val binaryPath: Path,
) {
    companion object {
        private const val binaryPrefix = "gudule-daemon-greeting-"

        fun fromBinaryPath(path: Path): GreetingDaemonIdentity? =
            fromBinaryName(path.fileName.toString(), path)

        fun fromBinaryName(binaryName: String, binaryPath: Path): GreetingDaemonIdentity? {
            val normalized = binaryName.removeSuffix(".exe")
            if (!normalized.startsWith(binaryPrefix)) {
                return null
            }

            val variant = normalized.removePrefix(binaryPrefix)
            return GreetingDaemonIdentity(
                slug = "gudule-greeting-daemon-$variant",
                familyName = "Greeting-Daemon-${displayVariant(variant)}",
                binaryName = normalized,
                buildRunner = buildRunnerFor(variant),
                binaryPath = binaryPath.toAbsolutePath().normalize(),
            )
        }

        private fun displayVariant(variant: String): String {
            val overrides = mapOf(
                "cpp" to "CPP",
                "js" to "JS",
                "qt" to "Qt",
            )
            return variant.split('-').filter { it.isNotEmpty() }.joinToString("-") { token ->
                overrides[token] ?: token.replaceFirstChar { it.uppercase() }
            }
        }

        private fun buildRunnerFor(variant: String): String = when (variant) {
            "go" -> "go-module"
            "rust" -> "cargo"
            "swift" -> "swift-package"
            "kotlin" -> "gradle"
            "dart" -> "dart"
            "python" -> "python"
            "csharp" -> "dotnet"
            "node" -> "npm"
            else -> "go-module"
        }
    }
}

private data class BundledDaemonSession(
    val process: Process,
    val drainThread: Thread,
) {
    fun stop() {
        process.destroy()
        if (!process.waitFor(500, TimeUnit.MILLISECONDS)) {
            process.destroyForcibly()
            process.waitFor(2, TimeUnit.SECONDS)
        }
        drainThread.interrupt()
        drainThread.join(200)
    }
}

class DaemonProcess {
    companion object {
        private const val holonUUID = "951bf1cb-6e4f-4b32-ae34-97a033a5848b"
    }

    private var client: GreetingClient? = null
    private var stageRoot: Path? = null
    private var daemonSession: BundledDaemonSession? = null

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

        val daemon = resolveDaemon()
        val root = stageHolonRoot(daemon)
        val portFile = portFilePath(root, daemon.slug)
        val session = startBundledDaemon(daemon.binaryPath, portFile)
        stageRoot = root
        daemonSession = session

        val previousUserDir = System.getProperty("user.dir")
        try {
            System.setProperty("user.dir", root.toString())
            val channel = runBlocking {
                Connect.connect(
                    daemon.slug,
                    ConnectOptions(
                        transport = "tcp",
                        start = false,
                        portFile = portFile,
                    ),
                )
            }
            client = GreetingClient(channel)
        } catch (t: Throwable) {
            daemonSession = null
            session.stop()
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
        val session = daemonSession
        daemonSession = null
        val activeClient = client
        client = null

        try {
            activeClient?.close()
        } finally {
            runCatching { session?.stop() }
            if (root != null) {
                deleteStageRoot(root)
            }
        }
    }

    private fun resolveDaemon(): GreetingDaemonIdentity {
        val candidates = daemonCandidates().mapNotNull { GreetingDaemonIdentity.fromBinaryPath(it) }
        return candidates.firstOrNull { it.binaryPath.exists() && it.binaryPath.isRegularFile() }
            ?: error("Bundled daemon binary not found")
    }

    private fun daemonCandidates(): List<Path> {
        val results = linkedSetOf<Path>()
        val userDir = Path.of(System.getProperty("user.dir", ".")).toAbsolutePath().normalize()

        addBundledBinaries(results, userDir.resolve("build"))
        addBundledBinaries(results, userDir.resolve("../build").normalize())
        addSourceTreeDaemons(results, userDir.resolve("../../daemons").normalize())

        val codeSource = runCatching { Path.of(javaClass.protectionDomain.codeSource.location.toURI()) }.getOrNull()
        if (codeSource != null) {
            val codeDir = if (Files.isDirectory(codeSource)) codeSource else codeSource.parent
            if (codeDir != null) {
                addBundledBinaries(results, codeDir)
                addBundledBinaries(results, codeDir.resolve("daemon"))
                addBundledBinaries(results, codeDir.resolve("../daemon").normalize())
                addBundledBinaries(results, codeDir.resolve("../Resources").normalize())
                addBundledBinaries(results, codeDir.resolve("../Resources/daemon").normalize())
            }
        }

        return results.toList().sortedBy { it.toString() }
    }

    private fun addBundledBinaries(results: MutableSet<Path>, directory: Path) {
        if (!Files.isDirectory(directory)) {
            return
        }
        Files.list(directory).use { stream ->
            stream
                .filter { Files.isRegularFile(it) }
                .filter { it.fileName.toString().startsWith("gudule-daemon-greeting-") }
                .forEach { results.add(it.toAbsolutePath().normalize()) }
        }
    }

    private fun addSourceTreeDaemons(results: MutableSet<Path>, daemonsDir: Path) {
        if (!Files.isDirectory(daemonsDir)) {
            return
        }
        Files.list(daemonsDir).use { stream ->
            stream
                .filter { Files.isDirectory(it) }
                .filter { it.fileName.toString().startsWith("gudule-daemon-greeting-") }
                .forEach { daemonDir ->
                    val binaryName = daemonDir.fileName.toString()
                    results.add(
                        daemonDir.resolve(".op").resolve("build").resolve("bin").resolve(binaryName)
                            .toAbsolutePath().normalize(),
                    )
                    results.add(daemonDir.resolve(binaryName).toAbsolutePath().normalize())
                }
        }
    }

    private fun stageHolonRoot(daemon: GreetingDaemonIdentity): Path {
        val root = Files.createTempDirectory("gudule-greeting-hostui-compose-")
        val holonDir = root.resolve("holons").resolve(daemon.slug)
        Files.createDirectories(holonDir)
        Files.writeString(holonDir.resolve("holon.yaml"), manifestFor(daemon))
        return root
    }

    private fun portFilePath(root: Path, daemonSlug: String): Path =
        root.resolve(".op").resolve("run").resolve("$daemonSlug.port")

    private fun manifestFor(daemon: GreetingDaemonIdentity): String = """
schema: holon/v0
uuid: "$holonUUID"
given_name: gudule
family_name: "${daemon.familyName}"
motto: Greets users in 56 languages through the bundled daemon.
composer: Codex
clade: deterministic/pure
status: draft
born: "2026-03-12"
generated_by: manual
kind: native
build:
  runner: ${daemon.buildRunner}
artifacts:
  binary: "${yamlEscape(daemon.binaryPath.absolutePathString())}"
""".trimIndent() + "\n"

    private fun startBundledDaemon(binaryPath: Path, portFile: Path): BundledDaemonSession {
        val queue = LinkedBlockingQueue<String>()
        val process = ProcessBuilder(
            binaryPath.absolutePathString(),
            "serve",
            "--listen",
            "tcp://127.0.0.1:0",
        )
            .redirectErrorStream(true)
            .start()

        val drainThread = Thread {
            BufferedReader(InputStreamReader(process.inputStream, StandardCharsets.UTF_8)).use { reader ->
                reader.lineSequence().forEach { line ->
                    queue.offer(line)
                }
            }
        }.apply {
            isDaemon = true
            start()
        }

        val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
        var listenUri: String? = null
        val recentLines = ArrayDeque<String>()

        while (System.nanoTime() < deadline && listenUri == null) {
            val line = queue.poll(100, TimeUnit.MILLISECONDS)
            if (line != null) {
                if (recentLines.size == 8) {
                    recentLines.removeFirst()
                }
                recentLines.addLast(line)
                listenUri = firstUri(line)
            }
            if (!process.isAlive && listenUri == null) {
                val details = if (recentLines.isEmpty()) "" else ": ${recentLines.joinToString(" | ")}"
                throw IllegalStateException("Bundled daemon exited before advertising an address${details}")
            }
        }

        val advertised = listenUri ?: throw IllegalStateException("Bundled daemon did not advertise a tcp:// address")
        writePortFile(portFile, advertised)
        return BundledDaemonSession(process, drainThread)
    }

    private fun writePortFile(portFile: Path, uri: String) {
        Files.createDirectories(portFile.parent)
        Files.writeString(portFile, uri.trim() + "\n")
    }

    private fun firstUri(line: String): String? =
        """(tcp://[^\s]+)""".toRegex().find(line)?.groupValues?.get(1)

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

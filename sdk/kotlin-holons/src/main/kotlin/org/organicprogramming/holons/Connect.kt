package org.organicprogramming.holons

import io.grpc.ConnectivityState
import io.grpc.ManagedChannel
import io.grpc.ManagedChannelBuilder
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.BufferedReader
import java.io.Closeable
import java.io.File
import java.io.IOException
import java.io.InputStream
import java.io.InputStreamReader
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.nio.charset.StandardCharsets
import java.nio.channels.Channels
import java.nio.channels.SocketChannel
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.util.Collections
import java.util.IdentityHashMap
import java.util.Optional
import java.util.concurrent.BlockingQueue
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit

data class ConnectOptions(
    val timeout: Duration = Duration.ofSeconds(5),
    val transport: String = "stdio",
    val start: Boolean = true,
    val portFile: Path? = null,
)

object Connect {
    private data class StartedHandle(
        val process: Process?,
        val closeable: Closeable?,
        val ephemeral: Boolean,
    )

    private data class StartedProcess(
        val uri: String,
        val process: Process,
        val closeable: Closeable? = null,
    )

    private data class DialedChannel(
        val channel: ManagedChannel,
        val closeable: Closeable? = null,
    )

    private data class HostPort(
        val host: String,
        val port: Int,
    )

    private val started = Collections.synchronizedMap(IdentityHashMap<ManagedChannel, StartedHandle>())

    suspend fun connect(target: String): ManagedChannel = withContext(Dispatchers.IO) {
        connectInternal(target, ConnectOptions(), defaultEphemeral = true)
    }

    suspend fun connect(target: String, options: ConnectOptions): ManagedChannel = withContext(Dispatchers.IO) {
        connectInternal(target, options, defaultEphemeral = false)
    }

    suspend fun disconnect(channel: ManagedChannel?) = withContext(Dispatchers.IO) {
        if (channel == null) return@withContext

        val handle = synchronized(started) { started.remove(channel) }
        channel.shutdownNow()
        runCatching { channel.awaitTermination(2, TimeUnit.SECONDS) }

        closeQuietly(handle?.closeable)
        if (handle?.ephemeral == true) {
            stopProcess(handle.process)
        }
    }

    private fun connectInternal(target: String, options: ConnectOptions, defaultEphemeral: Boolean): ManagedChannel {
        val trimmed = target.trim()
        require(trimmed.isNotEmpty()) { "target is required" }

        val timeout = if (options.timeout.isZero || options.timeout.isNegative) Duration.ofSeconds(5) else options.timeout

        if (isDirectTarget(trimmed)) {
            return dialReady(normalizeDialTarget(trimmed), timeout).also { rememberDialedChannel(it, ephemeral = false) }.channel
        }

        val transport = options.transport.trim().ifEmpty { "stdio" }.lowercase()
        require(transport == "stdio" || transport == "tcp" || transport == "unix") { "unsupported transport \"$transport\"" }
        val ephemeral = defaultEphemeral || transport == "stdio"

        val entry = Discover.findBySlug(trimmed)
            ?: throw IllegalStateException("holon \"$trimmed\" not found")

        val portFile = options.portFile ?: defaultPortFilePath(entry.slug)
        val reusable = usablePortFile(portFile, timeout)
        if (reusable != null) {
            return dialReady(normalizeDialTarget(reusable), timeout).also { rememberDialedChannel(it, ephemeral = false) }.channel
        }
        check(options.start) { "holon \"$trimmed\" is not running" }

        val binaryPath = resolveBinaryPath(entry)
        val startedProcess = if (transport == "stdio") {
            startStdioHolon(binaryPath, timeout)
        } else if (transport == "unix") {
            startUnixHolon(binaryPath, entry.slug, portFile, timeout)
        } else {
            startTcpHolon(binaryPath, timeout)
        }

        val dialed = try {
            dialReady(normalizeDialTarget(startedProcess.uri), timeout)
        } catch (t: Throwable) {
            closeQuietly(startedProcess.closeable)
            stopProcess(startedProcess.process)
            throw t
        }
        val channel = dialed.channel

        if (!ephemeral && (transport == "tcp" || transport == "unix")) {
            try {
                writePortFile(portFile, startedProcess.uri)
            } catch (t: Throwable) {
                channel.shutdownNow()
                stopProcess(startedProcess.process)
                throw t
            }
        }

        synchronized(started) {
            started[channel] = StartedHandle(startedProcess.process, combineCloseables(startedProcess.closeable, dialed.closeable), ephemeral)
        }
        return channel
    }

    private fun dialReady(target: String, timeout: Duration): DialedChannel {
        val dialed = if (target.startsWith("unix://")) {
            val bridge = UnixBridge(target)
            val hostPort = parseHostPort(normalizeDialTarget(bridge.uri()))
            DialedChannel(ManagedChannelBuilder.forAddress(hostPort.host, hostPort.port).usePlaintext().build(), bridge)
        } else {
            val hostPort = parseHostPort(target)
            DialedChannel(ManagedChannelBuilder.forAddress(hostPort.host, hostPort.port).usePlaintext().build())
        }

        return try {
            waitForReady(dialed.channel, timeout)
            dialed
        } catch (t: Throwable) {
            dialed.channel.shutdownNow()
            closeQuietly(dialed.closeable)
            throw t
        }
    }

    private fun waitForReady(channel: ManagedChannel, timeout: Duration) {
        val deadline = System.nanoTime() + timeout.toNanos()
        var state = channel.getState(true)
        while (state != ConnectivityState.READY) {
            check(state != ConnectivityState.SHUTDOWN) { "gRPC channel shut down before becoming ready" }

            val remaining = deadline - System.nanoTime()
            check(remaining > 0) { "timed out waiting for gRPC readiness" }

            val latch = CountDownLatch(1)
            channel.notifyWhenStateChanged(state) { latch.countDown() }
            if (!latch.await(remaining, TimeUnit.NANOSECONDS)) {
                error("timed out waiting for gRPC readiness")
            }
            state = channel.getState(false)
        }
    }

    private fun usablePortFile(portFile: Path, timeout: Duration): String? {
        return try {
            val raw = Files.readString(portFile).trim()
            if (raw.isEmpty()) {
                Files.deleteIfExists(portFile)
                return null
            }

            val probe = dialReady(normalizeDialTarget(raw), minOf(timeout, Duration.ofSeconds(1)))
            probe.channel.shutdownNow()
            closeQuietly(probe.closeable)
            raw
        } catch (_: Throwable) {
            runCatching { Files.deleteIfExists(portFile) }
            null
        }
    }

    private fun startTcpHolon(binaryPath: String, timeout: Duration): StartedProcess {
        val process = ProcessBuilder(binaryPath, "serve", "--listen", "tcp://127.0.0.1:0").start()
        val lines: BlockingQueue<String> = LinkedBlockingQueue()
        val stderr = StringBuilder()

        startReader(process.inputStream, lines, null)
        startReader(process.errorStream, lines, stderr)

        val deadline = System.nanoTime() + timeout.toNanos()
        while (System.nanoTime() < deadline) {
            if (!process.isAlive) {
                throw IOException("holon exited before advertising an address: ${stderr.toString().trim()}")
            }

            val line = lines.poll(50, TimeUnit.MILLISECONDS) ?: continue
            val uri = firstUri(line)
            if (uri.isNotBlank()) {
                return StartedProcess(uri, process)
            }
        }

        stopProcess(process)
        throw IOException("timed out waiting for holon startup")
    }

    private fun startUnixHolon(binaryPath: String, slug: String, portFile: Path, timeout: Duration): StartedProcess {
        val uri = defaultUnixSocketUri(slug, portFile)
        val socketPath = Path.of(uri.removePrefix("unix://"))
        val process = ProcessBuilder(binaryPath, "serve", "--listen", uri).start()
        val stderr = StringBuilder()
        startDrainThread(process.errorStream, stderr, "holons-unix-connect-stderr")

        val deadline = System.nanoTime() + timeout.toNanos()
        while (System.nanoTime() < deadline) {
            if (Files.exists(socketPath)) {
                return StartedProcess(uri, process)
            }
            if (!process.isAlive) {
                throw IOException("holon exited before binding unix socket: ${stderr.toString().trim()}")
            }
            Thread.sleep(20)
        }

        stopProcess(process)
        val details = stderr.toString().trim()
        throw IOException("timed out waiting for unix holon startup" + if (details.isBlank()) "" else ": $details")
    }

    private fun startStdioHolon(binaryPath: String, timeout: Duration): StartedProcess {
        val process = ProcessBuilder(binaryPath, "serve", "--listen", "stdio://").start()
        val bridge = StdioBridge(process)
        val startupWindowMs = maxOf(1L, minOf(timeout.toMillis(), 200L))
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(startupWindowMs)

        while (System.nanoTime() < deadline) {
            if (!process.isAlive) {
                val stderr = bridge.stderrText()
                bridge.close()
                throw IOException("holon exited before stdio startup" + if (stderr.isBlank()) "" else ": $stderr")
            }
            Thread.sleep(10)
        }

        return StartedProcess(bridge.uri(), process, bridge)
    }

    private fun startReader(stream: InputStream, lines: BlockingQueue<String>, capture: StringBuilder?) {
        Thread {
            BufferedReader(InputStreamReader(stream, StandardCharsets.UTF_8)).use { reader ->
                while (true) {
                    val line = reader.readLine() ?: break
                    capture?.append(line)?.append('\n')
                    lines.offer(line)
                }
            }
        }.apply {
            isDaemon = true
            start()
        }
    }

    private fun resolveBinaryPath(entry: HolonEntry): String {
        val manifest = entry.manifest ?: error("holon \"${entry.slug}\" has no manifest")
        val binary = manifest.artifacts.binary.trim()
        require(binary.isNotEmpty()) { "holon \"${entry.slug}\" has no artifacts.binary" }

        val configured = Path.of(binary)
        if (configured.isAbsolute && Files.isRegularFile(configured)) {
            return configured.toString()
        }

        val candidate = entry.dir.resolve(".op").resolve("build").resolve("bin").resolve(configured.fileName)
        if (Files.isRegularFile(candidate)) {
            return candidate.toString()
        }

        val pathEnv = System.getenv("PATH") ?: ""
        pathEnv.split(File.pathSeparator).forEach { dir ->
            val resolved = Path.of(dir).resolve(configured.fileName.toString())
            if (Files.isRegularFile(resolved) && Files.isExecutable(resolved)) {
                return resolved.toString()
            }
        }

        error("built binary not found for holon \"${entry.slug}\"")
    }

    private fun defaultPortFilePath(slug: String): Path =
        Path.of(System.getProperty("user.dir", ".")).resolve(".op").resolve("run").resolve("$slug.port")

    private fun defaultUnixSocketUri(slug: String, portFile: Path): String {
        val label = socketLabel(slug)
        val hash = fnv1a64(portFile.toString()) and 0xFFFFFFFFFFFFL
        return "unix:///tmp/holons-$label-${hash.toString(16).padStart(12, '0')}.sock"
    }

    private fun socketLabel(slug: String): String {
        val label = StringBuilder()
        var lastDash = false

        for (ch in slug.trim().lowercase()) {
            when {
                ch in 'a'..'z' || ch in '0'..'9' -> {
                    label.append(ch)
                    lastDash = false
                }
                (ch == '-' || ch == '_') && label.isNotEmpty() && !lastDash -> {
                    label.append('-')
                    lastDash = true
                }
            }

            if (label.length >= 24) {
                break
            }
        }

        return label.toString().trim('-').ifEmpty { "socket" }
    }

    private fun fnv1a64(text: String): Long {
        var hash = -3750763034362895579L
        for (byte in text.toByteArray(StandardCharsets.UTF_8)) {
            hash = hash xor (byte.toLong() and 0xff)
            hash *= 1099511628211L
        }
        return hash
    }

    private fun writePortFile(portFile: Path, uri: String) {
        Files.createDirectories(portFile.parent)
        Files.writeString(portFile, uri.trim() + System.lineSeparator())
    }

    private fun stopProcess(process: Process?) {
        if (process == null || !process.isAlive) return

        process.destroy()
        if (!process.waitFor(2, TimeUnit.SECONDS)) {
            process.destroyForcibly()
            process.waitFor(2, TimeUnit.SECONDS)
        }
    }

    private fun isDirectTarget(target: String): Boolean =
        target.contains("://") || target.contains(':')

    private fun closeQuietly(closeable: Closeable?) {
        runCatching { closeable?.close() }
    }

    private fun rememberDialedChannel(dialed: DialedChannel, ephemeral: Boolean) {
        if (dialed.closeable == null) {
            return
        }
        synchronized(started) {
            started[dialed.channel] = StartedHandle(null, dialed.closeable, ephemeral)
        }
    }

    private fun combineCloseables(first: Closeable?, second: Closeable?): Closeable? {
        if (first == null) return second
        if (second == null) return first
        return Closeable {
            var firstError: Exception? = null
            try {
                first.close()
            } catch (e: Exception) {
                firstError = e
            }

            try {
                second.close()
            } catch (e: Exception) {
                firstError?.let { e.addSuppressed(it) }
                throw e
            }

            firstError?.let { throw it }
        }
    }

    private fun normalizeDialTarget(target: String): String {
        if (!target.contains("://")) return target

        val parsed = Transport.parseURI(target)
        return when (parsed.scheme) {
            "tcp" -> {
                val host = if (parsed.host.isNullOrBlank() || parsed.host == "0.0.0.0") "127.0.0.1" else parsed.host
                "$host:${parsed.port}"
            }
            "unix" -> "unix://${parsed.path}"
            else -> target
        }
    }

    private fun firstUri(line: String): String {
        for (field in line.split(Regex("\\s+"))) {
            val trimmed = field.trim().trim('"', '\'', '(', ')', '[', ']', '{', '}', '.', ',')
            if (trimmed.startsWith("tcp://") ||
                trimmed.startsWith("unix://") ||
                trimmed.startsWith("stdio://") ||
                trimmed.startsWith("ws://") ||
                trimmed.startsWith("wss://")
            ) {
                return trimmed
            }
        }
        return ""
    }

    private fun parseHostPort(target: String): HostPort {
        val index = target.lastIndexOf(':')
        require(index > 0 && index < target.length - 1) { "invalid host:port target \"$target\"" }
        return HostPort(target.substring(0, index), target.substring(index + 1).toInt())
    }

    private fun startDrainThread(stream: InputStream, capture: StringBuilder, name: String) {
        Thread({
            val buffer = ByteArray(4096)
            try {
                while (true) {
                    val read = stream.read(buffer)
                    if (read < 0) {
                        break
                    }
                    synchronized(capture) {
                        capture.append(String(buffer, 0, read, StandardCharsets.UTF_8))
                    }
                }
            } catch (_: IOException) {
                // Stream closed during shutdown.
            }
        }, name).apply {
            isDaemon = true
            start()
        }
    }

    private class StdioBridge(private val process: Process) : Closeable {
        private val listener = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"))
        private val stderr = StringBuilder()
        private val acceptThread = Thread(::acceptLoop, "holons-stdio-bridge-accept").apply {
            isDaemon = true
            start()
        }

        @Volatile
        private var socket: Socket? = null

        @Volatile
        private var closed = false

        init {
            startDrainThread(process.errorStream, stderr, "holons-stdio-bridge-stderr")
        }

        fun uri(): String = "tcp://127.0.0.1:${listener.localPort}"

        fun stderrText(): String = synchronized(stderr) { stderr.toString().trim() }

        override fun close() {
            closed = true
            runCatching { listener.close() }
            socket?.let { runCatching { it.close() } }
            socket = null
            runCatching { process.outputStream.close() }
            runCatching { process.inputStream.close() }
            runCatching { process.errorStream.close() }
            runCatching { acceptThread.join(200) }
        }

        private fun acceptLoop() {
            try {
                val accepted = listener.accept()
                if (closed) {
                    accepted.close()
                    return
                }
                socket = accepted

                val upstream = startPump(
                    accepted.getInputStream(),
                    process.outputStream,
                    closeOutput = true,
                    name = "holons-stdio-bridge-up",
                )
                val downstream = startPump(
                    process.inputStream,
                    accepted.getOutputStream(),
                    closeOutput = true,
                    name = "holons-stdio-bridge-down",
                )

                upstream.join()
                downstream.join()
            } catch (_: IOException) {
                // Listener/socket closed during shutdown.
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            } finally {
                socket?.let { runCatching { it.close() } }
                socket = null
            }
        }

        private fun startPump(input: InputStream, output: OutputStream, closeOutput: Boolean, name: String): Thread {
            return Thread({
                val buffer = ByteArray(16 * 1024)
                try {
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) {
                            break
                        }
                        output.write(buffer, 0, read)
                        output.flush()
                    }
                } catch (_: IOException) {
                    // Pipe/socket closed during shutdown.
                } finally {
                    if (closeOutput) {
                        runCatching { output.close() }
                    }
                }
            }, name).apply {
                isDaemon = true
                start()
            }
        }

        private fun startDrainThread(stream: InputStream, capture: StringBuilder, name: String) {
            Thread({
                val buffer = ByteArray(4096)
                try {
                    while (true) {
                        val read = stream.read(buffer)
                        if (read < 0) {
                            break
                        }
                        synchronized(capture) {
                            capture.append(String(buffer, 0, read, StandardCharsets.UTF_8))
                        }
                    }
                } catch (_: IOException) {
                    // Stream closed during shutdown.
                }
            }, name).apply {
                isDaemon = true
                start()
            }
        }
    }

    private class UnixBridge(private val target: String) : Closeable {
        private val listener = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"))
        private val acceptThread = Thread(::acceptLoop, "holons-unix-bridge-accept").apply {
            isDaemon = true
            start()
        }

        @Volatile
        private var closed = false

        @Volatile
        private var socket: Socket? = null

        @Volatile
        private var upstream: SocketChannel? = null

        fun uri(): String = "tcp://127.0.0.1:${listener.localPort}"

        override fun close() {
            closed = true
            runCatching { listener.close() }
            socket?.let { runCatching { it.close() } }
            socket = null
            upstream?.let { runCatching { it.close() } }
            upstream = null
            runCatching { acceptThread.join(200) }
        }

        private fun acceptLoop() {
            try {
                val accepted = listener.accept()
                if (closed) {
                    accepted.close()
                    return
                }

                val unixChannel = Transport.dialUnix(target)
                socket = accepted
                upstream = unixChannel

                val upstreamPump = startBridgePump(
                    accepted.getInputStream(),
                    Channels.newOutputStream(unixChannel),
                    closeOutput = true,
                    name = "holons-unix-bridge-up",
                )
                val downstreamPump = startBridgePump(
                    Channels.newInputStream(unixChannel),
                    accepted.getOutputStream(),
                    closeOutput = true,
                    name = "holons-unix-bridge-down",
                )

                upstreamPump.join()
                downstreamPump.join()
            } catch (_: IOException) {
                // Listener/socket closed during shutdown.
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            } finally {
                socket?.let { runCatching { it.close() } }
                socket = null
                upstream?.let { runCatching { it.close() } }
                upstream = null
            }
        }

        private fun startBridgePump(input: InputStream, output: OutputStream, closeOutput: Boolean, name: String): Thread {
            return Thread({
                val buffer = ByteArray(16 * 1024)
                try {
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) {
                            break
                        }
                        output.write(buffer, 0, read)
                        output.flush()
                    }
                } catch (_: IOException) {
                    // Bridge closed during shutdown.
                } finally {
                    if (closeOutput) {
                        runCatching { output.close() }
                    }
                }
            }, name).apply {
                isDaemon = true
                start()
            }
        }
    }
}

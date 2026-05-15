package org.organicprogramming.holons

import holons.v1.Observability as ObsProto
import io.grpc.CallOptions
import io.grpc.ManagedChannel
import io.grpc.stub.ClientCalls
import java.io.Closeable
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.nio.file.Files
import java.nio.file.Path
import java.security.SecureRandom
import java.time.Duration
import java.util.Locale
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

object Composite {
    private val random = SecureRandom()

    @JvmField
    val TRANSPORT_COVERAGE_SEQUENCE: List<String> =
        listOf("stdio", "stdio", "tcp", "unix", "tcp", "tcp", "stdio", "unix", "unix", "stdio")

    @JvmStatic
    @Throws(IOException::class)
    fun member(id: String): Path {
        val executable = System.getenv("OP_HOLON_EXECUTABLE")
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
            ?: ProcessHandle.current().info().command().orElse("")
        if (executable.isBlank()) {
            throw IOException("OP_HOLON_EXECUTABLE is not set")
        }
        return memberFromExecutable(Path.of(executable), id)
    }

    @JvmStatic
    @Throws(IOException::class)
    fun memberFromExecutable(executable: Path, id: String): Path {
        require(id.isNotBlank()) { "member id is required" }
        val memberDir = executable.toAbsolutePath().normalize().parent.resolve("holons").resolve(id)
        if (!Files.isDirectory(memberDir)) {
            throw IOException("member directory not found: $memberDir")
        }
        Files.list(memberDir).use { stream ->
            return stream
                .filter { Files.isRegularFile(it) }
                .filter { Files.isExecutable(it) || it.fileName.toString().endsWith(".exe") }
                .sorted()
                .findFirst()
                .orElseThrow { IOException("no executable found in $memberDir") }
        }
    }

    data class ChildSpec(val slug: String, val binary: String) {
        fun normalized(): ChildSpec = ChildSpec(slug.trim(), binary.trim())
    }

    fun interface DialOption {
        fun apply(options: DialOptions)
    }

    class DialOptions {
        var transitiveObservability: Boolean? = null
    }

    @JvmStatic
    fun withTransitiveObservability(enabled: Boolean): DialOption =
        DialOption { options -> options.transitiveObservability = enabled }

    class SpawnOptions {
        var slug: String = ""
        var binaryPath: String = ""
        var transport: String = "stdio"
        var instanceUid: String = ""
        var downstreamChain: List<ChildSpec> = emptyList()
        var extraEnv: Map<String, String> = emptyMap()
        var dialOptions: List<DialOption> = emptyList()
    }

    class SpawnedMember internal constructor(
        val slug: String,
        val uid: String,
        val listenUri: String,
        val conn: ManagedChannel,
        private val process: Process?,
        private val bridge: Closeable?,
        private val relay: Observability.MemberRelay?,
    ) : AutoCloseable {
        @Volatile private var stopped = false

        @Synchronized
        fun stop(timeout: Duration = Duration.ofSeconds(3)) {
            if (stopped) return
            stopped = true
            runCatching { relay?.close() }
            runBlocking { Connect.disconnect(conn) }
            closeQuietly(bridge)
            val active = process ?: return
            if (!active.isAlive) return
            active.destroy()
            try {
                val millis = timeout.toMillis().coerceAtLeast(1)
                if (!active.waitFor(millis, TimeUnit.MILLISECONDS)) {
                    active.destroyForcibly()
                    active.waitFor(millis, TimeUnit.MILLISECONDS)
                }
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
                active.destroyForcibly()
            }
        }

        override fun close() {
            stop()
        }
    }

    @JvmStatic
    @Throws(IOException::class)
    fun spawnMember(opts: SpawnOptions): SpawnedMember {
        val binary = opts.binaryPath.trim()
        val slug = opts.slug.trim().ifEmpty { Path.of(binary).fileName?.toString().orEmpty() }
        require(slug.isNotBlank()) { "spawn member: slug is required" }
        require(binary.isNotBlank()) { "spawn member $slug: binary path is required" }
        val uid = opts.instanceUid.trim().ifEmpty { newInstanceUid() }
        val transport = opts.transport.trim().ifEmpty { "stdio" }.lowercase(Locale.ROOT)
        val listenUri = listenUriForSpawn(transport, uid)
        if (listenUri.startsWith("unix://")) {
            Files.deleteIfExists(Path.of(listenUri.removePrefix("unix://")))
        }

        val command = mutableListOf(binary, "serve", "--listen", listenUri, "--transport", transport)
        for (child in opts.downstreamChain.map { it.normalized() }) {
            require(child.slug.isNotBlank() && child.binary.isNotBlank()) {
                "spawn member $slug: downstream child requires slug and binary"
            }
            command += "--child"
            command += "${child.slug}=${child.binary}"
        }

        val builder = ProcessBuilder(command)
        Path.of(binary).toAbsolutePath().normalize().parent?.let { builder.directory(it.toFile()) }
        val env = builder.environment()
        env.putAll(spawnEnvironment(uid, opts.extraEnv))

        val process = builder.start()
        var bridge: Closeable? = null
        val channel: ManagedChannel
        val publicUri: String
        try {
            if (transport == "stdio") {
                startDrainThread(process.errorStream, "holons-spawn-$slug-stderr")
                val stdio = StdioBridge(process)
                bridge = stdio
                publicUri = "stdio://"
                channel = runBlocking {
                    Connect.connect(
                        stdio.uri(),
                        ConnectOptions(timeout = Duration.ofSeconds(10), transport = "tcp", start = false),
                    )
                }
            } else {
                startDrainThread(process.inputStream, "holons-spawn-$slug-stdout")
                startDrainThread(process.errorStream, "holons-spawn-$slug-stderr")
                val meta = waitSpawnMeta(runRootFromEnv(env), slug, uid, Duration.ofSeconds(10))
                publicUri = meta.address
                channel = runBlocking {
                    Connect.connect(
                        publicUri,
                        ConnectOptions(timeout = Duration.ofSeconds(10), transport = "tcp", start = false),
                    )
                }
            }
            describeReady(channel, Duration.ofSeconds(10))
        } catch (error: Exception) {
            closeQuietly(bridge)
            stopProcess(process)
            if (error is IOException) throw error
            throw IOException("spawn member $slug: ${error.message}", error)
        }

        val dialOptions = applyDialOptions(opts.dialOptions)
        val transitive = dialOptions.transitiveObservability ?: true
        val relay = if (transitive) {
            Observability.MemberRelay(slug, uid, channel, Observability.current()).also { it.start() }
        } else {
            null
        }
        return SpawnedMember(slug, uid, publicUri, channel, process, bridge, relay)
    }

    class CascadeOptions {
        var transport: String = "stdio"
        var members: List<ChildSpec> = emptyList()
        var extraEnv: Map<String, String> = emptyMap()
    }

    class Cascade internal constructor(val top: SpawnedMember) : AutoCloseable {
        fun stop() {
            top.stop()
        }

        override fun close() {
            stop()
        }
    }

    @JvmStatic
    @Throws(IOException::class)
    fun buildCascade(opts: CascadeOptions): Cascade {
        require(opts.members.isNotEmpty()) { "build cascade: at least one member is required" }
        val top = opts.members.first().normalized()
        val spawn = SpawnOptions().apply {
            slug = top.slug
            binaryPath = top.binary
            transport = opts.transport
            downstreamChain = opts.members.drop(1)
            extraEnv = opts.extraEnv
        }
        return Cascade(spawnMember(spawn))
    }

    @JvmStatic
    @Throws(IOException::class)
    fun dial(address: String, vararg options: DialOption): ManagedChannel {
        val target = normalizeAddressForDial(address)
        val channel = runBlocking {
            Connect.connect(target, ConnectOptions(timeout = Duration.ofSeconds(10), transport = "tcp", start = false))
        }
        val dialOptions = applyDialOptions(options.asList())
        if (dialOptions.transitiveObservability == true) {
            val identity = Observability.resolveMemberIdentity(channel, "", "")
            Observability.MemberRelay(identity.slug, identity.uid, channel, Observability.current()).start()
        }
        return channel
    }

    data class ParsedChildFlags(val children: List<ChildSpec>, val remaining: Array<String>)

    @JvmStatic
    fun parseChildFlags(args: Array<String>): ParsedChildFlags {
        val children = mutableListOf<ChildSpec>()
        val remaining = mutableListOf<String>()
        var index = 0
        while (index < args.size) {
            val arg = args[index]
            when {
                arg == "--child" -> {
                    index += 1
                    require(index < args.size) { "--child requires <slug>=<binary>" }
                    children += parseChild(args[index])
                }
                arg.startsWith("--child=") -> children += parseChild(arg.removePrefix("--child="))
                else -> remaining += arg
            }
            index += 1
        }
        return ParsedChildFlags(children.toList(), remaining.toTypedArray())
    }

    private fun parseChild(raw: String): ChildSpec {
        val idx = raw.indexOf('=')
        require(idx >= 0) { "--child requires <slug>=<binary>" }
        val slug = raw.substring(0, idx).trim()
        val binary = raw.substring(idx + 1).trim()
        require(slug.isNotEmpty() && binary.isNotEmpty()) { "--child requires non-empty slug and binary" }
        return ChildSpec(slug, binary)
    }

    data class ChainHop(val slug: String, val instanceUid: String)
    data class CheckOutcome(val pass: Boolean, val evidence: String = "")

    class LogCheckOptions {
        var conn: ManagedChannel? = null
        var sender: String = ""
        var leafUid: String = ""
        var expectedChain: List<ChainHop> = emptyList()
        var timeout: Duration = Duration.ofSeconds(3)
        var pollInterval: Duration = Duration.ofMillis(100)
        var live: Boolean = false
    }

    class EventCheckOptions {
        var conn: ManagedChannel? = null
        var eventType: Observability.EventType = Observability.EventType.INSTANCE_READY
        var leafUid: String = ""
        var expectedChain: List<ChainHop> = emptyList()
        var timeout: Duration = Duration.ofSeconds(3)
        var pollInterval: Duration = Duration.ofMillis(100)
        var live: Boolean = false
    }

    @JvmStatic
    fun checkRelayedLog(opts: LogCheckOptions): CheckOutcome {
        val deadline = System.nanoTime() + opts.timeout.toNanos()
        var last = CheckOutcome(false)
        while (true) {
            try {
                last = matchRelayedLog(readLogEntries(opts.conn), opts)
                if (last.pass) return last
            } catch (error: Exception) {
                last = CheckOutcome(false, compactEvidence(error.message))
            }
            if (System.nanoTime() > deadline) return last
            sleep(opts.pollInterval.toMillis())
        }
    }

    @JvmStatic
    fun checkRelayedEvent(opts: EventCheckOptions): CheckOutcome {
        val deadline = System.nanoTime() + opts.timeout.toNanos()
        var last = CheckOutcome(false)
        while (true) {
            try {
                last = matchRelayedEvent(readEventEntries(opts.conn), opts)
                if (last.pass) return last
            } catch (error: Exception) {
                last = CheckOutcome(false, compactEvidence(error.message))
            }
            if (System.nanoTime() > deadline) return last
            sleep(opts.pollInterval.toMillis())
        }
    }

    private fun readLogEntries(conn: ManagedChannel?): List<Observability.LogEntry> {
        if (conn == null) {
            return Observability.current().logRing?.drain() ?: emptyList()
        }
        val iterator = ClientCalls.blockingServerStreamingCall(
            conn,
            Observability.logsMethod,
            CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
            ObsProto.LogsRequest.newBuilder()
                .setMinLevel(ObsProto.LogLevel.INFO)
                .build(),
        )
        return iterator.asSequence().map { Observability.fromProtoLogEntry(it) }.toList()
    }

    private fun readEventEntries(conn: ManagedChannel?): List<Observability.Event> {
        if (conn == null) {
            return Observability.current().eventBus?.drain() ?: emptyList()
        }
        val iterator = ClientCalls.blockingServerStreamingCall(
            conn,
            Observability.eventsMethod,
            CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
            ObsProto.EventsRequest.getDefaultInstance(),
        )
        return iterator.asSequence().map { Observability.fromProtoEvent(it) }.toList()
    }

    private fun matchRelayedLog(entries: List<Observability.LogEntry>, opts: LogCheckOptions): CheckOutcome {
        for (entry in entries) {
            if (entry.message != "tick received") continue
            if (entry.fields["sender"] != opts.sender || entry.fields["responder_uid"] != opts.leafUid) continue
            val evidence = compareChain(entry.chain, opts.expectedChain)
            if (evidence.isNotEmpty()) {
                return CheckOutcome(false, compactEvidence("matching log bad chain: $evidence"))
            }
            return CheckOutcome(true)
        }
        return CheckOutcome(
            false,
            compactEvidence("no relayed tick log sender=${opts.sender} leaf_uid=${opts.leafUid} entries=${entries.size}"),
        )
    }

    private fun matchRelayedEvent(events: List<Observability.Event>, opts: EventCheckOptions): CheckOutcome {
        for (event in events) {
            if (event.type != opts.eventType || event.instanceUid != opts.leafUid) continue
            val evidence = compareChain(event.chain, opts.expectedChain)
            if (evidence.isNotEmpty()) {
                return CheckOutcome(false, compactEvidence("matching event bad chain: $evidence"))
            }
            return CheckOutcome(true)
        }
        return CheckOutcome(
            false,
            compactEvidence("no relayed ${opts.eventType} event leaf_uid=${opts.leafUid} events=${events.size}"),
        )
    }

    private fun compareChain(got: List<Observability.Hop>, want: List<ChainHop>): String {
        if (got.size != want.size) return "chain length ${got.size} want ${want.size}"
        for (index in want.indices) {
            val actual = got[index]
            val expected = want[index]
            if (actual.slug != expected.slug || actual.instanceUid != expected.instanceUid) {
                return "hop $index=${actual.slug}/${actual.instanceUid} want ${expected.slug}/${expected.instanceUid}"
            }
        }
        return ""
    }

    private fun applyDialOptions(options: List<DialOption>?): DialOptions {
        val out = DialOptions()
        options.orEmpty().forEach { it.apply(out) }
        return out
    }

    private fun listenUriForSpawn(transport: String, uid: String): String = when (transport) {
        "stdio" -> "stdio://"
        "tcp" -> "tcp://127.0.0.1:0"
        "unix" -> "unix://${Path.of(System.getProperty("java.io.tmpdir"), "op-${cleanSocketToken(uid)}.sock")}"
        else -> throw IllegalArgumentException("unsupported transport \"$transport\"")
    }

    private fun spawnEnvironment(uid: String, extra: Map<String, String>): Map<String, String> {
        val env = linkedMapOf<String, String>()
        env["OP_INSTANCE_UID"] = uid
        env["OP_RUN_DIR"] = runRootFromEnv(System.getenv())
        env["HOLONS_PARENT_PID"] = ProcessHandle.current().pid().toString()
        val families = activeObservabilityFamilies()
        if (families.isNotBlank()) env["OP_OBS"] = families
        env.putAll(extra)
        return env
    }

    private fun activeObservabilityFamilies(): String {
        val obs = Observability.current()
        val families = mutableListOf<String>()
        if (obs.enabled(Observability.Family.LOGS)) families += "logs"
        if (obs.enabled(Observability.Family.METRICS)) families += "metrics"
        if (obs.enabled(Observability.Family.EVENTS)) families += "events"
        if (obs.enabled(Observability.Family.PROM)) families += "prom"
        return families.joinToString(",")
    }

    private fun runRootFromEnv(env: Map<String, String>): String {
        env["OP_RUN_DIR"]?.takeIf { it.isNotBlank() }?.let { return it }
        env["OPPATH"]?.takeIf { it.isNotBlank() }?.let { return Path.of(it, "run").toString() }
        env["HOME"]?.takeIf { it.isNotBlank() }?.let { return Path.of(it, ".op", "run").toString() }
        return Path.of(System.getProperty("java.io.tmpdir"), ".op", "run").toString()
    }

    private fun waitSpawnMeta(runRoot: String, slug: String, uid: String, timeout: Duration): MetaJson {
        val metaPath = Path.of(runRoot, slug, uid, "meta.json")
        val deadline = System.nanoTime() + timeout.toNanos()
        var last: Exception? = null
        while (System.nanoTime() < deadline) {
            try {
                val meta = parseMetaJson(Files.readString(metaPath))
                if (meta.uid == uid && meta.address.isNotBlank()) return meta
            } catch (error: Exception) {
                last = error
            }
            sleep(50)
        }
        throw IOException("meta not ready for $slug/$uid: ${last?.message ?: "timeout"}")
    }

    private fun parseMetaJson(raw: String): MetaJson {
        val obj = Json.parseToJsonElement(raw).jsonObject
        return MetaJson(
            uid = obj["uid"]?.jsonPrimitive?.content.orEmpty(),
            address = obj["address"]?.jsonPrimitive?.content.orEmpty(),
        )
    }

    private fun describeReady(channel: ManagedChannel, timeout: Duration) {
        val deadline = System.nanoTime() + timeout.toNanos()
        var last: Exception? = null
        while (System.nanoTime() < deadline) {
            try {
                ClientCalls.blockingUnaryCall(
                    channel,
                    Describe.describeMethod(),
                    CallOptions.DEFAULT.withDeadlineAfter(500, TimeUnit.MILLISECONDS),
                    holons.v1.Describe.DescribeRequest.getDefaultInstance(),
                )
                return
            } catch (error: Exception) {
                last = error
                sleep(50)
            }
        }
        throw IOException("describe readiness failed: ${last?.message ?: "timeout"}")
    }

    private fun normalizeAddressForDial(address: String): String {
        val trimmed = address.trim()
        require(trimmed.isNotEmpty()) { "dial address is required" }
        require(!trimmed.startsWith("stdio://")) { "Composite.dial does not support stdio addresses; use spawnMember" }
        if (trimmed.startsWith("tcp://") || trimmed.startsWith("unix://")) return trimmed
        require(!trimmed.contains("://")) { "unsupported dial address \"$address\"" }
        require(trimmed.contains(":")) { "dial address must be tcp://host:port, unix:///path, or host:port" }
        return trimmed
    }

    private fun newInstanceUid(): String {
        val bytes = ByteArray(12)
        random.nextBytes(bytes)
        return bytes.joinToString("") { "%02x".format(it.toInt() and 0xff) }
    }

    private fun cleanSocketToken(value: String): String =
        value.trim().take(24).replace(Regex("[/\\\\: ]"), "-")

    private fun stopProcess(process: Process?) {
        if (process == null || !process.isAlive) return
        process.destroy()
        try {
            if (!process.waitFor(2, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                process.waitFor(2, TimeUnit.SECONDS)
            }
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
            process.destroyForcibly()
        }
    }

    private fun startDrainThread(stream: InputStream, name: String) {
        Thread({
            val buffer = ByteArray(4096)
            try {
                while (stream.read(buffer) >= 0) {
                    // Drain only; child process logs are not protocol output.
                }
            } catch (_: IOException) {
            }
        }, name).apply {
            isDaemon = true
            start()
        }
    }

    private fun sleep(millis: Long) {
        try {
            Thread.sleep(millis.coerceAtLeast(1))
        } catch (_: InterruptedException) {
            Thread.currentThread().interrupt()
        }
    }

    private fun compactEvidence(value: String?): String {
        val compact = value.orEmpty().split(Regex("\\s+")).joinToString(" ").trim()
        return if (compact.length <= 240) compact else compact.take(240) + "..."
    }

    private fun closeQuietly(closeable: AutoCloseable?) {
        runCatching { closeable?.close() }
    }

    private data class MetaJson(val uid: String, val address: String)

    private class StdioBridge(private val process: Process) : Closeable {
        private val listener = ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"))
        private val acceptThread = Thread(::acceptLoop, "holons-composite-stdio-accept")
        @Volatile private var socket: Socket? = null
        @Volatile private var closed = false

        init {
            acceptThread.isDaemon = true
            acceptThread.start()
        }

        fun uri(): String = "tcp://127.0.0.1:${listener.localPort}"

        override fun close() {
            closed = true
            closeQuietly(listener)
            socket?.let { closeQuietly(it) }
            socket = null
            closeQuietly(process.outputStream)
            closeQuietly(process.inputStream)
            try {
                acceptThread.join(200)
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            }
        }

        private fun acceptLoop() {
            try {
                val accepted = listener.accept()
                if (closed) {
                    accepted.close()
                    return
                }
                socket = accepted
                val upstream = startPump(accepted.getInputStream(), process.outputStream, true, "holons-composite-stdio-up")
                val downstream = startPump(process.inputStream, accepted.getOutputStream(), true, "holons-composite-stdio-down")
                upstream.join()
                downstream.join()
            } catch (_: IOException) {
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            }
        }

        private fun startPump(input: InputStream, output: OutputStream, closeOutput: Boolean, name: String): Thread =
            Thread({
                val buffer = ByteArray(16 * 1024)
                try {
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        output.write(buffer, 0, read)
                        output.flush()
                    }
                } catch (_: IOException) {
                } finally {
                    if (closeOutput) closeQuietly(output)
                }
            }, name).apply {
                isDaemon = true
                start()
            }
    }
}

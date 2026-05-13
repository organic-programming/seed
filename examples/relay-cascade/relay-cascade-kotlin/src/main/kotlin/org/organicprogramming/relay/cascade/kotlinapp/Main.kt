package org.organicprogramming.relay.cascade.kotlinapp

import com.google.gson.Gson
import holons.v1.Describe
import holons.v1.Observability as ObsProto
import io.grpc.CallOptions
import io.grpc.ManagedChannel
import io.grpc.stub.ClientCalls
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.util.Collections
import java.util.Locale
import java.util.concurrent.TimeUnit
import kotlin.io.path.isExecutable
import kotlin.io.path.isRegularFile
import kotlinx.coroutines.runBlocking
import org.organicprogramming.holons.Connect
import org.organicprogramming.holons.ConnectOptions
import org.organicprogramming.holons.Observability
import relay.v1.Relay
import relay.v1.RelayServiceGrpc

private const val RUN_PHASES = 4
private const val RUN_TICKS = 3
private val ROLE_ORDER = listOf("D", "C", "B", "A")
private val TRANSPORTS = listOf("tcp", "unix", "tcp", "unix")
private const val KOTLIN_SLUG = "cascade-node-kotlin"
private const val GO_SLUG = "cascade-node-go"

private val gson = Gson()
private val http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build()

fun main(args: Array<String>) {
    try {
        val app = App()
        when {
            "--multi-pattern" in args -> app.runMultiPattern()
            "--live-stream" in args -> app.runLiveStream()
            else -> app.runDefault()
        }
    } catch (error: Exception) {
        System.err.println()
        System.err.println("FAIL: ${error.message}")
        kotlin.system.exitProcess(1)
    }
}

private class App {
    private val relayRoot = findRelayRoot()
    private val repoRoot = findRepoRoot(relayRoot)

    fun runDefault() {
        val binary = findBinary(KOTLIN_SLUG)
        val runRoot = Files.createTempDirectory("relay-cascade-kotlin-")
        println("=== relay-cascade-kotlin ===")
        println()
        var totalPass = 0
        var totalFail = 0
        var previous = ""
        TRANSPORTS.forEachIndexed { index, transport ->
            val phase = index + 1
            if (previous.isEmpty()) {
                println("Phase $phase/$RUN_PHASES: transport=$transport")
            } else {
                println("Phase $phase/$RUN_PHASES: transport=$transport (switching from $previous)")
            }
            val started = nowMillis()
            val cascade = try {
                spawnCascade(phase, transport, allKotlinSpecs(binary), runRoot)
            } catch (error: Exception) {
                totalFail += RUN_TICKS
                println("  spawn FAIL: ${error.message}\n")
                previous = transport
                return@forEachIndexed
            }
            println("  spawned 4 nodes in ${elapsed(started)}")
            var previousMetric = 0.0
            for (tick in 1..RUN_TICKS) {
                val tickStart = nowMillis()
                val outcome = cascade.runTick(tick, previousMetric)
                if (outcome.metric.pass) previousMetric = outcome.metricValue
                val overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                if (overall) totalPass++ else totalFail++
                println(
                    "  Tick $tick/$RUN_TICKS: log ${passText(outcome.log.pass)}, " +
                        "event ${passText(outcome.event.pass)}, metric ${passText(outcome.metric.pass)} " +
                        "(overall ${passText(overall)} in ${elapsed(tickStart)})",
                )
                printFailureEvidence("log", outcome.log)
                printFailureEvidence("event", outcome.event)
                printFailureEvidence("metric", outcome.metric)
            }
            cascade.stop()
            println()
            previous = transport
        }
        println("Summary: ${totalPass + totalFail} ticks, $totalPass PASS, $totalFail FAIL")
        check(totalFail == 0) { "$totalFail tick(s) failed" }
    }

    fun runLiveStream() {
        val binary = findBinary(KOTLIN_SLUG)
        val runRoot = Files.createTempDirectory("relay-cascade-kotlin-live-")
        println("=== relay-cascade-kotlin --live-stream ===")
        println()
        println("Setup: opening long-lived Follow:true streams on A")
        println("       (initial transport: tcp)")
        println()
        var totalPass = 0
        var totalFail = 0
        var cascade: Cascade? = null
        var streams: LiveStreams? = null
        val specs = allKotlinSpecs(binary)
        TRANSPORTS.forEachIndexed { index, transport ->
            val phase = index + 1
            if (phase == 1) {
                println("Phase $phase/$RUN_PHASES: initial chain ($transport)")
            } else {
                println("Phase $phase/$RUN_PHASES: respawn on $transport")
                val killStart = nowMillis()
                streams?.stop()
                cascade?.stop()
                println("  killed 4 nodes in ${elapsed(killStart)}")
            }
            val spawnStart = nowMillis()
            val phaseCascade = try {
                spawnCascade(phase, transport, specs, runRoot)
            } catch (error: Exception) {
                totalFail += RUN_TICKS
                println("  spawn FAIL: ${error.message}\n")
                streams = null
                return@forEachIndexed
            }
            println("  spawned 4 nodes in ${elapsed(spawnStart)}")
            if (phase > 1) println("  re-opening Follow:true streams on new A")
            var streamError: String? = null
            streams = try {
                LiveStreams(phaseCascade.roles.getValue("A").relayAddress).also { it.start() }
            } catch (error: Exception) {
                streamError = error.message
                println("  stream re-open failed: ${error.message}")
                null
            }
            var previousMetric = 0.0
            for (tick in 1..RUN_TICKS) {
                val tickStart = nowMillis()
                val outcome = phaseCascade.runLiveTick(streams, streamError, tick, previousMetric)
                if (outcome.metric.pass) previousMetric = outcome.metricValue
                val overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                if (overall) totalPass++ else totalFail++
                println(
                    "  Tick $tick/$RUN_TICKS: log ${passText(outcome.log.pass)}, " +
                        "event ${passText(outcome.event.pass)}, metric ${passText(outcome.metric.pass)} " +
                        "(overall ${passText(overall)} in ${elapsed(tickStart)})",
                )
                printFailureEvidence("log", outcome.log)
                printFailureEvidence("event", outcome.event)
                printFailureEvidence("metric", outcome.metric)
            }
            println()
            cascade = phaseCascade
        }
        streams?.stop()
        cascade?.stop()
        println("Summary: $totalPass PASS / $totalFail FAIL across ${totalPass + totalFail} ticks")
        check(totalFail == 0) { "$totalFail tick(s) failed" }
    }

    fun runMultiPattern() {
        val kotlinBinary = findBinary(KOTLIN_SLUG)
        val goBinary = findBinary(GO_SLUG)
        val patterns = listOf(
            CascadePattern("kotlin-kotlin-kotlin-kotlin", allKotlinSpecs(kotlinBinary)),
            CascadePattern(
                "kotlin-kotlin-go-kotlin",
                mapOf(
                    "A" to RoleSpec(KOTLIN_SLUG, kotlinBinary),
                    "B" to RoleSpec(KOTLIN_SLUG, kotlinBinary),
                    "C" to RoleSpec(GO_SLUG, goBinary),
                    "D" to RoleSpec(KOTLIN_SLUG, kotlinBinary),
                ),
            ),
            CascadePattern(
                "kotlin-kotlin-go-go",
                mapOf(
                    "A" to RoleSpec(KOTLIN_SLUG, kotlinBinary),
                    "B" to RoleSpec(KOTLIN_SLUG, kotlinBinary),
                    "C" to RoleSpec(GO_SLUG, goBinary),
                    "D" to RoleSpec(GO_SLUG, goBinary),
                ),
            ),
        )
        val runRoot = Files.createTempDirectory("relay-cascade-kotlin-multi-")
        println("=== relay-cascade-kotlin (multi-pattern) ===")
        println()
        var totalPass = 0
        var totalFail = 0
        patterns.forEachIndexed { patternIndex, pattern ->
            println("Pattern ${patternIndex + 1}/${patterns.size}: ${pattern.name}")
            var patternPass = 0
            TRANSPORTS.forEachIndexed { index, transport ->
                val phase = index + 1
                val started = nowMillis()
                val cascade = try {
                    spawnCascade(phase, transport, pattern.roles, runRoot)
                } catch (error: Exception) {
                    totalFail += RUN_TICKS
                    println("  Phase $phase/$RUN_PHASES ($transport): spawn FAIL (${error.message})")
                    return@forEachIndexed
                }
                var streams: LiveStreams? = null
                var streamError: String? = null
                try {
                    val opened = LiveStreams(cascade.roles.getValue("A").relayAddress)
                    opened.start()
                    streams = opened
                    val ready = waitFor(5000, { cascade.checkLiveEvent(opened) }, 50)
                    if (!ready.pass) streamError = "live relay readiness: ${ready.evidence}"
                } catch (error: Exception) {
                    streamError = error.message
                }
                var previousMetric = 0.0
                val results = mutableListOf<String>()
                val evidence = mutableListOf<String>()
                for (tick in 1..RUN_TICKS) {
                    val sender = "${pattern.name}-phase-$phase-tick-$tick"
                    val outcome = cascade.runLiveTickWithSender(streams, streamError, sender, previousMetric)
                    if (outcome.metric.pass) previousMetric = outcome.metricValue
                    val overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
                    if (overall) {
                        patternPass++
                        totalPass++
                        results += "Tick $tick PASS"
                    } else {
                        totalFail++
                        results += "Tick $tick FAIL (${failureSummary(outcome)})"
                        evidence += "      Tick $tick evidence: ${compactEvidence(outcome)}"
                    }
                }
                println("  Phase $phase/$RUN_PHASES ($transport): ${results.joinToString(", ")} (spawned in ${elapsed(started)})")
                evidence.forEach(::println)
                streams?.stop()
                cascade.stop()
            }
            println("  Subtotal: $patternPass/12 PASS")
            println()
        }
        println("Summary: $totalPass PASS / $totalFail FAIL across ${totalPass + totalFail} ticks")
        check(totalFail == 0) { "$totalFail tick(s) failed" }
    }

    private fun spawnCascade(
        phase: Int,
        transport: String,
        specs: Map<String, RoleSpec>,
        runRoot: Path,
    ): Cascade {
        val roles = linkedMapOf<String, RoleRuntime>()
        ROLE_ORDER.forEach { role ->
            roles[role] = newRoleRuntime(phase, transport, role, specs.getValue(role))
        }
        roles.values.forEach { runtime ->
            Files.createDirectories(runRoot)
            deleteRecursively(runRoot.resolve(runtime.slug).resolve(runtime.uid))
        }
        val cascade = Cascade(phase, transport, runRoot, roles)
        ROLE_ORDER.forEach { role ->
            val runtime = roles.getValue(role)
            val child = childRole(role)
            if (child.isNotEmpty()) {
                runtime.memberAddress = roles.getValue(child).relayAddress
                runtime.memberSlug = roles.getValue(child).slug
            }
            startRole(cascade, runtime)
        }
        sleep(150)
        return cascade
    }

    private fun newRoleRuntime(phase: Int, transport: String, role: String, spec: RoleSpec): RoleRuntime {
        val uid = "relay-p${phase.toString().padStart(2, '0')}-${role.lowercase()}"
        if (transport == "tcp") {
            return RoleRuntime(role, uid, spec.slug, spec.binaryPath, "tcp://127.0.0.1:0")
        }
        if (transport == "unix") {
            val socket = Path.of("/tmp/relay-cascade-kotlin-p$phase-${role.lowercase()}-${ProcessHandle.current().pid()}.sock")
            Files.deleteIfExists(socket)
            val uri = "unix://$socket"
            return RoleRuntime(role, uid, spec.slug, spec.binaryPath, uri, relayAddress = uri, clientTarget = uri)
        }
        error("unknown transport $transport")
    }

    private fun startRole(cascade: Cascade, runtime: RoleRuntime) {
        val args = mutableListOf(runtime.binaryPath.toString(), "serve", "--listen", runtime.listenUri)
        if (runtime.memberAddress.isNotEmpty()) {
            args += "--member"
            args += "${runtime.memberSlug}=${runtime.memberAddress}"
        }
        val stderrPath = Files.createTempFile("relay-cascade-kotlin-${runtime.uid}-", ".stderr")
        runtime.stderrPath = stderrPath
        val builder = ProcessBuilder(args)
            .directory(repoRoot.toFile())
            .redirectOutput(ProcessBuilder.Redirect.DISCARD)
            .redirectError(stderrPath.toFile())
        builder.environment().putAll(
            mapOf(
                "OP_OBS" to "logs,events,metrics,prom",
                "OP_RUN_DIR" to cascade.runRoot.toString(),
                "OP_INSTANCE_UID" to runtime.uid,
                "OP_ORGANISM_UID" to cascade.roles.getValue("A").uid,
                "OP_ORGANISM_SLUG" to cascade.roles.getValue("A").slug,
                "OP_PROM_ADDR" to "127.0.0.1:0",
            ),
        )
        runtime.process = builder.start()
        try {
            val meta = waitMeta(cascade.runRoot, runtime.slug, runtime.uid, 15000)
            runtime.metricsAddr = meta.metrics_addr
            runtime.relayAddress = meta.address
            runtime.channel = runBlocking {
                Connect.connect(
                    runtime.relayAddress,
                    ConnectOptions(timeout = Duration.ofSeconds(5), transport = "tcp", start = false),
                )
            }
            runtime.relayClient = RelayServiceGrpc.newBlockingStub(runtime.channel)
            dialReady(runtime.channel)
        } catch (error: Exception) {
            val stderr = runtime.stderrPath?.takeIf { Files.exists(it) }?.let { Files.readString(it) }.orEmpty()
            throw IllegalStateException("start ${runtime.role}: ${stderr.ifBlank { error.message.orEmpty() }}", error)
        }
    }

    private fun waitMeta(runRoot: Path, slug: String, uid: String, timeoutMillis: Long): MetaJson {
        val metaPath = runRoot.resolve(slug).resolve(uid).resolve("meta.json")
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis)
        var last: Exception? = null
        while (System.nanoTime() < deadline) {
            try {
                val meta = gson.fromJson(Files.readString(metaPath), MetaJson::class.java)
                if (meta.uid == uid && meta.metrics_addr.isNotBlank()) return meta
            } catch (error: Exception) {
                last = error
            }
            sleep(50)
        }
        error("meta not ready for $slug/$uid: ${last?.message ?: "timeout"}")
    }

    private fun dialReady(channel: ManagedChannel) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10)
        var last: Exception? = null
        while (System.nanoTime() < deadline) {
            try {
                ClientCalls.blockingUnaryCall(
                    channel,
                    org.organicprogramming.holons.Describe.describeMethod(),
                    CallOptions.DEFAULT.withDeadlineAfter(500, TimeUnit.MILLISECONDS),
                    Describe.DescribeRequest.getDefaultInstance(),
                )
                return
            } catch (error: Exception) {
                last = error
                sleep(50)
            }
        }
        error("dial readiness failed: ${last?.message ?: "timeout"}")
    }

    private fun allKotlinSpecs(binary: Path): Map<String, RoleSpec> =
        ROLE_ORDER.associateWith { RoleSpec(KOTLIN_SLUG, binary) }

    private fun findBinary(slug: String): Path {
        val envName = "CASCADE_NODE_${slug.removePrefix("cascade-node-").uppercase(Locale.ROOT).replace('-', '_')}_BIN"
        System.getenv(envName)?.trim()?.takeIf { it.isNotEmpty() }?.let { return Path.of(it) }
        val process = ProcessBuilder("op", "--bin", slug)
            .directory(relayRoot.toFile())
            .redirectError(ProcessBuilder.Redirect.DISCARD)
            .start()
        val out = process.inputStream.readAllBytes().toString(StandardCharsets.UTF_8).trim()
        if (process.waitFor() == 0 && out.isNotBlank()) return Path.of(out)
        val home = Path.of(System.getProperty("user.home"), ".op", "bin", "$slug.holon", "bin")
        findExecutable(home, slug)?.let { return it }
        error("$slug binary not found; run op build $slug --install")
    }

    private fun findExecutable(root: Path, name: String): Path? {
        if (!Files.isDirectory(root)) return null
        val paths = Files.list(root).use { stream -> stream.sorted().toList() }
        for (path in paths) {
            if (Files.isDirectory(path)) {
                findExecutable(path, name)?.let { return it }
            } else if (path.fileName.toString() == name && path.isRegularFile() && path.isExecutable()) {
                return path
            }
        }
        return null
    }
}

private data class RoleSpec(val slug: String, val binaryPath: Path)
private data class CascadePattern(val name: String, val roles: Map<String, RoleSpec>)

private class RoleRuntime(
    val role: String,
    val uid: String,
    val slug: String,
    val binaryPath: Path,
    val listenUri: String,
    var relayAddress: String = "",
    var clientTarget: String = "",
) {
    var memberAddress: String = ""
    var memberSlug: String = ""
    var metricsAddr: String = ""
    var process: Process? = null
    var stderrPath: Path? = null
    lateinit var channel: ManagedChannel
    lateinit var relayClient: RelayServiceGrpc.RelayServiceBlockingStub

    fun hasChannel(): Boolean = this::channel.isInitialized
}

private data class CheckResult(val pass: Boolean, val evidence: String = "")
private data class TickOutcome(val log: CheckResult, val event: CheckResult, val metric: CheckResult, val metricValue: Double)

private class Cascade(
    val phase: Int,
    val transport: String,
    val runRoot: Path,
    val roles: Map<String, RoleRuntime>,
) {
    fun runTick(tick: Int, previousMetric: Double): TickOutcome =
        runTickWithSender("phase-$phase-tick-$tick", previousMetric)

    fun runTickWithSender(sender: String, previousMetric: Double): TickOutcome {
        val request = Relay.TickRequest.newBuilder()
            .setSender(sender)
            .setNote(transport)
            .build()
        try {
            roles.getValue("D").relayClient.withDeadlineAfter(5, TimeUnit.SECONDS).tick(request)
        } catch (error: Exception) {
            val failed = CheckResult(false, error.message.orEmpty())
            return TickOutcome(failed, failed, failed, previousMetric)
        }
        val log = waitFor(3000, { checkLog(sender) }, 100)
        val event = waitFor(3000, { checkEvent() }, 100)
        val metricCheck = MetricCheck(previousMetric)
        val metric = waitFor(3000, { metricCheck.check(this) }, 100)
        return TickOutcome(log, event, metric, metricCheck.value)
    }

    fun runLiveTick(streams: LiveStreams?, streamOpenError: String?, tick: Int, previousMetric: Double): TickOutcome =
        runLiveTickWithSender(streams, streamOpenError, "phase-$phase-tick-$tick", previousMetric)

    fun runLiveTickWithSender(
        streams: LiveStreams?,
        streamOpenError: String?,
        sender: String,
        previousMetric: Double,
    ): TickOutcome {
        val request = Relay.TickRequest.newBuilder()
            .setSender(sender)
            .setNote(transport)
            .build()
        try {
            roles.getValue("D").relayClient.withDeadlineAfter(5, TimeUnit.SECONDS).tick(request)
        } catch (error: Exception) {
            val failed = CheckResult(false, error.message.orEmpty())
            return TickOutcome(failed, failed, failed, previousMetric)
        }
        val log: CheckResult
        val event: CheckResult
        if (streamOpenError == null && streams != null) {
            log = waitFor(1000, { checkLiveLog(streams, sender) }, 50)
            event = waitFor(1000, { checkLiveEvent(streams) }, 50)
        } else {
            val evidence = "stream re-open failed: ${streamOpenError ?: "streams not open"}"
            log = CheckResult(false, evidence)
            event = CheckResult(false, evidence)
        }
        val metricCheck = MetricCheck(previousMetric)
        val metric = waitFor(1000, { metricCheck.check(this) }, 50)
        return TickOutcome(log, event, metric, metricCheck.value)
    }

    fun checkLog(sender: String): CheckResult {
        val entries = readLogs(roles.getValue("A").channel)
        entries.forEach { entry ->
            if (entry.message != "tick received") return@forEach
            if (entry.fieldsMap["sender"] != sender) return@forEach
            if (entry.fieldsMap["responder_uid"] != roles.getValue("D").uid) return@forEach
            val err = checkChain(entry.chainList)
            if (err.isNotEmpty()) return CheckResult(false, "matching log has bad chain: $err entry=$entry")
            return CheckResult(true, entry.toString())
        }
        return CheckResult(false, "no relayed D tick log for sender=$sender in ${entries.size} A log entries")
    }

    fun checkEvent(): CheckResult {
        val events = readEvents(roles.getValue("A").channel)
        events.forEach { event ->
            if (event.typeValue != Observability.EventType.INSTANCE_READY.code || event.instanceUid != roles.getValue("D").uid) {
                return@forEach
            }
            val err = checkChain(event.chainList)
            if (err.isNotEmpty()) return CheckResult(false, "matching event has bad chain: $err event=$event")
            return CheckResult(true, event.toString())
        }
        return CheckResult(false, "no relayed D INSTANCE_READY event in ${events.size} A events")
    }

    fun checkLiveLog(streams: LiveStreams, sender: String): CheckResult {
        val entries = streams.logEntries()
        entries.forEach { entry ->
            if (entry.message != "tick received") return@forEach
            if (entry.fieldsMap["sender"] != sender) return@forEach
            if (entry.fieldsMap["responder_uid"] != roles.getValue("D").uid) return@forEach
            val err = checkChain(entry.chainList)
            if (err.isNotEmpty()) return CheckResult(false, "matching live log has bad chain: $err entry=$entry")
            return CheckResult(true, entry.toString())
        }
        return CheckResult(false, "no live log found for sender=$sender; buffer=${entries.size} errors=${streams.errors()}")
    }

    fun checkLiveEvent(streams: LiveStreams): CheckResult {
        val events = streams.eventEntries()
        events.forEach { event ->
            if (event.typeValue != Observability.EventType.INSTANCE_READY.code || event.instanceUid != roles.getValue("D").uid) {
                return@forEach
            }
            val err = checkChain(event.chainList)
            if (err.isNotEmpty()) return CheckResult(false, "matching live event has bad chain: $err event=$event")
            return CheckResult(true, event.toString())
        }
        return CheckResult(false, "no live INSTANCE_READY event for D; buffer=${events.size} errors=${streams.errors()}")
    }

    fun checkMetric(metricCheck: MetricCheck): CheckResult {
        val body = fetchMetrics(roles.getValue("D").metricsAddr)
        val value = parseCascadeTicks(body, roles.getValue("D").uid)
            ?: return CheckResult(false, body)
        metricCheck.value = value
        if (value <= metricCheck.previous) {
            return CheckResult(false, "cascade_ticks_total=$value did not increase beyond ${metricCheck.previous}\n$body")
        }
        return CheckResult(true, "cascade_ticks_total=$value")
    }

    private fun checkChain(chain: List<ObsProto.ChainHop>): String {
        listOf("D", "C", "B").forEachIndexed { index, role ->
            if (index >= chain.size) return "chain length ${chain.size} < 3"
            val hop = chain[index]
            val want = roles.getValue(role)
            if (hop.slug != want.slug || hop.instanceUid != want.uid) {
                return "hop $index = ${hop.slug}/${hop.instanceUid}, want ${want.slug}/${want.uid}"
            }
        }
        return ""
    }

    fun stop() {
        ROLE_ORDER.asReversed().forEach { role ->
            val runtime = roles.getValue(role)
            runCatching { if (runtime.hasChannel()) runBlocking { Connect.disconnect(runtime.channel) } }
            runtime.process?.takeIf { it.isAlive }?.destroy()
        }
        ROLE_ORDER.asReversed().forEach { role ->
            val process = roles.getValue(role).process ?: return@forEach
            if (process.isAlive && !process.waitFor(2, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                process.waitFor(2, TimeUnit.SECONDS)
            }
        }
    }
}

private class MetricCheck(val previous: Double) {
    var value: Double = previous
    fun check(cascade: Cascade): CheckResult = cascade.checkMetric(this)
}

private class LiveStreams(private val address: String) {
    private lateinit var channel: ManagedChannel
    private val logs = Collections.synchronizedList(mutableListOf<ObsProto.LogEntry>())
    private val events = Collections.synchronizedList(mutableListOf<ObsProto.EventInfo>())
    private val errs = Collections.synchronizedList(mutableListOf<String>())
    private val threads = mutableListOf<Thread>()

    fun start() {
        channel = runBlocking {
            Connect.connect(address, ConnectOptions(timeout = Duration.ofSeconds(5), transport = "tcp", start = false))
        }
        threads += Thread(::readLogStream, "relay-cascade-kotlin-live-logs").also {
            it.isDaemon = true
            it.start()
        }
        threads += Thread(::readEventStream, "relay-cascade-kotlin-live-events").also {
            it.isDaemon = true
            it.start()
        }
    }

    fun stop() {
        runCatching { if (::channel.isInitialized) runBlocking { Connect.disconnect(channel) } }
        threads.forEach { it.interrupt() }
    }

    fun logEntries(): List<ObsProto.LogEntry> = synchronized(logs) { logs.toList() }
    fun eventEntries(): List<ObsProto.EventInfo> = synchronized(events) { events.toList() }
    fun errors(): List<String> = synchronized(errs) { errs.toList() }

    private fun readLogStream() {
        try {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.logsMethod,
                CallOptions.DEFAULT,
                ObsProto.LogsRequest.newBuilder()
                    .setMinLevel(ObsProto.LogLevel.INFO)
                    .setFollow(true)
                    .build(),
            )
            while (!Thread.currentThread().isInterrupted && iterator.hasNext()) {
                logs += iterator.next()
            }
        } catch (error: Exception) {
            errs += "logs stream ended: ${error.message}"
        }
    }

    private fun readEventStream() {
        try {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.eventsMethod,
                CallOptions.DEFAULT,
                ObsProto.EventsRequest.newBuilder()
                    .setFollow(true)
                    .build(),
            )
            while (!Thread.currentThread().isInterrupted && iterator.hasNext()) {
                events += iterator.next()
            }
        } catch (error: Exception) {
            errs += "events stream ended: ${error.message}"
        }
    }
}

private data class MetaJson(
    val uid: String = "",
    val address: String = "",
    val metrics_addr: String = "",
)

private fun readLogs(channel: ManagedChannel): List<ObsProto.LogEntry> =
    ClientCalls.blockingServerStreamingCall(
        channel,
        Observability.logsMethod,
        CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
        ObsProto.LogsRequest.newBuilder()
            .setMinLevel(ObsProto.LogLevel.INFO)
            .build(),
    ).asSequence().toList()

private fun readEvents(channel: ManagedChannel): List<ObsProto.EventInfo> =
    ClientCalls.blockingServerStreamingCall(
        channel,
        Observability.eventsMethod,
        CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
        ObsProto.EventsRequest.getDefaultInstance(),
    ).asSequence().toList()

private fun fetchMetrics(addr: String): String {
    val request = HttpRequest.newBuilder(URI.create(addr)).timeout(Duration.ofSeconds(2)).GET().build()
    return http.send(request, HttpResponse.BodyHandlers.ofString()).body()
}

private fun parseCascadeTicks(body: String, uid: String): Double? {
    val needle = "responder_uid=\"$uid\""
    body.lineSequence().forEach { line ->
        if (line.startsWith("cascade_ticks_total{") && needle in line) {
            return line.trim().split(Regex("\\s+")).lastOrNull()?.toDoubleOrNull()
        }
    }
    return null
}

private fun waitFor(timeoutMillis: Long, fn: () -> CheckResult, intervalMillis: Long): CheckResult {
    val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis)
    var last = CheckResult(false)
    while (true) {
        last = fn()
        if (last.pass || System.nanoTime() > deadline) return last
        sleep(intervalMillis)
    }
}

private fun childRole(role: String): String = when (role) {
    "A" -> "B"
    "B" -> "C"
    "C" -> "D"
    else -> ""
}

private fun findRelayRoot(): Path {
    var current: Path? = Path.of(System.getProperty("user.dir")).toAbsolutePath().normalize()
    while (current != null) {
        if (Files.isDirectory(current.resolve("cascade-node-kotlin")) && Files.isDirectory(current.resolve("cascade-node-go"))) {
            return current
        }
        current = current.parent
    }
    error("relay-cascade root not found")
}

private fun findRepoRoot(start: Path): Path {
    var current: Path? = start.toAbsolutePath().normalize()
    while (current != null) {
        if (Files.isDirectory(current.resolve("sdk")) && Files.isDirectory(current.resolve("examples"))) {
            return current
        }
        current = current.parent
    }
    error("repository root not found")
}

private fun nowMillis(): Long = System.nanoTime() / 1_000_000

private fun elapsed(startedMillis: Long): String {
    val elapsed = (nowMillis() - startedMillis).coerceAtLeast(0)
    return if (elapsed < 1000) "${elapsed}ms" else "%.1fs".format(Locale.ROOT, elapsed / 1000.0)
}

private fun passText(value: Boolean): String = if (value) "PASS" else "FAIL"

private fun printFailureEvidence(family: String, result: CheckResult) {
    if (!result.pass) println("    $family evidence: ${result.evidence.ifBlank { "<empty>" }}")
}

private fun failureSummary(outcome: TickOutcome): String =
    buildList {
        if (!outcome.log.pass) add("log family")
        if (!outcome.event.pass) add("event family")
        if (!outcome.metric.pass) add("metric family")
    }.ifEmpty { listOf("unknown") }.joinToString(", ")

private fun compactEvidence(outcome: TickOutcome): String =
    buildList {
        if (!outcome.log.pass) add("log=${outcome.log.evidence}")
        if (!outcome.event.pass) add("event=${outcome.event.evidence}")
        if (!outcome.metric.pass) add("metric=${outcome.metric.evidence}")
    }.joinToString(" | ")

private fun sleep(millis: Long) {
    try {
        Thread.sleep(millis)
    } catch (_: InterruptedException) {
        Thread.currentThread().interrupt()
    }
}

private fun deleteRecursively(path: Path) {
    if (!Files.exists(path)) return
    Files.walk(path).use { stream ->
        stream.sorted(Collections.reverseOrder()).forEach { Files.deleteIfExists(it) }
    }
}

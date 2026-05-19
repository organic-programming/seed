package org.organicprogramming.observability.cascade.kotlinapp

import io.grpc.stub.StreamObserver
import java.time.Duration
import java.util.Locale
import java.util.concurrent.TimeUnit
import observability_cascade.v1.ObservabilityCascadeServiceGrpc
import observability_cascade.v1.Service
import org.organicprogramming.holons.Composite
import org.organicprogramming.holons.Describe
import org.organicprogramming.holons.Observability
import org.organicprogramming.holons.Serve
import relay.v1.Relay
import relay.v1.RelayServiceGrpc

private const val KOTLIN_SLUG = "observability-cascade-kotlin-node"
private const val GO_SLUG = "observability-cascade-go-node"
private const val RUN_TICKS = 3

fun main(args: Array<String>) {
    try {
        if (args.isNotEmpty() && canonicalCommand(args[0]) == "serve") {
            serveComposite(args.drop(1).toTypedArray())
            return
        }

        val multi = "--multi-pattern" in args
        val live = "--live-stream" in args
        val failed = if (multi) {
            runMultiPatternReport(emit = true).totalFail
        } else {
            runReport(
                name = if (live) "live-stream" else "default",
                members = ownLanguageMembers(),
                live = live,
                emit = true,
            ).fail
        }
        if (failed > 0) {
            kotlin.system.exitProcess(1)
        }
    } catch (error: Exception) {
        System.err.println("FAIL: ${error.message}")
        kotlin.system.exitProcess(1)
    }
}

private fun serveComposite(args: Array<String>) {
    Describe.useStaticResponse(gen.DescribeGenerated.StaticDescribeResponse())
    val parsed = Serve.parseOptions(args)
    Serve.runWithOptions(
        normalizeListenUri(parsed.listenUri),
        listOf(CascadeService()),
        Serve.Options(
            reflect = parsed.reflect,
            slug = "observability-cascade-kotlin",
        ),
    )
}

private class CascadeService : ObservabilityCascadeServiceGrpc.ObservabilityCascadeServiceImplBase() {
    override fun runDefault(
        request: Service.RunRequest,
        responseObserver: StreamObserver<Service.CascadeReport>,
    ) {
        responseObserver.onNext(runReport("default", ownLanguageMembers(), live = false, emit = false))
        responseObserver.onCompleted()
    }

    override fun runLiveStream(
        request: Service.RunRequest,
        responseObserver: StreamObserver<Service.CascadeReport>,
    ) {
        responseObserver.onNext(runReport("live-stream", ownLanguageMembers(), live = true, emit = false))
        responseObserver.onCompleted()
    }

    override fun runMultiPattern(
        request: Service.RunRequest,
        responseObserver: StreamObserver<Service.MultiPatternReport>,
    ) {
        responseObserver.onNext(runMultiPatternReport(emit = false))
        responseObserver.onCompleted()
    }
}

private fun runMultiPatternReport(emit: Boolean): Service.MultiPatternReport {
    val totalStart = System.nanoTime()
    val out = Service.MultiPatternReport.newBuilder()
    val patterns = kotlinPatterns()
    if (emit) {
        println("=== observability-cascade-kotlin --multi-pattern ===")
        println()
    }
    patterns.forEachIndexed { index, pattern ->
        if (emit) {
            println("Pattern ${index + 1}/${patterns.size}: ${pattern.name}")
        }
        val report = runReport(pattern.name, pattern.members, live = true, emit = emit)
        out.addPatterns(report)
            .setTotalPass(out.totalPass + report.pass)
            .setTotalFail(out.totalFail + report.fail)
        if (emit) {
            val status = if (report.fail == 0) "PASS" else "FAIL"
            println("Pattern ${pattern.name}: ${report.pass}/${report.ticks} $status (elapsed=${elapsedText(report.elapsedUs)})")
            println()
        }
    }
    out.totalElapsedUs = elapsedUs(totalStart)
    if (emit) {
        println(
            "Summary: ${out.totalPass} PASS / ${out.totalFail} FAIL across " +
                "${out.totalPass + out.totalFail} ticks (total elapsed=${elapsedText(out.totalElapsedUs)})",
        )
    }
    return out.build()
}

private fun runReport(
    name: String,
    members: List<LanguageMember>,
    live: Boolean,
    emit: Boolean,
): Service.CascadeReport {
    ensureCascadeObservability()
    val reportStart = System.nanoTime()
    val report = Service.CascadeReport.newBuilder().setName(name)
    val timeout = if (live) Duration.ofSeconds(1) else Duration.ofSeconds(3)
    val poll = if (live) Duration.ofMillis(50) else Duration.ofMillis(100)
    if (emit) {
        println("=== observability-cascade-kotlin ${modeSuffix(name)}===")
        println()
    }

    Composite.TRANSPORT_COVERAGE_SEQUENCE.forEachIndexed { phaseIndex, transport ->
        val phaseStart = System.nanoTime()
        val from = if (phaseIndex == 0) transport else Composite.TRANSPORT_COVERAGE_SEQUENCE[phaseIndex - 1]
        val phase = Service.PhaseResult.newBuilder()
            .setName("%02d-%s→%s".format(Locale.ROOT, phaseIndex + 1, from, transport))
        if (emit) {
            println("Phase ${phaseIndex + 1}/${Composite.TRANSPORT_COVERAGE_SEQUENCE.size}: ${phase.name}")
        }

        val cascade = try {
            Composite.buildCascade(
                Composite.CascadeOptions().apply {
                    this.transport = transport
                    this.members = childSpecs(members)
                    extraEnv = mapOf(
                        "OP_OBS" to "logs,events,metrics,prom",
                        "OP_PROM_ADDR" to "127.0.0.1:0",
                    )
                },
            )
        } catch (error: Exception) {
            phase.fail = RUN_TICKS
            repeat(RUN_TICKS) { tick ->
                phase.addFailures("tick=${tick + 1} log=spawn event=spawn hops=${compactEvidence(error.message)}")
            }
            finishPhase(report, phase, phaseStart, emit)
            return@forEachIndexed
        }

        val previous = linkedMapOf<String, Long>()
        try {
            for (tick in 1..RUN_TICKS) {
                val sender = "%s-phase-%02d-tick-%d".format(Locale.ROOT, name, phaseIndex + 1, tick)
                val result = runTick(cascade, sender, transport, members, previous, timeout, poll, live)
                if (result.pass) {
                    phase.pass = phase.pass + 1
                } else {
                    phase.fail = phase.fail + 1
                    phase.addFailures(result.evidenceLine(tick))
                }
                if (emit) {
                    println("  Tick $tick/$RUN_TICKS: ${if (result.pass) "PASS" else "FAIL"}")
                    if (!result.pass) {
                        System.err.println("    ${result.evidenceLine(tick)}")
                    }
                }
            }
        } finally {
            cascade.stop()
        }
        finishPhase(report, phase, phaseStart, emit)
    }

    report.elapsedUs = elapsedUs(reportStart)
    if (emit) {
        println()
        println(
            "Summary: ${report.ticks} ticks, ${report.pass} PASS, " +
                "${report.fail} FAIL (total elapsed=${elapsedText(report.elapsedUs)})",
        )
    }
    return report.build()
}

private fun finishPhase(
    report: Service.CascadeReport.Builder,
    phase: Service.PhaseResult.Builder,
    phaseStart: Long,
    emit: Boolean,
) {
    phase.elapsedUs = elapsedUs(phaseStart)
    val built = phase.build()
    report.addPhases(built)
        .setPass(report.pass + built.pass)
        .setFail(report.fail + built.fail)
        .setTicks(report.ticks + built.pass + built.fail)
    if (emit) {
        val status = if (built.fail == 0) "PASS" else "FAIL"
        println("Phase ${built.name}: ${built.pass}/${built.pass + built.fail} $status (elapsed=${elapsedText(built.elapsedUs)})")
    }
}

private fun runTick(
    cascade: Composite.Cascade,
    sender: String,
    note: String,
    members: List<LanguageMember>,
    previous: MutableMap<String, Long>,
    timeout: Duration,
    poll: Duration,
    live: Boolean,
): TickResult {
    val response = try {
        RelayServiceGrpc.newBlockingStub(cascade.top.conn)
            .withDeadlineAfter(5, TimeUnit.SECONDS)
            .tick(Relay.TickRequest.newBuilder().setSender(sender).setNote(note).build())
    } catch (error: Exception) {
        val failed = Composite.CheckOutcome(false, compactEvidence(error.message))
        return TickResult(false, failed, failed, failed)
    }

    val hops = checkHops(response.hopsList, members, previous)
    if (!hops.pass) {
        val skipped = Composite.CheckOutcome(false, "skipped")
        return TickResult(false, skipped, skipped, hops)
    }

    val expected = hopChain(response.hopsList)
    val leafUid = response.hopsList.first().uid
    val log = Composite.checkRelayedLog(
        Composite.LogCheckOptions().apply {
            this.sender = sender
            this.leafUid = leafUid
            expectedChain = expected
            this.timeout = timeout
            pollInterval = poll
            this.live = live
        },
    )
    val event = Composite.checkRelayedEvent(
        Composite.EventCheckOptions().apply {
            eventName = Observability.EventName.INSTANCE_READY
            this.leafUid = leafUid
            expectedChain = expected
            this.timeout = timeout
            pollInterval = poll
            this.live = live
        },
    )
    return TickResult(hops.pass && log.pass && event.pass, log, event, hops)
}

private fun checkHops(
    hops: List<Relay.HopReceipt>,
    members: List<LanguageMember>,
    previous: MutableMap<String, Long>,
): Composite.CheckOutcome {
    if (hops.size != members.size) {
        return Composite.CheckOutcome(false, "hops length ${hops.size} want ${members.size}")
    }
    for (index in hops.indices) {
        val hop = hops[index]
        val want = members[members.size - 1 - index]
        if (hop.slug != want.slug) {
            return Composite.CheckOutcome(false, "hop $index slug=${hop.slug} want ${want.slug}")
        }
        if (hop.uid.isBlank()) {
            return Composite.CheckOutcome(false, "hop $index uid empty")
        }
        val last = previous[hop.uid] ?: 0L
        if (hop.received <= last) {
            return Composite.CheckOutcome(false, "hop $index received=${hop.received} previous=$last")
        }
        previous[hop.uid] = hop.received
    }
    return Composite.CheckOutcome(true)
}

private fun hopChain(hops: List<Relay.HopReceipt>): List<Composite.ChainHop> =
    hops.map { Composite.ChainHop(it.slug, it.uid) }

private fun ownLanguageMembers(): List<LanguageMember> {
    val binary = memberPath("kotlin-node")
    return listOf(
        LanguageMember("kotlin", KOTLIN_SLUG, binary),
        LanguageMember("kotlin", KOTLIN_SLUG, binary),
        LanguageMember("kotlin", KOTLIN_SLUG, binary),
    )
}

private fun kotlinPatterns(): List<NamedPattern> {
    val bins = mapOf(
        "kotlin" to LanguageMember("kotlin", KOTLIN_SLUG, memberPath("kotlin-node")),
        "go" to LanguageMember("go", GO_SLUG, memberPath("go-node")),
    )
    val names = listOf(
        "kotlin-kotlin-kotlin",
        "kotlin-kotlin-go",
        "kotlin-go-kotlin",
        "kotlin-go-go",
        "go-kotlin-kotlin",
        "go-kotlin-go",
        "go-go-kotlin",
        "go-go-go",
    )
    return names.map { name ->
        val parts = name.split("-")
        NamedPattern(name, listOf(bins.getValue(parts[0]), bins.getValue(parts[1]), bins.getValue(parts[2])))
    }
}

private fun childSpecs(members: List<LanguageMember>): List<Composite.ChildSpec> =
    members.map { Composite.ChildSpec(it.slug, it.binary) }

private fun memberPath(id: String): String =
    try {
        Composite.member(id).toString()
    } catch (_: Exception) {
        ""
    }

private fun ensureCascadeObservability() {
    val obs = Observability.current()
    if (obs.enabled(Observability.Family.LOGS) && obs.enabled(Observability.Family.EVENTS)) {
        return
    }
    Observability.configureFromEnv(
        Observability.Config(
            slug = "observability-cascade-kotlin",
            instanceUid = "kotlin-composite-${ProcessHandle.current().pid()}",
        ),
        mapOf("OP_OBS" to "logs,events,metrics,prom"),
    )
}

private fun elapsedUs(startedNanos: Long): Long =
    ((System.nanoTime() - startedNanos) / 1000).coerceAtLeast(1)

private fun elapsedText(elapsedUs: Long): String {
    val duration = Duration.ofNanos(elapsedUs * 1000)
    return when {
        duration < Duration.ofSeconds(1) -> "${duration.toMillis()}ms"
        duration < Duration.ofMinutes(1) -> "%.2fs".format(Locale.ROOT, duration.toNanos() / 1_000_000_000.0)
        else -> "%.1fm".format(Locale.ROOT, duration.seconds / 60.0)
    }
}

private fun modeSuffix(name: String): String =
    if (name == "default") "" else "--$name "

private fun compactEvidence(value: String?): String {
    val compact = value.orEmpty().split(Regex("\\s+")).joinToString(" ").trim().ifEmpty { "<empty>" }
    return if (compact.length <= 240) compact else compact.take(240) + "..."
}

private fun normalizeListenUri(listenUri: String): String =
    Regex("^tcp://:(\\d+)$").matchEntire(listenUri)?.let { "tcp://0.0.0.0:${it.groupValues[1]}" } ?: listenUri

private fun canonicalCommand(raw: String): String =
    raw.trim().lowercase(Locale.ROOT).replace("-", "").replace("_", "").replace(" ", "")

private data class LanguageMember(val lang: String, val slug: String, val binary: String)
private data class NamedPattern(val name: String, val members: List<LanguageMember>)

private data class TickResult(
    val pass: Boolean,
    val log: Composite.CheckOutcome,
    val event: Composite.CheckOutcome,
    val hops: Composite.CheckOutcome,
) {
    fun evidenceLine(tick: Int): String =
        "tick=$tick log=${evidenceText(log)} event=${evidenceText(event)} hops=${evidenceText(hops)}"

    private fun evidenceText(outcome: Composite.CheckOutcome): String =
        if (outcome.pass) "ok" else compactEvidence(outcome.evidence)
}

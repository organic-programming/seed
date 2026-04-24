// Kotlin reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

package org.organicprogramming.holons

import java.io.IOException
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.Paths
import java.nio.file.StandardCopyOption
import java.nio.file.StandardOpenOption
import java.time.Instant
import java.util.ArrayDeque
import java.util.EnumSet
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

object Observability {

    enum class Family { LOGS, METRICS, EVENTS, PROM, OTEL }

    private val V1_TOKENS = setOf("logs", "metrics", "events", "prom", "all")

    class InvalidTokenException(
        val token: String,
        reason: String,
        val variable: String = "OP_OBS",
    ) : RuntimeException("$variable: $reason: $token")

    enum class Level(val code: Int) {
        UNSET(0), TRACE(1), DEBUG(2), INFO(3), WARN(4), ERROR(5), FATAL(6);

        fun label(): String = when (this) {
            TRACE -> "TRACE"; DEBUG -> "DEBUG"; INFO -> "INFO"
            WARN -> "WARN"; ERROR -> "ERROR"; FATAL -> "FATAL"
            else -> "UNSPECIFIED"
        }
    }

    enum class EventType {
        UNSPECIFIED,
        INSTANCE_SPAWNED, INSTANCE_READY, INSTANCE_EXITED, INSTANCE_CRASHED,
        SESSION_STARTED, SESSION_ENDED, HANDLER_PANIC, CONFIG_RELOADED
    }

    data class Hop(val slug: String, val instanceUid: String)

    fun parseOpObs(raw: String?): Set<Family> {
        val out = EnumSet.noneOf(Family::class.java)
        if (raw.isNullOrBlank()) return out
        for (part in raw.split(",")) {
            val tok = part.trim()
            if (tok.isEmpty()) continue
            if (tok == "otel" || tok == "sessions") continue
            if (tok !in V1_TOKENS) continue
            when (tok) {
                "all" -> {
                    out.add(Family.LOGS); out.add(Family.METRICS)
                    out.add(Family.EVENTS); out.add(Family.PROM)
                }
                else -> out.add(Family.valueOf(tok.uppercase()))
            }
        }
        return out
    }

    @JvmOverloads
    fun checkEnv(env: Map<String, String>? = null) {
        val sessions = (env?.get("OP_SESSIONS") ?: System.getenv("OP_SESSIONS") ?: "").trim()
        if (sessions.isNotEmpty()) {
            throw InvalidTokenException(sessions,
                "sessions are reserved for v2; not implemented in v1",
                "OP_SESSIONS")
        }
        val raw = (env?.get("OP_OBS") ?: System.getenv("OP_OBS") ?: "").trim()
        if (raw.isEmpty()) return
        for (part in raw.split(",")) {
            val tok = part.trim()
            if (tok.isEmpty()) continue
            if (tok == "otel") throw InvalidTokenException(tok,
                "otel export is reserved for v2; not implemented in v1")
            if (tok == "sessions") throw InvalidTokenException(tok,
                "sessions are reserved for v2; not implemented in v1")
            if (tok !in V1_TOKENS) throw InvalidTokenException(tok, "unknown OP_OBS token")
        }
    }

    fun appendDirectChild(src: List<Hop>?, childSlug: String, childUid: String): List<Hop> {
        val base = src ?: emptyList()
        return base + Hop(childSlug, childUid)
    }

    fun enrichForMultilog(wire: List<Hop>?, srcSlug: String, srcUid: String): List<Hop> =
        appendDirectChild(wire, srcSlug, srcUid)

    data class LogEntry(
        val timestamp: Instant,
        val level: Level,
        val slug: String,
        val instanceUid: String,
        val sessionId: String = "",
        val rpcMethod: String = "",
        val message: String,
        val fields: Map<String, String> = emptyMap(),
        val caller: String = "",
        val chain: List<Hop> = emptyList(),
    )

    data class Event(
        val timestamp: Instant,
        val type: EventType,
        val slug: String,
        val instanceUid: String,
        val sessionId: String = "",
        val payload: Map<String, String> = emptyMap(),
        val chain: List<Hop> = emptyList(),
    )

    class LogRing(capacity: Int = 1024) {
        private val capacity = capacity.coerceAtLeast(1)
        private val buf = ArrayDeque<LogEntry>(this.capacity)
        private val subs = CopyOnWriteArrayList<(LogEntry) -> Unit>()

        @Synchronized
        fun push(e: LogEntry) {
            if (buf.size == capacity) buf.removeFirst()
            buf.addLast(e)
            subs.forEach { runCatching { it(e) } }
        }

        @Synchronized fun drain(): List<LogEntry> = buf.toList()
        @Synchronized fun drainSince(cutoff: Instant): List<LogEntry> =
            buf.filter { !it.timestamp.isBefore(cutoff) }
        fun subscribe(fn: (LogEntry) -> Unit): AutoCloseable {
            subs.add(fn)
            return AutoCloseable { subs.remove(fn) }
        }
        @Synchronized fun size(): Int = buf.size
    }

    class EventBus(capacity: Int = 256) {
        private val capacity = capacity.coerceAtLeast(1)
        private val buf = ArrayDeque<Event>(this.capacity)
        private val subs = CopyOnWriteArrayList<(Event) -> Unit>()
        @Volatile private var closed = false

        @Synchronized
        fun emit(e: Event) {
            if (closed) return
            if (buf.size == capacity) buf.removeFirst()
            buf.addLast(e)
            subs.forEach { runCatching { it(e) } }
        }

        @Synchronized fun drain(): List<Event> = buf.toList()
        @Synchronized fun drainSince(cutoff: Instant): List<Event> =
            buf.filter { !it.timestamp.isBefore(cutoff) }
        fun subscribe(fn: (Event) -> Unit): AutoCloseable {
            subs.add(fn)
            return AutoCloseable { subs.remove(fn) }
        }
        fun close() { closed = true; subs.clear() }
    }

    class Counter internal constructor(
        val name: String, val help: String, val labels: Map<String, String>
    ) {
        private val v = AtomicLong()
        fun inc() { v.incrementAndGet() }
        fun add(n: Long) { if (n >= 0) v.addAndGet(n) }
        fun value(): Long = v.get()
    }

    class Gauge internal constructor(
        val name: String, val help: String, val labels: Map<String, String>
    ) {
        private var v = 0.0
        private val lock = Any()
        fun set(x: Double) { synchronized(lock) { v = x } }
        fun add(d: Double) { synchronized(lock) { v += d } }
        fun value(): Double = synchronized(lock) { v }
    }

    data class HistogramSnapshot(
        val bounds: DoubleArray, val counts: LongArray,
        val total: Long, val sum: Double,
    ) {
        fun quantile(q: Double): Double {
            if (total == 0L) return Double.NaN
            val target = total * q
            for (i in counts.indices) if (counts[i] >= target) return bounds[i]
            return Double.POSITIVE_INFINITY
        }
    }

    val DEFAULT_BUCKETS = doubleArrayOf(
        50e-6, 100e-6, 250e-6, 500e-6,
        1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
        1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
    )

    class Histogram internal constructor(
        val name: String, val help: String,
        val labels: Map<String, String>, bounds: DoubleArray?,
    ) {
        private val bounds: DoubleArray = (bounds?.takeIf { it.isNotEmpty() } ?: DEFAULT_BUCKETS).copyOf().also { it.sort() }
        private val counts = LongArray(this.bounds.size)
        private var total = 0L
        private var sum = 0.0

        @Synchronized
        fun observe(v: Double) {
            total++; sum += v
            for (i in bounds.indices) if (v <= bounds[i]) counts[i]++
        }

        @Synchronized
        fun snapshot() = HistogramSnapshot(bounds.copyOf(), counts.copyOf(), total, sum)
    }

    private fun metricKey(name: String, labels: Map<String, String>): String {
        if (labels.isEmpty()) return name
        val sb = StringBuilder(name)
        labels.toSortedMap().forEach { (k, v) -> sb.append('|').append(k).append('=').append(v) }
        return sb.toString()
    }

    class Registry {
        private val counters = ConcurrentHashMap<String, Counter>()
        private val gauges = ConcurrentHashMap<String, Gauge>()
        private val histograms = ConcurrentHashMap<String, Histogram>()

        fun counter(name: String, help: String = "", labels: Map<String, String> = emptyMap()): Counter =
            counters.computeIfAbsent(metricKey(name, labels)) { Counter(name, help, labels) }
        fun gauge(name: String, help: String = "", labels: Map<String, String> = emptyMap()): Gauge =
            gauges.computeIfAbsent(metricKey(name, labels)) { Gauge(name, help, labels) }
        fun histogram(name: String, help: String = "", labels: Map<String, String> = emptyMap(),
                      bounds: DoubleArray? = null): Histogram =
            histograms.computeIfAbsent(metricKey(name, labels)) { Histogram(name, help, labels, bounds) }

        fun counters(): List<Counter> = counters.values.sortedBy { it.name }
        fun gauges(): List<Gauge> = gauges.values.sortedBy { it.name }
        fun histograms(): List<Histogram> = histograms.values.sortedBy { it.name }
    }

    data class Config(
        var slug: String = "",
        var defaultLogLevel: Level = Level.INFO,
        var promAddr: String = "",
        var redactedFields: List<String> = emptyList(),
        var logsRingSize: Int = 1024,
        var eventsRingSize: Int = 256,
        var runDir: String = "",
        var instanceUid: String = "",
        var organismUid: String = "",
        var organismSlug: String = "",
    )

    class Logger internal constructor(private val obs: Instance, val name: String) {
        @Volatile private var level: Level = obs.cfg.defaultLogLevel

        fun setLevel(l: Level) { level = l }
        fun enabled(l: Level): Boolean = l.code >= level.code

        fun log(l: Level, message: String, fields: Map<String, Any?>? = null) {
            if (!enabled(l)) return
            val redact = obs.cfg.redactedFields.toSet()
            val out = linkedMapOf<String, String>()
            fields?.forEach { (k, v) ->
                if (k.isNotEmpty()) {
                    out[k] = if (k in redact) "<redacted>" else v?.toString().orEmpty()
                }
            }
            val entry = LogEntry(
                timestamp = Instant.now(),
                level = l,
                slug = obs.cfg.slug,
                instanceUid = obs.cfg.instanceUid,
                message = message,
                fields = out,
            )
            obs.logRing?.push(entry)
        }

        fun trace(m: String, f: Map<String, Any?>? = null) = log(Level.TRACE, m, f)
        fun debug(m: String, f: Map<String, Any?>? = null) = log(Level.DEBUG, m, f)
        fun info(m: String, f: Map<String, Any?>? = null)  = log(Level.INFO, m, f)
        fun warn(m: String, f: Map<String, Any?>? = null)  = log(Level.WARN, m, f)
        fun error(m: String, f: Map<String, Any?>? = null) = log(Level.ERROR, m, f)
        fun fatal(m: String, f: Map<String, Any?>? = null) = log(Level.FATAL, m, f)
    }

    class Instance internal constructor(val cfg: Config, val families: Set<Family>) {
        val logRing: LogRing? = if (Family.LOGS in families) LogRing(cfg.logsRingSize) else null
        val eventBus: EventBus? = if (Family.EVENTS in families) EventBus(cfg.eventsRingSize) else null
        val registry: Registry? = if (Family.METRICS in families) Registry() else null
        private val loggers = ConcurrentHashMap<String, Logger>()

        fun enabled(f: Family) = f in families
        val isOrganismRoot: Boolean
            get() = cfg.organismUid.isNotEmpty() && cfg.organismUid == cfg.instanceUid

        fun logger(name: String): Logger {
            if (Family.LOGS !in families) return DISABLED_LOGGER
            return loggers.computeIfAbsent(name) { Logger(this, it) }
        }

        fun counter(name: String, help: String = "", labels: Map<String, String> = emptyMap()): Counter? =
            registry?.counter(name, help, labels)
        fun gauge(name: String, help: String = "", labels: Map<String, String> = emptyMap()): Gauge? =
            registry?.gauge(name, help, labels)
        fun histogram(name: String, help: String = "", labels: Map<String, String> = emptyMap(),
                      bounds: DoubleArray? = null): Histogram? =
            registry?.histogram(name, help, labels, bounds)

        fun emit(type: EventType, payload: Map<String, String>? = null) {
            val bus = eventBus ?: return
            val redact = cfg.redactedFields.toSet()
            val p = linkedMapOf<String, String>()
            payload?.forEach { (k, v) -> p[k] = if (k in redact) "<redacted>" else v }
            bus.emit(Event(Instant.now(), type, cfg.slug, cfg.instanceUid, payload = p))
        }

        fun close() { eventBus?.close() }
    }

    private val CURRENT = AtomicReference<Instance?>(null)
    private val DISABLED = Instance(Config(), EnumSet.noneOf(Family::class.java))
    private val DISABLED_LOGGER = Logger(DISABLED, "")

    fun configure(cfg: Config): Instance {
        val families = parseOpObs(System.getenv("OP_OBS"))
        if (cfg.slug.isEmpty()) {
            cfg.slug = System.getProperty("sun.java.command", "").split(" ").firstOrNull().orEmpty()
        }
        val inst = Instance(cfg, families)
        CURRENT.set(inst)
        return inst
    }

    fun fromEnv(base: Config = Config()): Instance {
        val env = System.getenv()
        if (base.instanceUid.isEmpty()) base.instanceUid = env["OP_INSTANCE_UID"].orEmpty()
        if (base.organismUid.isEmpty()) base.organismUid = env["OP_ORGANISM_UID"].orEmpty()
        if (base.organismSlug.isEmpty()) base.organismSlug = env["OP_ORGANISM_SLUG"].orEmpty()
        if (base.promAddr.isEmpty()) base.promAddr = env["OP_PROM_ADDR"].orEmpty()
        if (base.runDir.isEmpty()) base.runDir = env["OP_RUN_DIR"].orEmpty()
        return configure(base)
    }

    fun current(): Instance = CURRENT.get() ?: DISABLED

    fun reset() {
        CURRENT.getAndSet(null)?.close()
    }

    // --- Disk writers ---

    fun enableDiskWriters(runDir: String) {
        val inst = current()
        if (runDir.isEmpty()) return
        try { Files.createDirectories(Paths.get(runDir)) } catch (_: IOException) {}

        if (inst.enabled(Family.LOGS) && inst.logRing != null) {
            val fp = Paths.get(runDir, "stdout.log")
            inst.logRing.subscribe { appendLog(fp, it) }
        }
        if (inst.enabled(Family.EVENTS) && inst.eventBus != null) {
            val fp = Paths.get(runDir, "events.jsonl")
            inst.eventBus.subscribe { appendEvent(fp, it) }
        }
    }

    private fun appendLog(fp: Path, e: LogEntry) {
        val sb = StringBuilder()
        sb.append("{\"kind\":\"log\"")
          .append(",\"ts\":\"").append(e.timestamp).append("\"")
          .append(",\"level\":\"").append(e.level.label()).append("\"")
          .append(",\"slug\":").append(quote(e.slug))
          .append(",\"instance_uid\":").append(quote(e.instanceUid))
          .append(",\"message\":").append(quote(e.message))
        if (e.sessionId.isNotEmpty()) sb.append(",\"session_id\":").append(quote(e.sessionId))
        if (e.rpcMethod.isNotEmpty()) sb.append(",\"rpc_method\":").append(quote(e.rpcMethod))
        if (e.fields.isNotEmpty()) { sb.append(",\"fields\":"); jsonMap(sb, e.fields) }
        if (e.caller.isNotEmpty()) sb.append(",\"caller\":").append(quote(e.caller))
        if (e.chain.isNotEmpty()) { sb.append(",\"chain\":"); jsonChain(sb, e.chain) }
        sb.append("}\n")
        appendFile(fp, sb.toString())
    }

    private fun appendEvent(fp: Path, e: Event) {
        val sb = StringBuilder()
        sb.append("{\"kind\":\"event\"")
          .append(",\"ts\":\"").append(e.timestamp).append("\"")
          .append(",\"type\":\"").append(e.type.name).append("\"")
          .append(",\"slug\":").append(quote(e.slug))
          .append(",\"instance_uid\":").append(quote(e.instanceUid))
        if (e.sessionId.isNotEmpty()) sb.append(",\"session_id\":").append(quote(e.sessionId))
        if (e.payload.isNotEmpty()) { sb.append(",\"payload\":"); jsonMap(sb, e.payload) }
        if (e.chain.isNotEmpty()) { sb.append(",\"chain\":"); jsonChain(sb, e.chain) }
        sb.append("}\n")
        appendFile(fp, sb.toString())
    }

    private fun appendFile(fp: Path, s: String) {
        runCatching {
            Files.newBufferedWriter(fp, StandardCharsets.UTF_8,
                StandardOpenOption.CREATE, StandardOpenOption.APPEND).use { it.write(s) }
        }
    }

    private fun quote(s: String): String {
        val sb = StringBuilder("\"")
        s.forEach { c ->
            when (c) {
                '\\' -> sb.append("\\\\"); '"' -> sb.append("\\\"")
                '\n' -> sb.append("\\n"); '\r' -> sb.append("\\r"); '\t' -> sb.append("\\t")
                else -> if (c.code < 0x20) sb.append("\\u%04x".format(c.code)) else sb.append(c)
            }
        }
        sb.append("\""); return sb.toString()
    }

    private fun jsonMap(sb: StringBuilder, m: Map<String, String>) {
        sb.append("{"); var first = true
        m.forEach { (k, v) ->
            if (!first) sb.append(","); first = false
            sb.append(quote(k)).append(":").append(quote(v))
        }
        sb.append("}")
    }

    private fun jsonChain(sb: StringBuilder, c: List<Hop>) {
        sb.append("["); var first = true
        c.forEach { h ->
            if (!first) sb.append(","); first = false
            sb.append("{\"slug\":").append(quote(h.slug))
              .append(",\"instance_uid\":").append(quote(h.instanceUid)).append("}")
        }
        sb.append("]")
    }

    data class MetaJson(
        var slug: String = "", var uid: String = "",
        var pid: Long = 0, var startedAt: Instant = Instant.now(),
        var mode: String = "persistent", var transport: String = "", var address: String = "",
        var metricsAddr: String = "", var logPath: String = "", var logBytesRotated: Long = 0,
        var organismUid: String = "", var organismSlug: String = "",
        var isDefault: Boolean = false,
    )

    fun writeMetaJson(runDir: String, m: MetaJson) {
        val dir = Paths.get(runDir)
        Files.createDirectories(dir)
        val sb = StringBuilder("{")
        sb.append("\"slug\":").append(quote(m.slug)).append(",")
        sb.append("\"uid\":").append(quote(m.uid)).append(",")
        sb.append("\"pid\":").append(m.pid).append(",")
        sb.append("\"started_at\":").append(quote(m.startedAt.toString())).append(",")
        sb.append("\"mode\":").append(quote(m.mode)).append(",")
        sb.append("\"transport\":").append(quote(m.transport)).append(",")
        sb.append("\"address\":").append(quote(m.address))
        if (m.metricsAddr.isNotEmpty()) sb.append(",\"metrics_addr\":").append(quote(m.metricsAddr))
        if (m.logPath.isNotEmpty()) sb.append(",\"log_path\":").append(quote(m.logPath))
        if (m.logBytesRotated > 0) sb.append(",\"log_bytes_rotated\":").append(m.logBytesRotated)
        if (m.organismUid.isNotEmpty()) sb.append(",\"organism_uid\":").append(quote(m.organismUid))
        if (m.organismSlug.isNotEmpty()) sb.append(",\"organism_slug\":").append(quote(m.organismSlug))
        if (m.isDefault) sb.append(",\"default\":true")
        sb.append("}")
        val tmp = dir.resolve("meta.json.tmp")
        Files.writeString(tmp, sb.toString())
        Files.move(tmp, dir.resolve("meta.json"), StandardCopyOption.REPLACE_EXISTING)
    }
}

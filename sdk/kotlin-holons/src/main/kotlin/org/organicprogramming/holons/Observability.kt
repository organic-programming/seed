// Kotlin reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

package org.organicprogramming.holons

import com.google.protobuf.Duration
import com.google.protobuf.Timestamp
import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import io.grpc.CallOptions
import io.grpc.ManagedChannel
import io.grpc.MethodDescriptor
import io.grpc.ServerServiceDefinition
import io.grpc.Status
import io.grpc.protobuf.ProtoUtils
import io.grpc.stub.ClientCalls
import io.grpc.stub.ServerCallStreamObserver
import io.grpc.stub.ServerCalls
import java.io.IOException
import java.io.OutputStream
import java.net.InetSocketAddress
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
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import kotlin.concurrent.thread
import kotlinx.coroutines.runBlocking

object Observability {
    private const val HOLON_OBSERVABILITY_SERVICE = "holons.v1.HolonObservability"

    val logsMethod: MethodDescriptor<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogEntry> =
        MethodDescriptor.newBuilder<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogEntry>()
            .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Logs"))
            .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogEntry.getDefaultInstance()))
            .build()

    val metricsMethod: MethodDescriptor<holons.v1.Observability.MetricsRequest, holons.v1.Observability.MetricsSnapshot> =
        MethodDescriptor.newBuilder<holons.v1.Observability.MetricsRequest, holons.v1.Observability.MetricsSnapshot>()
            .setType(MethodDescriptor.MethodType.UNARY)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Metrics"))
            .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.MetricsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.MetricsSnapshot.getDefaultInstance()))
            .build()

    val eventsMethod: MethodDescriptor<holons.v1.Observability.EventsRequest, holons.v1.Observability.EventInfo> =
        MethodDescriptor.newBuilder<holons.v1.Observability.EventsRequest, holons.v1.Observability.EventInfo>()
            .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Events"))
            .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.EventsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.EventInfo.getDefaultInstance()))
            .build()

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

    enum class EventType(val code: Int) {
        UNSPECIFIED(0),
        INSTANCE_SPAWNED(1), INSTANCE_READY(2), INSTANCE_EXITED(3), INSTANCE_CRASHED(4),
        SESSION_STARTED(5), SESSION_ENDED(6), HANDLER_PANIC(7), CONFIG_RELOADED(8)
    }

    data class Hop(val slug: String, val instanceUid: String)

    fun parseOpObs(raw: String?): Set<Family> {
        val out = EnumSet.noneOf(Family::class.java)
        if (raw.isNullOrBlank()) return out
        for (part in raw.split(",")) {
            val tok = part.trim()
            if (tok.isEmpty()) continue
            if (tok == "otel") throw InvalidTokenException(tok,
                "otel export is reserved for v2; not implemented in v1")
            if (tok == "sessions") throw InvalidTokenException(tok,
                "sessions are reserved for v2; not implemented in v1")
            if (tok !in V1_TOKENS) throw InvalidTokenException(tok, "unknown OP_OBS token")
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

    fun configure(cfg: Config): Instance = configureFromEnv(cfg, System.getenv())

    fun configureFromEnv(cfg: Config, env: Map<String, String> = System.getenv()): Instance {
        val families = parseOpObs(env["OP_OBS"])
        if (cfg.slug.isEmpty()) {
            cfg.slug = System.getProperty("sun.java.command", "").split(" ").firstOrNull().orEmpty()
        }
        if (cfg.instanceUid.isEmpty()) {
            cfg.instanceUid = newInstanceUid()
        }
        if (cfg.runDir.isNotEmpty()) {
            cfg.runDir = deriveRunDir(cfg.runDir, cfg.slug, cfg.instanceUid)
        }
        val inst = Instance(cfg, families)
        CURRENT.set(inst)
        return inst
    }

    fun fromEnv(base: Config = Config()): Instance = fromEnvMap(base, System.getenv())

    fun fromEnvMap(base: Config = Config(), env: Map<String, String> = System.getenv()): Instance {
        if (base.instanceUid.isEmpty()) base.instanceUid = env["OP_INSTANCE_UID"].orEmpty()
        if (base.organismUid.isEmpty()) base.organismUid = env["OP_ORGANISM_UID"].orEmpty()
        if (base.organismSlug.isEmpty()) base.organismSlug = env["OP_ORGANISM_SLUG"].orEmpty()
        if (base.promAddr.isEmpty()) base.promAddr = env["OP_PROM_ADDR"].orEmpty()
        if (base.runDir.isEmpty()) base.runDir = env["OP_RUN_DIR"].orEmpty()
        return configureFromEnv(base, env)
    }

    fun current(): Instance = CURRENT.get() ?: DISABLED

    fun reset() {
        CURRENT.getAndSet(null)?.close()
    }

    fun deriveRunDir(root: String, slug: String, uid: String): String {
        if (root.isEmpty() || slug.isEmpty() || uid.isEmpty()) return root
        return Paths.get(root, slug, uid).toString()
    }

    private fun newInstanceUid(): String =
        "${ProcessHandle.current().pid()}-${System.nanoTime()}"

    // --- gRPC service ---

    fun service(inst: Instance = current()): ServerServiceDefinition =
        ServerServiceDefinition.builder(HOLON_OBSERVABILITY_SERVICE)
            .addMethod(
                logsMethod,
                ServerCalls.asyncServerStreamingCall { request, observer ->
                    if (!inst.enabled(Family.LOGS) || inst.logRing == null) {
                        observer.onError(
                            Status.FAILED_PRECONDITION
                                .withDescription("logs family is not enabled (OP_OBS)")
                                .asRuntimeException(),
                        )
                        return@asyncServerStreamingCall
                    }
                    val minLevel = if (request.getMinLevelValue() == 0) Level.INFO.code else request.getMinLevelValue()
                    val entries = if (request.hasSince()) {
                        inst.logRing.drainSince(cutoffFromDuration(request.getSince()))
                    } else {
                        inst.logRing.drain()
                    }
                    entries
                        .asSequence()
                        .filter {
                            it.level.code >= minLevel &&
                                (request.sessionIdsList.isEmpty() || request.sessionIdsList.contains(it.sessionId)) &&
                                (request.rpcMethodsList.isEmpty() || request.rpcMethodsList.contains(it.rpcMethod))
                        }
                        .forEach { observer.onNext(toProtoLogEntry(it)) }
                    if (!request.follow) {
                        observer.onCompleted()
                        return@asyncServerStreamingCall
                    }
                    val subscription = inst.logRing.subscribe { entry ->
                        if (entry.level.code < minLevel ||
                            (request.sessionIdsList.isNotEmpty() && !request.sessionIdsList.contains(entry.sessionId)) ||
                            (request.rpcMethodsList.isNotEmpty() && !request.rpcMethodsList.contains(entry.rpcMethod))
                        ) {
                            return@subscribe
                        }
                        runCatching { observer.onNext(toProtoLogEntry(entry)) }
                    }
                    (observer as? ServerCallStreamObserver<*>)?.setOnCancelHandler {
                        closeQuietly(subscription)
                    }
                },
            )
            .addMethod(
                metricsMethod,
                ServerCalls.asyncUnaryCall { request, observer ->
                    val registry = inst.registry
                    if (!inst.enabled(Family.METRICS) || registry == null) {
                        observer.onError(
                            Status.FAILED_PRECONDITION
                                .withDescription("metrics family is not enabled (OP_OBS)")
                                .asRuntimeException(),
                        )
                        return@asyncUnaryCall
                    }
                    val snapshot = holons.v1.Observability.MetricsSnapshot.newBuilder()
                        .setCapturedAt(timestamp(Instant.now()))
                        .setSlug(inst.cfg.slug)
                        .setInstanceUid(inst.cfg.instanceUid)
                    toProtoMetricSamples(registry)
                        .filter { sample ->
                            request.namePrefixesList.isEmpty() ||
                                request.namePrefixesList.any { prefix -> sample.name.startsWith(prefix) }
                        }
                        .forEach { snapshot.addSamples(it) }
                    observer.onNext(snapshot.build())
                    observer.onCompleted()
                },
            )
            .addMethod(
                eventsMethod,
                ServerCalls.asyncServerStreamingCall { request, observer ->
                    val bus = inst.eventBus
                    if (!inst.enabled(Family.EVENTS) || bus == null) {
                        observer.onError(
                            Status.FAILED_PRECONDITION
                                .withDescription("events family is not enabled (OP_OBS)")
                                .asRuntimeException(),
                        )
                        return@asyncServerStreamingCall
                    }
                    val wanted = request.typesValueList.toSet()
                    val events = if (request.hasSince()) {
                        bus.drainSince(cutoffFromDuration(request.getSince()))
                    } else {
                        bus.drain()
                    }
                    events
                        .asSequence()
                        .filter { wanted.isEmpty() || it.type.code in wanted }
                        .forEach { observer.onNext(toProtoEvent(it)) }
                    if (!request.follow) {
                        observer.onCompleted()
                        return@asyncServerStreamingCall
                    }
                    val subscription = bus.subscribe { event ->
                        if (wanted.isNotEmpty() && event.type.code !in wanted) {
                            return@subscribe
                        }
                        runCatching { observer.onNext(toProtoEvent(event)) }
                    }
                    (observer as? ServerCallStreamObserver<*>)?.setOnCancelHandler {
                        closeQuietly(subscription)
                    }
                },
            )
            .build()

    private fun timestamp(instant: Instant): Timestamp =
        Timestamp.newBuilder()
            .setSeconds(instant.epochSecond)
            .setNanos(instant.nano)
            .build()

    private fun cutoffFromDuration(duration: Duration): Instant =
        Instant.now()
            .minusSeconds(duration.seconds.coerceAtLeast(0))
            .minusNanos(duration.nanos.coerceAtLeast(0).toLong())

    fun toProtoLogEntry(entry: LogEntry): holons.v1.Observability.LogEntry {
        val builder = holons.v1.Observability.LogEntry.newBuilder()
            .setTs(timestamp(entry.timestamp))
            .setLevelValue(entry.level.code)
            .setSlug(entry.slug)
            .setInstanceUid(entry.instanceUid)
            .setSessionId(entry.sessionId)
            .setRpcMethod(entry.rpcMethod)
            .setMessage(entry.message)
            .putAllFields(entry.fields)
            .setCaller(entry.caller)
        entry.chain.forEach { builder.addChain(toProtoHop(it)) }
        return builder.build()
    }

    fun fromProtoLogEntry(entry: holons.v1.Observability.LogEntry): LogEntry =
        LogEntry(
            timestamp = if (entry.hasTs()) instant(entry.ts) else Instant.now(),
            level = levelFromCode(entry.levelValue),
            slug = entry.slug,
            instanceUid = entry.instanceUid,
            sessionId = entry.sessionId,
            rpcMethod = entry.rpcMethod,
            message = entry.message,
            fields = entry.fieldsMap,
            caller = entry.caller,
            chain = entry.chainList.map { fromProtoHop(it) },
        )

    fun toProtoMetricSamples(registry: Registry): List<holons.v1.Observability.MetricSample> {
        val samples = mutableListOf<holons.v1.Observability.MetricSample>()
        registry.counters().forEach { counter ->
            samples += holons.v1.Observability.MetricSample.newBuilder()
                .setName(counter.name)
                .putAllLabels(counter.labels)
                .setHelp(counter.help)
                .setCounter(counter.value())
                .build()
        }
        registry.gauges().forEach { gauge ->
            samples += holons.v1.Observability.MetricSample.newBuilder()
                .setName(gauge.name)
                .putAllLabels(gauge.labels)
                .setHelp(gauge.help)
                .setGauge(gauge.value())
                .build()
        }
        registry.histograms().forEach { histogram ->
            samples += holons.v1.Observability.MetricSample.newBuilder()
                .setName(histogram.name)
                .putAllLabels(histogram.labels)
                .setHelp(histogram.help)
                .setHistogram(toProtoHistogram(histogram.snapshot()))
                .build()
        }
        return samples
    }

    private fun toProtoHistogram(snapshot: HistogramSnapshot): holons.v1.Observability.HistogramSample {
        val builder = holons.v1.Observability.HistogramSample.newBuilder()
            .setCount(snapshot.total)
            .setSum(snapshot.sum)
        snapshot.bounds.indices.forEach { index ->
            builder.addBuckets(
                holons.v1.Observability.Bucket.newBuilder()
                    .setUpperBound(snapshot.bounds[index])
                    .setCount(snapshot.counts[index])
                    .build(),
            )
        }
        return builder.build()
    }

    fun toProtoEvent(event: Event): holons.v1.Observability.EventInfo {
        val builder = holons.v1.Observability.EventInfo.newBuilder()
            .setTs(timestamp(event.timestamp))
            .setTypeValue(event.type.code)
            .setSlug(event.slug)
            .setInstanceUid(event.instanceUid)
            .setSessionId(event.sessionId)
            .putAllPayload(event.payload)
        event.chain.forEach { builder.addChain(toProtoHop(it)) }
        return builder.build()
    }

    fun fromProtoEvent(event: holons.v1.Observability.EventInfo): Event =
        Event(
            timestamp = if (event.hasTs()) instant(event.ts) else Instant.now(),
            type = eventTypeFromCode(event.typeValue),
            slug = event.slug,
            instanceUid = event.instanceUid,
            sessionId = event.sessionId,
            payload = event.payloadMap,
            chain = event.chainList.map { fromProtoHop(it) },
        )

    private fun toProtoHop(hop: Hop): holons.v1.Observability.ChainHop =
        holons.v1.Observability.ChainHop.newBuilder()
            .setSlug(hop.slug)
            .setInstanceUid(hop.instanceUid)
            .build()

    private fun fromProtoHop(hop: holons.v1.Observability.ChainHop): Hop =
        Hop(hop.slug, hop.instanceUid)

    private fun instant(ts: Timestamp): Instant =
        Instant.ofEpochSecond(ts.seconds, ts.nanos.toLong())

    private fun levelFromCode(code: Int): Level =
        Level.entries.firstOrNull { it.code == code } ?: Level.UNSET

    private fun eventTypeFromCode(code: Int): EventType =
        EventType.entries.firstOrNull { it.code == code } ?: EventType.UNSPECIFIED

    // --- Prometheus exposition ---

    class PromServer(private val addr: String = ":0") : AutoCloseable {
        private var server: HttpServer? = null

        @Synchronized
        fun start() {
            if (server != null) return
            val (host, port) = parsePromAddr(addr.ifBlank { ":0" })
            server = HttpServer.create(InetSocketAddress(host, port), 0).also { http ->
                http.createContext("/metrics", this::handleMetrics)
                http.start()
            }
        }

        @Synchronized
        fun addrUrl(): String {
            val http = server ?: return ""
            val address = http.address
            return "http://${advertisedPromHost(address.hostString)}:${address.port}/metrics"
        }

        @Synchronized
        override fun close() {
            server?.stop(0)
            server = null
        }

        private fun handleMetrics(exchange: HttpExchange) {
            if (exchange.requestURI?.path != "/metrics") {
                exchange.sendResponseHeaders(404, -1)
                exchange.close()
                return
            }

            val inst = current()
            val body: String
            val status: Int
            if (!inst.enabled(Family.METRICS)) {
                status = 503
                body = "# metrics family disabled (OP_OBS)\n"
            } else if (!inst.enabled(Family.PROM)) {
                status = 503
                body = "# prom family disabled (OP_OBS)\n"
            } else {
                status = 200
                body = toPrometheusText(inst)
            }

            val bytes = body.toByteArray(StandardCharsets.UTF_8)
            exchange.responseHeaders.set("Content-Type", "text/plain; version=0.0.4")
            exchange.sendResponseHeaders(status, bytes.size.toLong())
            exchange.responseBody.use { out: OutputStream -> out.write(bytes) }
        }
    }

    fun toPrometheusText(inst: Instance): String {
        val registry = inst.registry ?: return "# metrics family disabled (OP_OBS)\n"
        if (!inst.enabled(Family.METRICS)) {
            return "# metrics family disabled (OP_OBS)\n"
        }
        val out = StringBuilder()
        registry.counters().forEach { counter ->
            appendPromHelpType(out, counter.name, counter.help, "counter")
            out.append(counter.name)
                .append(promLabels(mergePromLabels(counter.labels, inst)))
                .append(' ')
                .append(counter.value())
                .append('\n')
        }
        registry.gauges().forEach { gauge ->
            appendPromHelpType(out, gauge.name, gauge.help, "gauge")
            out.append(gauge.name)
                .append(promLabels(mergePromLabels(gauge.labels, inst)))
                .append(' ')
                .append(formatPromFloat(gauge.value()))
                .append('\n')
        }
        registry.histograms().forEach { histogram ->
            appendPromHelpType(out, histogram.name, histogram.help, "histogram")
            val labels = mergePromLabels(histogram.labels, inst)
            val snapshot = histogram.snapshot()
            snapshot.bounds.indices.forEach { index ->
                val bucketLabels = labels.toMutableMap()
                bucketLabels["le"] = formatPromFloat(snapshot.bounds[index])
                out.append(histogram.name)
                    .append("_bucket")
                    .append(promLabels(bucketLabels))
                    .append(' ')
                    .append(snapshot.counts[index])
                    .append('\n')
            }
            val infLabels = labels.toMutableMap()
            infLabels["le"] = "+Inf"
            out.append(histogram.name).append("_bucket").append(promLabels(infLabels)).append(' ')
                .append(snapshot.total).append('\n')
            out.append(histogram.name).append("_sum").append(promLabels(labels)).append(' ')
                .append(formatPromFloat(snapshot.sum)).append('\n')
            out.append(histogram.name).append("_count").append(promLabels(labels)).append(' ')
                .append(snapshot.total).append('\n')
        }
        return out.toString()
    }

    private fun appendPromHelpType(out: StringBuilder, name: String, help: String, type: String) {
        out.append("# HELP ").append(name).append(' ').append(promEscapeHelp(help)).append('\n')
        out.append("# TYPE ").append(name).append(' ').append(type).append('\n')
    }

    private fun mergePromLabels(labels: Map<String, String>, inst: Instance): Map<String, String> {
        val out = linkedMapOf<String, String>()
        if (inst.cfg.slug.isNotEmpty()) out["slug"] = inst.cfg.slug
        if (inst.cfg.instanceUid.isNotEmpty()) out["instance_uid"] = inst.cfg.instanceUid
        out.putAll(labels)
        return out
    }

    private fun promLabels(labels: Map<String, String>): String {
        if (labels.isEmpty()) return ""
        return labels.toSortedMap().entries.joinToString(prefix = "{", postfix = "}") { (key, value) ->
            "$key=\"${promEscapeValue(value)}\""
        }
    }

    private fun promEscapeValue(value: String): String =
        value.replace("\\", "\\\\").replace("\n", "\\n").replace("\"", "\\\"")

    private fun promEscapeHelp(value: String): String =
        value.replace("\\", "\\\\").replace("\n", "\\n")

    private fun formatPromFloat(value: Double): String = when {
        value.isNaN() -> "NaN"
        value == Double.POSITIVE_INFINITY -> "+Inf"
        value == Double.NEGATIVE_INFINITY -> "-Inf"
        else -> value.toString()
    }

    private data class HostPort(val host: String, val port: Int)

    private fun parsePromAddr(raw: String): HostPort {
        val trimmed = raw.ifBlank { ":0" }
        if (trimmed.startsWith(":")) {
            return HostPort("0.0.0.0", trimmed.removePrefix(":").ifBlank { "0" }.toInt())
        }
        val idx = trimmed.lastIndexOf(':')
        require(idx >= 0) { "invalid Prometheus address \"$raw\"" }
        val host = trimmed.substring(0, idx).ifBlank { "0.0.0.0" }
        val port = trimmed.substring(idx + 1).toInt()
        return HostPort(host, port)
    }

    private fun advertisedPromHost(host: String): String = when (host) {
        "", "0.0.0.0" -> "127.0.0.1"
        "::" -> "::1"
        else -> host
    }

    // --- Member observability relay ---

    data class MemberIdentity(val slug: String, val uid: String)

    fun resolveMemberIdentity(channel: ManagedChannel, fallbackSlug: String, fallbackUid: String = ""): MemberIdentity {
        if (fallbackUid.isNotBlank()) {
            return MemberIdentity(fallbackSlug.trim(), fallbackUid.trim())
        }
        runCatching {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                eventsMethod,
                CallOptions.DEFAULT,
                holons.v1.Observability.EventsRequest.newBuilder()
                    .addTypesValue(EventType.INSTANCE_READY.code)
                    .build(),
            )
            while (iterator.hasNext()) {
                val event = iterator.next()
                if (event.chainCount == 0 && event.instanceUid.isNotBlank()) {
                    return MemberIdentity(event.slug.ifBlank { fallbackSlug.trim() }, event.instanceUid)
                }
            }
        }
        runCatching {
            val snapshot = ClientCalls.blockingUnaryCall(
                channel,
                metricsMethod,
                CallOptions.DEFAULT,
                holons.v1.Observability.MetricsRequest.getDefaultInstance(),
            )
            if (snapshot.instanceUid.isNotBlank()) {
                return MemberIdentity(snapshot.slug.ifBlank { fallbackSlug.trim() }, snapshot.instanceUid)
            }
        }
        return MemberIdentity(fallbackSlug.trim(), "")
    }

    class MemberRelay(
        private val childSlug: String,
        private val childUid: String,
        private val channel: ManagedChannel,
        private val inst: Instance,
        private val retryDelayMillis: Long = 2000,
    ) : AutoCloseable {
        @Volatile private var stopped = false
        private val threads = CopyOnWriteArrayList<Thread>()

        fun start() {
            if (inst.enabled(Family.LOGS) && inst.logRing != null) {
                startThread("logs") { pumpLogs() }
            }
            if (inst.enabled(Family.EVENTS) && inst.eventBus != null) {
                startThread("events") { pumpEvents() }
            }
        }

        override fun close() {
            stopped = true
            runBlocking { Connect.disconnect(channel) }
            threads.forEach { it.interrupt() }
        }

        private fun startThread(family: String, block: () -> Unit) {
            threads += thread(start = true, isDaemon = true, name = "holons-member-relay-$family-$childSlug") {
                block()
            }
        }

        private fun pumpLogs() {
            while (!stopped) {
                try {
                    val iterator = ClientCalls.blockingServerStreamingCall(
                        channel,
                        logsMethod,
                        CallOptions.DEFAULT,
                        holons.v1.Observability.LogsRequest.newBuilder()
                            .setFollow(true)
                            .setMinLevelValue(Level.INFO.code)
                            .build(),
                    )
                    while (!stopped && iterator.hasNext()) {
                        val entry = fromProtoLogEntry(iterator.next())
                        inst.logRing?.push(entry.copy(chain = appendDirectChild(entry.chain, childSlug, childUid)))
                    }
                } catch (_: Exception) {
                    retryPause()
                }
            }
        }

        private fun pumpEvents() {
            while (!stopped) {
                try {
                    val iterator = ClientCalls.blockingServerStreamingCall(
                        channel,
                        eventsMethod,
                        CallOptions.DEFAULT,
                        holons.v1.Observability.EventsRequest.newBuilder()
                            .setFollow(true)
                            .build(),
                    )
                    while (!stopped && iterator.hasNext()) {
                        val event = fromProtoEvent(iterator.next())
                        inst.eventBus?.emit(event.copy(chain = appendDirectChild(event.chain, childSlug, childUid)))
                    }
                } catch (_: Exception) {
                    retryPause()
                }
            }
        }

        private fun retryPause() {
            if (stopped) return
            try {
                Thread.sleep(retryDelayMillis.coerceAtLeast(1))
            } catch (_: InterruptedException) {
                Thread.currentThread().interrupt()
            }
        }
    }

    private fun closeQuietly(closeable: AutoCloseable?) {
        runCatching { closeable?.close() }
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

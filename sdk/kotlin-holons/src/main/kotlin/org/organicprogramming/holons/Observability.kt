// Kotlin reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

package org.organicprogramming.holons

import com.google.protobuf.Duration
import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import holons.v1.Observability as ObsProto
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
import java.util.concurrent.BlockingQueue
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import kotlin.concurrent.thread
import kotlinx.coroutines.runBlocking

object Observability {
    private const val HOLON_OBSERVABILITY_SERVICE = "holons.v1.HolonObservability"

    const val ATTR_HOLONS_SLUG = "holons.slug"
    const val ATTR_HOLONS_INSTANCE_UID = "holons.instance_uid"
    const val ATTR_HOLONS_SESSION_ID = "holons.session_id"
    const val ATTR_SERVICE_NAME = "service.name"
    const val ATTR_SERVICE_INSTANCE_ID = "service.instance.id"
    const val ATTR_RPC_METHOD = "rpc.method"
    const val ATTR_LOGGER_NAME = "logger.name"
    const val ATTR_CODE_CALLER = "code.caller"

    object EventName {
        const val INSTANCE_SPAWNED = "instance.spawned"
        const val INSTANCE_READY = "instance.ready"
        const val INSTANCE_EXITED = "instance.exited"
        const val INSTANCE_CRASHED = "instance.crashed"
        const val SESSION_STARTED = "session.started"
        const val SESSION_ENDED = "session.ended"
        const val HANDLER_PANIC = "handler.panic"
        const val CONFIG_RELOADED = "config.reloaded"
    }

    val logsMethod: MethodDescriptor<ObsProto.LogsRequest, ObsProto.LogRecord> =
        MethodDescriptor.newBuilder<ObsProto.LogsRequest, ObsProto.LogRecord>()
            .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Logs"))
            .setRequestMarshaller(ProtoUtils.marshaller(ObsProto.LogsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(ObsProto.LogRecord.getDefaultInstance()))
            .build()

    val metricsMethod: MethodDescriptor<ObsProto.MetricsRequest, ObsProto.Metric> =
        MethodDescriptor.newBuilder<ObsProto.MetricsRequest, ObsProto.Metric>()
            .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Metrics"))
            .setRequestMarshaller(ProtoUtils.marshaller(ObsProto.MetricsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(ObsProto.Metric.getDefaultInstance()))
            .build()

    val eventsMethod: MethodDescriptor<ObsProto.EventsRequest, ObsProto.LogRecord> =
        MethodDescriptor.newBuilder<ObsProto.EventsRequest, ObsProto.LogRecord>()
            .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Events"))
            .setRequestMarshaller(ProtoUtils.marshaller(ObsProto.EventsRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(ObsProto.LogRecord.getDefaultInstance()))
            .build()

    enum class Family { LOGS, METRICS, EVENTS, PROM, OTEL }

    private val V1_TOKENS = setOf("logs", "metrics", "events", "prom", "all")

    class InvalidTokenException(
        val token: String,
        reason: String,
        val variable: String = "OP_OBS",
    ) : RuntimeException("$variable: $reason: $token")

    enum class Level(val severity: ObsProto.SeverityNumber) {
        UNSET(ObsProto.SeverityNumber.SEVERITY_NUMBER_UNSPECIFIED),
        TRACE(ObsProto.SeverityNumber.SEVERITY_NUMBER_TRACE),
        DEBUG(ObsProto.SeverityNumber.SEVERITY_NUMBER_DEBUG),
        INFO(ObsProto.SeverityNumber.SEVERITY_NUMBER_INFO),
        WARN(ObsProto.SeverityNumber.SEVERITY_NUMBER_WARN),
        ERROR(ObsProto.SeverityNumber.SEVERITY_NUMBER_ERROR),
        FATAL(ObsProto.SeverityNumber.SEVERITY_NUMBER_FATAL);

        val code: Int get() = severity.number

        fun label(): String = when (this) {
            TRACE -> "TRACE"; DEBUG -> "DEBUG"; INFO -> "INFO"
            WARN -> "WARN"; ERROR -> "ERROR"; FATAL -> "FATAL"
            else -> "UNSPECIFIED"
        }
    }

    data class Hop(val slug: String, val instanceUid: String)

    private val SYSTEM_ATTRIBUTES = setOf(
        ATTR_HOLONS_SLUG,
        ATTR_HOLONS_INSTANCE_UID,
        ATTR_HOLONS_SESSION_ID,
        ATTR_SERVICE_NAME,
        ATTR_SERVICE_INSTANCE_ID,
        ATTR_RPC_METHOD,
        ATTR_LOGGER_NAME,
        ATTR_CODE_CALLER,
    )

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

    fun appendDirectChild(src: List<String>?, childSlug: String, childUid: String = ""): List<String> {
        childUid
        val base = src ?: emptyList()
        return if (childSlug.isBlank()) base else base + childSlug
    }

    fun enrichForMultilog(wire: List<String>?, srcSlug: String, srcUid: String = ""): List<String> =
        appendDirectChild(wire, srcSlug, srcUid)

    data class LogRecord(
        val timestamp: Instant,
        val level: Level,
        val slug: String,
        val instanceUid: String,
        val sessionId: String = "",
        val rpcMethod: String = "",
        val message: String,
        val fields: Map<String, Any?> = emptyMap(),
        val caller: String = "",
        val eventName: String = "",
        val chain: List<String> = emptyList(),
        val privateEntry: Boolean = false,
    )

    class LogRing(capacity: Int = 1024) {
        private val capacity = capacity.coerceAtLeast(1)
        private val buf = ArrayDeque<LogRecord>(this.capacity)
        private val subs = CopyOnWriteArrayList<(LogRecord) -> Unit>()

        @Synchronized
        fun push(e: LogRecord) {
            if (buf.size == capacity) buf.removeFirst()
            buf.addLast(e)
            subs.forEach { runCatching { it(e) } }
        }

        @Synchronized fun drain(): List<LogRecord> = buf.toList()
        @Synchronized fun drainSince(cutoff: Instant): List<LogRecord> =
            buf.filter { !it.timestamp.isBefore(cutoff) }
        fun subscribe(fn: (LogRecord) -> Unit): AutoCloseable {
            subs.add(fn)
            return AutoCloseable { subs.remove(fn) }
        }
        @Synchronized
        fun replayAndSubscribe(cutoff: Instant? = null, bufferSize: Int = 128): ReplaySubscription<LogRecord> {
            val replay = cutoff?.let { drainSince(it) } ?: buf.toList()
            val queue = LinkedBlockingQueue<LogRecord>(bufferSize.coerceAtLeast(1))
            val fn: (LogRecord) -> Unit = { entry -> queue.offer(entry) }
            // Snapshot and live registration share this critical section so
            // follow=true streams cannot drop entries between replay and live delivery.
            subs.add(fn)
            return ReplaySubscription(replay, queue, AutoCloseable { subs.remove(fn) })
        }
        @Synchronized fun size(): Int = buf.size
    }

    class EventBus(capacity: Int = 256) {
        private val capacity = capacity.coerceAtLeast(1)
        private val buf = ArrayDeque<LogRecord>(this.capacity)
        private val subs = CopyOnWriteArrayList<(LogRecord) -> Unit>()
        @Volatile private var closed = false

        @Synchronized
        fun emit(e: LogRecord) {
            if (closed) return
            if (buf.size == capacity) buf.removeFirst()
            buf.addLast(e)
            subs.forEach { runCatching { it(e) } }
        }

        @Synchronized fun drain(): List<LogRecord> = buf.toList()
        @Synchronized fun drainSince(cutoff: Instant): List<LogRecord> =
            buf.filter { !it.timestamp.isBefore(cutoff) }
        fun subscribe(fn: (LogRecord) -> Unit): AutoCloseable {
            subs.add(fn)
            return AutoCloseable { subs.remove(fn) }
        }
        @Synchronized
        fun replayAndSubscribe(cutoff: Instant? = null, bufferSize: Int = 64): ReplaySubscription<LogRecord> {
            val replay = cutoff?.let { drainSince(it) } ?: buf.toList()
            val queue = LinkedBlockingQueue<LogRecord>(bufferSize.coerceAtLeast(1))
            val fn: (LogRecord) -> Unit = { event -> queue.offer(event) }
            // Snapshot and live registration share this critical section so
            // follow=true streams cannot drop entries between replay and live delivery.
            subs.add(fn)
            return ReplaySubscription(replay, queue, AutoCloseable { subs.remove(fn) })
        }
        fun close() { closed = true; subs.clear() }
    }

    data class ReplaySubscription<T>(
        val replay: List<T>,
        val live: BlockingQueue<T>,
        private val closeable: AutoCloseable,
    ) : AutoCloseable {
        override fun close() {
            runCatching { closeable.close() }
        }
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

        fun log(l: Level, message: String, fields: Map<String, Any?>? = null, privateEntry: Boolean = false) {
            if (!enabled(l)) return
            val redact = obs.cfg.redactedFields.toSet()
            val out = linkedMapOf<String, Any?>()
            fields?.forEach { (k, v) ->
                if (k.isNotEmpty()) {
                    out[k] = if (k in redact) "<redacted>" else v
                }
            }
            val entry = LogRecord(
                timestamp = Instant.now(),
                level = l,
                slug = obs.cfg.slug,
                instanceUid = obs.cfg.instanceUid,
                message = message,
                fields = out,
                caller = name,
                privateEntry = privateEntry,
            )
            obs.logRing?.push(entry)
        }

        fun trace(m: String, f: Map<String, Any?>? = null) = log(Level.TRACE, m, f)
        fun debug(m: String, f: Map<String, Any?>? = null) = log(Level.DEBUG, m, f)
        fun info(m: String, f: Map<String, Any?>? = null)  = log(Level.INFO, m, f)
        fun warn(m: String, f: Map<String, Any?>? = null)  = log(Level.WARN, m, f)
        fun error(m: String, f: Map<String, Any?>? = null) = log(Level.ERROR, m, f)
        fun fatal(m: String, f: Map<String, Any?>? = null) = log(Level.FATAL, m, f)
        fun privateInfo(m: String, f: Map<String, Any?>? = null) = log(Level.INFO, m, f, privateEntry = true)
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

        fun emit(eventName: String, payload: Map<String, Any?>? = null, privateEntry: Boolean = false) {
            val bus = eventBus ?: return
            val redact = cfg.redactedFields.toSet()
            val p = linkedMapOf<String, Any?>()
            payload?.forEach { (k, v) -> p[k] = if (k in redact) "<redacted>" else v }
            bus.emit(
                LogRecord(
                    timestamp = Instant.now(),
                    level = Level.INFO,
                    slug = cfg.slug,
                    instanceUid = cfg.instanceUid,
                    message = eventName,
                    fields = p,
                    eventName = eventName,
                    privateEntry = privateEntry,
                ),
            )
        }

        fun emitPrivate(eventName: String, payload: Map<String, Any?>? = null) =
            emit(eventName, payload, privateEntry = true)

        fun close() { eventBus?.close() }
    }

    private val CURRENT = AtomicReference<Instance?>(null)
    private val DISABLED = Instance(Config(), EnumSet.noneOf(Family::class.java))
    private val DISABLED_LOGGER = Logger(DISABLED, "")

    fun configure(cfg: Config): Instance = configureFromEnv(cfg, System.getenv())

    fun configureFromEnv(cfg: Config, env: Map<String, String> = System.getenv()): Instance {
        val families = parseOpObs(env["OP_OBS"])
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
                    val minLevel = if (request.getMinSeverityNumberValue() == 0) {
                        Level.INFO.code
                    } else {
                        request.getMinSeverityNumberValue()
                    }
                    val subscription = if (request.follow) {
                        inst.logRing.replayAndSubscribe(
                            cutoff = if (request.hasSince()) cutoffFromDuration(request.getSince()) else null,
                            bufferSize = 128,
                        )
                    } else {
                        null
                    }
                    val entries = subscription?.replay ?: if (request.hasSince()) {
                        inst.logRing.drainSince(cutoffFromDuration(request.getSince()))
                    } else {
                        inst.logRing.drain()
                    }
                    entries
                        .asSequence()
                        .filter {
                            (!request.follow || !it.privateEntry) &&
                            it.level.code >= minLevel &&
                                (request.sessionIdsList.isEmpty() || request.sessionIdsList.contains(it.sessionId)) &&
                                (request.rpcMethodsList.isEmpty() || request.rpcMethodsList.contains(it.rpcMethod))
                        }
                        .forEach { observer.onNext(toProtoLogRecord(it)) }
                    if (!request.follow) {
                        observer.onCompleted()
                        return@asyncServerStreamingCall
                    }
                    val liveSubscription = subscription ?: return@asyncServerStreamingCall
                    val liveThread = thread(start = true, isDaemon = true, name = "holons-observability-logs-follow") {
                        try {
                            while (!Thread.currentThread().isInterrupted) {
                                val entry = liveSubscription.live.take()
                                if (entry.privateEntry ||
                                    entry.level.code < minLevel ||
                                    (request.sessionIdsList.isNotEmpty() && !request.sessionIdsList.contains(entry.sessionId)) ||
                                    (request.rpcMethodsList.isNotEmpty() && !request.rpcMethodsList.contains(entry.rpcMethod))
                                ) {
                                    continue
                                }
                                observer.onNext(toProtoLogRecord(entry))
                            }
                        } catch (_: InterruptedException) {
                            Thread.currentThread().interrupt()
                        } catch (_: Exception) {
                            // Cancelled clients are handled by the gRPC runtime.
                        } finally {
                            closeQuietly(liveSubscription)
                        }
                    }
                    (observer as? ServerCallStreamObserver<*>)?.setOnCancelHandler {
                        closeQuietly(liveSubscription)
                        liveThread.interrupt()
                    }
                },
            )
            .addMethod(
                metricsMethod,
                ServerCalls.asyncServerStreamingCall { request, observer ->
                    val registry = inst.registry
                    if (!inst.enabled(Family.METRICS) || registry == null) {
                        observer.onError(
                            Status.FAILED_PRECONDITION
                                .withDescription("metrics family is not enabled (OP_OBS)")
                                .asRuntimeException(),
                        )
                        return@asyncServerStreamingCall
                    }
                    toProtoMetrics(registry, inst.cfg.slug, inst.cfg.instanceUid, Instant.now())
                        .filter { metric ->
                            request.namePrefixesList.isEmpty() ||
                                request.namePrefixesList.any { prefix -> metric.name.startsWith(prefix) }
                        }
                        .forEach { observer.onNext(it) }
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
                    val wanted = request.eventNamesList.toSet()
                    val subscription = if (request.follow) {
                        bus.replayAndSubscribe(
                            cutoff = if (request.hasSince()) cutoffFromDuration(request.getSince()) else null,
                            bufferSize = 64,
                        )
                    } else {
                        null
                    }
                    val events = subscription?.replay ?: if (request.hasSince()) {
                        bus.drainSince(cutoffFromDuration(request.getSince()))
                    } else {
                        bus.drain()
                    }
                    events
                        .asSequence()
                        .filter { (!request.follow || !it.privateEntry) && (wanted.isEmpty() || it.eventName in wanted) }
                        .forEach { observer.onNext(toProtoLogRecord(it)) }
                    if (!request.follow) {
                        observer.onCompleted()
                        return@asyncServerStreamingCall
                    }
                    val liveSubscription = subscription ?: return@asyncServerStreamingCall
                    val liveThread = thread(start = true, isDaemon = true, name = "holons-observability-events-follow") {
                        try {
                            while (!Thread.currentThread().isInterrupted) {
                                val event = liveSubscription.live.take()
                                if (event.privateEntry || (wanted.isNotEmpty() && event.eventName !in wanted)) {
                                    continue
                                }
                                observer.onNext(toProtoLogRecord(event))
                            }
                        } catch (_: InterruptedException) {
                            Thread.currentThread().interrupt()
                        } catch (_: Exception) {
                            // Cancelled clients are handled by the gRPC runtime.
                        } finally {
                            closeQuietly(liveSubscription)
                        }
                    }
                    (observer as? ServerCallStreamObserver<*>)?.setOnCancelHandler {
                        closeQuietly(liveSubscription)
                        liveThread.interrupt()
                    }
                },
            )
            .build()

    private fun cutoffFromDuration(duration: Duration): Instant =
        Instant.now()
            .minusSeconds(duration.seconds.coerceAtLeast(0))
            .minusNanos(duration.nanos.coerceAtLeast(0).toLong())

    fun toProtoLogRecord(entry: LogRecord): ObsProto.LogRecord {
        val now = unixNano(entry.timestamp)
        val builder = ObsProto.LogRecord.newBuilder()
            .setTimeUnixNano(now)
            .setObservedTimeUnixNano(now)
            .setSeverityNumber(entry.level.severity)
            .setSeverityText(entry.level.label())
            .setBody(toAnyValue(entry.message))
            .addAllAttributes(logAttributes(entry))
            .addAllChain(entry.chain)
        if (entry.eventName.isNotEmpty()) {
            builder.eventName = entry.eventName
        }
        return builder.build()
    }

    fun fromProtoLogRecord(record: ObsProto.LogRecord): LogRecord =
        LogRecord(
            timestamp = if (record.timeUnixNano == 0L) Instant.now() else instantFromUnixNano(record.timeUnixNano),
            level = levelFromCode(record.severityNumberValue),
            slug = stringAttribute(record.attributesList, ATTR_HOLONS_SLUG),
            instanceUid = stringAttribute(record.attributesList, ATTR_HOLONS_INSTANCE_UID),
            sessionId = stringAttribute(record.attributesList, ATTR_HOLONS_SESSION_ID),
            rpcMethod = stringAttribute(record.attributesList, ATTR_RPC_METHOD),
            message = anyValueString(record.body),
            fields = userAttributes(record.attributesList),
            caller = stringAttribute(record.attributesList, ATTR_LOGGER_NAME),
            eventName = record.eventName,
            chain = record.chainList.toList(),
        )

    fun toProtoMetrics(
        registry: Registry,
        slug: String,
        instanceUid: String,
        capturedAt: Instant = Instant.now(),
        startedAt: Instant = capturedAt,
    ): List<ObsProto.Metric> {
        val metrics = mutableListOf<ObsProto.Metric>()
        val startNano = unixNano(startedAt)
        val timeNano = unixNano(capturedAt)
        registry.counters().forEach { counter ->
            metrics += ObsProto.Metric.newBuilder()
                .setName(counter.name)
                .setDescription(counter.help)
                .setSum(
                    ObsProto.Sum.newBuilder()
                        .setAggregationTemporality(ObsProto.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE)
                        .setIsMonotonic(true)
                        .addDataPoints(
                            ObsProto.NumberDataPoint.newBuilder()
                                .setStartTimeUnixNano(startNano)
                                .setTimeUnixNano(timeNano)
                                .setAsInt(counter.value())
                                .addAllAttributes(metricAttributes(slug, instanceUid, counter.labels))
                                .build(),
                        )
                        .build(),
                )
                .build()
        }
        registry.gauges().forEach { gauge ->
            metrics += ObsProto.Metric.newBuilder()
                .setName(gauge.name)
                .setDescription(gauge.help)
                .setGauge(
                    ObsProto.Gauge.newBuilder()
                        .addDataPoints(
                            ObsProto.NumberDataPoint.newBuilder()
                                .setStartTimeUnixNano(startNano)
                                .setTimeUnixNano(timeNano)
                                .setAsDouble(gauge.value())
                                .addAllAttributes(metricAttributes(slug, instanceUid, gauge.labels))
                                .build(),
                        )
                        .build(),
                )
                .build()
        }
        registry.histograms().forEach { histogram ->
            val snap = histogram.snapshot()
            metrics += ObsProto.Metric.newBuilder()
                .setName(histogram.name)
                .setDescription(histogram.help)
                .setHistogram(
                    ObsProto.Histogram.newBuilder()
                        .setAggregationTemporality(ObsProto.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE)
                        .addDataPoints(
                            ObsProto.HistogramDataPoint.newBuilder()
                                .setStartTimeUnixNano(startNano)
                                .setTimeUnixNano(timeNano)
                                .setCount(snap.total)
                                .setSum(snap.sum)
                                .addAllBucketCounts(histogramBucketCounts(snap))
                                .addAllExplicitBounds(snap.bounds.toList())
                                .addAllAttributes(metricAttributes(slug, instanceUid, histogram.labels))
                                .build(),
                        )
                        .build(),
                )
                .build()
        }
        return metrics
    }

    private fun histogramBucketCounts(snapshot: HistogramSnapshot): List<Long> {
        val out = mutableListOf<Long>()
        var previous = 0L
        snapshot.counts.forEach { cumulative ->
            out += (cumulative - previous).coerceAtLeast(0)
            previous = cumulative
        }
        out += (snapshot.total - previous).coerceAtLeast(0)
        return out
    }

    fun toProtoEvent(event: LogRecord): ObsProto.LogRecord = toProtoLogRecord(event)
    fun fromProtoEvent(event: ObsProto.LogRecord): LogRecord = fromProtoLogRecord(event)

    private fun levelFromCode(code: Int): Level =
        Level.entries.firstOrNull { it.code == code } ?: Level.UNSET

    fun toAnyValue(value: Any?): ObsProto.AnyValue {
        val builder = ObsProto.AnyValue.newBuilder()
        when (value) {
            null -> builder.stringValue = ""
            is String -> builder.stringValue = value
            is Boolean -> builder.boolValue = value
            is Byte -> builder.intValue = value.toLong()
            is Short -> builder.intValue = value.toLong()
            is Int -> builder.intValue = value.toLong()
            is Long -> builder.intValue = value
            is Float -> builder.doubleValue = value.toDouble()
            is Double -> builder.doubleValue = value
            else -> builder.stringValue = value.toString()
        }
        return builder.build()
    }

    private fun keyValue(key: String, value: Any?): ObsProto.KeyValue =
        ObsProto.KeyValue.newBuilder()
            .setKey(key)
            .setValue(toAnyValue(value))
            .build()

    private fun resourceAttributes(slug: String, uid: String, sessionId: String = ""): List<ObsProto.KeyValue> =
        listOf(
            keyValue(ATTR_HOLONS_SLUG, slug),
            keyValue(ATTR_SERVICE_NAME, slug),
            keyValue(ATTR_HOLONS_INSTANCE_UID, uid),
            keyValue(ATTR_SERVICE_INSTANCE_ID, uid),
            keyValue(ATTR_HOLONS_SESSION_ID, sessionId),
        )

    private fun logAttributes(entry: LogRecord): List<ObsProto.KeyValue> {
        val out = mutableListOf<ObsProto.KeyValue>()
        out += resourceAttributes(entry.slug, entry.instanceUid, entry.sessionId)
        if (entry.rpcMethod.isNotEmpty()) out += keyValue(ATTR_RPC_METHOD, entry.rpcMethod)
        if (entry.caller.isNotEmpty()) out += keyValue(ATTR_LOGGER_NAME, entry.caller)
        entry.fields.toSortedMap().forEach { (key, value) ->
            if (key.isNotEmpty()) out += keyValue(key, value)
        }
        return out
    }

    private fun metricAttributes(
        slug: String,
        instanceUid: String,
        labels: Map<String, String>,
    ): List<ObsProto.KeyValue> {
        val out = mutableListOf<ObsProto.KeyValue>()
        out += resourceAttributes(slug, instanceUid, "")
        labels.toSortedMap().forEach { (key, value) ->
            if (key.isNotEmpty()) out += keyValue(key, value)
        }
        return out
    }

    fun stringAttribute(attrs: List<ObsProto.KeyValue>, key: String): String =
        attrs.firstOrNull { it.key == key }?.value?.let(::anyValueString).orEmpty()

    fun anyValueString(value: ObsProto.AnyValue): String = when (value.valueCase) {
        ObsProto.AnyValue.ValueCase.STRING_VALUE -> value.stringValue
        ObsProto.AnyValue.ValueCase.BOOL_VALUE -> value.boolValue.toString()
        ObsProto.AnyValue.ValueCase.INT_VALUE -> value.intValue.toString()
        ObsProto.AnyValue.ValueCase.DOUBLE_VALUE -> value.doubleValue.toString()
        else -> ""
    }

    private fun userAttributes(attrs: List<ObsProto.KeyValue>): Map<String, Any?> =
        attrs
            .filter { it.key !in SYSTEM_ATTRIBUTES }
            .associate { it.key to nativeAnyValue(it.value) }

    private fun nativeAnyValue(value: ObsProto.AnyValue): Any? = when (value.valueCase) {
        ObsProto.AnyValue.ValueCase.STRING_VALUE -> value.stringValue
        ObsProto.AnyValue.ValueCase.BOOL_VALUE -> value.boolValue
        ObsProto.AnyValue.ValueCase.INT_VALUE -> value.intValue
        ObsProto.AnyValue.ValueCase.DOUBLE_VALUE -> value.doubleValue
        else -> null
    }

    private fun unixNano(instant: Instant): Long =
        Math.addExact(Math.multiplyExact(instant.epochSecond, 1_000_000_000L), instant.nano.toLong())

    private fun instantFromUnixNano(nanos: Long): Instant =
        Instant.ofEpochSecond(nanos / 1_000_000_000L, nanos % 1_000_000_000L)

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
                ObsProto.EventsRequest.newBuilder()
                    .addEventNames(EventName.INSTANCE_READY)
                    .build(),
            )
            while (iterator.hasNext()) {
                val event = iterator.next()
                val uid = stringAttribute(event.attributesList, ATTR_HOLONS_INSTANCE_UID)
                val slug = stringAttribute(event.attributesList, ATTR_HOLONS_SLUG)
                if (event.chainCount == 0 && uid.isNotBlank()) {
                    return MemberIdentity(slug.ifBlank { fallbackSlug.trim() }, uid)
                }
            }
        }
        runCatching {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                metricsMethod,
                CallOptions.DEFAULT,
                ObsProto.MetricsRequest.getDefaultInstance(),
            )
            while (iterator.hasNext()) {
                val metric = iterator.next()
                val attrs = metricAttributes(metric)
                val uid = stringAttribute(attrs, ATTR_HOLONS_INSTANCE_UID)
                val slug = stringAttribute(attrs, ATTR_HOLONS_SLUG)
                if (uid.isNotBlank()) {
                    return MemberIdentity(slug.ifBlank { fallbackSlug.trim() }, uid)
                }
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
                        ObsProto.LogsRequest.newBuilder()
                            .setFollow(true)
                            .setMinSeverityNumberValue(Level.INFO.code)
                            .build(),
                    )
                    while (!stopped && iterator.hasNext()) {
                        val entry = fromProtoLogRecord(iterator.next())
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
                        ObsProto.EventsRequest.newBuilder()
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

    private fun metricAttributes(metric: ObsProto.Metric): List<ObsProto.KeyValue> = when (metric.dataCase) {
        ObsProto.Metric.DataCase.SUM -> metric.sum.dataPointsList.firstOrNull()?.attributesList.orEmpty()
        ObsProto.Metric.DataCase.GAUGE -> metric.gauge.dataPointsList.firstOrNull()?.attributesList.orEmpty()
        ObsProto.Metric.DataCase.HISTOGRAM -> metric.histogram.dataPointsList.firstOrNull()?.attributesList.orEmpty()
        else -> emptyList()
    }

    private fun appendLog(fp: Path, e: LogRecord) {
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

    private fun appendEvent(fp: Path, e: LogRecord) {
        val sb = StringBuilder()
        sb.append("{\"kind\":\"event\"")
          .append(",\"ts\":\"").append(e.timestamp).append("\"")
          .append(",\"event_name\":\"").append(e.eventName).append("\"")
          .append(",\"slug\":").append(quote(e.slug))
          .append(",\"instance_uid\":").append(quote(e.instanceUid))
        if (e.sessionId.isNotEmpty()) sb.append(",\"session_id\":").append(quote(e.sessionId))
        if (e.fields.isNotEmpty()) { sb.append(",\"payload\":"); jsonMap(sb, e.fields) }
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

    private fun jsonMap(sb: StringBuilder, m: Map<String, Any?>) {
        sb.append("{"); var first = true
        m.forEach { (k, v) ->
            if (!first) sb.append(","); first = false
            sb.append(quote(k)).append(":").append(quote(v?.toString().orEmpty()))
        }
        sb.append("}")
    }

    private fun jsonChain(sb: StringBuilder, c: List<String>) {
        sb.append("["); var first = true
        c.forEach { slug ->
            if (!first) sb.append(","); first = false
            sb.append(quote(slug))
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

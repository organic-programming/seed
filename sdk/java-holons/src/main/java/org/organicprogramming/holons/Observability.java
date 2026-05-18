// Java reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

package org.organicprogramming.holons;

import com.google.protobuf.Duration;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.MethodDescriptor;
import io.grpc.ServerServiceDefinition;
import io.grpc.Status;
import io.grpc.protobuf.ProtoUtils;
import io.grpc.stub.ClientCalls;
import io.grpc.stub.ServerCallStreamObserver;
import io.grpc.stub.ServerCalls;

import java.io.BufferedWriter;
import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Consumer;

public final class Observability {
    private static final String HOLON_OBSERVABILITY_SERVICE = "holons.v1.HolonObservability";

    public static final String ATTR_HOLONS_SLUG = "holons.slug";
    public static final String ATTR_HOLONS_INSTANCE_UID = "holons.instance_uid";
    public static final String ATTR_HOLONS_SESSION_ID = "holons.session_id";
    public static final String ATTR_HOLONS_TRANSPORT = "holons.transport";
    public static final String ATTR_SERVICE_NAME = "service.name";
    public static final String ATTR_SERVICE_INSTANCE_ID = "service.instance.id";
    public static final String ATTR_RPC_METHOD = "rpc.method";
    public static final String ATTR_LOGGER_NAME = "logger.name";

    public static final String EVENT_INSTANCE_SPAWNED = "instance.spawned";
    public static final String EVENT_INSTANCE_READY = "instance.ready";
    public static final String EVENT_INSTANCE_EXITED = "instance.exited";
    public static final String EVENT_INSTANCE_CRASHED = "instance.crashed";
    public static final String EVENT_SESSION_STARTED = "session.started";
    public static final String EVENT_SESSION_ENDED = "session.ended";
    public static final String EVENT_HANDLER_PANIC = "handler.panic";
    public static final String EVENT_CONFIG_RELOADED = "config.reloaded";

    private static final MethodDescriptor<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogRecord> LOGS_METHOD =
            MethodDescriptor.<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogRecord>newBuilder()
                    .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Logs"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogRecord.getDefaultInstance()))
                    .build();

    private static final MethodDescriptor<holons.v1.Observability.MetricsRequest, holons.v1.Observability.Metric> METRICS_METHOD =
            MethodDescriptor.<holons.v1.Observability.MetricsRequest, holons.v1.Observability.Metric>newBuilder()
                    .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Metrics"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.MetricsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.Metric.getDefaultInstance()))
                    .build();

    private static final MethodDescriptor<holons.v1.Observability.EventsRequest, holons.v1.Observability.LogRecord> EVENTS_METHOD =
            MethodDescriptor.<holons.v1.Observability.EventsRequest, holons.v1.Observability.LogRecord>newBuilder()
                    .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Events"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.EventsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogRecord.getDefaultInstance()))
                    .build();

    // --- Families & tokens ---

    public enum Family {
        LOGS, METRICS, EVENTS, PROM, OTEL
    }

    private static final Set<String> V1_TOKENS = Set.of("logs", "metrics", "events", "prom", "all");

    public static final class InvalidTokenException extends RuntimeException {
        public final String token;
        public final String variable;
        public InvalidTokenException(String token, String reason) {
            this("OP_OBS", token, reason);
        }
        public InvalidTokenException(String variable, String token, String reason) {
            super(variable + ": " + reason + ": " + token);
            this.variable = variable;
            this.token = token;
        }
    }

    public static Set<Family> parseOpObs(String raw) {
        EnumSet<Family> out = EnumSet.noneOf(Family.class);
        if (raw == null) return out;
        String t = raw.trim();
        if (t.isEmpty()) return out;
        for (String part : t.split(",")) {
            String tok = part.trim();
            if (tok.isEmpty()) continue;
            if (tok.equals("otel")) {
                throw new InvalidTokenException(tok, "otel export is reserved for v2; not implemented in v1");
            }
            if (tok.equals("sessions")) {
                throw new InvalidTokenException(tok, "sessions are reserved for v2; not implemented in v1");
            }
            if (!V1_TOKENS.contains(tok)) {
                throw new InvalidTokenException(tok, "unknown OP_OBS token");
            }
            if (tok.equals("all")) {
                out.add(Family.LOGS); out.add(Family.METRICS);
                out.add(Family.EVENTS); out.add(Family.PROM);
            } else {
                out.add(Family.valueOf(tok.toUpperCase(Locale.ROOT)));
            }
        }
        return out;
    }

    public static void checkEnv(Map<String, String> env) {
        String sessions = (env != null ? env.getOrDefault("OP_SESSIONS", "") :
                System.getenv().getOrDefault("OP_SESSIONS", "")).trim();
        if (!sessions.isEmpty()) {
            throw new InvalidTokenException("OP_SESSIONS", sessions, "sessions are reserved for v2; not implemented in v1");
        }
        String raw = (env != null ? env.getOrDefault("OP_OBS", "") :
                System.getenv().getOrDefault("OP_OBS", "")).trim();
        if (raw.isEmpty()) return;
        for (String part : raw.split(",")) {
            String tok = part.trim();
            if (tok.isEmpty()) continue;
            if (tok.equals("otel")) {
                throw new InvalidTokenException(tok, "otel export is reserved for v2; not implemented in v1");
            }
            if (tok.equals("sessions")) {
                throw new InvalidTokenException(tok, "sessions are reserved for v2; not implemented in v1");
            }
            if (!V1_TOKENS.contains(tok)) {
                throw new InvalidTokenException(tok, "unknown OP_OBS token");
            }
        }
    }

    public static void checkEnv() { checkEnv(null); }

    // --- Levels / records ---

    public enum Level {
        UNSET(0), TRACE(1), DEBUG(5), INFO(9), WARN(13), ERROR(17), FATAL(21);
        public final int code;
        Level(int c) { this.code = c; }
        public String label() {
            return switch (this) {
                case TRACE -> "TRACE"; case DEBUG -> "DEBUG"; case INFO -> "INFO";
                case WARN -> "WARN"; case ERROR -> "ERROR"; case FATAL -> "FATAL";
                default -> "UNSPECIFIED";
            };
        }
    }

    public static final class Hop {
        public final String slug;
        public final String instanceUid;
        public Hop(String slug, String instanceUid) { this.slug = slug; this.instanceUid = instanceUid; }
    }

    public static List<String> appendDirectChild(List<String> src, String childSlug, String childUid) {
        List<String> out = new ArrayList<>(src != null ? src : List.of());
        out.add(childSlug == null ? "" : childSlug);
        return out;
    }

    public static List<String> enrichForMultilog(List<String> wire, String srcSlug, String srcUid) {
        return appendDirectChild(wire, srcSlug, srcUid);
    }

    public static final class LogRecord {
        public final holons.v1.Observability.LogRecord record;
        public final boolean privateEntry;

        public LogRecord(holons.v1.Observability.LogRecord record) {
            this(record, false);
        }

        public LogRecord(holons.v1.Observability.LogRecord record, boolean privateEntry) {
            this.record = record == null
                    ? holons.v1.Observability.LogRecord.getDefaultInstance()
                    : record.toBuilder().build();
            this.privateEntry = privateEntry;
        }

        public Instant timestamp() {
            long nanos = record.getTimeUnixNano();
            return nanos == 0 ? Instant.EPOCH : Instant.ofEpochSecond(0, nanos);
        }

        public String bodyString() {
            return anyValueString(record.getBody());
        }

        public String attr(String key) {
            return stringAttribute(record.getAttributesList(), key);
        }
    }

    public static final class LogRing {
        private final int capacity;
        private final ArrayDeque<LogRecord> buf;
        private final CopyOnWriteArrayList<Consumer<LogRecord>> subs = new CopyOnWriteArrayList<>();

        public LogRing(int capacity) {
            this.capacity = Math.max(1, capacity);
            this.buf = new ArrayDeque<>(this.capacity);
        }

        public synchronized void push(LogRecord e) {
            if (buf.size() == capacity) buf.removeFirst();
            buf.addLast(e);
            for (Consumer<LogRecord> fn : subs) {
                try { fn.accept(e); } catch (Exception ignored) {}
            }
        }

        public synchronized List<LogRecord> drain() {
            return new ArrayList<>(buf);
        }

        public synchronized List<LogRecord> drainSince(Instant cutoff) {
            List<LogRecord> out = new ArrayList<>();
            for (LogRecord e : buf) if (!e.timestamp().isBefore(cutoff)) out.add(e);
            return out;
        }

        public AutoCloseable subscribe(Consumer<LogRecord> fn) {
            subs.add(fn);
            return () -> subs.remove(fn);
        }

        public synchronized ReplaySubscription<LogRecord> replayAndSubscribe(Instant cutoff, int bufferSize) {
            List<LogRecord> replay = cutoff == null ? new ArrayList<>(buf) : drainSince(cutoff);
            BlockingQueue<LogRecord> queue = new LinkedBlockingQueue<>(Math.max(1, bufferSize));
            Consumer<LogRecord> fn = entry -> queue.offer(entry);
            // Snapshot and subscription are in one critical section, so a
            // follow=true stream cannot miss emissions at the replay/live seam.
            subs.add(fn);
            return new ReplaySubscription<>(replay, queue, () -> subs.remove(fn));
        }

        public synchronized int size() { return buf.size(); }
    }

    public static final class EventBus {
        private final int capacity;
        private final ArrayDeque<LogRecord> buf;
        private final CopyOnWriteArrayList<Consumer<LogRecord>> subs = new CopyOnWriteArrayList<>();
        private volatile boolean closed;

        public EventBus(int capacity) {
            this.capacity = Math.max(1, capacity);
            this.buf = new ArrayDeque<>(this.capacity);
        }

        public synchronized void emit(LogRecord e) {
            if (closed) return;
            if (buf.size() == capacity) buf.removeFirst();
            buf.addLast(e);
            for (Consumer<LogRecord> fn : subs) {
                try { fn.accept(e); } catch (Exception ignored) {}
            }
        }

        public synchronized List<LogRecord> drain() { return new ArrayList<>(buf); }
        public synchronized List<LogRecord> drainSince(Instant cutoff) {
            List<LogRecord> out = new ArrayList<>();
            for (LogRecord e : buf) if (!e.timestamp().isBefore(cutoff)) out.add(e);
            return out;
        }

        public AutoCloseable subscribe(Consumer<LogRecord> fn) {
            subs.add(fn);
            return () -> subs.remove(fn);
        }

        public synchronized ReplaySubscription<LogRecord> replayAndSubscribe(Instant cutoff, int bufferSize) {
            List<LogRecord> replay = cutoff == null ? new ArrayList<>(buf) : drainSince(cutoff);
            BlockingQueue<LogRecord> queue = new LinkedBlockingQueue<>(Math.max(1, bufferSize));
            Consumer<LogRecord> fn = event -> queue.offer(event);
            // Snapshot and subscription are in one critical section, so a
            // follow=true stream cannot miss emissions at the replay/live seam.
            subs.add(fn);
            return new ReplaySubscription<>(replay, queue, () -> subs.remove(fn));
        }

        public void close() {
            closed = true;
            subs.clear();
        }
    }

    public record ReplaySubscription<T>(List<T> replay, BlockingQueue<T> live, AutoCloseable closeable) implements AutoCloseable {
        @Override
        public void close() throws Exception {
            closeable.close();
        }
    }

    // --- Metrics ---

    public static final class Counter {
        public final String name, help;
        public final Map<String, String> labels;
        private final AtomicLong v = new AtomicLong();
        Counter(String n, String h, Map<String,String> l) { name=n; help=h; labels = Map.copyOf(l); }
        public void inc() { v.incrementAndGet(); }
        public void add(long n) { if (n >= 0) v.addAndGet(n); }
        public long value() { return v.get(); }
    }

    public static final class Gauge {
        public final String name, help;
        public final Map<String, String> labels;
        private final Object lock = new Object();
        private double v;
        Gauge(String n, String h, Map<String,String> l) { name=n; help=h; labels=Map.copyOf(l); }
        public void set(double x) { synchronized (lock) { v = x; } }
        public void add(double d) { synchronized (lock) { v += d; } }
        public double value() { synchronized (lock) { return v; } }
    }

    public static final class HistogramSnapshot {
        public final double[] bounds;
        public final long[] counts;
        public final long total;
        public final double sum;
        public HistogramSnapshot(double[] b, long[] c, long t, double s) {
            bounds = b; counts = c; total = t; sum = s;
        }
        public double quantile(double q) {
            if (total == 0) return Double.NaN;
            double target = total * q;
            for (int i = 0; i < counts.length; i++) if (counts[i] >= target) return bounds[i];
            return Double.POSITIVE_INFINITY;
        }
    }

    public static final double[] DEFAULT_BUCKETS = {
        50e-6, 100e-6, 250e-6, 500e-6,
        1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
        1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
    };

    public static final class Histogram {
        public final String name, help;
        public final Map<String, String> labels;
        private final double[] bounds;
        private final long[] counts;
        private long total;
        private double sum;

        Histogram(String n, String h, Map<String,String> l, double[] bnds) {
            name=n; help=h; labels=Map.copyOf(l);
            double[] b = bnds != null && bnds.length > 0 ? bnds.clone() : DEFAULT_BUCKETS.clone();
            Arrays.sort(b);
            this.bounds = b;
            this.counts = new long[b.length];
        }

        public synchronized void observe(double x) {
            total++;
            sum += x;
            for (int i = 0; i < bounds.length; i++) if (x <= bounds[i]) counts[i]++;
        }

        public synchronized HistogramSnapshot snapshot() {
            return new HistogramSnapshot(bounds.clone(), counts.clone(), total, sum);
        }
    }

    private static String metricKey(String name, Map<String, String> labels) {
        if (labels == null || labels.isEmpty()) return name;
        StringBuilder sb = new StringBuilder(name);
        labels.entrySet().stream()
                .sorted(Map.Entry.comparingByKey())
                .forEach(e -> sb.append('|').append(e.getKey()).append('=').append(e.getValue()));
        return sb.toString();
    }

    public static final class Registry {
        private final ConcurrentHashMap<String, Counter> counters = new ConcurrentHashMap<>();
        private final ConcurrentHashMap<String, Gauge> gauges = new ConcurrentHashMap<>();
        private final ConcurrentHashMap<String, Histogram> histograms = new ConcurrentHashMap<>();

        public Counter counter(String n, String h, Map<String,String> labels) {
            Map<String,String> l = labels == null ? Map.of() : labels;
            return counters.computeIfAbsent(metricKey(n, l), k -> new Counter(n, h, l));
        }
        public Gauge gauge(String n, String h, Map<String,String> labels) {
            Map<String,String> l = labels == null ? Map.of() : labels;
            return gauges.computeIfAbsent(metricKey(n, l), k -> new Gauge(n, h, l));
        }
        public Histogram histogram(String n, String h, Map<String,String> labels, double[] bounds) {
            Map<String,String> l = labels == null ? Map.of() : labels;
            return histograms.computeIfAbsent(metricKey(n, l), k -> new Histogram(n, h, l, bounds));
        }

        public List<Counter> counters() {
            List<Counter> l = new ArrayList<>(counters.values());
            l.sort(Comparator.comparing(c -> c.name));
            return l;
        }
        public List<Gauge> gauges() {
            List<Gauge> l = new ArrayList<>(gauges.values());
            l.sort(Comparator.comparing(g -> g.name));
            return l;
        }
        public List<Histogram> histograms() {
            List<Histogram> l = new ArrayList<>(histograms.values());
            l.sort(Comparator.comparing(h -> h.name));
            return l;
        }
    }

    public static holons.v1.Observability.AnyValue anyValue(Object value) {
        holons.v1.Observability.AnyValue.Builder builder = holons.v1.Observability.AnyValue.newBuilder();
        if (value == null) {
            return builder.setStringValue("").build();
        }
        if (value instanceof Boolean b) {
            return builder.setBoolValue(b).build();
        }
        if (value instanceof Byte || value instanceof Short || value instanceof Integer || value instanceof Long) {
            return builder.setIntValue(((Number) value).longValue()).build();
        }
        if (value instanceof Float || value instanceof Double) {
            return builder.setDoubleValue(((Number) value).doubleValue()).build();
        }
        if (value instanceof Number number) {
            return builder.setDoubleValue(number.doubleValue()).build();
        }
        if (value instanceof CharSequence text) {
            return builder.setStringValue(text.toString()).build();
        }
        return builder.setStringValue(String.valueOf(value)).build();
    }

    private static holons.v1.Observability.KeyValue keyValue(String key, Object value) {
        return holons.v1.Observability.KeyValue.newBuilder()
                .setKey(key == null ? "" : key)
                .setValue(anyValue(value))
                .build();
    }

    private static List<holons.v1.Observability.KeyValue> resourceAttributes(String slug, String uid, String sessionId) {
        List<holons.v1.Observability.KeyValue> attrs = new ArrayList<>();
        attrs.add(keyValue(ATTR_HOLONS_SLUG, slug == null ? "" : slug));
        attrs.add(keyValue(ATTR_SERVICE_NAME, slug == null ? "" : slug));
        attrs.add(keyValue(ATTR_HOLONS_INSTANCE_UID, uid == null ? "" : uid));
        attrs.add(keyValue(ATTR_SERVICE_INSTANCE_ID, uid == null ? "" : uid));
        attrs.add(keyValue(ATTR_HOLONS_SESSION_ID, sessionId == null ? "" : sessionId));
        return attrs;
    }

    private static List<holons.v1.Observability.KeyValue> sortedStringAttributes(Map<String, String> values) {
        if (values == null || values.isEmpty()) {
            return List.of();
        }
        return values.entrySet().stream()
                .sorted(Map.Entry.comparingByKey())
                .map(entry -> keyValue(entry.getKey(), entry.getValue()))
                .toList();
    }

    public static String stringAttribute(List<holons.v1.Observability.KeyValue> attrs, String key) {
        for (holons.v1.Observability.KeyValue attr : attrs) {
            if (attr != null && key.equals(attr.getKey())) {
                return anyValueString(attr.getValue());
            }
        }
        return "";
    }

    public static String anyValueString(holons.v1.Observability.AnyValue value) {
        if (value == null) {
            return "";
        }
        return switch (value.getValueCase()) {
            case STRING_VALUE -> value.getStringValue();
            case BOOL_VALUE -> Boolean.toString(value.getBoolValue());
            case INT_VALUE -> Long.toString(value.getIntValue());
            case DOUBLE_VALUE -> Double.toString(value.getDoubleValue());
            default -> "";
        };
    }

    private static List<holons.v1.Observability.KeyValue> metricAttributes(holons.v1.Observability.Metric metric) {
        return switch (metric.getDataCase()) {
            case SUM -> metric.getSum().getDataPointsCount() == 0
                    ? List.of()
                    : metric.getSum().getDataPoints(0).getAttributesList();
            case GAUGE -> metric.getGauge().getDataPointsCount() == 0
                    ? List.of()
                    : metric.getGauge().getDataPoints(0).getAttributesList();
            case HISTOGRAM -> metric.getHistogram().getDataPointsCount() == 0
                    ? List.of()
                    : metric.getHistogram().getDataPoints(0).getAttributesList();
            default -> List.of();
        };
    }

    // --- Config + Observability root ---

    public static final class Config {
        public String slug = "";
        public Level defaultLogLevel = Level.INFO;
        public String promAddr = "";
        public List<String> redactedFields = List.of();
        public int logsRingSize = 1024;
        public int eventsRingSize = 256;
        public String runDir = "";
        public String instanceUid = "", organismUid = "", organismSlug = "";
    }

    public static final class Logger {
        private final Observability obs;
        public final String name;
        private volatile Level level;
        Logger(Observability o, String n) {
            this.obs = o; this.name = n;
            this.level = o != null ? o.cfg.defaultLogLevel : Level.FATAL;
        }
        public void setLevel(Level l) { this.level = l; }
        public boolean enabled(Level l) { return obs != null && l.code >= level.code; }

        public void log(Level l, String msg, Map<String, Object> fields) {
            log(l, msg, fields, false);
        }

        public void log(Level l, String msg, Map<String, Object> fields, boolean privateEntry) {
            if (!enabled(l)) return;
            Set<String> redact = new HashSet<>(obs.cfg.redactedFields);
            List<holons.v1.Observability.KeyValue> attrs = resourceAttributes(obs.cfg.slug, obs.cfg.instanceUid, "");
            if (name != null && !name.isBlank()) {
                attrs.add(keyValue(ATTR_LOGGER_NAME, name));
            }
            if (fields != null) {
                for (Map.Entry<String, Object> e : fields.entrySet()) {
                    if (e.getKey() == null || e.getKey().isEmpty()) continue;
                    attrs.add(keyValue(e.getKey(), redact.contains(e.getKey()) ? "<redacted>" : e.getValue()));
                }
            }
            long now = System.currentTimeMillis() * 1_000_000L;
            LogRecord entry = new LogRecord(holons.v1.Observability.LogRecord.newBuilder()
                    .setTimeUnixNano(now)
                    .setObservedTimeUnixNano(now)
                    .setSeverityNumberValue(l.code)
                    .setSeverityText(l.label())
                    .setBody(anyValue(msg))
                    .addAllAttributes(attrs)
                    .build(), privateEntry);
            if (obs.logRing != null) obs.logRing.push(entry);
        }

        public void trace(String m, Map<String,Object> f) { log(Level.TRACE, m, f); }
        public void debug(String m, Map<String,Object> f) { log(Level.DEBUG, m, f); }
        public void info(String m, Map<String,Object> f)  { log(Level.INFO, m, f); }
        public void warn(String m, Map<String,Object> f)  { log(Level.WARN, m, f); }
        public void error(String m, Map<String,Object> f) { log(Level.ERROR, m, f); }
        public void fatal(String m, Map<String,Object> f) { log(Level.FATAL, m, f); }
        public void privateInfo(String m, Map<String,Object> f) { log(Level.INFO, m, f, true); }
    }

    public final Config cfg;
    public final Set<Family> families;
    public final LogRing logRing;
    public final EventBus eventBus;
    public final Registry registry;
    private final ConcurrentHashMap<String, Logger> loggers = new ConcurrentHashMap<>();

    private Observability(Config cfg, Set<Family> families) {
        this.cfg = cfg;
        this.families = Collections.unmodifiableSet(EnumSet.copyOf(families.isEmpty() ?
                EnumSet.noneOf(Family.class) : families));
        this.logRing = families.contains(Family.LOGS) ? new LogRing(cfg.logsRingSize) : null;
        this.eventBus = families.contains(Family.EVENTS) ? new EventBus(cfg.eventsRingSize) : null;
        this.registry = families.contains(Family.METRICS) ? new Registry() : null;
    }

    public boolean enabled(Family f) { return families.contains(f); }

    public boolean isOrganismRoot() {
        return cfg.organismUid != null && !cfg.organismUid.isEmpty()
                && cfg.organismUid.equals(cfg.instanceUid);
    }

    public Logger logger(String name) {
        if (!families.contains(Family.LOGS)) return DISABLED_LOGGER;
        return loggers.computeIfAbsent(name, n -> new Logger(this, n));
    }

    public Counter counter(String n, String h, Map<String,String> l) {
        return registry == null ? null : registry.counter(n, h, l);
    }
    public Gauge gauge(String n, String h, Map<String,String> l) {
        return registry == null ? null : registry.gauge(n, h, l);
    }
    public Histogram histogram(String n, String h, Map<String,String> l, double[] b) {
        return registry == null ? null : registry.histogram(n, h, l, b);
    }

    public void emit(String eventName, Map<String, String> payload) {
        emit(eventName, payload, false);
    }

    public void emit(String eventName, Map<String, String> payload, boolean privateEntry) {
        if (eventBus == null) return;
        Set<String> redact = new HashSet<>(cfg.redactedFields);
        Map<String, String> p = new LinkedHashMap<>();
        if (payload != null) {
            for (Map.Entry<String, String> e : payload.entrySet()) {
                p.put(e.getKey(), redact.contains(e.getKey()) ? "<redacted>" : e.getValue());
            }
        }
        List<holons.v1.Observability.KeyValue> attrs = resourceAttributes(cfg.slug, cfg.instanceUid, "");
        attrs.addAll(sortedStringAttributes(p));
        long now = System.currentTimeMillis() * 1_000_000L;
        eventBus.emit(new LogRecord(holons.v1.Observability.LogRecord.newBuilder()
                .setTimeUnixNano(now)
                .setObservedTimeUnixNano(now)
                .setSeverityNumber(holons.v1.Observability.SeverityNumber.SEVERITY_NUMBER_INFO)
                .setSeverityText("INFO")
                .setBody(anyValue(eventName))
                .setEventName(eventName == null ? "" : eventName)
                .addAllAttributes(attrs)
                .build(), privateEntry));
    }

    public void emitPrivate(String eventName, Map<String, String> payload) {
        emit(eventName, payload, true);
    }

    public void close() {
        if (eventBus != null) eventBus.close();
    }

    // --- Package-scope singleton ---

    private static final AtomicReference<Observability> CURRENT = new AtomicReference<>();

    private static final Observability DISABLED =
            new Observability(new Config(), EnumSet.noneOf(Family.class));
    private static final Logger DISABLED_LOGGER = new Logger(DISABLED, "");

    public static Observability configure(Config cfg) {
        return configureFromEnv(cfg, System.getenv());
    }

    public static Observability configureFromEnv(Config cfg, Map<String, String> env) {
        if (cfg == null) cfg = new Config();
        Map<String, String> source = env != null ? env : System.getenv();
        Set<Family> families = parseOpObs(source.get("OP_OBS"));
        if (cfg.slug == null) cfg.slug = "";
        if (cfg.instanceUid == null || cfg.instanceUid.isEmpty()) {
            cfg.instanceUid = newInstanceUid();
        }
        if (cfg.runDir != null && !cfg.runDir.isEmpty()) {
            cfg.runDir = deriveRunDir(cfg.runDir, cfg.slug, cfg.instanceUid);
        }
        Observability obs = new Observability(cfg, families);
        CURRENT.set(obs);
        return obs;
    }

    public static Observability fromEnv(Config base) {
        return fromEnvMap(base, System.getenv());
    }

    public static Observability fromEnvMap(Config base, Map<String, String> env) {
        if (base == null) base = new Config();
        Map<String, String> source = env != null ? env : System.getenv();
        if (base.instanceUid.isEmpty()) base.instanceUid = source.getOrDefault("OP_INSTANCE_UID", "");
        if (base.organismUid.isEmpty()) base.organismUid = source.getOrDefault("OP_ORGANISM_UID", "");
        if (base.organismSlug.isEmpty()) base.organismSlug = source.getOrDefault("OP_ORGANISM_SLUG", "");
        if (base.promAddr.isEmpty()) base.promAddr = source.getOrDefault("OP_PROM_ADDR", "");
        if (base.runDir.isEmpty()) base.runDir = source.getOrDefault("OP_RUN_DIR", "");
        return configureFromEnv(base, source);
    }

    public static Observability current() {
        Observability o = CURRENT.get();
        return o != null ? o : DISABLED;
    }

    public static void reset() {
        Observability o = CURRENT.getAndSet(null);
        if (o != null) o.close();
    }

    public static String deriveRunDir(String root, String slug, String uid) {
        if (root == null || root.isEmpty() || slug == null || slug.isEmpty() || uid == null || uid.isEmpty()) {
            return root == null ? "" : root;
        }
        return Paths.get(root, slug, uid).toString();
    }

    private static String newInstanceUid() {
        return ProcessHandle.current().pid() + "-" + System.nanoTime();
    }

    // --- gRPC service ---

    public static MethodDescriptor<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogRecord> logsMethod() {
        return LOGS_METHOD;
    }

    public static MethodDescriptor<holons.v1.Observability.MetricsRequest, holons.v1.Observability.Metric> metricsMethod() {
        return METRICS_METHOD;
    }

    public static MethodDescriptor<holons.v1.Observability.EventsRequest, holons.v1.Observability.LogRecord> eventsMethod() {
        return EVENTS_METHOD;
    }

    public static ServerServiceDefinition service() {
        return service(current());
    }

    public static ServerServiceDefinition service(Observability obs) {
        Objects.requireNonNull(obs, "obs");
        return ServerServiceDefinition.builder(HOLON_OBSERVABILITY_SERVICE)
                .addMethod(LOGS_METHOD, ServerCalls.asyncServerStreamingCall((request, observer) -> {
                    if (!obs.enabled(Family.LOGS) || obs.logRing == null) {
                        observer.onError(Status.FAILED_PRECONDITION
                                .withDescription("logs family is not enabled (OP_OBS)")
                                .asRuntimeException());
                        return;
                    }
                    int minLevel = request.getMinSeverityNumberValue() == 0 ? Level.INFO.code : request.getMinSeverityNumberValue();
                    ReplaySubscription<LogRecord> subscription = null;
                    List<LogRecord> entries;
                    if (request.getFollow()) {
                        subscription = obs.logRing.replayAndSubscribe(
                                request.hasSince() ? cutoffFromDuration(request.getSince()) : null,
                                128);
                        entries = subscription.replay();
                    } else {
                        entries = request.hasSince()
                                ? obs.logRing.drainSince(cutoffFromDuration(request.getSince()))
                                : obs.logRing.drain();
                    }
                    for (LogRecord entry : entries) {
                        if (request.getFollow() && entry.privateEntry) {
                            continue;
                        }
                        if (matchesLog(entry, minLevel, request.getSessionIdsList(), request.getRpcMethodsList())) {
                            observer.onNext(toProtoLogRecord(entry));
                        }
                    }
                    if (!request.getFollow()) {
                        observer.onCompleted();
                        return;
                    }
                    ReplaySubscription<LogRecord> liveSubscription = subscription;
                    Thread liveThread = new Thread(() -> {
                        try {
                            while (!Thread.currentThread().isInterrupted()) {
                                LogRecord entry = liveSubscription.live().take();
                                if (entry.privateEntry
                                        || !matchesLog(entry, minLevel, request.getSessionIdsList(), request.getRpcMethodsList())) {
                                    continue;
                                }
                                observer.onNext(toProtoLogRecord(entry));
                            }
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        } catch (Exception ignored) {
                            // Cancelled clients are handled by the gRPC runtime.
                        } finally {
                            closeQuietly(liveSubscription);
                        }
                    }, "holons-observability-logs-follow");
                    liveThread.setDaemon(true);
                    liveThread.start();
                    if (observer instanceof ServerCallStreamObserver<?> serverObserver) {
                        serverObserver.setOnCancelHandler(() -> {
                            closeQuietly(liveSubscription);
                            liveThread.interrupt();
                        });
                    }
                }))
                .addMethod(METRICS_METHOD, ServerCalls.asyncServerStreamingCall((request, observer) -> {
                    if (!obs.enabled(Family.METRICS) || obs.registry == null) {
                        observer.onError(Status.FAILED_PRECONDITION
                                .withDescription("metrics family is not enabled (OP_OBS)")
                                .asRuntimeException());
                        return;
                    }
                    for (holons.v1.Observability.Metric metric : toProtoMetrics(obs.registry, obs.cfg.slug, obs.cfg.instanceUid)) {
                        if (request.getNamePrefixesCount() == 0
                                || request.getNamePrefixesList().stream().anyMatch(prefix -> metric.getName().startsWith(prefix))) {
                            observer.onNext(metric);
                        }
                    }
                    observer.onCompleted();
                }))
                .addMethod(EVENTS_METHOD, ServerCalls.asyncServerStreamingCall((request, observer) -> {
                    if (!obs.enabled(Family.EVENTS) || obs.eventBus == null) {
                        observer.onError(Status.FAILED_PRECONDITION
                                .withDescription("events family is not enabled (OP_OBS)")
                                .asRuntimeException());
                        return;
                    }
                    Set<String> wanted = new HashSet<>(request.getEventNamesList());
                    ReplaySubscription<LogRecord> subscription = null;
                    List<LogRecord> events;
                    if (request.getFollow()) {
                        subscription = obs.eventBus.replayAndSubscribe(
                                request.hasSince() ? cutoffFromDuration(request.getSince()) : null,
                                64);
                        events = subscription.replay();
                    } else {
                        events = request.hasSince()
                                ? obs.eventBus.drainSince(cutoffFromDuration(request.getSince()))
                                : obs.eventBus.drain();
                    }
                    for (LogRecord event : events) {
                        if (request.getFollow() && event.privateEntry) {
                            continue;
                        }
                        if (wanted.isEmpty() || wanted.contains(event.record.getEventName())) {
                            observer.onNext(toProtoLogRecord(event));
                        }
                    }
                    if (!request.getFollow()) {
                        observer.onCompleted();
                        return;
                    }
                    ReplaySubscription<LogRecord> liveSubscription = subscription;
                    Thread liveThread = new Thread(() -> {
                        try {
                            while (!Thread.currentThread().isInterrupted()) {
                                LogRecord event = liveSubscription.live().take();
                                if (event.privateEntry || (!wanted.isEmpty() && !wanted.contains(event.record.getEventName()))) {
                                    continue;
                                }
                                observer.onNext(toProtoLogRecord(event));
                            }
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        } catch (Exception ignored) {
                            // Cancelled clients are handled by the gRPC runtime.
                        } finally {
                            closeQuietly(liveSubscription);
                        }
                    }, "holons-observability-events-follow");
                    liveThread.setDaemon(true);
                    liveThread.start();
                    if (observer instanceof ServerCallStreamObserver<?> serverObserver) {
                        serverObserver.setOnCancelHandler(() -> {
                            closeQuietly(liveSubscription);
                            liveThread.interrupt();
                        });
                    }
                }))
                .build();
    }

    private static Instant cutoffFromDuration(Duration duration) {
        long seconds = Math.max(0, duration.getSeconds());
        int nanos = Math.max(0, duration.getNanos());
        return Instant.now().minusSeconds(seconds).minusNanos(nanos);
    }

    private static boolean matchesLog(LogRecord entry, int minLevel, List<String> sessionIds, List<String> rpcMethods) {
        if (entry.record.getSeverityNumberValue() < minLevel) return false;
        String sessionId = entry.attr(ATTR_HOLONS_SESSION_ID);
        String rpcMethod = entry.attr(ATTR_RPC_METHOD);
        if (!sessionIds.isEmpty() && !sessionIds.contains(sessionId)) return false;
        return rpcMethods.isEmpty() || rpcMethods.contains(rpcMethod);
    }

    public static holons.v1.Observability.LogRecord toProtoLogRecord(LogRecord entry) {
        return entry == null || entry.record == null
                ? holons.v1.Observability.LogRecord.getDefaultInstance()
                : entry.record.toBuilder().build();
    }

    public static LogRecord fromProtoLogRecord(holons.v1.Observability.LogRecord record) {
        return new LogRecord(record);
    }

    public static List<holons.v1.Observability.Metric> toProtoMetrics(Registry registry, String slug, String uid) {
        List<holons.v1.Observability.Metric> metrics = new ArrayList<>();
        long now = System.currentTimeMillis() * 1_000_000L;
        long start = now;
        for (Counter counter : registry.counters()) {
            metrics.add(holons.v1.Observability.Metric.newBuilder()
                    .setName(counter.name)
                    .setDescription(counter.help)
                    .setSum(holons.v1.Observability.Sum.newBuilder()
                            .setAggregationTemporality(holons.v1.Observability.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE)
                            .setIsMonotonic(true)
                            .addDataPoints(numberDataPoint(start, now, counter.value(), resourceAndLabels(slug, uid, counter.labels)))
                            .build())
                    .build());
        }
        for (Gauge gauge : registry.gauges()) {
            metrics.add(holons.v1.Observability.Metric.newBuilder()
                    .setName(gauge.name)
                    .setDescription(gauge.help)
                    .setGauge(holons.v1.Observability.Gauge.newBuilder()
                            .addDataPoints(numberDataPoint(start, now, gauge.value(), resourceAndLabels(slug, uid, gauge.labels)))
                            .build())
                    .build());
        }
        for (Histogram histogram : registry.histograms()) {
            HistogramSnapshot snapshot = histogram.snapshot();
            metrics.add(holons.v1.Observability.Metric.newBuilder()
                    .setName(histogram.name)
                    .setDescription(histogram.help)
                    .setHistogram(holons.v1.Observability.Histogram.newBuilder()
                            .setAggregationTemporality(holons.v1.Observability.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE)
                            .addDataPoints(histogramDataPoint(start, now, snapshot, resourceAndLabels(slug, uid, histogram.labels)))
                            .build())
                    .build());
        }
        return metrics;
    }

    private static List<holons.v1.Observability.KeyValue> resourceAndLabels(String slug, String uid, Map<String, String> labels) {
        List<holons.v1.Observability.KeyValue> attrs = resourceAttributes(slug, uid, "");
        attrs.addAll(sortedStringAttributes(labels));
        return attrs;
    }

    private static holons.v1.Observability.NumberDataPoint numberDataPoint(
            long start,
            long now,
            long value,
            List<holons.v1.Observability.KeyValue> attrs) {
        return holons.v1.Observability.NumberDataPoint.newBuilder()
                .setStartTimeUnixNano(start)
                .setTimeUnixNano(now)
                .setAsInt(value)
                .addAllAttributes(attrs)
                .build();
    }

    private static holons.v1.Observability.NumberDataPoint numberDataPoint(
            long start,
            long now,
            double value,
            List<holons.v1.Observability.KeyValue> attrs) {
        return holons.v1.Observability.NumberDataPoint.newBuilder()
                .setStartTimeUnixNano(start)
                .setTimeUnixNano(now)
                .setAsDouble(value)
                .addAllAttributes(attrs)
                .build();
    }

    private static holons.v1.Observability.HistogramDataPoint histogramDataPoint(
            long start,
            long now,
            HistogramSnapshot snapshot,
            List<holons.v1.Observability.KeyValue> attrs) {
        holons.v1.Observability.HistogramDataPoint.Builder builder =
                holons.v1.Observability.HistogramDataPoint.newBuilder()
                        .setStartTimeUnixNano(start)
                        .setTimeUnixNano(now)
                        .setCount(snapshot.total)
                        .setSum(snapshot.sum)
                        .addAllAttributes(attrs);
        long previous = 0;
        for (long cumulative : snapshot.counts) {
            long delta = Math.max(0, cumulative - previous);
            builder.addBucketCounts(delta);
            previous = cumulative;
        }
        builder.addBucketCounts(Math.max(0, snapshot.total - previous));
        for (double bound : snapshot.bounds) {
            builder.addExplicitBounds(bound);
        }
        if (snapshot.total > 0) {
            builder.setMin(0).setMax(snapshot.bounds.length == 0 ? 0 : snapshot.bounds[snapshot.bounds.length - 1]);
        }
        return builder.build();
    }

    private static Level levelFromCode(int code) {
        for (Level level : Level.values()) {
            if (level.code == code) {
                return level;
            }
        }
        return Level.UNSET;
    }

    // --- Prometheus exposition ---

    public static final class PromServer implements AutoCloseable {
        private final String addr;
        private HttpServer server;

        public PromServer(String addr) {
            this.addr = addr == null || addr.isBlank() ? ":0" : addr.trim();
        }

        public synchronized void start() throws IOException {
            if (server != null) {
                return;
            }
            HostPort hostPort = parsePromAddr(addr);
            server = HttpServer.create(new InetSocketAddress(hostPort.host(), hostPort.port()), 0);
            server.createContext("/metrics", this::handleMetrics);
            server.start();
        }

        public synchronized String addrUrl() {
            if (server == null) {
                return "";
            }
            InetSocketAddress address = server.getAddress();
            return "http://" + advertisedPromHost(address.getHostString()) + ":" + address.getPort() + "/metrics";
        }

        @Override
        public synchronized void close() {
            if (server == null) {
                return;
            }
            server.stop(0);
            server = null;
        }

        private void handleMetrics(HttpExchange exchange) throws IOException {
            String path = exchange.getRequestURI() != null ? exchange.getRequestURI().getPath() : "";
            if (!"/metrics".equals(path)) {
                exchange.sendResponseHeaders(404, -1);
                exchange.close();
                return;
            }

            Observability obs = current();
            int status = 200;
            String body;
            if (!obs.enabled(Family.METRICS)) {
                status = 503;
                body = "# metrics family disabled (OP_OBS)\n";
            } else if (!obs.enabled(Family.PROM)) {
                status = 503;
                body = "# prom family disabled (OP_OBS)\n";
            } else {
                body = toPrometheusText(obs);
            }
            byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "text/plain; version=0.0.4");
            exchange.sendResponseHeaders(status, bytes.length);
            try (OutputStream out = exchange.getResponseBody()) {
                out.write(bytes);
            }
        }
    }

    public static String toPrometheusText(Observability obs) {
        if (obs == null || !obs.enabled(Family.METRICS) || obs.registry == null) {
            return "# metrics family disabled (OP_OBS)\n";
        }

        StringBuilder out = new StringBuilder();
        List<Counter> counters = obs.registry.counters();
        List<Gauge> gauges = obs.registry.gauges();
        List<Histogram> histograms = obs.registry.histograms();

        for (Counter counter : counters) {
            appendPromHelpType(out, counter.name, counter.help, "counter");
            out.append(counter.name)
                    .append(promLabels(mergePromLabels(counter.labels, obs)))
                    .append(' ')
                    .append(counter.value())
                    .append('\n');
        }
        for (Gauge gauge : gauges) {
            appendPromHelpType(out, gauge.name, gauge.help, "gauge");
            out.append(gauge.name)
                    .append(promLabels(mergePromLabels(gauge.labels, obs)))
                    .append(' ')
                    .append(formatPromFloat(gauge.value()))
                    .append('\n');
        }
        for (Histogram histogram : histograms) {
            appendPromHelpType(out, histogram.name, histogram.help, "histogram");
            Map<String, String> labels = mergePromLabels(histogram.labels, obs);
            HistogramSnapshot snapshot = histogram.snapshot();
            for (int i = 0; i < snapshot.bounds.length; i++) {
                Map<String, String> bucketLabels = new LinkedHashMap<>(labels);
                bucketLabels.put("le", formatPromFloat(snapshot.bounds[i]));
                out.append(histogram.name)
                        .append("_bucket")
                        .append(promLabels(bucketLabels))
                        .append(' ')
                        .append(snapshot.counts[i])
                        .append('\n');
            }
            Map<String, String> infLabels = new LinkedHashMap<>(labels);
            infLabels.put("le", "+Inf");
            out.append(histogram.name).append("_bucket").append(promLabels(infLabels)).append(' ')
                    .append(snapshot.total).append('\n');
            out.append(histogram.name).append("_sum").append(promLabels(labels)).append(' ')
                    .append(formatPromFloat(snapshot.sum)).append('\n');
            out.append(histogram.name).append("_count").append(promLabels(labels)).append(' ')
                    .append(snapshot.total).append('\n');
        }

        return out.toString();
    }

    private static void appendPromHelpType(StringBuilder out, String name, String help, String type) {
        out.append("# HELP ").append(name).append(' ').append(promEscapeHelp(help)).append('\n');
        out.append("# TYPE ").append(name).append(' ').append(type).append('\n');
    }

    private static Map<String, String> mergePromLabels(Map<String, String> labels, Observability obs) {
        Map<String, String> out = new LinkedHashMap<>();
        if (obs.cfg.slug != null && !obs.cfg.slug.isEmpty()) {
            out.put("slug", obs.cfg.slug);
        }
        if (obs.cfg.instanceUid != null && !obs.cfg.instanceUid.isEmpty()) {
            out.put("instance_uid", obs.cfg.instanceUid);
        }
        if (labels != null) {
            out.putAll(labels);
        }
        return out;
    }

    private static String promLabels(Map<String, String> labels) {
        if (labels == null || labels.isEmpty()) {
            return "";
        }
        StringBuilder out = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> entry : labels.entrySet().stream()
                .sorted(Map.Entry.comparingByKey())
                .toList()) {
            if (!first) {
                out.append(',');
            }
            first = false;
            out.append(entry.getKey()).append("=\"").append(promEscapeValue(entry.getValue())).append('"');
        }
        out.append('}');
        return out.toString();
    }

    private static String promEscapeValue(String value) {
        return String.valueOf(value).replace("\\", "\\\\").replace("\n", "\\n").replace("\"", "\\\"");
    }

    private static String promEscapeHelp(String value) {
        return String.valueOf(value == null ? "" : value).replace("\\", "\\\\").replace("\n", "\\n");
    }

    private static String formatPromFloat(double value) {
        if (Double.isNaN(value)) {
            return "NaN";
        }
        if (value == Double.POSITIVE_INFINITY) {
            return "+Inf";
        }
        if (value == Double.NEGATIVE_INFINITY) {
            return "-Inf";
        }
        return Double.toString(value);
    }

    private record HostPort(String host, int port) {
    }

    private static HostPort parsePromAddr(String raw) {
        String trimmed = raw == null || raw.isBlank() ? ":0" : raw.trim();
        if (trimmed.startsWith(":")) {
            return new HostPort("0.0.0.0", Integer.parseInt(trimmed.substring(1).isEmpty() ? "0" : trimmed.substring(1)));
        }
        int idx = trimmed.lastIndexOf(':');
        if (idx < 0) {
            throw new IllegalArgumentException("invalid Prometheus address \"" + raw + "\"");
        }
        String host = trimmed.substring(0, idx);
        int port = Integer.parseInt(trimmed.substring(idx + 1));
        return new HostPort(host.isBlank() ? "0.0.0.0" : host, port);
    }

    private static String advertisedPromHost(String host) {
        if (host == null || host.isBlank() || "0.0.0.0".equals(host)) {
            return "127.0.0.1";
        }
        if ("::".equals(host)) {
            return "::1";
        }
        return host;
    }

    // --- Member observability relay ---

    public record MemberIdentity(String slug, String uid) {
        public MemberIdentity {
            slug = slug == null ? "" : slug.trim();
            uid = uid == null ? "" : uid.trim();
        }
    }

    public static MemberIdentity resolveMemberIdentity(ManagedChannel channel, String fallbackSlug, String fallbackUid) {
        String fallbackSlugValue = fallbackSlug == null ? "" : fallbackSlug.trim();
        String fallbackUidValue = fallbackUid == null ? "" : fallbackUid.trim();
        if (!fallbackUidValue.isEmpty()) {
            return new MemberIdentity(fallbackSlugValue, fallbackUidValue);
        }

        try {
            Iterator<holons.v1.Observability.LogRecord> iterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    EVENTS_METHOD,
                    CallOptions.DEFAULT,
                    holons.v1.Observability.EventsRequest.newBuilder()
                            .addEventNames(EVENT_INSTANCE_READY)
                            .build());
            while (iterator.hasNext()) {
                holons.v1.Observability.LogRecord event = iterator.next();
                String uid = stringAttribute(event.getAttributesList(), ATTR_HOLONS_INSTANCE_UID);
                if (event.getChainCount() == 0 && !uid.isBlank()) {
                    String slug = stringAttribute(event.getAttributesList(), ATTR_HOLONS_SLUG);
                    return new MemberIdentity(slug.isBlank() ? fallbackSlugValue : slug, uid);
                }
            }
        } catch (Exception ignored) {
            // Fall back to metric resource attributes.
        }

        try {
            Iterator<holons.v1.Observability.Metric> metrics = ClientCalls.blockingServerStreamingCall(
                    channel,
                    METRICS_METHOD,
                    CallOptions.DEFAULT,
                    holons.v1.Observability.MetricsRequest.getDefaultInstance());
            if (metrics.hasNext()) {
                List<holons.v1.Observability.KeyValue> attrs = metricAttributes(metrics.next());
                String uid = stringAttribute(attrs, ATTR_HOLONS_INSTANCE_UID);
                if (!uid.isBlank()) {
                    String slug = stringAttribute(attrs, ATTR_HOLONS_SLUG);
                    return new MemberIdentity(slug.isBlank() ? fallbackSlugValue : slug, uid);
                }
            }
        } catch (Exception ignored) {
            // Leave unresolved.
        }

        return new MemberIdentity(fallbackSlugValue, fallbackUidValue);
    }

    public static final class MemberRelay implements AutoCloseable {
        private final String childSlug;
        private final String childUid;
        private final ManagedChannel channel;
        private final Observability observability;
        private final long retryDelayMillis;
        private final List<Thread> threads = new CopyOnWriteArrayList<>();
        private volatile boolean stopped;

        public MemberRelay(String childSlug, String childUid, ManagedChannel channel, Observability observability) {
            this(childSlug, childUid, channel, observability, 2000);
        }

        public MemberRelay(
                String childSlug,
                String childUid,
                ManagedChannel channel,
                Observability observability,
                long retryDelayMillis) {
            this.childSlug = childSlug == null ? "" : childSlug.trim();
            this.childUid = childUid == null ? "" : childUid.trim();
            this.channel = Objects.requireNonNull(channel, "channel");
            this.observability = Objects.requireNonNull(observability, "observability");
            this.retryDelayMillis = Math.max(1, retryDelayMillis);
        }

        public void start() {
            if (observability.enabled(Family.LOGS) && observability.logRing != null) {
                startThread("logs", this::pumpLogs);
            }
            if (observability.enabled(Family.EVENTS) && observability.eventBus != null) {
                startThread("events", this::pumpEvents);
            }
        }

        @Override
        public void close() {
            stopped = true;
            Connect.disconnect(channel);
            for (Thread thread : threads) {
                thread.interrupt();
            }
        }

        private void startThread(String family, Runnable task) {
            Thread thread = new Thread(task, "holons-member-relay-" + family + "-" + childSlug);
            thread.setDaemon(true);
            threads.add(thread);
            thread.start();
        }

        private void pumpLogs() {
            while (!stopped) {
                try {
                    Iterator<holons.v1.Observability.LogRecord> iterator = ClientCalls.blockingServerStreamingCall(
                            channel,
                            LOGS_METHOD,
                            CallOptions.DEFAULT,
                            holons.v1.Observability.LogsRequest.newBuilder()
                                    .setFollow(true)
                                    .setMinSeverityNumberValue(Level.INFO.code)
                                    .build());
                    while (!stopped && iterator.hasNext()) {
                        LogRecord entry = fromProtoLogRecord(iterator.next());
                        LogRecord enriched = new LogRecord(entry.record.toBuilder()
                                .clearChain()
                                .addAllChain(appendDirectChild(entry.record.getChainList(), childSlug, childUid))
                                .build());
                        observability.logRing.push(enriched);
                    }
                } catch (Exception ignored) {
                    retryPause();
                }
            }
        }

        private void pumpEvents() {
            while (!stopped) {
                try {
                    Iterator<holons.v1.Observability.LogRecord> iterator = ClientCalls.blockingServerStreamingCall(
                            channel,
                            EVENTS_METHOD,
                            CallOptions.DEFAULT,
                            holons.v1.Observability.EventsRequest.newBuilder()
                                    .setFollow(true)
                                    .build());
                    while (!stopped && iterator.hasNext()) {
                        LogRecord event = fromProtoLogRecord(iterator.next());
                        LogRecord enriched = new LogRecord(event.record.toBuilder()
                                .clearChain()
                                .addAllChain(appendDirectChild(event.record.getChainList(), childSlug, childUid))
                                .build());
                        observability.eventBus.emit(enriched);
                    }
                } catch (Exception ignored) {
                    retryPause();
                }
            }
        }

        private void retryPause() {
            if (stopped) {
                return;
            }
            try {
                TimeUnit.MILLISECONDS.sleep(retryDelayMillis);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }
    }

    private static void closeQuietly(AutoCloseable closeable) {
        if (closeable == null) {
            return;
        }
        try {
            closeable.close();
        } catch (Exception ignored) {
            // Best-effort subscription cleanup.
        }
    }

    // --- Disk writers ---

    public static void enableDiskWriters(String runDir) {
        Observability obs = current();
        if (obs == null || runDir == null || runDir.isEmpty()) return;
        try {
            Files.createDirectories(Paths.get(runDir));
        } catch (IOException ignored) {}

        if (obs.enabled(Family.LOGS) && obs.logRing != null) {
            Path fp = Paths.get(runDir, "stdout.log");
            obs.logRing.subscribe(e -> appendLogJsonl(fp, e));
        }
        if (obs.enabled(Family.EVENTS) && obs.eventBus != null) {
            Path fp = Paths.get(runDir, "events.jsonl");
            obs.eventBus.subscribe(e -> appendEventJsonl(fp, e));
        }
    }

    private static void appendLogJsonl(Path fp, LogRecord e) {
        holons.v1.Observability.LogRecord record = e.record;
        StringBuilder sb = new StringBuilder();
        sb.append("{\"kind\":\"log\"")
          .append(",\"ts\":\"").append(e.timestamp()).append("\"")
          .append(",\"level\":\"").append(record.getSeverityText()).append("\"")
          .append(",\"slug\":").append(quote(e.attr(ATTR_HOLONS_SLUG)))
          .append(",\"instance_uid\":").append(quote(e.attr(ATTR_HOLONS_INSTANCE_UID)))
          .append(",\"message\":").append(quote(e.bodyString()));
        String sessionId = e.attr(ATTR_HOLONS_SESSION_ID);
        String rpcMethod = e.attr(ATTR_RPC_METHOD);
        if (!sessionId.isEmpty()) sb.append(",\"session_id\":").append(quote(sessionId));
        if (!rpcMethod.isEmpty()) sb.append(",\"rpc_method\":").append(quote(rpcMethod));
        Map<String, String> fields = userAttributes(record.getAttributesList());
        if (!fields.isEmpty()) { sb.append(",\"fields\":"); jsonMap(sb, fields); }
        if (record.getChainCount() > 0) { sb.append(",\"chain\":"); jsonStringArray(sb, record.getChainList()); }
        sb.append("}\n");
        append(fp, sb.toString());
    }

    private static void appendEventJsonl(Path fp, LogRecord e) {
        holons.v1.Observability.LogRecord record = e.record;
        StringBuilder sb = new StringBuilder();
        sb.append("{\"kind\":\"event\"")
          .append(",\"ts\":\"").append(e.timestamp()).append("\"")
          .append(",\"event_name\":").append(quote(record.getEventName()))
          .append(",\"slug\":").append(quote(e.attr(ATTR_HOLONS_SLUG)))
          .append(",\"instance_uid\":").append(quote(e.attr(ATTR_HOLONS_INSTANCE_UID)));
        String sessionId = e.attr(ATTR_HOLONS_SESSION_ID);
        if (!sessionId.isEmpty()) sb.append(",\"session_id\":").append(quote(sessionId));
        Map<String, String> payload = userAttributes(record.getAttributesList());
        if (!payload.isEmpty()) { sb.append(",\"payload\":"); jsonMap(sb, payload); }
        if (record.getChainCount() > 0) { sb.append(",\"chain\":"); jsonStringArray(sb, record.getChainList()); }
        sb.append("}\n");
        append(fp, sb.toString());
    }

    private static Map<String, String> userAttributes(List<holons.v1.Observability.KeyValue> attrs) {
        Map<String, String> out = new LinkedHashMap<>();
        for (holons.v1.Observability.KeyValue attr : attrs) {
            String key = attr.getKey();
            if (Set.of(ATTR_HOLONS_SLUG, ATTR_SERVICE_NAME, ATTR_HOLONS_INSTANCE_UID,
                    ATTR_SERVICE_INSTANCE_ID, ATTR_HOLONS_SESSION_ID, ATTR_RPC_METHOD,
                    ATTR_LOGGER_NAME).contains(key)) {
                continue;
            }
            out.put(key, anyValueString(attr.getValue()));
        }
        return out;
    }

    private static void append(Path p, String s) {
        try (BufferedWriter w = Files.newBufferedWriter(p, StandardCharsets.UTF_8,
                StandardOpenOption.CREATE, StandardOpenOption.APPEND)) {
            w.write(s);
        } catch (IOException ignored) {}
    }

    private static String quote(String s) {
        if (s == null) return "\"\"";
        StringBuilder sb = new StringBuilder("\"");
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '\\': sb.append("\\\\"); break;
                case '"': sb.append("\\\""); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default:
                    if (c < 0x20) sb.append(String.format("\\u%04x", (int) c));
                    else sb.append(c);
            }
        }
        sb.append("\"");
        return sb.toString();
    }

    private static void jsonMap(StringBuilder sb, Map<String, String> m) {
        sb.append("{");
        boolean first = true;
        for (Map.Entry<String, String> e : m.entrySet()) {
            if (!first) sb.append(",");
            first = false;
            sb.append(quote(e.getKey())).append(":").append(quote(e.getValue()));
        }
        sb.append("}");
    }

    private static void jsonStringArray(StringBuilder sb, List<String> values) {
        sb.append("[");
        boolean first = true;
        for (String value : values) {
            if (!first) sb.append(",");
            first = false;
            sb.append(quote(value));
        }
        sb.append("]");
    }

    // --- meta.json ---

    public static final class MetaJson {
        public String slug = "", uid = "", mode = "persistent", transport = "", address = "";
        public String metricsAddr = "", logPath = "", organismUid = "", organismSlug = "";
        public long pid, logBytesRotated;
        public Instant startedAt = Instant.now();
        public boolean isDefault;
    }

    public static void writeMetaJson(String runDir, MetaJson m) throws IOException {
        Path dir = Paths.get(runDir);
        Files.createDirectories(dir);
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"slug\":").append(quote(m.slug)).append(",");
        sb.append("\"uid\":").append(quote(m.uid)).append(",");
        sb.append("\"pid\":").append(m.pid).append(",");
        sb.append("\"started_at\":").append(quote(m.startedAt.toString())).append(",");
        sb.append("\"mode\":").append(quote(m.mode)).append(",");
        sb.append("\"transport\":").append(quote(m.transport)).append(",");
        sb.append("\"address\":").append(quote(m.address));
        if (!m.metricsAddr.isEmpty()) sb.append(",\"metrics_addr\":").append(quote(m.metricsAddr));
        if (!m.logPath.isEmpty()) sb.append(",\"log_path\":").append(quote(m.logPath));
        if (m.logBytesRotated > 0) sb.append(",\"log_bytes_rotated\":").append(m.logBytesRotated);
        if (!m.organismUid.isEmpty()) sb.append(",\"organism_uid\":").append(quote(m.organismUid));
        if (!m.organismSlug.isEmpty()) sb.append(",\"organism_slug\":").append(quote(m.organismSlug));
        if (m.isDefault) sb.append(",\"default\":true");
        sb.append("}");
        Path tmp = dir.resolve("meta.json.tmp");
        Files.writeString(tmp, sb.toString());
        Files.move(tmp, dir.resolve("meta.json"), StandardCopyOption.REPLACE_EXISTING);
    }

    private Observability() { this(new Config(), EnumSet.noneOf(Family.class)); }
}

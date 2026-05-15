// Java reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability. See OBSERVABILITY.md.

package org.organicprogramming.holons;

import com.google.protobuf.Duration;
import com.google.protobuf.Timestamp;
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

    private static final MethodDescriptor<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogEntry> LOGS_METHOD =
            MethodDescriptor.<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogEntry>newBuilder()
                    .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Logs"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.LogEntry.getDefaultInstance()))
                    .build();

    private static final MethodDescriptor<holons.v1.Observability.MetricsRequest, holons.v1.Observability.MetricsSnapshot> METRICS_METHOD =
            MethodDescriptor.<holons.v1.Observability.MetricsRequest, holons.v1.Observability.MetricsSnapshot>newBuilder()
                    .setType(MethodDescriptor.MethodType.UNARY)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Metrics"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.MetricsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.MetricsSnapshot.getDefaultInstance()))
                    .build();

    private static final MethodDescriptor<holons.v1.Observability.EventsRequest, holons.v1.Observability.EventInfo> EVENTS_METHOD =
            MethodDescriptor.<holons.v1.Observability.EventsRequest, holons.v1.Observability.EventInfo>newBuilder()
                    .setType(MethodDescriptor.MethodType.SERVER_STREAMING)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_OBSERVABILITY_SERVICE, "Events"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Observability.EventsRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Observability.EventInfo.getDefaultInstance()))
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

    // --- Levels / events ---

    public enum Level {
        UNSET(0), TRACE(1), DEBUG(2), INFO(3), WARN(4), ERROR(5), FATAL(6);
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

    public enum EventType {
        UNSPECIFIED(0),
        INSTANCE_SPAWNED(1), INSTANCE_READY(2), INSTANCE_EXITED(3), INSTANCE_CRASHED(4),
        SESSION_STARTED(5), SESSION_ENDED(6),
        HANDLER_PANIC(7), CONFIG_RELOADED(8);
        public final int code;
        EventType(int c) { this.code = c; }
    }

    public static final class Hop {
        public final String slug;
        public final String instanceUid;
        public Hop(String slug, String instanceUid) { this.slug = slug; this.instanceUid = instanceUid; }
    }

    public static List<Hop> appendDirectChild(List<Hop> src, String childSlug, String childUid) {
        List<Hop> out = new ArrayList<>(src != null ? src : List.of());
        out.add(new Hop(childSlug, childUid));
        return out;
    }

    public static List<Hop> enrichForMultilog(List<Hop> wire, String srcSlug, String srcUid) {
        return appendDirectChild(wire, srcSlug, srcUid);
    }

    // --- Log entries + ring ---

    public static final class LogEntry {
        public final Instant timestamp;
        public final Level level;
        public final String slug, instanceUid, sessionId, rpcMethod, message, caller;
        public final Map<String, String> fields;
        public final List<Hop> chain;
        public final boolean privateEntry;
        public LogEntry(Instant ts, Level l, String slug, String uid, String sid, String m,
                        String msg, Map<String,String> f, String caller, List<Hop> chain) {
            this(ts, l, slug, uid, sid, m, msg, f, caller, chain, false);
        }
        public LogEntry(Instant ts, Level l, String slug, String uid, String sid, String m,
                        String msg, Map<String,String> f, String caller, List<Hop> chain, boolean privateEntry) {
            this.timestamp = ts; this.level = l; this.slug = slug; this.instanceUid = uid;
            this.sessionId = sid; this.rpcMethod = m; this.message = msg;
            this.fields = f == null ? Map.of() : Map.copyOf(f);
            this.caller = caller == null ? "" : caller;
            this.chain = chain == null ? List.of() : List.copyOf(chain);
            this.privateEntry = privateEntry;
        }
    }

    public static final class LogRing {
        private final int capacity;
        private final ArrayDeque<LogEntry> buf;
        private final CopyOnWriteArrayList<Consumer<LogEntry>> subs = new CopyOnWriteArrayList<>();

        public LogRing(int capacity) {
            this.capacity = Math.max(1, capacity);
            this.buf = new ArrayDeque<>(this.capacity);
        }

        public synchronized void push(LogEntry e) {
            if (buf.size() == capacity) buf.removeFirst();
            buf.addLast(e);
            for (Consumer<LogEntry> fn : subs) {
                try { fn.accept(e); } catch (Exception ignored) {}
            }
        }

        public synchronized List<LogEntry> drain() {
            return new ArrayList<>(buf);
        }

        public synchronized List<LogEntry> drainSince(Instant cutoff) {
            List<LogEntry> out = new ArrayList<>();
            for (LogEntry e : buf) if (!e.timestamp.isBefore(cutoff)) out.add(e);
            return out;
        }

        public AutoCloseable subscribe(Consumer<LogEntry> fn) {
            subs.add(fn);
            return () -> subs.remove(fn);
        }

        public synchronized ReplaySubscription<LogEntry> replayAndSubscribe(Instant cutoff, int bufferSize) {
            List<LogEntry> replay = cutoff == null ? new ArrayList<>(buf) : drainSince(cutoff);
            BlockingQueue<LogEntry> queue = new LinkedBlockingQueue<>(Math.max(1, bufferSize));
            Consumer<LogEntry> fn = entry -> queue.offer(entry);
            // Snapshot and subscription are in one critical section, so a
            // follow=true stream cannot miss emissions at the replay/live seam.
            subs.add(fn);
            return new ReplaySubscription<>(replay, queue, () -> subs.remove(fn));
        }

        public synchronized int size() { return buf.size(); }
    }

    // --- Events ---

    public static final class Event {
        public final Instant timestamp;
        public final EventType type;
        public final String slug, instanceUid, sessionId;
        public final Map<String, String> payload;
        public final List<Hop> chain;
        public final boolean privateEntry;
        public Event(Instant ts, EventType t, String slug, String uid, String sid,
                     Map<String,String> payload, List<Hop> chain) {
            this(ts, t, slug, uid, sid, payload, chain, false);
        }
        public Event(Instant ts, EventType t, String slug, String uid, String sid,
                     Map<String,String> payload, List<Hop> chain, boolean privateEntry) {
            this.timestamp = ts; this.type = t; this.slug = slug; this.instanceUid = uid;
            this.sessionId = sid == null ? "" : sid;
            this.payload = payload == null ? Map.of() : Map.copyOf(payload);
            this.chain = chain == null ? List.of() : List.copyOf(chain);
            this.privateEntry = privateEntry;
        }
    }

    public static final class EventBus {
        private final int capacity;
        private final ArrayDeque<Event> buf;
        private final CopyOnWriteArrayList<Consumer<Event>> subs = new CopyOnWriteArrayList<>();
        private volatile boolean closed;

        public EventBus(int capacity) {
            this.capacity = Math.max(1, capacity);
            this.buf = new ArrayDeque<>(this.capacity);
        }

        public synchronized void emit(Event e) {
            if (closed) return;
            if (buf.size() == capacity) buf.removeFirst();
            buf.addLast(e);
            for (Consumer<Event> fn : subs) {
                try { fn.accept(e); } catch (Exception ignored) {}
            }
        }

        public synchronized List<Event> drain() { return new ArrayList<>(buf); }
        public synchronized List<Event> drainSince(Instant cutoff) {
            List<Event> out = new ArrayList<>();
            for (Event e : buf) if (!e.timestamp.isBefore(cutoff)) out.add(e);
            return out;
        }

        public AutoCloseable subscribe(Consumer<Event> fn) {
            subs.add(fn);
            return () -> subs.remove(fn);
        }

        public synchronized ReplaySubscription<Event> replayAndSubscribe(Instant cutoff, int bufferSize) {
            List<Event> replay = cutoff == null ? new ArrayList<>(buf) : drainSince(cutoff);
            BlockingQueue<Event> queue = new LinkedBlockingQueue<>(Math.max(1, bufferSize));
            Consumer<Event> fn = event -> queue.offer(event);
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
            Map<String, String> out = new LinkedHashMap<>();
            if (fields != null) {
                for (Map.Entry<String, Object> e : fields.entrySet()) {
                    if (e.getKey() == null || e.getKey().isEmpty()) continue;
                    out.put(e.getKey(), redact.contains(e.getKey()) ? "<redacted>" :
                            (e.getValue() == null ? "" : e.getValue().toString()));
                }
            }
            LogEntry entry = new LogEntry(Instant.now(), l, obs.cfg.slug, obs.cfg.instanceUid,
                    "", "", msg, out, "", List.of(), privateEntry);
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

    public void emit(EventType type, Map<String, String> payload) {
        emit(type, payload, false);
    }

    public void emit(EventType type, Map<String, String> payload, boolean privateEntry) {
        if (eventBus == null) return;
        Set<String> redact = new HashSet<>(cfg.redactedFields);
        Map<String, String> p = new LinkedHashMap<>();
        if (payload != null) {
            for (Map.Entry<String, String> e : payload.entrySet()) {
                p.put(e.getKey(), redact.contains(e.getKey()) ? "<redacted>" : e.getValue());
            }
        }
        eventBus.emit(new Event(Instant.now(), type, cfg.slug, cfg.instanceUid, "", p, List.of(), privateEntry));
    }

    public void emitPrivate(EventType type, Map<String, String> payload) {
        emit(type, payload, true);
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
        if (cfg.slug == null || cfg.slug.isEmpty()) {
            cfg.slug = System.getProperty("sun.java.command", "").split(" ")[0];
        }
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

    public static MethodDescriptor<holons.v1.Observability.LogsRequest, holons.v1.Observability.LogEntry> logsMethod() {
        return LOGS_METHOD;
    }

    public static MethodDescriptor<holons.v1.Observability.MetricsRequest, holons.v1.Observability.MetricsSnapshot> metricsMethod() {
        return METRICS_METHOD;
    }

    public static MethodDescriptor<holons.v1.Observability.EventsRequest, holons.v1.Observability.EventInfo> eventsMethod() {
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
                    int minLevel = request.getMinLevelValue() == 0 ? Level.INFO.code : request.getMinLevelValue();
                    ReplaySubscription<LogEntry> subscription = null;
                    List<LogEntry> entries;
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
                    for (LogEntry entry : entries) {
                        if (request.getFollow() && entry.privateEntry) {
                            continue;
                        }
                        if (matchesLog(entry, minLevel, request.getSessionIdsList(), request.getRpcMethodsList())) {
                            observer.onNext(toProtoLogEntry(entry));
                        }
                    }
                    if (!request.getFollow()) {
                        observer.onCompleted();
                        return;
                    }
                    ReplaySubscription<LogEntry> liveSubscription = subscription;
                    Thread liveThread = new Thread(() -> {
                        try {
                            while (!Thread.currentThread().isInterrupted()) {
                                LogEntry entry = liveSubscription.live().take();
                                if (entry.privateEntry
                                        || !matchesLog(entry, minLevel, request.getSessionIdsList(), request.getRpcMethodsList())) {
                                    continue;
                                }
                                observer.onNext(toProtoLogEntry(entry));
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
                .addMethod(METRICS_METHOD, ServerCalls.asyncUnaryCall((request, observer) -> {
                    if (!obs.enabled(Family.METRICS) || obs.registry == null) {
                        observer.onError(Status.FAILED_PRECONDITION
                                .withDescription("metrics family is not enabled (OP_OBS)")
                                .asRuntimeException());
                        return;
                    }
                    holons.v1.Observability.MetricsSnapshot.Builder snapshot =
                            holons.v1.Observability.MetricsSnapshot.newBuilder()
                                    .setCapturedAt(timestamp(Instant.now()))
                                    .setSlug(obs.cfg.slug)
                                    .setInstanceUid(obs.cfg.instanceUid);
                    for (holons.v1.Observability.MetricSample sample : toProtoMetricSamples(obs.registry)) {
                        if (request.getNamePrefixesCount() == 0
                                || request.getNamePrefixesList().stream().anyMatch(prefix -> sample.getName().startsWith(prefix))) {
                            snapshot.addSamples(sample);
                        }
                    }
                    observer.onNext(snapshot.build());
                    observer.onCompleted();
                }))
                .addMethod(EVENTS_METHOD, ServerCalls.asyncServerStreamingCall((request, observer) -> {
                    if (!obs.enabled(Family.EVENTS) || obs.eventBus == null) {
                        observer.onError(Status.FAILED_PRECONDITION
                                .withDescription("events family is not enabled (OP_OBS)")
                                .asRuntimeException());
                        return;
                    }
                    Set<Integer> wanted = new HashSet<>(request.getTypesValueList());
                    ReplaySubscription<Event> subscription = null;
                    List<Event> events;
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
                    for (Event event : events) {
                        if (request.getFollow() && event.privateEntry) {
                            continue;
                        }
                        if (wanted.isEmpty() || wanted.contains(event.type.code)) {
                            observer.onNext(toProtoEvent(event));
                        }
                    }
                    if (!request.getFollow()) {
                        observer.onCompleted();
                        return;
                    }
                    ReplaySubscription<Event> liveSubscription = subscription;
                    Thread liveThread = new Thread(() -> {
                        try {
                            while (!Thread.currentThread().isInterrupted()) {
                                Event event = liveSubscription.live().take();
                                if (event.privateEntry || (!wanted.isEmpty() && !wanted.contains(event.type.code))) {
                                    continue;
                                }
                                observer.onNext(toProtoEvent(event));
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

    private static Timestamp timestamp(Instant instant) {
        return Timestamp.newBuilder()
                .setSeconds(instant.getEpochSecond())
                .setNanos(instant.getNano())
                .build();
    }

    private static Instant cutoffFromDuration(Duration duration) {
        long seconds = Math.max(0, duration.getSeconds());
        int nanos = Math.max(0, duration.getNanos());
        return Instant.now().minusSeconds(seconds).minusNanos(nanos);
    }

    private static boolean matchesLog(LogEntry entry, int minLevel, List<String> sessionIds, List<String> rpcMethods) {
        if (entry.level.code < minLevel) return false;
        if (!sessionIds.isEmpty() && !sessionIds.contains(entry.sessionId)) return false;
        return rpcMethods.isEmpty() || rpcMethods.contains(entry.rpcMethod);
    }

    public static holons.v1.Observability.LogEntry toProtoLogEntry(LogEntry entry) {
        holons.v1.Observability.LogEntry.Builder builder = holons.v1.Observability.LogEntry.newBuilder()
                .setTs(timestamp(entry.timestamp))
                .setLevelValue(entry.level.code)
                .setSlug(entry.slug)
                .setInstanceUid(entry.instanceUid)
                .setSessionId(entry.sessionId)
                .setRpcMethod(entry.rpcMethod)
                .setMessage(entry.message)
                .putAllFields(entry.fields)
                .setCaller(entry.caller);
        for (Hop hop : entry.chain) {
            builder.addChain(toProtoHop(hop));
        }
        return builder.build();
    }

    public static LogEntry fromProtoLogEntry(holons.v1.Observability.LogEntry entry) {
        Instant ts = entry.hasTs() ? instant(entry.getTs()) : Instant.now();
        List<Hop> chain = new ArrayList<>();
        for (holons.v1.Observability.ChainHop hop : entry.getChainList()) {
            chain.add(fromProtoHop(hop));
        }
        return new LogEntry(
                ts,
                levelFromCode(entry.getLevelValue()),
                entry.getSlug(),
                entry.getInstanceUid(),
                entry.getSessionId(),
                entry.getRpcMethod(),
                entry.getMessage(),
                entry.getFieldsMap(),
                entry.getCaller(),
                chain);
    }

    public static List<holons.v1.Observability.MetricSample> toProtoMetricSamples(Registry registry) {
        List<holons.v1.Observability.MetricSample> samples = new ArrayList<>();
        for (Counter counter : registry.counters()) {
            samples.add(holons.v1.Observability.MetricSample.newBuilder()
                    .setName(counter.name)
                    .putAllLabels(counter.labels)
                    .setHelp(counter.help)
                    .setCounter(counter.value())
                    .build());
        }
        for (Gauge gauge : registry.gauges()) {
            samples.add(holons.v1.Observability.MetricSample.newBuilder()
                    .setName(gauge.name)
                    .putAllLabels(gauge.labels)
                    .setHelp(gauge.help)
                    .setGauge(gauge.value())
                    .build());
        }
        for (Histogram histogram : registry.histograms()) {
            samples.add(holons.v1.Observability.MetricSample.newBuilder()
                    .setName(histogram.name)
                    .putAllLabels(histogram.labels)
                    .setHelp(histogram.help)
                    .setHistogram(toProtoHistogram(histogram.snapshot()))
                    .build());
        }
        return samples;
    }

    private static holons.v1.Observability.HistogramSample toProtoHistogram(HistogramSnapshot snapshot) {
        holons.v1.Observability.HistogramSample.Builder builder =
                holons.v1.Observability.HistogramSample.newBuilder()
                        .setCount(snapshot.total)
                        .setSum(snapshot.sum);
        for (int i = 0; i < snapshot.bounds.length; i++) {
            builder.addBuckets(holons.v1.Observability.Bucket.newBuilder()
                    .setUpperBound(snapshot.bounds[i])
                    .setCount(snapshot.counts[i])
                    .build());
        }
        return builder.build();
    }

    public static holons.v1.Observability.EventInfo toProtoEvent(Event event) {
        holons.v1.Observability.EventInfo.Builder builder = holons.v1.Observability.EventInfo.newBuilder()
                .setTs(timestamp(event.timestamp))
                .setTypeValue(event.type.code)
                .setSlug(event.slug)
                .setInstanceUid(event.instanceUid)
                .setSessionId(event.sessionId)
                .putAllPayload(event.payload);
        for (Hop hop : event.chain) {
            builder.addChain(toProtoHop(hop));
        }
        return builder.build();
    }

    public static Event fromProtoEvent(holons.v1.Observability.EventInfo event) {
        Instant ts = event.hasTs() ? instant(event.getTs()) : Instant.now();
        List<Hop> chain = new ArrayList<>();
        for (holons.v1.Observability.ChainHop hop : event.getChainList()) {
            chain.add(fromProtoHop(hop));
        }
        return new Event(
                ts,
                eventTypeFromCode(event.getTypeValue()),
                event.getSlug(),
                event.getInstanceUid(),
                event.getSessionId(),
                event.getPayloadMap(),
                chain);
    }

    private static holons.v1.Observability.ChainHop toProtoHop(Hop hop) {
        return holons.v1.Observability.ChainHop.newBuilder()
                .setSlug(hop.slug)
                .setInstanceUid(hop.instanceUid)
                .build();
    }

    private static Hop fromProtoHop(holons.v1.Observability.ChainHop hop) {
        return new Hop(hop.getSlug(), hop.getInstanceUid());
    }

    private static Instant instant(Timestamp ts) {
        return Instant.ofEpochSecond(ts.getSeconds(), ts.getNanos());
    }

    private static Level levelFromCode(int code) {
        for (Level level : Level.values()) {
            if (level.code == code) {
                return level;
            }
        }
        return Level.UNSET;
    }

    private static EventType eventTypeFromCode(int code) {
        for (EventType type : EventType.values()) {
            if (type.code == code) {
                return type;
            }
        }
        return EventType.UNSPECIFIED;
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
            Iterator<holons.v1.Observability.EventInfo> iterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    EVENTS_METHOD,
                    CallOptions.DEFAULT,
                    holons.v1.Observability.EventsRequest.newBuilder()
                            .addTypesValue(EventType.INSTANCE_READY.code)
                            .build());
            while (iterator.hasNext()) {
                holons.v1.Observability.EventInfo event = iterator.next();
                if (event.getChainCount() == 0 && !event.getInstanceUid().isBlank()) {
                    String slug = event.getSlug().isBlank() ? fallbackSlugValue : event.getSlug();
                    return new MemberIdentity(slug, event.getInstanceUid());
                }
            }
        } catch (Exception ignored) {
            // Fall back to the Metrics snapshot.
        }

        try {
            holons.v1.Observability.MetricsSnapshot snapshot = ClientCalls.blockingUnaryCall(
                    channel,
                    METRICS_METHOD,
                    CallOptions.DEFAULT,
                    holons.v1.Observability.MetricsRequest.getDefaultInstance());
            if (!snapshot.getInstanceUid().isBlank()) {
                String slug = snapshot.getSlug().isBlank() ? fallbackSlugValue : snapshot.getSlug();
                return new MemberIdentity(slug, snapshot.getInstanceUid());
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
                    Iterator<holons.v1.Observability.LogEntry> iterator = ClientCalls.blockingServerStreamingCall(
                            channel,
                            LOGS_METHOD,
                            CallOptions.DEFAULT,
                            holons.v1.Observability.LogsRequest.newBuilder()
                                    .setFollow(true)
                                    .setMinLevelValue(Level.INFO.code)
                                    .build());
                    while (!stopped && iterator.hasNext()) {
                        LogEntry entry = fromProtoLogEntry(iterator.next());
                        LogEntry enriched = new LogEntry(
                                entry.timestamp,
                                entry.level,
                                entry.slug,
                                entry.instanceUid,
                                entry.sessionId,
                                entry.rpcMethod,
                                entry.message,
                                entry.fields,
                                entry.caller,
                                appendDirectChild(entry.chain, childSlug, childUid));
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
                    Iterator<holons.v1.Observability.EventInfo> iterator = ClientCalls.blockingServerStreamingCall(
                            channel,
                            EVENTS_METHOD,
                            CallOptions.DEFAULT,
                            holons.v1.Observability.EventsRequest.newBuilder()
                                    .setFollow(true)
                                    .build());
                    while (!stopped && iterator.hasNext()) {
                        Event event = fromProtoEvent(iterator.next());
                        Event enriched = new Event(
                                event.timestamp,
                                event.type,
                                event.slug,
                                event.instanceUid,
                                event.sessionId,
                                event.payload,
                                appendDirectChild(event.chain, childSlug, childUid));
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

    private static void appendLogJsonl(Path fp, LogEntry e) {
        StringBuilder sb = new StringBuilder();
        sb.append("{\"kind\":\"log\"")
          .append(",\"ts\":\"").append(e.timestamp.toString()).append("\"")
          .append(",\"level\":\"").append(e.level.label()).append("\"")
          .append(",\"slug\":").append(quote(e.slug))
          .append(",\"instance_uid\":").append(quote(e.instanceUid))
          .append(",\"message\":").append(quote(e.message));
        if (!e.sessionId.isEmpty()) sb.append(",\"session_id\":").append(quote(e.sessionId));
        if (!e.rpcMethod.isEmpty()) sb.append(",\"rpc_method\":").append(quote(e.rpcMethod));
        if (!e.fields.isEmpty()) { sb.append(",\"fields\":"); jsonMap(sb, e.fields); }
        if (!e.caller.isEmpty()) sb.append(",\"caller\":").append(quote(e.caller));
        if (!e.chain.isEmpty()) { sb.append(",\"chain\":"); jsonChain(sb, e.chain); }
        sb.append("}\n");
        append(fp, sb.toString());
    }

    private static void appendEventJsonl(Path fp, Event e) {
        StringBuilder sb = new StringBuilder();
        sb.append("{\"kind\":\"event\"")
          .append(",\"ts\":\"").append(e.timestamp.toString()).append("\"")
          .append(",\"type\":\"").append(e.type.name()).append("\"")
          .append(",\"slug\":").append(quote(e.slug))
          .append(",\"instance_uid\":").append(quote(e.instanceUid));
        if (!e.sessionId.isEmpty()) sb.append(",\"session_id\":").append(quote(e.sessionId));
        if (!e.payload.isEmpty()) { sb.append(",\"payload\":"); jsonMap(sb, e.payload); }
        if (!e.chain.isEmpty()) { sb.append(",\"chain\":"); jsonChain(sb, e.chain); }
        sb.append("}\n");
        append(fp, sb.toString());
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

    private static void jsonChain(StringBuilder sb, List<Hop> c) {
        sb.append("[");
        boolean first = true;
        for (Hop h : c) {
            if (!first) sb.append(",");
            first = false;
            sb.append("{\"slug\":").append(quote(h.slug))
              .append(",\"instance_uid\":").append(quote(h.instanceUid)).append("}");
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

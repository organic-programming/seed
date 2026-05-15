package org.organicprogramming.holons;

import com.google.gson.Gson;
import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.stub.ClientCalls;

import java.io.Closeable;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.SecureRandom;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;

public final class Composite {
    private static final Gson GSON = new Gson();
    private static final SecureRandom RANDOM = new SecureRandom();

    public static final List<String> TRANSPORT_COVERAGE_SEQUENCE = List.of(
            "stdio", "stdio", "tcp", "unix", "tcp", "tcp", "stdio", "unix", "unix", "stdio");

    private Composite() {}

    public static Path member(String id) throws IOException {
        String executable = System.getenv("OP_HOLON_EXECUTABLE");
        if (executable == null || executable.isBlank()) {
            executable = ProcessHandle.current().info().command().orElse("");
        }
        if (executable.isBlank()) {
            throw new IOException("OP_HOLON_EXECUTABLE is not set");
        }
        return memberFromExecutable(Path.of(executable), id);
    }

    public static Path memberFromExecutable(Path executable, String id) throws IOException {
        if (id == null || id.isBlank()) {
            throw new IllegalArgumentException("member id is required");
        }
        Path memberDir = executable.toAbsolutePath().normalize().getParent().resolve("holons").resolve(id);
        if (!Files.isDirectory(memberDir)) {
            throw new IOException("member directory not found: " + memberDir);
        }
        try (var stream = Files.list(memberDir)) {
            return stream
                    .filter(Files::isRegularFile)
                    .filter(path -> Files.isExecutable(path) || path.getFileName().toString().endsWith(".exe"))
                    .sorted()
                    .findFirst()
                    .orElseThrow(() -> new IOException("no executable found in " + memberDir));
        }
    }

    public record ChildSpec(String slug, String binary) {
        public ChildSpec {
            slug = slug == null ? "" : slug.trim();
            binary = binary == null ? "" : binary.trim();
        }
    }

    public interface DialOption {
        void apply(DialOptions options);
    }

    public static final class DialOptions {
        private Boolean transitiveObservability;

        public Boolean transitiveObservability() {
            return transitiveObservability;
        }
    }

    public static DialOption withTransitiveObservability(boolean enabled) {
        return options -> options.transitiveObservability = enabled;
    }

    public static final class SpawnOptions {
        public String slug = "";
        public String binaryPath = "";
        public String transport = "stdio";
        public String instanceUid = "";
        public List<ChildSpec> downstreamChain = List.of();
        public Map<String, String> extraEnv = Map.of();
        public List<DialOption> dialOptions = List.of();
    }

    public static final class SpawnedMember implements AutoCloseable {
        private final Process process;
        private final Closeable bridge;
        private final Observability.MemberRelay relay;
        private boolean stopped;

        public final String slug;
        public final String uid;
        public final String listenUri;
        public final ManagedChannel conn;

        private SpawnedMember(
                String slug,
                String uid,
                String listenUri,
                ManagedChannel conn,
                Process process,
                Closeable bridge,
                Observability.MemberRelay relay) {
            this.slug = slug;
            this.uid = uid;
            this.listenUri = listenUri;
            this.conn = conn;
            this.process = process;
            this.bridge = bridge;
            this.relay = relay;
        }

        public synchronized void stop() {
            stop(Duration.ofSeconds(3));
        }

        public synchronized void stop(Duration timeout) {
            if (stopped) {
                return;
            }
            stopped = true;
            if (relay != null) {
                relay.close();
            }
            Connect.disconnect(conn);
            closeQuietly(bridge);
            if (process == null || !process.isAlive()) {
                return;
            }
            process.destroy();
            try {
                if (!process.waitFor(Math.max(1, timeout.toMillis()), TimeUnit.MILLISECONDS)) {
                    process.destroyForcibly();
                    process.waitFor(Math.max(1, timeout.toMillis()), TimeUnit.MILLISECONDS);
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                process.destroyForcibly();
            }
        }

        @Override
        public void close() {
            stop();
        }
    }

    public static SpawnedMember spawnMember(SpawnOptions opts) throws IOException {
        Objects.requireNonNull(opts, "opts");
        String slug = opts.slug == null || opts.slug.isBlank()
                ? Path.of(opts.binaryPath == null ? "" : opts.binaryPath).getFileName().toString()
                : opts.slug.trim();
        String binary = opts.binaryPath == null ? "" : opts.binaryPath.trim();
        if (slug.isBlank()) {
            throw new IllegalArgumentException("spawn member: slug is required");
        }
        if (binary.isBlank()) {
            throw new IllegalArgumentException("spawn member " + slug + ": binary path is required");
        }
        String uid = opts.instanceUid == null || opts.instanceUid.isBlank() ? newInstanceUid() : opts.instanceUid.trim();
        String transport = opts.transport == null || opts.transport.isBlank() ? "stdio" : opts.transport.trim().toLowerCase();
        String listenUri = listenUriForSpawn(transport, uid);
        if (listenUri.startsWith("unix://")) {
            Files.deleteIfExists(Path.of(listenUri.substring("unix://".length())));
        }

        List<String> command = new ArrayList<>();
        command.add(binary);
        command.add("serve");
        command.add("--listen");
        command.add(listenUri);
        command.add("--transport");
        command.add(transport);
        for (ChildSpec child : opts.downstreamChain == null ? List.<ChildSpec>of() : opts.downstreamChain) {
            if (child.slug().isBlank() || child.binary().isBlank()) {
                throw new IllegalArgumentException("spawn member " + slug + ": downstream child requires slug and binary");
            }
            command.add("--child");
            command.add(child.slug() + "=" + child.binary());
        }

        ProcessBuilder builder = new ProcessBuilder(command);
        Path parent = Path.of(binary).toAbsolutePath().normalize().getParent();
        if (parent != null) {
            builder.directory(parent.toFile());
        }
        Map<String, String> env = builder.environment();
        env.putAll(spawnEnvironment(uid, opts.extraEnv));

        Process process = builder.start();
        Closeable bridge = null;
        ManagedChannel channel;
        String publicUri;
        try {
            if ("stdio".equals(transport)) {
                startDrainThread(process.getErrorStream(), "holons-spawn-" + slug + "-stderr");
                StdioBridge stdio = new StdioBridge(process);
                bridge = stdio;
                publicUri = "stdio://";
                channel = Connect.connect(stdio.uri(), new Connect.ConnectOptions(Duration.ofSeconds(10), "tcp", false, null));
            } else {
                startDrainThread(process.getInputStream(), "holons-spawn-" + slug + "-stdout");
                startDrainThread(process.getErrorStream(), "holons-spawn-" + slug + "-stderr");
                MetaJson meta = waitSpawnMeta(runRootFromEnv(env), slug, uid, Duration.ofSeconds(10));
                publicUri = meta.address;
                channel = Connect.connect(publicUri, new Connect.ConnectOptions(Duration.ofSeconds(10), "tcp", false, null));
            }
            describeReady(channel, Duration.ofSeconds(10));
        } catch (Exception error) {
            closeQuietly(bridge);
            stopProcess(process);
            if (error instanceof IOException ioException) {
                throw ioException;
            }
            throw new IOException("spawn member " + slug + ": " + error.getMessage(), error);
        }

        boolean transitive = true;
        DialOptions dialOptions = applyDialOptions(opts.dialOptions);
        if (dialOptions.transitiveObservability() != null) {
            transitive = dialOptions.transitiveObservability();
        }
        Observability.MemberRelay relay = null;
        if (transitive) {
            relay = new Observability.MemberRelay(slug, uid, channel, Observability.current());
            relay.start();
        }
        return new SpawnedMember(slug, uid, publicUri, channel, process, bridge, relay);
    }

    public static final class CascadeOptions {
        public String transport = "stdio";
        public List<ChildSpec> members = List.of();
        public Map<String, String> extraEnv = Map.of();
    }

    public static final class Cascade implements AutoCloseable {
        public final SpawnedMember top;

        private Cascade(SpawnedMember top) {
            this.top = top;
        }

        public void stop() {
            if (top != null) {
                top.stop();
            }
        }

        @Override
        public void close() {
            stop();
        }
    }

    public static Cascade buildCascade(CascadeOptions opts) throws IOException {
        Objects.requireNonNull(opts, "opts");
        if (opts.members == null || opts.members.isEmpty()) {
            throw new IllegalArgumentException("build cascade: at least one member is required");
        }
        ChildSpec top = opts.members.get(0);
        SpawnOptions spawn = new SpawnOptions();
        spawn.slug = top.slug();
        spawn.binaryPath = top.binary();
        spawn.transport = opts.transport;
        spawn.downstreamChain = opts.members.subList(1, opts.members.size());
        spawn.extraEnv = opts.extraEnv;
        return new Cascade(spawnMember(spawn));
    }

    public static ManagedChannel dial(String address, DialOption... options) throws IOException {
        String target = normalizeAddressForDial(address);
        ManagedChannel channel = Connect.connect(target, new Connect.ConnectOptions(Duration.ofSeconds(10), "tcp", false, null));
        DialOptions dialOptions = applyDialOptions(List.of(options));
        if (Boolean.TRUE.equals(dialOptions.transitiveObservability())) {
            Observability.MemberIdentity identity = Observability.resolveMemberIdentity(channel, "", "");
            Observability.MemberRelay relay = new Observability.MemberRelay(identity.slug(), identity.uid(), channel, Observability.current());
            relay.start();
        }
        return channel;
    }

    public record ParsedChildFlags(List<ChildSpec> children, String[] remaining) {
    }

    public static ParsedChildFlags parseChildFlags(String[] args) {
        List<ChildSpec> children = new ArrayList<>();
        List<String> remaining = new ArrayList<>();
        for (int index = 0; index < args.length; index++) {
            String arg = args[index];
            if ("--child".equals(arg)) {
                index++;
                if (index >= args.length) {
                    throw new IllegalArgumentException("--child requires <slug>=<binary>");
                }
                children.add(parseChild(args[index]));
            } else if (arg.startsWith("--child=")) {
                children.add(parseChild(arg.substring("--child=".length())));
            } else {
                remaining.add(arg);
            }
        }
        return new ParsedChildFlags(List.copyOf(children), remaining.toArray(String[]::new));
    }

    private static ChildSpec parseChild(String raw) {
        int idx = raw.indexOf('=');
        if (idx < 0) {
            throw new IllegalArgumentException("--child requires <slug>=<binary>");
        }
        String slug = raw.substring(0, idx).trim();
        String binary = raw.substring(idx + 1).trim();
        if (slug.isEmpty() || binary.isEmpty()) {
            throw new IllegalArgumentException("--child requires non-empty slug and binary");
        }
        return new ChildSpec(slug, binary);
    }

    public record ChainHop(String slug, String instanceUid) {
    }

    public record CheckOutcome(boolean pass, String evidence) {
    }

    public static final class LogCheckOptions {
        public ManagedChannel conn;
        public String sender = "";
        public String leafUid = "";
        public List<ChainHop> expectedChain = List.of();
        public Duration timeout = Duration.ofSeconds(3);
        public Duration pollInterval = Duration.ofMillis(100);
        public boolean live;
    }

    public static final class EventCheckOptions {
        public ManagedChannel conn;
        public Observability.EventType eventType = Observability.EventType.INSTANCE_READY;
        public String leafUid = "";
        public List<ChainHop> expectedChain = List.of();
        public Duration timeout = Duration.ofSeconds(3);
        public Duration pollInterval = Duration.ofMillis(100);
        public boolean live;
    }

    public static CheckOutcome checkRelayedLog(LogCheckOptions opts) {
        long deadline = System.nanoTime() + opts.timeout.toNanos();
        CheckOutcome last = new CheckOutcome(false, "");
        while (true) {
            try {
                last = matchRelayedLog(readLogEntries(opts.conn), opts);
                if (last.pass()) {
                    return last;
                }
            } catch (Exception error) {
                last = new CheckOutcome(false, compactEvidence(error.getMessage()));
            }
            if (System.nanoTime() > deadline) {
                return last;
            }
            sleep(opts.pollInterval.toMillis());
        }
    }

    public static CheckOutcome checkRelayedEvent(EventCheckOptions opts) {
        long deadline = System.nanoTime() + opts.timeout.toNanos();
        CheckOutcome last = new CheckOutcome(false, "");
        while (true) {
            try {
                last = matchRelayedEvent(readEventEntries(opts.conn), opts);
                if (last.pass()) {
                    return last;
                }
            } catch (Exception error) {
                last = new CheckOutcome(false, compactEvidence(error.getMessage()));
            }
            if (System.nanoTime() > deadline) {
                return last;
            }
            sleep(opts.pollInterval.toMillis());
        }
    }

    private static List<Observability.LogEntry> readLogEntries(ManagedChannel conn) {
        if (conn == null) {
            return Observability.current().logRing == null ? List.of() : Observability.current().logRing.drain();
        }
        var iterator = ClientCalls.blockingServerStreamingCall(
                conn,
                Observability.logsMethod(),
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                holons.v1.Observability.LogsRequest.newBuilder()
                        .setMinLevel(holons.v1.Observability.LogLevel.INFO)
                        .build());
        List<Observability.LogEntry> out = new ArrayList<>();
        iterator.forEachRemaining(entry -> out.add(Observability.fromProtoLogEntry(entry)));
        return out;
    }

    private static List<Observability.Event> readEventEntries(ManagedChannel conn) {
        if (conn == null) {
            return Observability.current().eventBus == null ? List.of() : Observability.current().eventBus.drain();
        }
        var iterator = ClientCalls.blockingServerStreamingCall(
                conn,
                Observability.eventsMethod(),
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                holons.v1.Observability.EventsRequest.getDefaultInstance());
        List<Observability.Event> out = new ArrayList<>();
        iterator.forEachRemaining(event -> out.add(Observability.fromProtoEvent(event)));
        return out;
    }

    private static CheckOutcome matchRelayedLog(List<Observability.LogEntry> entries, LogCheckOptions opts) {
        for (Observability.LogEntry entry : entries) {
            if (!"tick received".equals(entry.message)) {
                continue;
            }
            if (!opts.sender.equals(entry.fields.getOrDefault("sender", ""))
                    || !opts.leafUid.equals(entry.fields.getOrDefault("responder_uid", ""))) {
                continue;
            }
            String evidence = compareChain(entry.chain, opts.expectedChain);
            if (!evidence.isEmpty()) {
                return new CheckOutcome(false, compactEvidence("matching log bad chain: " + evidence));
            }
            return new CheckOutcome(true, "");
        }
        return new CheckOutcome(false, compactEvidence(
                "no relayed tick log sender=" + opts.sender + " leaf_uid=" + opts.leafUid + " entries=" + entries.size()));
    }

    private static CheckOutcome matchRelayedEvent(List<Observability.Event> events, EventCheckOptions opts) {
        for (Observability.Event event : events) {
            if (event.type != opts.eventType || !opts.leafUid.equals(event.instanceUid)) {
                continue;
            }
            String evidence = compareChain(event.chain, opts.expectedChain);
            if (!evidence.isEmpty()) {
                return new CheckOutcome(false, compactEvidence("matching event bad chain: " + evidence));
            }
            return new CheckOutcome(true, "");
        }
        return new CheckOutcome(false, compactEvidence(
                "no relayed " + opts.eventType + " event leaf_uid=" + opts.leafUid + " events=" + events.size()));
    }

    private static String compareChain(List<Observability.Hop> got, List<ChainHop> want) {
        if (got.size() != want.size()) {
            return "chain length " + got.size() + " want " + want.size();
        }
        for (int index = 0; index < want.size(); index++) {
            Observability.Hop actual = got.get(index);
            ChainHop expected = want.get(index);
            if (!expected.slug().equals(actual.slug) || !expected.instanceUid().equals(actual.instanceUid)) {
                return "hop " + index + "=" + actual.slug + "/" + actual.instanceUid
                        + " want " + expected.slug() + "/" + expected.instanceUid();
            }
        }
        return "";
    }

    private static DialOptions applyDialOptions(List<DialOption> options) {
        DialOptions out = new DialOptions();
        if (options != null) {
            for (DialOption option : options) {
                if (option != null) {
                    option.apply(out);
                }
            }
        }
        return out;
    }

    private static String listenUriForSpawn(String transport, String uid) {
        return switch (transport) {
            case "stdio" -> "stdio://";
            case "tcp" -> "tcp://127.0.0.1:0";
            case "unix" -> "unix://" + Path.of(System.getProperty("java.io.tmpdir"),
                    "op-" + cleanSocketToken(uid) + ".sock");
            default -> throw new IllegalArgumentException("unsupported transport \"" + transport + "\"");
        };
    }

    private static Map<String, String> spawnEnvironment(String uid, Map<String, String> extra) {
        Map<String, String> env = new LinkedHashMap<>();
        env.put("OP_INSTANCE_UID", uid);
        env.put("OP_RUN_DIR", runRootFromEnv(System.getenv()));
        env.put("HOLONS_PARENT_PID", Long.toString(ProcessHandle.current().pid()));
        String families = activeObservabilityFamilies();
        if (!families.isBlank()) {
            env.put("OP_OBS", families);
        }
        if (extra != null) {
            env.putAll(extra);
        }
        return env;
    }

    private static String activeObservabilityFamilies() {
        Observability obs = Observability.current();
        List<String> families = new ArrayList<>();
        if (obs.enabled(Observability.Family.LOGS)) families.add("logs");
        if (obs.enabled(Observability.Family.METRICS)) families.add("metrics");
        if (obs.enabled(Observability.Family.EVENTS)) families.add("events");
        if (obs.enabled(Observability.Family.PROM)) families.add("prom");
        return String.join(",", families);
    }

    private static String runRootFromEnv(Map<String, String> env) {
        if (env != null && env.get("OP_RUN_DIR") != null && !env.get("OP_RUN_DIR").isBlank()) {
            return env.get("OP_RUN_DIR");
        }
        if (env != null && env.get("OPPATH") != null && !env.get("OPPATH").isBlank()) {
            return Path.of(env.get("OPPATH"), "run").toString();
        }
        String home = env == null ? "" : env.getOrDefault("HOME", "");
        if (!home.isBlank()) {
            return Path.of(home, ".op", "run").toString();
        }
        return Path.of(System.getProperty("java.io.tmpdir"), ".op", "run").toString();
    }

    private static MetaJson waitSpawnMeta(String runRoot, String slug, String uid, Duration timeout) throws IOException {
        Path metaPath = Path.of(runRoot, slug, uid, "meta.json");
        long deadline = System.nanoTime() + timeout.toNanos();
        Exception last = null;
        while (System.nanoTime() < deadline) {
            try {
                MetaJson meta = GSON.fromJson(Files.readString(metaPath), MetaJson.class);
                if (uid.equals(meta.uid) && meta.address != null && !meta.address.isBlank()) {
                    return meta;
                }
            } catch (Exception error) {
                last = error;
            }
            sleep(50);
        }
        throw new IOException("meta not ready for " + slug + "/" + uid + ": "
                + (last == null ? "timeout" : last.getMessage()));
    }

    private static void describeReady(ManagedChannel channel, Duration timeout) throws IOException {
        long deadline = System.nanoTime() + timeout.toNanos();
        Exception last = null;
        while (System.nanoTime() < deadline) {
            try {
                ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT.withDeadlineAfter(500, TimeUnit.MILLISECONDS),
                        holons.v1.Describe.DescribeRequest.getDefaultInstance());
                return;
            } catch (Exception error) {
                last = error;
                sleep(50);
            }
        }
        throw new IOException("describe readiness failed: " + (last == null ? "timeout" : last.getMessage()));
    }

    private static String normalizeAddressForDial(String address) {
        String trimmed = address == null ? "" : address.trim();
        if (trimmed.isEmpty()) {
            throw new IllegalArgumentException("dial address is required");
        }
        if (trimmed.startsWith("stdio://")) {
            throw new IllegalArgumentException("Composite.dial does not support stdio addresses; use spawnMember");
        }
        if (trimmed.startsWith("tcp://") || trimmed.startsWith("unix://")) {
            return trimmed;
        }
        if (trimmed.contains("://")) {
            throw new IllegalArgumentException("unsupported dial address \"" + address + "\"");
        }
        if (!trimmed.contains(":")) {
            throw new IllegalArgumentException("dial address must be tcp://host:port, unix:///path, or host:port");
        }
        return trimmed;
    }

    private static String newInstanceUid() {
        byte[] bytes = new byte[12];
        RANDOM.nextBytes(bytes);
        StringBuilder out = new StringBuilder();
        for (byte b : bytes) {
            out.append(String.format("%02x", b & 0xff));
        }
        return out.toString();
    }

    private static String cleanSocketToken(String value) {
        String token = value == null ? "" : value.trim();
        if (token.length() > 24) {
            token = token.substring(0, 24);
        }
        return token.replaceAll("[/\\\\: ]", "-");
    }

    private static void stopProcess(Process process) {
        if (process == null || !process.isAlive()) {
            return;
        }
        process.destroy();
        try {
            if (!process.waitFor(2, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                process.waitFor(2, TimeUnit.SECONDS);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            process.destroyForcibly();
        }
    }

    private static void startDrainThread(InputStream stream, String name) {
        Thread thread = new Thread(() -> {
            byte[] buffer = new byte[4096];
            try {
                while (stream.read(buffer) >= 0) {
                    // Drain only; child process logs are not protocol output.
                }
            } catch (IOException ignored) {
            }
        }, name);
        thread.setDaemon(true);
        thread.start();
    }

    private static void sleep(long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    private static String compactEvidence(String value) {
        String compact = String.join(" ", String.valueOf(value == null ? "" : value).split("\\s+")).trim();
        if (compact.length() <= 240) {
            return compact;
        }
        return compact.substring(0, 240) + "...";
    }

    private static void closeQuietly(AutoCloseable closeable) {
        if (closeable == null) {
            return;
        }
        try {
            closeable.close();
        } catch (Exception ignored) {
        }
    }

    private record MetaJson(String uid, String address) {
    }

    private static final class StdioBridge implements Closeable {
        private final Process process;
        private final ServerSocket listener;
        private final Thread acceptThread;
        private volatile Socket socket;
        private volatile boolean closed;

        private StdioBridge(Process process) throws IOException {
            this.process = process;
            this.listener = new ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"));
            this.acceptThread = new Thread(this::acceptLoop, "holons-composite-stdio-accept");
            this.acceptThread.setDaemon(true);
            this.acceptThread.start();
        }

        private String uri() {
            return "tcp://127.0.0.1:" + listener.getLocalPort();
        }

        @Override
        public void close() throws IOException {
            closed = true;
            listener.close();
            Socket active = socket;
            socket = null;
            if (active != null) {
                active.close();
            }
            process.getOutputStream().close();
            process.getInputStream().close();
            try {
                acceptThread.join(200);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private void acceptLoop() {
            try {
                Socket accepted = listener.accept();
                if (closed) {
                    accepted.close();
                    return;
                }
                socket = accepted;
                Thread upstream = startPump(accepted.getInputStream(), process.getOutputStream(), true,
                        "holons-composite-stdio-up");
                Thread downstream = startPump(process.getInputStream(), accepted.getOutputStream(), true,
                        "holons-composite-stdio-down");
                upstream.join();
                downstream.join();
            } catch (IOException ignored) {
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private static Thread startPump(InputStream input, OutputStream output, boolean closeOutput, String name) {
            Thread thread = new Thread(() -> {
                byte[] buffer = new byte[16 * 1024];
                try {
                    while (true) {
                        int read = input.read(buffer);
                        if (read < 0) {
                            break;
                        }
                        output.write(buffer, 0, read);
                        output.flush();
                    }
                } catch (IOException ignored) {
                } finally {
                    if (closeOutput) {
                        closeQuietly(output);
                    }
                }
            }, name);
            thread.setDaemon(true);
            thread.start();
            return thread;
        }
    }
}

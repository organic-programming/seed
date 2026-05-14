package org.organicprogramming.observability.cascade.javaapp;

import com.google.gson.Gson;
import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.stub.ClientCalls;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.holons.Connect;
import org.organicprogramming.holons.Composite;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Observability;
import org.organicprogramming.holons.Serve;
import observability_cascade.v1.ObservabilityCascadeServiceGrpc;
import observability_cascade.v1.Service;
import relay.v1.Relay;
import relay.v1.RelayServiceGrpc;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public final class Main {
    private static final int RUN_PHASES = 4;
    private static final int RUN_TICKS = 3;
    private static final List<String> ROLE_ORDER = List.of("D", "C", "B", "A");
    private static final List<String> TRANSPORTS = List.of("tcp", "unix", "tcp", "unix");
    private static final String JAVA_SLUG = "observability-cascade-java-node";
    private static final String GO_SLUG = "observability-cascade-go-node";
    private static final Gson GSON = new Gson();
    private static final HttpClient HTTP = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(2))
            .build();

    private final Path repoRoot;

    private Main() {
        repoRoot = findRepoRoot(Path.of(System.getProperty("user.dir")));
    }

    public static void main(String[] args) {
        try {
            Main main = new Main();
            if (args.length > 0 && "serve".equals(canonicalCommand(args[0]))) {
                main.serveComposite(Arrays.copyOfRange(args, 1, args.length));
            } else if (Arrays.asList(args).contains("--multi-pattern")) {
                main.runMultiPattern(true);
            } else if (Arrays.asList(args).contains("--live-stream")) {
                main.runLiveStream(true);
            } else {
                main.runDefault(true);
            }
        } catch (Exception error) {
            System.err.println();
            System.err.println("FAIL: " + error.getMessage());
            System.exit(1);
        }
    }

    private void serveComposite(String[] args) throws Exception {
        Describe.useStaticResponse(gen.describe_generated.StaticDescribeResponse());
        Serve.ParsedFlags parsed = Serve.parseOptions(args);
        Serve.runWithOptions(
                normalizeListenUri(parsed.listenUri()),
                List.of(new ObservabilityCascadeServer(this)),
                new Serve.Options()
                        .withReflect(parsed.reflect())
                        .withSlug("observability-cascade-java"));
    }

    private static final class ObservabilityCascadeServer extends ObservabilityCascadeServiceGrpc.ObservabilityCascadeServiceImplBase {
        private final Main app;

        private ObservabilityCascadeServer(Main app) {
            this.app = app;
        }

        @Override
        public void runDefault(Service.RunRequest request, StreamObserver<Service.CascadeReport> responseObserver) {
            try {
                responseObserver.onNext(toCascadeReport(app.runDefault(false)));
                responseObserver.onCompleted();
            } catch (Exception error) {
                responseObserver.onError(error);
            }
        }

        @Override
        public void runLiveStream(Service.RunRequest request, StreamObserver<Service.CascadeReport> responseObserver) {
            try {
                responseObserver.onNext(toCascadeReport(app.runLiveStream(false)));
                responseObserver.onCompleted();
            } catch (Exception error) {
                responseObserver.onError(error);
            }
        }

        @Override
        public void runMultiPattern(Service.RunRequest request, StreamObserver<Service.MultiPatternReport> responseObserver) {
            try {
                responseObserver.onNext(toMultiPatternReport(app.runMultiPattern(false)));
                responseObserver.onCompleted();
            } catch (Exception error) {
                responseObserver.onError(error);
            }
        }
    }

    private CascadeReportData runDefault(boolean emit) throws Exception {
        Path binary = findBinary(JAVA_SLUG);
        Path runRoot = Files.createTempDirectory("observability-cascade-java-");
        output(emit, "=== observability-cascade-java ===");
        output(emit);
        int totalPass = 0;
        int totalFail = 0;
        String previous = "";
        for (int index = 0; index < TRANSPORTS.size(); index++) {
            int phase = index + 1;
            String transport = TRANSPORTS.get(index);
            if (previous.isEmpty()) {
                outputf(emit, "Phase %d/%d: transport=%s%n", phase, RUN_PHASES, transport);
            } else {
                outputf(emit, "Phase %d/%d: transport=%s (switching from %s)%n", phase, RUN_PHASES, transport, previous);
            }
            long started = nowMillis();
            Cascade cascade;
            try {
                Map<String, RoleSpec> specs = allJavaSpecs(binary);
                cascade = spawnCascade(phase, transport, specs, runRoot);
            } catch (Exception error) {
                totalFail += RUN_TICKS;
                outputf(emit, "  spawn FAIL: %s%n%n", error.getMessage());
                previous = transport;
                continue;
            }
            outputf(emit, "  spawned 4 nodes in %s%n", elapsed(started));
            double previousMetric = 0;
            for (int tick = 1; tick <= RUN_TICKS; tick++) {
                long tickStart = nowMillis();
                TickOutcome outcome = cascade.runTick(tick, previousMetric);
                if (outcome.metric.pass) {
                    previousMetric = outcome.metricValue;
                }
                boolean overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
                if (overall) {
                    totalPass++;
                } else {
                    totalFail++;
                }
                outputf(emit,
                        "  Tick %d/%d: log %s, event %s, metric %s (overall %s in %s)%n",
                        tick,
                        RUN_TICKS,
                        passText(outcome.log.pass),
                        passText(outcome.event.pass),
                        passText(outcome.metric.pass),
                        passText(overall),
                        elapsed(tickStart));
                if (emit) {
                    printFailureEvidence("log", outcome.log);
                    printFailureEvidence("event", outcome.event);
                    printFailureEvidence("metric", outcome.metric);
                }
            }
            cascade.stop();
            output(emit);
            previous = transport;
        }
        outputf(emit, "Summary: %d ticks, %d PASS, %d FAIL%n", totalPass + totalFail, totalPass, totalFail);
        if (totalFail > 0) {
            throw new IllegalStateException(totalFail + " tick(s) failed");
        }
        return new CascadeReportData(
                totalPass + totalFail,
                totalPass,
                totalFail,
                List.of(new PhaseReportData("default", totalPass, totalFail, List.of())));
    }

    private CascadeReportData runLiveStream(boolean emit) throws Exception {
        Path binary = findBinary(JAVA_SLUG);
        Path runRoot = Files.createTempDirectory("observability-cascade-java-live-");
        output(emit, "=== observability-cascade-java --live-stream ===");
        output(emit);
        output(emit, "Setup: opening long-lived Follow:true streams on A");
        output(emit, "       (initial transport: tcp)");
        output(emit);
        int totalPass = 0;
        int totalFail = 0;
        Cascade cascade = null;
        LiveStreams streams = null;
        Map<String, RoleSpec> specs = allJavaSpecs(binary);
        for (int index = 0; index < TRANSPORTS.size(); index++) {
            int phase = index + 1;
            String transport = TRANSPORTS.get(index);
            if (phase == 1) {
                outputf(emit, "Phase %d/%d: initial chain (%s)%n", phase, RUN_PHASES, transport);
            } else {
                outputf(emit, "Phase %d/%d: respawn on %s%n", phase, RUN_PHASES, transport);
                long killStart = nowMillis();
                if (streams != null) {
                    streams.stop();
                }
                if (cascade != null) {
                    cascade.stop();
                }
                outputf(emit, "  killed 4 nodes in %s%n", elapsed(killStart));
            }
            long spawnStart = nowMillis();
            Cascade phaseCascade;
            try {
                phaseCascade = spawnCascade(phase, transport, specs, runRoot);
            } catch (Exception error) {
                totalFail += RUN_TICKS;
                outputf(emit, "  spawn FAIL: %s%n%n", error.getMessage());
                streams = null;
                continue;
            }
            outputf(emit, "  spawned 4 nodes in %s%n", elapsed(spawnStart));
            if (phase > 1) {
                output(emit, "  re-opening Follow:true streams on new A");
            }
            String streamError = null;
            try {
                streams = new LiveStreams(phaseCascade.roles.get("A").relayAddress);
                streams.start();
            } catch (Exception error) {
                streams = null;
                streamError = error.getMessage();
                outputf(emit, "  stream re-open failed: %s%n", error.getMessage());
            }
            double previousMetric = 0;
            for (int tick = 1; tick <= RUN_TICKS; tick++) {
                long tickStart = nowMillis();
                TickOutcome outcome = phaseCascade.runLiveTick(streams, streamError, tick, previousMetric);
                if (outcome.metric.pass) {
                    previousMetric = outcome.metricValue;
                }
                boolean overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
                if (overall) {
                    totalPass++;
                } else {
                    totalFail++;
                }
                outputf(emit,
                        "  Tick %d/%d: log %s, event %s, metric %s (overall %s in %s)%n",
                        tick,
                        RUN_TICKS,
                        passText(outcome.log.pass),
                        passText(outcome.event.pass),
                        passText(outcome.metric.pass),
                        passText(overall),
                        elapsed(tickStart));
                if (emit) {
                    printFailureEvidence("log", outcome.log);
                    printFailureEvidence("event", outcome.event);
                    printFailureEvidence("metric", outcome.metric);
                }
            }
            output(emit);
            cascade = phaseCascade;
        }
        if (streams != null) {
            streams.stop();
        }
        if (cascade != null) {
            cascade.stop();
        }
        outputf(emit, "Summary: %d PASS / %d FAIL across %d ticks%n", totalPass, totalFail, totalPass + totalFail);
        if (totalFail > 0) {
            throw new IllegalStateException(totalFail + " tick(s) failed");
        }
        return new CascadeReportData(
                totalPass + totalFail,
                totalPass,
                totalFail,
                List.of(new PhaseReportData("live-stream", totalPass, totalFail, List.of())));
    }

    private MultiPatternReportData runMultiPattern(boolean emit) throws Exception {
        Path javaBinary = findBinary(JAVA_SLUG);
        Path goBinary = findBinary(GO_SLUG);
        List<CascadePattern> patterns = List.of(
                new CascadePattern("java-java-java-java", allJavaSpecs(javaBinary)),
                new CascadePattern("java-java-go-java", Map.of(
                        "A", new RoleSpec(JAVA_SLUG, javaBinary),
                        "B", new RoleSpec(JAVA_SLUG, javaBinary),
                        "C", new RoleSpec(GO_SLUG, goBinary),
                        "D", new RoleSpec(JAVA_SLUG, javaBinary))),
                new CascadePattern("java-java-go-go", Map.of(
                        "A", new RoleSpec(JAVA_SLUG, javaBinary),
                        "B", new RoleSpec(JAVA_SLUG, javaBinary),
                        "C", new RoleSpec(GO_SLUG, goBinary),
                        "D", new RoleSpec(GO_SLUG, goBinary))));
        Path runRoot = Files.createTempDirectory("observability-cascade-java-multi-");
        output(emit, "=== observability-cascade-java (multi-pattern) ===");
        output(emit);
        int totalPass = 0;
        int totalFail = 0;
        for (int patternIndex = 0; patternIndex < patterns.size(); patternIndex++) {
            CascadePattern pattern = patterns.get(patternIndex);
            outputf(emit, "Pattern %d/%d: %s%n", patternIndex + 1, patterns.size(), pattern.name);
            int patternPass = 0;
            for (int index = 0; index < TRANSPORTS.size(); index++) {
                int phase = index + 1;
                String transport = TRANSPORTS.get(index);
                long started = nowMillis();
                Cascade cascade;
                try {
                    cascade = spawnCascade(phase, transport, pattern.roles, runRoot);
                } catch (Exception error) {
                    totalFail += RUN_TICKS;
                    outputf(emit, "  Phase %d/%d (%s): spawn FAIL (%s)%n", phase, RUN_PHASES, transport, error.getMessage());
                    continue;
                }
                String streamError = null;
                LiveStreams streams = null;
                try {
                    streams = new LiveStreams(cascade.roles.get("A").relayAddress);
                    streams.start();
                    LiveStreams openedStreams = streams;
                    CheckResult ready = waitFor(5000, () -> cascade.checkLiveEvent(openedStreams), 50);
                    if (!ready.pass) {
                        streamError = "live relay readiness: " + ready.evidence;
                    }
                } catch (Exception error) {
                    streamError = error.getMessage();
                }
                double previousMetric = 0;
                List<String> results = new ArrayList<>();
                List<String> evidence = new ArrayList<>();
                for (int tick = 1; tick <= RUN_TICKS; tick++) {
                    String sender = pattern.name + "-phase-" + phase + "-tick-" + tick;
                    TickOutcome outcome = cascade.runLiveTickWithSender(streams, streamError, sender, previousMetric);
                    if (outcome.metric.pass) {
                        previousMetric = outcome.metricValue;
                    }
                    boolean overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass;
                    if (overall) {
                        patternPass++;
                        totalPass++;
                        results.add("Tick " + tick + " PASS");
                    } else {
                        totalFail++;
                        results.add("Tick " + tick + " FAIL (" + failureSummary(outcome) + ")");
                        evidence.add("      Tick " + tick + " evidence: " + compactEvidence(outcome));
                    }
                }
                outputf(emit,
                        "  Phase %d/%d (%s): %s (spawned in %s)%n",
                        phase,
                        RUN_PHASES,
                        transport,
                        String.join(", ", results),
                        elapsed(started));
                if (emit) {
                    for (String line : evidence) {
                        System.out.println(line);
                    }
                }
                if (streams != null) {
                    streams.stop();
                }
                cascade.stop();
            }
            outputf(emit, "  Subtotal: %d/12 PASS%n%n", patternPass);
        }
        outputf(emit, "Summary: %d PASS / %d FAIL across %d ticks%n", totalPass, totalFail, totalPass + totalFail);
        if (totalFail > 0) {
            throw new IllegalStateException(totalFail + " tick(s) failed");
        }
        return new MultiPatternReportData(
                List.of(new CascadeReportData(
                        totalPass + totalFail,
                        totalPass,
                        totalFail,
                        List.of(new PhaseReportData("multi-pattern", totalPass, totalFail, List.of())))),
                totalPass,
                totalFail);
    }

    private Cascade spawnCascade(int phase, String transport, Map<String, RoleSpec> specs, Path runRoot) throws Exception {
        Map<String, RoleRuntime> roles = new LinkedHashMap<>();
        for (String role : ROLE_ORDER) {
            roles.put(role, newRoleRuntime(phase, transport, role, specs.get(role)));
        }
        for (RoleRuntime runtime : roles.values()) {
            Files.createDirectories(runRoot);
            deleteRecursively(runRoot.resolve(runtime.slug).resolve(runtime.uid));
        }
        Cascade cascade = new Cascade(phase, transport, runRoot, roles);
        for (String role : ROLE_ORDER) {
            RoleRuntime runtime = roles.get(role);
            String child = childRole(role);
            if (!child.isEmpty()) {
                runtime.memberAddress = roles.get(child).relayAddress;
                runtime.memberSlug = roles.get(child).slug;
            }
            startRole(cascade, runtime);
        }
        sleep(150);
        return cascade;
    }

    private RoleRuntime newRoleRuntime(int phase, String transport, String role, RoleSpec spec) throws IOException {
        Objects.requireNonNull(spec, "spec");
        String uid = "relay-p" + String.format("%02d", phase) + "-" + role.toLowerCase(Locale.ROOT);
        if ("tcp".equals(transport)) {
            return new RoleRuntime(role, uid, spec.slug, spec.binaryPath, "tcp://127.0.0.1:0", "", "");
        }
        if ("unix".equals(transport)) {
            Path socket = Path.of("/tmp/observability-cascade-java-p" + phase + "-" + role.toLowerCase(Locale.ROOT) + "-" + ProcessHandle.current().pid() + ".sock");
            Files.deleteIfExists(socket);
            String uri = "unix://" + socket;
            return new RoleRuntime(role, uid, spec.slug, spec.binaryPath, uri, uri, uri);
        }
        throw new IllegalArgumentException("unknown transport " + transport);
    }

    private void startRole(Cascade cascade, RoleRuntime runtime) throws Exception {
        List<String> args = new ArrayList<>();
        args.add(runtime.binaryPath.toString());
        args.add("serve");
        args.add("--listen");
        args.add(runtime.listenUri);
        if (!runtime.memberAddress.isEmpty()) {
            args.add("--member");
            args.add(runtime.memberSlug + "=" + runtime.memberAddress);
        }
        runtime.stderrPath = Files.createTempFile("observability-cascade-java-" + runtime.uid + "-", ".stderr");
        ProcessBuilder builder = new ProcessBuilder(args);
        builder.directory(workingDirectory().toFile());
        builder.redirectOutput(ProcessBuilder.Redirect.DISCARD);
        builder.redirectError(runtime.stderrPath.toFile());
        Map<String, String> env = builder.environment();
        env.put("OP_OBS", "logs,events,metrics,prom");
        env.put("OP_RUN_DIR", cascade.runRoot.toString());
        env.put("OP_INSTANCE_UID", runtime.uid);
        env.put("OP_ORGANISM_UID", cascade.roles.get("A").uid);
        env.put("OP_ORGANISM_SLUG", cascade.roles.get("A").slug);
        env.put("OP_PROM_ADDR", "127.0.0.1:0");
        runtime.process = builder.start();

        try {
            MetaJson meta = waitMeta(cascade.runRoot, runtime.slug, runtime.uid, 15000);
            runtime.metricsAddr = meta.metrics_addr;
            runtime.relayAddress = meta.address;
            runtime.channel = Connect.connect(
                    runtime.relayAddress,
                    new Connect.ConnectOptions(Duration.ofSeconds(5), "tcp", false, null));
            runtime.relayClient = RelayServiceGrpc.newBlockingStub(runtime.channel);
            dialReady(runtime.channel, 10000);
        } catch (Exception error) {
            String stderr = Files.exists(runtime.stderrPath) ? Files.readString(runtime.stderrPath) : "";
            throw new IllegalStateException("start " + runtime.role + ": " + (stderr.isBlank() ? error.getMessage() : stderr.trim()), error);
        }
    }

    private MetaJson waitMeta(Path runRoot, String slug, String uid, long timeoutMillis) throws Exception {
        Path metaPath = runRoot.resolve(slug).resolve(uid).resolve("meta.json");
        long deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis);
        Exception last = null;
        while (System.nanoTime() < deadline) {
            try {
                MetaJson meta = GSON.fromJson(Files.readString(metaPath), MetaJson.class);
                if (uid.equals(meta.uid) && meta.metrics_addr != null && !meta.metrics_addr.isBlank()) {
                    return meta;
                }
            } catch (Exception error) {
                last = error;
            }
            sleep(50);
        }
        throw new IllegalStateException("meta not ready for " + slug + "/" + uid + ": " + (last == null ? "timeout" : last.getMessage()));
    }

    private void dialReady(ManagedChannel channel, long timeoutMillis) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis);
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
        throw new IllegalStateException("dial readiness failed: " + (last == null ? "timeout" : last.getMessage()));
    }

    private Map<String, RoleSpec> allJavaSpecs(Path binary) {
        Map<String, RoleSpec> out = new LinkedHashMap<>();
        for (String role : ROLE_ORDER) {
            out.put(role, new RoleSpec(JAVA_SLUG, binary));
        }
        return out;
    }

    private Path findBinary(String slug) throws IOException, InterruptedException {
        if (JAVA_SLUG.equals(slug)) {
            return Composite.member("java-node");
        }

        List<Path> roots = new ArrayList<>();
        roots.add(Path.of(System.getenv().getOrDefault("OPBIN", Path.of(System.getProperty("user.home"), ".op", "bin").toString()), slug + ".holon", "bin"));
        for (Path root : roots) {
            Path found = findExecutable(root, slug);
            if (found != null) {
                return found;
            }
        }
        Process process = new ProcessBuilder("op", "--bin", slug)
                .directory(workingDirectory().toFile())
                .redirectError(ProcessBuilder.Redirect.DISCARD)
                .start();
        String out = new String(process.getInputStream().readAllBytes(), StandardCharsets.UTF_8).trim();
        if (process.waitFor() == 0 && !out.isBlank()) {
            return Path.of(out);
        }
        Path home = Path.of(System.getProperty("user.home"), ".op", "bin", slug + ".holon", "bin");
        Path found = findExecutable(home, slug);
        if (found != null) {
            return found;
        }
        throw new IllegalStateException(slug + " binary not found; run op build " + slug + " --install");
    }

    private Path findExecutable(Path root, String name) throws IOException {
        if (!Files.isDirectory(root)) {
            return null;
        }
        try (var stream = Files.list(root)) {
            for (Path path : stream.sorted().toList()) {
                if (Files.isDirectory(path)) {
                    Path nested = findExecutable(path, name);
                    if (nested != null) {
                        return nested;
                    }
                } else if (path.getFileName().toString().equals(name) && Files.isExecutable(path)) {
                    return path;
                }
            }
        }
        return null;
    }

    private static Path findRepoRoot(Path start) {
        Path current = start.toAbsolutePath().normalize();
        while (current != null) {
            if (Files.isDirectory(current.resolve("sdk")) && Files.isDirectory(current.resolve("examples"))) {
                return current;
            }
            current = current.getParent();
        }
        return null;
    }

    private Path workingDirectory() {
        return repoRoot != null ? repoRoot : Path.of(System.getProperty("user.dir")).toAbsolutePath().normalize();
    }

    private static String childRole(String role) {
        return switch (role) {
            case "A" -> "B";
            case "B" -> "C";
            case "C" -> "D";
            default -> "";
        };
    }

    private static List<holons.v1.Observability.LogEntry> readLogs(ManagedChannel channel) {
        List<holons.v1.Observability.LogEntry> out = new ArrayList<>();
        var iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.logsMethod(),
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                holons.v1.Observability.LogsRequest.newBuilder()
                        .setMinLevel(holons.v1.Observability.LogLevel.INFO)
                        .build());
        iterator.forEachRemaining(out::add);
        return out;
    }

    private static List<holons.v1.Observability.EventInfo> readEvents(ManagedChannel channel) {
        List<holons.v1.Observability.EventInfo> out = new ArrayList<>();
        var iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.eventsMethod(),
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                holons.v1.Observability.EventsRequest.getDefaultInstance());
        iterator.forEachRemaining(out::add);
        return out;
    }

    private static String fetchMetrics(String addr) throws Exception {
        HttpRequest request = HttpRequest.newBuilder(URI.create(addr))
                .timeout(Duration.ofSeconds(2))
                .GET()
                .build();
        return HTTP.send(request, HttpResponse.BodyHandlers.ofString()).body();
    }

    private static Double parseCascadeTicks(String body, String uid) {
        String needle = "responder_uid=\"" + uid + "\"";
        for (String line : body.split("\\R")) {
            if (!line.startsWith("cascade_ticks_total{") || !line.contains(needle)) {
                continue;
            }
            String[] parts = line.trim().split("\\s+");
            if (parts.length >= 2) {
                return Double.parseDouble(parts[parts.length - 1]);
            }
        }
        return null;
    }

    private static CheckResult waitFor(long timeoutMillis, CheckedSupplier<CheckResult> fn, long intervalMillis) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis);
        CheckResult last = new CheckResult(false, "");
        while (true) {
            last = fn.get();
            if (last.pass || System.nanoTime() > deadline) {
                return last;
            }
            sleep(intervalMillis);
        }
    }

    private static Service.CascadeReport toCascadeReport(CascadeReportData report) {
        Service.CascadeReport.Builder out = Service.CascadeReport.newBuilder()
                .setTicks(report.ticks())
                .setPass(report.pass())
                .setFail(report.fail());
        for (PhaseReportData phase : report.phases()) {
            out.addPhases(Service.PhaseResult.newBuilder()
                    .setName(phase.name())
                    .setPass(phase.pass())
                    .setFail(phase.fail())
                    .addAllFailures(phase.failures())
                    .build());
        }
        return out.build();
    }

    private static Service.MultiPatternReport toMultiPatternReport(MultiPatternReportData report) {
        Service.MultiPatternReport.Builder out = Service.MultiPatternReport.newBuilder()
                .setTotalPass(report.totalPass())
                .setTotalFail(report.totalFail());
        for (CascadeReportData pattern : report.patterns()) {
            out.addPatterns(toCascadeReport(pattern));
        }
        return out.build();
    }

    private static String normalizeListenUri(String listenUri) {
        Matcher matcher = Pattern.compile("^tcp://:(\\d+)$").matcher(listenUri == null ? "" : listenUri);
        if (matcher.matches()) {
            return "tcp://0.0.0.0:" + matcher.group(1);
        }
        return listenUri;
    }

    private static void output(boolean emit) {
        if (emit) {
            System.out.println();
        }
    }

    private static void output(boolean emit, String value) {
        if (emit) {
            System.out.println(value);
        }
    }

    private static void outputf(boolean emit, String format, Object... args) {
        if (emit) {
            System.out.printf(format, args);
        }
    }

    private static String canonicalCommand(String raw) {
        return raw == null ? "" : raw.trim().toLowerCase(Locale.ROOT).replace("-", "").replace("_", "").replace(" ", "");
    }

    private static long nowMillis() {
        return System.nanoTime() / 1_000_000;
    }

    private static String elapsed(long startedMillis) {
        long elapsed = Math.max(0, nowMillis() - startedMillis);
        if (elapsed < 1000) {
            return elapsed + "ms";
        }
        return String.format(Locale.ROOT, "%.1fs", elapsed / 1000.0);
    }

    private static String passText(boolean value) {
        return value ? "PASS" : "FAIL";
    }

    private static void printFailureEvidence(String family, CheckResult result) {
        if (!result.pass) {
            System.out.printf("    %s evidence: %s%n", family, result.evidence == null || result.evidence.isBlank() ? "<empty>" : result.evidence);
        }
    }

    private static String failureSummary(TickOutcome outcome) {
        List<String> missing = new ArrayList<>();
        if (!outcome.log.pass) {
            missing.add("log family");
        }
        if (!outcome.event.pass) {
            missing.add("event family");
        }
        if (!outcome.metric.pass) {
            missing.add("metric family");
        }
        return missing.isEmpty() ? "unknown" : String.join(", ", missing);
    }

    private static String compactEvidence(TickOutcome outcome) {
        List<String> parts = new ArrayList<>();
        if (!outcome.log.pass) {
            parts.add("log=" + outcome.log.evidence);
        }
        if (!outcome.event.pass) {
            parts.add("event=" + outcome.event.evidence);
        }
        if (!outcome.metric.pass) {
            parts.add("metric=" + outcome.metric.evidence);
        }
        return String.join(" | ", parts);
    }

    private static void sleep(long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException error) {
            Thread.currentThread().interrupt();
        }
    }

    private static void deleteRecursively(Path path) throws IOException {
        if (!Files.exists(path)) {
            return;
        }
        try (var stream = Files.walk(path)) {
            for (Path current : stream.sorted(Collections.reverseOrder()).toList()) {
                Files.deleteIfExists(current);
            }
        }
    }

    private interface CheckedSupplier<T> {
        T get() throws Exception;
    }

    private record RoleSpec(String slug, Path binaryPath) {
    }

    private record CascadePattern(String name, Map<String, RoleSpec> roles) {
    }

    private static final class RoleRuntime {
        final String role;
        final String uid;
        final String slug;
        final Path binaryPath;
        final String listenUri;
        String relayAddress;
        String clientTarget;
        String memberAddress = "";
        String memberSlug = "";
        String metricsAddr = "";
        Process process;
        Path stderrPath;
        ManagedChannel channel;
        RelayServiceGrpc.RelayServiceBlockingStub relayClient;

        RoleRuntime(String role, String uid, String slug, Path binaryPath, String listenUri, String relayAddress, String clientTarget) {
            this.role = role;
            this.uid = uid;
            this.slug = slug;
            this.binaryPath = binaryPath;
            this.listenUri = listenUri;
            this.relayAddress = relayAddress;
            this.clientTarget = clientTarget;
        }
    }

    private static final class CheckResult {
        final boolean pass;
        final String evidence;

        CheckResult(boolean pass, String evidence) {
            this.pass = pass;
            this.evidence = evidence;
        }
    }

    private static final class TickOutcome {
        final CheckResult log;
        final CheckResult event;
        final CheckResult metric;
        final double metricValue;

        TickOutcome(CheckResult log, CheckResult event, CheckResult metric, double metricValue) {
            this.log = log;
            this.event = event;
            this.metric = metric;
            this.metricValue = metricValue;
        }
    }

    private static final class Cascade {
        final int phase;
        final String transport;
        final Path runRoot;
        final Map<String, RoleRuntime> roles;

        Cascade(int phase, String transport, Path runRoot, Map<String, RoleRuntime> roles) {
            this.phase = phase;
            this.transport = transport;
            this.runRoot = runRoot;
            this.roles = roles;
        }

        TickOutcome runTick(int tick, double previousMetric) throws Exception {
            return runTickWithSender("phase-" + phase + "-tick-" + tick, previousMetric);
        }

        TickOutcome runTickWithSender(String sender, double previousMetric) throws Exception {
            Relay.TickRequest request = Relay.TickRequest.newBuilder()
                    .setSender(sender)
                    .setNote(transport)
                    .build();
            try {
                roles.get("D").relayClient.withDeadlineAfter(5, TimeUnit.SECONDS).tick(request);
            } catch (Exception error) {
                CheckResult failed = new CheckResult(false, error.getMessage());
                return new TickOutcome(failed, failed, failed, previousMetric);
            }
            CheckResult log = waitFor(3000, () -> checkLog(sender), 100);
            CheckResult event = waitFor(3000, this::checkEvent, 100);
            MetricCheck metricCheck = new MetricCheck(previousMetric);
            CheckResult metric = waitFor(3000, () -> metricCheck.check(this), 100);
            return new TickOutcome(log, event, metric, metricCheck.value);
        }

        TickOutcome runLiveTick(LiveStreams streams, String streamOpenError, int tick, double previousMetric) throws Exception {
            return runLiveTickWithSender(streams, streamOpenError, "phase-" + phase + "-tick-" + tick, previousMetric);
        }

        TickOutcome runLiveTickWithSender(LiveStreams streams, String streamOpenError, String sender, double previousMetric) throws Exception {
            Relay.TickRequest request = Relay.TickRequest.newBuilder()
                    .setSender(sender)
                    .setNote(transport)
                    .build();
            try {
                roles.get("D").relayClient.withDeadlineAfter(5, TimeUnit.SECONDS).tick(request);
            } catch (Exception error) {
                CheckResult failed = new CheckResult(false, error.getMessage());
                return new TickOutcome(failed, failed, failed, previousMetric);
            }

            CheckResult log;
            CheckResult event;
            if (streamOpenError == null && streams != null) {
                log = waitFor(1000, () -> checkLiveLog(streams, sender), 50);
                event = waitFor(1000, () -> checkLiveEvent(streams), 50);
            } else {
                String evidence = "stream re-open failed: " + (streamOpenError == null ? "streams not open" : streamOpenError);
                log = new CheckResult(false, evidence);
                event = new CheckResult(false, evidence);
            }

            MetricCheck metricCheck = new MetricCheck(previousMetric);
            CheckResult metric = waitFor(1000, () -> metricCheck.check(this), 50);
            return new TickOutcome(log, event, metric, metricCheck.value);
        }

        CheckResult checkLog(String sender) {
            List<holons.v1.Observability.LogEntry> entries = readLogs(roles.get("A").channel);
            for (holons.v1.Observability.LogEntry entry : entries) {
                if (!"tick received".equals(entry.getMessage())) {
                    continue;
                }
                if (!sender.equals(entry.getFieldsMap().getOrDefault("sender", ""))) {
                    continue;
                }
                if (!roles.get("D").uid.equals(entry.getFieldsMap().getOrDefault("responder_uid", ""))) {
                    continue;
                }
                String err = checkChain(entry.getChainList());
                if (!err.isEmpty()) {
                    return new CheckResult(false, "matching log has bad chain: " + err + " entry=" + entry);
                }
                return new CheckResult(true, entry.toString());
            }
            return new CheckResult(false, "no relayed D tick log for sender=" + sender + " in " + entries.size() + " A log entries");
        }

        CheckResult checkEvent() {
            List<holons.v1.Observability.EventInfo> events = readEvents(roles.get("A").channel);
            for (holons.v1.Observability.EventInfo event : events) {
                if (event.getTypeValue() != Observability.EventType.INSTANCE_READY.code
                        || !roles.get("D").uid.equals(event.getInstanceUid())) {
                    continue;
                }
                String err = checkChain(event.getChainList());
                if (!err.isEmpty()) {
                    return new CheckResult(false, "matching event has bad chain: " + err + " event=" + event);
                }
                return new CheckResult(true, event.toString());
            }
            return new CheckResult(false, "no relayed D INSTANCE_READY event in " + events.size() + " A events");
        }

        CheckResult checkLiveLog(LiveStreams streams, String sender) {
            List<holons.v1.Observability.LogEntry> entries = streams.logEntries();
            for (holons.v1.Observability.LogEntry entry : entries) {
                if (!"tick received".equals(entry.getMessage())) {
                    continue;
                }
                if (!sender.equals(entry.getFieldsMap().getOrDefault("sender", ""))) {
                    continue;
                }
                if (!roles.get("D").uid.equals(entry.getFieldsMap().getOrDefault("responder_uid", ""))) {
                    continue;
                }
                String err = checkChain(entry.getChainList());
                if (!err.isEmpty()) {
                    return new CheckResult(false, "matching live log has bad chain: " + err + " entry=" + entry);
                }
                return new CheckResult(true, entry.toString());
            }
            return new CheckResult(false, "no live log found for sender=" + sender + "; buffer=" + entries.size() + " errors=" + streams.errors());
        }

        CheckResult checkLiveEvent(LiveStreams streams) {
            List<holons.v1.Observability.EventInfo> events = streams.eventEntries();
            for (holons.v1.Observability.EventInfo event : events) {
                if (event.getTypeValue() != Observability.EventType.INSTANCE_READY.code
                        || !roles.get("D").uid.equals(event.getInstanceUid())) {
                    continue;
                }
                String err = checkChain(event.getChainList());
                if (!err.isEmpty()) {
                    return new CheckResult(false, "matching live event has bad chain: " + err + " event=" + event);
                }
                return new CheckResult(true, event.toString());
            }
            return new CheckResult(false, "no live INSTANCE_READY event for D; buffer=" + events.size() + " errors=" + streams.errors());
        }

        CheckResult checkMetric(MetricCheck metricCheck) throws Exception {
            String body = fetchMetrics(roles.get("D").metricsAddr);
            Double value = parseCascadeTicks(body, roles.get("D").uid);
            if (value == null) {
                return new CheckResult(false, body);
            }
            metricCheck.value = value;
            if (value <= metricCheck.previous) {
                return new CheckResult(false, "cascade_ticks_total=" + value + " did not increase beyond " + metricCheck.previous + "\n" + body);
            }
            return new CheckResult(true, "cascade_ticks_total=" + value);
        }

        String checkChain(List<holons.v1.Observability.ChainHop> chain) {
            List<String> expected = List.of("D", "C", "B");
            for (int index = 0; index < expected.size(); index++) {
                if (index >= chain.size()) {
                    return "chain length " + chain.size() + " < 3";
                }
                String role = expected.get(index);
                RoleRuntime want = roles.get(role);
                holons.v1.Observability.ChainHop hop = chain.get(index);
                if (!want.slug.equals(hop.getSlug()) || !want.uid.equals(hop.getInstanceUid())) {
                    return "hop " + index + " = " + hop.getSlug() + "/" + hop.getInstanceUid()
                            + ", want " + want.slug + "/" + want.uid;
                }
            }
            return "";
        }

        void stop() {
            List<String> reversed = new ArrayList<>(ROLE_ORDER);
            Collections.reverse(reversed);
            for (String role : reversed) {
                RoleRuntime runtime = roles.get(role);
                if (runtime.channel != null) {
                    Connect.disconnect(runtime.channel);
                }
                if (runtime.process != null && runtime.process.isAlive()) {
                    runtime.process.destroy();
                }
            }
            for (String role : reversed) {
                RoleRuntime runtime = roles.get(role);
                if (runtime.process == null) {
                    continue;
                }
                try {
                    if (!runtime.process.waitFor(2, TimeUnit.SECONDS)) {
                        runtime.process.destroyForcibly();
                        runtime.process.waitFor(2, TimeUnit.SECONDS);
                    }
                } catch (InterruptedException error) {
                    Thread.currentThread().interrupt();
                }
            }
        }
    }

    private static final class MetricCheck {
        final double previous;
        double value;

        MetricCheck(double previous) {
            this.previous = previous;
            this.value = previous;
        }

        CheckResult check(Cascade cascade) throws Exception {
            return cascade.checkMetric(this);
        }
    }

    private static final class LiveStreams {
        private final String address;
        private ManagedChannel channel;
        private final List<holons.v1.Observability.LogEntry> logs = Collections.synchronizedList(new ArrayList<>());
        private final List<holons.v1.Observability.EventInfo> events = Collections.synchronizedList(new ArrayList<>());
        private final List<String> errors = Collections.synchronizedList(new ArrayList<>());
        private final List<Thread> threads = new ArrayList<>();

        LiveStreams(String address) {
            this.address = address;
        }

        void start() throws Exception {
            channel = Connect.connect(address, new Connect.ConnectOptions(Duration.ofSeconds(5), "tcp", false, null));
            Thread logThread = new Thread(this::readLogStream, "observability-cascade-java-live-logs");
            Thread eventThread = new Thread(this::readEventStream, "observability-cascade-java-live-events");
            logThread.setDaemon(true);
            eventThread.setDaemon(true);
            threads.add(logThread);
            threads.add(eventThread);
            logThread.start();
            eventThread.start();
        }

        void stop() {
            if (channel != null) {
                Connect.disconnect(channel);
            }
            for (Thread thread : threads) {
                thread.interrupt();
            }
        }

        List<holons.v1.Observability.LogEntry> logEntries() {
            synchronized (logs) {
                return List.copyOf(logs);
            }
        }

        List<holons.v1.Observability.EventInfo> eventEntries() {
            synchronized (events) {
                return List.copyOf(events);
            }
        }

        List<String> errors() {
            synchronized (errors) {
                return List.copyOf(errors);
            }
        }

        private void readLogStream() {
            try {
                var iterator = ClientCalls.blockingServerStreamingCall(
                        channel,
                        Observability.logsMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Observability.LogsRequest.newBuilder()
                                .setMinLevel(holons.v1.Observability.LogLevel.INFO)
                                .setFollow(true)
                                .build());
                while (!Thread.currentThread().isInterrupted() && iterator.hasNext()) {
                    logs.add(iterator.next());
                }
            } catch (Exception error) {
                errors.add("logs stream ended: " + error.getMessage());
            }
        }

        private void readEventStream() {
            try {
                var iterator = ClientCalls.blockingServerStreamingCall(
                        channel,
                        Observability.eventsMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Observability.EventsRequest.newBuilder()
                                .setFollow(true)
                                .build());
                while (!Thread.currentThread().isInterrupted() && iterator.hasNext()) {
                    events.add(iterator.next());
                }
            } catch (Exception error) {
                errors.add("events stream ended: " + error.getMessage());
            }
        }
    }

    private record PhaseReportData(String name, int pass, int fail, List<String> failures) {
    }

    private record CascadeReportData(int ticks, int pass, int fail, List<PhaseReportData> phases) {
    }

    private record MultiPatternReportData(List<CascadeReportData> patterns, int totalPass, int totalFail) {
    }

    private static final class MetaJson {
        String uid = "";
        String address = "";
        String metrics_addr = "";
    }
}

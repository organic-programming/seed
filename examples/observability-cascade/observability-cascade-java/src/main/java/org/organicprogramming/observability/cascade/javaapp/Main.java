package org.organicprogramming.observability.cascade.javaapp;

import io.grpc.stub.StreamObserver;
import observability_cascade.v1.ObservabilityCascadeServiceGrpc;
import observability_cascade.v1.Service;
import org.organicprogramming.holons.Composite;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Observability;
import org.organicprogramming.holons.Serve;
import relay.v1.Relay;
import relay.v1.RelayServiceGrpc;

import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public final class Main {
    private static final String JAVA_SLUG = "observability-cascade-java-node";
    private static final String GO_SLUG = "observability-cascade-go-node";
    private static final int RUN_TICKS = 3;

    private Main() {
    }

    public static void main(String[] args) {
        try {
            if (args.length > 0 && "serve".equals(canonicalCommand(args[0]))) {
                serveComposite(Arrays.copyOfRange(args, 1, args.length));
                return;
            }

            boolean multi = Arrays.asList(args).contains("--multi-pattern");
            boolean live = Arrays.asList(args).contains("--live-stream");
            int failed;
            if (multi) {
                Service.MultiPatternReport report = runMultiPatternReport(true);
                failed = report.getTotalFail();
            } else {
                Service.CascadeReport report = runReport(live ? "live-stream" : "default", ownLanguageMembers(), live, true);
                failed = report.getFail();
            }
            if (failed > 0) {
                System.exit(1);
            }
        } catch (Exception error) {
            System.err.println("FAIL: " + error.getMessage());
            System.exit(1);
        }
    }

    private static void serveComposite(String[] args) throws Exception {
        Describe.useStaticResponse(gen.describe_generated.StaticDescribeResponse());
        Serve.ParsedFlags parsed = Serve.parseOptions(args);
        Serve.runWithOptions(
                normalizeListenUri(parsed.listenUri()),
                List.of(new CascadeService()),
                new Serve.Options()
                        .withReflect(parsed.reflect())
                        .withSlug("observability-cascade-java"));
    }

    private static final class CascadeService extends ObservabilityCascadeServiceGrpc.ObservabilityCascadeServiceImplBase {
        @Override
        public void runDefault(Service.RunRequest request, StreamObserver<Service.CascadeReport> responseObserver) {
            responseObserver.onNext(runReport("default", ownLanguageMembers(), false, false));
            responseObserver.onCompleted();
        }

        @Override
        public void runLiveStream(Service.RunRequest request, StreamObserver<Service.CascadeReport> responseObserver) {
            responseObserver.onNext(runReport("live-stream", ownLanguageMembers(), true, false));
            responseObserver.onCompleted();
        }

        @Override
        public void runMultiPattern(Service.RunRequest request, StreamObserver<Service.MultiPatternReport> responseObserver) {
            responseObserver.onNext(runMultiPatternReport(false));
            responseObserver.onCompleted();
        }
    }

    private static Service.MultiPatternReport runMultiPatternReport(boolean emit) {
        long totalStart = System.nanoTime();
        List<NamedPattern> patterns = javaPatterns();
        Service.MultiPatternReport.Builder out = Service.MultiPatternReport.newBuilder();
        if (emit) {
            System.out.println("=== observability-cascade-java --multi-pattern ===");
            System.out.println();
        }
        for (int index = 0; index < patterns.size(); index++) {
            NamedPattern pattern = patterns.get(index);
            if (emit) {
                System.out.printf("Pattern %d/%d: %s%n", index + 1, patterns.size(), pattern.name());
            }
            Service.CascadeReport report = runReport(pattern.name(), pattern.members(), true, emit);
            out.addPatterns(report)
                    .setTotalPass(out.getTotalPass() + report.getPass())
                    .setTotalFail(out.getTotalFail() + report.getFail());
            if (emit) {
                String status = report.getFail() == 0 ? "PASS" : "FAIL";
                System.out.printf("Pattern %s: %d/%d %s (elapsed=%s)%n%n",
                        pattern.name(), report.getPass(), report.getTicks(), status, elapsedText(report.getElapsedUs()));
            }
        }
        out.setTotalElapsedUs(elapsedUs(totalStart));
        if (emit) {
            System.out.printf("Summary: %d PASS / %d FAIL across %d ticks (total elapsed=%s)%n",
                    out.getTotalPass(),
                    out.getTotalFail(),
                    out.getTotalPass() + out.getTotalFail(),
                    elapsedText(out.getTotalElapsedUs()));
        }
        return out.build();
    }

    private static Service.CascadeReport runReport(String name, List<LanguageMember> members, boolean live, boolean emit) {
        ensureCascadeObservability();
        long reportStart = System.nanoTime();
        Service.CascadeReport.Builder report = Service.CascadeReport.newBuilder().setName(name);
        Duration timeout = live ? Duration.ofSeconds(1) : Duration.ofSeconds(3);
        Duration poll = live ? Duration.ofMillis(50) : Duration.ofMillis(100);
        if (emit) {
            System.out.printf("=== observability-cascade-java %s===%n%n", modeSuffix(name));
        }

        for (int phaseIndex = 0; phaseIndex < Composite.TRANSPORT_COVERAGE_SEQUENCE.size(); phaseIndex++) {
            long phaseStart = System.nanoTime();
            String transport = Composite.TRANSPORT_COVERAGE_SEQUENCE.get(phaseIndex);
            String from = phaseIndex == 0 ? transport : Composite.TRANSPORT_COVERAGE_SEQUENCE.get(phaseIndex - 1);
            Service.PhaseResult.Builder phase = Service.PhaseResult.newBuilder()
                    .setName(String.format(Locale.ROOT, "%02d-%s→%s", phaseIndex + 1, from, transport));
            if (emit) {
                System.out.printf("Phase %d/%d: %s%n",
                        phaseIndex + 1, Composite.TRANSPORT_COVERAGE_SEQUENCE.size(), phase.getName());
            }

            Composite.Cascade cascade = null;
            try {
                Composite.CascadeOptions opts = new Composite.CascadeOptions();
                opts.transport = transport;
                opts.members = childSpecs(members);
                opts.extraEnv = Map.of(
                        "OP_OBS", "logs,events,metrics,prom",
                        "OP_PROM_ADDR", "127.0.0.1:0");
                cascade = Composite.buildCascade(opts);
            } catch (Exception error) {
                phase.setFail(RUN_TICKS);
                for (int tick = 1; tick <= RUN_TICKS; tick++) {
                    phase.addFailures("tick=" + tick + " log=spawn event=spawn hops=" + compactEvidence(error.getMessage()));
                }
                finishPhase(report, phase, phaseStart, emit);
                continue;
            }

            Map<String, Long> previous = new LinkedHashMap<>();
            try {
                for (int tick = 1; tick <= RUN_TICKS; tick++) {
                    String sender = String.format(Locale.ROOT, "%s-phase-%02d-tick-%d", name, phaseIndex + 1, tick);
                    TickResult result = runTick(cascade, sender, transport, members, previous, timeout, poll, live);
                    if (result.pass()) {
                        phase.setPass(phase.getPass() + 1);
                    } else {
                        phase.setFail(phase.getFail() + 1);
                        phase.addFailures(result.evidenceLine(tick));
                    }
                    if (emit) {
                        System.out.printf("  Tick %d/%d: %s%n", tick, RUN_TICKS, result.pass() ? "PASS" : "FAIL");
                        if (!result.pass()) {
                            System.err.println("    " + result.evidenceLine(tick));
                        }
                    }
                }
            } finally {
                cascade.stop();
            }
            finishPhase(report, phase, phaseStart, emit);
        }

        report.setElapsedUs(elapsedUs(reportStart));
        if (emit) {
            System.out.printf("%nSummary: %d ticks, %d PASS, %d FAIL (total elapsed=%s)%n",
                    report.getTicks(), report.getPass(), report.getFail(), elapsedText(report.getElapsedUs()));
        }
        return report.build();
    }

    private static void finishPhase(
            Service.CascadeReport.Builder report,
            Service.PhaseResult.Builder phase,
            long phaseStart,
            boolean emit) {
        phase.setElapsedUs(elapsedUs(phaseStart));
        Service.PhaseResult built = phase.build();
        report.addPhases(built)
                .setPass(report.getPass() + built.getPass())
                .setFail(report.getFail() + built.getFail())
                .setTicks(report.getTicks() + built.getPass() + built.getFail());
        if (emit) {
            String status = built.getFail() == 0 ? "PASS" : "FAIL";
            System.out.printf("Phase %s: %d/%d %s (elapsed=%s)%n",
                    built.getName(), built.getPass(), built.getPass() + built.getFail(), status, elapsedText(built.getElapsedUs()));
        }
    }

    private static TickResult runTick(
            Composite.Cascade cascade,
            String sender,
            String note,
            List<LanguageMember> members,
            Map<String, Long> previous,
            Duration timeout,
            Duration poll,
            boolean live) {
        Relay.TickResponse response;
        try {
            response = RelayServiceGrpc.newBlockingStub(cascade.top.conn)
                    .withDeadlineAfter(5, TimeUnit.SECONDS)
                    .tick(Relay.TickRequest.newBuilder().setSender(sender).setNote(note).build());
        } catch (Exception error) {
            Composite.CheckOutcome failed = new Composite.CheckOutcome(false, compactEvidence(error.getMessage()));
            return new TickResult(false, failed, failed, failed);
        }

        Composite.CheckOutcome hops = checkHops(response.getHopsList(), members, previous);
        if (!hops.pass()) {
            return new TickResult(false, new Composite.CheckOutcome(false, "skipped"), new Composite.CheckOutcome(false, "skipped"), hops);
        }
        List<Composite.ChainHop> expected = hopChain(response.getHopsList());
        String leafUid = response.getHopsList().get(0).getUid();

        Composite.LogCheckOptions logOptions = new Composite.LogCheckOptions();
        logOptions.sender = sender;
        logOptions.leafUid = leafUid;
        logOptions.expectedChain = expected;
        logOptions.timeout = timeout;
        logOptions.pollInterval = poll;
        logOptions.live = live;
        Composite.CheckOutcome log = Composite.checkRelayedLog(logOptions);

        Composite.EventCheckOptions eventOptions = new Composite.EventCheckOptions();
        eventOptions.eventName = Observability.EVENT_INSTANCE_READY;
        eventOptions.leafUid = leafUid;
        eventOptions.expectedChain = expected;
        eventOptions.timeout = timeout;
        eventOptions.pollInterval = poll;
        eventOptions.live = live;
        Composite.CheckOutcome event = Composite.checkRelayedEvent(eventOptions);

        return new TickResult(hops.pass() && log.pass() && event.pass(), log, event, hops);
    }

    private static Composite.CheckOutcome checkHops(
            List<Relay.HopReceipt> hops,
            List<LanguageMember> members,
            Map<String, Long> previous) {
        if (hops.size() != members.size()) {
            return new Composite.CheckOutcome(false, "hops length " + hops.size() + " want " + members.size());
        }
        for (int index = 0; index < hops.size(); index++) {
            Relay.HopReceipt hop = hops.get(index);
            LanguageMember want = members.get(members.size() - 1 - index);
            if (!want.slug().equals(hop.getSlug())) {
                return new Composite.CheckOutcome(false, "hop " + index + " slug=" + hop.getSlug() + " want " + want.slug());
            }
            if (hop.getUid().isBlank()) {
                return new Composite.CheckOutcome(false, "hop " + index + " uid empty");
            }
            long last = previous.getOrDefault(hop.getUid(), 0L);
            if (hop.getReceived() <= last) {
                return new Composite.CheckOutcome(false,
                        "hop " + index + " received=" + hop.getReceived() + " previous=" + last);
            }
            previous.put(hop.getUid(), hop.getReceived());
        }
        return new Composite.CheckOutcome(true, "");
    }

    private static List<Composite.ChainHop> hopChain(List<Relay.HopReceipt> hops) {
        List<Composite.ChainHop> out = new ArrayList<>();
        for (Relay.HopReceipt hop : hops) {
            out.add(new Composite.ChainHop(hop.getSlug(), hop.getUid()));
        }
        return out;
    }

    private static List<LanguageMember> ownLanguageMembers() {
        String binary = memberPath("java-node");
        return List.of(
                new LanguageMember("java", JAVA_SLUG, binary),
                new LanguageMember("java", JAVA_SLUG, binary),
                new LanguageMember("java", JAVA_SLUG, binary));
    }

    private static List<NamedPattern> javaPatterns() {
        Map<String, LanguageMember> bins = Map.of(
                "java", new LanguageMember("java", JAVA_SLUG, memberPath("java-node")),
                "go", new LanguageMember("go", GO_SLUG, memberPath("go-node")));
        String[] names = {
                "java-java-java", "java-java-go", "java-go-java", "java-go-go",
                "go-java-java", "go-java-go", "go-go-java", "go-go-go"
        };
        List<NamedPattern> out = new ArrayList<>();
        for (String name : names) {
            String[] parts = name.split("-");
            out.add(new NamedPattern(name, List.of(bins.get(parts[0]), bins.get(parts[1]), bins.get(parts[2]))));
        }
        return out;
    }

    private static List<Composite.ChildSpec> childSpecs(List<LanguageMember> members) {
        List<Composite.ChildSpec> out = new ArrayList<>();
        for (LanguageMember member : members) {
            out.add(new Composite.ChildSpec(member.slug(), member.binary()));
        }
        return out;
    }

    private static String memberPath(String id) {
        try {
            return Composite.member(id).toString();
        } catch (Exception error) {
            return "";
        }
    }

    private static void ensureCascadeObservability() {
        Observability obs = Observability.current();
        if (obs.enabled(Observability.Family.LOGS) && obs.enabled(Observability.Family.EVENTS)) {
            return;
        }
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "observability-cascade-java";
        cfg.instanceUid = "java-composite-" + ProcessHandle.current().pid();
        Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs,events,metrics,prom"));
    }

    private static long elapsedUs(long startedNanos) {
        return Math.max(1, (System.nanoTime() - startedNanos) / 1000);
    }

    private static String elapsedText(long elapsedUs) {
        Duration duration = Duration.ofNanos(elapsedUs * 1000);
        if (duration.compareTo(Duration.ofSeconds(1)) < 0) {
            return duration.toMillis() + "ms";
        }
        if (duration.compareTo(Duration.ofMinutes(1)) < 0) {
            return String.format(Locale.ROOT, "%.2fs", duration.toNanos() / 1_000_000_000.0);
        }
        return String.format(Locale.ROOT, "%.1fm", duration.toSeconds() / 60.0);
    }

    private static String modeSuffix(String name) {
        return "default".equals(name) ? "" : "--" + name + " ";
    }

    private static String compactEvidence(String value) {
        String compact = String.join(" ", String.valueOf(value == null ? "" : value).split("\\s+")).trim();
        if (compact.isEmpty()) {
            return "<empty>";
        }
        if (compact.length() <= 240) {
            return compact;
        }
        return compact.substring(0, 240) + "...";
    }

    private static String normalizeListenUri(String listenUri) {
        Matcher matcher = Pattern.compile("^tcp://:(\\d+)$").matcher(listenUri == null ? "" : listenUri);
        if (matcher.matches()) {
            return "tcp://0.0.0.0:" + matcher.group(1);
        }
        return listenUri;
    }

    private static String canonicalCommand(String raw) {
        return raw == null ? "" : raw.trim().toLowerCase(Locale.ROOT).replace("-", "").replace("_", "").replace(" ", "");
    }

    private record LanguageMember(String lang, String slug, String binary) {
    }

    private record NamedPattern(String name, List<LanguageMember> members) {
    }

    private record TickResult(
            boolean pass,
            Composite.CheckOutcome log,
            Composite.CheckOutcome event,
            Composite.CheckOutcome hops) {
        String evidenceLine(int tick) {
            return "tick=" + tick
                    + " log=" + evidenceText(log)
                    + " event=" + evidenceText(event)
                    + " hops=" + evidenceText(hops);
        }

        private static String evidenceText(Composite.CheckOutcome outcome) {
            return outcome.pass() ? "ok" : compactEvidence(outcome.evidence());
        }
    }
}

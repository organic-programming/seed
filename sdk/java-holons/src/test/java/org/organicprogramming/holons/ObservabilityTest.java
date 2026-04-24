package org.organicprogramming.holons;

import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.ClientCalls;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

final class ObservabilityTest {
    @Test
    void parseOpObsDropsV2Tokens() {
        Set<Observability.Family> all = Set.of(
                Observability.Family.LOGS,
                Observability.Family.METRICS,
                Observability.Family.EVENTS,
                Observability.Family.PROM);
        assertEquals(all, Observability.parseOpObs("all,otel"));
        assertEquals(all, Observability.parseOpObs("all,sessions"));
    }

    @Test
    void checkEnvRejectsV2TokensAndOpSessions() {
        assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_OBS", "logs,otel")));
        assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_OBS", "logs,sessions")));
        Observability.InvalidTokenException err = assertThrows(Observability.InvalidTokenException.class,
                () -> Observability.checkEnv(Map.of("OP_SESSIONS", "metrics")));
        assertEquals("OP_SESSIONS", err.variable);
        assertDoesNotThrow(() -> Observability.checkEnv(Map.of("OP_OBS", "logs,metrics,events,prom,all")));
    }

    @Test
    void runDirDerivesFromRegistryRoot(@TempDir Path tmp) {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "gabriel-greeting-java";
        cfg.instanceUid = "uid-1";
        cfg.runDir = tmp.toString();

        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs"));

        assertEquals(tmp.resolve("gabriel-greeting-java").resolve("uid-1").toString(), obs.cfg.runDir);
        Observability.reset();
    }

    @Test
    void serviceReplaysLogsMetricsEventsAndDiskFiles(@TempDir Path tmp) throws Exception {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "gabriel-greeting-java";
        cfg.instanceUid = "uid-2";
        cfg.runDir = tmp.toString();
        Observability obs = Observability.configureFromEnv(
                cfg,
                Map.of("OP_OBS", "logs,metrics,events"));

        Observability.enableDiskWriters(obs.cfg.runDir);
        obs.logger("test").info("service-log", Map.of("component", "java"));
        obs.counter("java_requests_total", "", Map.of()).inc();
        obs.emit(Observability.EventType.INSTANCE_READY, Map.of("listener", "tcp://127.0.0.1:1"));
        Observability.MetaJson meta = new Observability.MetaJson();
        meta.slug = obs.cfg.slug;
        meta.uid = obs.cfg.instanceUid;
        meta.pid = 123;
        meta.transport = "tcp";
        meta.address = "tcp://127.0.0.1:1";
        meta.logPath = Path.of(obs.cfg.runDir, "stdout.log").toString();
        Observability.writeMetaJson(obs.cfg.runDir, meta);

        Path runDir = tmp.resolve("gabriel-greeting-java").resolve("uid-2");
        assertTrue(Files.readString(runDir.resolve("stdout.log")).contains("\"message\":\"service-log\""));
        assertTrue(Files.readString(runDir.resolve("events.jsonl")).contains("\"type\":\"INSTANCE_READY\""));
        assertTrue(Files.readString(runDir.resolve("meta.json")).contains("\"uid\":\"uid-2\""));

        Server server = ServerBuilder.forPort(0)
                .addService(Observability.service(obs))
                .build()
                .start();
        ManagedChannel channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .build();
        try {
            List<holons.v1.Observability.LogEntry> logs = new ArrayList<>();
            Iterator<holons.v1.Observability.LogEntry> logIterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.logsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.LogsRequest.newBuilder()
                            .setMinLevel(holons.v1.Observability.LogLevel.INFO)
                            .build());
            logIterator.forEachRemaining(logs::add);
            assertTrue(logs.stream().anyMatch(entry -> entry.getMessage().equals("service-log")));

            holons.v1.Observability.MetricsSnapshot metrics = ClientCalls.blockingUnaryCall(
                    channel,
                    Observability.metricsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.MetricsRequest.getDefaultInstance());
            assertTrue(metrics.getSamplesList().stream()
                    .anyMatch(sample -> sample.getName().equals("java_requests_total")));
            assertFalse(metrics.hasSessionRollup());

            List<holons.v1.Observability.EventInfo> events = new ArrayList<>();
            Iterator<holons.v1.Observability.EventInfo> eventIterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.EventsRequest.getDefaultInstance());
            eventIterator.forEachRemaining(events::add);
            assertTrue(events.stream()
                    .anyMatch(event -> event.getType() == holons.v1.Observability.EventType.INSTANCE_READY));
        } finally {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
            Observability.reset();
        }
    }
}

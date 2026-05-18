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
    void parseOpObsRejectsV2Tokens() {
        Set<Observability.Family> all = Set.of(
                Observability.Family.LOGS,
                Observability.Family.METRICS,
                Observability.Family.EVENTS,
                Observability.Family.PROM);
        assertEquals(all, Observability.parseOpObs("all"));
        assertThrows(Observability.InvalidTokenException.class, () -> Observability.parseOpObs("all,otel"));
        assertThrows(Observability.InvalidTokenException.class, () -> Observability.parseOpObs("all,sessions"));
        assertThrows(Observability.InvalidTokenException.class, () -> Observability.parseOpObs("unknown"));
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
        obs.emit(Observability.EVENT_INSTANCE_READY, Map.of("listener", "tcp://127.0.0.1:1"));
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
        assertTrue(Files.readString(runDir.resolve("events.jsonl")).contains("\"event_name\":\"instance.ready\""));
        assertTrue(Files.readString(runDir.resolve("meta.json")).contains("\"uid\":\"uid-2\""));

        Server server = ServerBuilder.forPort(0)
                .addService(Observability.service(obs))
                .build()
                .start();
        ManagedChannel channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .build();
        try {
            List<holons.v1.Observability.LogRecord> logs = new ArrayList<>();
            Iterator<holons.v1.Observability.LogRecord> logIterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.logsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.LogsRequest.newBuilder()
                            .setMinSeverityNumber(holons.v1.Observability.SeverityNumber.SEVERITY_NUMBER_INFO)
                            .build());
            logIterator.forEachRemaining(logs::add);
            assertTrue(logs.stream().anyMatch(entry -> entry.getBody().getStringValue().equals("service-log")));

            List<holons.v1.Observability.Metric> metrics = new ArrayList<>();
            Iterator<holons.v1.Observability.Metric> metricIterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.metricsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.MetricsRequest.getDefaultInstance());
            metricIterator.forEachRemaining(metrics::add);
            assertTrue(metrics.stream().anyMatch(metric -> metric.getName().equals("java_requests_total")
                    && metric.hasSum()
                    && metric.getSum().getIsMonotonic()
                    && metric.getSum().getAggregationTemporality()
                    == holons.v1.Observability.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE));

            List<holons.v1.Observability.LogRecord> events = new ArrayList<>();
            Iterator<holons.v1.Observability.LogRecord> eventIterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod(),
                    CallOptions.DEFAULT,
                    holons.v1.Observability.EventsRequest.getDefaultInstance());
            eventIterator.forEachRemaining(events::add);
            assertTrue(events.stream()
                    .anyMatch(event -> event.getEventName().equals(Observability.EVENT_INSTANCE_READY)));
        } finally {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
            Observability.reset();
        }
    }

    @Test
    void prometheusTextInjectsIdentityLabelsAndRoundTripsProto() {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "cascade-node-java";
        cfg.instanceUid = "java-prom-1";
        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs,metrics,events,prom"));

        obs.counter("cascade_ticks_total", "Ticks received by this cascade node.", Map.of("responder_uid", "java-prom-1")).inc();
        String text = Observability.toPrometheusText(obs);
        assertTrue(text.contains("# HELP cascade_ticks_total Ticks received by this cascade node."));
        assertTrue(text.contains("# TYPE cascade_ticks_total counter"));
        assertTrue(text.contains("instance_uid=\"java-prom-1\""));
        assertTrue(text.contains("responder_uid=\"java-prom-1\""));
        assertTrue(text.contains("slug=\"cascade-node-java\""));

        Observability.LogRecord log = new Observability.LogRecord(holons.v1.Observability.LogRecord.newBuilder()
                .setBody(Observability.anyValue("tick received"))
                .addChain("leaf")
                .addAttributes(holons.v1.Observability.KeyValue.newBuilder()
                        .setKey("sender")
                        .setValue(Observability.anyValue("test")))
                .build());
        Observability.LogRecord roundTrip = Observability.fromProtoLogRecord(Observability.toProtoLogRecord(log));
        assertEquals("tick received", roundTrip.bodyString());
        assertEquals("leaf", roundTrip.record.getChain(0));
        Observability.reset();
    }

    @Test
    void testLogsFollowReplaysRingOnSubscribe() throws Exception {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "java-log-replay";
        cfg.instanceUid = "log-replay-1";
        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs"));
        obs.logger("test").info("before-subscribe", Map.of());

        Server server = ServerBuilder.forPort(0)
                .addService(Observability.service(obs))
                .build()
                .start();
        ManagedChannel channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .build();
        try {
            Iterator<holons.v1.Observability.LogRecord> iterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.logsMethod(),
                    CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                    holons.v1.Observability.LogsRequest.newBuilder()
                            .setFollow(true)
                            .setMinSeverityNumber(holons.v1.Observability.SeverityNumber.SEVERITY_NUMBER_INFO)
                            .build());

            assertTrue(iterator.hasNext());
            assertEquals("before-subscribe", iterator.next().getBody().getStringValue());
            obs.logger("test").info("after-subscribe", Map.of());
            assertTrue(iterator.hasNext());
            assertEquals("after-subscribe", iterator.next().getBody().getStringValue());
        } finally {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
            Observability.reset();
        }
    }

    @Test
    void testEventsFollowReplaysRingOnSubscribe() throws Exception {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "java-event-replay";
        cfg.instanceUid = "event-replay-1";
        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "events"));
        obs.emit(Observability.EVENT_INSTANCE_READY, Map.of("listener", "tcp://127.0.0.1:1"));

        Server server = ServerBuilder.forPort(0)
                .addService(Observability.service(obs))
                .build()
                .start();
        ManagedChannel channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.getPort())
                .usePlaintext()
                .build();
        try {
            Iterator<holons.v1.Observability.LogRecord> iterator = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod(),
                    CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                    holons.v1.Observability.EventsRequest.newBuilder()
                            .setFollow(true)
                            .build());

            assertTrue(iterator.hasNext());
            assertEquals(Observability.EVENT_INSTANCE_READY, iterator.next().getEventName());
            obs.emit(Observability.EVENT_INSTANCE_EXITED, Map.of("listener", "tcp://127.0.0.1:1"));
            assertTrue(iterator.hasNext());
            assertEquals(Observability.EVENT_INSTANCE_EXITED, iterator.next().getEventName());
        } finally {
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
            Observability.reset();
        }
    }
}

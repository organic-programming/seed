package org.organicprogramming.holons;

import io.grpc.BindableService;
import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.ServerServiceDefinition;
import io.grpc.stub.ClientCalls;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.function.BooleanSupplier;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ServeTest {

    @Test
    void startWithOptionsServesDescribeOverTcp(@TempDir Path tmp) throws Exception {
        Path root = writeEchoHolon(tmp);
        List<String> announced = new ArrayList<>();
        Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")));

        Serve.RunningServer running = null;
        try {
            running = Serve.startWithOptions(
                    "tcp://127.0.0.1:0",
                    List.of(new EmptyService()),
                    new Serve.Options().withOnListen(announced::add));

            String publicUri = running.publicUri();
            assertTrue(publicUri.startsWith("tcp://127.0.0.1:"));
            assertEquals(List.of(publicUri), announced);

            String target = publicUri.substring("tcp://".length());
            int idx = target.lastIndexOf(':');
            String host = target.substring(0, idx);
            int port = Integer.parseInt(target.substring(idx + 1));

            ManagedChannel channel = ManagedChannelBuilder.forAddress(host, port)
                    .usePlaintext()
                    .build();

            try {
                holons.v1.Describe.DescribeResponse response = ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Describe.DescribeRequest.getDefaultInstance());

                assertEquals("Echo", response.getManifest().getIdentity().getGivenName());
                assertEquals(1, response.getServicesCount());
                assertEquals("echo.v1.Echo", response.getServices(0).getName());
            } finally {
                channel.shutdownNow();
                channel.awaitTermination(5, TimeUnit.SECONDS);
            }
        } finally {
            Describe.useStaticResponse(null);
            if (running != null) {
                running.stop();
            }
        }
    }

    @Test
    void startWithOptionsRegistersObservabilityService(@TempDir Path tmp) throws Exception {
        Describe.useStaticResponse(staticDescribeResponse());
        Observability.reset();
        Path registryRoot = tmp.resolve("runs");
        Map<String, String> env = Map.of(
                "OP_OBS", "logs,metrics,events",
                "OP_RUN_DIR", registryRoot.toString(),
                "OP_INSTANCE_UID", "java-obs-1");

        Serve.RunningServer running = null;
        try {
            running = Serve.startWithOptions(
                    "tcp://127.0.0.1:0",
                    List.of(new EmptyService()),
                    new Serve.Options()
                            .withEnv(env)
                            .withOnListen(uri -> {
                            }));

            Observability obs = Observability.current();
            obs.logger("serve-test").info("serve-log", Map.of("sdk", "java"));
            obs.counter("serve_requests_total", "", Map.of()).inc();
            obs.emit(Observability.EVENT_INSTANCE_READY, Map.of());

            String target = running.publicUri().substring("tcp://".length());
            int idx = target.lastIndexOf(':');
            String host = target.substring(0, idx);
            int port = Integer.parseInt(target.substring(idx + 1));
            ManagedChannel channel = ManagedChannelBuilder.forAddress(host, port)
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
                assertTrue(logs.stream().anyMatch(entry -> entry.getBody().getStringValue().equals("serve-log")));

                List<holons.v1.Observability.Metric> metrics = new ArrayList<>();
                Iterator<holons.v1.Observability.Metric> metricIterator = ClientCalls.blockingServerStreamingCall(
                        channel,
                        Observability.metricsMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Observability.MetricsRequest.getDefaultInstance());
                metricIterator.forEachRemaining(metrics::add);
                assertTrue(metrics.stream().anyMatch(metric -> metric.getName().equals("serve_requests_total")));

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
            }

            assertTrue(Files.isRegularFile(registryRoot
                    .resolve(obs.cfg.slug)
                    .resolve("java-obs-1")
                    .resolve("meta.json")));
        } finally {
            Describe.useStaticResponse(null);
            Observability.reset();
            if (running != null) {
                running.stop();
            }
        }
    }

    @Test
    void currentTransportTracksServeLifecycle() throws Exception {
        Describe.useStaticResponse(staticDescribeResponse());
        assertEquals("", Serve.currentTransport());
        Serve.RunningServer running = null;
        try {
            running = Serve.startWithOptions(
                    "stdio://",
                    List.of(new EmptyService()),
                    new Serve.Options().withOnListen(uri -> {
                    }));
            assertEquals("stdio", Serve.currentTransport());
        } finally {
            if (running != null) {
                running.stop();
            }
            Describe.useStaticResponse(null);
        }
        assertEquals("", Serve.currentTransport());
    }

    @Test
    void startWithOptionsRelaysMemberObservabilityAndWritesPrometheusAddress(@TempDir Path tmp) throws Exception {
        Describe.useStaticResponse(staticDescribeResponse());
        Observability.reset();
        Path registryRoot = tmp.resolve("runs");

        Serve.RunningServer child = null;
        Serve.RunningServer parent = null;
        try {
            child = Serve.startWithOptions(
                    "tcp://127.0.0.1:0",
                    List.of(new EmptyService()),
                    new Serve.Options()
                            .withSlug("cascade-node-java-child")
                            .withEnv(Map.of(
                                    "OP_OBS", "logs,metrics,events",
                                    "OP_RUN_DIR", registryRoot.toString(),
                                    "OP_INSTANCE_UID", "java-child-1")));

            Observability childObs = Observability.current();
            childObs.logger("tick").info("tick received", Map.of("sender", "serve-test"));
            childObs.emit(Observability.EVENT_CONFIG_RELOADED, Map.of("source", "serve-test"));

            parent = Serve.startWithOptions(
                    "tcp://127.0.0.1:0",
                    List.of(new EmptyService()),
                    new Serve.Options()
                            .withSlug("cascade-node-java-parent")
                            .withEnv(Map.of(
                                    "OP_OBS", "logs,metrics,events,prom",
                                    "OP_RUN_DIR", registryRoot.toString(),
                                    "OP_INSTANCE_UID", "java-parent-1",
                                    "OP_PROM_ADDR", ":0"))
                            .withMemberEndpoints(List.of(new Serve.MemberRef(
                                    "cascade-node-java-child",
                                    "",
                                    child.publicUri()))));

            Observability parentObs = Observability.current();
            awaitCondition(() -> parentObs.logRing.drain().stream()
                    .anyMatch(entry -> entry.bodyString().equals("tick received")
                            && entry.record.getChainCount() == 1
                            && entry.record.getChain(0).equals("cascade-node-java-child")));
            awaitCondition(() -> parentObs.eventBus.drain().stream()
                    .anyMatch(event -> event.record.getEventName().equals(Observability.EVENT_CONFIG_RELOADED)
                            && event.record.getChainCount() == 1
                            && event.record.getChain(0).equals("cascade-node-java-child")));

            String parentMeta = Files.readString(registryRoot
                    .resolve("cascade-node-java-parent")
                    .resolve("java-parent-1")
                    .resolve("meta.json"));
            assertTrue(parentMeta.contains("\"metrics_addr\""));
        } finally {
            if (parent != null) {
                parent.stop();
            }
            if (child != null) {
                child.stop();
            }
            Observability.reset();
            Describe.useStaticResponse(null);
        }
    }

    @Test
    void startWithOptionsServesDescribeOverUnix(@TempDir Path tmp) throws Exception {
        Path root = writeEchoHolon(tmp);
        Path socketPath = root.resolve("serve.sock");
        Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")));

        Serve.RunningServer running = null;
        try {
            running = Serve.startWithOptions(
                    "unix://" + socketPath,
                    List.of(new EmptyService()),
                    new Serve.Options().withOnListen(uri -> {
                    }));

            ManagedChannel channel = Connect.connect("unix://" + socketPath);

            try {
                holons.v1.Describe.DescribeResponse response = ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Describe.DescribeRequest.getDefaultInstance());

                assertEquals("unix://" + socketPath, running.publicUri());
                assertEquals("Echo", response.getManifest().getIdentity().getGivenName());
                assertEquals(1, response.getServicesCount());
                assertEquals("echo.v1.Echo", response.getServices(0).getName());
            } finally {
                Connect.disconnect(channel);
            }
        } finally {
            Describe.useStaticResponse(null);
            if (running != null) {
                running.stop();
            }
        }
    }

    @Test
    void startWithOptionsFailsWithoutStaticDescribeRegistration() {
        Describe.useStaticResponse(null);
        List<String> logs = new ArrayList<>();

        IOException error = assertThrows(IOException.class, () -> Serve.startWithOptions(
                "tcp://127.0.0.1:0",
                List.of(new EmptyService()),
                new Serve.Options().withLogger(logs::add)));

        assertTrue(error.getMessage().contains(Describe.NO_INCODE_DESCRIPTION_MESSAGE));
        assertTrue(logs.stream().anyMatch(line ->
                line.contains("HolonMeta registration failed: " + Describe.NO_INCODE_DESCRIPTION_MESSAGE)));
    }

    @Test
    void startWithOptionsRejectsWebSocketServeUris() {
        Describe.useStaticResponse(staticDescribeResponse());
        try {
            IllegalArgumentException wsError = assertThrows(IllegalArgumentException.class, () -> Serve.startWithOptions(
                    "ws://127.0.0.1:0/grpc",
                    List.of(new EmptyService()),
                    new Serve.Options()));
            assertTrue(wsError.getMessage().contains("tcp://, unix:// and stdio:// only"));

            IllegalArgumentException wssError = assertThrows(IllegalArgumentException.class, () -> Serve.startWithOptions(
                    "wss://127.0.0.1:0/grpc",
                    List.of(new EmptyService()),
                    new Serve.Options()));
            assertTrue(wssError.getMessage().contains("tcp://, unix:// and stdio:// only"));
        } finally {
            Describe.useStaticResponse(null);
        }
    }

    @Test
    void startWithOptionsRejectsHttpSseServeUris() {
        Describe.useStaticResponse(staticDescribeResponse());
        try {
            IllegalArgumentException error = assertThrows(IllegalArgumentException.class, () -> Serve.startWithOptions(
                    "http://127.0.0.1:8080/api/v1/rpc",
                    List.of(new EmptyService()),
                    new Serve.Options()));
            assertTrue(error.getMessage().contains("unsupported transport URI"));
        } finally {
            Describe.useStaticResponse(null);
        }
    }

    private static Path writeEchoHolon(Path root) throws Exception {
        Path protoDir = root.resolve("protos/echo/v1");
        Files.createDirectories(protoDir);

        Files.writeString(root.resolve("holon.proto"), """
                syntax = "proto3";
                package holons.test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    given_name: "Echo"
                    family_name: "Server"
                    motto: "Reply precisely."
                  }
                };
                """);

        Files.writeString(protoDir.resolve("echo.proto"), """
                syntax = "proto3";
                package echo.v1;

                service Echo {
                  rpc Ping(PingRequest) returns (PingResponse);
                }

                message PingRequest {
                  string message = 1;
                }

                message PingResponse {
                  string message = 1;
                }
                """);

        return root;
    }

    private static final class EmptyService implements BindableService {
        @Override
        public ServerServiceDefinition bindService() {
            return ServerServiceDefinition.builder("empty.v1.Empty").build();
        }
    }

    private static holons.v1.Describe.DescribeResponse staticDescribeResponse() {
        return holons.v1.Describe.DescribeResponse.newBuilder()
                .setManifest(holons.v1.Manifest.HolonManifest.newBuilder()
                        .setIdentity(holons.v1.Manifest.HolonManifest.Identity.newBuilder()
                                .setUuid("serve-transport-0001")
                                .setGivenName("Serve")
                                .setFamilyName("Transport")
                                .build())
                        .setLang("java")
                        .build())
                .build();
    }

    private static void awaitCondition(BooleanSupplier condition) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
        while (System.nanoTime() < deadline) {
            if (condition.getAsBoolean()) {
                return;
            }
            Thread.sleep(50);
        }
        assertTrue(condition.getAsBoolean());
    }
}

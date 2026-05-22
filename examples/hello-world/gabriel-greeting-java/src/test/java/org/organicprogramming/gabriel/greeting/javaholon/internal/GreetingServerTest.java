package org.organicprogramming.gabriel.greeting.javaholon.internal;

import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.Test;
import org.organicprogramming.holons.Observability;
import org.organicprogramming.holons.Serve;

import java.util.UUID;
import java.util.Map;
import java.util.Set;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class GreetingServerTest {
    @Test
    void listLanguagesReturnsAllLanguages() throws Exception {
        String serverName = UUID.randomUUID().toString();
        io.grpc.Server server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(new GreetingServer())
                .build()
                .start();
        try {
            ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
            try {
                GreetingServiceGrpc.GreetingServiceBlockingStub stub = GreetingServiceGrpc.newBlockingStub(channel);
                Greeting.ListLanguagesResponse response = stub.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance());
                assertEquals(56, response.getLanguagesCount());
                assertFalse(response.getLanguagesList().stream().anyMatch(language ->
                        language.getCode().isEmpty() || language.getName().isEmpty() || language.getNative().isEmpty()));
            } finally {
                channel.shutdownNow();
            }
        } finally {
            server.shutdownNow();
        }
    }

    @Test
    void sayHelloUsesRequestedLanguage() throws Exception {
        String serverName = UUID.randomUUID().toString();
        io.grpc.Server server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(new GreetingServer())
                .build()
                .start();
        try {
            ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
            try {
                GreetingServiceGrpc.GreetingServiceBlockingStub stub = GreetingServiceGrpc.newBlockingStub(channel);
                Greeting.SayHelloResponse response = stub.sayHello(Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("fr")
                        .build());
                assertEquals("Bonjour Bob", response.getGreeting());
                assertEquals("French", response.getLanguage());
                assertEquals("fr", response.getLangCode());
            } finally {
                channel.shutdownNow();
            }
        } finally {
            server.shutdownNow();
        }
    }

    @Test
    void sayHelloUsesLocalizedDefaultName() throws Exception {
        String serverName = UUID.randomUUID().toString();
        io.grpc.Server server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(new GreetingServer())
                .build()
                .start();
        try {
            ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
            try {
                GreetingServiceGrpc.GreetingServiceBlockingStub stub = GreetingServiceGrpc.newBlockingStub(channel);
                Greeting.SayHelloResponse response = stub.sayHello(Greeting.SayHelloRequest.newBuilder()
                        .setLangCode("fr")
                        .build());
                assertEquals("Bonjour Marie", response.getGreeting());
                assertEquals("fr", response.getLangCode());
            } finally {
                channel.shutdownNow();
            }
        } finally {
            server.shutdownNow();
        }
    }

    @Test
    void sayHelloFallsBackToEnglish() throws Exception {
        String serverName = UUID.randomUUID().toString();
        io.grpc.Server server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(new GreetingServer())
                .build()
                .start();
        try {
            ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
            try {
                GreetingServiceGrpc.GreetingServiceBlockingStub stub = GreetingServiceGrpc.newBlockingStub(channel);
                Greeting.SayHelloResponse response = stub.sayHello(Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("xx")
                        .build());
                assertEquals("Hello Bob", response.getGreeting());
                assertEquals("en", response.getLangCode());
            } finally {
                channel.shutdownNow();
            }
        } finally {
            server.shutdownNow();
        }
    }

    @Test
    void sayHelloEmitsGreetingObservability() throws Exception {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "gabriel-greeting-java-test";
        cfg.instanceUid = "gabriel-greeting-java-test-1";
        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs,metrics"));
        try {
            String serverName = UUID.randomUUID().toString();
            io.grpc.Server server = InProcessServerBuilder.forName(serverName)
                    .directExecutor()
                    .addService(new GreetingServer())
                    .build()
                    .start();
            try {
                ManagedChannel channel = InProcessChannelBuilder.forName(serverName).directExecutor().build();
                try {
                    GreetingServiceGrpc.GreetingServiceBlockingStub stub = GreetingServiceGrpc.newBlockingStub(channel);
                    Greeting.SayHelloResponse response = stub.sayHello(Greeting.SayHelloRequest.newBuilder()
                            .setName("Bob")
                            .setLangCode("fr")
                            .build());
                    assertEquals("Bonjour Bob", response.getGreeting());
                } finally {
                    channel.shutdownNow();
                }
            } finally {
                server.shutdownNow();
            }

            Observability.LogRecord entry = obs.logRing.drain().stream()
                    .filter(log -> "Greeted Bob in French (fr)".equals(log.bodyString()))
                    .findFirst()
                    .orElseThrow();
            Map<String, holons.v1.Observability.AnyValue> attrs = attrs(entry.record);
            assertTrue(attrs.keySet().containsAll(Set.of(
                    Observability.ATTR_HOLONS_SLUG,
                    Observability.ATTR_SERVICE_NAME,
                    Observability.ATTR_HOLONS_INSTANCE_UID,
                    Observability.ATTR_SERVICE_INSTANCE_ID,
                    Observability.ATTR_HOLONS_SESSION_ID,
                    "lang_code", "language", "name", "greeting", "transport", "duration_ns")));
            assertEquals("fr", attrs.get("lang_code").getStringValue());
            assertEquals("French", attrs.get("language").getStringValue());
            assertEquals("Bob", attrs.get("name").getStringValue());
            assertEquals("Bonjour Bob", attrs.get("greeting").getStringValue());
            assertEquals("unknown", attrs.get("transport").getStringValue());
            assertEquals(holons.v1.Observability.AnyValue.ValueCase.INT_VALUE, attrs.get("duration_ns").getValueCase());
            assertTrue(attrs.get("duration_ns").getIntValue() >= 0);

            Observability.Counter counter = obs.registry.counters().stream()
                    .filter(metric -> "greeting_emitted_total".equals(metric.name))
                    .findFirst()
                    .orElseThrow();
            assertEquals(1, counter.value());
            assertEquals(Set.of("lang_code", "language", "transport"), counter.labels.keySet());
            assertEquals(Map.of("lang_code", "fr", "language", "French", "transport", "unknown"), counter.labels);
        } finally {
            Observability.reset();
        }
    }

    @Test
    void sayHelloEmitsStdioTransportWhenServeLifecycleIsStdio() throws Exception {
        Observability.reset();
        Observability.Config cfg = new Observability.Config();
        cfg.slug = "gabriel-greeting-java-test";
        cfg.instanceUid = "gabriel-greeting-java-test-stdio";
        Observability obs = Observability.configureFromEnv(cfg, Map.of("OP_OBS", "logs"));
        Serve.RunningServer running = null;
        try {
            running = Serve.startWithOptions(
                    "stdio://",
                    List.of(new GreetingServer()),
                    new Serve.Options()
                            .withDescribe(false)
                            .withOnListen(uri -> {
                            }));
            assertEquals("stdio", Serve.currentTransport());

            AtomicReference<Greeting.SayHelloResponse> responseRef = new AtomicReference<>();
            new GreetingServer().sayHello(
                    Greeting.SayHelloRequest.newBuilder().setName("Ada").setLangCode("en").build(),
                    new StreamObserver<>() {
                        @Override
                        public void onNext(Greeting.SayHelloResponse value) {
                            responseRef.set(value);
                        }

                        @Override
                        public void onError(Throwable t) {
                            throw new AssertionError(t);
                        }

                        @Override
                        public void onCompleted() {
                        }
                    });
            assertEquals("Hello Ada", responseRef.get().getGreeting());

            Observability.LogRecord entry = obs.logRing.drain().stream()
                    .filter(log -> "Greeted Ada in English (en)".equals(log.bodyString()))
                    .findFirst()
                    .orElseThrow();
            assertEquals("stdio", attrs(entry.record).get("transport").getStringValue());
        } finally {
            if (running != null) {
                running.stop();
            }
            Observability.reset();
        }
        assertEquals("", Serve.currentTransport());
    }

    private static Map<String, holons.v1.Observability.AnyValue> attrs(holons.v1.Observability.LogRecord record) {
        Map<String, holons.v1.Observability.AnyValue> out = new java.util.HashMap<>();
        for (holons.v1.Observability.KeyValue attr : record.getAttributesList()) {
            out.put(attr.getKey(), attr.getValue());
        }
        return out;
    }
}

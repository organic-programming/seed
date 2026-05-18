package org.organicprogramming.gabriel.greeting.javaholon.internal;

import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.Test;
import org.organicprogramming.holons.Observability;

import java.util.UUID;
import java.util.Map;
import java.util.Set;

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

            Observability.LogEntry entry = obs.logRing.drain().stream()
                    .filter(log -> "Greeted Bob in French (fr)".equals(log.message))
                    .findFirst()
                    .orElseThrow();
            assertEquals(Set.of("lang_code", "language", "name", "greeting", "transport", "duration_ns"),
                    entry.fields.keySet());
            assertEquals("fr", entry.fields.get("lang_code"));
            assertEquals("French", entry.fields.get("language"));
            assertEquals("Bob", entry.fields.get("name"));
            assertEquals("Bonjour Bob", entry.fields.get("greeting"));
            assertEquals("unknown", entry.fields.get("transport"));
            assertTrue(Long.parseLong(entry.fields.get("duration_ns")) >= 0);

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
}

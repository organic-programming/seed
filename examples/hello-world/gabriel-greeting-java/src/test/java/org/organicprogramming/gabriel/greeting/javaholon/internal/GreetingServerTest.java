package org.organicprogramming.gabriel.greeting.javaholon.internal;

import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.ManagedChannel;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import org.junit.jupiter.api.Test;

import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;

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
}

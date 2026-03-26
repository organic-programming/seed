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
import java.util.List;
import java.util.concurrent.TimeUnit;

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
}

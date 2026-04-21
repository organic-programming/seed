package org.organicprogramming.holons;

import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.ClientCalls;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.organicprogramming.holons.support.ProtoLessDescribeHelperMain;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class DescribeTest {
    @Test
    void buildResponseFromEchoProto(@TempDir Path tmp) throws IOException {
        Path root = writeEchoHolon(tmp);

        holons.v1.Describe.DescribeResponse response = Describe.buildResponse(root.resolve("protos"));

        assertEquals("Echo", response.getManifest().getIdentity().getGivenName());
        assertEquals("Server", response.getManifest().getIdentity().getFamilyName());
        assertEquals("Reply precisely.", response.getManifest().getIdentity().getMotto());
        assertEquals(1, response.getServicesCount());

        holons.v1.Describe.ServiceDoc service = response.getServices(0);
        assertEquals("echo.v1.Echo", service.getName());
        assertEquals("Echo echoes request payloads for documentation tests.", service.getDescription());
        assertEquals(1, service.getMethodsCount());

        holons.v1.Describe.MethodDoc method = service.getMethods(0);
        assertEquals("Ping", method.getName());
        assertEquals("Ping echoes the inbound message.", method.getDescription());
        assertEquals("echo.v1.PingRequest", method.getInputType());
        assertEquals("echo.v1.PingResponse", method.getOutputType());
        assertEquals("{\"message\":\"hello\",\"sdk\":\"go-holons\"}", method.getExampleInput());
        assertEquals(2, method.getInputFieldsCount());

        holons.v1.Describe.FieldDoc field = method.getInputFields(0);
        assertEquals("message", field.getName());
        assertEquals("string", field.getType());
        assertEquals(1, field.getNumber());
        assertEquals("Message to echo back.", field.getDescription());
        assertEquals(holons.v1.Describe.FieldLabel.FIELD_LABEL_OPTIONAL, field.getLabel());
        assertTrue(field.getRequired());
        assertEquals("\"hello\"", field.getExample());
    }

    @Test
    void registersWorkingDescribeRpc(@TempDir Path tmp) throws Exception {
        Path root = writeEchoHolon(tmp);
        Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")));

        Server server = ServerBuilder.forPort(0)
                .addService(Describe.service())
                .build()
                .start();

        ManagedChannel channel = ManagedChannelBuilder
                .forAddress("127.0.0.1", server.getPort())
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
            assertEquals("Ping", response.getServices(0).getMethods(0).getName());
        } finally {
            Describe.useStaticResponse(null);
            channel.shutdownNow();
            channel.awaitTermination(5, TimeUnit.SECONDS);
            server.shutdownNow();
            server.awaitTermination(5, TimeUnit.SECONDS);
        }
    }

    @Test
    void handlesMissingProtoDirectory(@TempDir Path tmp) throws IOException {
        Files.writeString(tmp.resolve("holon.proto"), """
                syntax = "proto3";
                package test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    given_name: "Silent"
                    family_name: "Holon"
                    motto: "Quietly available."
                  }
                };
                """);

        holons.v1.Describe.DescribeResponse response = Describe.buildResponse(tmp.resolve("protos"));

        assertEquals("Silent", response.getManifest().getIdentity().getGivenName());
        assertEquals("Holon", response.getManifest().getIdentity().getFamilyName());
        assertEquals("Quietly available.", response.getManifest().getIdentity().getMotto());
        assertEquals(0, response.getServicesCount());
    }

    @Test
    void serviceRequiresStaticDescribeRegistration() {
        Describe.useStaticResponse(null);

        IllegalStateException error = assertThrows(IllegalStateException.class, Describe::service);
        assertEquals(Describe.NO_INCODE_DESCRIPTION_MESSAGE, error.getMessage());
    }

    @Test
    void builtHelperServesStaticDescribeWithoutAdjacentProtoFiles(@TempDir Path tmp) throws Exception {
        try (Stream<Path> walk = Files.walk(tmp)) {
            assertFalse(walk.anyMatch(path ->
                    Identity.PROTO_MANIFEST_FILE_NAME.equals(path.getFileName() != null ? path.getFileName().toString() : "")));
        }

        Process process = new ProcessBuilder(
                javaBinary(),
                "-cp",
                System.getProperty("java.class.path"),
                ProtoLessDescribeHelperMain.class.getName())
                .directory(tmp.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader stdout = new BufferedReader(new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
            String publicUri = readLineWithTimeout(stdout, Duration.ofSeconds(20));
            assertNotNull(publicUri);
            assertTrue(publicUri.startsWith("tcp://127.0.0.1:"));

            String target = publicUri.substring("tcp://".length());
            int index = target.lastIndexOf(':');
            ManagedChannel channel = ManagedChannelBuilder.forAddress(
                    target.substring(0, index),
                    Integer.parseInt(target.substring(index + 1)))
                    .usePlaintext()
                    .build();

            try {
                holons.v1.Describe.DescribeResponse response = ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Describe.DescribeRequest.getDefaultInstance());

                assertEquals("Static", response.getManifest().getIdentity().getGivenName());
                assertEquals("Only", response.getManifest().getIdentity().getFamilyName());
                assertEquals("Served without runtime proto parsing.", response.getManifest().getIdentity().getMotto());
                assertEquals(1, response.getServicesCount());
                assertEquals("static.v1.Echo", response.getServices(0).getName());
            } finally {
                channel.shutdownNow();
                channel.awaitTermination(5, TimeUnit.SECONDS);
            }
        } finally {
            process.destroy();
            if (!process.waitFor(10, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                process.waitFor(5, TimeUnit.SECONDS);
            }
        }
    }

    private static Path writeEchoHolon(Path root) throws IOException {
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

                // Echo echoes request payloads for documentation tests.
                service Echo {
                  // Ping echoes the inbound message.
                  // @example {"message":"hello","sdk":"go-holons"}
                  rpc Ping(PingRequest) returns (PingResponse);
                }

                message PingRequest {
                  // Message to echo back.
                  // @required
                  // @example "hello"
                  string message = 1;

                  // SDK marker included in the response.
                  // @example "go-holons"
                  string sdk = 2;
                }

                message PingResponse {
                  // Echoed message.
                  string message = 1;

                  // SDK marker from the server.
                  string sdk = 2;
                }
                """);

        return root;
    }

    private static String javaBinary() {
        return Path.of(System.getProperty("java.home"), "bin", "java").toString();
    }

    private static String readLineWithTimeout(BufferedReader reader, Duration timeout)
            throws InterruptedException, ExecutionException, TimeoutException {
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try {
            Future<String> lineFuture = executor.submit(reader::readLine);
            return lineFuture.get(timeout.toMillis(), TimeUnit.MILLISECONDS);
        } finally {
            executor.shutdownNow();
        }
    }
}

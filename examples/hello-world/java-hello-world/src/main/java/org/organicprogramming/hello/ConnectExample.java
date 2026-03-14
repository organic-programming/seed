package org.organicprogramming.hello;

import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.MethodDescriptor;
import io.grpc.stub.ClientCalls;
import org.organicprogramming.holons.Connect;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;

public final class ConnectExample {
    private static final MethodDescriptor<String, String> PING_METHOD =
            MethodDescriptor.<String, String>newBuilder()
                    .setType(MethodDescriptor.MethodType.UNARY)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName("echo.v1.Echo", "Ping"))
                    .setRequestMarshaller(StringMarshaller.INSTANCE)
                    .setResponseMarshaller(StringMarshaller.INSTANCE)
                    .build();

    private ConnectExample() {
    }

    public static void main(String[] args) throws Exception {
        Path tempRoot = Files.createTempDirectory("java-holons-connect-");
        String previousDir = System.getProperty("user.dir", ".");

        try {
            writeEchoHolon(tempRoot, resolveEchoServer());
            System.setProperty("user.dir", tempRoot.toString());

            ManagedChannel channel = Connect.connect("echo-server");
            try {
                String response = ClientCalls.blockingUnaryCall(
                        channel,
                        PING_METHOD,
                        CallOptions.DEFAULT.withDeadlineAfter(5, TimeUnit.SECONDS),
                        "{\"message\":\"hello-from-java\"}");
                System.out.println(response);
            } finally {
                Connect.disconnect(channel);
            }
        } finally {
            System.setProperty("user.dir", previousDir);
            deleteRecursively(tempRoot);
        }
    }

    private static String resolveEchoServer() {
        Path path = Path.of(System.getProperty("user.dir", "."))
                .resolve("../../sdk/java-holons/bin/echo-server")
                .toAbsolutePath()
                .normalize();
        if (!Files.isRegularFile(path)) {
            throw new IllegalStateException("echo-server not found at " + path);
        }
        return path.toString();
    }

    private static void writeEchoHolon(Path root, String binaryPath) throws IOException {
        Path holonDir = root.resolve("holons/echo-server");
        Files.createDirectories(holonDir);
        Files.writeString(
                holonDir.resolve("holon.yaml"),
                """
                uuid: "echo-server-connect-example"
                given_name: Echo
                family_name: Server
                motto: Reply precisely.
                composer: "connect-example"
                kind: service
                build:
                  runner: java
                  main: bin/echo-server
                artifacts:
                  binary: "%s"
                """.formatted(binaryPath));
    }

    private static void deleteRecursively(Path root) throws IOException {
        if (root == null || !Files.exists(root)) {
            return;
        }
        try (var walk = Files.walk(root)) {
            walk.sorted((left, right) -> right.compareTo(left))
                    .forEach(path -> {
                        try {
                            Files.deleteIfExists(path);
                        } catch (IOException ignored) {
                            // Best-effort tempdir cleanup.
                        }
                    });
        }
    }

    private enum StringMarshaller implements MethodDescriptor.Marshaller<String> {
        INSTANCE;

        @Override
        public InputStream stream(String value) {
            return new ByteArrayInputStream(value.getBytes(StandardCharsets.UTF_8));
        }

        @Override
        public String parse(InputStream stream) {
            try {
                return new String(stream.readAllBytes(), StandardCharsets.UTF_8);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
    }
}

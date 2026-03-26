package org.organicprogramming.holons;

import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import io.grpc.BindableService;
import io.grpc.CallOptions;
import io.grpc.ManagedChannel;
import io.grpc.MethodDescriptor;
import io.grpc.ServerServiceDefinition;
import io.grpc.stub.ClientCalls;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Type;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ConnectTest {

    private static final Path SDK_ROOT = Path.of("").toAbsolutePath().normalize();
    private static final Gson GSON = new Gson();
    private static final Type MAP_TYPE = new TypeToken<Map<String, Object>>() {
    }.getType();
    private static final MethodDescriptor<String, String> PING_METHOD =
            MethodDescriptor.<String, String>newBuilder()
                    .setType(MethodDescriptor.MethodType.UNARY)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName("echo.v1.Echo", "Ping"))
                    .setRequestMarshaller(stringMarshaller())
                    .setResponseMarshaller(stringMarshaller())
                    .build();

    @Test
    void connectStartsSlugOverStdioByDefault(@TempDir Path tmpRoot) throws Exception {
        ConnectFixture fixture = createFixture(tmpRoot, "Connect", "Stdio");

        withConnectEnvironment(tmpRoot, () -> {
            ManagedChannel channel = Connect.connect(fixture.slug());
            int pid = waitForPidFile(fixture.pidFile());
            List<String> args = waitForArgsFile(fixture.argsFile());

            try {
                Map<String, Object> out = invokePing(channel, "java-connect-stdio");
                assertEquals("java-connect-stdio", out.get("message"));
                assertEquals("java-holons", out.get("sdk"));
                assertEquals(List.of("serve", "--listen", "stdio://"), args);
                assertFalse(Files.exists(fixture.portFile()));
            } finally {
                Connect.disconnect(channel);
            }

            waitForPidExit(pid);
        });
    }

    @Test
    void connectWritesUnixPortFileInPersistentMode(@TempDir Path tmpRoot) throws Exception {
        ConnectFixture fixture = createFixture(tmpRoot, "Connect", "Unix");

        withConnectEnvironment(tmpRoot, () -> {
            ManagedChannel channel = Connect.connect(
                    fixture.slug(),
                    new Connect.ConnectOptions(Duration.ofSeconds(5), "unix", true, null));
            int pid = waitForPidFile(fixture.pidFile());

            try {
                Map<String, Object> out = invokePing(channel, "java-connect-unix");
                assertEquals("java-connect-unix", out.get("message"));
            } finally {
                Connect.disconnect(channel);
            }

            String target = Files.readString(fixture.portFile()).trim();
            assertTrue(target.startsWith("unix:///tmp/holons-"));
            assertTrue(ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false));

            ManagedChannel reused = Connect.connect(fixture.slug());
            try {
                Map<String, Object> out = invokePing(reused, "java-connect-unix-reuse");
                assertEquals("java-connect-unix-reuse", out.get("message"));
            } finally {
                Connect.disconnect(reused);
                ProcessHandle.of(pid).ifPresent(ProcessHandle::destroy);
                waitForPidExit(pid);
            }
        });
    }

    @Test
    void connectDialsDirectTcpTarget(@TempDir Path tmpRoot) throws Exception {
        Describe.useStaticResponse(staticDescribeResponse());
        Serve.RunningServer running = null;

        try {
            running = Serve.startWithOptions(
                    "tcp://127.0.0.1:0",
                    List.of(new EmptyService()),
                    new Serve.Options().withLogger(line -> {
                    }));

            ManagedChannel channel = Connect.connect(running.publicUri());
            try {
                holons.v1.Describe.DescribeResponse response = ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT,
                        holons.v1.Describe.DescribeRequest.getDefaultInstance());
                assertEquals("Connect", response.getManifest().getIdentity().getGivenName());
                assertEquals("Direct", response.getManifest().getIdentity().getFamilyName());
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
    void connectRejectsHttpSseTargets() {
        IllegalArgumentException error = assertThrows(IllegalArgumentException.class,
                () -> Connect.connect("http://127.0.0.1:8080/api/v1/rpc"));
        assertTrue(error.getMessage().contains("unsupported transport URI"));
    }

    private static Map<String, Object> invokePing(ManagedChannel channel, String message) {
        String response = ClientCalls.blockingUnaryCall(
                channel,
                PING_METHOD,
                CallOptions.DEFAULT.withDeadlineAfter(2, java.util.concurrent.TimeUnit.SECONDS),
                GSON.toJson(Map.of("message", message)));
        return GSON.fromJson(response, MAP_TYPE);
    }

    private static MethodDescriptor.Marshaller<String> stringMarshaller() {
        return new MethodDescriptor.Marshaller<>() {
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
        };
    }

    private static ConnectFixture createFixture(Path root, String givenName, String familyName) throws IOException {
        String slug = (givenName + "-" + familyName).toLowerCase();
        Path holonDir = root.resolve("holons").resolve(slug);
        Path binaryDir = holonDir.resolve(".op").resolve("build").resolve("bin");
        Files.createDirectories(binaryDir);

        Path wrapper = binaryDir.resolve("echo-wrapper");
        Path pidFile = root.resolve(slug + ".pid");
        Path argsFile = root.resolve(slug + ".args");
        Path portFile = root.resolve(".op").resolve("run").resolve(slug + ".port");

        Files.writeString(wrapper, """
                #!/usr/bin/env bash
                set -euo pipefail
                printf '%%s\n' "$$" > '%s'
                : > '%s'
                for arg in "$@"; do
                  printf '%%s\n' "$arg" >> '%s'
                done
                exec '%s' "$@"
                """.formatted(
                pidFile,
                argsFile,
                argsFile,
                SDK_ROOT.resolve("bin").resolve("echo-server")));
        assertTrue(wrapper.toFile().setExecutable(true), "fixture wrapper should be executable");

        Files.createDirectories(holonDir);
        Files.writeString(holonDir.resolve("holon.proto"), """
                syntax = "proto3";
                package holons.test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    uuid: "%s-uuid"
                    given_name: "%s"
                    family_name: "%s"
                    composer: "connect-test"
                  }
                  kind: "service"
                  build: {
                    runner: "shell"
                  }
                  artifacts: {
                    binary: "echo-wrapper"
                  }
                };
                """.formatted(slug, givenName, familyName));

        return new ConnectFixture(root, slug, pidFile, argsFile, portFile);
    }

    private static void withConnectEnvironment(Path root, CheckedRunnable runnable) throws Exception {
        String previousUserDir = System.getProperty("user.dir");
        String previousOpPath = System.getProperty("OPPATH");
        String previousOpBin = System.getProperty("OPBIN");

        System.setProperty("user.dir", root.toString());
        System.setProperty("OPPATH", root.resolve(".op-home").toString());
        System.setProperty("OPBIN", root.resolve(".op-bin").toString());

        try {
            runnable.run();
        } finally {
            restoreProperty("user.dir", previousUserDir);
            restoreProperty("OPPATH", previousOpPath);
            restoreProperty("OPBIN", previousOpBin);
        }
    }

    private static void restoreProperty(String key, String value) {
        if (value == null) {
            System.clearProperty(key);
        } else {
            System.setProperty(key, value);
        }
    }

    private static int waitForPidFile(Path path) throws Exception {
        long deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos();
        while (System.nanoTime() < deadline) {
            try {
                int pid = Integer.parseInt(Files.readString(path).trim());
                if (pid > 0) {
                    return pid;
                }
            } catch (Exception ignored) {
                // Wrapper is still starting.
            }
            Thread.sleep(25);
        }
        throw new AssertionError("timed out waiting for pid file " + path);
    }

    private static List<String> waitForArgsFile(Path path) throws Exception {
        long deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos();
        while (System.nanoTime() < deadline) {
            try {
                List<String> lines = Files.readAllLines(path, StandardCharsets.UTF_8).stream()
                        .filter(line -> !line.isBlank())
                        .toList();
                if (!lines.isEmpty()) {
                    return lines;
                }
            } catch (IOException ignored) {
                // Wrapper is still starting.
            }
            Thread.sleep(25);
        }
        throw new AssertionError("timed out waiting for args file " + path);
    }

    private static void waitForPidExit(int pid) throws Exception {
        long deadline = System.nanoTime() + Duration.ofSeconds(2).toNanos();
        while (System.nanoTime() < deadline) {
            if (!ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false)) {
                return;
            }
            Thread.sleep(25);
        }
        throw new AssertionError("process " + pid + " did not exit");
    }

    private record ConnectFixture(Path root, String slug, Path pidFile, Path argsFile, Path portFile) {
    }

    @FunctionalInterface
    private interface CheckedRunnable {
        void run() throws Exception;
    }

    private static holons.v1.Describe.DescribeResponse staticDescribeResponse() {
        return holons.v1.Describe.DescribeResponse.newBuilder()
                .setManifest(holons.v1.Manifest.HolonManifest.newBuilder()
                        .setIdentity(holons.v1.Manifest.HolonManifest.Identity.newBuilder()
                                .setUuid("connect-direct-0001")
                                .setGivenName("Connect")
                                .setFamilyName("Direct")
                                .build())
                        .setLang("java")
                        .build())
                .build();
    }

    private static final class EmptyService implements BindableService {
        @Override
        public ServerServiceDefinition bindService() {
            return ServerServiceDefinition.builder("empty.v1.Empty").build();
        }
    }
}

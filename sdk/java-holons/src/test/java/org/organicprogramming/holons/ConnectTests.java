package org.organicprogramming.holons;

import io.grpc.ManagedChannel;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.organicprogramming.holons.DiscoveryTestSupport.PackageSeed;
import org.organicprogramming.holons.DiscoveryTestSupport.RuntimeFixture;
import org.organicprogramming.holons.DiscoveryTypes.ConnectResult;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.organicprogramming.holons.DiscoveryTestSupport.platformTag;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeEnv;
import static org.organicprogramming.holons.DiscoveryTestSupport.runtimeFixture;
import static org.organicprogramming.holons.DiscoveryTestSupport.slugFor;
import static org.organicprogramming.holons.DiscoveryTestSupport.writeExecutable;
import static org.organicprogramming.holons.DiscoveryTestSupport.writePackageHolon;

class ConnectTests {
    private static final Path SDK_ROOT = Path.of("").toAbsolutePath().normalize();

    @Test
    void unresolvableTarget(@TempDir Path root) throws Exception {
        RuntimeFixture fixture = runtimeFixture(root);
        try (var env = runtimeEnv(fixture)) {
            ConnectResult result = Connect.connect(Connect.LOCAL, "missing", root.toString(), Connect.INSTALLED, 1_000);
            assertNotEquals("", result.error);
            assertNull(result.channel);
        }
    }

    @Test
    void returnsConnectResult(@TempDir Path root) throws Exception {
        ConnectFixture fixture = createFixture(root, "Connect", "Result");
        try (var env = runtimeEnv(fixture.runtime())) {
            ConnectResult result = Connect.connect(Connect.LOCAL, fixture.slug(), root.toString(), Connect.INSTALLED, 5_000);
            assertEquals("", result.error);
            assertNotNull(result.channel);
            assertTrue(result.channel instanceof ManagedChannel);
            Connect.disconnect(result);
        }
    }

    @Test
    void populatesOrigin(@TempDir Path root) throws Exception {
        ConnectFixture fixture = createFixture(root, "Origin", "Fixture");
        try (var env = runtimeEnv(fixture.runtime())) {
            ConnectResult result = Connect.connect(Connect.LOCAL, fixture.slug(), root.toString(), Connect.INSTALLED, 5_000);
            assertEquals("", result.error);
            assertNotNull(result.origin);
            assertNotNull(result.origin.info);
            assertEquals(fixture.slug(), result.origin.info.slug);
            assertEquals(fixture.packageDir().toAbsolutePath().normalize().toUri().toString(), result.origin.url);
            Connect.disconnect(result);
        }
    }

    @Test
    void disconnectAcceptsConnectResult(@TempDir Path root) throws Exception {
        ConnectFixture fixture = createFixture(root, "Disconnect", "Fixture");
        try (var env = runtimeEnv(fixture.runtime())) {
            ConnectResult result = Connect.connect(Connect.LOCAL, fixture.slug(), root.toString(), Connect.INSTALLED, 5_000);
            assertEquals("", result.error);
            int pid = waitForPidFile(fixture.pidFile());
            Connect.disconnect(result);
            waitForPidExit(pid);
        }
    }

    private static ConnectFixture createFixture(Path root, String givenName, String familyName) throws Exception {
        RuntimeFixture runtime = runtimeFixture(root);
        String slug = slugFor(givenName, familyName);
        Path packageDir = runtime.opBin().resolve(slug + ".holon");
        Path binaryDir = packageDir.resolve("bin").resolve(platformTag());
        Path wrapper = binaryDir.resolve("echo-wrapper");
        Path pidFile = root.resolve(slug + ".pid");
        Path argsFile = root.resolve(slug + ".args");

        writeExecutable(wrapper, """
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

        writePackageHolon(packageDir, new PackageSeed(slug + "-uuid", givenName, familyName)
                .slug(slug)
                .entrypoint("echo-wrapper")
                .hasDist(true));

        return new ConnectFixture(runtime, slug, packageDir, pidFile);
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

    private static void waitForPidExit(int pid) throws Exception {
        long deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos();
        while (System.nanoTime() < deadline) {
            if (ProcessHandle.of(pid).isEmpty() || !ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false)) {
                return;
            }
            Thread.sleep(25);
        }
        throw new AssertionError("timed out waiting for process exit: " + pid);
    }

    private record ConnectFixture(RuntimeFixture runtime, String slug, Path packageDir, Path pidFile) {
    }
}

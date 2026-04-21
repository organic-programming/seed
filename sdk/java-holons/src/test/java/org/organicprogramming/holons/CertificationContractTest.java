package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class CertificationContractTest {

    @Test
    void echoClientScriptIsRunnableAndTargetsSdkHelper() throws IOException {
        Path script = projectRoot().resolve("bin/echo-client");
        assertTrue(Files.isRegularFile(script));
        assertTrue(Files.isExecutable(script));

        String content = Files.readString(script, StandardCharsets.UTF_8);
        assertTrue(content.contains("cmd/echo-client-go/main.go"));
        assertTrue(content.contains("SDK_DIR"));
        assertTrue(content.contains("--sdk java-holons"));
        assertTrue(content.contains("--server-sdk go-holons"));
    }

    @Test
    void echoServerScriptIsRunnable() throws IOException {
        Path script = projectRoot().resolve("bin/echo-server");
        assertTrue(Files.isRegularFile(script));
        assertTrue(Files.isExecutable(script));

        String content = Files.readString(script, StandardCharsets.UTF_8);
        assertTrue(content.contains("cmd/echo-server-go/main.go"));
        assertTrue(content.contains("forward_signal"));
        assertTrue(content.contains("--sdk"));
        assertTrue(content.contains("java-holons"));
    }

    @Test
    void echoServerWrapperPreservesServeArgumentOrder() throws Exception {
        String capture = runWrapperWithFakeGo(
                projectRoot().resolve("bin/echo-server"),
                "serve", "--listen", "stdio://");

        assertTrue(capture.contains("ARG0=run"));
        assertTrue(capture.contains("ARG1=" + projectRoot().resolve("cmd/echo-server-go/main.go")));
        assertTrue(capture.contains("ARG2=serve"));
        assertTrue(capture.contains("ARG3=--listen"));
        assertTrue(capture.contains("ARG4=stdio://"));
        assertTrue(capture.contains("ARG5=--sdk"));
        assertTrue(capture.contains("ARG6=java-holons"));
    }

    @Test
    void echoServerWrapperForwardsSigtermAndExitsZero() throws Exception {
        Path wrapper = projectRoot().resolve("bin/echo-server");
        Path tmp = Files.createTempDirectory("java-holons-signal-wrapper");
        Path fakeLog = tmp.resolve("fake-go.log");
        Path fakeGo = tmp.resolve("go");

        Files.writeString(fakeGo, """
                #!/usr/bin/env bash
                set -euo pipefail
                : "${FAKE_GO_LOG:?FAKE_GO_LOG is required}"
                printf 'CWD=%s\\n' "${PWD}" > "${FAKE_GO_LOG}"
                index=0
                for arg in "$@"; do
                  printf 'ARG%d=%s\\n' "${index}" "${arg}" >> "${FAKE_GO_LOG}"
                  index=$((index + 1))
                done

                child() {
                  trap 'printf "CHILD_TERM=1\\n" >> "${FAKE_GO_LOG}"; exit 0' TERM INT
                  printf 'CHILD_READY=1\\n' >> "${FAKE_GO_LOG}"
                  while true; do
                    sleep 0.1
                  done
                }

                if [[ "${1:-}" == "run" ]]; then
                  child &
                  child_pid=$!
                  printf 'GO_PID=%d\\n' "$$" >> "${FAKE_GO_LOG}"
                  printf 'CHILD_PID=%d\\n' "${child_pid}" >> "${FAKE_GO_LOG}"
                  wait "${child_pid}"
                fi
                """, StandardCharsets.UTF_8);
        fakeGo.toFile().setExecutable(true);

        ProcessBuilder pb = new ProcessBuilder(wrapper.toString(), "--listen", "tcp://127.0.0.1:0");
        pb.directory(projectRoot().toFile());
        pb.environment().put("GO_BIN", fakeGo.toString());
        pb.environment().put("FAKE_GO_LOG", fakeLog.toString());
        pb.redirectErrorStream(true);
        Process process = pb.start();

        for (int i = 0; i < 40; i++) {
            if (Files.exists(fakeLog)) {
                String capture = Files.readString(fakeLog, StandardCharsets.UTF_8);
                if (capture.contains("CHILD_READY=1")) {
                    break;
                }
            }
            Thread.sleep(50);
        }

        process.destroy();
        if (!process.waitFor(10, TimeUnit.SECONDS)) {
            process.destroyForcibly();
            process.waitFor(5, TimeUnit.SECONDS);
            throw new IllegalStateException("echo-server wrapper did not exit on SIGTERM");
        }

        String capture = Files.readString(fakeLog, StandardCharsets.UTF_8);
        assertEquals(0, process.exitValue(), capture);
    }

    @Test
    void holonRpcServerScriptIsRunnableAndTargetsSdkHelper() throws IOException {
        Path script = projectRoot().resolve("bin/holon-rpc-server");
        assertTrue(Files.isRegularFile(script));
        assertTrue(Files.isExecutable(script));

        String content = Files.readString(script, StandardCharsets.UTF_8);
        assertTrue(content.contains("cmd/holon-rpc-server-go/main.go"));
        assertTrue(content.contains("--sdk"));
        assertTrue(content.contains("java-holons"));
    }

    @Test
    void holonRpcServerWrapperForwardsSdkAndUri() throws Exception {
        String capture = runWrapperWithFakeGo(
                projectRoot().resolve("bin/holon-rpc-server"),
                "ws://127.0.0.1:8080/rpc",
                "--once");

        assertTrue(capture.contains("ARG0=run"));
        assertTrue(capture.contains("ARG1=" + projectRoot().resolve("cmd/holon-rpc-server-go/main.go")));
        assertTrue(capture.contains("ARG2=--sdk"));
        assertTrue(capture.contains("ARG3=java-holons"));
        assertTrue(capture.contains("ARG4=ws://127.0.0.1:8080/rpc"));
        assertTrue(capture.contains("ARG5=--once"));
    }

    private static String runWrapperWithFakeGo(Path wrapper, String... args) throws Exception {
        Path tmp = Files.createTempDirectory("java-holons-wrapper");
        Path fakeLog = tmp.resolve("fake-go.log");
        Path fakeGo = tmp.resolve("go");

        Files.writeString(fakeGo, """
                #!/usr/bin/env bash
                set -euo pipefail
                : "${FAKE_GO_LOG:?FAKE_GO_LOG is required}"
                printf 'CWD=%s\\n' "${PWD}" > "${FAKE_GO_LOG}"
                index=0
                for arg in "$@"; do
                  printf 'ARG%d=%s\\n' "${index}" "${arg}" >> "${FAKE_GO_LOG}"
                  index=$((index + 1))
                done
                """, StandardCharsets.UTF_8);
        fakeGo.toFile().setExecutable(true);

        ProcessBuilder pb = new ProcessBuilder(wrapper.toString());
        pb.command().addAll(List.of(args));
        pb.directory(projectRoot().toFile());
        pb.environment().put("GO_BIN", fakeGo.toString());
        pb.environment().put("FAKE_GO_LOG", fakeLog.toString());
        pb.redirectErrorStream(true);
        Process process = pb.start();

        if (!process.waitFor(10, TimeUnit.SECONDS)) {
            process.destroyForcibly();
            process.waitFor(5, TimeUnit.SECONDS);
            throw new IllegalStateException("wrapper did not exit in time: " + wrapper);
        }
        assertEquals(0, process.exitValue());

        return Files.readString(fakeLog, StandardCharsets.UTF_8);
    }

    private static Path projectRoot() {
        return Path.of(System.getProperty("user.dir"));
    }
}

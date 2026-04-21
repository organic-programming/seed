package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class CertificationInteropTest {

    @Test
    void echoClientSupportsWebSocketGrpcDial() throws Exception {
        Path projectRoot = projectRoot();
        Path goHolonsDir = sdkRoot().resolve("go-holons");

        Process goServer = new ProcessBuilder(
                resolveGoBinary(),
                "run",
                "./cmd/echo-server",
                "--listen",
                "ws://127.0.0.1:0/grpc",
                "--sdk",
                "go-holons")
                .directory(goHolonsDir.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader serverStdout = new BufferedReader(
                new InputStreamReader(goServer.getInputStream(), StandardCharsets.UTF_8))) {
            String wsURI = readLineWithTimeout(serverStdout, Duration.ofSeconds(20));
            assertNotNull(wsURI, "go ws echo server did not emit listen URI");
            assertTrue(wsURI.startsWith("ws://"), "unexpected ws URI: " + wsURI);

            Process javaClient = new ProcessBuilder(
                    projectRoot.resolve("bin/echo-client").toString(),
                    "--message",
                    "cert-l3-ws",
                    wsURI)
                    .directory(projectRoot.toFile())
                    .redirectErrorStream(false)
                    .start();

            assertTrue(javaClient.waitFor(30, TimeUnit.SECONDS), "echo-client did not exit");
            String clientStdout = readAll(javaClient.getInputStream());
            String clientStderr = readAll(javaClient.getErrorStream());
            assertEquals(0, javaClient.exitValue(), clientStderr);
            assertTrue(clientStdout.contains("\"status\":\"pass\""), clientStdout);
            assertTrue(clientStdout.contains("\"response_sdk\":\"go-holons\""), clientStdout);
        } finally {
            destroyProcess(goServer);
        }
    }

    @Test
    void holonRpcServerScriptHandlesEchoRoundTrip() throws Exception {
        Path projectRoot = projectRoot();

        Process javaServer = new ProcessBuilder(
                projectRoot.resolve("bin/holon-rpc-server").toString(),
                "--once")
                .directory(projectRoot.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader serverStdout = new BufferedReader(
                new InputStreamReader(javaServer.getInputStream(), StandardCharsets.UTF_8))) {
            String wsURL = readLineWithTimeout(serverStdout, Duration.ofSeconds(20));
            assertNotNull(wsURL, "holon-rpc-server did not emit bind URL");

            HolonRPCClient client = new HolonRPCClient(
                    250, 250, 100, 400,
                    2.0, 0.1, 10_000, 10_000);
            client.connect(wsURL);
            Map<String, Object> out = client.invoke(
                    "echo.v1.Echo/Ping",
                    Map.of("message", "hello"));
            assertEquals("hello", out.get("message"));
            assertEquals("java-holons", out.get("sdk"));
            client.close();

            assertTrue(javaServer.waitFor(20, TimeUnit.SECONDS), "holon-rpc-server did not exit");
            String serverStderr = readAll(javaServer.getErrorStream());
            assertEquals(0, javaServer.exitValue(), serverStderr);
        } finally {
            destroyProcess(javaServer);
        }
    }

    @Test
    void echoServerExitsZeroOnSigterm() throws Exception {
        Path projectRoot = projectRoot();
        Path goHolonsDir = sdkRoot().resolve("go-holons");

        Process javaServer = new ProcessBuilder(
                projectRoot.resolve("bin/echo-server").toString(),
                "--listen", "tcp://127.0.0.1:0")
                .directory(projectRoot.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader serverStdout = new BufferedReader(
                new InputStreamReader(javaServer.getInputStream(), StandardCharsets.UTF_8))) {
            String uri = readLineWithTimeout(serverStdout, Duration.ofSeconds(20));
            assertNotNull(uri, "echo-server did not emit listen URI");

            Process probeClient = new ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    "./cmd/echo-client",
                    "--server-sdk", "java-holons",
                    "--message", "l5-1-probe",
                    uri)
                    .directory(goHolonsDir.toFile())
                    .redirectErrorStream(false)
                    .start();

            assertTrue(probeClient.waitFor(20, TimeUnit.SECONDS), "probe client did not exit");
            String probeErr = readAll(probeClient.getErrorStream());
            assertEquals(0, probeClient.exitValue(), probeErr);

            javaServer.destroy();
            assertTrue(javaServer.waitFor(10, TimeUnit.SECONDS), "echo-server did not exit on SIGTERM");
            assertEquals(0, javaServer.exitValue());
        } finally {
            destroyProcess(javaServer);
        }
    }

    @Test
    void echoServerSleepModePropagatesDeadlineExceeded() throws Exception {
        Path projectRoot = projectRoot();
        Path goHolonsDir = sdkRoot().resolve("go-holons");

        Process javaServer = new ProcessBuilder(
                projectRoot.resolve("bin/echo-server").toString(),
                "--listen", "tcp://127.0.0.1:0",
                "--sleep-ms", "5000")
                .directory(projectRoot.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader serverStdout = new BufferedReader(
                new InputStreamReader(javaServer.getInputStream(), StandardCharsets.UTF_8))) {
            String uri = readLineWithTimeout(serverStdout, Duration.ofSeconds(20));
            assertNotNull(uri, "echo-server did not emit listen URI");

            Process timeoutClient = new ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    "./cmd/echo-client",
                    "--server-sdk", "java-holons",
                    "--message", "timeout-check",
                    "--timeout-ms", "2000",
                    uri)
                    .directory(goHolonsDir.toFile())
                    .redirectErrorStream(false)
                    .start();

            assertTrue(timeoutClient.waitFor(25, TimeUnit.SECONDS), "timeout client did not exit");
            String timeoutOut = readAll(timeoutClient.getInputStream());
            String timeoutErr = readAll(timeoutClient.getErrorStream());
            assertNotEquals(0, timeoutClient.exitValue(), timeoutOut + timeoutErr);
            assertTrue(
                    timeoutErr.contains("DeadlineExceeded")
                            || timeoutErr.contains("deadline exceeded"),
                    timeoutErr);

            Process followupClient = new ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    "./cmd/echo-client",
                    "--server-sdk", "java-holons",
                    "--message", "timeout-followup",
                    "--timeout-ms", "7000",
                    uri)
                    .directory(goHolonsDir.toFile())
                    .redirectErrorStream(false)
                    .start();

            assertTrue(followupClient.waitFor(25, TimeUnit.SECONDS), "follow-up client did not exit");
            String followupOut = readAll(followupClient.getInputStream());
            String followupErr = readAll(followupClient.getErrorStream());
            assertEquals(0, followupClient.exitValue(), followupErr);
            assertTrue(followupOut.contains("\"status\":\"pass\""), followupOut);
        } finally {
            destroyProcess(javaServer);
        }
    }

    @Test
    void echoServerRejectsOversizedMessagesAndStaysHealthy() throws Exception {
        Path projectRoot = projectRoot();
        Path goHolonsDir = sdkRoot().resolve("go-holons");

        Process javaServer = new ProcessBuilder(
                projectRoot.resolve("bin/echo-server").toString(),
                "--listen", "tcp://127.0.0.1:0")
                .directory(projectRoot.toFile())
                .redirectErrorStream(false)
                .start();

        Path probeFile = Files.createTempFile("java-holons-l5-7-", ".go");
        Files.writeString(probeFile, GO_OVERSIZE_PROBE_SOURCE, StandardCharsets.UTF_8);

        try (BufferedReader serverStdout = new BufferedReader(
                new InputStreamReader(javaServer.getInputStream(), StandardCharsets.UTF_8))) {
            String uri = readLineWithTimeout(serverStdout, Duration.ofSeconds(20));
            assertNotNull(uri, "echo-server did not emit listen URI");

            Process probe = new ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    probeFile.toString(),
                    uri)
                    .directory(goHolonsDir.toFile())
                    .redirectErrorStream(false)
                    .start();

            assertTrue(probe.waitFor(30, TimeUnit.SECONDS), "oversize probe did not exit");
            String probeOut = readAll(probe.getInputStream());
            String probeErr = readAll(probe.getErrorStream());

            assertEquals(0, probe.exitValue(), probeOut + probeErr);
            assertTrue(probeOut.contains("RESULT=RESOURCE_EXHAUSTED"), probeOut + probeErr);
            assertTrue(probeOut.contains("SMALL=OK"), probeOut + probeErr);
        } finally {
            Files.deleteIfExists(probeFile);
            destroyProcess(javaServer);
        }
    }

    private static String resolveGoBinary() {
        Path preferred = Path.of("/Users/bpds/go/go1.25.1/bin/go");
        if (Files.isExecutable(preferred)) {
            return preferred.toString();
        }
        return "go";
    }

    private static Path projectRoot() {
        return Path.of(System.getProperty("user.dir"));
    }

    private static Path sdkRoot() {
        Path parent = projectRoot().getParent();
        if (parent == null) {
            throw new IllegalStateException("sdk root not found");
        }
        return parent;
    }

    private static void destroyProcess(Process process) throws InterruptedException {
        if (process == null || !process.isAlive()) {
            return;
        }
        process.destroy();
        if (!process.waitFor(5, TimeUnit.SECONDS)) {
            process.destroyForcibly();
            process.waitFor(5, TimeUnit.SECONDS);
        }
    }

    private static String readAll(InputStream stream) throws IOException {
        return new String(stream.readAllBytes(), StandardCharsets.UTF_8);
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

    private static final String GO_OVERSIZE_PROBE_SOURCE = """
            package main

            import (
              "context"
              "encoding/json"
              "fmt"
              "os"
              "strings"
              "time"

              "google.golang.org/grpc"
              "google.golang.org/grpc/credentials/insecure"
            )

            type PingRequest struct { Message string `json:"message"` }
            type PingResponse struct { Message string `json:"message"`; SDK string `json:"sdk"` }
            type jsonCodec struct{}

            func (jsonCodec) Name() string { return "json" }
            func (jsonCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }
            func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

            func main() {
              if len(os.Args) != 2 {
                fmt.Println("RESULT=BAD_ARGS")
                os.Exit(2)
              }

              target := strings.TrimPrefix(os.Args[1], "tcp://")
              dialCtx, cancelDial := context.WithTimeout(context.Background(), 5*time.Second)
              defer cancelDial()

              conn, err := grpc.DialContext(dialCtx, target,
                grpc.WithTransportCredentials(insecure.NewCredentials()),
                grpc.WithBlock(),
                grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
              )
              if err != nil {
                fmt.Printf("RESULT=DIAL_ERROR err=%v\\n", err)
                os.Exit(1)
              }
              defer conn.Close()

              big := strings.Repeat("x", 2*1024*1024)
              reqCtx, cancelReq := context.WithTimeout(context.Background(), 5*time.Second)
              defer cancelReq()

              var largeOut PingResponse
              err = conn.Invoke(reqCtx, "/echo.v1.Echo/Ping", &PingRequest{Message: big}, &largeOut, grpc.ForceCodec(jsonCodec{}))
              if err != nil {
                low := strings.ToLower(err.Error())
                if strings.Contains(low, "resource_exhausted") || strings.Contains(low, "resourceexhausted") {
                  fmt.Println("RESULT=RESOURCE_EXHAUSTED")
                } else {
                  fmt.Printf("RESULT=ERROR err=%v\\n", err)
                }
              } else {
                fmt.Printf("RESULT=OK RESP_LEN=%d SDK=%s\\n", len(largeOut.Message), largeOut.SDK)
              }

              var smallOut PingResponse
              err = conn.Invoke(context.Background(), "/echo.v1.Echo/Ping", &PingRequest{Message: "ok"}, &smallOut, grpc.ForceCodec(jsonCodec{}))
              if err != nil {
                fmt.Printf("SMALL=ERROR err=%v\\n", err)
                return
              }

              fmt.Printf("SMALL=OK SDK=%s\\n", smallOut.SDK)
            }
            """;
}

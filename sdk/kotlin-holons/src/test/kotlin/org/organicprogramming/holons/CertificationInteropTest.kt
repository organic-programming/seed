package org.organicprogramming.holons

import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.BufferedReader
import java.io.File
import java.io.InputStream
import java.io.InputStreamReader
import java.nio.charset.StandardCharsets
import java.time.Duration
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class CertificationInteropTest {
    @Test fun echoClientSupportsWebSocketGrpcDial() {
        val projectRoot = projectRoot()
        val goHolonsDir = sdkRoot().resolve("go-holons")

        val goServer = ProcessBuilder(
            resolveGoBinary(),
            "run",
            "./cmd/echo-server",
            "--listen",
            "ws://127.0.0.1:0/grpc",
            "--sdk",
            "go-holons",
        )
            .directory(goHolonsDir)
            .redirectErrorStream(false)
            .start()

        try {
            BufferedReader(InputStreamReader(goServer.inputStream, StandardCharsets.UTF_8)).use { serverStdout ->
                val wsURI = readLineWithTimeout(serverStdout, Duration.ofSeconds(20))
                assertNotNull(wsURI, "go ws echo server did not emit listen URI")
                assertTrue(wsURI.startsWith("ws://"), "unexpected ws URI: $wsURI")

                val kotlinClient = ProcessBuilder(
                    projectRoot.resolve("bin/echo-client").absolutePath,
                    "--message",
                    "cert-l3-ws",
                    wsURI,
                )
                    .directory(projectRoot)
                    .redirectErrorStream(false)
                    .start()

                assertTrue(kotlinClient.waitFor(45, TimeUnit.SECONDS), "echo-client did not exit")
                val clientStdout = readAll(kotlinClient.inputStream)
                val clientStderr = readAll(kotlinClient.errorStream)
                assertEquals(0, kotlinClient.exitValue(), clientStderr)
                assertTrue(clientStdout.contains("\"status\":\"pass\""), clientStdout)
                assertTrue(clientStdout.contains("\"response_sdk\":\"go-holons\""), clientStdout)
            }
        } finally {
            destroyProcess(goServer)
        }
    }

    @Test fun echoServerSleepMsPropagatesDeadlineExceeded() {
        val projectRoot = projectRoot()
        val goHolonsDir = sdkRoot().resolve("go-holons")

        val kotlinServer = ProcessBuilder(
            projectRoot.resolve("bin/echo-server").absolutePath,
            "--listen",
            "tcp://127.0.0.1:0",
            "--sleep-ms",
            "5000",
        )
            .directory(projectRoot)
            .redirectErrorStream(false)
            .start()

        try {
            BufferedReader(InputStreamReader(kotlinServer.inputStream, StandardCharsets.UTF_8)).use { serverStdout ->
                val serverURI = readLineWithTimeout(serverStdout, Duration.ofSeconds(20))
                assertNotNull(serverURI, "kotlin echo-server did not emit listen URI")
                assertTrue(serverURI.startsWith("tcp://"), "unexpected server URI: $serverURI")

                val timeoutClient = ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    "./cmd/echo-client",
                    "--sdk",
                    "go-holons",
                    "--server-sdk",
                    "kotlin-holons",
                    "--message",
                    "deadline-probe",
                    "--timeout-ms",
                    "2000",
                    serverURI,
                )
                    .directory(goHolonsDir)
                    .redirectErrorStream(false)
                    .start()

                assertTrue(timeoutClient.waitFor(20, TimeUnit.SECONDS), "deadline probe client did not exit")
                val timeoutStdout = readAll(timeoutClient.inputStream).trim()
                val timeoutStderr = readAll(timeoutClient.errorStream).trim()
                assertTrue(
                    timeoutClient.exitValue() != 0,
                    "expected deadline probe failure\nstdout:\n$timeoutStdout\nstderr:\n$timeoutStderr",
                )
                assertTrue(
                    timeoutStderr.contains("DeadlineExceeded") || timeoutStderr.contains("DEADLINE_EXCEEDED"),
                    "expected deadline exceeded error\nstdout:\n$timeoutStdout\nstderr:\n$timeoutStderr",
                )

                val followupClient = ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    "./cmd/echo-client",
                    "--sdk",
                    "go-holons",
                    "--server-sdk",
                    "kotlin-holons",
                    "--message",
                    "post-timeout",
                    "--timeout-ms",
                    "7000",
                    serverURI,
                )
                    .directory(goHolonsDir)
                    .redirectErrorStream(false)
                    .start()

                assertTrue(followupClient.waitFor(30, TimeUnit.SECONDS), "follow-up client did not exit")
                val followupStdout = readAll(followupClient.inputStream).trim()
                val followupStderr = readAll(followupClient.errorStream).trim()
                assertEquals(
                    0,
                    followupClient.exitValue(),
                    "follow-up request failed\nstdout:\n$followupStdout\nstderr:\n$followupStderr",
                )
                assertTrue(followupStdout.contains("\"status\":\"pass\""), followupStdout)
            }
        } finally {
            destroyProcess(kotlinServer)
        }
    }

    @Test fun echoServerRejectsOversizedMessageAndStaysAlive() {
        val projectRoot = projectRoot()
        val goHolonsDir = sdkRoot().resolve("go-holons")

        val kotlinServer = ProcessBuilder(
            projectRoot.resolve("bin/echo-server").absolutePath,
            "--listen",
            "tcp://127.0.0.1:0",
        )
            .directory(projectRoot)
            .redirectErrorStream(false)
            .start()

        try {
            BufferedReader(InputStreamReader(kotlinServer.inputStream, StandardCharsets.UTF_8)).use { serverStdout ->
                val serverURI = readLineWithTimeout(serverStdout, Duration.ofSeconds(20))
                assertNotNull(serverURI, "kotlin echo-server did not emit listen URI")
                assertTrue(serverURI.startsWith("tcp://"), "unexpected server URI: $serverURI")

                val oversizeProbe = ProcessBuilder(
                    resolveGoBinary(),
                    "run",
                    projectRoot.resolve("cmd/large-ping-go/main.go").absolutePath,
                    serverURI,
                )
                    .directory(goHolonsDir)
                    .redirectErrorStream(false)
                    .start()

                assertTrue(oversizeProbe.waitFor(40, TimeUnit.SECONDS), "oversize probe did not exit")
                val probeStdout = readAll(oversizeProbe.inputStream).trim()
                val probeStderr = readAll(oversizeProbe.errorStream).trim()
                assertEquals(
                    0,
                    oversizeProbe.exitValue(),
                    "oversize probe failed\nstdout:\n$probeStdout\nstderr:\n$probeStderr",
                )
                assertTrue(probeStdout.contains("RESULT=RESOURCE_EXHAUSTED SMALL_OK"), probeStdout)
                assertTrue(probeStdout.contains("SDK=kotlin-holons"), probeStdout)
            }
        } finally {
            destroyProcess(kotlinServer)
        }
    }

    @Test fun holonRpcServerScriptHandlesEchoRoundTrip() = runBlocking {
        val projectRoot = projectRoot()
        val kotlinServer = ProcessBuilder(
            projectRoot.resolve("bin/holon-rpc-server").absolutePath,
        )
            .directory(projectRoot)
            .redirectErrorStream(false)
            .start()

        try {
            BufferedReader(InputStreamReader(kotlinServer.inputStream, StandardCharsets.UTF_8)).use { serverStdout ->
                val wsURL = readLineWithTimeout(serverStdout, Duration.ofSeconds(20))
                if (wsURL == null) {
                    val serverStderr = readAll(kotlinServer.errorStream)
                    val exitState = if (kotlinServer.isAlive) "running" else "exit=${kotlinServer.exitValue()}"
                    throw AssertionError("holon-rpc-server did not emit bind URL ($exitState): $serverStderr")
                }

                val client = HolonRPCClient(
                    heartbeatIntervalMs = 60_000,
                    heartbeatTimeoutMs = 5_000,
                    reconnectMinDelayMs = 100,
                    reconnectMaxDelayMs = 400,
                )
                try {
                    client.connect(wsURL)
                    val out = client.invoke(
                        "echo.v1.Echo/Ping",
                        buildJsonObject {
                            put("message", JsonPrimitive("hello"))
                        },
                    )
                    assertEquals("hello", out["message"]?.jsonPrimitive?.content)
                    assertEquals("kotlin-holons", out["sdk"]?.jsonPrimitive?.content)
                } finally {
                    client.close()
                }

                destroyProcess(kotlinServer)
                assertTrue(!kotlinServer.isAlive, "holon-rpc-server did not stop")
            }
        } finally {
            destroyProcess(kotlinServer)
        }
    }

    private fun resolveGoBinary(): String {
        val preferred = File("/Users/bpds/go/go1.25.1/bin/go")
        if (preferred.canExecute()) {
            return preferred.absolutePath
        }
        return "go"
    }

    private fun projectRoot(): File {
        return File(System.getProperty("user.dir"))
    }

    private fun sdkRoot(): File {
        return projectRoot().parentFile ?: error("sdk root not found")
    }

    private fun destroyProcess(process: Process) {
        if (!process.isAlive) {
            return
        }
        process.destroy()
        if (!process.waitFor(5, TimeUnit.SECONDS)) {
            process.destroyForcibly()
            process.waitFor(5, TimeUnit.SECONDS)
        }
    }

    private fun readAll(stream: InputStream): String {
        return String(stream.readAllBytes(), StandardCharsets.UTF_8)
    }

    private fun readLineWithTimeout(
        reader: BufferedReader,
        timeout: Duration,
    ): String? {
        val executor = Executors.newSingleThreadExecutor()
        try {
            val lineFuture: Future<String?> = executor.submit<String?> { reader.readLine() }
            return lineFuture.get(timeout.toMillis(), TimeUnit.MILLISECONDS)
        } catch (e: ExecutionException) {
            throw AssertionError("failed to read line", e)
        } catch (e: TimeoutException) {
            throw AssertionError("timed out waiting for process output", e)
        } finally {
            executor.shutdownNow()
        }
    }
}

package org.organicprogramming.holons

import io.grpc.CallOptions
import io.grpc.BindableService
import io.grpc.ManagedChannel
import io.grpc.MethodDescriptor
import io.grpc.ServerServiceDefinition
import io.grpc.stub.ClientCalls
import io.grpc.stub.ServerCalls
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.ByteArrayInputStream
import java.io.IOException
import java.io.InputStream
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.util.concurrent.TimeUnit
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ConnectTest {
    private val sdkRoot = Path.of("").toAbsolutePath().normalize()
    private val json = Json
    private val pingMethod = MethodDescriptor.newBuilder<String, String>()
        .setType(MethodDescriptor.MethodType.UNARY)
        .setFullMethodName(MethodDescriptor.generateFullMethodName("echo.v1.Echo", "Ping"))
        .setRequestMarshaller(stringMarshaller())
        .setResponseMarshaller(stringMarshaller())
        .build()

    @Test
    fun connectStartsSlugOverStdioByDefault() = runBlocking {
        val root = Files.createTempDirectory("kotlin-holons-connect-")
        val fixture = createFixture(root, "Connect", "Stdio")

        try {
            withConnectEnvironment(root) {
                val channel = Connect.connect(fixture.slug)
                val pid = waitForPidFile(fixture.pidFile)
                val args = waitForArgsFile(fixture.argsFile)

                try {
                    val out = invokePing(channel, "kotlin-connect-stdio")
                    assertEquals("kotlin-connect-stdio", out["message"]?.jsonPrimitive?.content)
                    assertEquals("kotlin-holons", out["sdk"]?.jsonPrimitive?.content)
                    assertEquals(listOf("serve", "--listen", "stdio://"), args)
                    assertFalse(Files.exists(fixture.portFile))
                } finally {
                    Connect.disconnect(channel)
                }

                waitForPidExit(pid)
            }
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun connectWritesUnixPortFileInPersistentMode() = runBlocking {
        val root = Files.createTempDirectory("kotlin-holons-connect-")
        val fixture = createFixture(root, "Connect", "Unix")

        try {
            withConnectEnvironment(root) {
                val channel = Connect.connect(
                    fixture.slug,
                    ConnectOptions(timeout = Duration.ofSeconds(5), transport = "unix", start = true),
                )
                val pid = waitForPidFile(fixture.pidFile)

                try {
                    val out = invokePing(channel, "kotlin-connect-unix")
                    assertEquals("kotlin-connect-unix", out["message"]?.jsonPrimitive?.content)
                } finally {
                    Connect.disconnect(channel)
                }

                val target = Files.readString(fixture.portFile).trim()
                assertTrue(target.startsWith("unix:///tmp/holons-"))
                assertTrue(ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false))

                val reused = Connect.connect(fixture.slug)
                try {
                    val out = invokePing(reused, "kotlin-connect-unix-reuse")
                    assertEquals("kotlin-connect-unix-reuse", out["message"]?.jsonPrimitive?.content)
                } finally {
                    Connect.disconnect(reused)
                    ProcessHandle.of(pid).ifPresent(ProcessHandle::destroy)
                    waitForPidExit(pid)
                }
            }
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun connectDialsDirectTcpTarget() = runBlocking {
        val running = Serve.startWithOptions(
            "tcp://127.0.0.1:0",
            listOf(echoBindableService()),
            Serve.Options(describe = false),
        )

        val channel = Connect.connect(running.publicUri)
        try {
            val out = invokePing(channel, "kotlin-connect-direct-tcp")
            assertEquals("kotlin-connect-direct-tcp", out["message"]?.jsonPrimitive?.content)
        } finally {
            Connect.disconnect(channel)
            running.stop()
        }
    }

    private fun invokePing(channel: ManagedChannel, message: String) =
        json.parseToJsonElement(
            ClientCalls.blockingUnaryCall(
                channel,
                pingMethod,
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                """{"message":"$message"}""",
            ),
        ).jsonObject

    private fun stringMarshaller() = object : MethodDescriptor.Marshaller<String> {
        override fun stream(value: String): InputStream =
            ByteArrayInputStream(value.toByteArray(StandardCharsets.UTF_8))

        override fun parse(stream: InputStream): String =
            stream.readAllBytes().toString(StandardCharsets.UTF_8)
    }

    private fun echoBindableService(): BindableService =
        object : BindableService {
            override fun bindService(): ServerServiceDefinition =
                ServerServiceDefinition.builder("echo.v1.Echo")
                    .addMethod(
                        pingMethod,
                        ServerCalls.asyncUnaryCall<String, String> { request, observer ->
                            observer.onNext(request)
                            observer.onCompleted()
                        },
                    )
                    .build()
        }

    private fun createFixture(root: Path, givenName: String, familyName: String): ConnectFixture {
        val slug = "$givenName-$familyName".lowercase()
        val holonDir = root.resolve("holons").resolve(slug)
        val binaryDir = holonDir.resolve(".op").resolve("build").resolve("bin")
        Files.createDirectories(binaryDir)

        val wrapper = binaryDir.resolve("echo-wrapper")
        val pidFile = root.resolve("$slug.pid")
        val argsFile = root.resolve("$slug.args")
        val portFile = root.resolve(".op").resolve("run").resolve("$slug.port")

        Files.writeString(
            wrapper,
            """
            #!/usr/bin/env bash
            set -euo pipefail
            printf '%s\n' "${'$'}${'$'}" > '$pidFile'
            : > '$argsFile'
            for arg in "${'$'}@"; do
              printf '%s\n' "${'$'}arg" >> '$argsFile'
            done
            exec '${sdkRoot.resolve("bin").resolve("echo-server")}' "${'$'}@"
            """.trimIndent() + "\n",
        )
        assertTrue(wrapper.toFile().setExecutable(true), "fixture wrapper should be executable")

        Files.createDirectories(holonDir)
        Files.writeString(
            holonDir.resolve("holon.proto"),
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                uuid: "$slug-uuid"
                given_name: "$givenName"
                family_name: "$familyName"
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
            """.trimIndent() + "\n",
        )

        return ConnectFixture(root, slug, pidFile, argsFile, portFile)
    }

    private suspend fun withConnectEnvironment(root: Path, block: suspend () -> Unit) {
        val previousUserDir = System.getProperty("user.dir")
        val previousOpPath = System.getProperty("OPPATH")
        val previousOpBin = System.getProperty("OPBIN")

        System.setProperty("user.dir", root.toString())
        System.setProperty("OPPATH", root.resolve(".op-home").toString())
        System.setProperty("OPBIN", root.resolve(".op-bin").toString())

        try {
            block()
        } finally {
            restoreProperty("user.dir", previousUserDir)
            restoreProperty("OPPATH", previousOpPath)
            restoreProperty("OPBIN", previousOpBin)
        }
    }

    private fun restoreProperty(key: String, value: String?) {
        if (value == null) {
            System.clearProperty(key)
        } else {
            System.setProperty(key, value)
        }
    }

    private fun waitForPidFile(path: Path): Long {
        val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
        while (System.nanoTime() < deadline) {
            try {
                val pid = Files.readString(path).trim().toLong()
                if (pid > 0) {
                    return pid
                }
            } catch (_: Exception) {
                // Wrapper is still starting.
            }
            Thread.sleep(25)
        }
        error("timed out waiting for pid file $path")
    }

    private fun waitForArgsFile(path: Path): List<String> {
        val deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos()
        while (System.nanoTime() < deadline) {
            try {
                val lines = Files.readAllLines(path, StandardCharsets.UTF_8).filter { it.isNotBlank() }
                if (lines.isNotEmpty()) {
                    return lines
                }
            } catch (_: IOException) {
                // Wrapper is still starting.
            }
            Thread.sleep(25)
        }
        error("timed out waiting for args file $path")
    }

    private fun waitForPidExit(pid: Long) {
        val deadline = System.nanoTime() + Duration.ofSeconds(2).toNanos()
        while (System.nanoTime() < deadline) {
            if (!ProcessHandle.of(pid).map(ProcessHandle::isAlive).orElse(false)) {
                return
            }
            Thread.sleep(25)
        }
        error("process $pid did not exit")
    }

    private data class ConnectFixture(
        val root: Path,
        val slug: String,
        val pidFile: Path,
        val argsFile: Path,
        val portFile: Path,
    )
}

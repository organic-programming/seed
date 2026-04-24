package org.organicprogramming.holons

import holons.v1.Describe as HolonsDescribe
import holons.v1.Observability as ObsProto
import io.grpc.CallOptions
import io.grpc.ManagedChannelBuilder
import io.grpc.stub.ClientCalls
import java.io.BufferedReader
import java.io.InputStream
import java.io.InputStreamReader
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path
import java.time.Duration
import java.util.concurrent.Callable
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import kotlin.io.path.createDirectories
import kotlinx.coroutines.runBlocking
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class ServeTest {
    @Test
    fun startWithOptionsAdvertisesEphemeralTcpAndServesRegisteredDescribe() {
        val root = Files.createTempDirectory("kotlin-holons-serve")
        try {
            writeEchoHolon(root)
            Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")))

            val running = Serve.startWithOptions(
                "tcp://127.0.0.1:0",
                emptyList<io.grpc.BindableService>(),
            )
            val (host, port) = parseTarget(running.publicUri)
            val channel = ManagedChannelBuilder.forAddress(host, port).usePlaintext().build()

            try {
                assertTrue(port > 0)
                val response = ClientCalls.blockingUnaryCall(
                    channel,
                    Describe.describeMethod(),
                    CallOptions.DEFAULT,
                    HolonsDescribe.DescribeRequest.getDefaultInstance(),
                )

                assertEquals("Echo", response.manifest.identity.givenName)
                assertEquals("echo.v1.Echo", response.servicesList.single().name)
            } finally {
                channel.shutdownNow()
                channel.awaitTermination(5, TimeUnit.SECONDS)
                running.stop()
            }
        } finally {
            Describe.useStaticResponse(null)
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun startWithOptionsRegistersObservabilityService() {
        val root = Files.createTempDirectory("kotlin-holons-observe")
        try {
            Describe.useStaticResponse(staticDescribeResponse())
            Observability.reset()
            val registryRoot = root.resolve("runs")
            val env = mapOf(
                "OP_OBS" to "logs,metrics,events",
                "OP_RUN_DIR" to registryRoot.toString(),
                "OP_INSTANCE_UID" to "kotlin-obs-1",
            )
            val running = Serve.startWithOptions(
                "tcp://127.0.0.1:0",
                emptyList<io.grpc.BindableService>(),
                Serve.Options(env = env, onListen = {}),
            )
            val (host, port) = parseTarget(running.publicUri)
            val channel = ManagedChannelBuilder.forAddress(host, port).usePlaintext().build()
            try {
                val inst = Observability.current()
                inst.logger("serve-test").info("serve-log", mapOf("sdk" to "kotlin"))
                inst.counter("serve_requests_total")!!.inc()
                inst.emit(Observability.EventType.INSTANCE_READY)

                val logs = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.logsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.LogsRequest.newBuilder()
                        .setMinLevel(ObsProto.LogLevel.INFO)
                        .build(),
                ).asSequence().toList()
                assertTrue(logs.any { it.message == "serve-log" })

                val metrics = ClientCalls.blockingUnaryCall(
                    channel,
                    Observability.metricsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.MetricsRequest.getDefaultInstance(),
                )
                assertTrue(metrics.samplesList.any { it.name == "serve_requests_total" })

                val events = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.EventsRequest.getDefaultInstance(),
                ).asSequence().toList()
                assertTrue(events.any { it.type == ObsProto.EventType.INSTANCE_READY })

                assertTrue(
                    Files.isRegularFile(
                        registryRoot
                            .resolve(inst.cfg.slug)
                            .resolve("kotlin-obs-1")
                            .resolve("meta.json"),
                    ),
                )
            } finally {
                channel.shutdownNow()
                channel.awaitTermination(5, TimeUnit.SECONDS)
                running.stop()
            }
        } finally {
            Describe.useStaticResponse(null)
            Observability.reset()
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun startWithOptionsAdvertisesUnixAndServesRegisteredDescribe() = runBlocking {
        val root = Files.createTempDirectory("kotlin-holons-serve")
        try {
            writeEchoHolon(root)
            Describe.useStaticResponse(Describe.buildResponse(root.resolve("protos")))

            val socketPath = root.resolve("serve.sock")
            val running = Serve.startWithOptions(
                "unix://$socketPath",
                emptyList<io.grpc.BindableService>(),
            )
            val channel = Connect.connect("unix://$socketPath")

            try {
                val response = ClientCalls.blockingUnaryCall(
                    channel,
                    Describe.describeMethod(),
                    CallOptions.DEFAULT,
                    HolonsDescribe.DescribeRequest.getDefaultInstance(),
                )

                assertEquals("unix://$socketPath", running.publicUri)
                assertEquals("Echo", response.manifest.identity.givenName)
                assertEquals("echo.v1.Echo", response.servicesList.single().name)
            } finally {
                Connect.disconnect(channel)
                running.stop()
            }
        } finally {
            Describe.useStaticResponse(null)
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun startWithOptionsFailsWithoutRegisteredDescribe() {
        val logs = mutableListOf<String>()
        Describe.useStaticResponse(null)

        val error = assertFailsWith<IllegalStateException> {
            Serve.startWithOptions(
                "tcp://127.0.0.1:0",
                emptyList<io.grpc.BindableService>(),
                Serve.Options(logger = logs::add),
            )
        }

        assertEquals(Describe.NO_INCODE_DESCRIPTION_MESSAGE, error.message)
        assertTrue(logs.any { it == "HolonMeta registration failed: ${Describe.NO_INCODE_DESCRIPTION_MESSAGE}" })
    }

    @Test
    fun spawnedJvmServesStaticDescribeWithoutAdjacentProtoFiles() {
        val workDir = Files.createTempDirectory("kotlin-holons-static-only-")
        val process = ProcessBuilder(
            Path.of(System.getProperty("java.home"), "bin", "java").toString(),
            "-cp",
            System.getProperty("java.class.path"),
            "org.organicprogramming.holons.StaticDescribeServerMain",
            "tcp://127.0.0.1:0",
        )
            .directory(workDir.toFile())
            .redirectErrorStream(true)
            .start()

        try {
            assertTrue(!Files.exists(workDir.resolve("holon.proto")))

            BufferedReader(InputStreamReader(process.inputStream, StandardCharsets.UTF_8)).use { reader ->
                val publicUri = readUriWithTimeout(reader, Duration.ofSeconds(20))
                assertNotNull(publicUri, "static describe server did not advertise a listen URI")

                val (host, port) = parseTarget(publicUri)
                val channel = ManagedChannelBuilder.forAddress(host, port).usePlaintext().build()

                try {
                    val response = ClientCalls.blockingUnaryCall(
                        channel,
                        Describe.describeMethod(),
                        CallOptions.DEFAULT,
                        HolonsDescribe.DescribeRequest.getDefaultInstance(),
                    )

                    assertEquals("Static", response.manifest.identity.givenName)
                    assertEquals("Only", response.manifest.identity.familyName)
                    assertEquals("Registered at startup.", response.manifest.identity.motto)
                    assertEquals("echo.v1.Echo", response.servicesList.single().name)
                } finally {
                    channel.shutdownNow()
                    channel.awaitTermination(5, TimeUnit.SECONDS)
                }
            }
        } finally {
            destroyProcess(process)
            workDir.toFile().deleteRecursively()
        }
    }

    private fun parseTarget(uri: String): Pair<String, Int> {
        require(uri.startsWith("tcp://")) { "unexpected uri: $uri" }
        val target = uri.removePrefix("tcp://")
        val idx = target.lastIndexOf(':')
        return target.substring(0, idx) to target.substring(idx + 1).toInt()
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

    private fun readUriWithTimeout(reader: BufferedReader, timeout: Duration): String? {
        val executor = Executors.newSingleThreadExecutor()
        try {
            val future: Future<String?> = executor.submit(Callable<String?> {
                while (true) {
                    val line = reader.readLine() ?: return@Callable null
                    if (line.startsWith("tcp://")) {
                        return@Callable line
                    }
                }
                null
            })
            return future.get(timeout.toMillis(), TimeUnit.MILLISECONDS)
        } finally {
            executor.shutdownNow()
        }
    }

    private fun staticDescribeResponse(): HolonsDescribe.DescribeResponse =
        HolonsDescribe.DescribeResponse.newBuilder()
            .setManifest(
                holons.v1.Manifest.HolonManifest.newBuilder()
                    .setIdentity(
                        holons.v1.Manifest.HolonManifest.Identity.newBuilder()
                            .setUuid("serve-observe-0001")
                            .setGivenName("Serve")
                            .setFamilyName("Observability")
                            .build(),
                    )
                    .setLang("kotlin")
                    .build(),
            )
            .build()

    private fun writeEchoHolon(root: Path) {
        val protoDir = root.resolve("protos/echo/v1")
        protoDir.createDirectories()

        Files.writeString(
            root.resolve("holon.proto"),
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                given_name: "Echo"
                family_name: "Server"
                motto: "Reply precisely."
              }
            };
            """.trimIndent(),
        )

        Files.writeString(
            protoDir.resolve("echo.proto"),
            """
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
            """.trimIndent(),
        )
    }
}

package org.organicprogramming.holons

import holons.v1.Observability as ObsProto
import io.grpc.CallOptions
import io.grpc.ManagedChannelBuilder
import io.grpc.ServerBuilder
import io.grpc.stub.ClientCalls
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.TimeUnit
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ObservabilityTest {
    @Test
    fun parseOpObsRejectsV2Tokens() {
        val all = setOf(
            Observability.Family.LOGS,
            Observability.Family.METRICS,
            Observability.Family.EVENTS,
            Observability.Family.PROM,
        )
        assertEquals(all, Observability.parseOpObs("all"))
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.parseOpObs("all,otel")
        }
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.parseOpObs("all,sessions")
        }
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.parseOpObs("unknown")
        }
    }

    @Test
    fun checkEnvRejectsV2TokensAndOpSessions() {
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_OBS" to "logs,otel"))
        }
        assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_OBS" to "logs,sessions"))
        }
        val err = assertFailsWith<Observability.InvalidTokenException> {
            Observability.checkEnv(mapOf("OP_SESSIONS" to "metrics"))
        }
        assertEquals("OP_SESSIONS", err.variable)
    }

    @Test
    fun runDirDerivesFromRegistryRoot() {
        Observability.reset()
        val root = Files.createTempDirectory("kotlin-obs-root")
        try {
            val inst = Observability.configureFromEnv(
                Observability.Config(
                    slug = "gabriel-greeting-kotlin",
                    instanceUid = "uid-1",
                    runDir = root.toString(),
                ),
                mapOf("OP_OBS" to "logs"),
            )

            assertEquals(root.resolve("gabriel-greeting-kotlin").resolve("uid-1").toString(), inst.cfg.runDir)
        } finally {
            Observability.reset()
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun serviceReplaysLogsMetricsEventsAndDiskFiles() {
        Observability.reset()
        val root = Files.createTempDirectory("kotlin-obs-service")
        try {
            val inst = Observability.configureFromEnv(
                Observability.Config(
                    slug = "gabriel-greeting-kotlin",
                    instanceUid = "uid-2",
                    runDir = root.toString(),
                ),
                mapOf("OP_OBS" to "logs,metrics,events"),
            )
            Observability.enableDiskWriters(inst.cfg.runDir)
            inst.logger("test").info("service-log", mapOf("component" to "kotlin"))
            inst.counter("kotlin_requests_total")!!.inc()
            inst.emit(Observability.EventType.INSTANCE_READY, mapOf("listener" to "tcp://127.0.0.1:1"))
            Observability.writeMetaJson(
                inst.cfg.runDir,
                Observability.MetaJson(
                    slug = inst.cfg.slug,
                    uid = inst.cfg.instanceUid,
                    pid = 123,
                    transport = "tcp",
                    address = "tcp://127.0.0.1:1",
                    logPath = Path.of(inst.cfg.runDir, "stdout.log").toString(),
                ),
            )

            val runDir = root.resolve("gabriel-greeting-kotlin").resolve("uid-2")
            assertTrue(Files.readString(runDir.resolve("stdout.log")).contains("\"message\":\"service-log\""))
            assertTrue(Files.readString(runDir.resolve("events.jsonl")).contains("\"type\":\"INSTANCE_READY\""))
            assertTrue(Files.readString(runDir.resolve("meta.json")).contains("\"uid\":\"uid-2\""))

            val server = ServerBuilder.forPort(0)
                .addService(Observability.service(inst))
                .build()
                .start()
            val channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.port).usePlaintext().build()
            try {
                val logs = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.logsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.LogsRequest.newBuilder()
                        .setMinLevel(ObsProto.LogLevel.INFO)
                        .build(),
                ).asSequence().toList()
                assertTrue(logs.any { it.message == "service-log" })

                val metrics = ClientCalls.blockingUnaryCall(
                    channel,
                    Observability.metricsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.MetricsRequest.getDefaultInstance(),
                )
                assertTrue(metrics.samplesList.any { it.name == "kotlin_requests_total" })
                assertFalse(metrics.hasSessionRollup())

                val events = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.EventsRequest.getDefaultInstance(),
                ).asSequence().toList()
                assertTrue(events.any { it.type == ObsProto.EventType.INSTANCE_READY })
            } finally {
                channel.shutdownNow()
                channel.awaitTermination(5, TimeUnit.SECONDS)
                server.shutdownNow()
                server.awaitTermination(5, TimeUnit.SECONDS)
            }
        } finally {
            Observability.reset()
            root.toFile().deleteRecursively()
        }
    }
}

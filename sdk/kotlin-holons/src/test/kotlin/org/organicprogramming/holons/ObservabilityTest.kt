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
            inst.emit(Observability.EventName.INSTANCE_READY, mapOf("listener" to "tcp://127.0.0.1:1"))
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
            assertTrue(Files.readString(runDir.resolve("events.jsonl")).contains("\"event_name\":\"instance.ready\""))
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
                        .setMinSeverityNumber(ObsProto.SeverityNumber.SEVERITY_NUMBER_INFO)
                        .build(),
                ).asSequence().toList()
                assertTrue(logs.any { it.body.stringValue == "service-log" })

                val metrics = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.metricsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.MetricsRequest.getDefaultInstance(),
                ).asSequence().toList()
                assertTrue(metrics.any {
                    it.name == "kotlin_requests_total" &&
                        it.hasSum() &&
                        it.sum.isMonotonic &&
                        it.sum.aggregationTemporality ==
                        ObsProto.AggregationTemporality.AGGREGATION_TEMPORALITY_CUMULATIVE
                })

                val events = ClientCalls.blockingServerStreamingCall(
                    channel,
                    Observability.eventsMethod,
                    CallOptions.DEFAULT,
                    ObsProto.EventsRequest.getDefaultInstance(),
                ).asSequence().toList()
                assertTrue(events.any { it.eventName == Observability.EventName.INSTANCE_READY })
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

    @Test
    fun prometheusTextInjectsIdentityLabelsAndRoundTripsProto() {
        Observability.reset()
        try {
            val inst = Observability.configureFromEnv(
                Observability.Config(
                    slug = "cascade-node-kotlin",
                    instanceUid = "kotlin-prom-1",
                ),
                mapOf("OP_OBS" to "logs,metrics,events,prom"),
            )
            inst.counter(
                "cascade_ticks_total",
                "Ticks received by this cascade node.",
                mapOf("responder_uid" to "kotlin-prom-1"),
            )!!.inc()

            val text = Observability.toPrometheusText(inst)
            assertTrue(text.contains("# HELP cascade_ticks_total Ticks received by this cascade node."))
            assertTrue(text.contains("# TYPE cascade_ticks_total counter"))
            assertTrue(text.contains("instance_uid=\"kotlin-prom-1\""))
            assertTrue(text.contains("responder_uid=\"kotlin-prom-1\""))
            assertTrue(text.contains("slug=\"cascade-node-kotlin\""))

            val log = Observability.LogRecord(
                timestamp = java.time.Instant.now(),
                level = Observability.Level.INFO,
                slug = "child",
                instanceUid = "uid-child",
                message = "tick received",
                fields = mapOf("sender" to "test"),
                chain = listOf("leaf"),
            )
            val roundTrip = Observability.fromProtoLogRecord(Observability.toProtoLogRecord(log))
            assertEquals("tick received", roundTrip.message)
            assertEquals("leaf", roundTrip.chain.single())
        } finally {
            Observability.reset()
        }
    }

    @Test
    fun testLogsFollowReplaysRingOnSubscribe() {
        Observability.reset()
        val inst = Observability.configureFromEnv(
            Observability.Config(slug = "kotlin-log-replay", instanceUid = "log-replay-1"),
            mapOf("OP_OBS" to "logs"),
        )
        inst.logger("test").info("before-subscribe")

        val server = ServerBuilder.forPort(0)
            .addService(Observability.service(inst))
            .build()
            .start()
        val channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.port).usePlaintext().build()
        try {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.logsMethod,
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                ObsProto.LogsRequest.newBuilder()
                    .setFollow(true)
                    .setMinSeverityNumber(ObsProto.SeverityNumber.SEVERITY_NUMBER_INFO)
                    .build(),
            )

            assertTrue(iterator.hasNext())
            assertEquals("before-subscribe", iterator.next().body.stringValue)
            inst.logger("test").info("after-subscribe")
            assertTrue(iterator.hasNext())
            assertEquals("after-subscribe", iterator.next().body.stringValue)
        } finally {
            channel.shutdownNow()
            channel.awaitTermination(5, TimeUnit.SECONDS)
            server.shutdownNow()
            server.awaitTermination(5, TimeUnit.SECONDS)
            Observability.reset()
        }
    }

    @Test
    fun testEventsFollowReplaysRingOnSubscribe() {
        Observability.reset()
        val inst = Observability.configureFromEnv(
            Observability.Config(slug = "kotlin-event-replay", instanceUid = "event-replay-1"),
            mapOf("OP_OBS" to "events"),
        )
        inst.emit(Observability.EventName.INSTANCE_READY, mapOf("listener" to "tcp://127.0.0.1:1"))

        val server = ServerBuilder.forPort(0)
            .addService(Observability.service(inst))
            .build()
            .start()
        val channel = ManagedChannelBuilder.forAddress("127.0.0.1", server.port).usePlaintext().build()
        try {
            val iterator = ClientCalls.blockingServerStreamingCall(
                channel,
                Observability.eventsMethod,
                CallOptions.DEFAULT.withDeadlineAfter(2, TimeUnit.SECONDS),
                ObsProto.EventsRequest.newBuilder()
                    .setFollow(true)
                    .build(),
            )

            assertTrue(iterator.hasNext())
            assertEquals(Observability.EventName.INSTANCE_READY, iterator.next().eventName)
            inst.emit(Observability.EventName.INSTANCE_EXITED, mapOf("listener" to "tcp://127.0.0.1:1"))
            assertTrue(iterator.hasNext())
            assertEquals(Observability.EventName.INSTANCE_EXITED, iterator.next().eventName)
        } finally {
            channel.shutdownNow()
            channel.awaitTermination(5, TimeUnit.SECONDS)
            server.shutdownNow()
            server.awaitTermination(5, TimeUnit.SECONDS)
            Observability.reset()
        }
    }
}

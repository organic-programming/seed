package org.organicprogramming.gabriel.greeting.kotlinholon.internal

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import io.grpc.inprocess.InProcessChannelBuilder
import io.grpc.inprocess.InProcessServerBuilder
import kotlinx.coroutines.test.runTest
import org.organicprogramming.holons.Observability
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

class GreetingServerTest {
    @Test
    fun listLanguagesReturnsAllLanguages() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance())
                assertEquals(56, response.languagesCount)
                assertFalse(response.languagesList.any { it.code.isEmpty() || it.name.isEmpty() || it.native.isEmpty() })
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloUsesRequestedLanguage() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("fr")
                        .build(),
                )
                assertEquals("Bonjour Bob", response.greeting)
                assertEquals("French", response.language)
                assertEquals("fr", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloUsesLocalizedDefaultName() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setLangCode("fr")
                        .build(),
                )
                assertEquals("Bonjour Marie", response.greeting)
                assertEquals("fr", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloFallsBackToEnglish() = runTest {
        val serverName = UUID.randomUUID().toString()
        val server = InProcessServerBuilder.forName(serverName)
            .directExecutor()
            .addService(GreetingServer())
            .build()
            .start()
        try {
            val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
            try {
                val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                val response = stub.sayHello(
                    Greeting.SayHelloRequest.newBuilder()
                        .setName("Bob")
                        .setLangCode("xx")
                        .build(),
                )
                assertEquals("Hello Bob", response.greeting)
                assertEquals("en", response.langCode)
            } finally {
                channel.shutdownNow()
            }
        } finally {
            server.shutdownNow()
        }
    }

    @Test
    fun sayHelloEmitsGreetingObservability() = runTest {
        Observability.reset()
        val obs = Observability.configureFromEnv(
            Observability.Config(
                slug = "gabriel-greeting-kotlin-test",
                instanceUid = "gabriel-greeting-kotlin-test-1",
            ),
            mapOf("OP_OBS" to "logs,metrics"),
        )
        try {
            val serverName = UUID.randomUUID().toString()
            val server = InProcessServerBuilder.forName(serverName)
                .directExecutor()
                .addService(GreetingServer())
                .build()
                .start()
            try {
                val channel = InProcessChannelBuilder.forName(serverName).directExecutor().build()
                try {
                    val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
                    val response = stub.sayHello(
                        Greeting.SayHelloRequest.newBuilder()
                            .setName("Bob")
                            .setLangCode("fr")
                            .build(),
                    )
                    assertEquals("Bonjour Bob", response.greeting)
                } finally {
                    channel.shutdownNow()
                }
            } finally {
                server.shutdownNow()
            }

            val entry = obs.logRing?.drain()
                ?.singleOrNull { it.message == "Greeted Bob in French (fr)" }
            assertNotNull(entry)
            assertEquals(setOf("lang_code", "language", "name", "greeting", "transport", "duration_ns"), entry.fields.keys)
            assertEquals("fr", entry.fields["lang_code"])
            assertEquals("French", entry.fields["language"])
            assertEquals("Bob", entry.fields["name"])
            assertEquals("Bonjour Bob", entry.fields["greeting"])
            assertEquals("unknown", entry.fields["transport"])
            assertTrue(entry.fields.getValue("duration_ns").toLong() >= 0)

            val counter = obs.registry?.counters()
                ?.singleOrNull { it.name == "greeting_emitted_total" }
            assertNotNull(counter)
            assertEquals(1, counter.value())
            assertEquals(setOf("lang_code", "language", "transport"), counter.labels.keys)
            assertEquals(mapOf("lang_code" to "fr", "language" to "French", "transport" to "unknown"), counter.labels)
        } finally {
            Observability.reset()
        }
    }
}

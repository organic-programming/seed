package org.organicprogramming.gabriel.greeting.kotlincompose.controller

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.organicprogramming.gabriel.greeting.kotlincompose.model.AppPlatformCapabilities
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeGreetingHolonConnection
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeHolonCatalog
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeHolonConnector
import org.organicprogramming.gabriel.greeting.kotlincompose.support.holon
import org.organicprogramming.gabriel.greeting.kotlincompose.support.language

class GreetingControllerTest {
    @Test
    fun initializesWithPreferredHolonAndGreetsEnglish() = runBlocking {
        val swiftConnection = FakeGreetingHolonConnection(
            languages = listOf(
                language("en", "English", "English"),
                language("fr", "French", "Francais"),
            ),
            greetingBuilder = { name, _ -> "Hello $name from Swift" },
        )
        val connector = FakeHolonConnector(
            mapOf(
                "gabriel-greeting-go" to { _ ->
                    FakeGreetingHolonConnection(
                        languages = listOf(language("en", "English", "English")),
                        greetingBuilder = { name, _ -> "Hello $name from Go" },
                    )
                },
                "gabriel-greeting-swift" to { _ -> swiftConnection },
            ),
        )
        val controller = GreetingController(
            catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-go"), holon("gabriel-greeting-swift"))),
            connector = connector,
            initialTransport = "stdio",
        )

        controller.initialize()

        assertEquals("gabriel-greeting-swift", controller.state.value.selectedHolon?.slug)
        assertEquals("en", controller.state.value.selectedLanguageCode)
        assertEquals("Hello World from Swift", controller.state.value.greeting)
        assertEquals(listOf("gabriel-greeting-swift" to "stdio"), connector.connectCalls)
        assertEquals(listOf("World" to "en"), swiftConnection.sayHelloCalls)
    }

    @Test
    fun changingTheUserNameRefreshesTheGreeting() = runBlocking {
        val connection = FakeGreetingHolonConnection(
            languages = listOf(language("en", "English", "English")),
            greetingBuilder = { name, _ -> "Hello $name" },
        )
        val controller = GreetingController(
            catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-swift"))),
            connector = FakeHolonConnector(mapOf("gabriel-greeting-swift" to { _ -> connection })),
        )

        controller.initialize()
        controller.setUserName("Alice")

        assertEquals("Hello Alice", controller.state.value.greeting)
        assertEquals("Alice" to "en", connection.sayHelloCalls.last())
    }

    @Test
    fun rejectsUnixTransportWhenPlatformCapabilityDisablesIt() {
        runBlocking {
            val controller = GreetingController(
                catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-swift"))),
                connector = FakeHolonConnector(
                    mapOf(
                        "gabriel-greeting-swift" to { _ ->
                            FakeGreetingHolonConnection(
                                languages = listOf(language("en", "English", "English")),
                                greetingBuilder = { name, _ -> "Hello $name" },
                            )
                        },
                    ),
                ),
                capabilities = AppPlatformCapabilities(supportsUnixSockets = false),
            )

            assertThrows(IllegalArgumentException::class.java) {
                runBlocking {
                    controller.setTransport("unix")
                }
            }
        }
    }

    @Test
    fun surfacesConnectorFailuresAsErrors() = runBlocking {
        val controller = GreetingController(
            catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-swift"))),
            connector = FakeHolonConnector(
                mapOf(
                    "gabriel-greeting-swift" to { _ ->
                        FakeGreetingHolonConnection(
                            languages = emptyList(),
                            greetingBuilder = { _, _ -> "ignored" },
                            listLanguagesError = IllegalStateException("boot failure"),
                        )
                    },
                ),
            ),
        )

        controller.initialize()

        assertNull(controller.state.value.connectionError)
        assertTrue(controller.state.value.error?.contains("Failed to load languages") == true)
        assertTrue(controller.state.value.isRunning)
    }

    @Test
    fun transportChangeInvalidatesInflightStart() = runBlocking {
        coroutineScope {
        val stdioDeferred = CompletableDeferred<FakeGreetingHolonConnection>()
        val tcpConnection = FakeGreetingHolonConnection(
            languages = listOf(language("en", "English", "English")),
            greetingBuilder = { name, _ -> "Hello $name" },
        )
        val connectCalls = mutableListOf<Pair<String, String>>()
        val controller = GreetingController(
            catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-swift"))),
            connector = object : org.organicprogramming.gabriel.greeting.kotlincompose.runtime.HolonConnector {
                override suspend fun connect(
                    holon: org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity,
                    transport: String,
                ): org.organicprogramming.gabriel.greeting.kotlincompose.runtime.GreetingHolonConnection {
                    connectCalls += holon.slug to transport
                    return when (transport) {
                        "stdio" -> stdioDeferred.await()
                        "tcp" -> tcpConnection
                        else -> error("unexpected transport $transport")
                    }
                }
            },
            initialTransport = "stdio",
        )

        controller.refreshHolons()
        val firstStart = async { controller.ensureStarted() }
        delay(25)
        controller.setTransport("tcp", reload = false)
        controller.ensureStarted()

        assertEquals(
            listOf(
                "gabriel-greeting-swift" to "stdio",
                "gabriel-greeting-swift" to "tcp",
            ),
            connectCalls,
        )
        assertEquals("tcp", controller.state.value.transport)
        assertTrue(controller.state.value.isRunning)

        stdioDeferred.complete(
            FakeGreetingHolonConnection(
                languages = listOf(language("en", "English", "English")),
                greetingBuilder = { name, _ -> "Hello $name from stdio" },
            ),
        )
        firstStart.await()

        assertEquals("tcp", controller.state.value.transport)
        assertTrue(controller.state.value.isRunning)
        }
    }
}

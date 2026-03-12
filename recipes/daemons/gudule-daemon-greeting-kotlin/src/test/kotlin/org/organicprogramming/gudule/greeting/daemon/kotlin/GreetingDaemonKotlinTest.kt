package org.organicprogramming.gudule.greeting.daemon.kotlin

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import io.grpc.ManagedChannelBuilder
import kotlinx.coroutines.runBlocking
import org.organicprogramming.holons.Serve
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class GreetingDaemonKotlinTest {
    @Test
    fun greetingTableExposes56Languages() {
        assertEquals(56, greetings.size)
    }

    @Test
    fun lookupFallsBackToEnglish() {
        assertEquals("en", lookupGreeting("??").code)
    }

    @Test
    fun serveRoundTripReturnsBonjourForFrench() = runBlocking {
        val root = findRecipeRoot()
        val running = Serve.startWithOptions(
            "tcp://127.0.0.1:0",
            listOf(GreetingServiceImpl()),
            Serve.Options(
                protoDir = root.resolve("protos"),
                holonYamlPath = root.resolve("holon.yaml"),
            ),
        )
        val (host, port) = parseTarget(running.publicUri)
        val channel = ManagedChannelBuilder.forAddress(host, port).usePlaintext().build()

        try {
            val client = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)
            val languages = client.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance())
            val greeting = client.sayHello(
                Greeting.SayHelloRequest.newBuilder()
                    .setLangCode("fr")
                    .setName("Ada")
                    .build()
            )

            assertEquals(56, languages.languagesCount)
            assertEquals("Bonjour, Ada !", greeting.greeting)
        } finally {
            channel.shutdownNow()
            running.stop()
        }
    }

    private fun parseTarget(uri: String): Pair<String, Int> {
        assertTrue(uri.startsWith("tcp://"))
        val target = uri.removePrefix("tcp://")
        val idx = target.lastIndexOf(':')
        return target.substring(0, idx) to target.substring(idx + 1).toInt()
    }
}

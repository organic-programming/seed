package org.organicprogramming.gabriel.greeting.kotlinholon.internal

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import org.organicprogramming.holons.CurrentTransport
import org.organicprogramming.gabriel.greeting.kotlinholon.api.PublicApi
import org.organicprogramming.holons.Observability
import org.organicprogramming.holons.Serve

class GreetingServer : GreetingServiceGrpcKt.GreetingServiceCoroutineImplBase() {
    override suspend fun listLanguages(request: Greeting.ListLanguagesRequest): Greeting.ListLanguagesResponse =
        PublicApi.listLanguages(request)

    override suspend fun sayHello(request: Greeting.SayHelloRequest): Greeting.SayHelloResponse {
        val startedAt = System.nanoTime()
        val response = PublicApi.sayHello(request)
        val name = resolvedName(request, response)
        val transport = currentTransport()
        val durationNs = System.nanoTime() - startedAt
        emitGreeting(response, name, transport, durationNs)
        return response
    }

    private fun resolvedName(
        request: Greeting.SayHelloRequest,
        response: Greeting.SayHelloResponse,
    ): String {
        val name = request.name.trim()
        if (name.isNotEmpty()) {
            return name
        }
        return GREETING_BY_CODE.getValue(response.langCode).defaultName
    }

    private fun currentTransport(): String = CurrentTransport.get().ifEmpty { TRANSPORT_UNKNOWN }

    private fun emitGreeting(
        response: Greeting.SayHelloResponse,
        name: String,
        transport: String,
        durationNs: Long,
    ) {
        val obs = Observability.current()
        val message = "Greeted $name in ${response.language} (${response.langCode})"
        obs.logger("greeting").info(
            message,
            linkedMapOf(
                "lang_code" to response.langCode,
                "language" to response.language,
                "name" to name,
                "greeting" to response.greeting,
                "transport" to transport,
                "duration_ns" to durationNs,
            ),
        )
        obs.counter(
            "greeting_emitted_total",
            "Greetings emitted, partitioned by language and transport.",
            linkedMapOf(
                "lang_code" to response.langCode,
                "language" to response.language,
                "transport" to transport,
            ),
        )?.inc()
    }

    companion object {
        // Kotlin Serve does not yet expose a handler-visible current transport.
        private const val TRANSPORT_UNKNOWN = "unknown"

        fun listenAndServe(listenUri: String, reflect: Boolean) {
            Serve.runWithOptions(
                listenUri,
                listOf(GreetingServer()),
                Serve.Options(reflect = reflect, slug = "gabriel-greeting-kotlin"),
            )
        }
    }
}

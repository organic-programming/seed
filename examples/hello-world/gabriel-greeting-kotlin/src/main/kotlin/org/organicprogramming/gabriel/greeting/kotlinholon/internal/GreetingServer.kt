package org.organicprogramming.gabriel.greeting.kotlinholon.internal

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import org.organicprogramming.gabriel.greeting.kotlinholon.api.PublicApi
import org.organicprogramming.holons.Serve

class GreetingServer : GreetingServiceGrpcKt.GreetingServiceCoroutineImplBase() {
    override suspend fun listLanguages(request: Greeting.ListLanguagesRequest): Greeting.ListLanguagesResponse =
        PublicApi.listLanguages(request)

    override suspend fun sayHello(request: Greeting.SayHelloRequest): Greeting.SayHelloResponse =
        PublicApi.sayHello(request)

    companion object {
        fun listenAndServe(listenUri: String, reflect: Boolean) {
            Serve.runWithOptions(listenUri, listOf(GreetingServer()), Serve.Options(reflect = reflect))
        }
    }
}

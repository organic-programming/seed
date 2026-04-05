package org.organicprogramming.gabriel.greeting.kotlincompose.runtime

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpc
import io.grpc.CallOptions
import io.grpc.ManagedChannel
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity
import org.organicprogramming.gabriel.greeting.kotlincompose.model.LanguageOption
import org.organicprogramming.gabriel.greeting.kotlincompose.model.normalizedTransportSelection
import org.organicprogramming.holons.Connect
import org.organicprogramming.holons.ConnectOptions
import java.time.Duration
import java.util.concurrent.TimeUnit

interface GreetingHolonConnection {
    suspend fun listLanguages(): List<LanguageOption>
    suspend fun sayHello(name: String, langCode: String): String
    suspend fun close()
}

interface HolonConnector {
    suspend fun connect(
        holon: GabrielHolonIdentity,
        transport: String,
    ): GreetingHolonConnection
}

class DesktopHolonConnector : HolonConnector {
    override suspend fun connect(
        holon: GabrielHolonIdentity,
        transport: String,
    ): GreetingHolonConnection {
        AppPaths.configureRuntimeEnvironment()
        val channel = Connect.connect(
            holon.slug,
            ConnectOptions(
                transport = normalizedTransportSelection(transport),
                timeout = Duration.ofSeconds(5),
            ),
        )
        return DesktopGreetingHolonConnection(channel)
    }
}

class DesktopGreetingHolonConnection(
    private val channel: ManagedChannel,
) : GreetingHolonConnection {
    private val client = GreetingServiceGrpc.newBlockingStub(channel)

    override suspend fun listLanguages(): List<LanguageOption> {
        val response = client
            .withDeadlineAfter(2, TimeUnit.SECONDS)
            .listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance())
        return response.languagesList.map { language ->
            LanguageOption(
                code = language.code,
                name = language.name,
                nativeName = language.native,
            )
        }
    }

    override suspend fun sayHello(name: String, langCode: String): String {
        val response = client
            .withDeadlineAfter(2, TimeUnit.SECONDS)
            .sayHello(
                Greeting.SayHelloRequest.newBuilder()
                    .setName(name)
                    .setLangCode(langCode)
                    .build(),
            )
        return response.greeting
    }

    override suspend fun close() {
        Connect.disconnect(channel)
    }
}

package org.organicprogramming.gabriel.greeting.kotlincompose.support

import greeting.v1.GreetRequest
import greeting.v1.GreetingAppServiceGrpc
import greeting.v1.SelectHolonRequest
import greeting.v1.SelectLanguageRequest
import io.grpc.ManagedChannel
import io.grpc.ManagedChannelBuilder
import kotlinx.coroutines.CompletableDeferred
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity
import org.organicprogramming.gabriel.greeting.kotlincompose.model.LanguageOption
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.GreetingHolonConnection
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.HolonCatalog
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.HolonConnector
import java.net.ServerSocket
import java.net.URI

fun holon(slug: String): GabrielHolonIdentity =
    GabrielHolonIdentity(
        slug = slug,
        familyName = slug,
        binaryName = slug,
        buildRunner = "recipe",
        displayName = GabrielHolonIdentity.displayNameFor(slug),
        sortRank = GabrielHolonIdentity.sortRankFor(slug),
        holonUuid = "uuid-$slug",
        born = "",
        sourceKind = "test",
        discoveryPath = slug,
        hasSource = true,
    )

fun language(code: String, name: String, native: String): LanguageOption =
    LanguageOption(code = code, name = name, nativeName = native)

class FakeHolonCatalog(
    private val holons: List<GabrielHolonIdentity>,
) : HolonCatalog {
    override suspend fun discover(): List<GabrielHolonIdentity> = holons
}

class FakeGreetingHolonConnection(
    private val languages: List<LanguageOption>,
    private val greetingBuilder: (name: String, langCode: String) -> String,
    private val listLanguagesError: Throwable? = null,
) : GreetingHolonConnection {
    val sayHelloCalls = mutableListOf<Pair<String, String>>()
    var closed = false

    override suspend fun listLanguages(): List<LanguageOption> {
        listLanguagesError?.let { throw it }
        return languages
    }

    override suspend fun sayHello(name: String, langCode: String): String {
        sayHelloCalls += name to langCode
        return greetingBuilder(name, langCode)
    }

    override suspend fun close() {
        closed = true
    }
}

class FakeHolonConnector(
    private val factories: Map<String, suspend (String) -> GreetingHolonConnection>,
) : HolonConnector {
    val connectCalls = mutableListOf<Pair<String, String>>()

    override suspend fun connect(holon: GabrielHolonIdentity, transport: String): GreetingHolonConnection {
        connectCalls += holon.slug to transport
        return factories[holon.slug]?.invoke(transport)
            ?: error("missing connector for ${holon.slug}")
    }
}

fun deferredConnector(
    block: suspend (String) -> GreetingHolonConnection,
): HolonConnector = object : HolonConnector {
    val calls = mutableListOf<Pair<String, String>>()

    override suspend fun connect(holon: GabrielHolonIdentity, transport: String): GreetingHolonConnection {
        calls += holon.slug to transport
        return block(transport)
    }
}

fun reserveTcpPort(): Int = ServerSocket(0).use { it.localPort }

fun clientChannelFromListenUri(listenUri: String): ManagedChannel {
    val uri = URI.create(listenUri)
    return ManagedChannelBuilder.forAddress(uri.host, uri.port)
        .usePlaintext()
        .build()
}

package greeting.gokotlin.grpc

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import io.grpc.ManagedChannel
import kotlinx.coroutines.runBlocking
import org.organicprogramming.holons.Connect

data class GreetingLanguage(
    val code: String,
    val name: String,
    val native: String,
)

class GreetingClient(
    private val channel: ManagedChannel,
) {
    private val stub = GreetingServiceGrpcKt.GreetingServiceCoroutineStub(channel)

    suspend fun listLanguages(): List<GreetingLanguage> {
        return stub.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance()).languagesList.map { language ->
            GreetingLanguage(
                code = language.code,
                name = language.name,
                native = language.native,
            )
        }
    }

    suspend fun sayHello(name: String, langCode: String): String {
        val request = Greeting.SayHelloRequest.newBuilder()
            .setName(name)
            .setLangCode(langCode)
            .build()
        return stub.sayHello(request).greeting
    }

    fun close() {
        runBlocking { Connect.disconnect(channel) }
    }
}

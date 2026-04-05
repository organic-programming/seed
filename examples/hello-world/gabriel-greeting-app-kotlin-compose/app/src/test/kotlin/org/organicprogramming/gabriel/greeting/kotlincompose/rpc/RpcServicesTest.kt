package org.organicprogramming.gabriel.greeting.kotlincompose.rpc

import greeting.v1.GreetRequest
import greeting.v1.GreetingAppServiceGrpc
import greeting.v1.SelectHolonRequest
import greeting.v1.SelectLanguageRequest
import holons.v1.CoaxServiceGrpc
import holons.v1.Coax.ListMembersRequest
import holons.v1.Coax.MemberState
import holons.v1.Coax.MemberStatusRequest
import holons.v1.Coax.TellRequest
import holons.v1.Coax.TurnOffCoaxRequest
import io.grpc.StatusRuntimeException
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.CoaxController
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.GreetingController
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.MemorySettingsStore
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeGreetingHolonConnection
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeHolonCatalog
import org.organicprogramming.gabriel.greeting.kotlincompose.support.FakeHolonConnector
import org.organicprogramming.gabriel.greeting.kotlincompose.support.clientChannelFromListenUri
import org.organicprogramming.gabriel.greeting.kotlincompose.support.holon
import org.organicprogramming.gabriel.greeting.kotlincompose.support.language
import org.organicprogramming.gabriel.greeting.kotlincompose.support.reserveTcpPort

class RpcServicesTest {
    @Test
    fun coaxAndGreetingRpcServicesDriveSharedState() = runBlocking {
        val connection = FakeGreetingHolonConnection(
            languages = listOf(
                language("en", "English", "English"),
                language("fr", "French", "Francais"),
            ),
            greetingBuilder = { name, langCode ->
                if (langCode == "fr") "Bonjour $name from Gabriel" else "Hello $name from Gabriel"
            },
        )
        val greetingController = GreetingController(
            catalog = FakeHolonCatalog(listOf(holon("gabriel-greeting-go"), holon("gabriel-greeting-swift"))),
            connector = FakeHolonConnector(
                mapOf(
                    "gabriel-greeting-go" to { _ -> connection },
                    "gabriel-greeting-swift" to { _ -> connection },
                ),
            ),
        )
        val coaxController = CoaxController(
            greetingController = greetingController,
            settingsStore = MemorySettingsStore(),
        )

        greetingController.initialize()
        coaxController.setServerPortText(reserveTcpPort().toString())
        coaxController.setIsEnabled(true)

        val listenUri = requireNotNull(coaxController.state.value.listenUri)
        val channel = clientChannelFromListenUri(listenUri)
        try {
            val coaxClient = CoaxServiceGrpc.newBlockingStub(channel)
            val appClient = GreetingAppServiceGrpc.newBlockingStub(channel)

            val members = coaxClient.listMembers(ListMembersRequest.getDefaultInstance())
            val slugs = members.membersList.map { it.slug }
            assertTrue(slugs.contains("gabriel-greeting-go"))
            assertTrue(slugs.contains("gabriel-greeting-swift"))

            val selectedHolon = appClient.selectHolon(
                SelectHolonRequest.newBuilder().setSlug("gabriel-greeting-go").build(),
            )
            assertEquals("gabriel-greeting-go", selectedHolon.slug)

            val selectedLanguage = appClient.selectLanguage(
                SelectLanguageRequest.newBuilder().setCode("fr").build(),
            )
            assertEquals("fr", selectedLanguage.code)

            val greeting = appClient.greet(
                GreetRequest.newBuilder().setName("Alice").setLangCode("fr").build(),
            )
            assertEquals("Bonjour Alice from Gabriel", greeting.greeting)
            assertEquals("Bonjour Alice from Gabriel", greetingController.state.value.greeting)

            val status = coaxClient.memberStatus(
                MemberStatusRequest.newBuilder().setSlug("gabriel-greeting-go").build(),
            )
            assertEquals(MemberState.MEMBER_STATE_CONNECTED, status.member.state)

            try {
                coaxClient.tell(
                    TellRequest.newBuilder()
                        .setMemberSlug("gabriel-greeting-go")
                        .setMethod("greeting.v1.GreetingService/SayHello")
                        .build(),
                )
            } catch (error: StatusRuntimeException) {
                assertTrue(error.status.code.name.contains("UNIMPLEMENTED"))
            }

            coaxClient.turnOffCoax(TurnOffCoaxRequest.getDefaultInstance())
            delay(200)
            assertTrue(!coaxController.state.value.isEnabled)
        } finally {
            channel.shutdownNow()
            coaxController.shutdown()
            greetingController.shutdown()
        }
    }
}

package org.organicprogramming.gabriel.greeting.kotlincompose.rpc

import greeting.v1.GreetRequest
import greeting.v1.GreetResponse
import greeting.v1.GreetingAppServiceGrpc
import greeting.v1.SelectHolonRequest
import greeting.v1.SelectHolonResponse
import greeting.v1.SelectLanguageRequest
import greeting.v1.SelectLanguageResponse
import io.grpc.Status
import io.grpc.stub.StreamObserver
import kotlinx.coroutines.runBlocking
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.GreetingController

class GreetingAppRpcService(
    private val controller: GreetingController,
) : GreetingAppServiceGrpc.GreetingAppServiceImplBase() {
    override fun selectHolon(
        request: SelectHolonRequest,
        responseObserver: StreamObserver<SelectHolonResponse>,
    ) {
        respond(responseObserver) {
            runBlocking {
                controller.selectHolonBySlug(request.slug)
                val identity = controller.state.value.selectedHolon
                    ?: throw Status.NOT_FOUND.withDescription("Holon '${request.slug}' not found").asRuntimeException()
                SelectHolonResponse.newBuilder()
                    .setSlug(identity.slug)
                    .setDisplayName(identity.displayName)
                    .build()
            }
        }
    }

    override fun selectLanguage(
        request: SelectLanguageRequest,
        responseObserver: StreamObserver<SelectLanguageResponse>,
    ) {
        respond(responseObserver) {
            runBlocking {
                controller.setSelectedLanguage(request.code, greetNow = false)
                SelectLanguageResponse.newBuilder().setCode(request.code).build()
            }
        }
    }

    override fun greet(
        request: GreetRequest,
        responseObserver: StreamObserver<GreetResponse>,
    ) {
        respond(responseObserver) {
            runBlocking {
                if (request.name.isNotBlank()) {
                    controller.setUserName(request.name, greetNow = false)
                }
                if (request.langCode.isNotBlank()) {
                    controller.setSelectedLanguage(request.langCode, greetNow = false)
                }
                if (controller.state.value.selectedLanguageCode.isBlank()) {
                    throw Status.INVALID_ARGUMENT.withDescription("No language selected").asRuntimeException()
                }
                controller.greet(
                    name = request.name.takeIf { it.isNotBlank() },
                    langCode = request.langCode.takeIf { it.isNotBlank() },
                )
                controller.state.value.error?.let { error ->
                    throw Status.UNAVAILABLE.withDescription(error).asRuntimeException()
                }
                GreetResponse.newBuilder()
                    .setGreeting(controller.state.value.greeting)
                    .build()
            }
        }
    }
}

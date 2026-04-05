package org.organicprogramming.gabriel.greeting.kotlincompose.rpc

import holons.v1.Coax.ConnectMemberRequest
import holons.v1.Coax.ConnectMemberResponse
import holons.v1.Coax.DisconnectMemberRequest
import holons.v1.Coax.DisconnectMemberResponse
import holons.v1.Coax.ListMembersRequest
import holons.v1.Coax.ListMembersResponse
import holons.v1.Coax.MemberInfo
import holons.v1.Coax.MemberState
import holons.v1.Coax.MemberStatusRequest
import holons.v1.Coax.MemberStatusResponse
import holons.v1.Coax.TellRequest
import holons.v1.Coax.TellResponse
import holons.v1.Coax.TurnOffCoaxRequest
import holons.v1.Coax.TurnOffCoaxResponse
import holons.v1.CoaxServiceGrpc
import holons.v1.Manifest
import io.grpc.Status
import io.grpc.stub.StreamObserver
import kotlinx.coroutines.runBlocking
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.CoaxController
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.GreetingController
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity

class CoaxRpcService(
    private val greetingController: GreetingController,
    private val coaxController: CoaxController,
) : CoaxServiceGrpc.CoaxServiceImplBase() {
    override fun listMembers(
        request: ListMembersRequest,
        responseObserver: StreamObserver<ListMembersResponse>,
    ) {
        respond(responseObserver) {
            ListMembersResponse.newBuilder()
                .addAllMembers(greetingController.state.value.availableHolons.map(::memberForIdentity))
                .build()
        }
    }

    override fun memberStatus(
        request: MemberStatusRequest,
        responseObserver: StreamObserver<MemberStatusResponse>,
    ) {
        respond(responseObserver) {
            val identity = greetingController.state.value.availableHolons.firstOrNull { it.slug == request.slug }
            MemberStatusResponse.newBuilder().apply {
                if (identity != null) {
                    member = memberForIdentity(identity)
                }
            }.build()
        }
    }

    override fun connectMember(
        request: ConnectMemberRequest,
        responseObserver: StreamObserver<ConnectMemberResponse>,
    ) {
        respond(responseObserver) {
            runBlocking {
                val identity = greetingController.state.value.availableHolons.firstOrNull { it.slug == request.slug }
                    ?: throw Status.NOT_FOUND.withDescription("Member '${request.slug}' not found").asRuntimeException()
                if (request.transport.isNotBlank()) {
                    greetingController.setTransport(request.transport, reload = false)
                }
                greetingController.selectHolonBySlug(identity.slug, reload = false)
                val state = try {
                    greetingController.ensureStarted()
                    if (greetingController.state.value.isRunning) MemberState.MEMBER_STATE_CONNECTED else MemberState.MEMBER_STATE_ERROR
                } catch (_: Throwable) {
                    MemberState.MEMBER_STATE_ERROR
                }
                ConnectMemberResponse.newBuilder()
                    .setMember(memberForIdentity(identity, state))
                    .build()
            }
        }
    }

    override fun disconnectMember(
        request: DisconnectMemberRequest,
        responseObserver: StreamObserver<DisconnectMemberResponse>,
    ) {
        respond(responseObserver) {
            runBlocking {
                greetingController.stop()
                DisconnectMemberResponse.getDefaultInstance()
            }
        }
    }

    override fun tell(
        request: TellRequest,
        responseObserver: StreamObserver<TellResponse>,
    ) {
        responseObserver.onError(Status.UNIMPLEMENTED.withDescription("Tell is not yet implemented").asRuntimeException())
    }

    override fun turnOffCoax(
        request: TurnOffCoaxRequest,
        responseObserver: StreamObserver<TurnOffCoaxResponse>,
    ) {
        coaxController.disableAfterRpc()
        responseObserver.onNext(TurnOffCoaxResponse.getDefaultInstance())
        responseObserver.onCompleted()
    }

    private fun memberForIdentity(
        identity: GabrielHolonIdentity,
        overrideState: MemberState? = null,
    ): MemberInfo =
        MemberInfo.newBuilder()
            .setSlug(identity.slug)
            .setIdentity(
                Manifest.HolonManifest.Identity.newBuilder()
                    .setFamilyName(identity.familyName)
                    .setGivenName(identity.displayName)
                    .build(),
            )
            .setState(overrideState ?: memberStateFor(identity))
            .setIsOrganism(false)
            .build()

    private fun memberStateFor(identity: GabrielHolonIdentity): MemberState =
        if (greetingController.state.value.selectedHolon?.slug == identity.slug && greetingController.state.value.isRunning) {
            MemberState.MEMBER_STATE_CONNECTED
        } else {
            MemberState.MEMBER_STATE_AVAILABLE
        }
}

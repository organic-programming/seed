package org.organicprogramming.holons

import io.grpc.ManagedChannel
import io.grpc.stub.StreamObserver
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong
import relay.v1.Relay
import relay.v1.RelayServiceGrpc

/** Canonical observability-cascade RelayService implementation. */
class RelayService(private val downstream: ManagedChannel?) : RelayServiceGrpc.RelayServiceImplBase() {
    private val received = AtomicLong()

    override fun tick(request: Relay.TickRequest, responseObserver: StreamObserver<Relay.TickResponse>) {
        try {
            val count = received.incrementAndGet()
            val obs = Observability.current()
            val slug = responderSlug(obs)
            val uid = obs.cfg.instanceUid
            obs.logger("tick").info(
                "tick received",
                mapOf(
                    "sender" to request.sender,
                    "note" to request.note,
                    "responder_slug" to slug,
                    "responder_uid" to uid,
                ),
            )
            obs.counter(
                "cascade_ticks_total",
                "Ticks received by this cascade node.",
                mapOf("responder_uid" to uid),
            )?.inc()

            val response = Relay.TickResponse.newBuilder()
                .setResponderSlug(slug)
                .setResponderInstanceUid(uid)
            if (downstream != null) {
                val child = RelayServiceGrpc.newBlockingStub(downstream)
                    .withDeadlineAfter(5, TimeUnit.SECONDS)
                    .tick(request)
                response.addAllHops(child.hopsList)
            }
            response.addHops(
                Relay.HopReceipt.newBuilder()
                    .setSlug(slug)
                    .setUid(uid)
                    .setReceived(count)
                    .build(),
            )
            responseObserver.onNext(response.build())
            responseObserver.onCompleted()
        } catch (error: Exception) {
            responseObserver.onError(error)
        }
    }

    private fun responderSlug(obs: Observability.Instance): String =
        obs.cfg.slug.trim().ifEmpty { "observability-cascade-kotlin-node" }
}

package org.organicprogramming.observability.cascade.kotlinholon.internal

import org.organicprogramming.holons.Serve
import org.organicprogramming.observability.cascade.kotlinholon.api.PublicApi
import relay.v1.Relay
import relay.v1.RelayServiceGrpcKt

class RelayServer : RelayServiceGrpcKt.RelayServiceCoroutineImplBase() {
    override suspend fun tick(request: Relay.TickRequest): Relay.TickResponse =
        PublicApi.tick(request)

    companion object {
        fun listenAndServe(listenUri: String, reflect: Boolean, members: List<Serve.MemberRef>) {
            Serve.runWithOptions(
                normalizeListenUri(listenUri),
                listOf(RelayServer()),
                Serve.Options(
                    reflect = reflect,
                    slug = "observability-cascade-node-kotlin",
                    memberEndpoints = members,
                ),
            )
        }

        private fun normalizeListenUri(listenUri: String): String =
            if (listenUri.startsWith("tcp://:")) {
                "tcp://0.0.0.0:${listenUri.removePrefix("tcp://:")}"
            } else {
                listenUri
            }
    }
}

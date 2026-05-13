package org.organicprogramming.relay.cascade.kotlinholon.api

import org.organicprogramming.holons.Observability
import relay.v1.Relay

object PublicApi {
    fun tick(request: Relay.TickRequest): Relay.TickResponse {
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
        return Relay.TickResponse.newBuilder()
            .setResponderSlug(slug)
            .setResponderInstanceUid(uid)
            .build()
    }

    private fun responderSlug(obs: Observability.Instance): String =
        obs.cfg.slug.trim().ifEmpty { "cascade-node-kotlin" }
}

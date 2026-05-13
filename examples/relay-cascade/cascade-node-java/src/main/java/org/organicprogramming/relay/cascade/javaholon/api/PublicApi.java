package org.organicprogramming.relay.cascade.javaholon.api;

import org.organicprogramming.holons.Observability;
import relay.v1.Relay;

import java.util.Map;

public final class PublicApi {
    private PublicApi() {
    }

    public static Relay.TickResponse tick(Relay.TickRequest request) {
        Observability obs = Observability.current();
        String slug = responderSlug(obs);
        String uid = obs.cfg.instanceUid == null ? "" : obs.cfg.instanceUid;
        obs.logger("tick").info("tick received", Map.of(
                "sender", request.getSender(),
                "note", request.getNote(),
                "responder_slug", slug,
                "responder_uid", uid));
        Observability.Counter counter = obs.counter(
                "cascade_ticks_total",
                "Ticks received by this cascade node.",
                Map.of("responder_uid", uid));
        if (counter != null) {
            counter.inc();
        }
        return Relay.TickResponse.newBuilder()
                .setResponderSlug(slug)
                .setResponderInstanceUid(uid)
                .build();
    }

    static String responderSlug(Observability obs) {
        String configured = obs.cfg.slug == null ? "" : obs.cfg.slug.trim();
        return configured.isEmpty() ? "cascade-node-java" : configured;
    }
}

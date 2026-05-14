import Foundation
import Holons

public enum PublicAPI {
    public static func tick(_ request: Relay_V1_TickRequest) -> Relay_V1_TickResponse {
        let obs = current()
        let slug = responderSlug(obs)
        let uid = obs.cfg.instanceUid

        obs.logger("tick").info(
            "tick received",
            [
                "sender": request.sender,
                "note": request.note,
                "responder_slug": slug,
                "responder_uid": uid,
            ]
        )
        obs.counter(
            "cascade_ticks_total",
            help: "Ticks received by this cascade node.",
            labels: ["responder_uid": uid]
        )?.inc()

        var response = Relay_V1_TickResponse()
        response.responderSlug = slug
        response.responderInstanceUid = uid
        return response
    }

    private static func responderSlug(_ obs: Observability) -> String {
        let slug = obs.cfg.slug.trimmingCharacters(in: .whitespacesAndNewlines)
        return slug.isEmpty ? "observability-cascade-swift-node" : slug
    }
}

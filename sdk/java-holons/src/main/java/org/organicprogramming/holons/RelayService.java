package org.organicprogramming.holons;

import io.grpc.ManagedChannel;
import io.grpc.stub.StreamObserver;
import relay.v1.Relay;
import relay.v1.RelayServiceGrpc;

import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;

/** Canonical observability-cascade RelayService implementation. */
public final class RelayService extends RelayServiceGrpc.RelayServiceImplBase {
    private final ManagedChannel downstream;
    private final AtomicLong received = new AtomicLong();

    public RelayService(ManagedChannel downstream) {
        this.downstream = downstream;
    }

    @Override
    public void tick(Relay.TickRequest request, StreamObserver<Relay.TickResponse> responseObserver) {
        try {
            long count = received.incrementAndGet();
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

            Relay.TickResponse.Builder response = Relay.TickResponse.newBuilder()
                    .setResponderSlug(slug)
                    .setResponderInstanceUid(uid);
            if (downstream != null) {
                Relay.TickResponse child = RelayServiceGrpc.newBlockingStub(downstream)
                        .withDeadlineAfter(5, TimeUnit.SECONDS)
                        .tick(request);
                response.addAllHops(child.getHopsList());
            }
            response.addHops(Relay.HopReceipt.newBuilder()
                    .setSlug(slug)
                    .setUid(uid)
                    .setReceived(count)
                    .build());
            responseObserver.onNext(response.build());
            responseObserver.onCompleted();
        } catch (Exception error) {
            responseObserver.onError(error);
        }
    }

    private static String responderSlug(Observability obs) {
        String configured = obs.cfg.slug == null ? "" : obs.cfg.slug.trim();
        return configured.isEmpty() ? "observability-cascade-java-node" : configured;
    }
}

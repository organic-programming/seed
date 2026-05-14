package org.organicprogramming.observability.cascade.javaholon.internal;

import gen.describe_generated;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Serve;
import org.organicprogramming.observability.cascade.javaholon.api.PublicApi;
import relay.v1.Relay;
import relay.v1.RelayServiceGrpc;

import java.util.List;

public final class RelayServer extends RelayServiceGrpc.RelayServiceImplBase {
    @Override
    public void tick(Relay.TickRequest request, StreamObserver<Relay.TickResponse> responseObserver) {
        responseObserver.onNext(PublicApi.tick(request));
        responseObserver.onCompleted();
    }

    public static void listenAndServe(String listenUri, boolean reflect, List<Serve.MemberRef> members) throws Exception {
        Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
        Serve.runWithOptions(
                normalizeListenUri(listenUri),
                List.of(new RelayServer()),
                new Serve.Options()
                        .withReflect(reflect)
                        .withSlug("observability-cascade-node-java")
                        .withMemberEndpoints(members));
    }

    private static String normalizeListenUri(String listenUri) {
        if (listenUri != null && listenUri.startsWith("tcp://:")) {
            return "tcp://0.0.0.0:" + listenUri.substring("tcp://:".length());
        }
        return listenUri;
    }
}

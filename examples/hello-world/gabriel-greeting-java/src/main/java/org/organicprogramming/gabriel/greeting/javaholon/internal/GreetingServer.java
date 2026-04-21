package org.organicprogramming.gabriel.greeting.javaholon.internal;

import gen.describe_generated;
import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.gabriel.greeting.javaholon.api.PublicApi;
import org.organicprogramming.holons.Serve;

import java.util.List;

public final class GreetingServer extends GreetingServiceGrpc.GreetingServiceImplBase {
    @Override
    public void listLanguages(
            Greeting.ListLanguagesRequest request,
            StreamObserver<Greeting.ListLanguagesResponse> responseObserver) {
        responseObserver.onNext(PublicApi.listLanguages(request));
        responseObserver.onCompleted();
    }

    @Override
    public void sayHello(
            Greeting.SayHelloRequest request,
            StreamObserver<Greeting.SayHelloResponse> responseObserver) {
        responseObserver.onNext(PublicApi.sayHello(request));
        responseObserver.onCompleted();
    }

    public static void listenAndServe(String listenUri, boolean reflect) throws Exception {
        Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
        Serve.runWithOptions(
                listenUri,
                List.of(new GreetingServer()),
                new Serve.Options().withReflect(reflect));
    }
}

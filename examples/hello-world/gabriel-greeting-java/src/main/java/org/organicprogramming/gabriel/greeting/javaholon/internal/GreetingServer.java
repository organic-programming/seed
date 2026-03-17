package org.organicprogramming.gabriel.greeting.javaholon.internal;

import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.protobuf.services.ProtoReflectionService;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.gabriel.greeting.javaholon.api.PublicApi;
import org.organicprogramming.holons.Serve;

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

    public static void listenAndServe(String listenUri) throws Exception {
        Serve.run(listenUri, new GreetingServer(), ProtoReflectionService.newInstance());
    }
}

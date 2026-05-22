package org.organicprogramming.gabriel.greeting.javaholon.internal;

import gen.describe_generated;
import greeting.v1.Greeting;
import greeting.v1.GreetingServiceGrpc;
import io.grpc.stub.StreamObserver;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.gabriel.greeting.javaholon.api.PublicApi;
import org.organicprogramming.holons.Observability;
import org.organicprogramming.holons.Serve;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class GreetingServer extends GreetingServiceGrpc.GreetingServiceImplBase {
    private static final String TRANSPORT_UNKNOWN = "unknown";

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
        long startedAt = System.nanoTime();
        Greeting.SayHelloResponse response = PublicApi.sayHello(request);
        String name = resolvedName(request, response);
        String transport = currentTransport();
        long durationNs = System.nanoTime() - startedAt;
        emitGreeting(response, name, transport, durationNs);
        responseObserver.onNext(response);
        responseObserver.onCompleted();
    }

    private static String resolvedName(Greeting.SayHelloRequest request, Greeting.SayHelloResponse response) {
        String name = request.getName().trim();
        if (!name.isEmpty()) {
            return name;
        }
        return GreetingCatalog.lookup(response.getLangCode()).defaultName();
    }

    private static String currentTransport() {
        String transport = Serve.currentTransport();
        return transport == null || transport.isBlank() ? TRANSPORT_UNKNOWN : transport;
    }

    private static void emitGreeting(
            Greeting.SayHelloResponse response,
            String name,
            String transport,
            long durationNs) {
        Observability obs = Observability.current();
        String message = "Greeted " + name + " in " + response.getLanguage() + " (" + response.getLangCode() + ")";

        Map<String, Object> fields = new LinkedHashMap<>();
        fields.put("lang_code", response.getLangCode());
        fields.put("language", response.getLanguage());
        fields.put("name", name);
        fields.put("greeting", response.getGreeting());
        fields.put("transport", transport);
        fields.put("duration_ns", durationNs);
        obs.logger("greeting").info(message, fields);

        Map<String, String> labels = new LinkedHashMap<>();
        labels.put("lang_code", response.getLangCode());
        labels.put("language", response.getLanguage());
        labels.put("transport", transport);
        Observability.Counter counter = obs.counter(
                "greeting_emitted_total",
                "Greetings emitted, partitioned by language and transport.",
                labels);
        if (counter != null) {
            counter.inc();
        }
    }

    public static void listenAndServe(String listenUri, boolean reflect) throws Exception {
        Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
        Serve.runWithOptions(
                listenUri,
                List.of(new GreetingServer()),
                new Serve.Options()
                        .withReflect(reflect)
                        .withSlug("gabriel-greeting-java"));
    }
}

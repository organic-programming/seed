package org.organicprogramming.gabriel.greeting.javaholon.api;

import greeting.v1.Greeting;
import org.organicprogramming.gabriel.greeting.javaholon.internal.GreetingCatalog;
import org.organicprogramming.gabriel.greeting.javaholon.internal.GreetingData;

public final class PublicApi {

    private PublicApi() {
    }

    public static Greeting.ListLanguagesResponse listLanguages(Greeting.ListLanguagesRequest request) {
        request.getClass();
        Greeting.ListLanguagesResponse.Builder response = Greeting.ListLanguagesResponse.newBuilder();
        for (GreetingData greeting : GreetingCatalog.GREETINGS) {
            response.addLanguages(Greeting.Language.newBuilder()
                    .setCode(greeting.langCode())
                    .setName(greeting.langEnglish())
                    .setNative(greeting.langNative())
                    .build());
        }
        return response.build();
    }

    public static Greeting.SayHelloResponse sayHello(Greeting.SayHelloRequest request) {
        GreetingData greeting = GreetingCatalog.lookup(request.getLangCode());
        String name = request.getName().trim();
        if (name.isEmpty()) {
            name = greeting.defaultName();
        }

        return Greeting.SayHelloResponse.newBuilder()
                .setGreeting(greeting.template().replace("%s", name))
                .setLanguage(greeting.langEnglish())
                .setLangCode(greeting.langCode())
                .build();
    }
}

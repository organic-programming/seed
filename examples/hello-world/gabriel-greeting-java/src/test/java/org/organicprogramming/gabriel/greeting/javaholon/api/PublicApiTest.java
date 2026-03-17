package org.organicprogramming.gabriel.greeting.javaholon.api;

import greeting.v1.Greeting;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

class PublicApiTest {
    @Test
    void listLanguagesIncludesEnglish() {
        Greeting.ListLanguagesResponse response = PublicApi.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance());
        Greeting.Language english = response.getLanguagesList().stream()
                .filter(language -> language.getCode().equals("en"))
                .findFirst()
                .orElse(null);

        assertNotNull(english);
        assertEquals("English", english.getName());
        assertEquals("English", english.getNative());
    }

    @Test
    void sayHelloUsesRequestedLanguage() {
        Greeting.SayHelloResponse response = PublicApi.sayHello(Greeting.SayHelloRequest.newBuilder()
                .setName("Alice")
                .setLangCode("fr")
                .build());

        assertEquals("Bonjour Alice", response.getGreeting());
        assertEquals("French", response.getLanguage());
        assertEquals("fr", response.getLangCode());
    }

    @Test
    void sayHelloUsesLocalizedDefaultName() {
        Greeting.SayHelloResponse response = PublicApi.sayHello(Greeting.SayHelloRequest.newBuilder()
                .setLangCode("ja")
                .build());

        assertEquals("こんにちは、マリアさん", response.getGreeting());
        assertEquals("Japanese", response.getLanguage());
        assertEquals("ja", response.getLangCode());
    }

    @Test
    void sayHelloFallsBackToEnglish() {
        Greeting.SayHelloResponse response = PublicApi.sayHello(Greeting.SayHelloRequest.newBuilder()
                .setLangCode("unknown")
                .build());

        assertEquals("Hello Mary", response.getGreeting());
        assertEquals("English", response.getLanguage());
        assertEquals("en", response.getLangCode());
    }
}

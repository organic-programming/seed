package org.organicprogramming.gabriel.greeting.kotlinholon.api

import greeting.v1.Greeting
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

class PublicApiTest {
    @Test
    fun listLanguagesIncludesEnglish() {
        val response = PublicApi.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance())
        val english = response.languagesList.firstOrNull { it.code == "en" }

        assertNotNull(english)
        assertEquals("English", english.name)
        assertEquals("English", english.native)
    }

    @Test
    fun sayHelloUsesRequestedLanguage() {
        val response = PublicApi.sayHello(
            Greeting.SayHelloRequest.newBuilder()
                .setName("Alice")
                .setLangCode("fr")
                .build(),
        )

        assertEquals("Bonjour Alice", response.greeting)
        assertEquals("French", response.language)
        assertEquals("fr", response.langCode)
    }

    @Test
    fun sayHelloUsesLocalizedDefaultName() {
        val response = PublicApi.sayHello(
            Greeting.SayHelloRequest.newBuilder()
                .setLangCode("ja")
                .build(),
        )

        assertEquals("こんにちは、マリアさん", response.greeting)
        assertEquals("Japanese", response.language)
        assertEquals("ja", response.langCode)
    }

    @Test
    fun sayHelloFallsBackToEnglish() {
        val response = PublicApi.sayHello(
            Greeting.SayHelloRequest.newBuilder()
                .setLangCode("unknown")
                .build(),
        )

        assertEquals("Hello Mary", response.greeting)
        assertEquals("English", response.language)
        assertEquals("en", response.langCode)
    }
}

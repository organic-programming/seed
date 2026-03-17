package org.organicprogramming.gabriel.greeting.kotlinholon.api

import greeting.v1.Greeting
import org.organicprogramming.gabriel.greeting.kotlinholon.internal.GREETING_BY_CODE
import org.organicprogramming.gabriel.greeting.kotlinholon.internal.GREETINGS

object PublicApi {
    fun listLanguages(request: Greeting.ListLanguagesRequest): Greeting.ListLanguagesResponse {
        request
        return Greeting.ListLanguagesResponse.newBuilder().apply {
            GREETINGS.forEach { greeting ->
                addLanguages(
                    Greeting.Language.newBuilder()
                        .setCode(greeting.langCode)
                        .setName(greeting.langEnglish)
                        .setNative(greeting.langNative)
                        .build(),
                )
            }
        }.build()
    }

    fun sayHello(request: Greeting.SayHelloRequest): Greeting.SayHelloResponse {
        val greeting = GREETING_BY_CODE[request.langCode] ?: GREETING_BY_CODE.getValue("en")
        val name = request.name.trim().ifEmpty { greeting.defaultName }
        return Greeting.SayHelloResponse.newBuilder()
            .setGreeting(greeting.template.replace("%s", name))
            .setLanguage(greeting.langEnglish)
            .setLangCode(greeting.langCode)
            .build()
    }
}

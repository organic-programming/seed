package org.organicprogramming.gabriel.greeting.javaholon.internal;

public record GreetingData(
        String langCode,
        String langEnglish,
        String langNative,
        String template,
        String defaultName) {
}

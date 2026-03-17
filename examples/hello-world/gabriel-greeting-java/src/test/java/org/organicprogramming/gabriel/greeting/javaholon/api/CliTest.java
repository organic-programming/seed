package org.organicprogramming.gabriel.greeting.javaholon.api;

import com.google.gson.Gson;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayOutputStream;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class CliTest {
    private static final Gson GSON = new Gson();

    @Test
    void runPrintsVersion() {
        ByteArrayOutputStream stdout = new ByteArrayOutputStream();
        ByteArrayOutputStream stderr = new ByteArrayOutputStream();

        int exitCode = Cli.run(new String[] { "version" }, new PrintStream(stdout), new PrintStream(stderr));

        assertEquals(0, exitCode);
        assertEquals(Cli.VERSION, stdout.toString(StandardCharsets.UTF_8).trim());
        assertEquals("", stderr.toString(StandardCharsets.UTF_8));
    }

    @Test
    void runPrintsHelp() {
        ByteArrayOutputStream stdout = new ByteArrayOutputStream();
        ByteArrayOutputStream stderr = new ByteArrayOutputStream();

        int exitCode = Cli.run(new String[] { "help" }, new PrintStream(stdout), new PrintStream(stderr));

        assertEquals(0, exitCode);
        assertTrue(stdout.toString(StandardCharsets.UTF_8).contains("usage: gabriel-greeting-java"));
        assertTrue(stdout.toString(StandardCharsets.UTF_8).contains("listLanguages"));
        assertEquals("", stderr.toString(StandardCharsets.UTF_8));
    }

    @Test
    void runRendersListLanguagesJson() {
        ByteArrayOutputStream stdout = new ByteArrayOutputStream();
        ByteArrayOutputStream stderr = new ByteArrayOutputStream();

        int exitCode = Cli.run(
                new String[] { "listLanguages", "--format", "json" },
                new PrintStream(stdout),
                new PrintStream(stderr));

        Map<?, ?> payload = GSON.fromJson(stdout.toString(StandardCharsets.UTF_8), Map.class);
        List<?> languages = (List<?>) payload.get("languages");

        assertEquals(0, exitCode);
        assertEquals(56, languages.size());
        Map<?, ?> first = (Map<?, ?>) languages.get(0);
        assertEquals("en", first.get("code"));
        assertEquals("English", first.get("name"));
        assertEquals("", stderr.toString(StandardCharsets.UTF_8));
    }

    @Test
    void runRendersSayHelloText() {
        ByteArrayOutputStream stdout = new ByteArrayOutputStream();
        ByteArrayOutputStream stderr = new ByteArrayOutputStream();

        int exitCode = Cli.run(
                new String[] { "sayHello", "Alice", "fr" },
                new PrintStream(stdout),
                new PrintStream(stderr));

        assertEquals(0, exitCode);
        assertEquals("Bonjour Alice", stdout.toString(StandardCharsets.UTF_8).trim());
        assertEquals("", stderr.toString(StandardCharsets.UTF_8));
    }

    @Test
    void runDefaultsSayHelloToEnglishJson() {
        ByteArrayOutputStream stdout = new ByteArrayOutputStream();
        ByteArrayOutputStream stderr = new ByteArrayOutputStream();

        int exitCode = Cli.run(
                new String[] { "sayHello", "--json" },
                new PrintStream(stdout),
                new PrintStream(stderr));

        Map<?, ?> payload = GSON.fromJson(stdout.toString(StandardCharsets.UTF_8), Map.class);

        assertEquals(0, exitCode);
        assertEquals("Hello Mary", payload.get("greeting"));
        assertEquals("English", payload.get("language"));
        assertEquals("en", payload.get("langCode"));
        assertEquals("", stderr.toString(StandardCharsets.UTF_8));
    }
}

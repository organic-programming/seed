package org.organicprogramming.gabriel.greeting.kotlinholon.api

import com.google.gson.Gson
import java.io.ByteArrayOutputStream
import java.io.PrintStream
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CliTest {
    private val gson = Gson()

    @Test
    fun runPrintsVersion() {
        val stdout = ByteArrayOutputStream()
        val stderr = ByteArrayOutputStream()

        val exitCode = Cli.run(arrayOf("version"), PrintStream(stdout), PrintStream(stderr))

        assertEquals(0, exitCode)
        assertEquals(Cli.VERSION, stdout.toString().trim())
        assertEquals("", stderr.toString())
    }

    @Test
    fun runPrintsHelp() {
        val stdout = ByteArrayOutputStream()
        val stderr = ByteArrayOutputStream()

        val exitCode = Cli.run(arrayOf("help"), PrintStream(stdout), PrintStream(stderr))

        assertEquals(0, exitCode)
        assertTrue(stdout.toString().contains("usage: gabriel-greeting-kotlin"))
        assertTrue(stdout.toString().contains("listLanguages"))
        assertEquals("", stderr.toString())
    }

    @Test
    fun runRendersListLanguagesJson() {
        val stdout = ByteArrayOutputStream()
        val stderr = ByteArrayOutputStream()

        val exitCode = Cli.run(arrayOf("listLanguages", "--format", "json"), PrintStream(stdout), PrintStream(stderr))
        val payload = gson.fromJson(stdout.toString(), Map::class.java)
        val languages = payload["languages"] as List<*>
        val first = languages.first() as Map<*, *>

        assertEquals(0, exitCode)
        assertEquals(56, languages.size)
        assertEquals("en", first["code"])
        assertEquals("English", first["name"])
        assertEquals("", stderr.toString())
    }

    @Test
    fun runRendersSayHelloText() {
        val stdout = ByteArrayOutputStream()
        val stderr = ByteArrayOutputStream()

        val exitCode = Cli.run(arrayOf("sayHello", "Alice", "fr"), PrintStream(stdout), PrintStream(stderr))

        assertEquals(0, exitCode)
        assertEquals("Bonjour Alice", stdout.toString().trim())
        assertEquals("", stderr.toString())
    }

    @Test
    fun runDefaultsSayHelloToEnglishJson() {
        val stdout = ByteArrayOutputStream()
        val stderr = ByteArrayOutputStream()

        val exitCode = Cli.run(arrayOf("sayHello", "--json"), PrintStream(stdout), PrintStream(stderr))
        val payload = gson.fromJson(stdout.toString(), Map::class.java)

        assertEquals(0, exitCode)
        assertEquals("Hello Mary", payload["greeting"] as String)
        assertEquals("English", payload["language"] as String)
        assertEquals("en", payload["langCode"] as String)
        assertEquals("", stderr.toString())
    }
}

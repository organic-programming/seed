package org.organicprogramming.gudule.greeting.daemon.kotlin

import greeting.v1.Greeting
import greeting.v1.GreetingServiceGrpcKt
import org.organicprogramming.holons.Serve
import java.io.File
import java.nio.file.Path
import kotlin.io.path.exists
import kotlin.system.exitProcess

class GreetingServiceImpl : GreetingServiceGrpcKt.GreetingServiceCoroutineImplBase() {
    override suspend fun listLanguages(request: Greeting.ListLanguagesRequest): Greeting.ListLanguagesResponse {
        val builder = Greeting.ListLanguagesResponse.newBuilder()
        greetings.forEach { entry ->
            builder.addLanguages(
                Greeting.Language.newBuilder()
                    .setCode(entry.code)
                    .setName(entry.name)
                    .setNative(entry.nativeLabel)
                    .build()
            )
        }
        return builder.build()
    }

    override suspend fun sayHello(request: Greeting.SayHelloRequest): Greeting.SayHelloResponse {
        val entry = lookupGreeting(request.langCode)
        val name = request.name.trim().ifEmpty { "World" }
        return Greeting.SayHelloResponse.newBuilder()
            .setGreeting(entry.template.format(name))
            .setLanguage(entry.name)
            .setLangCode(entry.code)
            .build()
    }
}

fun main(args: Array<String>) {
    if (args.isEmpty()) {
        usage()
    }

    when (args[0]) {
        "serve" -> {
            val recipeRoot = findRecipeRoot()
            val listenUri = Serve.parseFlags(args.drop(1).toTypedArray())
            Serve.runWithOptions(
                listenUri,
                listOf(GreetingServiceImpl()),
                Serve.Options(
                    protoDir = recipeRoot.resolve("protos"),
                    holonYamlPath = recipeRoot.resolve("holon.yaml"),
                ),
            )
        }
        "version" -> println("gudule-daemon-greeting-kotlin v0.4.2")
        else -> usage()
    }
}

private fun usage(): Nothing {
    System.err.println("usage: gudule-daemon-greeting-kotlin <serve|version> [flags]")
    exitProcess(1)
}

fun findRecipeRoot(): Path {
    val configured = System.getProperty("gudule.recipe.root")?.trim().orEmpty()
    if (configured.isNotEmpty()) {
        return Path.of(configured).toAbsolutePath().normalize()
    }

    val starts = listOf(
        Path.of("").toAbsolutePath().normalize(),
        Path.of(System.getProperty("java.class.path").split(File.pathSeparator).first()).toAbsolutePath().normalize(),
    )

    starts.forEach { start ->
        var current = if (start.toFile().isDirectory) start else start.parent
        while (current != null) {
            val holonYaml = current.resolve("holon.yaml")
            val buildFile = current.resolve("build.gradle.kts")
            if (holonYaml.exists() && buildFile.exists()) {
                return current
            }
            current = current.parent
        }
    }

    error("could not locate gudule-daemon-greeting-kotlin recipe root")
}

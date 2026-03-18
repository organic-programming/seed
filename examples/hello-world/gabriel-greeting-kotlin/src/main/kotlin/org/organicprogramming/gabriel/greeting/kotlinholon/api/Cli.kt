package org.organicprogramming.gabriel.greeting.kotlinholon.api

import com.google.protobuf.MessageOrBuilder
import com.google.protobuf.util.JsonFormat
import greeting.v1.Greeting
import org.organicprogramming.gabriel.greeting.kotlinholon.internal.GreetingServer
import org.organicprogramming.holons.Serve
import java.io.PrintStream

object Cli {
    const val VERSION = "gabriel-greeting-kotlin {{ .Version }}"

    fun run(args: Array<String>, stdout: PrintStream = System.out, stderr: PrintStream = System.err): Int {
        if (args.isEmpty()) {
            printUsage(stderr)
            return 1
        }

        return when (canonicalCommand(args[0])) {
            "serve" -> {
                try {
                    GreetingServer.listenAndServe(Serve.parseFlags(args.drop(1).toTypedArray()))
                    0
                } catch (error: Exception) {
                    stderr.println("serve: ${error.message}")
                    1
                }
            }
            "version" -> {
                stdout.println(VERSION)
                0
            }
            "help" -> {
                printUsage(stdout)
                0
            }
            "listlanguages" -> runListLanguages(args.drop(1), stdout, stderr)
            "sayhello" -> runSayHello(args.drop(1), stdout, stderr)
            else -> {
                stderr.println("unknown command \"${args[0]}\"")
                printUsage(stderr)
                1
            }
        }
    }

    private fun runListLanguages(args: List<String>, stdout: PrintStream, stderr: PrintStream): Int {
        return try {
            val options = parseCommandOptions(args)
            if (options.positional.isNotEmpty()) {
                stderr.println("listLanguages: accepts no positional arguments")
                1
            } else {
                writeResponse(stdout, PublicApi.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance()), options.format)
                0
            }
        } catch (error: Exception) {
            stderr.println("listLanguages: ${error.message}")
            1
        }
    }

    private fun runSayHello(args: List<String>, stdout: PrintStream, stderr: PrintStream): Int {
        return try {
            val options = parseCommandOptions(args)
            if (options.positional.size > 2) {
                stderr.println("sayHello: accepts at most <name> [lang_code]")
                return 1
            }

            val request = Greeting.SayHelloRequest.newBuilder().setLangCode("en")
            if (options.positional.isNotEmpty()) {
                request.name = options.positional[0]
            }
            if (options.positional.size >= 2) {
                if (options.lang.isNotEmpty()) {
                    stderr.println("sayHello: use either a positional lang_code or --lang, not both")
                    return 1
                }
                request.langCode = options.positional[1]
            }
            if (options.lang.isNotEmpty()) {
                request.langCode = options.lang
            }

            writeResponse(stdout, PublicApi.sayHello(request.build()), options.format)
            0
        } catch (error: Exception) {
            stderr.println("sayHello: ${error.message}")
            1
        }
    }

    private fun parseCommandOptions(args: List<String>): CommandOptions {
        var format = "text"
        var lang = ""
        val positional = mutableListOf<String>()
        var index = 0

        while (index < args.size) {
            when (val arg = args[index]) {
                "--json" -> format = "json"
                "--format" -> {
                    index += 1
                    require(index < args.size) { "--format requires a value" }
                    format = parseOutputFormat(args[index])
                }
                "--lang" -> {
                    index += 1
                    require(index < args.size) { "--lang requires a value" }
                    lang = args[index].trim()
                }
                else -> when {
                    arg.startsWith("--format=") -> format = parseOutputFormat(arg.removePrefix("--format="))
                    arg.startsWith("--lang=") -> lang = arg.removePrefix("--lang=").trim()
                    arg.startsWith("--") -> throw IllegalArgumentException("unknown flag \"$arg\"")
                    else -> positional += arg
                }
            }
            index += 1
        }

        return CommandOptions(format, lang, positional.toList())
    }

    private fun parseOutputFormat(raw: String): String =
        when (raw.trim().lowercase()) {
            "", "text", "txt" -> "text"
            "json" -> "json"
            else -> throw IllegalArgumentException("unsupported format \"$raw\"")
        }

    private fun writeResponse(stdout: PrintStream, message: MessageOrBuilder, format: String) {
        when (format) {
            "json" -> stdout.println(JsonFormat.printer().print(message))
            "text" -> writeText(stdout, message)
            else -> throw IllegalArgumentException("unsupported format \"$format\"")
        }
    }

    private fun writeText(stdout: PrintStream, message: MessageOrBuilder) {
        when (message) {
            is Greeting.SayHelloResponse -> stdout.println(message.greeting)
            is Greeting.ListLanguagesResponse -> message.languagesList.forEach { language ->
                stdout.println("${language.code}\t${language.name}\t${language.native}")
            }
            else -> throw IllegalArgumentException("unsupported text output for ${message::class.simpleName}")
        }
    }

    private fun canonicalCommand(raw: String): String =
        raw.trim().lowercase().replace("-", "").replace("_", "").replace(" ", "")

    private fun printUsage(output: PrintStream) {
        output.println("usage: gabriel-greeting-kotlin <command> [args] [flags]")
        output.println()
        output.println("commands:")
        output.println("  serve [--listen <uri>]                    Start the gRPC server")
        output.println("  version                                  Print version and exit")
        output.println("  help                                     Print usage")
        output.println("  listLanguages [--format text|json]       List supported languages")
        output.println("  sayHello [name] [lang_code] [--format text|json] [--lang <code>]")
        output.println()
        output.println("examples:")
        output.println("  gabriel-greeting-kotlin serve --listen tcp://:9090")
        output.println("  gabriel-greeting-kotlin listLanguages --format json")
        output.println("  gabriel-greeting-kotlin sayHello Alice fr")
        output.println("  gabriel-greeting-kotlin sayHello Alice --lang fr --format json")
    }

    private data class CommandOptions(
        val format: String,
        val lang: String,
        val positional: List<String>,
    )
}

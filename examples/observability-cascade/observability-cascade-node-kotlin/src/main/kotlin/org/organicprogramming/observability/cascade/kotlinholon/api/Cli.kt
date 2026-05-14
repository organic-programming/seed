package org.organicprogramming.observability.cascade.kotlinholon.api

import com.google.protobuf.MessageOrBuilder
import com.google.protobuf.util.JsonFormat
import org.organicprogramming.holons.Serve
import org.organicprogramming.observability.cascade.kotlinholon.internal.RelayServer
import relay.v1.Relay
import java.io.PrintStream

object Cli {
    const val VERSION = "observability-cascade-node-kotlin {{ .Version }}"

    fun run(args: Array<String>, stdout: PrintStream = System.out, stderr: PrintStream = System.err): Int {
        if (args.isEmpty()) {
            printUsage(stderr)
            return 1
        }

        return when (canonicalCommand(args[0])) {
            "serve" -> {
                try {
                    val serveArgs = args.drop(1).toTypedArray()
                    val parsed = Serve.parseOptions(serveArgs)
                    RelayServer.listenAndServe(parsed.listenUri, parsed.reflect, parseMembers(serveArgs))
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
            "tick" -> runTick(args.drop(1), stdout, stderr)
            else -> {
                stderr.println("unknown command \"${args[0]}\"")
                printUsage(stderr)
                1
            }
        }
    }

    private fun runTick(args: List<String>, stdout: PrintStream, stderr: PrintStream): Int {
        return try {
            val request = Relay.TickRequest.newBuilder()
            val positional = mutableListOf<String>()
            var index = 0
            while (index < args.size) {
                when (val arg = args[index]) {
                    "--sender" -> {
                        index += 1
                        require(index < args.size) { "--sender requires a value" }
                        request.sender = args[index]
                    }
                    "--note" -> {
                        index += 1
                        require(index < args.size) { "--note requires a value" }
                        request.note = args[index]
                    }
                    else -> when {
                        arg.startsWith("--sender=") -> request.sender = arg.removePrefix("--sender=")
                        arg.startsWith("--note=") -> request.note = arg.removePrefix("--note=")
                        arg.startsWith("--") -> throw IllegalArgumentException("unknown flag \"$arg\"")
                        else -> positional += arg
                    }
                }
                index += 1
            }
            if (request.sender.isBlank() && positional.isNotEmpty()) request.sender = positional[0]
            if (request.note.isBlank() && positional.size >= 2) request.note = positional[1]
            writeResponse(stdout, PublicApi.tick(request.build()))
            0
        } catch (error: Exception) {
            stderr.println("tick: ${error.message}")
            1
        }
    }

    private fun parseMembers(args: Array<String>): List<Serve.MemberRef> {
        val members = mutableListOf<Serve.MemberRef>()
        var index = 0
        while (index < args.size) {
            val arg = args[index]
            if (arg == "--member") {
                index += 1
                require(index < args.size) { "--member requires <slug>=<address>" }
                members += parseMember(args[index])
            } else if (arg.startsWith("--member=")) {
                members += parseMember(arg.removePrefix("--member="))
            }
            index += 1
        }
        return members
    }

    private fun parseMember(raw: String): Serve.MemberRef {
        val idx = raw.indexOf('=')
        require(idx >= 0) { "--member requires <slug>=<address>" }
        val slug = raw.substring(0, idx).trim()
        val address = raw.substring(idx + 1).trim()
        require(slug.isNotEmpty() && address.isNotEmpty()) { "--member requires non-empty slug and address" }
        return Serve.MemberRef(slug = slug, address = address)
    }

    private fun writeResponse(stdout: PrintStream, message: MessageOrBuilder) {
        stdout.println(JsonFormat.printer().print(message))
    }

    private fun canonicalCommand(raw: String): String =
        raw.trim().lowercase().replace("-", "").replace("_", "").replace(" ", "")

    private fun printUsage(output: PrintStream) {
        output.println("usage: observability-cascade-node-kotlin <command> [args] [flags]")
        output.println()
        output.println("commands:")
        output.println("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server")
        output.println("  tick [sender] [note]                                Emit one local tick")
        output.println("  version                                             Print version and exit")
        output.println("  help                                                Print usage")
    }
}

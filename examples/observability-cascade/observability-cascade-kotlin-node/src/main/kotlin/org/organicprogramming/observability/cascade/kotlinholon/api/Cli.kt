package org.organicprogramming.observability.cascade.kotlinholon.api

import gen.DescribeGenerated
import java.io.PrintStream
import java.util.Locale
import org.organicprogramming.holons.Composite
import org.organicprogramming.holons.Describe
import org.organicprogramming.holons.Observability
import org.organicprogramming.holons.RelayService
import org.organicprogramming.holons.Serve

object Cli {
    const val VERSION = "observability-cascade-kotlin-node {{ .Version }}"

    fun run(args: Array<String>, stdout: PrintStream = System.out, stderr: PrintStream = System.err): Int {
        if (args.isEmpty()) {
            printUsage(stderr)
            return 1
        }

        return when (canonicalCommand(args[0])) {
            "serve" -> {
                try {
                    serve(args.drop(1).toTypedArray())
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
            else -> {
                stderr.println("unknown command \"${args[0]}\"")
                printUsage(stderr)
                1
            }
        }
    }

    private fun serve(args: Array<String>) {
        val parsedChildren = Composite.parseChildFlags(args)
        val remaining = parsedChildren.remaining
        val parsedFlags = Serve.parseOptions(remaining)
        val transport = parseTransport(remaining)
        Observability.fromEnv(Observability.Config(slug = "observability-cascade-kotlin-node"))

        var child: Composite.SpawnedMember? = null
        if (parsedChildren.children.isNotEmpty()) {
            val first = parsedChildren.children.first()
            child = Composite.spawnMember(
                Composite.SpawnOptions().apply {
                    slug = first.slug
                    binaryPath = first.binary
                    this.transport = transport
                    downstreamChain = parsedChildren.children.drop(1)
                },
            )
        }

        Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
        try {
            Serve.runWithOptions(
                normalizeListenUri(parsedFlags.listenUri),
                listOf(RelayService(child?.conn)),
                Serve.Options(
                    reflect = parsedFlags.reflect,
                    slug = "observability-cascade-kotlin-node",
                ),
            )
        } finally {
            child?.stop()
        }
    }

    private fun parseTransport(args: Array<String>): String {
        var index = 0
        while (index < args.size) {
            val arg = args[index]
            if (arg == "--transport" && index + 1 < args.size) return args[index + 1]
            if (arg.startsWith("--transport=")) return arg.removePrefix("--transport=")
            index += 1
        }
        return "stdio"
    }

    private fun normalizeListenUri(listenUri: String): String =
        if (listenUri.startsWith("tcp://:")) {
            "tcp://0.0.0.0:${listenUri.removePrefix("tcp://:")}"
        } else {
            listenUri
        }

    private fun canonicalCommand(raw: String): String =
        raw.trim().lowercase(Locale.ROOT).replace("-", "").replace("_", "").replace(" ", "")

    private fun printUsage(output: PrintStream) {
        output.println("usage: observability-cascade-kotlin-node <command> [args] [flags]")
        output.println()
        output.println("commands:")
        output.println("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server")
        output.println("  version                                                           Print version and exit")
        output.println("  help                                                              Print usage")
    }
}

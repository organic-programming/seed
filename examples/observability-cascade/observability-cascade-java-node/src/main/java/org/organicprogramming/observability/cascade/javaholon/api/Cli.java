package org.organicprogramming.observability.cascade.javaholon.api;

import gen.describe_generated;
import org.organicprogramming.holons.Composite;
import org.organicprogramming.holons.Describe;
import org.organicprogramming.holons.Observability;
import org.organicprogramming.holons.RelayService;
import org.organicprogramming.holons.Serve;

import java.io.PrintStream;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

public final class Cli {
    public static final String VERSION = "observability-cascade-java-node {{ .Version }}";

    private Cli() {
    }

    public static int run(String[] args, PrintStream stdout, PrintStream stderr) {
        if (args.length == 0) {
            printUsage(stderr);
            return 1;
        }

        switch (canonicalCommand(args[0])) {
            case "serve":
                try {
                    serve(Arrays.copyOfRange(args, 1, args.length));
                    return 0;
                } catch (Exception error) {
                    stderr.printf("serve: %s%n", error.getMessage());
                    return 1;
                }
            case "version":
                stdout.println(VERSION);
                return 0;
            case "help":
                printUsage(stdout);
                return 0;
            default:
                stderr.printf("unknown command \"%s\"%n", args[0]);
                printUsage(stderr);
                return 1;
        }
    }

    private static void serve(String[] args) throws Exception {
        Composite.ParsedChildFlags parsedChildren = Composite.parseChildFlags(args);
        String[] remaining = parsedChildren.remaining();
        Serve.ParsedFlags parsedFlags = Serve.parseOptions(remaining);
        String transport = parseTransport(remaining);
        Observability.Config config = new Observability.Config();
        config.slug = "observability-cascade-java-node";
        Observability.fromEnv(config);

        Composite.SpawnedMember child = null;
        if (!parsedChildren.children().isEmpty()) {
            Composite.ChildSpec first = parsedChildren.children().get(0);
            Composite.SpawnOptions spawn = new Composite.SpawnOptions();
            spawn.slug = first.slug();
            spawn.binaryPath = first.binary();
            spawn.transport = transport;
            spawn.downstreamChain = parsedChildren.children().subList(1, parsedChildren.children().size());
            child = Composite.spawnMember(spawn);
        }

        Describe.useStaticResponse(describe_generated.StaticDescribeResponse());
        try {
            Serve.runWithOptions(
                    normalizeListenUri(parsedFlags.listenUri()),
                    List.of(new RelayService(child == null ? null : child.conn)),
                    new Serve.Options()
                            .withReflect(parsedFlags.reflect())
                            .withSlug("observability-cascade-java-node"));
        } finally {
            if (child != null) {
                child.stop();
            }
        }
    }

    private static String parseTransport(String[] args) {
        for (int index = 0; index < args.length; index++) {
            String arg = args[index];
            if ("--transport".equals(arg) && index + 1 < args.length) {
                return args[index + 1];
            }
            if (arg.startsWith("--transport=")) {
                return arg.substring("--transport=".length());
            }
        }
        return "stdio";
    }

    private static String normalizeListenUri(String listenUri) {
        if (listenUri != null && listenUri.startsWith("tcp://:")) {
            return "tcp://0.0.0.0:" + listenUri.substring("tcp://:".length());
        }
        return listenUri;
    }

    static String canonicalCommand(String raw) {
        return raw.trim().toLowerCase().replace("-", "").replace("_", "").replace(" ", "");
    }

    static void printUsage(PrintStream output) {
        output.println("usage: observability-cascade-java-node <command> [args] [flags]");
        output.println();
        output.println("commands:");
        output.println("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server");
        output.println("  version                                                           Print version and exit");
        output.println("  help                                                              Print usage");
    }
}

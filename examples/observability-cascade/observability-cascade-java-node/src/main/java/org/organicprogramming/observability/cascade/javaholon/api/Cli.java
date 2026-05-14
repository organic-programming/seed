package org.organicprogramming.observability.cascade.javaholon.api;

import com.google.protobuf.MessageOrBuilder;
import com.google.protobuf.util.JsonFormat;
import org.organicprogramming.holons.Serve;
import org.organicprogramming.observability.cascade.javaholon.internal.RelayServer;
import relay.v1.Relay;

import java.io.PrintStream;
import java.util.ArrayList;
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
                    String[] serveArgs = slice(args, 1);
                    Serve.ParsedFlags parsed = Serve.parseOptions(serveArgs);
                    RelayServer.listenAndServe(parsed.listenUri(), parsed.reflect(), parseMembers(serveArgs));
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
            case "tick":
                return runTick(slice(args, 1), stdout, stderr);
            default:
                stderr.printf("unknown command \"%s\"%n", args[0]);
                printUsage(stderr);
                return 1;
        }
    }

    static int runTick(String[] args, PrintStream stdout, PrintStream stderr) {
        try {
            Relay.TickRequest.Builder request = Relay.TickRequest.newBuilder();
            List<String> positional = new ArrayList<>();
            for (int index = 0; index < args.length; index++) {
                String arg = args[index];
                if ("--sender".equals(arg)) {
                    index++;
                    if (index >= args.length) {
                        throw new IllegalArgumentException("--sender requires a value");
                    }
                    request.setSender(args[index]);
                } else if (arg.startsWith("--sender=")) {
                    request.setSender(arg.substring("--sender=".length()));
                } else if ("--note".equals(arg)) {
                    index++;
                    if (index >= args.length) {
                        throw new IllegalArgumentException("--note requires a value");
                    }
                    request.setNote(args[index]);
                } else if (arg.startsWith("--note=")) {
                    request.setNote(arg.substring("--note=".length()));
                } else if (arg.startsWith("--")) {
                    throw new IllegalArgumentException("unknown flag \"" + arg + "\"");
                } else {
                    positional.add(arg);
                }
            }
            if (request.getSender().isBlank() && !positional.isEmpty()) {
                request.setSender(positional.get(0));
            }
            if (request.getNote().isBlank() && positional.size() >= 2) {
                request.setNote(positional.get(1));
            }
            writeResponse(stdout, PublicApi.tick(request.build()));
            return 0;
        } catch (Exception error) {
            stderr.printf("tick: %s%n", error.getMessage());
            return 1;
        }
    }

    static List<Serve.MemberRef> parseMembers(String[] args) {
        List<Serve.MemberRef> members = new ArrayList<>();
        for (int index = 0; index < args.length; index++) {
            String arg = args[index];
            if ("--member".equals(arg)) {
                index++;
                if (index >= args.length) {
                    throw new IllegalArgumentException("--member requires <slug>=<address>");
                }
                members.add(parseMember(arg, args[index]));
            } else if (arg.startsWith("--member=")) {
                members.add(parseMember("--member", arg.substring("--member=".length())));
            }
        }
        return members;
    }

    static Serve.MemberRef parseMember(String flag, String raw) {
        int idx = raw.indexOf('=');
        if (idx < 0) {
            throw new IllegalArgumentException(flag + " requires <slug>=<address>");
        }
        String slug = raw.substring(0, idx).trim();
        String address = raw.substring(idx + 1).trim();
        if (slug.isEmpty() || address.isEmpty()) {
            throw new IllegalArgumentException(flag + " requires non-empty slug and address");
        }
        return new Serve.MemberRef(slug, "", address);
    }

    static void writeResponse(PrintStream stdout, MessageOrBuilder message) throws Exception {
        stdout.println(JsonFormat.printer().print(message));
    }

    static String canonicalCommand(String raw) {
        return raw.trim().toLowerCase().replace("-", "").replace("_", "").replace(" ", "");
    }

    static void printUsage(PrintStream output) {
        output.println("usage: observability-cascade-java-node <command> [args] [flags]");
        output.println();
        output.println("commands:");
        output.println("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server");
        output.println("  tick [sender] [note]                                Emit one local tick");
        output.println("  version                                             Print version and exit");
        output.println("  help                                                Print usage");
    }

    private static String[] slice(String[] args, int offset) {
        String[] sliced = new String[Math.max(0, args.length - offset)];
        System.arraycopy(args, offset, sliced, 0, sliced.length);
        return sliced;
    }
}

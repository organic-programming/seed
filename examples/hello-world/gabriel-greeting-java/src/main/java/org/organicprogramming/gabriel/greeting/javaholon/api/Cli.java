package org.organicprogramming.gabriel.greeting.javaholon.api;

import com.google.protobuf.MessageOrBuilder;
import com.google.protobuf.util.JsonFormat;
import greeting.v1.Greeting;
import org.organicprogramming.gabriel.greeting.javaholon.internal.GreetingServer;
import org.organicprogramming.holons.Serve;

import java.io.PrintStream;
import java.util.ArrayList;
import java.util.List;

public final class Cli {
    public static final String VERSION = "gabriel-greeting-java v0.1.0";

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
                    GreetingServer.listenAndServe(Serve.parseFlags(slice(args, 1)));
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
            case "listlanguages":
                return runListLanguages(slice(args, 1), stdout, stderr);
            case "sayhello":
                return runSayHello(slice(args, 1), stdout, stderr);
            default:
                stderr.printf("unknown command \"%s\"%n", args[0]);
                printUsage(stderr);
                return 1;
        }
    }

    static int runListLanguages(String[] args, PrintStream stdout, PrintStream stderr) {
        try {
            CommandOptions options = parseCommandOptions(args);
            if (!options.positional().isEmpty()) {
                stderr.println("listLanguages: accepts no positional arguments");
                return 1;
            }

            Greeting.ListLanguagesResponse response = PublicApi.listLanguages(Greeting.ListLanguagesRequest.getDefaultInstance());
            writeResponse(stdout, response, options.format());
            return 0;
        } catch (Exception error) {
            stderr.printf("listLanguages: %s%n", error.getMessage());
            return 1;
        }
    }

    static int runSayHello(String[] args, PrintStream stdout, PrintStream stderr) {
        try {
            CommandOptions options = parseCommandOptions(args);
            if (options.positional().size() > 2) {
                stderr.println("sayHello: accepts at most <name> [lang_code]");
                return 1;
            }

            Greeting.SayHelloRequest.Builder request = Greeting.SayHelloRequest.newBuilder().setLangCode("en");
            if (!options.positional().isEmpty()) {
                request.setName(options.positional().get(0));
            }
            if (options.positional().size() >= 2) {
                if (!options.lang().isEmpty()) {
                    stderr.println("sayHello: use either a positional lang_code or --lang, not both");
                    return 1;
                }
                request.setLangCode(options.positional().get(1));
            }
            if (!options.lang().isEmpty()) {
                request.setLangCode(options.lang());
            }

            Greeting.SayHelloResponse response = PublicApi.sayHello(request.build());
            writeResponse(stdout, response, options.format());
            return 0;
        } catch (Exception error) {
            stderr.printf("sayHello: %s%n", error.getMessage());
            return 1;
        }
    }

    static CommandOptions parseCommandOptions(String[] args) {
        String format = "text";
        String lang = "";
        List<String> positional = new ArrayList<>();

        for (int index = 0; index < args.length; index++) {
            String arg = args[index];
            if ("--json".equals(arg)) {
                format = "json";
            } else if ("--format".equals(arg)) {
                index++;
                if (index >= args.length) {
                    throw new IllegalArgumentException("--format requires a value");
                }
                format = parseOutputFormat(args[index]);
            } else if (arg.startsWith("--format=")) {
                format = parseOutputFormat(arg.substring("--format=".length()));
            } else if ("--lang".equals(arg)) {
                index++;
                if (index >= args.length) {
                    throw new IllegalArgumentException("--lang requires a value");
                }
                lang = args[index].trim();
            } else if (arg.startsWith("--lang=")) {
                lang = arg.substring("--lang=".length()).trim();
            } else if (arg.startsWith("--")) {
                throw new IllegalArgumentException("unknown flag \"" + arg + "\"");
            } else {
                positional.add(arg);
            }
        }

        return new CommandOptions(format, lang, List.copyOf(positional));
    }

    static String parseOutputFormat(String raw) {
        String normalized = raw.trim().toLowerCase();
        return switch (normalized) {
            case "", "text", "txt" -> "text";
            case "json" -> "json";
            default -> throw new IllegalArgumentException("unsupported format \"" + raw + "\"");
        };
    }

    static void writeResponse(PrintStream stdout, MessageOrBuilder message, String format) throws Exception {
        switch (format) {
            case "json":
                stdout.println(JsonFormat.printer().print(message));
                return;
            case "text":
                writeText(stdout, message);
                return;
            default:
                throw new IllegalArgumentException("unsupported format \"" + format + "\"");
        }
    }

    static void writeText(PrintStream stdout, MessageOrBuilder message) {
        if (message instanceof Greeting.SayHelloResponse response) {
            stdout.println(response.getGreeting());
            return;
        }
        if (message instanceof Greeting.ListLanguagesResponse response) {
            for (Greeting.Language language : response.getLanguagesList()) {
                stdout.printf("%s\t%s\t%s%n", language.getCode(), language.getName(), language.getNative());
            }
            return;
        }
        throw new IllegalArgumentException("unsupported text output for " + message.getClass().getSimpleName());
    }

    static String canonicalCommand(String raw) {
        return raw.trim().toLowerCase().replace("-", "").replace("_", "").replace(" ", "");
    }

    static void printUsage(PrintStream output) {
        output.println("usage: gabriel-greeting-java <command> [args] [flags]");
        output.println();
        output.println("commands:");
        output.println("  serve [--listen <uri>]                    Start the gRPC server");
        output.println("  version                                  Print version and exit");
        output.println("  help                                     Print usage");
        output.println("  listLanguages [--format text|json]       List supported languages");
        output.println("  sayHello [name] [lang_code] [--format text|json] [--lang <code>]");
        output.println();
        output.println("examples:");
        output.println("  gabriel-greeting-java serve --listen tcp://:9090");
        output.println("  gabriel-greeting-java listLanguages --format json");
        output.println("  gabriel-greeting-java sayHello Alice fr");
        output.println("  gabriel-greeting-java sayHello Alice --lang fr --format json");
    }

    private static String[] slice(String[] args, int offset) {
        String[] sliced = new String[Math.max(0, args.length - offset)];
        System.arraycopy(args, offset, sliced, 0, sliced.length);
        return sliced;
    }

    record CommandOptions(String format, String lang, List<String> positional) {
    }
}

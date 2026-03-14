package org.organicprogramming.hello;

import org.organicprogramming.holons.Serve;

/**
 * Pure-logic HelloService — no gRPC dependency required for the test.
 * The greet logic is the holon's deterministic core.
 */
public final class HelloService {
    /** Greet returns a greeting for the given name. */
    public static String greet(String name) {
        String n = (name == null || name.isEmpty()) ? "World" : name;
        return "Hello, " + n + "!";
    }

    public static void main(String[] args) {
        if (args.length > 0 && "serve".equals(args[0])) {
            String[] serveArgs = java.util.Arrays.copyOfRange(args, 1, args.length);
            String listenUri = Serve.parseFlags(serveArgs);
            System.err.println("java-hello-world listening on " + listenUri);
            System.out.println("{\"message\":\"" + greet("") + "\"}");
            return;
        }

        String name = args.length > 0 ? args[0] : "";
        System.out.println(greet(name));
    }
}

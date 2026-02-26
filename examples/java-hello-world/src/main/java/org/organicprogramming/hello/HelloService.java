package org.organicprogramming.hello;

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
        String name = args.length > 0 ? args[0] : "";
        System.out.println(greet(name));
    }
}

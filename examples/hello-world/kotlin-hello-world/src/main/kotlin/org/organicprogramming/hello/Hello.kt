package org.organicprogramming.hello

import org.organicprogramming.holons.Serve

/** Pure deterministic HelloService. */
object HelloService {
    fun greet(name: String): String {
        val n = name.ifEmpty { "World" }
        return "Hello, $n!"
    }
}

fun main(args: Array<String>) {
    if (args.firstOrNull() == "serve") {
        val listenURI = Serve.parseFlags(args.drop(1).toTypedArray())
        System.err.println("kotlin-hello-world listening on $listenURI")
        println("""{"message":"${HelloService.greet("")}"}""")
        return
    }

    val name = args.firstOrNull() ?: ""
    println(HelloService.greet(name))
}

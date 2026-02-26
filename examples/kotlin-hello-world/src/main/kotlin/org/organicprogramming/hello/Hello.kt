package org.organicprogramming.hello

/** Pure deterministic HelloService. */
object HelloService {
    fun greet(name: String): String {
        val n = name.ifEmpty { "World" }
        return "Hello, $n!"
    }
}

fun main(args: Array<String>) {
    val name = args.firstOrNull() ?: ""
    println(HelloService.greet(name))
}

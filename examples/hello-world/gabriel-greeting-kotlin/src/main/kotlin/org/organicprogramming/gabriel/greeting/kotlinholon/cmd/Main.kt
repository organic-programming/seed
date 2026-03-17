package org.organicprogramming.gabriel.greeting.kotlinholon.cmd

import org.organicprogramming.gabriel.greeting.kotlinholon.api.Cli
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    val exitCode = Cli.run(args)
    if (exitCode != 0) {
        exitProcess(exitCode)
    }
}

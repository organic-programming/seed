package org.organicprogramming.gabriel.greeting.kotlinholon.cmd

import gen.DescribeGenerated
import org.organicprogramming.gabriel.greeting.kotlinholon.api.Cli
import org.organicprogramming.holons.Describe
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    val exitCode = Cli.run(args)
    if (exitCode != 0) {
        exitProcess(exitCode)
    }
}

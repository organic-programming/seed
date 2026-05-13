package org.organicprogramming.relay.cascade.kotlinholon.cmd

import gen.DescribeGenerated
import org.organicprogramming.holons.Describe
import org.organicprogramming.relay.cascade.kotlinholon.api.Cli
import kotlin.system.exitProcess

fun main(args: Array<String>) {
    Describe.useStaticResponse(DescribeGenerated.StaticDescribeResponse())
    val exitCode = Cli.run(args)
    if (exitCode != 0) {
        exitProcess(exitCode)
    }
}

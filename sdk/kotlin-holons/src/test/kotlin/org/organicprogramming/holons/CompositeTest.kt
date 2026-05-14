package org.organicprogramming.holons

import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

class CompositeTest {
    @Test
    fun memberResolvesExecutableRelativeToLauncher() {
        val root = Files.createTempDirectory("kotlin-composite-")
        try {
            val launcher = root.resolve("bin/darwin_arm64/parent")
            val memberDir = launcher.parent.resolve("holons/kotlin-node")
            val member = memberDir.resolve("observability-cascade-kotlin-node")
            Files.createDirectories(memberDir)
            Files.writeString(launcher, "#!/bin/sh\n")
            Files.writeString(member, "#!/bin/sh\n")
            launcher.toFile().setExecutable(true)
            member.toFile().setExecutable(true)

            assertEquals(member, Composite.memberFromExecutable(launcher, "kotlin-node"))
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun memberErrorsWhenMissing() {
        val root = Files.createTempDirectory("kotlin-composite-")
        try {
            val launcher = root.resolve("bin/darwin_arm64/parent")
            Files.createDirectories(launcher.parent)
            Files.writeString(launcher, "#!/bin/sh\n")

            assertFailsWith<Exception> {
                Composite.memberFromExecutable(launcher, "kotlin-node")
            }
        } finally {
            root.toFile().deleteRecursively()
        }
    }
}

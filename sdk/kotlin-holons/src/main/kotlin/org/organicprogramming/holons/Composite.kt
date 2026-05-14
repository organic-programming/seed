package org.organicprogramming.holons

import java.io.IOException
import java.nio.file.Files
import java.nio.file.Path

object Composite {
    @JvmStatic
    @Throws(IOException::class)
    fun member(id: String): Path {
        val executable = System.getenv("OP_HOLON_EXECUTABLE")
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
            ?: ProcessHandle.current().info().command().orElse("")
        if (executable.isBlank()) {
            throw IOException("OP_HOLON_EXECUTABLE is not set")
        }
        return memberFromExecutable(Path.of(executable), id)
    }

    @JvmStatic
    @Throws(IOException::class)
    fun memberFromExecutable(executable: Path, id: String): Path {
        require(id.isNotBlank()) { "member id is required" }
        val memberDir = executable.toAbsolutePath().normalize().parent.resolve("holons").resolve(id)
        if (!Files.isDirectory(memberDir)) {
            throw IOException("member directory not found: $memberDir")
        }
        val stream = Files.list(memberDir)
        try {
            return stream
                .filter { Files.isRegularFile(it) }
                .filter { Files.isExecutable(it) || it.fileName.toString().endsWith(".exe") }
                .sorted()
                .findFirst()
                .orElseThrow { IOException("no executable found in $memberDir") }
        } finally {
            stream.close()
        }
    }
}

package org.organicprogramming.gabriel.greeting.kotlincompose.runtime

import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.Paths

object AppPaths {
    fun configureRuntimeEnvironment() {
        val siblingsProperty = System.getProperty("holons.siblings.root").orEmpty()
        val siblingsEnv = System.getenv("HOLONS_SIBLINGS_ROOT").orEmpty()
        if (siblingsProperty.isNotBlank() || siblingsEnv.isNotBlank()) {
            return
        }

        findPackagedHolonsDir()?.let {
            System.setProperty("holons.siblings.root", it.toString())
        }
    }

    fun discoveryRoot(): String? =
        findPackagedHolonsDir()?.toString()

    fun findAppProtoDir(): Path? =
        findSourceApiDir() ?: findPackagedProtoDir()

    private fun findSourceApiDir(): Path? {
        var current = Paths.get(System.getProperty("user.dir")).toAbsolutePath().normalize()
        while (true) {
            val candidate = current.resolve("api")
            if (Files.exists(candidate.resolve("v1/holon.proto"))) {
                return candidate
            }
            val parent = current.parent ?: return null
            if (parent == current) {
                return null
            }
            current = parent
        }
    }

    private fun findPackagedProtoDir(): Path? {
        return ancestorCandidates().firstNotNullOfOrNull { candidate ->
            val appProto = candidate.resolve("AppProto")
            if (Files.exists(appProto.resolve("v1/holon.proto"))) appProto else null
        }
    }

    private fun findPackagedHolonsDir(): Path? {
        return ancestorCandidates().firstNotNullOfOrNull { candidate ->
            val holonsDir = candidate.resolve("Holons")
            if (Files.isDirectory(holonsDir)) holonsDir else null
        }
    }

    private fun ancestorCandidates(): Sequence<Path> = sequence {
        val codeSource = runCatching {
            Paths.get(AppPaths::class.java.protectionDomain.codeSource.location.toURI())
        }.getOrNull()
        val start = when {
            codeSource == null -> null
            Files.isDirectory(codeSource) -> codeSource
            else -> codeSource.parent
        } ?: return@sequence

        var current: Path? = start.toAbsolutePath().normalize()
        while (current != null) {
            yield(current)
            current = current.parent
        }
    }
}

package org.organicprogramming.holons

import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

class DiscoverTest {
    @Test
    fun discoverRecursesSkipsAndDedups() {
        val root = Files.createTempDirectory("holons-kotlin-discover-")
        try {
            writeHolon(root, "holons/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go")
            writeHolon(root, "nested/beta", "uuid-beta", "Beta", "Rust", "beta-rust")
            writeHolon(root, "nested/dup/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go")

            listOf(".git/hidden", ".op/hidden", "node_modules/hidden", "vendor/hidden", "build/hidden", "testdata/hidden", ".cache/hidden")
                .forEach { skipped ->
                    writeHolon(root, skipped, "ignored-${Path.of(skipped).fileName}", "Ignored", "Holon", "ignored-holon")
                }

            val entries = Discover.discover(root)
            assertEquals(2, entries.size)

            val alpha = entries.first { it.uuid == "uuid-alpha" }
            assertEquals("alpha-go", alpha.slug)
            assertEquals("holons/alpha", alpha.relativePath)
            assertEquals("go-module", alpha.manifest?.build?.runner)

            val beta = entries.first { it.uuid == "uuid-beta" }
            assertEquals("nested/beta", beta.relativePath)
        } finally {
            root.toFile().deleteRecursively()
        }
    }

    @Test
    fun discoverLocalAndFindHelpers() {
        val root = Files.createTempDirectory("holons-kotlin-find-")
        val originalUserDir = System.getProperty("user.dir")
        val originalOpPath = System.getProperty("OPPATH")
        val originalOpBin = System.getProperty("OPBIN")
        try {
            writeHolon(root, "rob-go", "c7f3a1b2-1111-1111-1111-111111111111", "Rob", "Go", "rob-go")

            System.setProperty("user.dir", root.toString())
            System.setProperty("OPPATH", root.resolve("runtime").toString())
            System.setProperty("OPBIN", root.resolve("runtime").resolve("bin").toString())

            val local = Discover.discoverLocal()
            assertEquals(1, local.size)
            assertEquals("rob-go", local.first().slug)

            val bySlug = Discover.findBySlug("rob-go")
            assertNotNull(bySlug)
            assertEquals("c7f3a1b2-1111-1111-1111-111111111111", bySlug.uuid)

            val byUUID = Discover.findByUUID("c7f3a1b2")
            assertNotNull(byUUID)
            assertEquals("rob-go", byUUID.slug)

            assertNull(Discover.findBySlug("missing"))
        } finally {
            restoreProperty("user.dir", originalUserDir)
            restoreProperty("OPPATH", originalOpPath)
            restoreProperty("OPBIN", originalOpBin)
            root.toFile().deleteRecursively()
        }
    }

    private fun restoreProperty(name: String, value: String?) {
        if (value == null) {
            System.clearProperty(name)
        } else {
            System.setProperty(name, value)
        }
    }

    private fun writeHolon(root: Path, relativeDir: String, uuid: String, givenName: String, familyName: String, binary: String) {
        val dir = root.resolve(relativeDir)
        Files.createDirectories(dir)
        File(dir.toFile(), "holon.proto").writeText(
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                uuid: "$uuid"
                given_name: "$givenName"
                family_name: "$familyName"
                motto: "Test"
                composer: "test"
                clade: "deterministic/pure"
                status: "draft"
                born: "2026-03-07"
              }
              lineage: {
                generated_by: "test"
              }
              kind: "native"
              build: {
                runner: "go-module"
              }
              artifacts: {
                binary: "$binary"
              }
            };
            """.trimIndent()
        )
    }
}

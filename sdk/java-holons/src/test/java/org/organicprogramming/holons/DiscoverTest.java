package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.*;

class DiscoverTest {

    @Test
    void discoverRecursesSkipsAndDedups(@TempDir Path root) throws IOException {
        writeHolon(root, "holons/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go");
        writeHolon(root, "nested/beta", "uuid-beta", "Beta", "Rust", "beta-rust");
        writeHolon(root, "nested/dup/alpha", "uuid-alpha", "Alpha", "Go", "alpha-go");

        for (String skipped : new String[] { ".git/hidden", ".op/hidden", "node_modules/hidden", "vendor/hidden", "build/hidden", ".cache/hidden" }) {
            writeHolon(root, skipped, "ignored-" + Path.of(skipped).getFileName(), "Ignored", "Holon", "ignored-holon");
        }

        var entries = Discover.discover(root);
        assertEquals(2, entries.size());

        var alpha = entries.stream().filter(entry -> entry.uuid().equals("uuid-alpha")).findFirst().orElseThrow();
        assertEquals("alpha-go", alpha.slug());
        assertEquals("holons/alpha", alpha.relativePath());
        assertEquals("go-module", alpha.manifest().build().runner());

        var beta = entries.stream().filter(entry -> entry.uuid().equals("uuid-beta")).findFirst().orElseThrow();
        assertEquals("nested/beta", beta.relativePath());
    }

    @Test
    void discoverLocalAndFindHelpers(@TempDir Path root) throws IOException {
        writeHolon(root, "rob-go", "c7f3a1b2-1111-1111-1111-111111111111", "Rob", "Go", "rob-go");

        String originalUserDir = System.getProperty("user.dir");
        String originalOpPath = System.getProperty("OPPATH");
        String originalOpBin = System.getProperty("OPBIN");
        try {
            System.setProperty("user.dir", root.toString());
            System.setProperty("OPPATH", root.resolve("runtime").toString());
            System.setProperty("OPBIN", root.resolve("runtime").resolve("bin").toString());

            var local = Discover.discoverLocal();
            assertEquals(1, local.size());
            assertEquals("rob-go", local.get(0).slug());

            var bySlug = Discover.findBySlug("rob-go");
            assertTrue(bySlug.isPresent());
            assertEquals("c7f3a1b2-1111-1111-1111-111111111111", bySlug.get().uuid());

            var byUUID = Discover.findByUUID("c7f3a1b2");
            assertTrue(byUUID.isPresent());
            assertEquals("rob-go", byUUID.get().slug());

            assertTrue(Discover.findBySlug("missing").isEmpty());
        } finally {
            restoreProperty("user.dir", originalUserDir);
            restoreProperty("OPPATH", originalOpPath);
            restoreProperty("OPBIN", originalOpBin);
        }
    }

    private static void restoreProperty(String name, String value) {
        if (value == null) {
            System.clearProperty(name);
        } else {
            System.setProperty(name, value);
        }
    }

    private static void writeHolon(Path root, String relativeDir, String uuid, String givenName, String familyName, String binary)
            throws IOException {
        Path dir = root.resolve(relativeDir);
        Files.createDirectories(dir);
        Files.writeString(dir.resolve("holon.proto"), """
                syntax = "proto3";
                package holons.test.v1;

                option (holons.v1.manifest) = {
                  identity: {
                    uuid: "%s"
                    given_name: "%s"
                    family_name: "%s"
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
                    binary: "%s"
                  }
                };
                """.formatted(uuid, givenName, familyName, binary));
    }
}

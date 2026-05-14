package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.nio.file.Files;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

final class CompositeTest {
    @TempDir
    Path tempDir;

    @Test
    void memberResolvesExecutableRelativeToLauncher() throws Exception {
        Path launcher = tempDir.resolve("bin/darwin_arm64/parent");
        Path memberDir = launcher.getParent().resolve("holons/java-node");
        Path member = memberDir.resolve("observability-cascade-java-node");
        Files.createDirectories(memberDir);
        Files.writeString(launcher, "#!/bin/sh\n");
        Files.writeString(member, "#!/bin/sh\n");
        launcher.toFile().setExecutable(true);
        member.toFile().setExecutable(true);

        assertEquals(member, Composite.memberFromExecutable(launcher, "java-node"));
    }

    @Test
    void memberErrorsWhenMissing() throws Exception {
        Path launcher = tempDir.resolve("bin/darwin_arm64/parent");
        Files.createDirectories(launcher.getParent());
        Files.writeString(launcher, "#!/bin/sh\n");

        assertThrows(Exception.class, () -> Composite.memberFromExecutable(launcher, "java-node"));
    }
}

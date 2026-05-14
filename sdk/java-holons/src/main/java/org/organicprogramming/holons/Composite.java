package org.organicprogramming.holons;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

public final class Composite {
    private Composite() {}

    public static Path member(String id) throws IOException {
        String executable = System.getenv("OP_HOLON_EXECUTABLE");
        if (executable == null || executable.isBlank()) {
            executable = ProcessHandle.current().info().command().orElse("");
        }
        if (executable.isBlank()) {
            throw new IOException("OP_HOLON_EXECUTABLE is not set");
        }
        return memberFromExecutable(Path.of(executable), id);
    }

    public static Path memberFromExecutable(Path executable, String id) throws IOException {
        if (id == null || id.isBlank()) {
            throw new IllegalArgumentException("member id is required");
        }
        Path memberDir = executable.toAbsolutePath().normalize().getParent().resolve("holons").resolve(id);
        if (!Files.isDirectory(memberDir)) {
            throw new IOException("member directory not found: " + memberDir);
        }
        try (var stream = Files.list(memberDir)) {
            return stream
                    .filter(Files::isRegularFile)
                    .filter(path -> Files.isExecutable(path) || path.getFileName().toString().endsWith(".exe"))
                    .sorted()
                    .findFirst()
                    .orElseThrow(() -> new IOException("no executable found in " + memberDir));
        }
    }
}

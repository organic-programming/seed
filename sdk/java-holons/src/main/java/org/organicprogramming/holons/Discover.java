package org.organicprogramming.holons;

import java.io.IOException;
import java.nio.file.FileVisitResult;
import java.nio.file.FileVisitor;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.regex.Pattern;

/** Discover nearby holons by scanning for holon.proto manifests. */
public final class Discover {
    private static final Pattern API_VERSION_DIR = Pattern.compile("^v[0-9]+(?:[A-Za-z0-9._-]*)?$");

    private Discover() {
    }

    public record HolonBuild(String runner, String main) {
        public HolonBuild {
            runner = runner != null ? runner : "";
            main = main != null ? main : "";
        }
    }

    public record HolonArtifacts(String binary, String primary) {
        public HolonArtifacts {
            binary = binary != null ? binary : "";
            primary = primary != null ? primary : "";
        }
    }

    public record HolonManifest(String kind, HolonBuild build, HolonArtifacts artifacts) {
        public HolonManifest {
            kind = kind != null ? kind : "";
            build = build != null ? build : new HolonBuild("", "");
            artifacts = artifacts != null ? artifacts : new HolonArtifacts("", "");
        }
    }

    public record HolonEntry(
            String slug,
            String uuid,
            Path dir,
            String relativePath,
            String origin,
            Identity.HolonIdentity identity,
            HolonManifest manifest) {
    }

    public static List<HolonEntry> discover(Path root) throws IOException {
        return discoverInRoot(root, "local");
    }

    public static List<HolonEntry> discoverLocal() throws IOException {
        return discover(Path.of(currentDir()));
    }

    public static List<HolonEntry> discoverAll() throws IOException {
        List<HolonEntry> entries = new ArrayList<>();
        Set<String> seen = new HashSet<>();
        for (RootSpec spec : List.of(
                new RootSpec(Path.of(currentDir()), "local"),
                new RootSpec(opbin(), "$OPBIN"),
                new RootSpec(cacheDir(), "cache"))) {
            for (HolonEntry entry : discoverInRoot(spec.root(), spec.origin())) {
                String key = entry.uuid().isBlank() ? entry.dir().toString() : entry.uuid();
                if (seen.add(key)) {
                    entries.add(entry);
                }
            }
        }
        return entries;
    }

    public static Optional<HolonEntry> findBySlug(String slug) throws IOException {
        String needle = slug == null ? "" : slug.trim();
        if (needle.isEmpty()) {
            return Optional.empty();
        }

        HolonEntry match = null;
        for (HolonEntry entry : discoverAll()) {
            if (!needle.equals(entry.slug())) {
                continue;
            }
            if (match != null && !match.uuid().equals(entry.uuid())) {
                throw new IllegalStateException("ambiguous holon \"" + needle + "\"");
            }
            match = entry;
        }
        return Optional.ofNullable(match);
    }

    public static Optional<HolonEntry> findByUUID(String prefix) throws IOException {
        String needle = prefix == null ? "" : prefix.trim();
        if (needle.isEmpty()) {
            return Optional.empty();
        }

        HolonEntry match = null;
        for (HolonEntry entry : discoverAll()) {
            if (!entry.uuid().startsWith(needle)) {
                continue;
            }
            if (match != null && !match.uuid().equals(entry.uuid())) {
                throw new IllegalStateException("ambiguous UUID prefix \"" + needle + "\"");
            }
            match = entry;
        }
        return Optional.ofNullable(match);
    }

    private static List<HolonEntry> discoverInRoot(Path root, String origin) throws IOException {
        Path resolvedRoot = (root == null || root.toString().isBlank() ? Path.of(currentDir()) : root)
                .toAbsolutePath()
                .normalize();
        if (!Files.isDirectory(resolvedRoot)) {
            return Collections.emptyList();
        }

        Map<String, HolonEntry> entriesByKey = new HashMap<>();
        List<String> orderedKeys = new ArrayList<>();
        Files.walkFileTree(resolvedRoot, newVisitor(resolvedRoot, origin, entriesByKey, orderedKeys));

        List<HolonEntry> entries = new ArrayList<>();
        for (String key : orderedKeys) {
            HolonEntry entry = entriesByKey.get(key);
            if (entry != null) {
                entries.add(entry);
            }
        }
        entries.sort(Comparator
                .comparing(HolonEntry::relativePath)
                .thenComparing(HolonEntry::uuid));
        return entries;
    }

    private static FileVisitor<Path> newVisitor(
            Path root,
            String origin,
            Map<String, HolonEntry> entriesByKey,
            List<String> orderedKeys) {
        return new SimpleFileVisitor<>() {
            @Override
            public FileVisitResult preVisitDirectory(Path dir, BasicFileAttributes attrs) {
                Path fileName = dir.getFileName();
                String name = fileName == null ? "" : fileName.toString();
                if (shouldSkipDirectory(root, dir, name)) {
                    return FileVisitResult.SKIP_SUBTREE;
                }
                return FileVisitResult.CONTINUE;
            }

            @Override
            public FileVisitResult visitFile(Path file, BasicFileAttributes attrs) {
                if (!attrs.isRegularFile() || !Identity.PROTO_MANIFEST_FILE_NAME.equals(file.getFileName().toString())) {
                    return FileVisitResult.CONTINUE;
                }

                try {
                    Identity.ResolvedManifest resolved = Identity.resolveProtoFile(file);
                    Path holonDir = manifestRoot(file).toAbsolutePath().normalize();
                    HolonEntry entry = new HolonEntry(
                            resolved.identity().slug(),
                            resolved.identity().uuid(),
                            holonDir,
                            relativePath(root, holonDir),
                            origin,
                            resolved.identity(),
                            manifestFromResolved(resolved));

                    String key = entry.uuid().isBlank() ? entry.dir().toString() : entry.uuid();
                    HolonEntry existing = entriesByKey.get(key);
                    if (existing != null) {
                        if (pathDepth(entry.relativePath()) < pathDepth(existing.relativePath())) {
                            entriesByKey.put(key, entry);
                        }
                    } else {
                        entriesByKey.put(key, entry);
                        orderedKeys.add(key);
                    }
                } catch (Exception ignored) {
                    // Skip invalid holon manifests.
                }

                return FileVisitResult.CONTINUE;
            }
        };
    }

    private static HolonManifest manifestFromResolved(Identity.ResolvedManifest resolved) {
        return new HolonManifest(
                resolved.kind(),
                new HolonBuild(resolved.buildRunner(), resolved.buildMain()),
                new HolonArtifacts(resolved.artifactBinary(), resolved.artifactPrimary()));
    }

    private static Path manifestRoot(Path manifestPath) {
        Path manifestDir = manifestPath.getParent();
        if (manifestDir == null) {
            return Path.of(".");
        }
        Path versionDir = manifestDir.getFileName();
        Path apiDir = manifestDir.getParent() != null ? manifestDir.getParent().getFileName() : null;
        if (versionDir != null
                && apiDir != null
                && API_VERSION_DIR.matcher(versionDir.toString()).matches()
                && "api".equals(apiDir.toString())) {
            Path holonRoot = manifestDir.getParent() != null ? manifestDir.getParent().getParent() : null;
            if (holonRoot != null) {
                return holonRoot;
            }
        }
        return manifestDir;
    }

    private static boolean shouldSkipDirectory(Path root, Path dir, String name) {
        if (root.equals(dir)) {
            return false;
        }
        return name.equals(".git")
                || name.equals(".op")
                || name.equals("node_modules")
                || name.equals("vendor")
                || name.equals("build")
                || name.equals("testdata")
                || name.startsWith(".");
    }

    private static String relativePath(Path root, Path dir) {
        Path rel = root.relativize(dir);
        String value = rel.toString().replace('\\', '/');
        return value.isEmpty() ? "." : value;
    }

    private static int pathDepth(String relativePath) {
        String trimmed = relativePath == null ? "" : relativePath.trim().replaceAll("^/+|/+$", "");
        if (trimmed.isEmpty() || ".".equals(trimmed)) {
            return 0;
        }
        return trimmed.split("/").length;
    }

    private static String currentDir() {
        return System.getProperty("user.dir", ".").trim();
    }

    private static Path opPath() {
        String configured = getenvOrProperty("OPPATH");
        if (!configured.isBlank()) {
            return Path.of(configured).toAbsolutePath().normalize();
        }
        return Path.of(System.getProperty("user.home", "."), ".op").toAbsolutePath().normalize();
    }

    private static Path opbin() {
        String configured = getenvOrProperty("OPBIN");
        if (!configured.isBlank()) {
            return Path.of(configured).toAbsolutePath().normalize();
        }
        return opPath().resolve("bin");
    }

    private static Path cacheDir() {
        return opPath().resolve("cache");
    }

    private static String getenvOrProperty(String name) {
        String env = System.getenv(name);
        if (env != null && !env.isBlank()) {
            return env.trim();
        }
        String prop = System.getProperty(name);
        return prop != null ? prop.trim() : "";
    }

    private record RootSpec(Path root, String origin) {
    }
}

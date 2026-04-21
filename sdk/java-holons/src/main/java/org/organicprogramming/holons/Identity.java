package org.organicprogramming.holons;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Objects;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

/** Parse holon.proto manifests into resolved holon metadata. */
public final class Identity {
    public static final String PROTO_MANIFEST_FILE_NAME = "holon.proto";

    private static final Pattern MANIFEST_PATTERN = Pattern.compile(
            "option\\s*\\(\\s*holons\\.v1\\.manifest\\s*\\)\\s*=\\s*\\{");

    private Identity() {
    }

    /** Parsed holon identity. */
    public record HolonIdentity(
            String uuid,
            String givenName,
            String familyName,
            String motto,
            String composer,
            String clade,
            String status,
            String born,
            String version,
            String lang,
            List<String> parents,
            String reproduction,
            String generatedBy,
            String protoStatus,
            List<String> aliases) {
        public HolonIdentity {
            uuid = defaultString(uuid);
            givenName = defaultString(givenName);
            familyName = defaultString(familyName);
            motto = defaultString(motto);
            composer = defaultString(composer);
            clade = defaultString(clade);
            status = defaultString(status);
            born = defaultString(born);
            version = defaultString(version);
            lang = defaultString(lang);
            parents = List.copyOf(parents != null ? parents : List.of());
            reproduction = defaultString(reproduction);
            generatedBy = defaultString(generatedBy);
            protoStatus = defaultString(protoStatus);
            aliases = List.copyOf(aliases != null ? aliases : List.of());
        }

        public String slug() {
            String given = givenName.trim();
            String family = familyName.trim().replaceFirst("\\?$", "");
            if (given.isEmpty() && family.isEmpty()) {
                return "";
            }
            return (given + "-" + family)
                    .trim()
                    .toLowerCase()
                    .replace(" ", "-")
                    .replaceAll("^-+|-+$", "");
        }
    }

    public record ResolvedSkill(
            String name,
            String description,
            String when,
            List<String> steps) {
        public ResolvedSkill {
            name = defaultString(name);
            description = defaultString(description);
            when = defaultString(when);
            steps = List.copyOf(steps != null ? steps : List.of());
        }
    }

    public record ResolvedSequenceParam(
            String name,
            String description,
            boolean required,
            String defaultValue) {
        public ResolvedSequenceParam {
            name = defaultString(name);
            description = defaultString(description);
            defaultValue = defaultString(defaultValue);
        }
    }

    public record ResolvedSequence(
            String name,
            String description,
            List<ResolvedSequenceParam> params,
            List<String> steps) {
        public ResolvedSequence {
            name = defaultString(name);
            description = defaultString(description);
            params = List.copyOf(params != null ? params : List.of());
            steps = List.copyOf(steps != null ? steps : List.of());
        }
    }

    public record ManifestIdentity(
            HolonIdentity identity,
            Path sourcePath) {
        public ManifestIdentity {
            Objects.requireNonNull(identity, "identity");
            Objects.requireNonNull(sourcePath, "sourcePath");
        }
    }

    /** Parsed manifest fields from a holon.proto file. */
    public record ResolvedManifest(
            HolonIdentity identity,
            Path sourcePath,
            String description,
            String kind,
            String buildRunner,
            String buildMain,
            String artifactBinary,
            String artifactPrimary,
            List<String> requiredFiles,
            List<String> memberPaths,
            List<ResolvedSkill> skills,
            List<ResolvedSequence> sequences) {
        public ResolvedManifest {
            Objects.requireNonNull(identity, "identity");
            Objects.requireNonNull(sourcePath, "sourcePath");
            description = defaultString(description);
            kind = defaultString(kind);
            buildRunner = defaultString(buildRunner);
            buildMain = defaultString(buildMain);
            artifactBinary = defaultString(artifactBinary);
            artifactPrimary = defaultString(artifactPrimary);
            requiredFiles = List.copyOf(requiredFiles != null ? requiredFiles : List.of());
            memberPaths = List.copyOf(memberPaths != null ? memberPaths : List.of());
            skills = List.copyOf(skills != null ? skills : List.of());
            sequences = List.copyOf(sequences != null ? sequences : List.of());
        }
    }

    /** Parse a holon.proto file and return its identity. */
    public static HolonIdentity parseHolon(Path path) throws IOException {
        return resolveProtoFile(path).identity();
    }

    /** Parse manifest fields from a holon.proto file. */
    public static ResolvedManifest parseManifest(Path path) throws IOException {
        return resolveProtoFile(path);
    }

    /** Resolve a nearby holon.proto starting from the supplied directory. */
    public static ResolvedManifest resolve(Path root) throws IOException {
        return resolveProtoFile(resolveManifestPath(root));
    }

    /** Preserve the identity-only API used by discover/connect. */
    public static ManifestIdentity resolveManifest(Path root) throws IOException {
        ResolvedManifest resolved = resolve(root);
        return new ManifestIdentity(resolved.identity(), resolved.sourcePath());
    }

    /** Resolve a specific holon.proto file into its full manifest view. */
    public static ResolvedManifest resolveProtoFile(Path path) throws IOException {
        Path resolved = path.toAbsolutePath().normalize();
        if (!Files.isRegularFile(resolved) || !PROTO_MANIFEST_FILE_NAME.equals(fileName(resolved))) {
            throw new IOException(resolved + " is not a " + PROTO_MANIFEST_FILE_NAME + " file");
        }
        return parseManifestFile(resolved);
    }

    public static Path findHolonProto(Path root) throws IOException {
        Path resolved = root.toAbsolutePath().normalize();
        if (Files.isRegularFile(resolved)) {
            return PROTO_MANIFEST_FILE_NAME.equals(fileName(resolved)) ? resolved : null;
        }
        if (!Files.isDirectory(resolved)) {
            return null;
        }

        Path direct = resolved.resolve(PROTO_MANIFEST_FILE_NAME);
        if (Files.isRegularFile(direct)) {
            return direct;
        }

        Path apiV1 = resolved.resolve("api").resolve("v1").resolve(PROTO_MANIFEST_FILE_NAME);
        if (Files.isRegularFile(apiV1)) {
            return apiV1;
        }

        try (Stream<Path> walk = Files.walk(resolved)) {
            return walk
                    .filter(path -> Files.isRegularFile(path) && PROTO_MANIFEST_FILE_NAME.equals(fileName(path)))
                    .sorted(Comparator.comparing(Path::toString))
                    .findFirst()
                    .orElse(null);
        }
    }

    public static Path resolveManifestPath(Path root) throws IOException {
        Path resolved = root.toAbsolutePath().normalize();
        List<Path> searchRoots = new ArrayList<>();
        searchRoots.add(resolved);
        Path fileName = resolved.getFileName();
        Path parent = resolved.getParent();
        if (fileName != null && "protos".equals(fileName.toString()) && parent != null) {
            searchRoots.add(parent);
        } else if (parent != null && !parent.equals(resolved)) {
            searchRoots.add(parent);
        }

        for (Path searchRoot : searchRoots) {
            Path candidate = findHolonProto(searchRoot);
            if (candidate != null) {
                return candidate;
            }
        }

        throw new IOException("no " + PROTO_MANIFEST_FILE_NAME + " found near " + resolved);
    }

    private static ResolvedManifest parseManifestFile(Path path) throws IOException {
        String text = Files.readString(path);
        String manifestBlock = extractManifestBlock(text);
        if (manifestBlock == null) {
            throw new IllegalArgumentException(path + ": missing holons.v1.manifest option in holon.proto");
        }

        String identityBlock = defaultString(extractFirstBlock("identity", manifestBlock));
        String lineageBlock = defaultString(extractFirstBlock("lineage", manifestBlock));
        String buildBlock = defaultString(extractFirstBlock("build", manifestBlock));
        String artifactsBlock = defaultString(extractFirstBlock("artifacts", manifestBlock));
        String requiresBlock = defaultString(extractFirstBlock("requires", manifestBlock));

        return new ResolvedManifest(
                new HolonIdentity(
                        scalar("uuid", identityBlock),
                        scalar("given_name", identityBlock),
                        scalar("family_name", identityBlock),
                        scalar("motto", identityBlock),
                        scalar("composer", identityBlock),
                        scalar("clade", identityBlock),
                        scalar("status", identityBlock),
                        scalar("born", identityBlock),
                        scalar("version", identityBlock),
                        scalar("lang", manifestBlock),
                        stringList("parents", lineageBlock),
                        scalar("reproduction", lineageBlock),
                        scalar("generated_by", lineageBlock),
                        scalar("proto_status", identityBlock),
                        stringList("aliases", identityBlock)),
                path,
                scalar("description", manifestBlock),
                scalar("kind", manifestBlock),
                scalar("runner", buildBlock),
                scalar("main", buildBlock),
                scalar("binary", artifactsBlock),
                scalar("primary", artifactsBlock),
                stringList("files", requiresBlock),
                memberPaths(buildBlock),
                resolvedSkills(manifestBlock),
                resolvedSequences(manifestBlock));
    }

    private static List<String> memberPaths(String buildBlock) {
        List<String> paths = new ArrayList<>();
        for (String block : blockList("members", buildBlock)) {
            String path = scalar("path", block);
            if (!path.isBlank()) {
                paths.add(path);
            }
        }
        return paths;
    }

    private static List<ResolvedSkill> resolvedSkills(String manifestBlock) {
        List<ResolvedSkill> skills = new ArrayList<>();
        for (String block : blockList("skills", manifestBlock)) {
            skills.add(new ResolvedSkill(
                    scalar("name", block),
                    scalar("description", block),
                    scalar("when", block),
                    stringList("steps", block)));
        }
        return skills;
    }

    private static List<ResolvedSequence> resolvedSequences(String manifestBlock) {
        List<ResolvedSequence> sequences = new ArrayList<>();
        for (String block : blockList("sequences", manifestBlock)) {
            List<ResolvedSequenceParam> params = new ArrayList<>();
            for (String paramBlock : blockList("params", block)) {
                params.add(new ResolvedSequenceParam(
                        scalar("name", paramBlock),
                        scalar("description", paramBlock),
                        Boolean.parseBoolean(scalar("required", paramBlock)),
                        scalar("default", paramBlock)));
            }
            sequences.add(new ResolvedSequence(
                    scalar("name", block),
                    scalar("description", block),
                    params,
                    stringList("steps", block)));
        }
        return sequences;
    }

    private static List<String> blockList(String name, String source) {
        List<String> blocks = new ArrayList<>();
        if (source == null || source.isBlank()) {
            return blocks;
        }

        Matcher inlineMatcher = Pattern.compile("\\b" + Pattern.quote(name) + "\\s*:\\s*\\{").matcher(source);
        while (inlineMatcher.find()) {
            int braceIndex = source.indexOf('{', inlineMatcher.start());
            int endIndex = balancedBlockEnd(source, braceIndex);
            if (endIndex >= 0) {
                blocks.add(source.substring(braceIndex + 1, endIndex));
            }
        }

        for (String arrayBody : arrayContents(name, source)) {
            blocks.addAll(extractInlineBlocks(arrayBody));
        }
        return blocks;
    }

    private static List<String> extractInlineBlocks(String source) {
        List<String> blocks = new ArrayList<>();
        boolean insideString = false;
        boolean escaped = false;

        for (int index = 0; index < source.length(); index++) {
            char ch = source.charAt(index);
            if (insideString) {
                if (escaped) {
                    escaped = false;
                } else if (ch == '\\') {
                    escaped = true;
                } else if (ch == '"') {
                    insideString = false;
                }
                continue;
            }

            if (ch == '"') {
                insideString = true;
                continue;
            }

            if (ch != '{') {
                continue;
            }

            int endIndex = balancedBlockEnd(source, index);
            if (endIndex >= 0) {
                blocks.add(source.substring(index + 1, endIndex));
                index = endIndex;
            }
        }

        return blocks;
    }

    private static String extractManifestBlock(String source) {
        Matcher matcher = MANIFEST_PATTERN.matcher(source);
        if (!matcher.find()) {
            return null;
        }
        int braceIndex = source.indexOf('{', matcher.start());
        return braceIndex >= 0 ? balancedBlockContents(source, braceIndex) : null;
    }

    private static String extractFirstBlock(String name, String source) {
        Matcher matcher = Pattern.compile("\\b" + Pattern.quote(name) + "\\s*:\\s*\\{").matcher(source);
        if (!matcher.find()) {
            return null;
        }
        int braceIndex = source.indexOf('{', matcher.start());
        return braceIndex >= 0 ? balancedBlockContents(source, braceIndex) : null;
    }

    private static String scalar(String name, String source) {
        Matcher quoted = Pattern.compile(
                "\\b" + Pattern.quote(name) + "\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)\"")
                .matcher(source);
        if (quoted.find()) {
            return unescapeProtoString(quoted.group(1));
        }

        Matcher bare = Pattern.compile(
                "\\b" + Pattern.quote(name) + "\\s*:\\s*([^\\s,\\]\\}]+)")
                .matcher(source);
        return bare.find() ? bare.group(1) : "";
    }

    private static List<String> stringList(String name, String source) {
        List<String> values = new ArrayList<>();
        for (String arrayBody : arrayContents(name, source)) {
            Matcher token = Pattern.compile("\"((?:[^\"\\\\]|\\\\.)*)\"|([^\\s,\\]]+)").matcher(arrayBody);
            while (token.find()) {
                if (token.group(1) != null) {
                    values.add(unescapeProtoString(token.group(1)));
                } else if (token.group(2) != null) {
                    values.add(token.group(2));
                }
            }
        }
        return values;
    }

    private static List<String> arrayContents(String name, String source) {
        List<String> arrays = new ArrayList<>();
        if (source == null || source.isBlank()) {
            return arrays;
        }

        Matcher matcher = Pattern.compile("\\b" + Pattern.quote(name) + "\\s*:\\s*\\[").matcher(source);
        while (matcher.find()) {
            int openIndex = source.indexOf('[', matcher.start());
            int endIndex = balancedRangeEnd(source, openIndex, '[', ']');
            if (endIndex >= 0) {
                arrays.add(source.substring(openIndex + 1, endIndex));
            }
        }
        return arrays;
    }

    private static String balancedBlockContents(String source, int openingBrace) {
        int endIndex = balancedBlockEnd(source, openingBrace);
        return endIndex >= 0 ? source.substring(openingBrace + 1, endIndex) : null;
    }

    private static int balancedBlockEnd(String source, int openingBrace) {
        return balancedRangeEnd(source, openingBrace, '{', '}');
    }

    private static int balancedRangeEnd(String source, int openingIndex, char open, char close) {
        int depth = 0;
        boolean insideString = false;
        boolean escaped = false;

        for (int index = openingIndex; index < source.length(); index++) {
            char ch = source.charAt(index);
            if (insideString) {
                if (escaped) {
                    escaped = false;
                } else if (ch == '\\') {
                    escaped = true;
                } else if (ch == '"') {
                    insideString = false;
                }
                continue;
            }

            if (ch == '"') {
                insideString = true;
            } else if (ch == open) {
                depth += 1;
            } else if (ch == close) {
                depth -= 1;
                if (depth == 0) {
                    return index;
                }
            }
        }

        return -1;
    }

    private static String unescapeProtoString(String value) {
        return value.replace("\\\"", "\"").replace("\\\\", "\\");
    }

    private static String defaultString(String value) {
        return value != null ? value : "";
    }

    private static String fileName(Path path) {
        Path fileName = path.getFileName();
        return fileName != null ? fileName.toString() : "";
    }
}

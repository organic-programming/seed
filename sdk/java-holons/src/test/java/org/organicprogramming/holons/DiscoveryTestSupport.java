package org.organicprogramming.holons;

import com.google.gson.Gson;
import org.organicprogramming.holons.DiscoveryTypes.DiscoverResult;
import org.organicprogramming.holons.DiscoveryTypes.HolonInfo;
import org.organicprogramming.holons.DiscoveryTypes.HolonRef;
import org.organicprogramming.holons.DiscoveryTypes.IdentityInfo;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

final class DiscoveryTestSupport {
    private static final Gson GSON = new Gson();

    private DiscoveryTestSupport() {
    }

    static RuntimeFixture runtimeFixture(Path root) throws IOException {
        Path opHome = root.resolve("runtime");
        Path opBin = opHome.resolve("bin");
        Files.createDirectories(opBin);
        return new RuntimeFixture(root, opHome, opBin);
    }

    static RuntimeEnv runtimeEnv(RuntimeFixture fixture) {
        return new RuntimeEnv(fixture.root(), fixture.opHome(), fixture.opBin());
    }

    static List<String> slugs(DiscoverResult result) {
        List<String> slugs = new ArrayList<>();
        if (result == null || result.found == null) {
            return slugs;
        }
        for (HolonRef ref : result.found) {
            if (ref != null && ref.info != null) {
                slugs.add(ref.info.slug);
            }
        }
        slugs.sort(String::compareTo);
        return slugs;
    }

    static void writePackageHolon(Path dir, PackageSeed seed) throws IOException {
        Files.createDirectories(dir);

        String slug = seed.slug != null && !seed.slug.isBlank()
                ? seed.slug
                : slugFor(seed.givenName, seed.familyName);
        String runner = seed.runner != null && !seed.runner.isBlank() ? seed.runner : "go-module";
        String kind = seed.kind != null && !seed.kind.isBlank() ? seed.kind : "native";

        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("schema", "holon-package/v1");
        payload.put("slug", slug);
        payload.put("uuid", seed.uuid);

        Map<String, Object> identity = new LinkedHashMap<>();
        identity.put("given_name", seed.givenName);
        identity.put("family_name", seed.familyName);
        identity.put("motto", seed.motto == null ? "" : seed.motto);
        identity.put("aliases", seed.aliases == null ? List.of() : seed.aliases);
        payload.put("identity", identity);

        payload.put("lang", seed.lang == null ? "go" : seed.lang);
        payload.put("runner", runner);
        payload.put("status", seed.status == null ? "draft" : seed.status);
        payload.put("kind", kind);
        payload.put("transport", seed.transport == null ? "" : seed.transport);
        payload.put("entrypoint", seed.entrypoint == null ? slug : seed.entrypoint);
        payload.put("architectures", seed.architectures == null ? List.of() : seed.architectures);
        payload.put("has_dist", seed.hasDist);
        payload.put("has_source", seed.hasSource);

        Files.writeString(dir.resolve(".holon.json"), GSON.toJson(payload) + System.lineSeparator());
    }

    static HolonInfo holonInfo(String slug, String uuid, String givenName, String familyName) {
        HolonInfo info = new HolonInfo();
        info.slug = slug;
        info.uuid = uuid;
        info.identity = new IdentityInfo();
        info.identity.givenName = givenName;
        info.identity.familyName = familyName;
        info.identity.aliases = new ArrayList<>();
        info.lang = "go";
        info.runner = "go-module";
        info.status = "draft";
        info.kind = "native";
        return info;
    }

    static HolonRef holonRef(Path path, String slug, String uuid, String givenName, String familyName) {
        HolonRef ref = new HolonRef();
        ref.url = path.toAbsolutePath().normalize().toUri().toString();
        ref.info = holonInfo(slug, uuid, givenName, familyName);
        return ref;
    }

    static String platformTag() {
        return normalizedOs() + "_" + normalizedArch();
    }

    static String slugFor(String givenName, String familyName) {
        return (safe(givenName) + "-" + safe(familyName))
                .trim()
                .toLowerCase()
                .replace(" ", "-")
                .replaceAll("^-+|-+$", "");
    }

    static void writeExecutable(Path path, String content) throws IOException {
        Files.createDirectories(path.getParent());
        Files.writeString(path, content);
        if (!path.toFile().setExecutable(true)) {
            throw new IOException("failed to mark executable: " + path);
        }
    }

    private static String normalizedOs() {
        String os = System.getProperty("os.name", "").toLowerCase();
        if (os.contains("mac") || os.contains("darwin")) {
            return "darwin";
        }
        if (os.contains("win")) {
            return "windows";
        }
        return "linux";
    }

    private static String normalizedArch() {
        String arch = System.getProperty("os.arch", "").toLowerCase();
        if ("aarch64".equals(arch) || "arm64".equals(arch)) {
            return "arm64";
        }
        if ("x86_64".equals(arch) || "amd64".equals(arch)) {
            return "amd64";
        }
        return arch.replace('-', '_');
    }

    private static String safe(String value) {
        return value == null ? "" : value;
    }

    record RuntimeFixture(Path root, Path opHome, Path opBin) {
    }

    static final class RuntimeEnv implements AutoCloseable {
        private final String previousUserDir = System.getProperty("user.dir");
        private final String previousOpPath = System.getProperty("OPPATH");
        private final String previousOpBin = System.getProperty("OPBIN");

        RuntimeEnv(Path root, Path opHome, Path opBin) {
            System.setProperty("user.dir", root.toString());
            System.setProperty("OPPATH", opHome.toString());
            System.setProperty("OPBIN", opBin.toString());
        }

        @Override
        public void close() {
            restore("user.dir", previousUserDir);
            restore("OPPATH", previousOpPath);
            restore("OPBIN", previousOpBin);
        }

        private static void restore(String key, String value) {
            if (value == null) {
                System.clearProperty(key);
            } else {
                System.setProperty(key, value);
            }
        }
    }

    static final class PackageSeed {
        String slug;
        String uuid;
        String givenName;
        String familyName;
        String motto = "";
        String lang = "go";
        String runner = "go-module";
        String status = "draft";
        String kind = "native";
        String transport = "";
        String entrypoint;
        List<String> architectures = List.of();
        boolean hasDist;
        boolean hasSource;
        List<String> aliases = List.of();

        PackageSeed(String uuid, String givenName, String familyName) {
            this.uuid = uuid;
            this.givenName = givenName;
            this.familyName = familyName;
        }

        PackageSeed slug(String value) {
            this.slug = value;
            return this;
        }

        PackageSeed entrypoint(String value) {
            this.entrypoint = value;
            return this;
        }

        PackageSeed aliases(List<String> value) {
            this.aliases = value;
            return this;
        }

        PackageSeed transport(String value) {
            this.transport = value;
            return this;
        }

        PackageSeed hasDist(boolean value) {
            this.hasDist = value;
            return this;
        }

        PackageSeed hasSource(boolean value) {
            this.hasSource = value;
            return this;
        }
    }
}

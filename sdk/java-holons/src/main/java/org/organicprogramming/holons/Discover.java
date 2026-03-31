package org.organicprogramming.holons;

import com.google.gson.Gson;
import com.google.gson.annotations.SerializedName;
import io.grpc.CallOptions;
import io.grpc.ConnectivityState;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.stub.ClientCalls;
import org.organicprogramming.holons.DiscoveryTypes.DiscoverResult;
import org.organicprogramming.holons.DiscoveryTypes.HolonInfo;
import org.organicprogramming.holons.DiscoveryTypes.HolonRef;
import org.organicprogramming.holons.DiscoveryTypes.IdentityInfo;
import org.organicprogramming.holons.DiscoveryTypes.ResolveResult;

import java.io.BufferedReader;
import java.io.Closeable;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.nio.file.FileVisitResult;
import java.nio.file.FileVisitor;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.SimpleFileVisitor;
import java.nio.file.attribute.BasicFileAttributes;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;

/**
 * Uniform Phase 1 discovery API.
 *
 * <p>The {@code expression} and {@code root} parameters are nullable in practice even though the
 * Java signature uses {@link String} references. Pass {@code null} for {@code expression} to return
 * every match in the requested layers. Pass {@code null} for {@code root} to use the current
 * working directory. Empty {@code root} strings are rejected.
 */
public final class Discover {
    public static final int LOCAL = DiscoveryTypes.LOCAL;
    public static final int PROXY = DiscoveryTypes.PROXY;
    public static final int DELEGATED = DiscoveryTypes.DELEGATED;

    public static final int SIBLINGS = DiscoveryTypes.SIBLINGS;
    public static final int CWD = DiscoveryTypes.CWD;
    public static final int SOURCE = DiscoveryTypes.SOURCE;
    public static final int BUILT = DiscoveryTypes.BUILT;
    public static final int INSTALLED = DiscoveryTypes.INSTALLED;
    public static final int CACHED = DiscoveryTypes.CACHED;
    public static final int ALL = DiscoveryTypes.ALL;

    public static final int NO_LIMIT = DiscoveryTypes.NO_LIMIT;
    public static final int NO_TIMEOUT = DiscoveryTypes.NO_TIMEOUT;

    private static final int DEFAULT_PROBE_TIMEOUT_MS = 5_000;
    private static final Gson GSON = new Gson();

    private static volatile PackageProbe packageProbe;
    private static volatile SourceBridge sourceBridge;
    private static volatile BundleRootResolver bundleRootResolver = Discover::defaultBundleHolonsRoot;

    private Discover() {
    }

    @FunctionalInterface
    interface PackageProbe {
        HolonInfo probe(Path packageDir) throws Exception;
    }

    @FunctionalInterface
    interface SourceBridge {
        DiscoverResult discover(String expression, Path root, int limit, int timeout) throws Exception;
    }

    @FunctionalInterface
    interface BundleRootResolver {
        Path resolve() throws Exception;
    }

    /**
     * Discover matching holons in the requested local layers.
     *
     * <p>{@code expression} and {@code root} may be {@code null}. {@code root == null} uses the
     * current working directory; empty {@code root} values return an error. {@code specifiers == 0}
     * is treated as {@link #ALL}.
     */
    public static DiscoverResult Discover(
            int scope,
            String expression,
            String root,
            int specifiers,
            int limit,
            int timeout) {
        DiscoverResult result = new DiscoverResult();

        if (scope != LOCAL) {
            result.error = "scope %d not supported".formatted(scope);
            return result;
        }
        if (specifiers < 0 || specifiers > ALL) {
            result.error = "invalid specifiers 0x%02X: valid range is 0x00-0x3F".formatted(specifiers);
            return result;
        }
        if (specifiers == 0) {
            specifiers = ALL;
        }
        if (limit < 0) {
            return result;
        }

        try {
            String normalizedExpression = normalizeExpression(expression);
            if (isDeferredDirectExpression(normalizedExpression)) {
                result.error = "direct URL expressions are not supported";
                return result;
            }

            Path searchRoot = resolveDiscoverRoot(root);

            Path candidatePath = pathExpressionCandidate(normalizedExpression, searchRoot);
            if (candidatePath != null) {
                PathLookup lookup = discoverRefAtPath(candidatePath, timeout);
                if (lookup.found() && lookup.ref() != null) {
                    result.found.add(lookup.ref());
                }
                result.found = applyLimit(result.found, limit);
                return result;
            }

            List<ScanEntry> entries = discoverEntries(searchRoot, normalizedExpression, specifiers, limit, timeout);
            for (ScanEntry entry : entries) {
                if (!matchesExpression(entry, normalizedExpression)) {
                    continue;
                }
                result.found.add(copyRef(entry.ref()));
                if (limit > 0 && result.found.size() >= limit) {
                    break;
                }
            }
            return result;
        } catch (Exception e) {
            result.error = messageOf(e);
            return result;
        }
    }

    /**
     * Resolve the first matching holon.
     *
     * <p>{@code expression} and {@code root} may be {@code null}; a {@code null} or blank
     * {@code expression} resolves to a not-found error.
     */
    public static ResolveResult resolve(int scope, String expression, String root, int specifiers, int timeout) {
        ResolveResult result = new ResolveResult();
        DiscoverResult discovered = Discover(scope, expression, root, specifiers, 1, timeout);
        if (!discovered.error.isBlank()) {
            result.error = discovered.error;
            return result;
        }
        if (discovered.found.isEmpty()) {
            result.error = "holon %s not found".formatted(quoted(expression));
            return result;
        }

        HolonRef ref = copyRef(discovered.found.get(0));
        result.ref = ref;
        if (ref.error != null && !ref.error.isBlank()) {
            result.error = ref.error;
        }
        return result;
    }

    static void setPackageProbeForTests(PackageProbe probe) {
        packageProbe = probe;
    }

    static void setSourceBridgeForTests(SourceBridge bridge) {
        sourceBridge = bridge;
    }

    static void setBundleRootResolverForTests(BundleRootResolver resolver) {
        bundleRootResolver = resolver != null ? resolver : Discover::defaultBundleHolonsRoot;
    }

    private static List<ScanEntry> discoverEntries(
            Path root,
            String expression,
            int specifiers,
            int limit,
            int timeout) throws Exception {
        Map<String, ScanEntry> seen = new LinkedHashMap<>();

        for (Layer layer : layers()) {
            if ((specifiers & layer.flag()) == 0) {
                continue;
            }

            for (ScanEntry entry : layer.scan(root, expression, limit, timeout)) {
                String key = entryKey(entry.ref());
                if (key.isBlank() || seen.containsKey(key)) {
                    continue;
                }
                seen.put(key, entry);
            }
        }

        return new ArrayList<>(seen.values());
    }

    private static List<Layer> layers() {
        return List.of(
                new Layer(SIBLINGS, "siblings", (root, expression, limit, timeout) -> {
                    Path bundleRoot = bundleRootResolver.resolve();
                    if (bundleRoot == null) {
                        return List.of();
                    }
                    return discoverPackagesDirect(bundleRoot, bundleRoot, "siblings");
                }),
                new Layer(CWD, "cwd", (root, expression, limit, timeout) -> discoverPackagesRecursive(root, root, "cwd")),
                new Layer(SOURCE, "source", (root, expression, limit, timeout) -> discoverSourceEntries(root, expression, limit, timeout)),
                new Layer(BUILT, "built", (root, expression, limit, timeout) -> discoverPackagesDirect(root.resolve(".op").resolve("build"), root, "built")),
                new Layer(INSTALLED, "installed", (root, expression, limit, timeout) -> discoverPackagesDirect(opBin(), opBin(), "installed")),
                new Layer(CACHED, "cached", (root, expression, limit, timeout) -> discoverPackagesRecursive(cacheDir(), cacheDir(), "cached")));
    }

    private static List<ScanEntry> discoverPackagesDirect(Path scanRoot, Path relativeRoot, String origin) throws IOException {
        return discoverPackagesFromDirs(relativeRoot, origin, packageDirsDirect(scanRoot));
    }

    private static List<ScanEntry> discoverPackagesRecursive(Path scanRoot, Path relativeRoot, String origin) throws IOException {
        return discoverPackagesFromDirs(relativeRoot, origin, packageDirsRecursive(scanRoot));
    }

    private static List<ScanEntry> discoverPackagesFromDirs(Path relativeRoot, String origin, List<Path> dirs) throws IOException {
        Map<String, ScanEntry> entriesByKey = new HashMap<>();
        List<String> keys = new ArrayList<>();

        for (Path dir : dirs) {
            ScanEntry entry = null;
            try {
                entry = loadPackageEntry(relativeRoot, dir, origin);
            } catch (Exception ignored) {
                // Fall back to Describe probing below.
            }
            if (entry == null) {
                try {
                    entry = probePackageEntry(relativeRoot, dir, origin, DEFAULT_PROBE_TIMEOUT_MS);
                } catch (Exception ignored) {
                    continue;
                }
            }

            String key = entryKey(entry.ref());
            if (key.isBlank()) {
                key = dir.toAbsolutePath().normalize().toString();
            }

            ScanEntry existing = entriesByKey.get(key);
            if (existing != null) {
                if (shouldReplaceEntry(existing, entry)) {
                    entriesByKey.put(key, entry);
                }
                continue;
            }

            entriesByKey.put(key, entry);
            keys.add(key);
        }

        List<ScanEntry> entries = new ArrayList<>();
        for (String key : keys) {
            ScanEntry entry = entriesByKey.get(key);
            if (entry != null) {
                entries.add(entry);
            }
        }
        entries.sort(Comparator
                .comparing(ScanEntry::relativePath)
                .thenComparing(entry -> safeUuid(entry.ref())));
        return entries;
    }

    private static List<Path> packageDirsDirect(Path root) throws IOException {
        Path resolved = absolute(root);
        if (!Files.isDirectory(resolved)) {
            return List.of();
        }

        List<Path> dirs = new ArrayList<>();
        try (var stream = Files.list(resolved)) {
            stream
                    .filter(Files::isDirectory)
                    .filter(path -> path.getFileName() != null && path.getFileName().toString().endsWith(".holon"))
                    .sorted()
                    .forEach(dirs::add);
        }
        return dirs;
    }

    private static List<Path> packageDirsRecursive(Path root) throws IOException {
        Path resolved = absolute(root);
        if (!Files.isDirectory(resolved)) {
            return List.of();
        }

        List<Path> dirs = new ArrayList<>();
        Files.walkFileTree(resolved, recursivePackageVisitor(resolved, dirs));
        dirs.sort(Comparator.naturalOrder());
        return dirs;
    }

    private static FileVisitor<Path> recursivePackageVisitor(Path root, List<Path> dirs) {
        return new SimpleFileVisitor<>() {
            @Override
            public FileVisitResult preVisitDirectory(Path dir, BasicFileAttributes attrs) {
                if (root.equals(dir)) {
                    return FileVisitResult.CONTINUE;
                }

                String name = dir.getFileName() == null ? "" : dir.getFileName().toString();
                if (name.endsWith(".holon")) {
                    dirs.add(dir);
                    return FileVisitResult.SKIP_SUBTREE;
                }
                if (shouldSkipDirectory(root, dir, name)) {
                    return FileVisitResult.SKIP_SUBTREE;
                }
                return FileVisitResult.CONTINUE;
            }
        };
    }

    private static List<ScanEntry> discoverSourceEntries(Path root, String expression, int limit, int timeout) throws Exception {
        SourceBridge bridge = sourceBridge;
        if (bridge == null) {
            return List.of();
        }

        DiscoverResult bridged = bridge.discover(expression, root, limit, timeout);
        if (bridged == null) {
            return List.of();
        }
        if (bridged.error != null && !bridged.error.isBlank()) {
            throw new IOException(bridged.error);
        }
        if (bridged == null || bridged.found == null || bridged.found.isEmpty()) {
            return List.of();
        }

        Map<String, ScanEntry> byKey = new LinkedHashMap<>();
        for (HolonRef ref : bridged.found) {
            if (ref == null) {
                continue;
            }
            ScanEntry entry = scanEntryFromRef(root, ref);
            String key = entryKey(entry.ref());
            if (key.isBlank() || byKey.containsKey(key)) {
                continue;
            }
            byKey.put(key, entry);
        }
        return new ArrayList<>(byKey.values());
    }

    private static boolean matchesExpression(ScanEntry entry, String expression) {
        if (expression == null) {
            return true;
        }

        String needle = expression.trim();
        if (needle.isEmpty()) {
            return false;
        }

        HolonInfo info = entry.ref().info;
        if (info != null) {
            if (needle.equals(info.slug)) {
                return true;
            }
            if (!safe(info.uuid).isBlank() && info.uuid.startsWith(needle)) {
                return true;
            }
            if (info.identity != null && info.identity.aliases != null) {
                for (String alias : info.identity.aliases) {
                    if (needle.equals(alias)) {
                        return true;
                    }
                }
            }
        }

        String basename = entry.basename();
        return basename.equals(needle);
    }

    private static PathLookup discoverRefAtPath(Path path, int timeout) throws Exception {
        Path absolutePath = absolute(path);
        if (!Files.exists(absolutePath)) {
            return new PathLookup(null, false);
        }

        if (Files.isDirectory(absolutePath)) {
            String name = absolutePath.getFileName() == null ? "" : absolutePath.getFileName().toString();
            if (name.endsWith(".holon") || Files.isRegularFile(absolutePath.resolve(".holon.json"))) {
                try {
                    return new PathLookup(loadPackageEntry(absolutePath.getParent(), absolutePath, "path").ref(), true);
                } catch (Exception ignored) {
                    try {
                        return new PathLookup(probePackageEntry(absolutePath.getParent(), absolutePath, "path", timeout).ref(), true);
                    } catch (Exception probeError) {
                        return new PathLookup(errorRef(fileURL(absolutePath), messageOf(probeError)), true);
                    }
                }
            }

            List<ScanEntry> entries = discoverSourceEntries(absolutePath, absolutePath.toString(), 1, timeout);
            if (entries.size() == 1) {
                return new PathLookup(copyRef(entries.get(0).ref()), true);
            }
            for (ScanEntry entry : entries) {
                Path entryPath = filePath(entry.ref());
                if (entryPath != null && absolute(entryPath).equals(absolutePath)) {
                    return new PathLookup(copyRef(entry.ref()), true);
                }
            }
            return new PathLookup(null, false);
        }

        if (Identity.PROTO_MANIFEST_FILE_NAME.equals(absolutePath.getFileName().toString())) {
            List<ScanEntry> entries = discoverSourceEntries(absolutePath.getParent(), absolutePath.toString(), 1, timeout);
            if (entries.size() == 1) {
                return new PathLookup(copyRef(entries.get(0).ref()), true);
            }
            return new PathLookup(null, false);
        }

        try {
            HolonInfo info = probeBinaryPath(absolutePath, timeout);
            return new PathLookup(newRef(fileURL(absolutePath), info), true);
        } catch (Exception e) {
            return new PathLookup(errorRef(fileURL(absolutePath), messageOf(e)), true);
        }
    }

    private static Path pathExpressionCandidate(String expression, Path root) {
        if (expression == null) {
            return null;
        }

        String trimmed = expression.trim();
        if (trimmed.isEmpty()) {
            return null;
        }
        if (trimmed.regionMatches(true, 0, "file://", 0, "file://".length())) {
            return pathFromFileURL(trimmed);
        }
        if (!(Path.of(trimmed).isAbsolute()
                || trimmed.startsWith(".")
                || trimmed.contains("/")
                || trimmed.contains("\\")
                || trimmed.endsWith(".holon"))) {
            return null;
        }
        return Path.of(trimmed).isAbsolute() ? Path.of(trimmed) : root.resolve(trimmed);
    }

    private static boolean isDeferredDirectExpression(String expression) {
        if (expression == null || expression.isBlank()) {
            return false;
        }
        return expression.contains("://") && !expression.regionMatches(true, 0, "file://", 0, "file://".length());
    }

    private static ScanEntry loadPackageEntry(Path relativeRoot, Path dir, String origin) throws IOException {
        HolonPackageJson payload = GSON.fromJson(Files.readString(dir.resolve(".holon.json")), HolonPackageJson.class);
        if (payload == null) {
            throw new IOException("empty .holon.json");
        }
        String schema = safe(payload.schema);
        if (!schema.isBlank() && !"holon-package/v1".equals(schema)) {
            throw new IOException("unsupported .holon.json schema " + schema);
        }

        HolonInfo info = new HolonInfo();
        info.slug = safe(payload.slug);
        info.uuid = safe(payload.uuid);
        info.identity = toIdentityInfo(payload.identity);
        info.lang = safe(payload.lang);
        info.runner = safe(payload.runner);
        info.status = safe(payload.status);
        info.kind = safe(payload.kind);
        info.transport = safe(payload.transport);
        info.entrypoint = safe(payload.entrypoint);
        info.architectures = copyList(payload.architectures);
        info.hasDist = payload.hasDist;
        info.hasSource = payload.hasSource;
        if (info.slug.isBlank()) {
            info.slug = slugForIdentity(info.identity);
        }

        Path absoluteDir = absolute(dir);
        return new ScanEntry(
                relativePath(relativeRoot, absoluteDir),
                basename(absoluteDir),
                newRef(fileURL(absoluteDir), info));
    }

    private static ScanEntry probePackageEntry(Path relativeRoot, Path dir, String origin, int timeout) throws Exception {
        HolonInfo info = null;
        if (packageProbe != null) {
            info = packageProbe.probe(absolute(dir));
        }
        if (info == null) {
            info = probeBinaryPath(packageBinaryPath(dir), timeout);
        }
        info = copyInfo(info);
        info.hasDist = true;
        info.hasSource = false;
        if (info.slug.isBlank()) {
            info.slug = basename(dir);
        }

        Path absoluteDir = absolute(dir);
        return new ScanEntry(
                relativePath(relativeRoot, absoluteDir),
                basename(absoluteDir),
                newRef(fileURL(absoluteDir), info));
    }

    private static HolonInfo probeBinaryPath(Path binaryPath, int timeout) throws Exception {
        Path absoluteBinary = absolute(binaryPath);
        if (!Files.isRegularFile(absoluteBinary)) {
            throw new IOException(absoluteBinary + " is not a file");
        }

        Process process = new ProcessBuilder(absoluteBinary.toString(), "serve", "--listen", "stdio://").start();
        ManagedChannel channel = null;
        try (StdioBridge bridge = new StdioBridge(process)) {
            waitForProcessStartup(process, bridge, timeout);
            channel = ManagedChannelBuilder
                    .forAddress("127.0.0.1", bridge.port())
                    .usePlaintext()
                    .build();
            waitForReady(channel, effectiveTimeout(timeout));

            holons.v1.Describe.DescribeResponse response = ClientCalls.blockingUnaryCall(
                    channel,
                    Describe.describeMethod(),
                    CallOptions.DEFAULT.withDeadlineAfter(effectiveTimeout(timeout).toMillis(), TimeUnit.MILLISECONDS),
                    holons.v1.Describe.DescribeRequest.getDefaultInstance());
            return holonInfoFromDescribe(response);
        } finally {
            if (channel != null) {
                channel.shutdownNow();
                try {
                    channel.awaitTermination(1, TimeUnit.SECONDS);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }
            stopProcess(process);
        }
    }

    private static HolonInfo holonInfoFromDescribe(holons.v1.Describe.DescribeResponse response) throws IOException {
        if (response == null || !response.hasManifest()) {
            throw new IOException("Describe returned no manifest");
        }

        holons.v1.Manifest.HolonManifest manifest = response.getManifest();
        if (!manifest.hasIdentity()) {
            throw new IOException("Describe returned no manifest identity");
        }

        holons.v1.Manifest.HolonManifest.Identity identity = manifest.getIdentity();
        HolonInfo info = new HolonInfo();
        info.uuid = identity.getUuid();
        info.identity = new IdentityInfo();
        info.identity.givenName = identity.getGivenName();
        info.identity.familyName = identity.getFamilyName();
        info.identity.motto = identity.getMotto();
        info.identity.aliases = new ArrayList<>(identity.getAliasesList());
        info.slug = slugForIdentity(info.identity);
        info.lang = manifest.getLang();
        info.runner = manifest.hasBuild() ? manifest.getBuild().getRunner() : "";
        info.status = identity.getStatus();
        info.kind = manifest.getKind();
        info.transport = manifest.getTransport();
        info.entrypoint = manifest.hasArtifacts() ? manifest.getArtifacts().getBinary() : "";
        info.architectures = new ArrayList<>(manifest.getPlatformsList());
        return info;
    }

    private static Path packageBinaryPath(Path dir) throws IOException {
        Path archDir = dir.resolve("bin").resolve(currentPlatformTag());
        if (!Files.isDirectory(archDir)) {
            throw new IOException("package binary directory not found: " + archDir);
        }

        List<Path> candidates = new ArrayList<>();
        try (var stream = Files.list(archDir)) {
            stream
                    .filter(Files::isRegularFile)
                    .sorted()
                    .forEach(candidates::add);
        }
        if (candidates.isEmpty()) {
            throw new IOException("no package binary found in " + archDir);
        }
        return candidates.get(0);
    }

    private static boolean shouldReplaceEntry(ScanEntry current, ScanEntry next) {
        return pathDepth(next.relativePath()) < pathDepth(current.relativePath());
    }

    private static boolean shouldSkipDirectory(Path root, Path dir, String name) {
        if (root.equals(dir)) {
            return false;
        }
        if (name.endsWith(".holon")) {
            return false;
        }
        return ".git".equals(name)
                || ".op".equals(name)
                || "node_modules".equals(name)
                || "vendor".equals(name)
                || "build".equals(name)
                || "testdata".equals(name)
                || name.startsWith(".");
    }

    private static Path resolveDiscoverRoot(String root) throws IOException {
        if (root == null) {
            return absolute(Path.of(currentDir()));
        }

        String trimmed = root.trim();
        if (trimmed.isEmpty()) {
            throw new IOException("root cannot be empty");
        }

        Path resolved = absolute(Path.of(trimmed));
        if (!Files.isDirectory(resolved)) {
            throw new IOException("root %s is not a directory".formatted(quoted(root)));
        }
        return resolved;
    }

    private static Path defaultBundleHolonsRoot() throws IOException {
        String command = ProcessHandle.current().info().command().orElse("").trim();
        if (command.isEmpty()) {
            return null;
        }

        Path current = absolute(Path.of(command));
        while (current != null) {
            if (current.toString().toLowerCase(Locale.ROOT).endsWith(".app")) {
                Path candidate = current.resolve("Contents").resolve("Resources").resolve("Holons");
                if (Files.isDirectory(candidate)) {
                    return candidate;
                }
            }
            current = current.getParent();
        }
        return null;
    }

    private static Path opPath() {
        String configured = getenvOrProperty("OPPATH");
        if (!configured.isBlank()) {
            return absolute(Path.of(configured));
        }
        return absolute(Path.of(System.getProperty("user.home", ".")).resolve(".op"));
    }

    private static Path opBin() {
        String configured = getenvOrProperty("OPBIN");
        if (!configured.isBlank()) {
            return absolute(Path.of(configured));
        }
        return opPath().resolve("bin");
    }

    private static Path cacheDir() {
        return opPath().resolve("cache");
    }

    private static String currentPlatformTag() {
        return normalizedOs() + "_" + normalizedArch();
    }

    private static String normalizedOs() {
        String os = System.getProperty("os.name", "").toLowerCase(Locale.ROOT);
        if (os.contains("mac") || os.contains("darwin")) {
            return "darwin";
        }
        if (os.contains("win")) {
            return "windows";
        }
        return "linux";
    }

    private static String normalizedArch() {
        String arch = System.getProperty("os.arch", "").toLowerCase(Locale.ROOT);
        if (arch.equals("aarch64") || arch.equals("arm64")) {
            return "arm64";
        }
        if (arch.equals("x86_64") || arch.equals("amd64")) {
            return "amd64";
        }
        return arch.replace('-', '_');
    }

    private static void waitForProcessStartup(Process process, StdioBridge bridge, int timeout) throws IOException {
        long startupWindowMs = Math.max(1L, Math.min(effectiveTimeout(timeout).toMillis(), 200L));
        long deadlineNanos = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(startupWindowMs);
        while (System.nanoTime() < deadlineNanos) {
            if (!process.isAlive()) {
                throw new IOException("holon exited before stdio startup" + suffix(bridge.stderrText()));
            }
            try {
                Thread.sleep(10);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new IOException("interrupted while waiting for stdio startup", e);
            }
        }
    }

    private static void waitForReady(ManagedChannel channel, Duration timeout) throws IOException {
        long deadlineNanos = System.nanoTime() + timeout.toNanos();
        ConnectivityState state = channel.getState(true);
        while (state != ConnectivityState.READY) {
            if (state == ConnectivityState.SHUTDOWN) {
                throw new IOException("gRPC channel shut down before becoming ready");
            }
            long remainingNanos = deadlineNanos - System.nanoTime();
            if (remainingNanos <= 0) {
                throw new IOException("timed out waiting for gRPC readiness");
            }

            CountDownLatch latch = new CountDownLatch(1);
            channel.notifyWhenStateChanged(state, latch::countDown);
            try {
                if (!latch.await(remainingNanos, TimeUnit.NANOSECONDS)) {
                    throw new IOException("timed out waiting for gRPC readiness");
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw new IOException("interrupted while waiting for gRPC readiness", e);
            }
            state = channel.getState(false);
        }
    }

    private static void stopProcess(Process process) {
        if (process == null || !process.isAlive()) {
            return;
        }

        process.destroy();
        try {
            if (!process.waitFor(500, TimeUnit.MILLISECONDS)) {
                process.destroyForcibly();
                process.waitFor(1, TimeUnit.SECONDS);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            process.destroyForcibly();
        }
    }

    private static IdentityInfo toIdentityInfo(HolonIdentityJson payload) {
        IdentityInfo info = new IdentityInfo();
        if (payload == null) {
            return info;
        }
        info.givenName = safe(payload.givenName);
        info.familyName = safe(payload.familyName);
        info.motto = safe(payload.motto);
        info.aliases = copyList(payload.aliases);
        return info;
    }

    private static ScanEntry scanEntryFromRef(Path root, HolonRef ref) {
        HolonRef copy = copyRef(ref);
        Path filePath = filePath(copy);
        String relativePath = filePath != null ? relativePath(root, filePath) : safe(copy.url);
        String basename = filePath != null ? basename(filePath) : basenameFromUrl(copy.url);
        return new ScanEntry(relativePath, basename, copy);
    }

    private static HolonRef newRef(String url, HolonInfo info) {
        HolonRef ref = new HolonRef();
        ref.url = safe(url);
        ref.info = copyInfo(info);
        return ref;
    }

    private static HolonRef errorRef(String url, String error) {
        HolonRef ref = new HolonRef();
        ref.url = safe(url);
        ref.error = safe(error);
        return ref;
    }

    private static HolonRef copyRef(HolonRef ref) {
        if (ref == null) {
            return null;
        }
        HolonRef copy = new HolonRef();
        copy.url = safe(ref.url);
        copy.info = copyInfo(ref.info);
        copy.error = safe(ref.error);
        return copy;
    }

    private static HolonInfo copyInfo(HolonInfo info) {
        if (info == null) {
            return null;
        }
        HolonInfo copy = new HolonInfo();
        copy.slug = safe(info.slug);
        copy.uuid = safe(info.uuid);
        copy.identity = copyIdentity(info.identity);
        copy.lang = safe(info.lang);
        copy.runner = safe(info.runner);
        copy.status = safe(info.status);
        copy.kind = safe(info.kind);
        copy.transport = safe(info.transport);
        copy.entrypoint = safe(info.entrypoint);
        copy.architectures = copyList(info.architectures);
        copy.hasDist = info.hasDist;
        copy.hasSource = info.hasSource;
        return copy;
    }

    private static IdentityInfo copyIdentity(IdentityInfo identity) {
        IdentityInfo copy = new IdentityInfo();
        if (identity == null) {
            return copy;
        }
        copy.givenName = safe(identity.givenName);
        copy.familyName = safe(identity.familyName);
        copy.motto = safe(identity.motto);
        copy.aliases = copyList(identity.aliases);
        return copy;
    }

    private static List<HolonRef> applyLimit(List<HolonRef> refs, int limit) {
        if (limit <= 0 || refs.size() <= limit) {
            return refs;
        }
        return new ArrayList<>(refs.subList(0, limit));
    }

    private static Path filePath(HolonRef ref) {
        if (ref == null || ref.url == null || !ref.url.regionMatches(true, 0, "file://", 0, "file://".length())) {
            return null;
        }
        return pathFromFileURL(ref.url);
    }

    private static Path pathFromFileURL(String raw) {
        URI uri = URI.create(raw.trim());
        if (!"file".equalsIgnoreCase(uri.getScheme())) {
            throw new IllegalArgumentException("holon URL %s is not a local file target".formatted(quoted(raw)));
        }
        return absolute(Path.of(uri));
    }

    private static String fileURL(Path path) {
        return absolute(path).toUri().toString();
    }

    private static String slugForIdentity(IdentityInfo identity) {
        String given = safe(identity.givenName).trim();
        String family = safe(identity.familyName).trim().replaceFirst("\\?$", "");
        if (given.isEmpty() && family.isEmpty()) {
            return "";
        }
        return (given + "-" + family)
                .trim()
                .toLowerCase(Locale.ROOT)
                .replace(" ", "-")
                .replaceAll("^-+|-+$", "");
    }

    private static String basename(Path path) {
        String name = path.getFileName() == null ? "" : path.getFileName().toString();
        return name.endsWith(".holon") ? name.substring(0, name.length() - ".holon".length()) : name;
    }

    private static String basenameFromUrl(String url) {
        try {
            return basename(pathFromFileURL(url));
        } catch (Exception ignored) {
            return safe(url);
        }
    }

    private static String relativePath(Path root, Path dir) {
        try {
            Path relative = absolute(root).relativize(absolute(dir));
            String value = relative.toString().replace('\\', '/');
            return value.isEmpty() ? "." : value;
        } catch (Exception ignored) {
            return absolute(dir).toString().replace('\\', '/');
        }
    }

    private static int pathDepth(String relativePath) {
        String trimmed = safe(relativePath).trim().replaceAll("^/+|/+$", "");
        if (trimmed.isEmpty() || ".".equals(trimmed)) {
            return 0;
        }
        return trimmed.split("/").length;
    }

    private static String entryKey(HolonRef ref) {
        if (ref == null) {
            return "";
        }
        if (ref.info != null && !safe(ref.info.uuid).isBlank()) {
            return ref.info.uuid.trim();
        }
        Path path = filePath(ref);
        return path == null ? safe(ref.url) : path.toString();
    }

    private static String normalizeExpression(String expression) {
        return expression == null ? null : expression.trim();
    }

    private static String safeUuid(HolonRef ref) {
        return ref != null && ref.info != null ? safe(ref.info.uuid) : "";
    }

    private static String safe(String value) {
        return value == null ? "" : value;
    }

    private static List<String> copyList(List<String> values) {
        return values == null ? new ArrayList<>() : new ArrayList<>(values);
    }

    private static Path absolute(Path path) {
        return path.toAbsolutePath().normalize();
    }

    private static String currentDir() {
        return System.getProperty("user.dir", ".");
    }

    private static String getenvOrProperty(String name) {
        String property = System.getProperty(name);
        if (property != null && !property.isBlank()) {
            return property.trim();
        }
        String env = System.getenv(name);
        return env == null ? "" : env.trim();
    }

    private static String messageOf(Throwable error) {
        if (error == null) {
            return "";
        }
        String message = error.getMessage();
        return message == null || message.isBlank() ? error.toString() : message;
    }

    private static String quoted(String value) {
        return value == null ? "\"\"" : "\"%s\"".formatted(value);
    }

    private static String suffix(String details) {
        return details == null || details.isBlank() ? "" : ": " + details;
    }

    private static Duration effectiveTimeout(int timeout) {
        return Duration.ofMillis(timeout > 0 ? timeout : DEFAULT_PROBE_TIMEOUT_MS);
    }

    private record Layer(int flag, String name, LayerScanner scanner) {
        List<ScanEntry> scan(Path root, String expression, int limit, int timeout) throws Exception {
            return scanner.scan(root, expression, limit, timeout);
        }
    }

    @FunctionalInterface
    private interface LayerScanner {
        List<ScanEntry> scan(Path root, String expression, int limit, int timeout) throws Exception;
    }

    private record ScanEntry(String relativePath, String basename, HolonRef ref) {
    }

    private record PathLookup(HolonRef ref, boolean found) {
    }

    private static final class HolonPackageJson {
        String schema = "";
        String slug = "";
        String uuid = "";
        HolonIdentityJson identity = new HolonIdentityJson();
        String lang = "";
        String runner = "";
        String status = "";
        String kind = "";
        String transport = "";
        String entrypoint = "";
        @SerializedName("architectures")
        List<String> architectures = new ArrayList<>();
        @SerializedName("has_dist")
        boolean hasDist;
        @SerializedName("has_source")
        boolean hasSource;
    }

    private static final class HolonIdentityJson {
        @SerializedName("given_name")
        String givenName = "";
        @SerializedName("family_name")
        String familyName = "";
        String motto = "";
        List<String> aliases = new ArrayList<>();
    }

    private static final class StdioBridge implements Closeable {
        private final Process process;
        private final ServerSocket listener;
        private final StringBuilder stderr = new StringBuilder();
        private final Thread acceptThread;
        private volatile boolean closed;

        private StdioBridge(Process process) throws IOException {
            this.process = Objects.requireNonNull(process, "process");
            this.listener = new ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"));
            startDrainThread(process.getErrorStream(), stderr, "holons-discover-probe-stderr");
            this.acceptThread = new Thread(this::acceptLoop, "holons-discover-probe-accept");
            this.acceptThread.setDaemon(true);
            this.acceptThread.start();
        }

        private int port() {
            return listener.getLocalPort();
        }

        private String stderrText() {
            synchronized (stderr) {
                return stderr.toString().trim();
            }
        }

        @Override
        public void close() throws IOException {
            closed = true;
            listener.close();
            closeQuietly(process.getOutputStream());
            closeQuietly(process.getInputStream());
            closeQuietly(process.getErrorStream());
            try {
                acceptThread.join(200);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private void acceptLoop() {
            try (var socket = listener.accept();
                    var processStdout = process.getInputStream();
                    var processStdin = process.getOutputStream();
                    var inbound = socket.getInputStream();
                    var outbound = socket.getOutputStream()) {
                if (closed) {
                    return;
                }
                Thread up = startPump(inbound, processStdin, true, "holons-discover-probe-up");
                Thread down = startPump(processStdout, outbound, true, "holons-discover-probe-down");
                up.join();
                down.join();
            } catch (IOException ignored) {
                // Closed during shutdown.
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private static Thread startPump(InputStream input, OutputStream output, boolean closeOutput, String name) {
            Thread thread = new Thread(() -> {
                byte[] buffer = new byte[16 * 1024];
                try {
                    while (true) {
                        int read = input.read(buffer);
                        if (read < 0) {
                            break;
                        }
                        output.write(buffer, 0, read);
                        output.flush();
                    }
                } catch (IOException ignored) {
                    // Closed during shutdown.
                } finally {
                    if (closeOutput) {
                        closeQuietly(output);
                    }
                }
            }, name);
            thread.setDaemon(true);
            thread.start();
            return thread;
        }

        private static void startDrainThread(InputStream stream, StringBuilder capture, String name) {
            Thread thread = new Thread(() -> {
                try (BufferedReader reader = new BufferedReader(new InputStreamReader(stream, StandardCharsets.UTF_8))) {
                    String line;
                    while ((line = reader.readLine()) != null) {
                        synchronized (capture) {
                            capture.append(line).append('\n');
                        }
                    }
                } catch (IOException ignored) {
                    // Closed during shutdown.
                }
            }, name);
            thread.setDaemon(true);
            thread.start();
        }
    }

    private static void closeQuietly(Closeable closeable) {
        if (closeable == null) {
            return;
        }
        try {
            closeable.close();
        } catch (IOException ignored) {
            // Best effort.
        }
    }
}

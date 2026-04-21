package org.organicprogramming.holons;

import io.grpc.ConnectivityState;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import org.organicprogramming.holons.DiscoveryTypes.ConnectResult;
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
import java.net.Socket;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.nio.channels.Channels;
import java.nio.channels.SocketChannel;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.IdentityHashMap;
import java.util.Map;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;

/**
 * Uniform Phase 1 connect API plus legacy convenience wrappers.
 *
 * <p>The {@code expression} and {@code root} parameters on the uniform API may be {@code null}
 * even though the Java signature uses {@link String} references. Normal operational failures are
 * returned through {@link ConnectResult#error}; callers should not expect checked exceptions from
 * the uniform API.
 */
public final class Connect {
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

    public static final int NO_TIMEOUT = DiscoveryTypes.NO_TIMEOUT;

    public static final Duration DEFAULT_TIMEOUT = Duration.ofSeconds(5);

    private static final Map<ManagedChannel, StartedHandle> STARTED = new IdentityHashMap<>();

    private Connect() {
    }

    public record ConnectOptions(Duration timeout, String transport, boolean start, Path portFile) {
        public ConnectOptions {
            timeout = timeout != null && !timeout.isNegative() && !timeout.isZero() ? timeout : DEFAULT_TIMEOUT;
            transport = transport != null && !transport.isBlank() ? transport.trim().toLowerCase() : "stdio";
        }

        public ConnectOptions() {
            this(DEFAULT_TIMEOUT, "stdio", true, null);
        }
    }

    private record StartedHandle(Process process, Closeable closeable, boolean ephemeral) {
    }

    private record StartedProcess(String uri, Process process, Closeable closeable) {
    }

    private record DialedChannel(ManagedChannel channel, Closeable closeable) {
    }

    private record ConnectedRef(ManagedChannel channel, HolonRef origin) {
    }

    /**
     * Resolve, dial, or launch a holon using the uniform Phase 1 contract.
     *
     * <p>{@code expression} and {@code root} may be {@code null}. Failures are returned in the
     * result object instead of being thrown.
     */
    public static ConnectResult connect(int scope, String expression, String root, int specifiers, int timeout) {
        ConnectResult result = new ConnectResult();

        if (scope != LOCAL) {
            result.error = "scope %d not supported".formatted(scope);
            return result;
        }

        String target = expression == null ? "" : expression.trim();
        if (target.isEmpty()) {
            result.error = "expression is required";
            return result;
        }

        ResolveResult resolved = Discover.resolve(scope, target, root, specifiers, timeout);
        if (resolved.ref != null) {
            result.origin = copyRef(resolved.ref);
        }
        if (resolved.error != null && !resolved.error.isBlank()) {
            result.error = resolved.error;
            return result;
        }
        if (resolved.ref == null) {
            result.error = "holon %s not found".formatted(quoted(target));
            return result;
        }
        if (resolved.ref.error != null && !resolved.ref.error.isBlank()) {
            result.error = resolved.ref.error;
            return result;
        }

        try {
            ConnectedRef connected = connectRefInternal(
                    copyRef(resolved.ref),
                    target,
                    new ConnectOptions(effectiveTimeout(timeout), hintedTransportOptions(resolved.ref), true, null),
                    true,
                    true);
            result.channel = connected.channel();
            result.origin = connected.origin();
            return result;
        } catch (Exception e) {
            result.error = messageOf(e);
            return result;
        }
    }

    public static void disconnect(ConnectResult result) {
        if (result == null || !(result.channel instanceof ManagedChannel channel)) {
            return;
        }
        disconnect(channel);
    }

    public static ManagedChannel connect(String target) throws IOException {
        return connectInternal(target, new ConnectOptions(), true);
    }

    public static ManagedChannel connect(String target, ConnectOptions options) throws IOException {
        return connectInternal(target, options != null ? options : new ConnectOptions(), false);
    }

    public static void disconnect(ManagedChannel channel) {
        if (channel == null) {
            return;
        }

        StartedHandle handle;
        synchronized (STARTED) {
            handle = STARTED.remove(channel);
        }

        channel.shutdownNow();
        try {
            channel.awaitTermination(2, TimeUnit.SECONDS);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }

        if (handle != null) {
            closeQuietly(handle.closeable());
        }
        if (handle != null && handle.ephemeral()) {
            stopProcess(handle.process());
        }
    }

    private static ManagedChannel connectInternal(String target, ConnectOptions options, boolean defaultEphemeral)
            throws IOException {
        String trimmed = target == null ? "" : target.trim();
        if (trimmed.isEmpty()) {
            throw new IllegalArgumentException("target is required");
        }

        if (isDirectTarget(trimmed)) {
            DialedChannel direct = dialReady(normalizeDialTarget(trimmed), options.timeout());
            rememberChannel(direct, false);
            return direct.channel();
        }

        if (!"stdio".equals(options.transport()) && !"tcp".equals(options.transport()) && !"unix".equals(options.transport())) {
            throw new IllegalArgumentException("unsupported transport \"" + options.transport() + "\"");
        }

        ResolveResult resolved = Discover.resolve(LOCAL, trimmed, null, ALL, (int) options.timeout().toMillis());
        if (resolved.error != null && !resolved.error.isBlank()) {
            throw new IllegalArgumentException(resolved.error);
        }
        if (resolved.ref == null) {
            throw new IllegalArgumentException("holon %s not found".formatted(quoted(trimmed)));
        }

        return connectRefInternal(copyRef(resolved.ref), trimmed, options, defaultEphemeral, false).channel();
    }

    private static ConnectedRef connectRefInternal(
            HolonRef ref,
            String targetName,
            ConnectOptions options,
            boolean defaultEphemeral,
            boolean preferTransportHint) throws IOException {
        if (ref == null) {
            throw new IllegalArgumentException("resolved holon ref is required");
        }

        String originalUrl = ref.url == null ? "" : ref.url.trim();
        if (isReachableTarget(originalUrl)) {
            DialedChannel direct = dialReady(normalizeDialTarget(originalUrl), options.timeout());
            rememberChannel(direct, false);
            return new ConnectedRef(direct.channel(), copyRef(ref));
        }

        String transport = preferTransportHint ? hintedTransport(ref) : options.transport();
        if (!"stdio".equals(transport) && !"tcp".equals(transport) && !"unix".equals(transport)) {
            throw new IllegalArgumentException("unsupported transport \"" + transport + "\"");
        }
        boolean ephemeral = defaultEphemeral || "stdio".equals(transport);

        String slug = safeSlug(ref, targetName);
        Path portFile = options.portFile() != null ? options.portFile() : defaultPortFilePath(slug);
        if (!ephemeral && ("tcp".equals(transport) || "unix".equals(transport))) {
            String reusable = usablePortFile(portFile, options.timeout());
            if (reusable != null) {
                DialedChannel direct = dialReady(normalizeDialTarget(reusable), options.timeout());
                rememberChannel(direct, false);
                HolonRef origin = copyRef(ref);
                origin.url = reusable;
                return new ConnectedRef(direct.channel(), origin);
            }
        }
        if (!options.start()) {
            throw new IllegalStateException("holon %s is not running".formatted(quoted(targetName)));
        }

        String binaryPath = resolveBinaryPath(ref);
        StartedProcess started = "stdio".equals(transport)
                ? startStdioHolon(binaryPath, options.timeout())
                : "unix".equals(transport)
                ? startUnixHolon(binaryPath, slug, portFile, options.timeout())
                : startAdvertisedHolon(binaryPath, "tcp://127.0.0.1:0", options.timeout());

        DialedChannel dialed;
        try {
            dialed = dialReady(normalizeDialTarget(started.uri()), options.timeout());
        } catch (IOException | RuntimeException e) {
            closeQuietly(started.closeable());
            stopProcess(started.process());
            throw e;
        }

        ManagedChannel channel = dialed.channel();
        if (!ephemeral && ("tcp".equals(transport) || "unix".equals(transport))) {
            try {
                writePortFile(portFile, started.uri());
            } catch (IOException e) {
                disconnect(channel);
                stopProcess(started.process());
                throw e;
            }
        }

        synchronized (STARTED) {
            STARTED.put(channel, new StartedHandle(
                    started.process(),
                    combineCloseables(started.closeable(), dialed.closeable()),
                    ephemeral));
        }

        HolonRef origin = copyRef(ref);
        if (!"stdio".equals(transport)) {
            origin.url = started.uri();
        }
        return new ConnectedRef(channel, origin);
    }

    private static String resolveBinaryPath(HolonRef ref) {
        if (ref.info == null) {
            throw new IllegalArgumentException("holon metadata unavailable");
        }

        Path targetPath = filePath(ref);
        String entrypoint = ref.info.entrypoint == null ? "" : ref.info.entrypoint.trim();
        String slug = safeSlug(ref, "holon");

        if (targetPath != null && Files.isRegularFile(targetPath)) {
            return targetPath.toString();
        }

        if (targetPath != null && Files.isDirectory(targetPath) && basename(targetPath).endsWith(".holon")) {
            Path packageBinary = resolvePackageBinary(targetPath, entrypoint);
            if (Files.isRegularFile(packageBinary)) {
                return packageBinary.toString();
            }
        }

        if (targetPath != null && !entrypoint.isBlank()) {
            Path configured = Path.of(entrypoint);
            if (configured.isAbsolute() && Files.isRegularFile(configured)) {
                return configured.toString();
            }

            Path candidate = targetPath.resolve(".op").resolve("build").resolve("bin").resolve(configured.getFileName());
            if (Files.isRegularFile(candidate)) {
                return candidate.toString();
            }
        }

        if (targetPath != null) {
            Path slugCandidate = targetPath.resolve(".op").resolve("build").resolve("bin").resolve(slug);
            if (Files.isRegularFile(slugCandidate)) {
                return slugCandidate.toString();
            }
        }

        String searchName = !entrypoint.isBlank() ? Path.of(entrypoint).getFileName().toString() : slug;
        String pathEnv = System.getenv("PATH");
        if (pathEnv != null) {
            for (String dir : pathEnv.split(java.io.File.pathSeparator)) {
                Path resolved = Path.of(dir).resolve(searchName);
                if (Files.isRegularFile(resolved) && Files.isExecutable(resolved)) {
                    return resolved.toString();
                }
            }
        }

        throw new IllegalArgumentException("built binary not found for holon %s".formatted(quoted(slug)));
    }

    private static Path resolvePackageBinary(Path packageDir, String entrypoint) {
        Path archDir = packageDir.resolve("bin").resolve(currentPlatformTag());
        if (!Files.isDirectory(archDir)) {
            return archDir.resolve(entrypoint.isBlank() ? "missing" : entrypoint);
        }

        if (!entrypoint.isBlank()) {
            Path named = archDir.resolve(Path.of(entrypoint).getFileName().toString());
            if (Files.isRegularFile(named)) {
                return named;
            }
        }

        try (var stream = Files.list(archDir)) {
            return stream
                    .filter(Files::isRegularFile)
                    .sorted()
                    .findFirst()
                    .orElse(archDir.resolve("missing"));
        } catch (IOException e) {
            return archDir.resolve("missing");
        }
    }

    private static String hintedTransportOptions(HolonRef ref) {
        return hintedTransport(ref);
    }

    private static String hintedTransport(HolonRef ref) {
        String transport = ref != null && ref.info != null && ref.info.transport != null
                ? ref.info.transport.trim().toLowerCase()
                : "";
        return switch (transport) {
            case "tcp" -> "tcp";
            case "unix" -> "unix";
            default -> "stdio";
        };
    }

    private static boolean isReachableTarget(String target) {
        if (target == null || target.isBlank()) {
            return false;
        }
        return target.startsWith("tcp://") || target.startsWith("unix://");
    }

    private static DialedChannel dialReady(String target, Duration timeout) throws IOException {
        ManagedChannel channel;
        Closeable closeable = null;
        if (target.startsWith("unix://")) {
            UnixBridge bridge = new UnixBridge(target);
            HostPort hostPort = parseHostPort(normalizeDialTarget(bridge.uri()));
            channel = ManagedChannelBuilder.forAddress(hostPort.host(), hostPort.port()).usePlaintext().build();
            closeable = bridge;
        } else {
            HostPort hostPort = parseHostPort(target);
            channel = ManagedChannelBuilder.forAddress(hostPort.host(), hostPort.port()).usePlaintext().build();
        }

        try {
            waitForReady(channel, timeout);
            return new DialedChannel(channel, closeable);
        } catch (IOException | RuntimeException e) {
            channel.shutdownNow();
            closeQuietly(closeable);
            throw e;
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

    private static String usablePortFile(Path portFile, Duration timeout) {
        try {
            String raw = Files.readString(portFile).trim();
            if (raw.isEmpty()) {
                Files.deleteIfExists(portFile);
                return null;
            }

            DialedChannel probe = dialReady(normalizeDialTarget(raw), min(timeout, Duration.ofSeconds(1)));
            probe.channel().shutdownNow();
            closeQuietly(probe.closeable());
            return raw;
        } catch (Exception ignored) {
            try {
                Files.deleteIfExists(portFile);
            } catch (IOException ignoredDelete) {
                // Best effort.
            }
            return null;
        }
    }

    private static StartedProcess startAdvertisedHolon(String binaryPath, String listenURI, Duration timeout) throws IOException {
        Process process = new ProcessBuilder(binaryPath, "serve", "--listen", listenURI).start();
        BlockingQueue<String> lines = new LinkedBlockingQueue<>();
        StringBuilder stderr = new StringBuilder();

        startReader(process.getInputStream(), lines, null);
        startReader(process.getErrorStream(), lines, stderr);

        long deadlineNanos = System.nanoTime() + timeout.toNanos();
        while (System.nanoTime() < deadlineNanos) {
            if (!process.isAlive()) {
                throw new IOException("holon exited before advertising an address" + suffix(stderr));
            }

            try {
                String line = lines.poll(50, TimeUnit.MILLISECONDS);
                if (line == null) {
                    continue;
                }
                String uri = firstURI(line);
                if (!uri.isBlank()) {
                    return new StartedProcess(uri, process, null);
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                stopProcess(process);
                throw new IOException("interrupted while waiting for holon startup", e);
            }
        }

        stopProcess(process);
        throw new IOException("timed out waiting for holon startup" + suffix(stderr));
    }

    private static StartedProcess startUnixHolon(String binaryPath, String slug, Path portFile, Duration timeout)
            throws IOException {
        String uri = defaultUnixSocketURI(slug, portFile);
        String socketPath = uri.substring("unix://".length());

        Process process = new ProcessBuilder(binaryPath, "serve", "--listen", uri).start();
        StringBuilder stderr = new StringBuilder();
        startDrainThread(process.getErrorStream(), stderr, "holons-unix-connect-stderr");

        long deadlineNanos = System.nanoTime() + timeout.toNanos();
        while (System.nanoTime() < deadlineNanos) {
            if (Files.exists(Path.of(socketPath))) {
                return new StartedProcess(uri, process, null);
            }
            if (!process.isAlive()) {
                throw new IOException("holon exited before binding unix socket" + suffix(stderr));
            }
            try {
                Thread.sleep(20);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                stopProcess(process);
                throw new IOException("interrupted while waiting for unix holon startup", e);
            }
        }

        stopProcess(process);
        throw new IOException("timed out waiting for unix holon startup" + suffix(stderr));
    }

    private static StartedProcess startStdioHolon(String binaryPath, Duration timeout) throws IOException {
        Process process = new ProcessBuilder(binaryPath, "serve", "--listen", "stdio://").start();
        StdioBridge bridge = new StdioBridge(process);
        long startupWindowMs = Math.max(1L, Math.min(timeout.toMillis(), 200L));
        long deadlineNanos = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(startupWindowMs);

        while (System.nanoTime() < deadlineNanos) {
            if (!process.isAlive()) {
                String stderr = bridge.stderrText();
                bridge.close();
                throw new IOException("holon exited before stdio startup" + (stderr.isBlank() ? "" : ": " + stderr));
            }
            try {
                Thread.sleep(10);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                bridge.close();
                stopProcess(process);
                throw new IOException("interrupted while waiting for stdio startup", e);
            }
        }

        return new StartedProcess(bridge.uri(), process, bridge);
    }

    private static void startReader(InputStream stream, BlockingQueue<String> lines, StringBuilder capture) {
        Thread thread = new Thread(() -> {
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(stream, StandardCharsets.UTF_8))) {
                String line;
                while ((line = reader.readLine()) != null) {
                    if (capture != null) {
                        capture.append(line).append('\n');
                    }
                    lines.offer(line);
                }
            } catch (IOException ignored) {
                // Startup timeout and shutdown paths tolerate closed pipes.
            }
        });
        thread.setDaemon(true);
        thread.start();
    }

    private static Path defaultPortFilePath(String slug) {
        return Path.of(System.getProperty("user.dir", ".")).resolve(".op").resolve("run").resolve(slug + ".port");
    }

    private static String defaultUnixSocketURI(String slug, Path portFile) {
        String label = socketLabel(slug);
        long hash = fnv1a64(portFile.toString()) & 0xFFFFFFFFFFFFL;
        return "unix:///tmp/holons-" + label + "-" + String.format("%012x", hash) + ".sock";
    }

    private static String socketLabel(String slug) {
        StringBuilder label = new StringBuilder();
        boolean lastDash = false;

        for (char ch : slug.trim().toLowerCase().toCharArray()) {
            if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
                label.append(ch);
                lastDash = false;
            } else if ((ch == '-' || ch == '_') && label.length() > 0 && !lastDash) {
                label.append('-');
                lastDash = true;
            }

            if (label.length() >= 24) {
                break;
            }
        }

        while (label.length() > 0 && label.charAt(0) == '-') {
            label.deleteCharAt(0);
        }
        while (label.length() > 0 && label.charAt(label.length() - 1) == '-') {
            label.deleteCharAt(label.length() - 1);
        }
        return label.isEmpty() ? "socket" : label.toString();
    }

    private static long fnv1a64(String text) {
        long hash = 0xcbf29ce484222325L;
        byte[] bytes = text.getBytes(StandardCharsets.UTF_8);
        for (byte value : bytes) {
            hash ^= (value & 0xffL);
            hash *= 0x100000001b3L;
        }
        return hash;
    }

    private static void writePortFile(Path portFile, String uri) throws IOException {
        Files.createDirectories(portFile.getParent());
        Files.writeString(portFile, uri.trim() + System.lineSeparator());
    }

    private static void stopProcess(Process process) {
        if (process == null || !process.isAlive()) {
            return;
        }

        process.destroy();
        try {
            if (!process.waitFor(2, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                process.waitFor(2, TimeUnit.SECONDS);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            process.destroyForcibly();
        }
    }

    private static boolean isDirectTarget(String target) {
        return target.contains("://") || target.contains(":");
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

    private static String normalizeDialTarget(String target) {
        if (!target.contains("://")) {
            return target;
        }

        Transport.ParsedURI parsed = Transport.parseURI(target);
        if ("tcp".equals(parsed.scheme())) {
            String host = parsed.host() == null || parsed.host().isBlank() || "0.0.0.0".equals(parsed.host())
                    ? "127.0.0.1"
                    : parsed.host();
            return host + ":" + parsed.port();
        }
        if ("unix".equals(parsed.scheme())) {
            return "unix://" + parsed.path();
        }
        return target;
    }

    private static String firstURI(String line) {
        for (String field : line.split("\\s+")) {
            String trimmed = field.trim().replaceAll("^[\"'()\\[\\]{}.,]+|[\"'()\\[\\]{}.,]+$", "");
            if (trimmed.startsWith("tcp://")
                    || trimmed.startsWith("unix://")
                    || trimmed.startsWith("stdio://")
                    || trimmed.startsWith("ws://")
                    || trimmed.startsWith("wss://")) {
                return trimmed;
            }
        }
        return "";
    }

    private static Duration min(Duration left, Duration right) {
        return left.compareTo(right) <= 0 ? left : right;
    }

    private record HostPort(String host, int port) {
    }

    private static HostPort parseHostPort(String target) {
        int idx = target.lastIndexOf(':');
        if (idx <= 0 || idx == target.length() - 1) {
            throw new IllegalArgumentException("invalid host:port target \"" + target + "\"");
        }
        return new HostPort(target.substring(0, idx), Integer.parseInt(target.substring(idx + 1)));
    }

    private static void rememberChannel(DialedChannel dialed, boolean ephemeral) {
        if (dialed.closeable() == null) {
            return;
        }
        synchronized (STARTED) {
            STARTED.put(dialed.channel(), new StartedHandle(null, dialed.closeable(), ephemeral));
        }
    }

    private static Closeable combineCloseables(Closeable first, Closeable second) {
        if (first == null) {
            return second;
        }
        if (second == null) {
            return first;
        }
        return () -> {
            IOException firstError = null;
            try {
                first.close();
            } catch (IOException e) {
                firstError = e;
            }
            try {
                second.close();
            } catch (IOException e) {
                if (firstError != null) {
                    e.addSuppressed(firstError);
                }
                throw e;
            }
            if (firstError != null) {
                throw firstError;
            }
        };
    }

    private static void startDrainThread(InputStream stream, StringBuilder capture, String name) {
        Thread thread = new Thread(() -> {
            byte[] buffer = new byte[4096];
            try {
                while (true) {
                    int read = stream.read(buffer);
                    if (read < 0) {
                        break;
                    }
                    synchronized (capture) {
                        capture.append(new String(buffer, 0, read, StandardCharsets.UTF_8));
                    }
                }
            } catch (IOException ignored) {
                // Closed during shutdown.
            }
        }, name);
        thread.setDaemon(true);
        thread.start();
    }

    private static String safeSlug(HolonRef ref, String fallback) {
        if (ref != null && ref.info != null && ref.info.slug != null && !ref.info.slug.isBlank()) {
            return ref.info.slug.trim();
        }
        return fallback == null || fallback.isBlank() ? "holon" : fallback.trim();
    }

    private static Path filePath(HolonRef ref) {
        if (ref == null || ref.url == null || !ref.url.regionMatches(true, 0, "file://", 0, "file://".length())) {
            return null;
        }
        return Path.of(URI.create(ref.url)).toAbsolutePath().normalize();
    }

    private static String basename(Path path) {
        return path.getFileName() == null ? "" : path.getFileName().toString();
    }

    private static Duration effectiveTimeout(int timeout) {
        return timeout > 0 ? Duration.ofMillis(timeout) : DEFAULT_TIMEOUT;
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

    private static String suffix(StringBuilder builder) {
        String details = builder == null ? "" : builder.toString().trim();
        return details.isBlank() ? "" : ": " + details;
    }

    private static String currentPlatformTag() {
        return normalizedOs() + "_" + normalizedArch();
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

    private static HolonRef copyRef(HolonRef ref) {
        if (ref == null) {
            return null;
        }
        HolonRef copy = new HolonRef();
        copy.url = ref.url == null ? "" : ref.url;
        copy.error = ref.error == null ? "" : ref.error;
        copy.info = copyInfo(ref.info);
        return copy;
    }

    private static HolonInfo copyInfo(HolonInfo info) {
        if (info == null) {
            return null;
        }
        HolonInfo copy = new HolonInfo();
        copy.slug = info.slug == null ? "" : info.slug;
        copy.uuid = info.uuid == null ? "" : info.uuid;
        copy.identity = copyIdentity(info.identity);
        copy.lang = info.lang == null ? "" : info.lang;
        copy.runner = info.runner == null ? "" : info.runner;
        copy.status = info.status == null ? "" : info.status;
        copy.kind = info.kind == null ? "" : info.kind;
        copy.transport = info.transport == null ? "" : info.transport;
        copy.entrypoint = info.entrypoint == null ? "" : info.entrypoint;
        copy.architectures = info.architectures == null ? new java.util.ArrayList<>() : new java.util.ArrayList<>(info.architectures);
        copy.hasDist = info.hasDist;
        copy.hasSource = info.hasSource;
        return copy;
    }

    private static IdentityInfo copyIdentity(IdentityInfo identity) {
        IdentityInfo copy = new IdentityInfo();
        if (identity == null) {
            return copy;
        }
        copy.givenName = identity.givenName == null ? "" : identity.givenName;
        copy.familyName = identity.familyName == null ? "" : identity.familyName;
        copy.motto = identity.motto == null ? "" : identity.motto;
        copy.aliases = identity.aliases == null ? new java.util.ArrayList<>() : new java.util.ArrayList<>(identity.aliases);
        return copy;
    }

    private static final class StdioBridge implements Closeable {
        private final Process process;
        private final ServerSocket listener;
        private final StringBuilder stderr = new StringBuilder();
        private final Thread acceptThread;
        private volatile Socket socket;
        private volatile boolean closed;

        private StdioBridge(Process process) throws IOException {
            this.process = process;
            this.listener = new ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"));
            startDrainThread(process.getErrorStream(), stderr, "holons-stdio-bridge-stderr");
            this.acceptThread = new Thread(this::acceptLoop, "holons-stdio-bridge-accept");
            this.acceptThread.setDaemon(true);
            this.acceptThread.start();
        }

        private String uri() {
            return "tcp://127.0.0.1:" + listener.getLocalPort();
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

            Socket active = socket;
            socket = null;
            if (active != null) {
                active.close();
            }

            closeStream(process.getOutputStream());
            closeStream(process.getInputStream());
            closeStream(process.getErrorStream());

            try {
                acceptThread.join(200);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private void acceptLoop() {
            try {
                Socket accepted = listener.accept();
                if (closed) {
                    accepted.close();
                    return;
                }
                socket = accepted;

                Thread upstream = startPump(
                        accepted.getInputStream(),
                        process.getOutputStream(),
                        true,
                        "holons-stdio-bridge-up");
                Thread downstream = startPump(
                        process.getInputStream(),
                        accepted.getOutputStream(),
                        true,
                        "holons-stdio-bridge-down");

                upstream.join();
                downstream.join();
            } catch (IOException ignored) {
                // Closed during shutdown.
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                Socket active = socket;
                socket = null;
                if (active != null) {
                    closeStream(active);
                }
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
                        closeStream(output);
                    }
                }
            }, name);
            thread.setDaemon(true);
            thread.start();
            return thread;
        }

        private static void closeStream(Closeable closeable) {
            try {
                closeable.close();
            } catch (IOException ignored) {
                // Best effort.
            }
        }
    }

    private static final class UnixBridge implements Closeable {
        private final String target;
        private final ServerSocket listener;
        private final Thread acceptThread;
        private volatile boolean closed;
        private volatile Socket socket;
        private volatile SocketChannel upstream;

        private UnixBridge(String target) throws IOException {
            this.target = target;
            this.listener = new ServerSocket(0, 1, InetAddress.getByName("127.0.0.1"));
            this.acceptThread = new Thread(this::acceptLoop, "holons-unix-bridge-accept");
            this.acceptThread.setDaemon(true);
            this.acceptThread.start();
        }

        private String uri() {
            return "tcp://127.0.0.1:" + listener.getLocalPort();
        }

        @Override
        public void close() throws IOException {
            closed = true;
            listener.close();
            Socket active = socket;
            socket = null;
            if (active != null) {
                closeQuietly(active);
            }
            SocketChannel activeUpstream = upstream;
            upstream = null;
            if (activeUpstream != null) {
                activeUpstream.close();
            }
            try {
                acceptThread.join(200);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        private void acceptLoop() {
            try {
                Socket accepted = listener.accept();
                if (closed) {
                    accepted.close();
                    return;
                }

                SocketChannel unixChannel = Transport.dialUnix(target);
                socket = accepted;
                upstream = unixChannel;

                Thread upstreamPump = startBridgePump(
                        accepted.getInputStream(),
                        Channels.newOutputStream(unixChannel),
                        true,
                        "holons-unix-bridge-up");
                Thread downstreamPump = startBridgePump(
                        Channels.newInputStream(unixChannel),
                        accepted.getOutputStream(),
                        true,
                        "holons-unix-bridge-down");

                upstreamPump.join();
                downstreamPump.join();
            } catch (IOException ignored) {
                // Closed during shutdown.
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                Socket active = socket;
                socket = null;
                if (active != null) {
                    closeQuietly(active);
                }
                SocketChannel activeUpstream = upstream;
                upstream = null;
                if (activeUpstream != null) {
                    try {
                        activeUpstream.close();
                    } catch (IOException ignored) {
                        // Best effort.
                    }
                }
            }
        }

        private Thread startBridgePump(InputStream input, OutputStream output, boolean closeOutput, String name) {
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
    }
}

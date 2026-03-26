package org.organicprogramming.holons;

import io.grpc.BindableService;
import io.grpc.Server;
import io.grpc.ServerServiceDefinition;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.protobuf.services.ProtoReflectionService;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.channels.Channels;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Consumer;

/** Standard gRPC server runner utilities. */
public final class Serve {

    private Serve() {
    }

    public static final class Options {
        private boolean describe = true;
        private boolean reflect = false;
        private Consumer<String> logger = System.err::println;
        private Consumer<String> onListen;
        private long shutdownGracePeriodSeconds = 10;
        private Path protoDir;

        public boolean describe() {
            return describe;
        }

        public Options withDescribe(boolean describe) {
            this.describe = describe;
            return this;
        }

        public boolean reflect() {
            return reflect;
        }

        public Options withReflect(boolean reflect) {
            this.reflect = reflect;
            return this;
        }

        public Consumer<String> logger() {
            return logger;
        }

        public Options withLogger(Consumer<String> logger) {
            this.logger = logger != null ? logger : System.err::println;
            return this;
        }

        public Consumer<String> onListen() {
            return onListen;
        }

        public Options withOnListen(Consumer<String> onListen) {
            this.onListen = onListen;
            return this;
        }

        public long shutdownGracePeriodSeconds() {
            return shutdownGracePeriodSeconds;
        }

        public Options withShutdownGracePeriodSeconds(long shutdownGracePeriodSeconds) {
            this.shutdownGracePeriodSeconds = shutdownGracePeriodSeconds > 0 ? shutdownGracePeriodSeconds : 10;
            return this;
        }

        public Path protoDir() {
            return protoDir;
        }

        public Options withProtoDir(Path protoDir) {
            this.protoDir = protoDir;
            return this;
        }
    }

    public static final class RunningServer {
        private final Server server;
        private final String publicUri;
        private final Consumer<String> logger;
        private final Runnable auxiliaryStop;
        private final AtomicBoolean stopped = new AtomicBoolean(false);

        private RunningServer(
                Server server,
                String publicUri,
                Consumer<String> logger,
                Runnable auxiliaryStop) {
            this.server = server;
            this.publicUri = publicUri;
            this.logger = logger;
            this.auxiliaryStop = auxiliaryStop;
        }

        public String publicUri() {
            return publicUri;
        }

        public void await() throws InterruptedException {
            server.awaitTermination();
        }

        public void stop() throws InterruptedException {
            stop(10);
        }

        public void stop(long gracePeriodSeconds) throws InterruptedException {
            if (!stopped.compareAndSet(false, true)) {
                return;
            }

            if (auxiliaryStop != null) {
                auxiliaryStop.run();
            }

            server.shutdown();
            if (!server.awaitTermination(gracePeriodSeconds, TimeUnit.SECONDS)) {
                logger.accept("graceful stop timed out after " + gracePeriodSeconds + "s; forcing hard stop");
                server.shutdownNow();
                server.awaitTermination(gracePeriodSeconds, TimeUnit.SECONDS);
            }
        }
    }

    private record BoundServer(Server server, String publicUri) {
    }

    public record ParsedFlags(String listenUri, boolean reflect) {
    }

    /**
     * Parse --listen or --port from command-line args.
     * Returns a transport URI; falls back to {@link Transport#DEFAULT_URI}.
     */
    public static String parseFlags(String[] args) {
        return parseOptions(args).listenUri();
    }

    /** Parse --listen, --port, and --reflect from command-line args. */
    public static ParsedFlags parseOptions(String[] args) {
        String listenUri = Transport.DEFAULT_URI;
        boolean reflect = false;
        for (int i = 0; i < args.length; i++) {
            if ("--listen".equals(args[i]) && i + 1 < args.length) {
                listenUri = args[i + 1];
            }
            if ("--port".equals(args[i]) && i + 1 < args.length) {
                listenUri = "tcp://:" + args[i + 1];
            }
            if ("--reflect".equals(args[i])) {
                reflect = true;
            }
        }
        return new ParsedFlags(listenUri, reflect);
    }

    public static void run(String listenUri, BindableService... services) throws IOException, InterruptedException {
        List<BindableService> serviceList = new ArrayList<>();
        for (BindableService service : services) {
            serviceList.add(service);
        }
        runWithOptions(listenUri, serviceList, new Options());
    }

    public static void runWithOptions(
            String listenUri,
            Iterable<? extends BindableService> services,
            Options options) throws IOException, InterruptedException {
        Options resolvedOptions = options != null ? options : new Options();
        RunningServer running = startWithOptions(listenUri, services, resolvedOptions);
        Thread shutdownHook = new Thread(() -> {
            resolvedOptions.logger().accept("shutting down gRPC server");
            try {
                running.stop(resolvedOptions.shutdownGracePeriodSeconds());
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }, "holons-serve-shutdown");

        Runtime.getRuntime().addShutdownHook(shutdownHook);
        try {
            running.await();
        } finally {
            try {
                Runtime.getRuntime().removeShutdownHook(shutdownHook);
            } catch (IllegalStateException ignored) {
                // JVM is already shutting down.
            }
        }
    }

    public static RunningServer startWithOptions(
            String listenUri,
            Iterable<? extends BindableService> services,
            Options options) throws IOException {
        Options resolvedOptions = options != null ? options : new Options();
        Transport.ParsedURI parsed = Transport.parseURI(
                listenUri != null && !listenUri.isBlank() ? listenUri : Transport.DEFAULT_URI);

        List<BindableService> bindableServices = new ArrayList<>();
        for (BindableService service : services) {
            bindableServices.add(service);
        }

        List<ServerServiceDefinition> extraDefinitions = new ArrayList<>();
        boolean describeEnabled;
        try {
            describeEnabled = maybeAddDescribe(extraDefinitions, resolvedOptions);
        } catch (Exception error) {
            resolvedOptions.logger().accept("HolonMeta registration failed: " + error.getMessage());
            if (error instanceof IOException ioException) {
                throw ioException;
            }
            throw new IOException("register HolonMeta: " + error.getMessage(), error);
        }
        boolean reflectionEnabled = maybeAddReflection(extraDefinitions, resolvedOptions);

        return switch (parsed.scheme()) {
            case "tcp" -> {
                String host = parsed.host() != null ? parsed.host() : "0.0.0.0";
                int port = parsed.port() != null ? parsed.port() : 9090;
                BoundServer bound = bindTcpServer(host, port, bindableServices, extraDefinitions);
                announce(bound.publicUri(), describeEnabled, reflectionEnabled, resolvedOptions);
                yield new RunningServer(bound.server(), bound.publicUri(), resolvedOptions.logger(), null);
            }
            case "stdio" -> {
                BoundServer bound = bindTcpServer("127.0.0.1", 0, bindableServices, extraDefinitions);
                String[] target = parseTarget(bound.publicUri());
                RunningServer[] runningRef = new RunningServer[1];
                StdioServerBridge bridge = new StdioServerBridge(target[0], Integer.parseInt(target[1]), () -> {
                    RunningServer running = runningRef[0];
                    if (running != null) {
                        try {
                            running.stop(resolvedOptions.shutdownGracePeriodSeconds());
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        }
                    }
                });
                RunningServer running = new RunningServer(
                        bound.server(),
                        "stdio://",
                        resolvedOptions.logger(),
                        bridge::close);
                runningRef[0] = running;
                bridge.start();
                announce("stdio://", describeEnabled, reflectionEnabled, resolvedOptions);
                yield running;
            }
            case "unix" -> {
                BoundServer bound = bindTcpServer("127.0.0.1", 0, bindableServices, extraDefinitions);
                String[] target = parseTarget(bound.publicUri());
                String publicUri = "unix://" + Objects.requireNonNull(parsed.path());
                UnixServerBridge bridge = new UnixServerBridge(Objects.requireNonNull(parsed.path()), target[0], Integer.parseInt(target[1]));
                RunningServer running = new RunningServer(
                        bound.server(),
                        publicUri,
                        resolvedOptions.logger(),
                        bridge::close);
                bridge.start();
                announce(publicUri, describeEnabled, reflectionEnabled, resolvedOptions);
                yield running;
            }
            default -> throw new IllegalArgumentException(
                    "Serve.run(...) currently supports tcp://, unix:// and stdio:// only: " + listenUri);
        };
    }

    private static BoundServer bindTcpServer(
            String host,
            int port,
            Iterable<? extends BindableService> services,
            Iterable<ServerServiceDefinition> definitions) throws IOException {
        NettyServerBuilder builder = NettyServerBuilder.forAddress(new InetSocketAddress(host, port));
        for (BindableService service : services) {
            builder.addService(service);
        }
        for (ServerServiceDefinition definition : definitions) {
            builder.addService(definition);
        }
        Server server = builder.build().start();
        String publicUri = "tcp://" + advertisedHost(host) + ":" + server.getPort();
        return new BoundServer(server, publicUri);
    }

    private static void announce(String publicUri, boolean describeEnabled, boolean reflectionEnabled, Options options) {
        if (options.onListen() != null) {
            options.onListen().accept(publicUri);
        }
        options.logger().accept(
                "gRPC server listening on " + publicUri + " ("
                        + (describeEnabled ? "Describe ON" : "Describe OFF")
                        + ", "
                        + (reflectionEnabled ? "reflection ON" : "reflection OFF")
                        + ")");
    }

    private static boolean maybeAddDescribe(List<ServerServiceDefinition> definitions, Options options) {
        if (!options.describe()) {
            return false;
        }
        definitions.add(Describe.service());
        return true;
    }

    private static boolean maybeAddReflection(List<ServerServiceDefinition> definitions, Options options) {
        if (!options.reflect()) {
            return false;
        }
        definitions.add(ProtoReflectionService.newInstance().bindService());
        return true;
    }

    private static Path resolveProtoDir(Options options) {
        if (options.protoDir() != null) {
            return options.protoDir().toAbsolutePath().normalize();
        }
        return Path.of("protos").toAbsolutePath().normalize();
    }

    private static String advertisedHost(String host) {
        return switch (host) {
            case "", "0.0.0.0" -> "127.0.0.1";
            case "::" -> "::1";
            default -> host;
        };
    }

    private static String[] parseTarget(String uri) {
        if (!uri.startsWith("tcp://")) {
            throw new IllegalArgumentException("unexpected uri: " + uri);
        }
        String target = uri.substring("tcp://".length());
        int idx = target.lastIndexOf(':');
        if (idx <= 0 || idx >= target.length() - 1) {
            throw new IllegalArgumentException("unexpected uri: " + uri);
        }
        return new String[] { target.substring(0, idx), target.substring(idx + 1) };
    }

    private static final class StdioServerBridge implements AutoCloseable {
        private final Socket socket;
        private final Runnable onDisconnect;
        private final AtomicBoolean closed = new AtomicBoolean(false);
        private final AtomicInteger completions = new AtomicInteger(2);

        private StdioServerBridge(String host, int port, Runnable onDisconnect) throws IOException {
            this.socket = new Socket(host, port);
            this.onDisconnect = Objects.requireNonNull(onDisconnect, "onDisconnect");
        }

        private void start() {
            Thread upstream = new Thread(
                    () -> {
                        pump(System.in, socketOutput(), true);
                        markComplete();
                    },
                    "holons-serve-stdio-up");
            upstream.setDaemon(true);
            upstream.start();

            Thread downstream = new Thread(
                    () -> {
                        pump(socketInput(), System.out, false);
                        markComplete();
                    },
                    "holons-serve-stdio-down");
            downstream.setDaemon(true);
            downstream.start();
        }

        @Override
        public void close() {
            if (!closed.compareAndSet(false, true)) {
                return;
            }
            try {
                socket.close();
            } catch (IOException ignored) {
                // Best-effort shutdown.
            }
        }

        private InputStream socketInput() {
            try {
                return socket.getInputStream();
            } catch (IOException e) {
                throw new IllegalStateException("failed to access stdio bridge input", e);
            }
        }

        private OutputStream socketOutput() {
            try {
                return socket.getOutputStream();
            } catch (IOException e) {
                throw new IllegalStateException("failed to access stdio bridge output", e);
            }
        }

        private void pump(InputStream input, OutputStream output, boolean shutdownOutput) {
            byte[] buffer = new byte[16 * 1024];
            try {
                while (true) {
                    int read = input.read(buffer);
                    if (read <= 0) {
                        if (shutdownOutput) {
                            try {
                                socket.shutdownOutput();
                            } catch (IOException ignored) {
                                // Socket already closed.
                            }
                        }
                        return;
                    }
                    output.write(buffer, 0, read);
                    output.flush();
                }
            } catch (Exception ignored) {
                // Closed during shutdown or EOF propagation.
            }
        }

        private void markComplete() {
            if (completions.decrementAndGet() == 0) {
                onDisconnect.run();
            }
        }
    }

    private static final class UnixServerBridge implements AutoCloseable {
        private final ServerSocketChannel listener;
        private final String path;
        private final String host;
        private final int port;
        private final AtomicBoolean closed = new AtomicBoolean(false);
        private final List<AutoCloseable> connections = Collections.synchronizedList(new ArrayList<>());
        private Thread acceptThread;

        private UnixServerBridge(String path, String host, int port) throws IOException {
            Transport.Listener listener = Transport.listen("unix://" + path);
            if (!(listener instanceof Transport.UnixListener unixListener)) {
                throw new IllegalArgumentException("expected unix listener for " + path);
            }
            this.listener = unixListener.channel();
            this.path = unixListener.path();
            this.host = host;
            this.port = port;
        }

        private void start() {
            acceptThread = new Thread(this::acceptLoop, "holons-serve-unix-accept");
            acceptThread.setDaemon(true);
            acceptThread.start();
        }

        @Override
        public void close() {
            if (!closed.compareAndSet(false, true)) {
                return;
            }

            try {
                listener.close();
            } catch (IOException ignored) {
            }

            synchronized (connections) {
                for (AutoCloseable connection : new ArrayList<>(connections)) {
                    try {
                        connection.close();
                    } catch (Exception ignored) {
                    }
                }
                connections.clear();
            }

            try {
                Files.deleteIfExists(Path.of(path));
            } catch (IOException ignored) {
            }

            if (acceptThread != null) {
                try {
                    acceptThread.join(200);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }
        }

        private void acceptLoop() {
            while (!closed.get()) {
                SocketChannel client = null;
                try {
                    client = listener.accept();
                    if (client == null) {
                        continue;
                    }
                    if (closed.get()) {
                        client.close();
                        return;
                    }

                    final SocketChannel accepted = client;
                    Thread handler = new Thread(() -> handleClient(accepted), "holons-serve-unix-client");
                    handler.setDaemon(true);
                    handler.start();
                } catch (IOException ignored) {
                    if (closed.get()) {
                        return;
                    }
                    if (client != null) {
                        try {
                            client.close();
                        } catch (IOException closeIgnored) {
                        }
                    }
                }
            }
        }

        private void handleClient(SocketChannel client) {
            Socket upstream = null;
            try {
                upstream = new Socket(host, port);
                track(client);
                track(upstream);

                InputStream clientInput = Channels.newInputStream(client);
                OutputStream clientOutput = Channels.newOutputStream(client);
                InputStream upstreamInput = upstream.getInputStream();
                OutputStream upstreamOutput = upstream.getOutputStream();

                Thread up = new Thread(
                        () -> pump(clientInput, upstreamOutput),
                        "holons-serve-unix-up");
                Thread down = new Thread(
                        () -> pump(upstreamInput, clientOutput),
                        "holons-serve-unix-down");
                up.setDaemon(true);
                down.setDaemon(true);
                up.start();
                down.start();
                up.join();
                down.join();
            } catch (Exception ignored) {
                // Closed during shutdown or client disconnect.
            } finally {
                untrack(client);
                try {
                    client.close();
                } catch (IOException ignored) {
                }
                if (upstream != null) {
                    untrack(upstream);
                    try {
                        upstream.close();
                    } catch (IOException ignored) {
                    }
                }
            }
        }

        private void track(AutoCloseable connection) {
            if (closed.get()) {
                try {
                    connection.close();
                } catch (Exception ignored) {
                }
                return;
            }
            connections.add(connection);
        }

        private void untrack(AutoCloseable connection) {
            connections.remove(connection);
        }

        private void pump(InputStream input, OutputStream output) {
            byte[] buffer = new byte[16 * 1024];
            try {
                while (true) {
                    int read = input.read(buffer);
                    if (read <= 0) {
                        return;
                    }
                    output.write(buffer, 0, read);
                    output.flush();
                }
            } catch (Exception ignored) {
                // Closed during shutdown or EOF propagation.
            }
        }
    }
}

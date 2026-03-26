package org.organicprogramming.holons;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.StandardProtocolFamily;
import java.net.UnixDomainSocketAddress;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Objects;

/**
 * URI-based listener factory for gRPC servers.
 *
 * <p>
 * Supported transports:
 * <ul>
 * <li>{@code tcp://<host>:<port>} — TCP socket</li>
 * <li>{@code unix://<path>} — Unix domain socket</li>
 * <li>{@code stdio://} — stdin/stdout pipe</li>
 * <li>{@code ws://<host>:<port>} — WebSocket endpoint metadata</li>
 * <li>{@code wss://<host>:<port>} — WebSocket over TLS metadata</li>
 * </ul>
 */
public final class Transport {

    /** Default transport URI when --listen is omitted. */
    public static final String DEFAULT_URI = "tcp://:9090";

    private Transport() {
    }

    /**
     * Extract the scheme from a transport URI.
     */
    public static String scheme(String uri) {
        int idx = uri.indexOf("://");
        return idx >= 0 ? uri.substring(0, idx) : uri;
    }

    /** Parsed transport URI. */
    public record ParsedURI(
            String raw,
            String scheme,
            String host,
            Integer port,
            String path,
            boolean secure) {
    }

    /** Marker interface for transport listener variants. */
    public interface Listener {
    }

    /** TCP socket listener. */
    public record TcpListener(ServerSocket socket) implements Listener {
    }

    /** Unix-domain socket listener. */
    public record UnixListener(ServerSocketChannel channel, String path) implements Listener {
    }

    /** stdio listener marker (single connection semantics). */
    public record StdioListener() implements Listener {
    }

    /** WebSocket listener metadata (path + security only, no runtime server). */
    public record WSListener(String host, int port, String path, boolean secure) implements Listener {
    }

    /**
     * Parse a transport URI into a normalized structure.
     */
    public static ParsedURI parseURI(String uri) {
        String s = scheme(uri);
        switch (s) {
            case "tcp":
                if (!uri.startsWith("tcp://")) {
                    throw new IllegalArgumentException("invalid tcp URI: " + uri);
                }
                HostPort tcp = splitHostPort(uri.substring(6), 9090);
                return new ParsedURI(uri, "tcp", tcp.host(), tcp.port(), null, false);
            case "unix":
                if (!uri.startsWith("unix://")) {
                    throw new IllegalArgumentException("invalid unix URI: " + uri);
                }
                String unixPath = uri.substring(7);
                if (unixPath.isEmpty()) {
                    throw new IllegalArgumentException("invalid unix URI: " + uri);
                }
                return new ParsedURI(uri, "unix", null, null, unixPath, false);
            case "stdio":
                return new ParsedURI("stdio://", "stdio", null, null, null, false);
            case "ws":
            case "wss":
                boolean secure = "wss".equals(s);
                String prefix = secure ? "wss://" : "ws://";
                if (!uri.startsWith(prefix)) {
                    throw new IllegalArgumentException("invalid ws URI: " + uri);
                }
                String trimmed = uri.substring(prefix.length());
                String addr = trimmed;
                String path = "/grpc";
                int slash = trimmed.indexOf('/');
                if (slash >= 0) {
                    addr = trimmed.substring(0, slash);
                    path = trimmed.substring(slash);
                    if (path.isEmpty()) {
                        path = "/grpc";
                    }
                }
                HostPort ws = splitHostPort(addr, secure ? 443 : 80);
                return new ParsedURI(uri, s, ws.host(), ws.port(), path, secure);
            default:
                throw new IllegalArgumentException("unsupported transport URI: " + uri);
        }
    }

    /**
     * Parse a transport URI and return a listener variant.
     *
     * <p>
     * TCP and Unix bind real sockets. stdio/ws/wss remain metadata-level variants.
     */
    public static Listener listen(String uri) throws IOException {
        ParsedURI parsed = parseURI(uri);

        switch (parsed.scheme()) {
            case "tcp":
                return new TcpListener(listenTcp(parsed));
            case "unix":
                return new UnixListener(listenUnix(parsed), Objects.requireNonNull(parsed.path()));
            case "stdio":
                return new StdioListener();
            case "ws":
            case "wss":
                return new WSListener(
                        Objects.requireNonNullElse(parsed.host(), "0.0.0.0"),
                        Objects.requireNonNullElse(parsed.port(), parsed.secure() ? 443 : 80),
                        Objects.requireNonNullElse(parsed.path(), "/grpc"),
                        parsed.secure());
            default:
                throw new IllegalArgumentException("unsupported transport URI: " + uri);
        }
    }

    /**
     * Dial a unix:// listener.
     */
    public static SocketChannel dialUnix(String uri) throws IOException {
        ParsedURI parsed = parseURI(uri);
        if (!"unix".equals(parsed.scheme())) {
            throw new IllegalArgumentException("dialUnix expects unix:// URI: " + uri);
        }
        String path = Objects.requireNonNull(parsed.path());
        return SocketChannel.open(UnixDomainSocketAddress.of(path));
    }

    /** Bind a TCP server socket. */
    private static ServerSocket listenTcp(ParsedURI parsed) throws IOException {
        String host = Objects.requireNonNullElse(parsed.host(), "0.0.0.0");
        int port = Objects.requireNonNullElse(parsed.port(), 9090);

        ServerSocket ss = new ServerSocket();
        ss.setReuseAddress(true);
        ss.bind(new InetSocketAddress(host, port));
        return ss;
    }

    private static ServerSocketChannel listenUnix(ParsedURI parsed) throws IOException {
        String path = Objects.requireNonNull(parsed.path());
        Files.deleteIfExists(Path.of(path));

        ServerSocketChannel channel = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
        channel.bind(UnixDomainSocketAddress.of(path));
        return channel;
    }

    private record HostPort(String host, int port) {
    }

    private static HostPort splitHostPort(String addr, int defaultPort) {
        if (addr.isEmpty()) {
            return new HostPort("0.0.0.0", defaultPort);
        }
        int colonIdx = addr.lastIndexOf(':');
        if (colonIdx >= 0) {
            String host = addr.substring(0, colonIdx);
            String portText = addr.substring(colonIdx + 1);
            int port = portText.isEmpty() ? defaultPort : Integer.parseInt(portText);
            return new HostPort(host.isEmpty() ? "0.0.0.0" : host, port);
        }
        return new HostPort(addr, defaultPort);
    }
}

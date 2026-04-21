using System;
using System.IO;
using System.Net;
using System.Net.Sockets;

namespace Holons;

/// <summary>
/// URI-based listener factory for gRPC servers.
/// Supported: tcp://, unix://, stdio://, ws://, wss://
/// </summary>
public static class Transport
{
    /// <summary>Default transport URI when --listen is omitted.</summary>
    public const string DefaultUri = "tcp://:9090";

    /// <summary>Extract the scheme from a transport URI.</summary>
    public static string Scheme(string uri)
    {
        int idx = uri.IndexOf("://", StringComparison.Ordinal);
        return idx >= 0 ? uri[..idx] : uri;
    }

    public record ParsedUri(
        string Raw,
        string Scheme,
        string? Host = null,
        int? Port = null,
        string? Path = null,
        bool Secure = false
    );

    public abstract record TransportListener
    {
        public sealed record Tcp(TcpListener Socket) : TransportListener;
        public sealed record Unix(Socket Socket, string Path) : TransportListener;
        public sealed record Stdio : TransportListener;
        public sealed record Ws(string Host, int Port, string Path, bool Secure) : TransportListener;
    }

    /// <summary>Parse a transport URI into a normalized structure.</summary>
    public static ParsedUri ParseUri(string uri)
    {
        var s = Scheme(uri);
        switch (s)
        {
            case "tcp":
                if (!uri.StartsWith("tcp://", StringComparison.Ordinal))
                    throw new ArgumentException($"invalid tcp URI: {uri}");
                var tcp = SplitHostPort(uri[6..], 9090);
                return new ParsedUri(uri, "tcp", tcp.host, tcp.port);

            case "unix":
                if (!uri.StartsWith("unix://", StringComparison.Ordinal))
                    throw new ArgumentException($"invalid unix URI: {uri}");
                var unixPath = uri[7..];
                if (string.IsNullOrEmpty(unixPath))
                    throw new ArgumentException($"invalid unix URI: {uri}");
                return new ParsedUri(uri, "unix", Path: unixPath);

            case "stdio":
                return new ParsedUri("stdio://", "stdio");

            case "ws":
            case "wss":
                var secure = s == "wss";
                var prefix = secure ? "wss://" : "ws://";
                if (!uri.StartsWith(prefix, StringComparison.Ordinal))
                    throw new ArgumentException($"invalid ws URI: {uri}");

                var trimmed = uri[prefix.Length..];
                var slash = trimmed.IndexOf('/');
                var addr = slash >= 0 ? trimmed[..slash] : trimmed;
                var path = slash >= 0 ? trimmed[slash..] : "/grpc";
                if (string.IsNullOrEmpty(path))
                    path = "/grpc";

                var ws = SplitHostPort(addr, secure ? 443 : 80);
                return new ParsedUri(uri, s, ws.host, ws.port, path, secure);

            default:
                throw new ArgumentException($"unsupported transport URI: {uri}");
        }
    }

    /// <summary>Parse a transport URI and create a listener variant.</summary>
    public static TransportListener Listen(string uri)
    {
        var parsed = ParseUri(uri);
        return parsed.Scheme switch
        {
            "tcp" => new TransportListener.Tcp(ListenTcp(parsed)),
            "unix" => new TransportListener.Unix(
                ListenUnix(parsed),
                parsed.Path ?? throw new ArgumentException("unix path missing")),
            "stdio" => new TransportListener.Stdio(),
            "ws" or "wss" => new TransportListener.Ws(
                parsed.Host ?? "0.0.0.0",
                parsed.Port ?? (parsed.Secure ? 443 : 80),
                parsed.Path ?? "/grpc",
                parsed.Secure
            ),
            _ => throw new ArgumentException($"unsupported transport URI: {uri}")
        };
    }

    public static Socket DialUnix(string uri)
    {
        var parsed = ParseUri(uri);
        if (parsed.Scheme != "unix")
            throw new ArgumentException($"DialUnix expects unix:// URI: {uri}");

        var path = parsed.Path ?? throw new ArgumentException($"invalid unix URI: {uri}");
        var socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        socket.Connect(new UnixDomainSocketEndPoint(path));
        return socket;
    }

    private static TcpListener ListenTcp(ParsedUri parsed)
    {
        var host = parsed.Host ?? "0.0.0.0";
        var port = parsed.Port ?? 9090;
        var listener = new TcpListener(ResolveAddress(host), port);
        listener.Start();
        return listener;
    }

    private static Socket ListenUnix(ParsedUri parsed)
    {
        var path = parsed.Path ?? throw new ArgumentException("unix path missing");
        if (File.Exists(path))
            File.Delete(path);

        var socket = new Socket(AddressFamily.Unix, SocketType.Stream, ProtocolType.Unspecified);
        socket.Bind(new UnixDomainSocketEndPoint(path));
        socket.Listen(128);
        return socket;
    }

    private static (string host, int port) SplitHostPort(string addr, int defaultPort)
    {
        if (string.IsNullOrEmpty(addr))
            return ("0.0.0.0", defaultPort);

        int lastColon = addr.LastIndexOf(':');
        if (lastColon >= 0)
        {
            string host = lastColon > 0 ? addr[..lastColon] : "0.0.0.0";
            string portText = addr[(lastColon + 1)..];
            int port = string.IsNullOrEmpty(portText) ? defaultPort : int.Parse(portText);
            return (host, port);
        }

        return (addr, defaultPort);
    }

    private static IPAddress ResolveAddress(string host)
    {
        if (host == "0.0.0.0")
            return IPAddress.Any;
        if (host == "::")
            return IPAddress.IPv6Any;
        if (IPAddress.TryParse(host, out var ip))
            return ip;

        var candidates = Dns.GetHostAddresses(host);
        if (candidates.Length == 0)
            throw new ArgumentException($"unable to resolve host: {host}");
        return candidates[0];
    }
}

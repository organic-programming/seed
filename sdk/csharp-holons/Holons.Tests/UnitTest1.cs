using Holons;
using System.Net.Sockets;
using System.Text;

namespace Holons.Tests;

public class HolonsTest
{
    // --- Transport ---

    [Fact]
    public void SchemeExtraction()
    {
        Assert.Equal("tcp", Transport.Scheme("tcp://:9090"));
        Assert.Equal("unix", Transport.Scheme("unix:///tmp/x.sock"));
        Assert.Equal("stdio", Transport.Scheme("stdio://"));
        Assert.Equal("ws", Transport.Scheme("ws://127.0.0.1:8080/grpc"));
        Assert.Equal("wss", Transport.Scheme("wss://example.com:443/grpc"));
    }

    [Fact]
    public void DefaultUri()
    {
        Assert.Equal("tcp://:9090", Transport.DefaultUri);
    }

    [Fact]
    public void TcpListen()
    {
        var lis = Assert.IsType<Transport.TransportListener.Tcp>(
            Transport.Listen("tcp://127.0.0.1:0"));
        try
        {
            var endpoint = (System.Net.IPEndPoint)lis.Socket.LocalEndpoint;
            Assert.True(endpoint.Port > 0);
        }
        finally
        {
            lis.Socket.Stop();
        }
    }

    [Fact]
    public void ParseUriWssDefaultPath()
    {
        var parsed = Transport.ParseUri("wss://example.com:8443");
        Assert.Equal("wss", parsed.Scheme);
        Assert.Equal("example.com", parsed.Host);
        Assert.Equal(8443, parsed.Port);
        Assert.Equal("/grpc", parsed.Path);
        Assert.True(parsed.Secure);
    }

    [Fact]
    public void StdioListenVariant()
    {
        Assert.IsType<Transport.TransportListener.Stdio>(Transport.Listen("stdio://"));
    }

    [Fact]
    public void UnixListenAndDialRoundTrip()
    {
        var path = Path.Combine(Path.GetTempPath(), $"holons-csharp-{Guid.NewGuid():N}.sock");
        var uri = $"unix://{path}";
        var lis = Assert.IsType<Transport.TransportListener.Unix>(Transport.Listen(uri));

        var serverTask = Task.Run(() =>
        {
            using var accepted = lis.Socket.Accept();
            var inbound = new byte[4];
            var read = 0;
            while (read < inbound.Length)
                read += accepted.Receive(inbound, read, inbound.Length - read, SocketFlags.None);
            accepted.Send(inbound, SocketFlags.None);
        });

        using var client = Transport.DialUnix(uri);
        client.Send(Encoding.UTF8.GetBytes("ping"), SocketFlags.None);

        var outbound = new byte[4];
        var got = 0;
        while (got < outbound.Length)
            got += client.Receive(outbound, got, outbound.Length - got, SocketFlags.None);
        Assert.Equal("ping", Encoding.UTF8.GetString(outbound));

        serverTask.Wait(TimeSpan.FromSeconds(3));
        lis.Socket.Dispose();
        if (File.Exists(path))
            File.Delete(path);
    }

    [Fact]
    public void WsListenVariant()
    {
        var ws = Assert.IsType<Transport.TransportListener.Ws>(
            Transport.Listen("ws://127.0.0.1:8080/holon"));
        Assert.Equal("127.0.0.1", ws.Host);
        Assert.Equal(8080, ws.Port);
        Assert.Equal("/holon", ws.Path);
        Assert.False(ws.Secure);
    }

    [Fact]
    public void UnsupportedUri()
    {
        Assert.Throws<ArgumentException>(() => Transport.Listen("ftp://host"));
    }

    // --- Serve ---

    [Fact]
    public void ParseFlagsListen()
    {
        Assert.Equal("tcp://:8080",
            Serve.ParseFlags(new[] { "--listen", "tcp://:8080" }));
    }

    [Fact]
    public void ParseFlagsPort()
    {
        Assert.Equal("tcp://:3000",
            Serve.ParseFlags(new[] { "--port", "3000" }));
    }

    [Fact]
    public void ParseFlagsDefault()
    {
        Assert.Equal(Transport.DefaultUri, Serve.ParseFlags(Array.Empty<string>()));
    }

    // --- Identity ---

    [Fact]
    public void ParseHolon()
    {
        var root = Path.Combine(Path.GetTempPath(), $"holons-csharp-identity-{Guid.NewGuid():N}");
        Directory.CreateDirectory(root);
        var manifestPath = Path.Combine(root, "holon.proto");
        File.WriteAllText(manifestPath,
            """
            syntax = "proto3";
            package test.v1;

            option (holons.v1.manifest) = {
              identity: {
                uuid: "abc-123"
                given_name: "test"
                family_name: "Test"
                motto: "A test."
                clade: "deterministic/pure"
              }
              lang: "csharp"
            };
            """);

        var id = Identity.ParseHolon(manifestPath);
        Assert.Equal("abc-123", id.Uuid);
        Assert.Equal("test", id.GivenName);
        Assert.Equal("csharp", id.Lang);

        var resolved = Identity.Resolve(root);
        Assert.Equal(Path.GetFullPath(manifestPath), resolved.SourcePath);

        Directory.Delete(root, recursive: true);
    }

    [Fact]
    public void ParseInvalidMapping()
    {
        var tmpFile = Path.GetTempFileName();
        File.WriteAllText(tmpFile, "syntax = \"proto3\";\npackage test.v1;\n");
        Assert.Throws<FormatException>(() => Identity.ParseHolon(tmpFile));
        File.Delete(tmpFile);
    }
}

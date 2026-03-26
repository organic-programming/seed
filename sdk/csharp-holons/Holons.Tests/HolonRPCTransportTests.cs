using System.Net;
using System.Net.WebSockets;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json.Nodes;
using Holons;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace Holons.Tests;

public class HolonRPCTransportTests
{
    [Fact]
    public async Task HolonRpcWssDialRoundTrip()
    {
        using var certificate = CreateSelfSignedCertificate();
        await using var server = await StartWssServerAsync(certificate);
        await using var client = new HolonRPCClient(
            heartbeatIntervalMs: 250,
            heartbeatTimeoutMs: 250,
            reconnectMinDelayMs: 100,
            reconnectMaxDelayMs: 400,
            serverCertificateValidation: (_, _, _, _) => true);

        await client.ConnectAsync(server.Url);
        var result = await client.InvokeAsync(
            "echo.v1.Echo/Ping",
            new JsonObject { ["message"] = "secure-hello" });

        Assert.Equal("secure-hello", result["message"]?.GetValue<string>());
        Assert.True(result["secure"]?.GetValue<bool>());
    }

    [Fact]
    public async Task HolonRpcHttpInvokeSupportsUnaryPost()
    {
        await using var server = await StartHttpSseServerAsync();
        await using var client = new HolonRPCClient();

        await client.ConnectAsync(server.Url);
        var result = await client.InvokeAsync(
            "echo.v1.Echo/Ping",
            new JsonObject { ["message"] = "hola-http" });

        Assert.Equal("hola-http", result["message"]?.GetValue<string>());
        Assert.Equal("http", result["transport"]?.GetValue<string>());
    }

    [Fact]
    public async Task HolonRpcHttpStreamSupportsPostAndGet()
    {
        await using var server = await StartHttpSseServerAsync();
        await using var client = new HolonRPCClient();

        await client.ConnectAsync(server.Url);

        var postEvents = await client.StreamAsync(
            "build.v1.Build/Watch",
            new JsonObject { ["project"] = "myapp" });
        Assert.Equal(3, postEvents.Count);
        Assert.Equal("message", postEvents[0].Event);
        Assert.Equal("1", postEvents[0].Id);
        Assert.Equal("building", postEvents[0].Result?["status"]?.GetValue<string>());
        Assert.Equal("done", postEvents[2].Event);

        var getEvents = await client.StreamQueryAsync(
            "build.v1.Build/Watch",
            new Dictionary<string, string> { ["project"] = "query-app" });
        Assert.Equal(3, getEvents.Count);
        Assert.Equal("query-app", getEvents[0].Result?["project"]?.GetValue<string>());
        Assert.Equal("done", getEvents[2].Event);
    }

    private static async Task<RunningTestServer> StartWssServerAsync(X509Certificate2 certificate)
    {
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.ConfigureKestrel(options =>
        {
            options.Listen(IPAddress.Loopback, 0, listen =>
            {
                listen.UseHttps(certificate);
                listen.Protocols = HttpProtocols.Http1;
            });
        });

        var app = builder.Build();
        app.UseWebSockets();
        app.Map("/rpc", wsApp =>
        {
            wsApp.Run(async context =>
            {
                if (!context.WebSockets.IsWebSocketRequest)
                {
                    context.Response.StatusCode = StatusCodes.Status400BadRequest;
                    return;
                }

                using var socket = await context.WebSockets.AcceptWebSocketAsync("holon-rpc");
                var request = await ReceiveJsonAsync(socket, context.RequestAborted);
                var message = request["params"]?["message"]?.GetValue<string>() ?? string.Empty;
                var response = new JsonObject
                {
                    ["jsonrpc"] = "2.0",
                    ["id"] = request["id"]?.DeepClone(),
                    ["result"] = new JsonObject
                    {
                        ["message"] = message,
                        ["secure"] = true,
                    },
                };

                await socket.SendAsync(
                    Encoding.UTF8.GetBytes(response.ToJsonString()),
                    WebSocketMessageType.Text,
                    endOfMessage: true,
                    context.RequestAborted);
                await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "done", context.RequestAborted);
            });
        });

        await app.StartAsync();

        var address = app.Services
            .GetRequiredService<IServer>()
            .Features
            .Get<IServerAddressesFeature>()?
            .Addresses
            .Single() ?? throw new InvalidOperationException("wss test server did not report an address");

        return new RunningTestServer(app, "wss://" + new Uri(address).Authority + "/rpc");
    }

    private static async Task<RunningTestServer> StartHttpSseServerAsync()
    {
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.ConfigureKestrel(options =>
        {
            options.Listen(IPAddress.Loopback, 0, listen =>
            {
                listen.Protocols = HttpProtocols.Http1;
            });
        });

        var app = builder.Build();
        app.MapPost("/api/v1/rpc/echo.v1.Echo/Ping", async context =>
        {
            var payload = (await JsonNode.ParseAsync(context.Request.Body, cancellationToken: context.RequestAborted)
                .ConfigureAwait(false)) as JsonObject ?? new JsonObject();
            var message = payload["message"]?.GetValue<string>() ?? string.Empty;

            context.Response.StatusCode = StatusCodes.Status200OK;
            context.Response.ContentType = "application/json";
            await context.Response.WriteAsync(
                new JsonObject
                {
                    ["jsonrpc"] = "2.0",
                    ["id"] = "h1",
                    ["result"] = new JsonObject
                    {
                        ["message"] = message,
                        ["transport"] = "http",
                    },
                }.ToJsonString(),
                context.RequestAborted);
        });
        app.MapMethods("/api/v1/rpc/build.v1.Build/Watch", new[] { "GET", "POST" }, async context =>
        {
            if (!context.Request.Headers.Accept.ToString().Contains("text/event-stream", StringComparison.OrdinalIgnoreCase))
            {
                context.Response.StatusCode = StatusCodes.Status400BadRequest;
                return;
            }

            var project = context.Request.Method == HttpMethods.Get
                ? context.Request.Query["project"].ToString()
                : (((await JsonNode.ParseAsync(context.Request.Body, cancellationToken: context.RequestAborted)
                    .ConfigureAwait(false)) as JsonObject)?["project"]?.GetValue<string>() ?? string.Empty);

            context.Response.StatusCode = StatusCodes.Status200OK;
            context.Response.ContentType = "text/event-stream";

            await WriteSseAsync(
                context,
                "message",
                "1",
                new JsonObject
                {
                    ["jsonrpc"] = "2.0",
                    ["id"] = "s1",
                    ["result"] = new JsonObject
                    {
                        ["project"] = project,
                        ["status"] = "building",
                    },
                });
            await WriteSseAsync(
                context,
                "message",
                "2",
                new JsonObject
                {
                    ["jsonrpc"] = "2.0",
                    ["id"] = "s1",
                    ["result"] = new JsonObject
                    {
                        ["project"] = project,
                        ["status"] = "done",
                    },
                });
            await context.Response.WriteAsync("event: done\ndata:\n\n", context.RequestAborted);
            await context.Response.Body.FlushAsync(context.RequestAborted);
        });

        await app.StartAsync();

        var address = app.Services
            .GetRequiredService<IServer>()
            .Features
            .Get<IServerAddressesFeature>()?
            .Addresses
            .Single() ?? throw new InvalidOperationException("http+sse test server did not report an address");

        return new RunningTestServer(app, new Uri(new Uri(address), "/api/v1/rpc").ToString().TrimEnd('/'));
    }

    private static async Task<JsonObject> ReceiveJsonAsync(WebSocket socket, CancellationToken cancellationToken)
    {
        var buffer = new byte[16 * 1024];
        using var stream = new MemoryStream();

        while (true)
        {
            var result = await socket.ReceiveAsync(buffer, cancellationToken);
            if (result.MessageType == WebSocketMessageType.Close)
                throw new InvalidOperationException("websocket closed before request was received");

            stream.Write(buffer, 0, result.Count);
            if (result.EndOfMessage)
                break;
        }

        return JsonNode.Parse(Encoding.UTF8.GetString(stream.ToArray()))?.AsObject()
            ?? throw new InvalidOperationException("invalid websocket JSON payload");
    }

    private static async Task WriteSseAsync(HttpContext context, string eventType, string id, JsonObject payload)
    {
        await context.Response.WriteAsync($"event: {eventType}\n", context.RequestAborted);
        await context.Response.WriteAsync($"id: {id}\n", context.RequestAborted);
        await context.Response.WriteAsync($"data: {payload.ToJsonString()}\n\n", context.RequestAborted);
        await context.Response.Body.FlushAsync(context.RequestAborted);
    }

    private static X509Certificate2 CreateSelfSignedCertificate()
    {
        using var rsa = RSA.Create(2048);
        var request = new CertificateRequest(
            "CN=127.0.0.1",
            rsa,
            HashAlgorithmName.SHA256,
            RSASignaturePadding.Pkcs1);
        request.CertificateExtensions.Add(
            new X509BasicConstraintsExtension(false, false, 0, false));
        request.CertificateExtensions.Add(
            new X509KeyUsageExtension(X509KeyUsageFlags.DigitalSignature, false));
        request.CertificateExtensions.Add(
            new X509SubjectKeyIdentifierExtension(request.PublicKey, false));

        var sanBuilder = new SubjectAlternativeNameBuilder();
        sanBuilder.AddIpAddress(IPAddress.Loopback);
        sanBuilder.AddDnsName("localhost");
        request.CertificateExtensions.Add(sanBuilder.Build());

        var certificate = request.CreateSelfSigned(
            DateTimeOffset.UtcNow.AddDays(-1),
            DateTimeOffset.UtcNow.AddDays(30));

        return new X509Certificate2(certificate.Export(X509ContentType.Pfx));
    }

    private sealed class RunningTestServer(WebApplication app, string url) : IAsyncDisposable
    {
        public string Url { get; } = url;

        public async ValueTask DisposeAsync()
        {
            await app.StopAsync();
            await app.DisposeAsync();
        }
    }
}

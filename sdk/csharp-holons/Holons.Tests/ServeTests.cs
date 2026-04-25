using Grpc.Core;
using Grpc.Net.Client;
using Holons.V1;
using Obs = Holons.Observability;

namespace Holons.Tests;

public class ServeTests
{
    [Fact]
    public async Task StartWithOptionsAdvertisesEphemeralTcpAndAutoRegistersDescribe()
    {
        var root = CreateTempHolon();
        try
        {
            Describe.UseStaticResponse(Describe.BuildResponse(Path.Combine(root, "protos")));
            using var server = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions());

            var callInvoker = GrpcChannel.ForAddress(
                $"http://127.0.0.1:{server.PublicUri.Split(':').Last()}").CreateCallInvoker();
            var response = await callInvoker.AsyncUnaryCall(
                Describe.DescribeMethod,
                string.Empty,
                new CallOptions(),
                new DescribeRequest()).ResponseAsync;

            Assert.Equal("Echo", response.Manifest.Identity.GivenName);
            Assert.Equal("echo.v1.Echo", Assert.Single(response.Services).Name);
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task StartWithOptionsUsesWorkingDirectoryDefaultsForDescribe()
    {
        var root = CreateTempHolon();
        var previousDirectory = Directory.GetCurrentDirectory();
        try
        {
            Directory.SetCurrentDirectory(root);
            Describe.UseStaticResponse(Describe.BuildResponse(Path.Combine(root, "protos")));

            using var server = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions());

            var callInvoker = GrpcChannel.ForAddress(
                $"http://127.0.0.1:{server.PublicUri.Split(':').Last()}").CreateCallInvoker();
            var response = await callInvoker.AsyncUnaryCall(
                Describe.DescribeMethod,
                string.Empty,
                new CallOptions(),
                new DescribeRequest()).ResponseAsync;

            Assert.Equal("Echo", response.Manifest.Identity.GivenName);
            Assert.Equal("echo.v1.Echo", Assert.Single(response.Services).Name);
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Directory.SetCurrentDirectory(previousDirectory);
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task StartWithOptionsAutoRegistersObservability()
    {
        var root = CreateTempHolon();
        try
        {
            Describe.UseStaticResponse(Describe.BuildResponse(Path.Combine(root, "protos")));
            Obs.ObservabilityRegistry.Reset();
            var registryRoot = Path.Combine(root, "runs");
            using var server = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions
                {
                    Env = new Dictionary<string, string>
                    {
                        ["OP_OBS"] = "logs,metrics,events",
                        ["OP_RUN_DIR"] = registryRoot,
                        ["OP_INSTANCE_UID"] = "csharp-obs-1",
                    },
                });

            var obs = Obs.ObservabilityRegistry.Current();
            obs.Logger("serve-test").Info("serve-log", new Dictionary<string, object?> { ["sdk"] = "csharp" });
            obs.Counter("serve_requests_total")!.Inc();
            obs.Emit(Obs.EventType.InstanceReady);

            using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
            var client = new HolonObservability.HolonObservabilityClient(channel);

            var logs = new List<LogEntry>();
            using (var call = client.Logs(new LogsRequest { MinLevel = LogLevel.Info }))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    logs.Add(call.ResponseStream.Current);
            }
            Assert.Contains(logs, entry => entry.Message == "serve-log");

            var metrics = await client.MetricsAsync(new MetricsRequest());
            Assert.Contains(metrics.Samples, sample => sample.Name == "serve_requests_total");

            var events = new List<EventInfo>();
            using (var call = client.Events(new EventsRequest()))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    events.Add(call.ResponseStream.Current);
            }
            Assert.Contains(events, ev => ev.Type == EventType.InstanceReady);

            Assert.True(File.Exists(Path.Combine(registryRoot, obs.Config.Slug, "csharp-obs-1", "meta.json")));
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Obs.ObservabilityRegistry.Reset();
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task StartWithOptionsAdvertisesUnixAndAutoRegistersDescribe()
    {
        var root = CreateTempHolon();
        try
        {
            Describe.UseStaticResponse(Describe.BuildResponse(Path.Combine(root, "protos")));
            var socketPath = Path.Combine(root, "serve.sock");
            using var server = Serve.StartWithOptions(
                $"unix://{socketPath}",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions());

            using var channel = Connect.ConnectTarget($"unix://{socketPath}");
            var response = await channel.CreateCallInvoker().AsyncUnaryCall(
                Describe.DescribeMethod,
                string.Empty,
                new CallOptions(),
                new DescribeRequest()).ResponseAsync;

            Assert.Equal($"unix://{socketPath}", server.PublicUri);
            Assert.Equal("Echo", response.Manifest.Identity.GivenName);
            Assert.Equal("echo.v1.Echo", Assert.Single(response.Services).Name);
            Connect.Disconnect(channel);
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public void StartWithOptionsFailsWithoutStaticDescribeEvenWhenProtoExists()
    {
        var root = CreateTempHolon();
        var previousDirectory = Directory.GetCurrentDirectory();
        var logs = new List<string>();

        try
        {
            Describe.UseStaticResponse(null);
            Directory.SetCurrentDirectory(root);

            var error = Assert.Throws<InvalidOperationException>(() => Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions
                {
                    Logger = logs.Add,
                }));

            Assert.Equal(Describe.NoIncodeDescriptionMessage, error.Message);
            Assert.Contains(logs, line => line.Contains("HolonMeta registration failed", StringComparison.Ordinal));
            Assert.Contains(logs, line => line.Contains(Describe.NoIncodeDescriptionMessage, StringComparison.Ordinal));
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Directory.SetCurrentDirectory(previousDirectory);
            Directory.Delete(root, recursive: true);
        }
    }

    private static string CreateTempHolon()
    {
        var id = Guid.NewGuid().ToString("N")[..8];
        var root = Path.Combine(Path.GetTempPath(), $"hcs-{id}");
        var protoDir = Path.Combine(root, "protos", "echo", "v1");
        Directory.CreateDirectory(protoDir);

        File.WriteAllText(
            Path.Combine(root, "holon.proto"),
            """
            syntax = "proto3";
            package holons.test.v1;

            option (holons.v1.manifest) = {
              identity: {
                given_name: "Echo"
                family_name: "Server"
                motto: "Reply precisely."
              }
            };
            """);
        File.WriteAllText(
            Path.Combine(protoDir, "echo.proto"),
            """
            syntax = "proto3";
            package echo.v1;

            service Echo {
              rpc Ping(PingRequest) returns (PingResponse);
            }

            message PingRequest {
              string message = 1;
            }

            message PingResponse {
              string message = 1;
            }
            """);

        return root;
    }
}

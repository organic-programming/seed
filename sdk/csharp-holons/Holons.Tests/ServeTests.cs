using Grpc.Core;
using Grpc.Net.Client;
using Holons.V1;
using Obs = Holons.Observability;
using System.Text.Json;

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
                    Slug = "echo-server",
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
            obs.Emit(Obs.EventNames.InstanceReady);

            using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
            var client = new HolonObservability.HolonObservabilityClient(channel);

            var logs = new List<LogRecord>();
            using (var call = client.Logs(new LogsRequest { MinSeverityNumber = SeverityNumber.Info }))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    logs.Add(call.ResponseStream.Current);
            }
            Assert.Contains(logs, entry => Obs.Wire.AnyValueString(entry.Body) == "serve-log");

            var metrics = new List<Metric>();
            using (var call = client.Metrics(new MetricsRequest()))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    metrics.Add(call.ResponseStream.Current);
            }
            Assert.Contains(metrics, sample => sample.Name == "serve_requests_total");

            var events = new List<LogRecord>();
            using (var call = client.Events(new EventsRequest()))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    events.Add(call.ResponseStream.Current);
            }
            Assert.Contains(events, ev => ev.EventName == Obs.EventNames.InstanceReady);

            Assert.True(File.Exists(Path.Combine(registryRoot, "echo-server", "csharp-obs-1", "meta.json")));
        }
        finally
        {
            Describe.UseStaticResponse(null);
            Obs.ObservabilityRegistry.Reset();
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task CurrentTransportTracksStdioServeLifecycle()
    {
        Assert.Equal("", Serve.CurrentTransport);
        await using var server = Serve.StartWithOptions(
            "stdio://",
            Array.Empty<Serve.GrpcServiceRegistration>(),
            new Serve.ServeOptions { Describe = false, Logger = _ => { } });

        Assert.Equal("stdio", Serve.CurrentTransport);
        await server.StopAsync();
        Assert.Equal("", Serve.CurrentTransport);
    }

    [Fact]
    public async Task StartWithOptionsRelaysMemberObservabilityAndWritesPrometheusAddress()
    {
        var root = CreateTempHolon();
        try
        {
            Describe.UseStaticResponse(null);
            Obs.ObservabilityRegistry.Reset();

            var childObs = Obs.ObservabilityRegistry.ConfigureFromEnv(
                new Obs.ObsConfig
                {
                    Slug = "child-holon",
                    InstanceUid = "child-1",
                },
                new Dictionary<string, string> { ["OP_OBS"] = "logs,events,metrics" });
            await using var child = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                new[] { Serve.Service(new Obs.ObservabilityGrpcService(childObs)) },
                new Serve.ServeOptions { Describe = false });
            var childEvent = new LogRecord
            {
                TimeUnixNano = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000,
                ObservedTimeUnixNano = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000,
                EventName = Obs.EventNames.InstanceReady,
            };
            childEvent.Attributes.Add(Obs.Wire.KeyValue(Obs.AttributeNames.HolonsSlug, "grandchild-holon"));
            childEvent.Attributes.Add(Obs.Wire.KeyValue(Obs.AttributeNames.ServiceName, "grandchild-holon"));
            childEvent.Attributes.Add(Obs.Wire.KeyValue(Obs.AttributeNames.HolonsInstanceUid, "grandchild-1"));
            childEvent.Attributes.Add(Obs.Wire.KeyValue(Obs.AttributeNames.ServiceInstanceId, "grandchild-1"));
            childEvent.Attributes.Add(Obs.Wire.KeyValue(Obs.AttributeNames.HolonsSessionId, ""));
            childEvent.Chain.Add("grandchild-holon");
            childObs.EventBus!.Emit(new Obs.LogRecord { Record = childEvent });
            childObs.Emit(Obs.EventNames.InstanceReady, new Dictionary<string, object?> { ["listener"] = child.PublicUri });
            childObs.Logger("tick").Info(
                "tick received",
                new Dictionary<string, object?>
                {
                    ["sender"] = "serve-test",
                    ["responder_uid"] = "child-1",
                });

            var registryRoot = Path.Combine(root, "runs");
            await using var server = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                Array.Empty<Serve.GrpcServiceRegistration>(),
                new Serve.ServeOptions
                {
                    Describe = false,
                    Slug = "root-holon",
                    Env = new Dictionary<string, string>
                    {
                        ["OP_OBS"] = "logs,metrics,events,prom",
                        ["OP_RUN_DIR"] = registryRoot,
                        ["OP_INSTANCE_UID"] = "root-1",
                        ["OP_PROM_ADDR"] = "127.0.0.1:0",
                    },
                    MemberEndpoints = new[] { new Serve.MemberRef("child-holon", child.PublicUri) },
                });
            var rootObs = Obs.ObservabilityRegistry.Current();
            rootObs.Counter(
                "root_ticks_total",
                "root ticks",
                new Dictionary<string, string> { ["responder_uid"] = "root-1" })!.Inc();

            Assert.True(await WaitUntilAsync(() =>
                rootObs.LogRing?.Drain().Any(entry =>
                    Obs.Wire.AnyValueString(entry.Record.Body) == "tick received" &&
                    Obs.Wire.AttributeString(entry.Record.Attributes, Obs.AttributeNames.HolonsInstanceUid) == "child-1" &&
                    entry.Record.Chain.Count == 1 &&
                    entry.Record.Chain[0] == "child-holon") == true));
            Assert.True(await WaitUntilAsync(() =>
                rootObs.EventBus?.Drain().Any(ev =>
                    ev.Record.EventName == Obs.EventNames.InstanceReady &&
                    Obs.Wire.AttributeString(ev.Record.Attributes, Obs.AttributeNames.HolonsInstanceUid) == "child-1" &&
                    ev.Record.Chain.Count == 1 &&
                    ev.Record.Chain[0] == "child-holon") == true));

            var metaPath = Path.Combine(registryRoot, "root-holon", "root-1", "meta.json");
            Assert.True(await WaitUntilAsync(() => File.Exists(metaPath)));
            using var meta = JsonDocument.Parse(await File.ReadAllTextAsync(metaPath));
            var metricsAddr = meta.RootElement.GetProperty("metrics_addr").GetString();
            Assert.False(string.IsNullOrWhiteSpace(metricsAddr));

            using var http = new HttpClient();
            var prometheus = await http.GetStringAsync(metricsAddr);
            Assert.Contains("# TYPE root_ticks_total counter", prometheus);
            Assert.Contains("responder_uid=\"root-1\"", prometheus);
            Assert.Contains("instance_uid=\"root-1\"", prometheus);
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

    private static async Task<bool> WaitUntilAsync(Func<bool> condition, int timeoutMillis = 5000)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromMilliseconds(timeoutMillis);
        while (DateTime.UtcNow < deadline)
        {
            if (condition())
                return true;
            await Task.Delay(50);
        }
        return condition();
    }
}

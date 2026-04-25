using Holons.Observability;
using Holons.V1;
using Grpc.Net.Client;
using System.Text.Json;

namespace Holons.Tests;

public class ObservabilityTests
{
    [Fact]
    public void ParseOpObsRejectsV2Tokens()
    {
        var all = new HashSet<Family> { Family.Logs, Family.Metrics, Family.Events, Family.Prom };
        Assert.True(all.SetEquals(Env.ParseOpObs("all")));
        Assert.Throws<InvalidTokenException>(() => Env.ParseOpObs("all,otel"));
        Assert.Throws<InvalidTokenException>(() => Env.ParseOpObs("all,sessions"));
        Assert.Throws<InvalidTokenException>(() => Env.ParseOpObs("unknown"));
    }

    [Fact]
    public void CheckEnvRejectsV2TokensAndOpSessions()
    {
        Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_OBS"] = "logs,otel" }));
        Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_OBS"] = "logs,sessions" }));
        var err = Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_SESSIONS"] = "metrics" }));
        Assert.Equal("OP_SESSIONS", err.Variable);
    }

    [Fact]
    public void RunDirDerivesFromRegistryRoot()
    {
        var root = CreateTempDir();
        try
        {
            var obs = ObservabilityRegistry.ConfigureFromEnv(
                new ObsConfig
                {
                    Slug = "gabriel-greeting-csharp",
                    InstanceUid = "uid-1",
                    RunDir = root,
                },
                new Dictionary<string, string> { ["OP_OBS"] = "logs" });

            Assert.Equal(Path.Combine(root, "gabriel-greeting-csharp", "uid-1"), obs.Config.RunDir);
        }
        finally
        {
            ObservabilityRegistry.Reset();
            Directory.Delete(root, recursive: true);
        }
    }

    [Fact]
    public async Task ServiceReplaysLogsMetricsEventsAndDiskFiles()
    {
        var root = CreateTempDir();
        try
        {
            var obs = ObservabilityRegistry.ConfigureFromEnv(
                new ObsConfig
                {
                    Slug = "gabriel-greeting-csharp",
                    InstanceUid = "uid-2",
                    RunDir = root,
                },
                new Dictionary<string, string> { ["OP_OBS"] = "logs,metrics,events" });
            DiskWriters.Enable(obs.Config.RunDir);
            obs.Logger("test").Info("service-log", new Dictionary<string, object?> { ["component"] = "csharp" });
            obs.Counter("csharp_requests_total")!.Inc();
            obs.Emit(Holons.Observability.EventType.InstanceReady, new Dictionary<string, string> { ["listener"] = "tcp://127.0.0.1:1" });
            MetaJson.Write(
                obs.Config.RunDir,
                new MetaJson
                {
                    Slug = obs.Config.Slug,
                    Uid = obs.Config.InstanceUid,
                    Pid = 123,
                    StartedAt = DateTime.UtcNow,
                    Transport = "tcp",
                    Address = "tcp://127.0.0.1:1",
                    LogPath = Path.Combine(obs.Config.RunDir, "stdout.log"),
                });

            var runDir = Path.Combine(root, "gabriel-greeting-csharp", "uid-2");
            Assert.Contains("\"message\":\"service-log\"", File.ReadAllText(Path.Combine(runDir, "stdout.log")));
            Assert.Contains("\"type\":\"INSTANCE_READY\"", File.ReadAllText(Path.Combine(runDir, "events.jsonl")));
            using var meta = JsonDocument.Parse(File.ReadAllText(Path.Combine(runDir, "meta.json")));
            Assert.Equal("uid-2", meta.RootElement.GetProperty("uid").GetString());

            using var server = Serve.StartWithOptions(
                "tcp://127.0.0.1:0",
                new[] { Serve.Service(new ObservabilityGrpcService(obs)) },
                new Serve.ServeOptions { Describe = false });
            using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
            var client = new HolonObservability.HolonObservabilityClient(channel);

            var logs = new List<Holons.V1.LogEntry>();
            using (var call = client.Logs(new LogsRequest { MinLevel = Holons.V1.LogLevel.Info }))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    logs.Add(call.ResponseStream.Current);
            }
            Assert.Contains(logs, entry => entry.Message == "service-log");

            var metrics = await client.MetricsAsync(new MetricsRequest());
            Assert.Contains(metrics.Samples, sample => sample.Name == "csharp_requests_total");
            Assert.Null(metrics.SessionRollup);

            var events = new List<EventInfo>();
            using (var call = client.Events(new EventsRequest()))
            {
                while (await call.ResponseStream.MoveNext(CancellationToken.None))
                    events.Add(call.ResponseStream.Current);
            }
            Assert.Contains(events, ev => ev.Type == Holons.V1.EventType.InstanceReady);
        }
        finally
        {
            ObservabilityRegistry.Reset();
            Directory.Delete(root, recursive: true);
        }
    }

    private static string CreateTempDir()
    {
        var id = Guid.NewGuid().ToString("N")[..8];
        var root = Path.Combine(Path.GetTempPath(), $"hcs-obs-{id}");
        Directory.CreateDirectory(root);
        return root;
    }
}

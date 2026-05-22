using GabrielGreeting.Csharp._Internal;
using Greeting.V1;
using Grpc.Net.Client;
using Holons;
using Holons.V1;
using Obs = Holons.Observability;

namespace GabrielGreeting.Csharp.Tests.Internal;

public class ServerTests
{
    [Fact]
    public async Task RpcListLanguagesReturnsAllLanguages()
    {
        await using var server = await GreetingServer.StartAsync("tcp://127.0.0.1:0");
        using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
        var client = new GreetingService.GreetingServiceClient(channel);

        var response = await client.ListLanguagesAsync(new ListLanguagesRequest());

        Assert.Equal(56, response.Languages.Count);
        Assert.DoesNotContain(response.Languages, language =>
            string.IsNullOrWhiteSpace(language.Code) ||
            string.IsNullOrWhiteSpace(language.Name) ||
            string.IsNullOrWhiteSpace(language.Native));
    }

    [Fact]
    public async Task RpcSayHelloUsesRequestedLanguage()
    {
        await using var server = await GreetingServer.StartAsync("tcp://127.0.0.1:0");
        using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
        var client = new GreetingService.GreetingServiceClient(channel);

        var response = await client.SayHelloAsync(new SayHelloRequest
        {
            Name = "Bob",
            LangCode = "fr",
        });

        Assert.Equal("Bonjour Bob", response.Greeting);
        Assert.Equal("French", response.Language);
        Assert.Equal("fr", response.LangCode);
    }

    [Fact]
    public async Task RpcSayHelloUsesLocalizedDefaultName()
    {
        await using var server = await GreetingServer.StartAsync("tcp://127.0.0.1:0");
        using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
        var client = new GreetingService.GreetingServiceClient(channel);

        var response = await client.SayHelloAsync(new SayHelloRequest
        {
            LangCode = "fr",
        });

        Assert.Equal("Bonjour Marie", response.Greeting);
        Assert.Equal("fr", response.LangCode);
    }

    [Fact]
    public async Task RpcSayHelloFallsBackToEnglish()
    {
        await using var server = await GreetingServer.StartAsync("tcp://127.0.0.1:0");
        using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
        var client = new GreetingService.GreetingServiceClient(channel);

        var response = await client.SayHelloAsync(new SayHelloRequest
        {
            Name = "Bob",
            LangCode = "xx",
        });

        Assert.Equal("Hello Bob", response.Greeting);
        Assert.Equal("en", response.LangCode);
    }

    [Fact]
    public async Task SayHelloEmitsObservabilitySignals()
    {
        Obs.ObservabilityRegistry.Reset();
        var obs = Obs.ObservabilityRegistry.ConfigureFromEnv(
            new Obs.ObsConfig { Slug = "gabriel-greeting-csharp" },
            new Dictionary<string, string> { ["OP_OBS"] = "logs,metrics" });
        try
        {
            var service = new GreetingServiceImpl();
            var response = await service.SayHello(new SayHelloRequest
            {
                Name = " Bob ",
                LangCode = "en",
            }, null!);

            Assert.Equal("Hello Bob", response.Greeting);

            var counter = Assert.Single(obs.Registry!.Counters, sample =>
                sample.Name == "greeting_emitted_total" &&
                sample.Labels["lang_code"] == "en" &&
                sample.Labels["language"] == "English" &&
                sample.Labels["transport"] == "unknown");
            Assert.Equal(1, counter.Value);

            var entry = Assert.Single(obs.LogRing!.Drain(), entry =>
                Obs.Wire.AnyValueString(entry.Record.Body) == "Greeted Bob in English (en)");
            var attrs = Attrs(entry.Record);
            Assert.Equal(SeverityNumber.Info, entry.Record.SeverityNumber);
            Assert.Equal("gabriel-greeting-csharp", attrs[Obs.AttributeNames.HolonsSlug].StringValue);
            Assert.Equal("gabriel-greeting-csharp", attrs[Obs.AttributeNames.ServiceName].StringValue);
            Assert.False(string.IsNullOrWhiteSpace(attrs[Obs.AttributeNames.HolonsInstanceUid].StringValue));
            Assert.Equal(attrs[Obs.AttributeNames.HolonsInstanceUid].StringValue, attrs[Obs.AttributeNames.ServiceInstanceId].StringValue);
            Assert.Equal("", attrs[Obs.AttributeNames.HolonsSessionId].StringValue);
            Assert.Equal("en", attrs["lang_code"].StringValue);
            Assert.Equal("English", attrs["language"].StringValue);
            Assert.Equal("Bob", attrs["name"].StringValue);
            Assert.Equal("Hello Bob", attrs["greeting"].StringValue);
            Assert.Equal("unknown", attrs["transport"].StringValue);
            Assert.Equal(AnyValue.ValueOneofCase.IntValue, attrs["duration_ns"].ValueCase);
            Assert.True(attrs["duration_ns"].IntValue >= 0);
        }
        finally
        {
            Obs.ObservabilityRegistry.Reset();
        }
    }

    [Fact]
    public async Task SayHelloEmitsStdioTransportUnderServeFixture()
    {
        Obs.ObservabilityRegistry.Reset();
        try
        {
            await using var server = await GreetingServer.StartAsync("stdio://");
            var obs = Obs.ObservabilityRegistry.ConfigureFromEnv(
                new Obs.ObsConfig { Slug = "gabriel-greeting-csharp" },
                new Dictionary<string, string> { ["OP_OBS"] = "logs,metrics" });

            var service = new GreetingServiceImpl();
            await service.SayHello(new SayHelloRequest { Name = "Bob", LangCode = "en" }, null!);

            var entry = Assert.Single(obs.LogRing!.Drain(), entry =>
                Obs.Wire.AnyValueString(entry.Record.Body) == "Greeted Bob in English (en)");
            Assert.Equal("stdio", Attrs(entry.Record)["transport"].StringValue);
            await server.StopAsync();
            Assert.Equal("", Serve.CurrentTransport);
        }
        finally
        {
            Obs.ObservabilityRegistry.Reset();
        }
    }

    [Fact]
    public async Task RpcSayHelloStreamsTypedObservabilityWireRecord()
    {
        Obs.ObservabilityRegistry.Reset();
        var previousOpObs = Environment.GetEnvironmentVariable("OP_OBS");
        Environment.SetEnvironmentVariable("OP_OBS", "logs,metrics");
        Obs.ObservabilityRegistry.ConfigureFromEnv(
            new Obs.ObsConfig { Slug = "gabriel-greeting-csharp", InstanceUid = "wire-uid" },
            new Dictionary<string, string> { ["OP_OBS"] = "logs,metrics" });
        try
        {
            await using var server = await GreetingServer.StartAsync("tcp://127.0.0.1:0");
            using var channel = GrpcChannel.ForAddress($"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
            var greeting = new GreetingService.GreetingServiceClient(channel);
            await greeting.SayHelloAsync(new SayHelloRequest { Name = "Bob", LangCode = "en" });

            var observability = new HolonObservability.HolonObservabilityClient(channel);
            using var call = observability.Logs(new LogsRequest { MinSeverityNumber = SeverityNumber.Info });
            Assert.True(await call.ResponseStream.MoveNext(CancellationToken.None));
            var record = call.ResponseStream.Current;
            var attrs = Attrs(record);

            Assert.Equal("Greeted Bob in English (en)", Obs.Wire.AnyValueString(record.Body));
            Assert.Equal("gabriel-greeting-csharp", attrs[Obs.AttributeNames.HolonsSlug].StringValue);
            Assert.Equal("wire-uid", attrs[Obs.AttributeNames.HolonsInstanceUid].StringValue);
            Assert.Equal("tcp", attrs["transport"].StringValue);
            Assert.Equal(AnyValue.ValueOneofCase.IntValue, attrs["duration_ns"].ValueCase);
        }
        finally
        {
            Environment.SetEnvironmentVariable("OP_OBS", previousOpObs);
            Obs.ObservabilityRegistry.Reset();
        }
    }

    private static Dictionary<string, AnyValue> Attrs(LogRecord record) =>
        record.Attributes.ToDictionary(attr => attr.Key, attr => attr.Value);
}

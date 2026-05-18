using GabrielGreeting.Csharp._Internal;
using Greeting.V1;
using Grpc.Net.Client;
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
                entry.Message == "Greeted Bob in English (en)");
            Assert.Equal("en", entry.Fields["lang_code"]);
            Assert.Equal("English", entry.Fields["language"]);
            Assert.Equal("Bob", entry.Fields["name"]);
            Assert.Equal("Hello Bob", entry.Fields["greeting"]);
            Assert.Equal("unknown", entry.Fields["transport"]);
            Assert.True(long.TryParse(entry.Fields["duration_ns"], out var durationNs));
            Assert.True(durationNs >= 0);
        }
        finally
        {
            Obs.ObservabilityRegistry.Reset();
        }
    }
}

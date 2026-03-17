using GabrielGreeting.Csharp._Internal;
using Greeting.V1;
using Grpc.Net.Client;

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
            Name = "Alice",
            LangCode = "fr",
        });

        Assert.Equal("Bonjour Alice", response.Greeting);
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
}

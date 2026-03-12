using Greeting.V1;
using Grpc.Net.Client;
using Holons;

namespace GreetingDaemon.Csharp.Tests;

public class GreetingDaemonCsharpTests
{
    [Fact]
    public void GreetingTableExposes56Languages()
    {
        Assert.Equal(56, Greetings.All.Count);
    }

    [Fact]
    public void LookupFallsBackToEnglish()
    {
        Assert.Equal("en", Greetings.Lookup("??").Code);
    }

    [Fact]
    public async Task ServeRoundTripReturnsBonjourForFrench()
    {
        var recipeRoot = FindRecipeRoot();
        using var server = Serve.StartWithOptions(
            "tcp://127.0.0.1:0",
            [Serve.Service(new GreetingServiceImpl())],
            new Serve.ServeOptions
            {
                ProtoDir = Path.Combine(recipeRoot, "protos"),
                HolonYamlPath = Path.Combine(recipeRoot, "holon.yaml"),
            });

        using var channel = GrpcChannel.ForAddress(
            $"http://127.0.0.1:{server.PublicUri.Split(':').Last()}");
        var client = new GreetingService.GreetingServiceClient(channel);

        var languages = await client.ListLanguagesAsync(new ListLanguagesRequest());
        var greeting = await client.SayHelloAsync(new SayHelloRequest
        {
            LangCode = "fr",
            Name = "Ada",
        });

        Assert.Equal(56, languages.Languages.Count);
        Assert.Equal("Bonjour, Ada !", greeting.Greeting);
    }

    private static string FindRecipeRoot() =>
        Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", ".."));
}

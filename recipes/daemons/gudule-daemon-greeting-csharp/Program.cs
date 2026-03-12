using Grpc.Core;
using Greeting.V1;
using Holons;

namespace GreetingDaemon.Csharp;

public static class Program
{
    public static int Main(string[] args)
    {
        if (args.Length == 0)
            Usage();

        switch (args[0])
        {
            case "serve":
            {
                var recipeRoot = RecipeRoot.Find();
                var listenUri = Serve.ParseFlags(args.Skip(1).ToArray());
                Serve.RunWithOptions(
                    listenUri,
                    [Serve.Service(new GreetingServiceImpl())],
                    new Serve.ServeOptions
                    {
                        ProtoDir = Path.Combine(recipeRoot, "protos"),
                        HolonYamlPath = Path.Combine(recipeRoot, "holon.yaml"),
                    });
                return 0;
            }
            case "version":
                Console.WriteLine("gudule-daemon-greeting-csharp v0.4.2");
                return 0;
            default:
                Usage();
                return 1;
        }
    }

    private static void Usage()
    {
        Console.Error.WriteLine("usage: gudule-daemon-greeting-csharp <serve|version> [flags]");
        Environment.Exit(1);
    }
}

public sealed class GreetingServiceImpl : GreetingService.GreetingServiceBase
{
    public override Task<ListLanguagesResponse> ListLanguages(ListLanguagesRequest request, ServerCallContext context)
    {
        _ = request;
        _ = context;

        var response = new ListLanguagesResponse();
        response.Languages.AddRange(Greetings.All.Select(entry => new Language
        {
            Code = entry.Code,
            Name = entry.Name,
            Native = entry.Native,
        }));
        return Task.FromResult(response);
    }

    public override Task<SayHelloResponse> SayHello(SayHelloRequest request, ServerCallContext context)
    {
        _ = context;
        var entry = Greetings.Lookup(request.LangCode);
        var name = string.IsNullOrWhiteSpace(request.Name) ? "World" : request.Name.Trim();

        return Task.FromResult(new SayHelloResponse
        {
            Greeting = entry.Template.Replace("%s", name, StringComparison.Ordinal),
            Language = entry.Name,
            LangCode = entry.Code,
        });
    }
}

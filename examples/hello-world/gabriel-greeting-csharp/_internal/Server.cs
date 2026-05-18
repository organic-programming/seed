using System.Diagnostics;
using Greeting.V1;
using Gen;
using Grpc.Core;
using Holons;
using Obs = Holons.Observability;

namespace GabrielGreeting.Csharp._Internal;

public sealed class GreetingServiceImpl : GreetingService.GreetingServiceBase
{
    public override Task<ListLanguagesResponse> ListLanguages(ListLanguagesRequest request, ServerCallContext context)
    {
        _ = context;
        return Task.FromResult(Api.PublicApi.ListLanguages(request));
    }

    public override Task<SayHelloResponse> SayHello(SayHelloRequest request, ServerCallContext context)
    {
        _ = context;
        var start = Stopwatch.GetTimestamp();
        var response = Api.PublicApi.SayHello(request);
        var durationNs = ElapsedNanoseconds(start);
        // C# Serve does not yet expose a handler-visible current transport.
        var transport = "unknown";
        var name = ResolveGreetingName(request, response);

        EmitGreetingObservability(response, name, transport, durationNs);

        return Task.FromResult(response);
    }

    private static string ResolveGreetingName(SayHelloRequest request, SayHelloResponse response)
    {
        var trimmedName = request.Name.Trim();
        if (trimmedName.Length > 0)
            return trimmedName;

        return Greetings.Lookup(response.LangCode).DefaultName;
    }

    private static long ElapsedNanoseconds(long start)
    {
        var elapsedTicks = Stopwatch.GetTimestamp() - start;
        return (long)(elapsedTicks * (1_000_000_000.0 / Stopwatch.Frequency));
    }

    private static void EmitGreetingObservability(
        SayHelloResponse response,
        string name,
        string transport,
        long durationNs)
    {
        var obs = Obs.ObservabilityRegistry.Current();
        var message = $"Greeted {name} in {response.Language} ({response.LangCode})";
        obs.Logger("greeting").Info(
            message,
            new Dictionary<string, object?>
            {
                ["lang_code"] = response.LangCode,
                ["language"] = response.Language,
                ["name"] = name,
                ["greeting"] = response.Greeting,
                ["transport"] = transport,
                ["duration_ns"] = durationNs,
            });
        obs.Counter(
            "greeting_emitted_total",
            "Greetings emitted, partitioned by language and transport.",
            new Dictionary<string, string>
            {
                ["lang_code"] = response.LangCode,
                ["language"] = response.Language,
                ["transport"] = transport,
            })?.Inc();
    }
}

public static class GreetingServer
{
    static GreetingServer()
    {
        Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());
    }

    public static Task ListenAndServeAsync(string listenUri, bool reflect)
    {
        Serve.RunWithOptions(listenUri, CreateRegistrations(), new Serve.ServeOptions { Reflect = reflect });
        return Task.CompletedTask;
    }

    public static Task<Serve.RunningServer> StartAsync(string listenUri, bool reflect = false)
    {
        return Task.FromResult(Serve.StartWithOptions(listenUri, CreateRegistrations(), new Serve.ServeOptions { Reflect = reflect }));
    }

    private static Serve.GrpcServiceRegistration[] CreateRegistrations() =>
    [
        Serve.Service<GreetingServiceImpl>()
    ];
}

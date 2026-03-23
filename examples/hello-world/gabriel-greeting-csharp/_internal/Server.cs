using Greeting.V1;
using Gen;
using Grpc.Core;
using Holons;

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
        return Task.FromResult(Api.PublicApi.SayHello(request));
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

using Greeting.V1;
using Grpc.Core;
using Holons;
using Microsoft.AspNetCore.Builder;
using Microsoft.Extensions.DependencyInjection;

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
    public static Task ListenAndServeAsync(string listenUri)
    {
        Serve.Run(listenUri, CreateRegistrations());
        return Task.CompletedTask;
    }

    public static Task<Serve.RunningServer> StartAsync(string listenUri)
    {
        return Task.FromResult(Serve.StartWithOptions(listenUri, CreateRegistrations()));
    }

    private static Serve.GrpcServiceRegistration[] CreateRegistrations() =>
    [
        Serve.Service<GreetingServiceImpl>(),
        new(
            services => services.AddGrpcReflection(),
            app => app.MapGrpcReflectionService()),
    ];
}

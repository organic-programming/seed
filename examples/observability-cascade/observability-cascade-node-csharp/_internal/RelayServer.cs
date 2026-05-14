using CascadeNode.Csharp.Api;
using Gen;
using Grpc.Core;
using Holons;
using Relay.V1;

namespace CascadeNode.Csharp._Internal;

public sealed class RelayServiceImpl : RelayService.RelayServiceBase
{
    public override Task<TickResponse> Tick(TickRequest request, ServerCallContext context)
    {
        _ = context;
        return Task.FromResult(PublicApi.Tick(request));
    }
}

public static class RelayServer
{
    static RelayServer()
    {
        Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());
    }

    public static Task ListenAndServeAsync(string listenUri, bool reflect, IReadOnlyList<Serve.MemberRef> members)
    {
        Serve.RunWithOptions(
            NormalizeListenUri(listenUri),
            CreateRegistrations(),
            new Serve.ServeOptions
            {
                Reflect = reflect,
                Slug = "observability-cascade-node-csharp",
                MemberEndpoints = members,
            });
        return Task.CompletedTask;
    }

    private static Serve.GrpcServiceRegistration[] CreateRegistrations() =>
    [
        Serve.Service<RelayServiceImpl>(),
    ];

    private static string NormalizeListenUri(string listenUri) =>
        listenUri.StartsWith("tcp://:", StringComparison.Ordinal)
            ? $"tcp://0.0.0.0:{listenUri["tcp://:".Length..]}"
            : listenUri;
}

using Grpc.Core;
using Grpc.Net.Client;
using Holons.Observability;
using Relay.V1;

namespace Holons;

public static class Relay
{
    public static Serve.GrpcServiceRegistration Service(GrpcChannel? downstream = null) =>
        Serve.Service(new RelayServiceImpl(downstream));
}

internal sealed class RelayServiceImpl : RelayService.RelayServiceBase
{
    private readonly GrpcChannel? _downstream;
    private long _received;

    public RelayServiceImpl(GrpcChannel? downstream)
    {
        _downstream = downstream;
    }

    public override async Task<TickResponse> Tick(TickRequest request, ServerCallContext context)
    {
        var received = Interlocked.Increment(ref _received);
        var obs = ObservabilityRegistry.Current();
        var slug = ResponderSlug(obs);
        var uid = obs.Config.InstanceUid ?? "";

        obs.Logger("tick").Info(
            "tick received",
            new Dictionary<string, object?>
            {
                ["sender"] = request.Sender,
                ["note"] = request.Note,
                ["responder_slug"] = slug,
                ["responder_uid"] = uid,
            });
        obs.Counter(
            "cascade_ticks_total",
            "Ticks received by this cascade node.",
            new Dictionary<string, string> { ["responder_uid"] = uid })?.Inc();

        var response = new TickResponse
        {
            ResponderSlug = slug,
            ResponderInstanceUid = uid,
        };

        if (_downstream is not null)
        {
            var downstream = await new RelayService.RelayServiceClient(_downstream)
                .TickAsync(request, cancellationToken: context.CancellationToken)
                .ResponseAsync
                .ConfigureAwait(false);
            response.Hops.Add(downstream.Hops);
        }

        response.Hops.Add(new HopReceipt
        {
            Slug = slug,
            Uid = uid,
            Received = received,
        });
        return response;
    }

    private static string ResponderSlug(Observability.Observability obs)
    {
        var configured = obs.Config.Slug?.Trim() ?? "";
        if (configured.Length > 0)
            return configured;
        return Path.GetFileNameWithoutExtension(Environment.ProcessPath ?? "observability-cascade-csharp-node");
    }
}

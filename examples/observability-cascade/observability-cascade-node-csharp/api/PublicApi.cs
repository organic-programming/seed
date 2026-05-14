using Holons.Observability;
using Relay.V1;

namespace CascadeNode.Csharp.Api;

public static class PublicApi
{
    public static TickResponse Tick(TickRequest request)
    {
        var obs = ObservabilityRegistry.Current();
        var slug = ResponderSlug(obs);
        var uid = obs.Config.InstanceUid ?? string.Empty;
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
        return new TickResponse
        {
            ResponderSlug = slug,
            ResponderInstanceUid = uid,
        };
    }

    private static string ResponderSlug(Observability obs)
    {
        var configured = obs.Config.Slug?.Trim() ?? string.Empty;
        return string.IsNullOrEmpty(configured) ? "observability-cascade-node-csharp" : configured;
    }
}

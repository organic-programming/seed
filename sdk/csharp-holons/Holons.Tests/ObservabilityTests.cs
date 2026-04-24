using Holons.Observability;

namespace Holons.Tests;

public class ObservabilityTests
{
    [Fact]
    public void ParseOpObsDropsV2Tokens()
    {
        var all = new HashSet<Family> { Family.Logs, Family.Metrics, Family.Events, Family.Prom };
        Assert.True(all.SetEquals(Env.ParseOpObs("all,otel")));
        Assert.True(all.SetEquals(Env.ParseOpObs("all,sessions")));
    }

    [Fact]
    public void CheckEnvRejectsV2TokensAndOpSessions()
    {
        Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_OBS"] = "logs,otel" }));
        Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_OBS"] = "logs,sessions" }));
        var err = Assert.Throws<InvalidTokenException>(() =>
            Env.CheckEnv(new Dictionary<string, string> { ["OP_SESSIONS"] = "metrics" }));
        Assert.Equal("OP_SESSIONS", err.Variable);
    }
}

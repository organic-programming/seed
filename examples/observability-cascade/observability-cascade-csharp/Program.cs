using Gen;
using Grpc.Core;
using Holons;
using Holons.Observability;
using ObservabilityCascade.V1;
using Relay.V1;

Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());

static string CanonicalCommand(string raw) =>
    raw.Trim().ToLowerInvariant().Replace("-", "", StringComparison.Ordinal).Replace("_", "", StringComparison.Ordinal).Replace(" ", "", StringComparison.Ordinal);

var app = new CascadeApp();
if (args.Length > 0 && CanonicalCommand(args[0]) == "serve")
{
    app.Serve(args.Skip(1).ToArray());
    return;
}

var report = args.Contains("--multi-pattern")
    ? await app.RunMultiPattern(emit: true)
    : null;
var failed = 0;
if (report is not null)
{
    failed = report.TotalFail;
}
else
{
    var cascade = await app.RunReport(
        args.Contains("--live-stream") ? "live-stream" : "default",
        await app.OwnLanguageMembers(),
        live: args.Contains("--live-stream"),
        emit: true);
    failed = cascade.Fail;
}
if (failed > 0)
    Environment.ExitCode = 1;

sealed class CascadeApp
{
    private const int RunTicks = 3;
    private const string CsharpSlug = "observability-cascade-csharp-node";
    private const string GoSlug = "observability-cascade-go-node";

    public void Serve(string[] args)
    {
        var parsed = Holons.Serve.ParseOptions(args);
        Holons.Serve.RunWithOptions(
            parsed.ListenUri,
            [Holons.Serve.Service(new CascadeService(this))],
            new Holons.Serve.ServeOptions
            {
                Reflect = parsed.Reflect,
                Slug = "observability-cascade-csharp",
            });
    }

    public async Task<CascadeReport> RunReport(string name, IReadOnlyList<LanguageMember> members, bool live, bool emit)
    {
        EnsureCascadeObservability();
        var reportStart = DateTime.UtcNow;
        var report = new CascadeReport { Name = name };
        var timeout = live ? TimeSpan.FromSeconds(1) : TimeSpan.FromSeconds(3);
        var poll = live ? TimeSpan.FromMilliseconds(50) : TimeSpan.FromMilliseconds(100);

        if (emit)
        {
            Console.WriteLine($"=== observability-cascade-csharp {ModeSuffix(name)}===");
            Console.WriteLine();
        }

        for (var phaseIndex = 0; phaseIndex < Composite.TransportCoverageSequence.Length; phaseIndex++)
        {
            var phaseStart = DateTime.UtcNow;
            var transport = Composite.TransportCoverageSequence[phaseIndex];
            var from = phaseIndex == 0 ? transport : Composite.TransportCoverageSequence[phaseIndex - 1];
            var phase = new PhaseResult { Name = $"{phaseIndex + 1:00}-{from}\u2192{transport}" };
            if (emit)
                Console.WriteLine($"Phase {phaseIndex + 1}/{Composite.TransportCoverageSequence.Length}: {phase.Name}");

            Cascade? cascade = null;
            try
            {
                cascade = await Composite.BuildCascade(
                    new CascadeOptions
                    {
                        Transport = transport,
                        Members = members.Select(m => new ChildSpec(m.Slug, m.Binary)).ToArray(),
                        ExtraEnv = new Dictionary<string, string>
                        {
                            ["OP_OBS"] = "logs,events,metrics,prom",
                            ["OP_PROM_ADDR"] = "127.0.0.1:0",
                        },
                    });

                var previous = new Dictionary<string, long>();
                for (var tick = 1; tick <= RunTicks; tick++)
                {
                    var sender = $"{name}-phase-{phaseIndex + 1:00}-tick-{tick}";
                    var result = await RunTick(cascade, sender, transport, members, previous, timeout, poll, live);
                    if (result.Pass)
                    {
                        phase.Pass++;
                    }
                    else
                    {
                        phase.Fail++;
                        phase.Failures.Add(result.EvidenceLine(tick));
                    }
                    if (emit)
                    {
                        Console.WriteLine($"  Tick {tick}/{RunTicks}: {(result.Pass ? "PASS" : "FAIL")}");
                        if (!result.Pass)
                            Console.Error.WriteLine("    " + result.EvidenceLine(tick));
                    }
                }
            }
            catch (Exception error)
            {
                phase.Fail += RunTicks;
                for (var tick = 1; tick <= RunTicks; tick++)
                    phase.Failures.Add($"tick={tick} log=spawn event=spawn hops={CompactEvidence(error.Message)}");
            }
            finally
            {
                if (cascade is not null)
                    await cascade.StopAsync();
            }

            phase.ElapsedUs = ElapsedUs(phaseStart);
            AddPhase(report, phase);
            if (emit)
                PrintPhaseSummary(phase);
        }

        report.ElapsedUs = ElapsedUs(reportStart);
        if (emit)
            Console.WriteLine($"\nSummary: {report.Ticks} ticks, {report.Pass} PASS, {report.Fail} FAIL (total elapsed={ElapsedText(report.ElapsedUs)})");
        return report;
    }

    public async Task<MultiPatternReport> RunMultiPattern(bool emit)
    {
        var totalStart = DateTime.UtcNow;
        var patterns = await Patterns();
        var output = new MultiPatternReport();
        if (emit)
        {
            Console.WriteLine("=== observability-cascade-csharp --multi-pattern ===");
            Console.WriteLine();
        }

        for (var index = 0; index < patterns.Count; index++)
        {
            var pattern = patterns[index];
            if (emit)
                Console.WriteLine($"Pattern {index + 1}/{patterns.Count}: {pattern.Name}");
            var report = await RunReport(pattern.Name, pattern.Members, live: true, emit: emit);
            output.Patterns.Add(report);
            output.TotalPass += report.Pass;
            output.TotalFail += report.Fail;
            if (emit)
            {
                var status = report.Fail == 0 ? "PASS" : "FAIL";
                Console.WriteLine($"Pattern {pattern.Name}: {report.Pass}/{report.Ticks} {status} (elapsed={ElapsedText(report.ElapsedUs)})");
                Console.WriteLine();
            }
        }
        output.TotalElapsedUs = ElapsedUs(totalStart);
        if (emit)
            Console.WriteLine($"Summary: {output.TotalPass} PASS / {output.TotalFail} FAIL across {output.TotalPass + output.TotalFail} ticks (total elapsed={ElapsedText(output.TotalElapsedUs)})");
        return output;
    }

    public async Task<IReadOnlyList<LanguageMember>> OwnLanguageMembers()
    {
        var binary = await ResolveMemberBinary("csharp-node");
        return
        [
            new("csharp", CsharpSlug, binary),
            new("csharp", CsharpSlug, binary),
            new("csharp", CsharpSlug, binary),
        ];
    }

    private async Task<IReadOnlyList<NamedPattern>> Patterns()
    {
        var csharp = new LanguageMember("csharp", CsharpSlug, await ResolveMemberBinary("csharp-node"));
        var go = new LanguageMember("go", GoSlug, await ResolveMemberBinary("go-node"));
        var members = new Dictionary<string, LanguageMember> { ["csharp"] = csharp, ["go"] = go };
        var names = new[]
        {
            "csharp-csharp-csharp", "csharp-csharp-go", "csharp-go-csharp", "csharp-go-go",
            "go-csharp-csharp", "go-csharp-go", "go-go-csharp", "go-go-go",
        };
        return names
            .Select(name =>
            {
                var parts = name.Split('-');
                return new NamedPattern(name, [members[parts[0]], members[parts[1]], members[parts[2]]]);
            })
            .ToArray();
    }

    private static Task<string> ResolveMemberBinary(string id)
    {
        try { return Task.FromResult(Composite.Member(id)); }
        catch { return Task.FromResult(""); }
    }

    private static async Task<TickResult> RunTick(
        Cascade cascade,
        string sender,
        string note,
        IReadOnlyList<LanguageMember> members,
        Dictionary<string, long> previous,
        TimeSpan timeout,
        TimeSpan poll,
        bool live)
    {
        TickResponse response;
        try
        {
            using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(5));
            response = await new RelayService.RelayServiceClient(cascade.Top.Conn)
                .TickAsync(new TickRequest { Sender = sender, Note = note }, cancellationToken: cts.Token)
                .ResponseAsync;
        }
        catch (Exception error)
        {
            var outcome = new CheckOutcome(Evidence: CompactEvidence(error.Message));
            return new TickResult(false, outcome, outcome, outcome);
        }

        var hops = CheckHops(response.Hops, members, previous);
        if (!hops.Pass || response.Hops.Count == 0)
            return new TickResult(false, new CheckOutcome(Evidence: "skipped"), new CheckOutcome(Evidence: "skipped"), hops);

        var expected = response.Hops.Select(hop => new Hop(hop.Slug, hop.Uid)).ToArray();
        var leafUid = response.Hops[0].Uid;
        var log = await Composite.CheckRelayedLogAsync(new LogCheckOptions
        {
            Sender = sender,
            LeafUid = leafUid,
            ExpectedChain = expected,
            Timeout = timeout,
            PollInterval = poll,
            Live = live,
        });
        var ev = await Composite.CheckRelayedEventAsync(new EventCheckOptions
        {
            EventName = EventNames.InstanceReady,
            LeafUid = leafUid,
            ExpectedChain = expected,
            Timeout = timeout,
            PollInterval = poll,
            Live = live,
        });
        return new TickResult(hops.Pass && log.Pass && ev.Pass, log, ev, hops);
    }

    private static CheckOutcome CheckHops(IReadOnlyList<HopReceipt> hops, IReadOnlyList<LanguageMember> members, Dictionary<string, long> previous)
    {
        if (hops.Count != members.Count)
            return new CheckOutcome(Evidence: $"hops length {hops.Count} want {members.Count}");
        for (var index = 0; index < hops.Count; index++)
        {
            var hop = hops[index];
            var want = members[members.Count - 1 - index];
            if (hop.Slug != want.Slug)
                return new CheckOutcome(Evidence: $"hop {index} slug={hop.Slug} want {want.Slug}");
            if (string.IsNullOrWhiteSpace(hop.Uid))
                return new CheckOutcome(Evidence: $"hop {index} uid empty");
            previous.TryGetValue(hop.Uid, out var oldReceived);
            if (hop.Received <= oldReceived)
                return new CheckOutcome(Evidence: $"hop {index} received={hop.Received} previous={oldReceived}");
            previous[hop.Uid] = hop.Received;
        }
        return new CheckOutcome(true);
    }

    private static void EnsureCascadeObservability()
    {
        var current = ObservabilityRegistry.Current();
        if (current.Enabled(Family.Logs) && current.Enabled(Family.Events))
            return;
        Environment.SetEnvironmentVariable("OP_OBS", "logs,events,metrics,prom");
        Environment.SetEnvironmentVariable("OP_PROM_ADDR", "127.0.0.1:0");
        ObservabilityRegistry.FromEnv(new ObsConfig { Slug = "observability-cascade-csharp" });
    }

    private static void AddPhase(CascadeReport report, PhaseResult phase)
    {
        report.Phases.Add(phase);
        report.Pass += phase.Pass;
        report.Fail += phase.Fail;
        report.Ticks += phase.Pass + phase.Fail;
    }

    private static void PrintPhaseSummary(PhaseResult phase)
    {
        var status = phase.Fail == 0 ? "PASS" : "FAIL";
        Console.WriteLine($"Phase {phase.Name}: {phase.Pass}/{phase.Pass + phase.Fail} {status} (elapsed={ElapsedText(phase.ElapsedUs)})");
    }

    private static string EvidenceText(CheckOutcome outcome) =>
        outcome.Pass ? "ok" : CompactEvidence(outcome.Evidence);

    private static string CompactEvidence(string value)
    {
        var compact = string.Join(" ", (value ?? "").Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries));
        return compact.Length <= 240 ? compact : compact[..240] + "...";
    }

    private static long ElapsedUs(DateTime start) =>
        (long)(DateTime.UtcNow - start).TotalMicroseconds;

    private static string ElapsedText(long elapsedUs)
    {
        var elapsed = TimeSpan.FromMicroseconds(elapsedUs);
        if (elapsed < TimeSpan.FromSeconds(1))
            return $"{elapsed.TotalMilliseconds:0}ms";
        if (elapsed < TimeSpan.FromMinutes(1))
            return $"{elapsed.TotalSeconds:0.00}s";
        return $"{elapsed.TotalMinutes:0.0}m";
    }

    private static string ModeSuffix(string name) => name == "default" ? "" : "--" + name + " ";

    private sealed class CascadeService(CascadeApp app) : ObservabilityCascadeService.ObservabilityCascadeServiceBase
    {
        public override async Task<CascadeReport> RunDefault(RunRequest request, ServerCallContext context) =>
            await app.RunReport("default", await app.OwnLanguageMembers(), live: false, emit: false);

        public override async Task<CascadeReport> RunLiveStream(RunRequest request, ServerCallContext context) =>
            await app.RunReport("live-stream", await app.OwnLanguageMembers(), live: true, emit: false);

        public override async Task<MultiPatternReport> RunMultiPattern(RunRequest request, ServerCallContext context) =>
            await app.RunMultiPattern(emit: false);
    }

    public sealed record LanguageMember(string Lang, string Slug, string Binary);
    private sealed record NamedPattern(string Name, IReadOnlyList<LanguageMember> Members);
    private sealed record TickResult(bool Pass, CheckOutcome Log, CheckOutcome Event, CheckOutcome Hops)
    {
        public string EvidenceLine(int tick) =>
            $"tick={tick} log={EvidenceText(Log)} event={EvidenceText(Event)} hops={EvidenceText(Hops)}";
    }
}

using System.Collections.Concurrent;
using System.Diagnostics;
using System.Globalization;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using Gen;
using Grpc.Core;
using Grpc.Net.Client;
using Holons;
using Holons.V1;
using ObservabilityCascade.V1;
using Relay.V1;

string[] roleOrder = ["D", "C", "B", "A"];
string[] transports = ["tcp", "unix", "tcp", "unix"];
static string CanonicalCommand(string raw) =>
    raw.Trim().ToLowerInvariant().Replace("-", "", StringComparison.Ordinal).Replace("_", "", StringComparison.Ordinal).Replace(" ", "", StringComparison.Ordinal);

var app = new App(roleOrder, transports);
try
{
    if (args.Length > 0 && CanonicalCommand(args[0]) == "serve")
        app.ServeComposite(args.Skip(1).ToArray());
    else if (args.Contains("--multi-pattern"))
        await app.RunMultiPattern(true);
    else if (args.Contains("--live-stream"))
        await app.RunLiveStream(true);
    else
        await app.RunDefault(true);
}
catch (Exception error)
{
    Console.Error.WriteLine();
    Console.Error.WriteLine($"FAIL: {error.Message}");
    Environment.ExitCode = 1;
}

sealed class App
{
    private const int runPhases = 4;
    private const int runTicks = 3;
    private const string csharpSlug = "observability-cascade-node-csharp";
    private const string goSlug = "observability-cascade-node-go";
    private static readonly JsonSerializerOptions JsonOptions = new() { PropertyNameCaseInsensitive = true };
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(2) };
    private readonly string[] _roleOrder;
    private readonly string[] _transports;
    private readonly string _sourceRoot;
    private readonly string _examplesRoot;
    private readonly string _repoRoot;

    public App(string[] roleOrder, string[] transports)
    {
        _roleOrder = roleOrder;
        _transports = transports;
        _sourceRoot = FindSourceRoot();
        _examplesRoot = Directory.GetParent(_sourceRoot)!.FullName;
        _repoRoot = FindRepoRoot(_sourceRoot);
    }

    public void ServeComposite(string[] args)
    {
        Describe.UseStaticResponse(DescribeGenerated.StaticDescribeResponse());
        var parsed = Serve.ParseOptions(args);
        Serve.RunWithOptions(
            NormalizeListenUri(parsed.ListenUri),
            [Serve.Service(new ObservabilityCascadeServiceImpl(this))],
            new Serve.ServeOptions
            {
                Reflect = parsed.Reflect,
                Slug = "observability-cascade-csharp",
            });
    }

    private sealed class ObservabilityCascadeServiceImpl(App app) : ObservabilityCascadeService.ObservabilityCascadeServiceBase
    {
        public override async Task<CascadeReport> RunDefault(RunRequest request, ServerCallContext context)
        {
            _ = request;
            _ = context;
            return ToCascadeReport(await app.RunDefault(false));
        }

        public override async Task<CascadeReport> RunLiveStream(RunRequest request, ServerCallContext context)
        {
            _ = request;
            _ = context;
            return ToCascadeReport(await app.RunLiveStream(false));
        }

        public override async Task<MultiPatternReport> RunMultiPattern(RunRequest request, ServerCallContext context)
        {
            _ = request;
            _ = context;
            return ToMultiPatternReport(await app.RunMultiPattern(false));
        }
    }

    public async Task<CascadeReportData> RunDefault(bool emit)
    {
        var binary = await FindBinary(csharpSlug);
        var runRoot = Directory.CreateTempSubdirectory("observability-cascade-csharp-").FullName;
        Output(emit, "=== observability-cascade-csharp ===");
        Output(emit);
        var totalPass = 0;
        var totalFail = 0;
        var previous = "";
        for (var index = 0; index < _transports.Length; index++)
        {
            var phase = index + 1;
            var transport = _transports[index];
            Output(emit, previous.Length == 0
                ? $"Phase {phase}/{runPhases}: transport={transport}"
                : $"Phase {phase}/{runPhases}: transport={transport} (switching from {previous})");
            var started = NowMillis();
            Cascade cascade;
            try
            {
                cascade = await SpawnCascade(phase, transport, AllCsharpSpecs(binary), runRoot);
            }
            catch (Exception error)
            {
                totalFail += runTicks;
                Output(emit, $"  spawn FAIL: {error.Message}\n");
                previous = transport;
                continue;
            }
            Output(emit, $"  spawned 4 nodes in {Elapsed(started)}");
            var previousMetric = 0.0;
            for (var tick = 1; tick <= runTicks; tick++)
            {
                var tickStart = NowMillis();
                var outcome = await cascade.RunTick(tick, previousMetric);
                if (outcome.Metric.Pass)
                    previousMetric = outcome.MetricValue;
                var overall = outcome.Log.Pass && outcome.Event.Pass && outcome.Metric.Pass;
                if (overall) totalPass++; else totalFail++;
                Output(emit,
                    $"  Tick {tick}/{runTicks}: log {PassText(outcome.Log.Pass)}, event {PassText(outcome.Event.Pass)}, metric {PassText(outcome.Metric.Pass)} (overall {PassText(overall)} in {Elapsed(tickStart)})");
                if (emit)
                {
                    PrintFailureEvidence("log", outcome.Log);
                    PrintFailureEvidence("event", outcome.Event);
                    PrintFailureEvidence("metric", outcome.Metric);
                }
            }
            cascade.Stop();
            Output(emit);
            previous = transport;
        }
        Output(emit, $"Summary: {totalPass + totalFail} ticks, {totalPass} PASS, {totalFail} FAIL");
        if (totalFail > 0)
            throw new InvalidOperationException($"{totalFail} tick(s) failed");
        return new CascadeReportData(
            totalPass + totalFail,
            totalPass,
            totalFail,
            [new PhaseReportData("default", totalPass, totalFail)]);
    }

    public async Task<CascadeReportData> RunLiveStream(bool emit)
    {
        var binary = await FindBinary(csharpSlug);
        var runRoot = Directory.CreateTempSubdirectory("observability-cascade-csharp-live-").FullName;
        Output(emit, "=== observability-cascade-csharp --live-stream ===");
        Output(emit);
        Output(emit, "Setup: opening long-lived Follow:true streams on A");
        Output(emit, "       (initial transport: tcp)");
        Output(emit);
        var totalPass = 0;
        var totalFail = 0;
        Cascade? cascade = null;
        LiveStreams? streams = null;
        var specs = AllCsharpSpecs(binary);
        for (var index = 0; index < _transports.Length; index++)
        {
            var phase = index + 1;
            var transport = _transports[index];
            if (phase == 1)
            {
                Output(emit, $"Phase {phase}/{runPhases}: initial chain ({transport})");
            }
            else
            {
                Output(emit, $"Phase {phase}/{runPhases}: respawn on {transport}");
                var killStart = NowMillis();
                streams?.Stop();
                cascade?.Stop();
                Output(emit, $"  killed 4 nodes in {Elapsed(killStart)}");
            }
            var spawnStart = NowMillis();
            Cascade phaseCascade;
            try
            {
                phaseCascade = await SpawnCascade(phase, transport, specs, runRoot);
            }
            catch (Exception error)
            {
                totalFail += runTicks;
                Output(emit, $"  spawn FAIL: {error.Message}\n");
                streams = null;
                continue;
            }
            Output(emit, $"  spawned 4 nodes in {Elapsed(spawnStart)}");
            if (phase > 1)
                Output(emit, "  re-opening Follow:true streams on new A");
            string? streamError = null;
            try
            {
                streams = new LiveStreams(phaseCascade.Roles["A"].RelayAddress);
                streams.Start();
            }
            catch (Exception error)
            {
                streams = null;
                streamError = error.Message;
                Output(emit, $"  stream re-open failed: {error.Message}");
            }
            var previousMetric = 0.0;
            for (var tick = 1; tick <= runTicks; tick++)
            {
                var tickStart = NowMillis();
                var outcome = await phaseCascade.RunLiveTick(streams, streamError, tick, previousMetric);
                if (outcome.Metric.Pass)
                    previousMetric = outcome.MetricValue;
                var overall = outcome.Log.Pass && outcome.Event.Pass && outcome.Metric.Pass;
                if (overall) totalPass++; else totalFail++;
                Output(emit,
                    $"  Tick {tick}/{runTicks}: log {PassText(outcome.Log.Pass)}, event {PassText(outcome.Event.Pass)}, metric {PassText(outcome.Metric.Pass)} (overall {PassText(overall)} in {Elapsed(tickStart)})");
                if (emit)
                {
                    PrintFailureEvidence("log", outcome.Log);
                    PrintFailureEvidence("event", outcome.Event);
                    PrintFailureEvidence("metric", outcome.Metric);
                }
            }
            Output(emit);
            cascade = phaseCascade;
        }
        streams?.Stop();
        cascade?.Stop();
        Output(emit, $"Summary: {totalPass} PASS / {totalFail} FAIL across {totalPass + totalFail} ticks");
        if (totalFail > 0)
            throw new InvalidOperationException($"{totalFail} tick(s) failed");
        return new CascadeReportData(
            totalPass + totalFail,
            totalPass,
            totalFail,
            [new PhaseReportData("live-stream", totalPass, totalFail)]);
    }

    public async Task<MultiPatternReportData> RunMultiPattern(bool emit)
    {
        var csharpBinary = await FindBinary(csharpSlug);
        var goBinary = await FindBinary(goSlug);
        var patterns = new[]
        {
            new CascadePattern("csharp-csharp-csharp-csharp", AllCsharpSpecs(csharpBinary)),
            new CascadePattern("csharp-csharp-go-csharp", new Dictionary<string, RoleSpec>
            {
                ["A"] = new(csharpSlug, csharpBinary),
                ["B"] = new(csharpSlug, csharpBinary),
                ["C"] = new(goSlug, goBinary),
                ["D"] = new(csharpSlug, csharpBinary),
            }),
            new CascadePattern("csharp-csharp-go-go", new Dictionary<string, RoleSpec>
            {
                ["A"] = new(csharpSlug, csharpBinary),
                ["B"] = new(csharpSlug, csharpBinary),
                ["C"] = new(goSlug, goBinary),
                ["D"] = new(goSlug, goBinary),
            }),
        };
        var runRoot = Directory.CreateTempSubdirectory("observability-cascade-csharp-multi-").FullName;
        Output(emit, "=== observability-cascade-csharp (multi-pattern) ===");
        Output(emit);
        var totalPass = 0;
        var totalFail = 0;
        var patternReports = new List<CascadeReportData>();
        for (var patternIndex = 0; patternIndex < patterns.Length; patternIndex++)
        {
            var pattern = patterns[patternIndex];
            Output(emit, $"Pattern {patternIndex + 1}/{patterns.Length}: {pattern.Name}");
            var patternPass = 0;
            var patternFail = 0;
            var failures = new List<string>();
            for (var index = 0; index < _transports.Length; index++)
            {
                var phase = index + 1;
                var transport = _transports[index];
                var started = NowMillis();
                Cascade cascade;
                try
                {
                    cascade = await SpawnCascade(phase, transport, pattern.Roles, runRoot);
                }
                catch (Exception error)
                {
                    totalFail += runTicks;
                    patternFail += runTicks;
                    var failure = $"Phase {phase}/{runPhases} ({transport}): spawn FAIL ({error.Message})";
                    failures.Add(failure);
                    Output(emit, $"  {failure}");
                    continue;
                }
                string? streamError = null;
                LiveStreams? streams = null;
                try
                {
                    streams = new LiveStreams(cascade.Roles["A"].RelayAddress);
                    streams.Start();
                    var openedStreams = streams;
                    var ready = await WaitFor(TimeSpan.FromSeconds(5), () => Task.FromResult(cascade.CheckLiveEvent(openedStreams)), TimeSpan.FromMilliseconds(50));
                    if (!ready.Pass)
                        streamError = $"live relay readiness: {ready.Evidence}";
                }
                catch (Exception error)
                {
                    streamError = error.Message;
                }
                var previousMetric = 0.0;
                var results = new List<string>();
                var evidence = new List<string>();
                for (var tick = 1; tick <= runTicks; tick++)
                {
                    var sender = $"{pattern.Name}-phase-{phase}-tick-{tick}";
                    var outcome = await cascade.RunLiveTickWithSender(streams, streamError, sender, previousMetric);
                    if (outcome.Metric.Pass)
                        previousMetric = outcome.MetricValue;
                    var overall = outcome.Log.Pass && outcome.Event.Pass && outcome.Metric.Pass;
                    if (overall)
                    {
                        patternPass++;
                        totalPass++;
                        results.Add($"Tick {tick} PASS");
                    }
                    else
                    {
                        totalFail++;
                        patternFail++;
                        results.Add($"Tick {tick} FAIL ({FailureSummary(outcome)})");
                        var failure = $"Phase {phase}/{runPhases} ({transport}) tick {tick}: {CompactEvidence(outcome)}";
                        failures.Add(failure);
                        evidence.Add($"      Tick {tick} evidence: {CompactEvidence(outcome)}");
                    }
                }
                Output(emit, $"  Phase {phase}/{runPhases} ({transport}): {string.Join(", ", results)} (spawned in {Elapsed(started)})");
                if (emit)
                {
                    foreach (var line in evidence)
                        Console.WriteLine(line);
                }
                streams?.Stop();
                cascade.Stop();
            }
            Output(emit, $"  Subtotal: {patternPass}/12 PASS");
            Output(emit);
            patternReports.Add(new CascadeReportData(
                patternPass + patternFail,
                patternPass,
                patternFail,
                [new PhaseReportData(pattern.Name, patternPass, patternFail, failures)]));
        }
        Output(emit, $"Summary: {totalPass} PASS / {totalFail} FAIL across {totalPass + totalFail} ticks");
        if (totalFail > 0)
            throw new InvalidOperationException($"{totalFail} tick(s) failed");
        return new MultiPatternReportData(patternReports, totalPass, totalFail);
    }

    private async Task<Cascade> SpawnCascade(int phase, string transport, IReadOnlyDictionary<string, RoleSpec> specs, string runRoot)
    {
        var roles = new Dictionary<string, RoleRuntime>();
        foreach (var role in _roleOrder)
            roles[role] = NewRoleRuntime(phase, transport, role, specs[role]);
        foreach (var runtime in roles.Values)
        {
            Directory.CreateDirectory(runRoot);
            DeleteRecursively(Path.Combine(runRoot, runtime.Slug, runtime.Uid));
        }
        var cascade = new Cascade(phase, transport, runRoot, roles);
        foreach (var role in _roleOrder)
        {
            var runtime = roles[role];
            var child = ChildRole(role);
            if (child.Length > 0)
            {
                runtime.MemberAddress = roles[child].RelayAddress;
                runtime.MemberSlug = roles[child].Slug;
            }
            await StartRole(cascade, runtime);
        }
        await Task.Delay(150);
        return cascade;
    }

    private RoleRuntime NewRoleRuntime(int phase, string transport, string role, RoleSpec spec)
    {
        var uid = $"relay-p{phase:00}-{role.ToLowerInvariant()}";
        if (transport == "tcp")
            return new RoleRuntime(role, uid, spec.Slug, spec.BinaryPath, "tcp://127.0.0.1:0", "", "");
        if (transport == "unix")
        {
            var socket = $"/tmp/observability-cascade-csharp-p{phase}-{role.ToLowerInvariant()}-{Environment.ProcessId}.sock";
            if (File.Exists(socket))
                File.Delete(socket);
            var uri = $"unix://{socket}";
            return new RoleRuntime(role, uid, spec.Slug, spec.BinaryPath, uri, uri, uri);
        }
        throw new ArgumentException($"unknown transport {transport}");
    }

    private async Task StartRole(Cascade cascade, RoleRuntime runtime)
    {
        var psi = new ProcessStartInfo
        {
            FileName = runtime.BinaryPath,
            WorkingDirectory = _repoRoot,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        psi.ArgumentList.Add("serve");
        psi.ArgumentList.Add("--listen");
        psi.ArgumentList.Add(runtime.ListenUri);
        if (runtime.MemberAddress.Length > 0)
        {
            psi.ArgumentList.Add("--member");
            psi.ArgumentList.Add($"{runtime.MemberSlug}={runtime.MemberAddress}");
        }
        psi.Environment["OP_OBS"] = "logs,events,metrics,prom";
        psi.Environment["OP_RUN_DIR"] = cascade.RunRoot;
        psi.Environment["OP_INSTANCE_UID"] = runtime.Uid;
        psi.Environment["OP_ORGANISM_UID"] = cascade.Roles["A"].Uid;
        psi.Environment["OP_ORGANISM_SLUG"] = cascade.Roles["A"].Slug;
        psi.Environment["OP_PROM_ADDR"] = "127.0.0.1:0";

        var process = new Process { StartInfo = psi, EnableRaisingEvents = true };
        process.OutputDataReceived += (_, _) => { };
        process.ErrorDataReceived += (_, evt) =>
        {
            if (evt.Data is not null)
                runtime.AppendStderr(evt.Data);
        };
        if (!process.Start())
            throw new InvalidOperationException($"failed to start {runtime.Role}");
        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
        runtime.Process = process;

        try
        {
            var meta = await WaitMeta(cascade.RunRoot, runtime.Slug, runtime.Uid, TimeSpan.FromSeconds(15));
            runtime.MetricsAddr = meta.MetricsAddr;
            runtime.RelayAddress = meta.Address;
            runtime.Channel = Connect.ConnectTarget(
                runtime.RelayAddress,
                new Connect.ConnectOptions { Timeout = TimeSpan.FromSeconds(5), Transport = "tcp", Start = false });
            runtime.RelayClient = new RelayService.RelayServiceClient(runtime.Channel);
            await DialReady(runtime.Channel, TimeSpan.FromSeconds(10));
        }
        catch (Exception error)
        {
            var stderr = runtime.Stderr;
            throw new InvalidOperationException($"start {runtime.Role}: {(string.IsNullOrWhiteSpace(stderr) ? error.Message : stderr.Trim())}", error);
        }
    }

    private static async Task<MetaJson> WaitMeta(string runRoot, string slug, string uid, TimeSpan timeout)
    {
        var metaPath = Path.Combine(runRoot, slug, uid, "meta.json");
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                var meta = JsonSerializer.Deserialize<MetaJson>(await File.ReadAllTextAsync(metaPath), JsonOptions);
                if (meta?.Uid == uid && !string.IsNullOrWhiteSpace(meta.MetricsAddr))
                    return meta;
            }
            catch (Exception error)
            {
                last = error;
            }
            await Task.Delay(50);
        }
        throw new InvalidOperationException($"meta not ready for {slug}/{uid}: {last?.Message ?? "timeout"}");
    }

    private static async Task DialReady(GrpcChannel channel, TimeSpan timeout)
    {
        var client = new HolonMeta.HolonMetaClient(channel);
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                await client.DescribeAsync(new DescribeRequest(), deadline: DateTime.UtcNow.AddMilliseconds(500));
                return;
            }
            catch (Exception error)
            {
                last = error;
                await Task.Delay(50);
            }
        }
        throw new InvalidOperationException($"dial readiness failed: {last?.Message ?? "timeout"}");
    }

    private Dictionary<string, RoleSpec> AllCsharpSpecs(string binary) =>
        _roleOrder.ToDictionary(role => role, _ => new RoleSpec(csharpSlug, binary));

    private async Task<string> FindBinary(string slug)
    {
        var envName = $"OBSERVABILITY_CASCADE_NODE_{slug["observability-cascade-node-".Length..].ToUpperInvariant().Replace('-', '_')}_BIN";
        var fromEnv = Environment.GetEnvironmentVariable(envName);
        if (!string.IsNullOrWhiteSpace(fromEnv))
            return fromEnv.Trim();
        var roots = new List<string>();
        if (slug == csharpSlug)
        {
            var csharpNode = Path.Combine(_sourceRoot, "holons", "observability-cascade-node");
            roots.Add(Path.Combine(csharpNode, ".op", "build", "observability-cascade-node.holon", "bin"));
            roots.Add(Path.Combine(csharpNode, ".op", "build", "observability-cascade-node-csharp.holon", "bin"));
        }
        if (slug == goSlug)
        {
            var goNode = Path.Combine(_examplesRoot, "observability-cascade-go", "holons", "observability-cascade-node");
            roots.Add(Path.Combine(goNode, ".op", "build", "observability-cascade-node.holon", "bin"));
            roots.Add(Path.Combine(goNode, ".op", "build", "observability-cascade-node-go.holon", "bin"));
        }
        foreach (var root in roots)
        {
            var foundInRoot = FindExecutable(root, slug);
            if (foundInRoot is not null)
                return foundInRoot;
        }
        var psi = new ProcessStartInfo("op", $"--bin {slug}")
        {
            WorkingDirectory = _repoRoot,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
        };
        var process = Process.Start(psi) ?? throw new InvalidOperationException("failed to run op --bin");
        var output = (await process.StandardOutput.ReadToEndAsync()).Trim();
        await process.WaitForExitAsync();
        if (process.ExitCode == 0 && output.Length > 0)
            return output;
        var home = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".op", "bin", $"{slug}.holon", "bin");
        var found = FindExecutable(home, slug);
        if (found is not null)
            return found;
        throw new InvalidOperationException($"{slug} binary not found; run op build {slug} --install");
    }

    private static string? FindExecutable(string root, string name)
    {
        if (!Directory.Exists(root))
            return null;
        foreach (var path in Directory.EnumerateFileSystemEntries(root).OrderBy(path => path, StringComparer.Ordinal))
        {
            if (Directory.Exists(path))
            {
                var nested = FindExecutable(path, name);
                if (nested is not null)
                    return nested;
            }
            else if (Path.GetFileName(path) == name)
            {
                return path;
            }
        }
        return null;
    }

    private static string FindSourceRoot()
    {
        var fromEnv = Environment.GetEnvironmentVariable("OBSERVABILITY_CASCADE_CSHARP_SOURCE_ROOT");
        if (!string.IsNullOrWhiteSpace(fromEnv))
            return Path.GetFullPath(fromEnv.Trim());
        var current = new DirectoryInfo(Directory.GetCurrentDirectory());
        while (current is not null)
        {
            if (IsSourceRoot(current.FullName))
                return current.FullName;
            var nested = Path.Combine(current.FullName, "examples", "observability-cascade", "observability-cascade-csharp");
            if (IsSourceRoot(nested))
                return nested;
            current = current.Parent;
        }
        throw new InvalidOperationException("observability-cascade-csharp source root not found");
    }

    private static bool IsSourceRoot(string path) =>
        File.Exists(Path.Combine(path, "api", "v1", "holon.proto")) &&
        Directory.Exists(Path.Combine(path, "holons", "observability-cascade-node"));

    private static string FindRepoRoot(string start)
    {
        var current = new DirectoryInfo(start);
        while (current is not null)
        {
            if (Directory.Exists(Path.Combine(current.FullName, "sdk")) &&
                Directory.Exists(Path.Combine(current.FullName, "examples")))
                return current.FullName;
            current = current.Parent;
        }
        throw new InvalidOperationException("repository root not found");
    }

    private static string ChildRole(string role) => role switch
    {
        "A" => "B",
        "B" => "C",
        "C" => "D",
        _ => "",
    };

    private static async Task<List<LogEntry>> ReadLogs(GrpcChannel channel)
    {
        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Logs(new LogsRequest { MinLevel = LogLevel.Info }, deadline: DateTime.UtcNow.AddSeconds(2));
        var entries = new List<LogEntry>();
        while (await call.ResponseStream.MoveNext(CancellationToken.None))
            entries.Add(call.ResponseStream.Current);
        return entries;
    }

    private static async Task<List<EventInfo>> ReadEvents(GrpcChannel channel)
    {
        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Events(new EventsRequest(), deadline: DateTime.UtcNow.AddSeconds(2));
        var events = new List<EventInfo>();
        while (await call.ResponseStream.MoveNext(CancellationToken.None))
            events.Add(call.ResponseStream.Current);
        return events;
    }

    private static async Task<string> FetchMetrics(string addr) => await Http.GetStringAsync(addr);

    private static double? ParseCascadeTicks(string body, string uid)
    {
        var needle = $"responder_uid=\"{uid}\"";
        foreach (var line in body.Split('\n'))
        {
            if (!line.StartsWith("cascade_ticks_total{", StringComparison.Ordinal) || !line.Contains(needle, StringComparison.Ordinal))
                continue;
            var parts = line.Trim().Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length >= 2 && double.TryParse(parts[^1], NumberStyles.Float, CultureInfo.InvariantCulture, out var value))
                return value;
        }
        return null;
    }

    private static async Task<CheckResult> WaitFor(TimeSpan timeout, Func<Task<CheckResult>> fn, TimeSpan interval)
    {
        var deadline = DateTime.UtcNow + timeout;
        var last = new CheckResult(false, "");
        while (true)
        {
            last = await fn();
            if (last.Pass || DateTime.UtcNow > deadline)
                return last;
            await Task.Delay(interval);
        }
    }

    private static CascadeReport ToCascadeReport(CascadeReportData report)
    {
        var output = new CascadeReport
        {
            Ticks = report.Ticks,
            Pass = report.Pass,
            Fail = report.Fail,
        };
        output.Phases.AddRange(report.Phases.Select(phase =>
        {
            var item = new PhaseResult
            {
                Name = phase.Name,
                Pass = phase.Pass,
                Fail = phase.Fail,
            };
            item.Failures.AddRange(phase.Failures);
            return item;
        }));
        return output;
    }

    private static MultiPatternReport ToMultiPatternReport(MultiPatternReportData report)
    {
        var output = new MultiPatternReport
        {
            TotalPass = report.TotalPass,
            TotalFail = report.TotalFail,
        };
        output.Patterns.AddRange(report.Patterns.Select(ToCascadeReport));
        return output;
    }

    private static string NormalizeListenUri(string listenUri) =>
        listenUri.StartsWith("tcp://:", StringComparison.Ordinal)
            ? $"tcp://0.0.0.0:{listenUri["tcp://:".Length..]}"
            : listenUri;

    private static void Output(bool emit, string value = "")
    {
        if (emit)
            Console.WriteLine(value);
    }

    private static long NowMillis() => Stopwatch.GetTimestamp() * 1000 / Stopwatch.Frequency;

    private static string Elapsed(long startedMillis)
    {
        var elapsed = Math.Max(0, NowMillis() - startedMillis);
        return elapsed < 1000
            ? $"{elapsed}ms"
            : $"{(elapsed / 1000.0).ToString("F1", CultureInfo.InvariantCulture)}s";
    }

    private static string PassText(bool value) => value ? "PASS" : "FAIL";

    private static void PrintFailureEvidence(string family, CheckResult result)
    {
        if (!result.Pass)
            Console.WriteLine($"    {family} evidence: {(string.IsNullOrWhiteSpace(result.Evidence) ? "<empty>" : result.Evidence)}");
    }

    private static string FailureSummary(TickOutcome outcome)
    {
        var missing = new List<string>();
        if (!outcome.Log.Pass) missing.Add("log family");
        if (!outcome.Event.Pass) missing.Add("event family");
        if (!outcome.Metric.Pass) missing.Add("metric family");
        return missing.Count == 0 ? "unknown" : string.Join(", ", missing);
    }

    private static string CompactEvidence(TickOutcome outcome)
    {
        var parts = new List<string>();
        if (!outcome.Log.Pass) parts.Add($"log={outcome.Log.Evidence}");
        if (!outcome.Event.Pass) parts.Add($"event={outcome.Event.Evidence}");
        if (!outcome.Metric.Pass) parts.Add($"metric={outcome.Metric.Evidence}");
        return string.Join(" | ", parts);
    }

    private static void DeleteRecursively(string path)
    {
        if (!Directory.Exists(path) && !File.Exists(path))
            return;
        foreach (var entry in Directory.EnumerateFileSystemEntries(path, "*", SearchOption.AllDirectories).OrderByDescending(p => p.Length))
        {
            if (Directory.Exists(entry)) Directory.Delete(entry, false);
            else File.Delete(entry);
        }
        if (Directory.Exists(path))
            Directory.Delete(path, false);
    }
}

sealed record RoleSpec(string Slug, string BinaryPath);
sealed record CascadePattern(string Name, IReadOnlyDictionary<string, RoleSpec> Roles);
sealed record PhaseReportData(string Name, int Pass, int Fail, IReadOnlyList<string> Failures)
{
    public PhaseReportData(string name, int pass, int fail) : this(name, pass, fail, Array.Empty<string>())
    {
    }
}
sealed record CascadeReportData(int Ticks, int Pass, int Fail, IReadOnlyList<PhaseReportData> Phases);
sealed record MultiPatternReportData(IReadOnlyList<CascadeReportData> Patterns, int TotalPass, int TotalFail);
sealed record CheckResult(bool Pass, string Evidence);
sealed record TickOutcome(CheckResult Log, CheckResult Event, CheckResult Metric, double MetricValue);

sealed class RoleRuntime(string role, string uid, string slug, string binaryPath, string listenUri, string relayAddress, string clientTarget)
{
    private readonly object _stderrLock = new();
    private readonly StringBuilder _stderr = new();

    public string Role { get; } = role;
    public string Uid { get; } = uid;
    public string Slug { get; } = slug;
    public string BinaryPath { get; } = binaryPath;
    public string ListenUri { get; } = listenUri;
    public string RelayAddress { get; set; } = relayAddress;
    public string ClientTarget { get; } = clientTarget;
    public string MemberAddress { get; set; } = "";
    public string MemberSlug { get; set; } = "";
    public string MetricsAddr { get; set; } = "";
    public Process? Process { get; set; }
    public GrpcChannel? Channel { get; set; }
    public RelayService.RelayServiceClient? RelayClient { get; set; }
    public string Stderr { get { lock (_stderrLock) return _stderr.ToString(); } }
    public void AppendStderr(string line) { lock (_stderrLock) _stderr.AppendLine(line); }
}

sealed class Cascade(int phase, string transport, string runRoot, IReadOnlyDictionary<string, RoleRuntime> roles)
{
    public int Phase { get; } = phase;
    public string Transport { get; } = transport;
    public string RunRoot { get; } = runRoot;
    public IReadOnlyDictionary<string, RoleRuntime> Roles { get; } = roles;

    public Task<TickOutcome> RunTick(int tick, double previousMetric) =>
        RunTickWithSender($"phase-{Phase}-tick-{tick}", previousMetric);

    public async Task<TickOutcome> RunTickWithSender(string sender, double previousMetric)
    {
        var request = new TickRequest { Sender = sender, Note = Transport };
        try
        {
            await Roles["D"].RelayClient!.TickAsync(request, deadline: DateTime.UtcNow.AddSeconds(5));
        }
        catch (Exception error)
        {
            var failed = new CheckResult(false, error.Message);
            return new TickOutcome(failed, failed, failed, previousMetric);
        }
        var log = await AppWait(TimeSpan.FromSeconds(3), () => CheckLog(sender), TimeSpan.FromMilliseconds(100));
        var eventCheck = await AppWait(TimeSpan.FromSeconds(3), CheckEvent, TimeSpan.FromMilliseconds(100));
        var metricCheck = new MetricCheck(previousMetric);
        var metric = await AppWait(TimeSpan.FromSeconds(3), () => metricCheck.Check(this), TimeSpan.FromMilliseconds(100));
        return new TickOutcome(log, eventCheck, metric, metricCheck.Value);
    }

    public Task<TickOutcome> RunLiveTick(LiveStreams? streams, string? streamOpenError, int tick, double previousMetric) =>
        RunLiveTickWithSender(streams, streamOpenError, $"phase-{Phase}-tick-{tick}", previousMetric);

    public async Task<TickOutcome> RunLiveTickWithSender(LiveStreams? streams, string? streamOpenError, string sender, double previousMetric)
    {
        var request = new TickRequest { Sender = sender, Note = Transport };
        try
        {
            await Roles["D"].RelayClient!.TickAsync(request, deadline: DateTime.UtcNow.AddSeconds(5));
        }
        catch (Exception error)
        {
            var failed = new CheckResult(false, error.Message);
            return new TickOutcome(failed, failed, failed, previousMetric);
        }
        CheckResult log;
        CheckResult eventCheck;
        if (streamOpenError is null && streams is not null)
        {
            log = await AppWait(TimeSpan.FromSeconds(1), () => Task.FromResult(CheckLiveLog(streams, sender)), TimeSpan.FromMilliseconds(50));
            eventCheck = await AppWait(TimeSpan.FromSeconds(1), () => Task.FromResult(CheckLiveEvent(streams)), TimeSpan.FromMilliseconds(50));
        }
        else
        {
            var evidence = $"stream re-open failed: {streamOpenError ?? "streams not open"}";
            log = new CheckResult(false, evidence);
            eventCheck = new CheckResult(false, evidence);
        }
        var metricCheck = new MetricCheck(previousMetric);
        var metric = await AppWait(TimeSpan.FromSeconds(1), () => metricCheck.Check(this), TimeSpan.FromMilliseconds(50));
        return new TickOutcome(log, eventCheck, metric, metricCheck.Value);
    }

    public async Task<CheckResult> CheckLog(string sender)
    {
        var entries = await ReadLogs(Roles["A"].Channel!);
        foreach (var entry in entries)
        {
            if (entry.Message != "tick received") continue;
            if (!entry.Fields.TryGetValue("sender", out var foundSender) || foundSender != sender) continue;
            if (!entry.Fields.TryGetValue("responder_uid", out var responder) || responder != Roles["D"].Uid) continue;
            var err = CheckChain(entry.Chain);
            return err.Length == 0 ? new CheckResult(true, entry.ToString()) : new CheckResult(false, $"matching log has bad chain: {err} entry={entry}");
        }
        return new CheckResult(false, $"no relayed D tick log for sender={sender} in {entries.Count} A log entries");
    }

    public async Task<CheckResult> CheckEvent()
    {
        var events = await ReadEvents(Roles["A"].Channel!);
        foreach (var ev in events)
        {
            if (ev.Type != EventType.InstanceReady || ev.InstanceUid != Roles["D"].Uid) continue;
            var err = CheckChain(ev.Chain);
            return err.Length == 0 ? new CheckResult(true, ev.ToString()) : new CheckResult(false, $"matching event has bad chain: {err} event={ev}");
        }
        return new CheckResult(false, $"no relayed D INSTANCE_READY event in {events.Count} A events");
    }

    public CheckResult CheckLiveLog(LiveStreams streams, string sender)
    {
        var entries = streams.LogEntries();
        foreach (var entry in entries)
        {
            if (entry.Message != "tick received") continue;
            if (!entry.Fields.TryGetValue("sender", out var foundSender) || foundSender != sender) continue;
            if (!entry.Fields.TryGetValue("responder_uid", out var responder) || responder != Roles["D"].Uid) continue;
            var err = CheckChain(entry.Chain);
            return err.Length == 0 ? new CheckResult(true, entry.ToString()) : new CheckResult(false, $"matching live log has bad chain: {err} entry={entry}");
        }
        return new CheckResult(false, $"no live log found for sender={sender}; buffer={entries.Count} errors={string.Join(",", streams.Errors())}");
    }

    public CheckResult CheckLiveEvent(LiveStreams streams)
    {
        var events = streams.EventEntries();
        foreach (var ev in events)
        {
            if (ev.Type != EventType.InstanceReady || ev.InstanceUid != Roles["D"].Uid) continue;
            var err = CheckChain(ev.Chain);
            return err.Length == 0 ? new CheckResult(true, ev.ToString()) : new CheckResult(false, $"matching live event has bad chain: {err} event={ev}");
        }
        return new CheckResult(false, $"no live INSTANCE_READY event for D; buffer={events.Count} errors={string.Join(",", streams.Errors())}");
    }

    public async Task<CheckResult> CheckMetric(MetricCheck metricCheck)
    {
        var body = await FetchMetrics(Roles["D"].MetricsAddr);
        var value = ParseCascadeTicks(body, Roles["D"].Uid);
        if (value is null)
            return new CheckResult(false, body);
        metricCheck.Value = value.Value;
        if (value <= metricCheck.Previous)
            return new CheckResult(false, $"cascade_ticks_total={value} did not increase beyond {metricCheck.Previous}\n{body}");
        return new CheckResult(true, $"cascade_ticks_total={value}");
    }

    private string CheckChain(IEnumerable<ChainHop> chain)
    {
        var hops = chain.ToList();
        var expected = new[] { "D", "C", "B" };
        for (var index = 0; index < expected.Length; index++)
        {
            if (index >= hops.Count)
                return $"chain length {hops.Count} < 3";
            var role = expected[index];
            var want = Roles[role];
            var hop = hops[index];
            if (hop.Slug != want.Slug || hop.InstanceUid != want.Uid)
                return $"hop {index} = {hop.Slug}/{hop.InstanceUid}, want {want.Slug}/{want.Uid}";
        }
        return "";
    }

    public void Stop()
    {
        foreach (var role in new[] { "A", "B", "C", "D" })
        {
            var runtime = Roles[role];
            if (runtime.Channel is not null)
                Connect.Disconnect(runtime.Channel);
            if (runtime.Process is { HasExited: false })
                runtime.Process.Kill(entireProcessTree: true);
        }
    }

    private static Task<CheckResult> AppWait(TimeSpan timeout, Func<Task<CheckResult>> fn, TimeSpan interval) =>
        WaitForLocal(timeout, fn, interval);

    private static async Task<CheckResult> WaitForLocal(TimeSpan timeout, Func<Task<CheckResult>> fn, TimeSpan interval)
    {
        var deadline = DateTime.UtcNow + timeout;
        var last = new CheckResult(false, "");
        while (true)
        {
            last = await fn();
            if (last.Pass || DateTime.UtcNow > deadline)
                return last;
            await Task.Delay(interval);
        }
    }

    private static Task<List<LogEntry>> ReadLogs(GrpcChannel channel) => AppReflection.ReadLogs(channel);
    private static Task<List<EventInfo>> ReadEvents(GrpcChannel channel) => AppReflection.ReadEvents(channel);
    private static Task<string> FetchMetrics(string addr) => AppReflection.FetchMetrics(addr);
    private static double? ParseCascadeTicks(string body, string uid) => AppReflection.ParseCascadeTicks(body, uid);
}

sealed class MetricCheck(double previous)
{
    public double Previous { get; } = previous;
    public double Value { get; set; } = previous;
    public Task<CheckResult> Check(Cascade cascade) => cascade.CheckMetric(this);
}

sealed class LiveStreams(string address)
{
    private readonly ConcurrentBag<LogEntry> _logs = [];
    private readonly ConcurrentBag<EventInfo> _events = [];
    private readonly ConcurrentBag<string> _errors = [];
    private readonly CancellationTokenSource _cts = new();
    private GrpcChannel? _channel;

    public void Start()
    {
        _channel = Connect.ConnectTarget(address, new Connect.ConnectOptions { Timeout = TimeSpan.FromSeconds(5), Transport = "tcp", Start = false });
        _ = Task.Run(ReadLogStream);
        _ = Task.Run(ReadEventStream);
    }

    public void Stop()
    {
        _cts.Cancel();
        if (_channel is not null)
            Connect.Disconnect(_channel);
    }

    public List<LogEntry> LogEntries() => _logs.ToList();
    public List<EventInfo> EventEntries() => _events.ToList();
    public List<string> Errors() => _errors.ToList();

    private async Task ReadLogStream()
    {
        try
        {
            var client = new HolonObservability.HolonObservabilityClient(_channel);
            using var call = client.Logs(new LogsRequest { MinLevel = LogLevel.Info, Follow = true }, cancellationToken: _cts.Token);
            while (await call.ResponseStream.MoveNext(_cts.Token))
                _logs.Add(call.ResponseStream.Current);
        }
        catch (Exception error)
        {
            _errors.Add($"logs stream ended: {error.Message}");
        }
    }

    private async Task ReadEventStream()
    {
        try
        {
            var client = new HolonObservability.HolonObservabilityClient(_channel);
            using var call = client.Events(new EventsRequest { Follow = true }, cancellationToken: _cts.Token);
            while (await call.ResponseStream.MoveNext(_cts.Token))
                _events.Add(call.ResponseStream.Current);
        }
        catch (Exception error)
        {
            _errors.Add($"events stream ended: {error.Message}");
        }
    }
}

sealed class MetaJson
{
    [JsonPropertyName("uid")]
    public string Uid { get; set; } = "";
    [JsonPropertyName("address")]
    public string Address { get; set; } = "";
    [JsonPropertyName("metrics_addr")]
    public string MetricsAddr { get; set; } = "";
}

static class AppReflection
{
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(2) };

    public static async Task<List<LogEntry>> ReadLogs(GrpcChannel channel)
    {
        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Logs(new LogsRequest { MinLevel = LogLevel.Info }, deadline: DateTime.UtcNow.AddSeconds(2));
        var entries = new List<LogEntry>();
        while (await call.ResponseStream.MoveNext(CancellationToken.None))
            entries.Add(call.ResponseStream.Current);
        return entries;
    }

    public static async Task<List<EventInfo>> ReadEvents(GrpcChannel channel)
    {
        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Events(new EventsRequest(), deadline: DateTime.UtcNow.AddSeconds(2));
        var events = new List<EventInfo>();
        while (await call.ResponseStream.MoveNext(CancellationToken.None))
            events.Add(call.ResponseStream.Current);
        return events;
    }

    public static Task<string> FetchMetrics(string addr) => Http.GetStringAsync(addr);

    public static double? ParseCascadeTicks(string body, string uid)
    {
        var needle = $"responder_uid=\"{uid}\"";
        foreach (var line in body.Split('\n'))
        {
            if (!line.StartsWith("cascade_ticks_total{", StringComparison.Ordinal) || !line.Contains(needle, StringComparison.Ordinal))
                continue;
            var parts = line.Trim().Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length >= 2 && double.TryParse(parts[^1], NumberStyles.Float, CultureInfo.InvariantCulture, out var value))
                return value;
        }
        return null;
    }
}

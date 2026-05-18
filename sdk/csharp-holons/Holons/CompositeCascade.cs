using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using Grpc.Net.Client;
using Holons.Observability;
using Holons.V1;

namespace Holons;

public sealed record ChildSpec(string Slug, string Binary);

public sealed record DialOptions
{
    public bool? TransitiveObservability { get; init; }
}

public sealed record SpawnOptions
{
    public string Slug { get; init; } = "";
    public string BinaryPath { get; init; } = "";
    public string Transport { get; init; } = "stdio";
    public string InstanceUid { get; init; } = "";
    public IReadOnlyList<ChildSpec> DownstreamChain { get; init; } = Array.Empty<ChildSpec>();
    public IReadOnlyDictionary<string, string>? ExtraEnv { get; init; }
    public DialOptions? DialOptions { get; init; }
}

public sealed record CascadeOptions
{
    public string Transport { get; init; } = "stdio";
    public IReadOnlyList<ChildSpec> Members { get; init; } = Array.Empty<ChildSpec>();
    public IReadOnlyDictionary<string, string>? ExtraEnv { get; init; }
}

public sealed class SpawnedMember : IAsyncDisposable, IDisposable
{
    private readonly Process? _process;
    private readonly IDisposable? _resource;
    private readonly MemberRelay? _relay;
    private int _stopped;

    internal SpawnedMember(
        string slug,
        string uid,
        string listenUri,
        GrpcChannel conn,
        Process? process,
        IDisposable? resource,
        MemberRelay? relay)
    {
        Slug = slug;
        UID = uid;
        ListenURI = listenUri;
        Conn = conn;
        _process = process;
        _resource = resource;
        _relay = relay;
    }

    public string Slug { get; }
    public string UID { get; }
    public string ListenURI { get; }
    public GrpcChannel Conn { get; }

    public void Dispose() => StopAsync().GetAwaiter().GetResult();

    public async ValueTask DisposeAsync() => await StopAsync().ConfigureAwait(false);

    public async Task StopAsync(CancellationToken cancellationToken = default)
    {
        if (Interlocked.Exchange(ref _stopped, 1) != 0)
            return;

        if (_relay is not null)
            await _relay.DisposeAsync().ConfigureAwait(false);
        Conn.Dispose();
        try { _resource?.Dispose(); } catch { }

        if (_process is null || _process.HasExited)
            return;

        try
        {
            _process.Kill(entireProcessTree: true);
            var wait = _process.WaitForExitAsync(cancellationToken);
            var completed = await Task.WhenAny(wait, Task.Delay(TimeSpan.FromSeconds(3), cancellationToken)).ConfigureAwait(false);
            if (completed != wait && !_process.HasExited)
                _process.Kill(entireProcessTree: true);
        }
        catch
        {
            // Process teardown is best-effort after the gRPC channel and relays are closed.
        }
    }
}

public sealed class Cascade : IAsyncDisposable, IDisposable
{
    internal Cascade(SpawnedMember top) { Top = top; }
    public SpawnedMember Top { get; }
    public void Dispose() => StopAsync().GetAwaiter().GetResult();
    public async ValueTask DisposeAsync() => await StopAsync().ConfigureAwait(false);
    public Task StopAsync(CancellationToken cancellationToken = default) => Top.StopAsync(cancellationToken);
}

public sealed record CheckOutcome(bool Pass = false, string Evidence = "");

public sealed record LogCheckOptions
{
    public GrpcChannel? Conn { get; init; }
    public string Sender { get; init; } = "";
    public string LeafUid { get; init; } = "";
    public IReadOnlyList<Hop> ExpectedChain { get; init; } = Array.Empty<Hop>();
    public TimeSpan Timeout { get; init; } = default;
    public TimeSpan PollInterval { get; init; } = default;
    public bool Live { get; init; }
}

public sealed record EventCheckOptions
{
    public GrpcChannel? Conn { get; init; }
    public string EventName { get; init; } = Observability.EventNames.InstanceReady;
    public string LeafUid { get; init; } = "";
    public IReadOnlyList<Hop> ExpectedChain { get; init; } = Array.Empty<Hop>();
    public TimeSpan Timeout { get; init; } = default;
    public TimeSpan PollInterval { get; init; } = default;
    public bool Live { get; init; }
}

public static partial class Composite
{
    public static readonly string[] TransportCoverageSequence =
    [
        "stdio", "stdio", "tcp", "unix", "tcp", "tcp",
        "stdio", "unix", "unix", "stdio",
    ];

    public static DialOptions WithTransitiveObservability(bool enabled) =>
        new() { TransitiveObservability = enabled };

    public static async Task<SpawnedMember> SpawnMember(SpawnOptions opts, CancellationToken cancellationToken = default)
    {
        var slug = string.IsNullOrWhiteSpace(opts.Slug)
            ? Path.GetFileNameWithoutExtension(opts.BinaryPath)
            : opts.Slug.Trim();
        if (string.IsNullOrWhiteSpace(slug))
            throw new ArgumentException("spawn member: slug is required", nameof(opts));
        if (string.IsNullOrWhiteSpace(opts.BinaryPath))
            throw new ArgumentException($"spawn member {slug}: binary path is required", nameof(opts));

        var uid = string.IsNullOrWhiteSpace(opts.InstanceUid) ? NewInstanceUid() : opts.InstanceUid.Trim();
        var transport = string.IsNullOrWhiteSpace(opts.Transport) ? "stdio" : opts.Transport.Trim().ToLowerInvariant();
        var (listenUri, cleanupPath) = ListenUriForSpawn(transport, uid);
        if (!string.IsNullOrEmpty(cleanupPath))
        {
            try { File.Delete(cleanupPath); } catch { }
        }

        var args = new List<string> { "serve", "--listen", listenUri, "--transport", transport };
        foreach (var child in opts.DownstreamChain)
        {
            if (string.IsNullOrWhiteSpace(child.Slug) || string.IsNullOrWhiteSpace(child.Binary))
                throw new ArgumentException($"spawn member {slug}: downstream child requires slug and binary");
            args.Add("--child");
            args.Add($"{child.Slug}={child.Binary}");
        }

        var workDir = Path.GetDirectoryName(Path.GetFullPath(opts.BinaryPath));
        var env = BuildSpawnEnvironment(uid, opts.ExtraEnv);
        Process process;
        DialedChannel dialed;
        string publicUri;
        if (transport == "stdio")
        {
            var started = await ConnectionInternals.StartStdioHolonAsync(
                new LaunchTarget(opts.BinaryPath, Array.Empty<string>(), workDir),
                TimeSpan.FromSeconds(10),
                args,
                env).ConfigureAwait(false);
            process = started.Process;
            dialed = new DialedChannel(started.Channel, started.Resource);
            publicUri = started.PublicUri;
        }
        else
        {
            var startInfo = new ProcessStartInfo(opts.BinaryPath)
            {
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
            };
            if (!string.IsNullOrWhiteSpace(workDir))
                startInfo.WorkingDirectory = workDir;
            foreach (var arg in args)
                startInfo.ArgumentList.Add(arg);
            foreach (var (key, value) in env)
                startInfo.Environment[key] = value;
            process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
            var advertised = new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
            var stderr = new StringBuilder();
            process.OutputDataReceived += (_, e) =>
            {
                var uri = FirstUri(e.Data);
                if (uri.Length > 0)
                    advertised.TrySetResult(uri);
            };
            process.ErrorDataReceived += (_, e) =>
            {
                if (!string.IsNullOrEmpty(e.Data))
                    stderr.AppendLine(e.Data);
                var uri = FirstUri(e.Data);
                if (uri.Length > 0)
                    advertised.TrySetResult(uri);
            };
            process.Exited += (_, _) =>
                advertised.TrySetException(new IOException($"holon exited before advertising an address: {stderr.ToString().Trim()}"));

            if (!process.Start())
                throw new IOException($"failed to start {opts.BinaryPath}");
            process.BeginOutputReadLine();
            process.BeginErrorReadLine();

            var completed = await Task.WhenAny(advertised.Task, Task.Delay(TimeSpan.FromSeconds(10), cancellationToken)).ConfigureAwait(false);
            if (completed != advertised.Task)
            {
                ConnectionInternals.StopProcess(process);
                throw new IOException($"timed out waiting for {slug} startup");
            }
            publicUri = await advertised.Task.ConfigureAwait(false);
            dialed = await ConnectionInternals.DialReadyAsync(publicUri, TimeSpan.FromSeconds(10)).ConfigureAwait(false);
        }

        try
        {
            await DescribeReadyAsync(dialed.Channel, TimeSpan.FromSeconds(10), cancellationToken).ConfigureAwait(false);
            MemberRelay? relay = null;
            var transitive = opts.DialOptions?.TransitiveObservability ?? true;
            if (transitive)
            {
                var obs = EnsureCompositeObservability();
                relay = new MemberRelay(obs, slug, uid, dialed.Channel);
                relay.Start();
            }

            return new SpawnedMember(slug, uid, publicUri, dialed.Channel, process, dialed.Resource, relay);
        }
        catch
        {
            dialed.Channel.Dispose();
            try { dialed.Resource?.Dispose(); } catch { }
            ConnectionInternals.StopProcess(process);
            throw;
        }
    }

    public static async Task<Cascade> BuildCascade(CascadeOptions opts, CancellationToken cancellationToken = default)
    {
        if (opts.Members.Count == 0)
            throw new ArgumentException("build cascade: at least one member is required", nameof(opts));
        var top = opts.Members[0];
        var spawned = await SpawnMember(
            new SpawnOptions
            {
                Slug = top.Slug,
                BinaryPath = top.Binary,
                Transport = opts.Transport,
                DownstreamChain = opts.Members.Skip(1).ToArray(),
                ExtraEnv = opts.ExtraEnv,
            },
            cancellationToken).ConfigureAwait(false);
        return new Cascade(spawned);
    }

    public static async Task<GrpcChannel> Dial(string address, DialOptions? opts = null, CancellationToken cancellationToken = default)
    {
        var trimmed = (address ?? string.Empty).Trim();
        if (trimmed.StartsWith("stdio://", StringComparison.Ordinal))
            throw new ArgumentException("Composite.Dial does not support stdio addresses; spawn the child via SpawnMember instead", nameof(address));
        if (!trimmed.StartsWith("tcp://", StringComparison.Ordinal)
            && !trimmed.StartsWith("unix://", StringComparison.Ordinal)
            && !IsHostPortTarget(trimmed))
            throw new ArgumentException("dial address must be tcp://host:port, unix:///path, or host:port", nameof(address));

        var dialed = await ConnectionInternals.DialReadyAsync(trimmed, TimeSpan.FromSeconds(10)).ConfigureAwait(false);
        if (opts?.TransitiveObservability == true)
        {
            var desc = await DescribeReadyAsync(dialed.Channel, TimeSpan.FromSeconds(3), cancellationToken).ConfigureAwait(false);
            var identity = await ResolveRelayIdentity(dialed.Channel, desc, cancellationToken).ConfigureAwait(false);
            var relay = new MemberRelay(EnsureCompositeObservability(), identity.Slug, identity.InstanceUid, dialed.Channel);
            relay.Start();
        }
        return dialed.Channel;
    }

    public static (IReadOnlyList<ChildSpec> Children, string[] Remaining) ParseChildFlags(string[] args)
    {
        var children = new List<ChildSpec>();
        var remaining = new List<string>();
        for (var index = 0; index < args.Length; index++)
        {
            var arg = args[index];
            string? raw = null;
            if (arg == "--child")
            {
                if (++index >= args.Length)
                    throw new ArgumentException("--child requires <slug>=<binary>");
                raw = args[index];
            }
            else if (arg.StartsWith("--child=", StringComparison.Ordinal))
            {
                raw = arg["--child=".Length..];
            }

            if (raw is null)
            {
                remaining.Add(arg);
                continue;
            }

            var split = raw.IndexOf('=', StringComparison.Ordinal);
            if (split <= 0 || split == raw.Length - 1)
                throw new ArgumentException("--child must be formatted as <slug>=<binary>");
            children.Add(new ChildSpec(raw[..split].Trim(), raw[(split + 1)..].Trim()));
        }

        return (children, remaining.ToArray());
    }

    public static CheckOutcome CheckRelayedLog(LogCheckOptions opts) =>
        CheckRelayedLogAsync(opts).GetAwaiter().GetResult();

    public static async Task<CheckOutcome> CheckRelayedLogAsync(LogCheckOptions opts, CancellationToken cancellationToken = default)
    {
        var timeout = opts.Timeout <= TimeSpan.Zero ? TimeSpan.FromSeconds(3) : opts.Timeout;
        var interval = opts.PollInterval <= TimeSpan.Zero ? TimeSpan.FromMilliseconds(100) : opts.PollInterval;
        var deadline = DateTime.UtcNow + timeout;
        CheckOutcome last = new(Evidence: "not checked");
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                last = MatchRelayedLog(await ReadLogs(opts.Conn, cancellationToken).ConfigureAwait(false), opts);
                if (last.Pass)
                    return last;
            }
            catch (Exception error)
            {
                last = new CheckOutcome(Evidence: CompactEvidence(error.Message));
            }
            await Task.Delay(interval, cancellationToken).ConfigureAwait(false);
        }
        return last;
    }

    public static CheckOutcome CheckRelayedEvent(EventCheckOptions opts) =>
        CheckRelayedEventAsync(opts).GetAwaiter().GetResult();

    public static async Task<CheckOutcome> CheckRelayedEventAsync(EventCheckOptions opts, CancellationToken cancellationToken = default)
    {
        var timeout = opts.Timeout <= TimeSpan.Zero ? TimeSpan.FromSeconds(3) : opts.Timeout;
        var interval = opts.PollInterval <= TimeSpan.Zero ? TimeSpan.FromMilliseconds(100) : opts.PollInterval;
        var deadline = DateTime.UtcNow + timeout;
        CheckOutcome last = new(Evidence: "not checked");
        while (DateTime.UtcNow < deadline)
        {
            try
            {
                last = MatchRelayedEvent(await ReadEvents(opts.Conn, cancellationToken).ConfigureAwait(false), opts);
                if (last.Pass)
                    return last;
            }
            catch (Exception error)
            {
                last = new CheckOutcome(Evidence: CompactEvidence(error.Message));
            }
            await Task.Delay(interval, cancellationToken).ConfigureAwait(false);
        }
        return last;
    }

    private static async Task<IReadOnlyList<Observability.LogRecord>> ReadLogs(GrpcChannel? channel, CancellationToken ct)
    {
        if (channel is null)
            return ObservabilityRegistry.Current().LogRing?.Drain() ?? throw new InvalidOperationException("logs family is not enabled");

        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(2));
        using var call = client.Logs(new LogsRequest { MinSeverityNumber = SeverityNumber.Info }, cancellationToken: timeout.Token);
        var entries = new List<Observability.LogRecord>();
        while (await call.ResponseStream.MoveNext(timeout.Token).ConfigureAwait(false))
            entries.Add(ObservabilityGrpcService.FromProtoLogRecord(call.ResponseStream.Current));
        return entries;
    }

    private static async Task<IReadOnlyList<Observability.LogRecord>> ReadEvents(GrpcChannel? channel, CancellationToken ct)
    {
        if (channel is null)
            return ObservabilityRegistry.Current().EventBus?.Drain() ?? throw new InvalidOperationException("events family is not enabled");

        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(2));
        using var call = client.Events(new EventsRequest(), cancellationToken: timeout.Token);
        var events = new List<Observability.LogRecord>();
        while (await call.ResponseStream.MoveNext(timeout.Token).ConfigureAwait(false))
            events.Add(ObservabilityGrpcService.FromProtoLogRecord(call.ResponseStream.Current));
        return events;
    }

    private static CheckOutcome MatchRelayedLog(IReadOnlyList<Observability.LogRecord> entries, LogCheckOptions opts)
    {
        foreach (var entry in entries)
        {
            if (Observability.Wire.AnyValueString(entry.Record.Body) != "tick received")
                continue;
            var attrs = AttrMap(entry.Record.Attributes);
            if (!attrs.TryGetValue("sender", out var sender) || Observability.Wire.AnyValueString(sender) != opts.Sender)
                continue;
            if (!attrs.TryGetValue("responder_uid", out var responderUid) || Observability.Wire.AnyValueString(responderUid) != opts.LeafUid)
                continue;
            var evidence = CompareChain(entry.Record.Chain, opts.ExpectedChain);
            return evidence.Length == 0
                ? new CheckOutcome(true)
                : new CheckOutcome(Evidence: CompactEvidence("matching log bad chain: " + evidence));
        }
        return new CheckOutcome(Evidence: CompactEvidence($"no relayed tick log sender={opts.Sender} leaf_uid={opts.LeafUid} entries={entries.Count}"));
    }

    private static CheckOutcome MatchRelayedEvent(IReadOnlyList<Observability.LogRecord> events, EventCheckOptions opts)
    {
        foreach (var ev in events)
        {
            if (ev.Record.EventName != opts.EventName ||
                Observability.Wire.AttributeString(ev.Record.Attributes, Observability.AttributeNames.HolonsInstanceUid) != opts.LeafUid)
                continue;
            var evidence = CompareChain(ev.Record.Chain, opts.ExpectedChain);
            return evidence.Length == 0
                ? new CheckOutcome(true)
                : new CheckOutcome(Evidence: CompactEvidence("matching event bad chain: " + evidence));
        }
        return new CheckOutcome(Evidence: CompactEvidence($"no relayed {opts.EventName} event leaf_uid={opts.LeafUid} events={events.Count}"));
    }

    private static string CompareChain(IReadOnlyList<string> got, IReadOnlyList<Hop> want)
    {
        if (got.Count != want.Count)
            return $"chain length {got.Count} want {want.Count}";
        for (var i = 0; i < want.Count; i++)
        {
            if (got[i] != want[i].Slug)
                return $"hop {i}={got[i]} want {want[i].Slug}";
        }
        return "";
    }

    private static Dictionary<string, AnyValue> AttrMap(IEnumerable<KeyValue> attrs) =>
        attrs.ToDictionary(attr => attr.Key, attr => attr.Value, StringComparer.Ordinal);

    private static async Task<DescribeResponse> DescribeReadyAsync(GrpcChannel channel, TimeSpan timeout, CancellationToken cancellationToken)
    {
        var client = new HolonMeta.HolonMetaClient(channel);
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;
        while (DateTime.UtcNow < deadline)
        {
            cancellationToken.ThrowIfCancellationRequested();
            try
            {
                return await client.DescribeAsync(new DescribeRequest(), deadline: DateTime.UtcNow.AddMilliseconds(500), cancellationToken: cancellationToken);
            }
            catch (Exception error)
            {
                last = error;
                await Task.Delay(50, cancellationToken).ConfigureAwait(false);
            }
        }
        throw new IOException("timed out waiting for HolonMeta.Describe", last);
    }

    private static async Task<MemberIdentity> ResolveRelayIdentity(GrpcChannel channel, DescribeResponse desc, CancellationToken ct)
    {
        var fallbackSlug = SlugFromDescribe(desc);
        var client = new HolonObservability.HolonObservabilityClient(channel);
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(1));
        try
        {
            using var events = client.Events(new EventsRequest(), cancellationToken: timeout.Token);
            while (await events.ResponseStream.MoveNext(timeout.Token).ConfigureAwait(false))
            {
                var ev = events.ResponseStream.Current;
                var instanceUid = Observability.Wire.AttributeString(ev.Attributes, Observability.AttributeNames.HolonsInstanceUid);
                if (ev.Chain.Count == 0 && !string.IsNullOrWhiteSpace(instanceUid))
                {
                    var slug = Observability.Wire.AttributeString(ev.Attributes, Observability.AttributeNames.HolonsSlug);
                    return new MemberIdentity(string.IsNullOrWhiteSpace(slug) ? fallbackSlug : slug, instanceUid);
                }
            }
        }
        catch { }

        throw new InvalidOperationException("resolve relay identity: peer did not expose a local event with instance_uid");
    }

    private static Observability.Observability EnsureCompositeObservability()
    {
        var current = ObservabilityRegistry.Current();
        if (current.Enabled(Observability.Family.Logs) && current.Enabled(Observability.Family.Events))
            return current;
        Environment.SetEnvironmentVariable("OP_OBS", "logs,events,metrics,prom");
        Environment.SetEnvironmentVariable("OP_PROM_ADDR", Environment.GetEnvironmentVariable("OP_PROM_ADDR") ?? "127.0.0.1:0");
        return ObservabilityRegistry.FromEnv(new ObsConfig { Slug = "observability-cascade-csharp" });
    }

    private static Dictionary<string, string> BuildSpawnEnvironment(string uid, IReadOnlyDictionary<string, string>? extra)
    {
        var env = new Dictionary<string, string>
        {
            ["OP_INSTANCE_UID"] = uid,
            ["OP_OBS"] = ReadEnv("OP_OBS", "logs,events,metrics,prom"),
            ["OP_PROM_ADDR"] = ReadEnv("OP_PROM_ADDR", "127.0.0.1:0"),
            ["OP_RUN_DIR"] = ReadEnv("OP_RUN_DIR", DefaultRunRoot()),
            ["HOLONS_PARENT_PID"] = Environment.ProcessId.ToString(),
        };
        if (extra is not null)
        {
            foreach (var (key, value) in extra)
                env[key] = value;
        }
        return env;
    }

    private static string ReadEnv(string name, string fallback) =>
        string.IsNullOrWhiteSpace(Environment.GetEnvironmentVariable(name))
            ? fallback
            : Environment.GetEnvironmentVariable(name)!;

    private static string DefaultRunRoot()
    {
        var opPath = Environment.GetEnvironmentVariable("OPPATH");
        if (!string.IsNullOrWhiteSpace(opPath))
            return Path.Combine(opPath, "run");
        var home = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        return string.IsNullOrWhiteSpace(home)
            ? Path.Combine(Path.GetTempPath(), ".op", "run")
            : Path.Combine(home, ".op", "run");
    }

    private static (string Uri, string CleanupPath) ListenUriForSpawn(string transport, string uid) =>
        transport switch
        {
            "stdio" => ("stdio://", ""),
            "tcp" => ("tcp://127.0.0.1:0", ""),
            "unix" => ($"unix://{Path.Combine(Path.GetTempPath(), "op-" + CleanSocketToken(uid) + ".sock")}",
                Path.Combine(Path.GetTempPath(), "op-" + CleanSocketToken(uid) + ".sock")),
            _ => throw new ArgumentException($"unsupported transport \"{transport}\""),
        };

    private static string NewInstanceUid()
    {
        Span<byte> bytes = stackalloc byte[12];
        RandomNumberGenerator.Fill(bytes);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }

    private static string CleanSocketToken(string value)
    {
        var clean = new string((value ?? string.Empty).Select(ch => char.IsLetterOrDigit(ch) ? ch : '-').ToArray()).Trim('-');
        if (clean.Length > 24)
            clean = clean[..24];
        return clean.Length == 0 ? NewInstanceUid()[..12] : clean;
    }

    private static string FirstUri(string? line)
    {
        if (string.IsNullOrWhiteSpace(line))
            return "";
        foreach (var field in line.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries))
        {
            var trimmed = field.Trim().Trim('"', '\'', '(', ')', '[', ']', '{', '}', '.', ',');
            if (trimmed.StartsWith("tcp://", StringComparison.Ordinal)
                || trimmed.StartsWith("unix://", StringComparison.Ordinal)
                || trimmed.StartsWith("stdio://", StringComparison.Ordinal))
                return trimmed;
        }
        return "";
    }

    private static void EnsureStdioStartup(Process process, SpawnStdioBridge bridge, TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromMilliseconds(Math.Min(timeout.TotalMilliseconds, 250));
        while (DateTime.UtcNow < deadline)
        {
            if (process.HasExited)
                throw new IOException($"holon exited before stdio startup: {bridge.StderrText}");
            Thread.Sleep(10);
        }
    }

    private static string SlugFromDescribe(DescribeResponse desc)
    {
        var identity = desc.Manifest?.Identity;
        var alias = identity?.Aliases.FirstOrDefault(value => !string.IsNullOrWhiteSpace(value));
        if (!string.IsNullOrWhiteSpace(alias))
            return alias;
        return (identity?.GivenName + "-" + identity?.FamilyName)
            .Trim('-')
            .ToLowerInvariant()
            .Replace(" ", "-", StringComparison.Ordinal);
    }

    private static bool IsHostPortTarget(string target)
    {
        var index = target.LastIndexOf(':');
        return index > 0
            && index < target.Length - 1
            && !target.Contains("://", StringComparison.Ordinal)
            && int.TryParse(target[(index + 1)..], out _);
    }

    private static string CompactEvidence(string value)
    {
        var compact = string.Join(" ", (value ?? string.Empty).Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries));
        return compact.Length <= 240 ? compact : compact[..240] + "...";
    }

    private static IDisposable? CombineResources(IDisposable? first, IDisposable? second)
    {
        if (first is null)
            return second;
        if (second is null)
            return first;
        return new CompositeDisposable(first, second);
    }

    private sealed class CompositeDisposable(IDisposable first, IDisposable second) : IDisposable
    {
        public void Dispose()
        {
            try { first.Dispose(); } finally { second.Dispose(); }
        }
    }

    private sealed class SpawnStdioBridge : IDisposable
    {
        private readonly Process _process;
        private readonly TcpListener _listener;
        private readonly StringBuilder _stderr = new();
        private readonly Thread _acceptThread;
        private volatile bool _closed;
        private TcpClient? _client;

        public SpawnStdioBridge(Process process)
        {
            _process = process;
            _listener = new TcpListener(IPAddress.Loopback, 0);
            _listener.Start();
            StartDrainThread(process.StandardError.BaseStream, _stderr);
            _acceptThread = new Thread(AcceptLoop) { IsBackground = true, Name = "holons-spawn-stdio-accept" };
            _acceptThread.Start();
        }

        public string Uri => $"tcp://127.0.0.1:{((_listener.LocalEndpoint as IPEndPoint)?.Port ?? 0)}";
        public string StderrText { get { lock (_stderr) return _stderr.ToString().Trim(); } }

        public void Dispose()
        {
            _closed = true;
            try { _listener.Stop(); } catch { }
            try { _client?.Close(); } catch { }
            TryDispose(_process.StandardInput);
            TryDispose(_process.StandardOutput);
            TryDispose(_process.StandardError);
            try { _acceptThread.Join(200); } catch { }
        }

        private void AcceptLoop()
        {
            try
            {
                while (!_closed)
                {
                    var accepted = _listener.AcceptTcpClient();
                    if (_closed)
                    {
                        accepted.Close();
                        return;
                    }

                    _client = accepted;
                    var network = accepted.GetStream();
                    var upstream = StartPump(network, _process.StandardInput.BaseStream, false, "holons-spawn-stdio-up");
                    var downstream = StartPump(_process.StandardOutput.BaseStream, network, true, "holons-spawn-stdio-down");
                    upstream.Join();
                    downstream.Join();
                    try { accepted.Close(); } catch { }
                    _client = null;
                }
            }
            catch
            {
                // Closed during shutdown.
            }
        }

        private static Thread StartPump(Stream input, Stream output, bool closeOutput, string name)
        {
            var thread = new Thread(() =>
            {
                var buffer = new byte[16 * 1024];
                try
                {
                    while (true)
                    {
                        var read = input.Read(buffer, 0, buffer.Length);
                        if (read <= 0)
                            break;
                        output.Write(buffer, 0, read);
                        output.Flush();
                    }
                }
                catch { }
                finally
                {
                    if (closeOutput)
                        TryDispose(output);
                }
            })
            {
                IsBackground = true,
                Name = name,
            };
            thread.Start();
            return thread;
        }

        private static void StartDrainThread(Stream stream, StringBuilder capture)
        {
            var thread = new Thread(() =>
            {
                var buffer = new byte[4096];
                try
                {
                    while (true)
                    {
                        var read = stream.Read(buffer, 0, buffer.Length);
                        if (read <= 0)
                            break;
                        lock (capture)
                            capture.Append(Encoding.UTF8.GetString(buffer, 0, read));
                    }
                }
                catch { }
            })
            {
                IsBackground = true,
                Name = "holons-spawn-stdio-stderr",
            };
            thread.Start();
        }

        private static void TryDispose(IDisposable? disposable)
        {
            try { disposable?.Dispose(); } catch { }
        }
    }
}

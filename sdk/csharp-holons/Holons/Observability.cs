// C# reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability.

using Google.Protobuf.WellKnownTypes;
using Grpc.Core;
using Grpc.Net.Client;
using System.Collections.Concurrent;
using System.Globalization;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Threading.Channels;

namespace Holons.Observability;

public enum Family { Logs, Metrics, Events, Prom, Otel }

public enum Level
{
    Unset = 0, Trace = 1, Debug = 5, Info = 9, Warn = 13, Error = 17, Fatal = 21,
}

public static class LevelExt
{
    public static string Name(this Level l) => l switch
    {
        Level.Trace => "TRACE", Level.Debug => "DEBUG", Level.Info => "INFO",
        Level.Warn => "WARN", Level.Error => "ERROR", Level.Fatal => "FATAL",
        _ => "UNSPECIFIED",
    };
}

public static class EventNames
{
    public const string InstanceSpawned = "instance.spawned";
    public const string InstanceReady = "instance.ready";
    public const string InstanceExited = "instance.exited";
    public const string InstanceCrashed = "instance.crashed";
    public const string SessionStarted = "session.started";
    public const string SessionEnded = "session.ended";
    public const string HandlerPanic = "handler.panic";
    public const string ConfigReloaded = "config.reloaded";
}

public sealed class InvalidTokenException : Exception
{
    public string Token { get; }
    public string Variable { get; }
    public InvalidTokenException(string token, string reason)
        : this("OP_OBS", token, reason) { }
    public InvalidTokenException(string variable, string token, string reason)
        : base($"{variable}: {reason}: {token}") { Variable = variable; Token = token; }
}

public static class Env
{
    private static readonly HashSet<string> V1Tokens =
        new() { "logs", "metrics", "events", "prom", "all" };

    public static HashSet<Family> ParseOpObs(string? raw)
    {
        var families = new HashSet<Family>();
        if (string.IsNullOrWhiteSpace(raw)) return families;
        foreach (var p in raw.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries))
        {
            if (p == "otel")
                throw new InvalidTokenException(p, "otel export is reserved for v2; not implemented in v1");
            if (p == "sessions")
                throw new InvalidTokenException(p, "sessions are reserved for v2; not implemented in v1");
            if (!V1Tokens.Contains(p))
                throw new InvalidTokenException(p, "unknown OP_OBS token");
            if (p == "all")
            {
                families.Add(Family.Logs); families.Add(Family.Metrics);
                families.Add(Family.Events); families.Add(Family.Prom);
            }
            else if (System.Enum.TryParse<Family>(char.ToUpper(p[0]) + p[1..], out var f))
            {
                families.Add(f);
            }
        }
        return families;
    }

    public static void CheckEnv(IDictionary<string, string>? env = null)
    {
        var sessions = env != null
            ? (env.TryGetValue("OP_SESSIONS", out var sv) ? sv : "")
            : (Environment.GetEnvironmentVariable("OP_SESSIONS") ?? "");
        if (!string.IsNullOrWhiteSpace(sessions))
            throw new InvalidTokenException("OP_SESSIONS", sessions.Trim(), "sessions are reserved for v2; not implemented in v1");

        string raw;
        if (env != null)
        {
            raw = env.TryGetValue("OP_OBS", out var v) ? v : "";
        }
        else
        {
            raw = Environment.GetEnvironmentVariable("OP_OBS") ?? "";
        }
        if (string.IsNullOrWhiteSpace(raw)) return;
        foreach (var p in raw.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries))
        {
            if (p == "otel")
                throw new InvalidTokenException(p, "otel export is reserved for v2; not implemented in v1");
            if (p == "sessions")
                throw new InvalidTokenException(p, "sessions are reserved for v2; not implemented in v1");
            if (!V1Tokens.Contains(p))
                throw new InvalidTokenException(p, "unknown OP_OBS token");
        }
    }
}

public sealed class Hop
{
    public string Slug { get; }
    public string InstanceUid { get; }
    public Hop(string slug, string instanceUid) { Slug = slug; InstanceUid = instanceUid; }
}

public static class Chain
{
    public static List<string> AppendDirectChild(IEnumerable<string> src, string childSlug, string childUid = "")
    {
        var l = new List<string>(src);
        if (!string.IsNullOrEmpty(childSlug))
            l.Add(childSlug);
        return l;
    }

    public static List<string> EnrichForMultilog(IEnumerable<string> wire, string srcSlug, string srcUid = "")
        => AppendDirectChild(wire, srcSlug, srcUid);
}

public static class AttributeNames
{
    public const string HolonsSlug = "holons.slug";
    public const string ServiceName = "service.name";
    public const string HolonsInstanceUid = "holons.instance_uid";
    public const string ServiceInstanceId = "service.instance.id";
    public const string HolonsSessionId = "holons.session_id";
    public const string HolonsTransport = "holons.transport";
    public const string RpcMethod = "rpc.method";
    public const string LoggerName = "logger.name";
    public const string CodeCaller = "code.caller";
}

public sealed class LogRecord
{
    public global::Holons.V1.LogRecord Record { get; init; } = new();
    public bool Private { get; init; }

    public DateTime Timestamp =>
        Record.TimeUnixNano == 0
            ? DateTime.MinValue
            : DateTimeOffset.FromUnixTimeMilliseconds(0)
                .AddTicks((long)(Record.TimeUnixNano / 100))
                .UtcDateTime;
}

public sealed class LogRing
{
    private readonly int _capacity;
    private readonly Queue<LogRecord> _buf;
    private readonly List<Action<LogRecord>> _subs = new();
    private readonly object _lock = new();

    public LogRing(int capacity = 1024)
    {
        _capacity = Math.Max(1, capacity);
        _buf = new Queue<LogRecord>(_capacity);
    }

    public void Push(LogRecord e)
    {
        List<Action<LogRecord>> copy;
        lock (_lock)
        {
            if (_buf.Count == _capacity) _buf.Dequeue();
            _buf.Enqueue(e);
            copy = new List<Action<LogRecord>>(_subs);
        }
        foreach (var s in copy) try { s(e); } catch { }
    }

    public List<LogRecord> Drain()
    {
        lock (_lock) return new List<LogRecord>(_buf);
    }

    public List<LogRecord> DrainSince(DateTime cutoff)
    {
        lock (_lock) return _buf.Where(e => e.Timestamp >= cutoff).ToList();
    }

    public IDisposable Subscribe(Action<LogRecord> fn)
    {
        lock (_lock) _subs.Add(fn);
        return new Sub(() => { lock (_lock) _subs.Remove(fn); });
    }

    public (List<LogRecord> Replay, IDisposable Subscription) ReplayAndSubscribe(DateTime? cutoff, Action<LogRecord> fn)
    {
        lock (_lock)
        {
            var replay = cutoff is { } since
                ? _buf.Where(e => e.Timestamp >= since).ToList()
                : new List<LogRecord>(_buf);
            _subs.Add(fn);
            return (replay, new Sub(() => { lock (_lock) _subs.Remove(fn); }));
        }
    }

    public int Count { get { lock (_lock) return _buf.Count; } }

    private sealed class Sub : IDisposable
    {
        private readonly Action _dispose;
        public Sub(Action d) { _dispose = d; }
        public void Dispose() => _dispose();
    }
}

public sealed class EventBus
{
    private readonly int _capacity;
    private readonly Queue<LogRecord> _buf;
    private readonly List<Action<LogRecord>> _subs = new();
    private readonly object _lock = new();
    private bool _closed;

    public EventBus(int capacity = 256)
    {
        _capacity = Math.Max(1, capacity);
        _buf = new Queue<LogRecord>(_capacity);
    }

    public void Emit(LogRecord e)
    {
        List<Action<LogRecord>> copy;
        lock (_lock)
        {
            if (_closed) return;
            if (_buf.Count == _capacity) _buf.Dequeue();
            _buf.Enqueue(e);
            copy = new List<Action<LogRecord>>(_subs);
        }
        foreach (var s in copy) try { s(e); } catch { }
    }

    public List<LogRecord> Drain() { lock (_lock) return new List<LogRecord>(_buf); }
    public List<LogRecord> DrainSince(DateTime cutoff) { lock (_lock) return _buf.Where(e => e.Timestamp >= cutoff).ToList(); }

    public IDisposable Subscribe(Action<LogRecord> fn)
    {
        lock (_lock) _subs.Add(fn);
        return new Sub(() => { lock (_lock) _subs.Remove(fn); });
    }

    public (List<LogRecord> Replay, IDisposable Subscription) ReplayAndSubscribe(DateTime? cutoff, Action<LogRecord> fn)
    {
        lock (_lock)
        {
            var replay = cutoff is { } since
                ? _buf.Where(e => e.Timestamp >= since).ToList()
                : new List<LogRecord>(_buf);
            if (!_closed)
                _subs.Add(fn);
            return (replay, new Sub(() => { lock (_lock) _subs.Remove(fn); }));
        }
    }

    public void Close()
    {
        lock (_lock) { _closed = true; _subs.Clear(); }
    }

    private sealed class Sub : IDisposable
    {
        private readonly Action _dispose;
        public Sub(Action d) { _dispose = d; }
        public void Dispose() => _dispose();
    }
}

public sealed class Counter
{
    public string Name { get; }
    public string Help { get; }
    public IReadOnlyDictionary<string, string> Labels { get; }
    private long _value;
    internal Counter(string name, string help, Dictionary<string, string> labels)
    {
        Name = name; Help = help; Labels = labels;
    }
    public void Inc(long n = 1) { if (n >= 0) Interlocked.Add(ref _value, n); }
    public void Add(long n) => Inc(n);
    public long Value => Interlocked.Read(ref _value);
}

public sealed class Gauge
{
    public string Name { get; }
    public string Help { get; }
    public IReadOnlyDictionary<string, string> Labels { get; }
    private double _value;
    private readonly object _lock = new();
    internal Gauge(string name, string help, Dictionary<string, string> labels)
    {
        Name = name; Help = help; Labels = labels;
    }
    public void Set(double v) { lock (_lock) _value = v; }
    public void Add(double d) { lock (_lock) _value += d; }
    public double Value { get { lock (_lock) return _value; } }
}

public sealed class HistogramSnapshot
{
    public IReadOnlyList<double> Bounds { get; init; } = Array.Empty<double>();
    public IReadOnlyList<long> Counts { get; init; } = Array.Empty<long>();
    public long Total { get; init; }
    public double Sum { get; init; }
    public double Min { get; init; }
    public double Max { get; init; }
    public double Quantile(double q)
    {
        if (Total == 0) return double.NaN;
        var target = Total * q;
        for (var i = 0; i < Counts.Count; i++) if (Counts[i] >= target) return Bounds[i];
        return double.PositiveInfinity;
    }
}

public sealed class Histogram
{
    public static readonly double[] DefaultBuckets = {
        50e-6, 100e-6, 250e-6, 500e-6,
        1e-3, 2.5e-3, 5e-3, 10e-3, 25e-3, 50e-3, 100e-3, 250e-3, 500e-3,
        1.0, 2.5, 5.0, 10.0, 30.0, 60.0,
    };

    public string Name { get; }
    public string Help { get; }
    public IReadOnlyDictionary<string, string> Labels { get; }
    private readonly double[] _bounds;
    private readonly long[] _counts;
    private long _total;
    private double _sum;
    private double _min = double.NaN;
    private double _max = double.NaN;
    private readonly object _lock = new();

    internal Histogram(string name, string help, Dictionary<string, string> labels, double[]? bounds)
    {
        Name = name; Help = help; Labels = labels;
        _bounds = (bounds is null || bounds.Length == 0 ? DefaultBuckets : (double[])bounds.Clone());
        Array.Sort(_bounds);
        _counts = new long[_bounds.Length];
    }

    public void Observe(double v)
    {
        lock (_lock)
        {
            _total++;
            _sum += v;
            _min = double.IsNaN(_min) ? v : Math.Min(_min, v);
            _max = double.IsNaN(_max) ? v : Math.Max(_max, v);
            for (var i = 0; i < _bounds.Length; i++)
                if (v <= _bounds[i]) _counts[i]++;
        }
    }

    public void ObserveDuration(TimeSpan d) => Observe(d.TotalSeconds);

    public HistogramSnapshot Snapshot()
    {
        lock (_lock)
        {
            return new HistogramSnapshot
            {
                Bounds = (double[])_bounds.Clone(),
                Counts = (long[])_counts.Clone(),
                Total = _total,
                Sum = _sum,
                Min = double.IsNaN(_min) ? 0 : _min,
                Max = double.IsNaN(_max) ? 0 : _max,
            };
        }
    }
}

public sealed class Registry
{
    private readonly ConcurrentDictionary<string, Counter> _counters = new();
    private readonly ConcurrentDictionary<string, Gauge> _gauges = new();
    private readonly ConcurrentDictionary<string, Histogram> _histograms = new();

    public Counter Counter(string name, string help = "", Dictionary<string, string>? labels = null)
        => _counters.GetOrAdd(Key(name, labels), _ => new Counter(name, help, labels ?? new()));
    public Gauge Gauge(string name, string help = "", Dictionary<string, string>? labels = null)
        => _gauges.GetOrAdd(Key(name, labels), _ => new Gauge(name, help, labels ?? new()));
    public Histogram Histogram(string name, string help = "", Dictionary<string, string>? labels = null,
        double[]? bounds = null)
        => _histograms.GetOrAdd(Key(name, labels), _ => new Histogram(name, help, labels ?? new(), bounds));

    private static string Key(string name, Dictionary<string, string>? labels)
    {
        if (labels == null || labels.Count == 0) return name;
        var keys = labels.Keys.OrderBy(x => x, StringComparer.Ordinal).ToArray();
        var sb = new StringBuilder(name);
        foreach (var k in keys) { sb.Append('|').Append(k).Append('=').Append(labels[k]); }
        return sb.ToString();
    }

    public IReadOnlyList<Counter> Counters => _counters.Values.OrderBy(c => c.Name).ToList();
    public IReadOnlyList<Gauge> Gauges => _gauges.Values.OrderBy(g => g.Name).ToList();
    public IReadOnlyList<Histogram> Histograms => _histograms.Values.OrderBy(h => h.Name).ToList();
}

public sealed record ObsConfig
{
    public string Slug { get; init; } = "";
    public Level DefaultLogLevel { get; init; } = Level.Info;
    public string SessionId { get; init; } = "";
    public string PromAddr { get; init; } = "";
    public string[] RedactedFields { get; init; } = Array.Empty<string>();
    public int LogsRingSize { get; init; } = 1024;
    public int EventsRingSize { get; init; } = 256;
    public string RunDir { get; init; } = "";
    public string InstanceUid { get; init; } = "";
    public string OrganismUid { get; init; } = "";
    public string OrganismSlug { get; init; } = "";
}

public static class Wire
{
    public static global::Holons.V1.AnyValue ToAnyValue(object? value)
    {
        return value switch
        {
            null => new global::Holons.V1.AnyValue { StringValue = "" },
            string s => new global::Holons.V1.AnyValue { StringValue = s },
            bool b => new global::Holons.V1.AnyValue { BoolValue = b },
            byte i => new global::Holons.V1.AnyValue { IntValue = i },
            sbyte i => new global::Holons.V1.AnyValue { IntValue = i },
            short i => new global::Holons.V1.AnyValue { IntValue = i },
            ushort i => new global::Holons.V1.AnyValue { IntValue = i },
            int i => new global::Holons.V1.AnyValue { IntValue = i },
            uint i => new global::Holons.V1.AnyValue { IntValue = i },
            long i => new global::Holons.V1.AnyValue { IntValue = i },
            ulong i when i <= long.MaxValue => new global::Holons.V1.AnyValue { IntValue = (long)i },
            float f => new global::Holons.V1.AnyValue { DoubleValue = f },
            double d => new global::Holons.V1.AnyValue { DoubleValue = d },
            IFormattable formattable => new global::Holons.V1.AnyValue { StringValue = formattable.ToString(null, CultureInfo.InvariantCulture) },
            _ => new global::Holons.V1.AnyValue { StringValue = value.ToString() ?? "" },
        };
    }

    public static string AnyValueString(global::Holons.V1.AnyValue? value)
    {
        if (value is null) return "";
        return value.ValueCase switch
        {
            global::Holons.V1.AnyValue.ValueOneofCase.StringValue => value.StringValue,
            global::Holons.V1.AnyValue.ValueOneofCase.BoolValue => value.BoolValue ? "true" : "false",
            global::Holons.V1.AnyValue.ValueOneofCase.IntValue => value.IntValue.ToString(CultureInfo.InvariantCulture),
            global::Holons.V1.AnyValue.ValueOneofCase.DoubleValue => value.DoubleValue.ToString("G17", CultureInfo.InvariantCulture),
            _ => "",
        };
    }

    public static global::Holons.V1.KeyValue KeyValue(string key, object? value) =>
        new() { Key = key, Value = ToAnyValue(value) };

    public static IReadOnlyList<global::Holons.V1.KeyValue> ResourceAttributes(ObsConfig cfg)
    {
        return
        [
            KeyValue(AttributeNames.HolonsSlug, cfg.Slug),
            KeyValue(AttributeNames.ServiceName, cfg.Slug),
            KeyValue(AttributeNames.HolonsInstanceUid, cfg.InstanceUid),
            KeyValue(AttributeNames.ServiceInstanceId, cfg.InstanceUid),
            KeyValue(AttributeNames.HolonsSessionId, cfg.SessionId),
        ];
    }

    public static string AttributeString(IEnumerable<global::Holons.V1.KeyValue> attrs, string key)
    {
        var attr = attrs.FirstOrDefault(a => string.Equals(a.Key, key, StringComparison.Ordinal));
        return attr is null ? "" : AnyValueString(attr.Value);
    }
}

public sealed class Logger
{
    private readonly Observability _obs;
    public string Name { get; }
    private int _level;
    internal Logger(Observability obs, string name)
    {
        _obs = obs; Name = name;
        _level = (int)obs.Config.DefaultLogLevel;
    }
    public void SetLevel(Level l) => Interlocked.Exchange(ref _level, (int)l);
    public bool Enabled(Level l) => (int)l >= Volatile.Read(ref _level);

    public void Log(Level lvl, string message, IDictionary<string, object?>? fields = null, bool privateEntry = false)
    {
        if (!Enabled(lvl)) return;
        var redact = new HashSet<string>(_obs.Config.RedactedFields);
        var attrs = new List<global::Holons.V1.KeyValue>(Wire.ResourceAttributes(_obs.Config));
        if (!string.IsNullOrEmpty(Name))
            attrs.Add(Wire.KeyValue(AttributeNames.LoggerName, Name));
        if (fields != null)
        {
            foreach (var (k, v) in fields)
            {
                if (string.IsNullOrEmpty(k)) continue;
                attrs.Add(Wire.KeyValue(k, redact.Contains(k) ? "<redacted>" : v));
            }
        }
        var now = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000;
        _obs.LogRing?.Push(new LogRecord
        {
            Record = new global::Holons.V1.LogRecord
            {
                TimeUnixNano = now,
                ObservedTimeUnixNano = now,
                SeverityNumber = (global::Holons.V1.SeverityNumber)lvl,
                SeverityText = lvl.Name(),
                Body = Wire.ToAnyValue(message),
                Attributes = { attrs },
            },
            Private = privateEntry,
        });
    }

    public void Trace(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Trace, m, f, privateEntry);
    public void Debug(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Debug, m, f, privateEntry);
    public void Info(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Info, m, f, privateEntry);
    public void Warn(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Warn, m, f, privateEntry);
    public void Error(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Error, m, f, privateEntry);
    public void Fatal(string m, IDictionary<string, object?>? f = null, bool privateEntry = false) => Log(Level.Fatal, m, f, privateEntry);
}

public sealed class Observability
{
    public ObsConfig Config { get; }
    public HashSet<Family> Families { get; }
    public LogRing? LogRing { get; }
    public EventBus? EventBus { get; }
    public Registry? Registry { get; }
    public ulong StartTimeUnixNano { get; }
    private readonly ConcurrentDictionary<string, Logger> _loggers = new();

    internal Observability(ObsConfig cfg, HashSet<Family> families)
    {
        Config = cfg;
        Families = families;
        StartTimeUnixNano = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000;
        LogRing = families.Contains(Family.Logs) ? new LogRing(cfg.LogsRingSize) : null;
        EventBus = families.Contains(Family.Events) ? new EventBus(cfg.EventsRingSize) : null;
        Registry = families.Contains(Family.Metrics) ? new Registry() : null;
    }

    public bool Enabled(Family f) => Families.Contains(f);

    public bool IsOrganismRoot =>
        !string.IsNullOrEmpty(Config.OrganismUid) && Config.OrganismUid == Config.InstanceUid;

    public Logger Logger(string name)
    {
        if (!Families.Contains(Family.Logs)) return DisabledLogger;
        return _loggers.GetOrAdd(name, n => new Logger(this, n));
    }

    public Counter? Counter(string name, string help = "", Dictionary<string, string>? labels = null)
        => Registry?.Counter(name, help, labels);
    public Gauge? Gauge(string name, string help = "", Dictionary<string, string>? labels = null)
        => Registry?.Gauge(name, help, labels);
    public Histogram? Histogram(string name, string help = "", Dictionary<string, string>? labels = null,
        double[]? bounds = null) => Registry?.Histogram(name, help, labels, bounds);

    public void Emit(string eventName, IDictionary<string, object?>? payload = null, bool privateEntry = false)
    {
        if (EventBus is null) return;
        var redact = new HashSet<string>(Config.RedactedFields);
        var attrs = new List<global::Holons.V1.KeyValue>(Wire.ResourceAttributes(Config));
        if (payload != null)
        {
            foreach (var (k, v) in payload)
            {
                if (string.IsNullOrEmpty(k)) continue;
                attrs.Add(Wire.KeyValue(k, redact.Contains(k) ? "<redacted>" : v));
            }
        }
        var now = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000;
        EventBus.Emit(new LogRecord
        {
            Record = new global::Holons.V1.LogRecord
            {
                TimeUnixNano = now,
                ObservedTimeUnixNano = now,
                SeverityNumber = global::Holons.V1.SeverityNumber.Info,
                SeverityText = Level.Info.Name(),
                EventName = eventName,
                Body = Wire.ToAnyValue(eventName),
                Attributes = { attrs },
            },
            Private = privateEntry,
        });
    }

    public void Close() => EventBus?.Close();

    internal static readonly Logger DisabledLogger = new(new Observability(new ObsConfig { DefaultLogLevel = Level.Fatal }, new HashSet<Family>()), "");
}

public static class ObservabilityRegistry
{
    private static Observability? _current;
    private static readonly object _lock = new();

    public static Observability Configure(ObsConfig cfg) => ConfigureFromEnv(cfg);

    public static Observability ConfigureFromEnv(ObsConfig cfg, IDictionary<string, string>? env = null)
    {
        Env.CheckEnv(env);
        var families = Env.ParseOpObs(Read(env, "OP_OBS"));
        var normalized = cfg with { };
        if (string.IsNullOrEmpty(normalized.Slug))
            normalized = normalized with { Slug = ResolveManifestSlug() };
        if (string.IsNullOrEmpty(normalized.InstanceUid))
            normalized = normalized with { InstanceUid = NewInstanceUid() };
        if (!string.IsNullOrEmpty(normalized.RunDir))
            normalized = normalized with { RunDir = DeriveRunDir(normalized.RunDir, normalized.Slug, normalized.InstanceUid) };
        var obs = new Observability(normalized, families);
        lock (_lock) _current = obs;
        return obs;
    }

    public static Observability FromEnv(ObsConfig? baseCfg = null) => FromEnvMap(baseCfg);

    public static Observability FromEnvMap(ObsConfig? baseCfg = null, IDictionary<string, string>? env = null)
    {
        var b = baseCfg ?? new ObsConfig();
        var cfg = b with
        {
            InstanceUid = string.IsNullOrEmpty(b.InstanceUid) ? Read(env, "OP_INSTANCE_UID") : b.InstanceUid,
            SessionId = string.IsNullOrEmpty(b.SessionId) ? Read(env, "OP_SESSION_ID") : b.SessionId,
            OrganismUid = string.IsNullOrEmpty(b.OrganismUid) ? Read(env, "OP_ORGANISM_UID") : b.OrganismUid,
            OrganismSlug = string.IsNullOrEmpty(b.OrganismSlug) ? Read(env, "OP_ORGANISM_SLUG") : b.OrganismSlug,
            PromAddr = string.IsNullOrEmpty(b.PromAddr) ? Read(env, "OP_PROM_ADDR") : b.PromAddr,
            RunDir = string.IsNullOrEmpty(b.RunDir) ? Read(env, "OP_RUN_DIR") : b.RunDir,
        };
        return ConfigureFromEnv(cfg, env);
    }

    public static Observability Current()
    {
        lock (_lock) { return _current ?? new Observability(new ObsConfig { DefaultLogLevel = Level.Fatal }, new HashSet<Family>()); }
    }

    public static void Reset()
    {
        lock (_lock) { _current?.Close(); _current = null; }
    }

    public static string DeriveRunDir(string root, string slug, string uid)
    {
        if (string.IsNullOrEmpty(root) || string.IsNullOrEmpty(slug) || string.IsNullOrEmpty(uid))
            return root;
        return Path.Combine(root, slug, uid);
    }

    private static string NewInstanceUid() =>
        $"{Environment.ProcessId}-{DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()}-{Guid.NewGuid():N}";

    private static string Read(IDictionary<string, string>? env, string key)
    {
        if (env != null && env.TryGetValue(key, out var value))
            return value;
        return Environment.GetEnvironmentVariable(key) ?? "";
    }

    private static string ResolveManifestSlug()
    {
        try
        {
            return global::Holons.Identity.Resolve(".").Identity.Slug();
        }
        catch
        {
            return "";
        }
    }
}

public sealed class ObservabilityGrpcService : global::Holons.V1.HolonObservability.HolonObservabilityBase
{
    private readonly Observability _obs;

    public ObservabilityGrpcService(Observability obs)
    {
        _obs = obs;
    }

    public override async Task Logs(
        global::Holons.V1.LogsRequest request,
        IServerStreamWriter<global::Holons.V1.LogRecord> responseStream,
        ServerCallContext context)
    {
        if (!_obs.Enabled(Family.Logs) || _obs.LogRing == null)
            throw new RpcException(new Status(StatusCode.FailedPrecondition, "logs family is not enabled (OP_OBS)"));

        var minLevel = request.MinSeverityNumber == global::Holons.V1.SeverityNumber.Unspecified
            ? (int)Level.Info
            : (int)request.MinSeverityNumber;
        var cutoff = request.Since is null
            ? (DateTime?)null
            : DateTime.UtcNow - request.Since.ToTimeSpan();
        IDisposable? subscription = null;
        var entries = request.Follow
            ? new List<LogRecord>()
            : cutoff is null
                ? _obs.LogRing.Drain()
                : _obs.LogRing.DrainSince(cutoff.Value);
        Channel<LogRecord>? queue = null;
        if (request.Follow)
        {
            queue = Channel.CreateUnbounded<LogRecord>();
            // Snapshot and subscription are registered under one ring lock to avoid a replay/live drop.
            var replayAndSub = _obs.LogRing.ReplayAndSubscribe(cutoff, entry => queue.Writer.TryWrite(entry));
            entries = replayAndSub.Replay;
            subscription = replayAndSub.Subscription;
        }
        foreach (var entry in entries)
        {
            if (entry.Private)
                continue;
            if ((int)entry.Record.SeverityNumber < minLevel)
                continue;
            if (request.SessionIds.Count > 0 && !request.SessionIds.Contains(Wire.AttributeString(entry.Record.Attributes, AttributeNames.HolonsSessionId)))
                continue;
            if (request.RpcMethods.Count > 0 && !request.RpcMethods.Contains(Wire.AttributeString(entry.Record.Attributes, AttributeNames.RpcMethod)))
                continue;
            await responseStream.WriteAsync(ToProtoLogRecord(entry)).ConfigureAwait(false);
        }

        if (!request.Follow)
            return;

        using (subscription)
        {
        try
        {
            await foreach (var entry in queue!.Reader.ReadAllAsync(context.CancellationToken).ConfigureAwait(false))
            {
                if (entry.Private)
                    continue;
                if ((int)entry.Record.SeverityNumber < minLevel)
                    continue;
                if (request.SessionIds.Count > 0 && !request.SessionIds.Contains(Wire.AttributeString(entry.Record.Attributes, AttributeNames.HolonsSessionId)))
                    continue;
                if (request.RpcMethods.Count > 0 && !request.RpcMethods.Contains(Wire.AttributeString(entry.Record.Attributes, AttributeNames.RpcMethod)))
                    continue;
                await responseStream.WriteAsync(ToProtoLogRecord(entry)).ConfigureAwait(false);
            }
        }
        catch (OperationCanceledException)
        {
            // Client closed the follow stream.
        }
        }
    }

    public override async Task Metrics(
        global::Holons.V1.MetricsRequest request,
        IServerStreamWriter<global::Holons.V1.Metric> responseStream,
        ServerCallContext context)
    {
        _ = context;
        if (!_obs.Enabled(Family.Metrics) || _obs.Registry == null)
            throw new RpcException(new Status(StatusCode.FailedPrecondition, "metrics family is not enabled (OP_OBS)"));

        foreach (var metric in ToProtoMetrics(_obs.Registry, _obs.Config, _obs.StartTimeUnixNano))
        {
            if (request.NamePrefixes.Count == 0 || request.NamePrefixes.Any(prefix => metric.Name.StartsWith(prefix, StringComparison.Ordinal)))
                await responseStream.WriteAsync(metric).ConfigureAwait(false);
        }
    }

    public override async Task Events(
        global::Holons.V1.EventsRequest request,
        IServerStreamWriter<global::Holons.V1.LogRecord> responseStream,
        ServerCallContext context)
    {
        if (!_obs.Enabled(Family.Events) || _obs.EventBus == null)
            throw new RpcException(new Status(StatusCode.FailedPrecondition, "events family is not enabled (OP_OBS)"));

        var wanted = request.EventNames.ToHashSet(StringComparer.Ordinal);
        var cutoff = request.Since is null
            ? (DateTime?)null
            : DateTime.UtcNow - request.Since.ToTimeSpan();
        IDisposable? subscription = null;
        var events = request.Follow
            ? new List<LogRecord>()
            : cutoff is null
                ? _obs.EventBus.Drain()
                : _obs.EventBus.DrainSince(cutoff.Value);
        Channel<LogRecord>? queue = null;
        if (request.Follow)
        {
            queue = Channel.CreateUnbounded<LogRecord>();
            // Snapshot and subscription are registered under one bus lock to avoid a replay/live drop.
            var replayAndSub = _obs.EventBus.ReplayAndSubscribe(cutoff, ev => queue.Writer.TryWrite(ev));
            events = replayAndSub.Replay;
            subscription = replayAndSub.Subscription;
        }
        foreach (var ev in events)
        {
            if (ev.Private)
                continue;
            if (wanted.Count > 0 && !wanted.Contains(ev.Record.EventName))
                continue;
            await responseStream.WriteAsync(ToProtoLogRecord(ev)).ConfigureAwait(false);
        }

        if (!request.Follow)
            return;

        using (subscription)
        {
        try
        {
            await foreach (var ev in queue!.Reader.ReadAllAsync(context.CancellationToken).ConfigureAwait(false))
            {
                if (ev.Private)
                    continue;
                if (wanted.Count > 0 && !wanted.Contains(ev.Record.EventName))
                    continue;
                await responseStream.WriteAsync(ToProtoLogRecord(ev)).ConfigureAwait(false);
            }
        }
        catch (OperationCanceledException)
        {
            // Client closed the follow stream.
        }
        }
    }

    public static global::Holons.V1.LogRecord ToProtoLogRecord(LogRecord entry) =>
        entry.Record.Clone();

    public static IReadOnlyList<global::Holons.V1.Metric> ToProtoMetrics(Registry registry, ObsConfig config, ulong startTimeUnixNano)
    {
        var metrics = new List<global::Holons.V1.Metric>();
        var now = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() * 1_000_000;
        foreach (var counter in registry.Counters)
        {
            var point = new global::Holons.V1.NumberDataPoint
            {
                StartTimeUnixNano = startTimeUnixNano,
                TimeUnixNano = now,
                AsInt = counter.Value,
            };
            point.Attributes.Add(MetricAttributes(config, counter.Labels));
            metrics.Add(new global::Holons.V1.Metric
            {
                Name = counter.Name,
                Description = counter.Help,
                Sum = new global::Holons.V1.Sum
                {
                    AggregationTemporality = global::Holons.V1.AggregationTemporality.Cumulative,
                    IsMonotonic = true,
                    DataPoints = { point },
                },
            });
        }
        foreach (var gauge in registry.Gauges)
        {
            var point = new global::Holons.V1.NumberDataPoint
            {
                StartTimeUnixNano = startTimeUnixNano,
                TimeUnixNano = now,
                AsDouble = gauge.Value,
            };
            point.Attributes.Add(MetricAttributes(config, gauge.Labels));
            metrics.Add(new global::Holons.V1.Metric
            {
                Name = gauge.Name,
                Description = gauge.Help,
                Gauge = new global::Holons.V1.Gauge { DataPoints = { point } },
            });
        }
        foreach (var histogram in registry.Histograms)
        {
            var snapshot = histogram.Snapshot();
            var point = new global::Holons.V1.HistogramDataPoint
            {
                StartTimeUnixNano = startTimeUnixNano,
                TimeUnixNano = now,
                Count = (ulong)snapshot.Total,
                Sum = snapshot.Sum,
                Min = snapshot.Min,
                Max = snapshot.Max,
            };
            point.ExplicitBounds.Add(snapshot.Bounds);
            point.BucketCounts.Add(HistogramBucketCounts(snapshot));
            point.Attributes.Add(MetricAttributes(config, histogram.Labels));
            metrics.Add(new global::Holons.V1.Metric
            {
                Name = histogram.Name,
                Description = histogram.Help,
                Histogram = new global::Holons.V1.Histogram
                {
                    AggregationTemporality = global::Holons.V1.AggregationTemporality.Cumulative,
                    DataPoints = { point },
                },
            });
        }
        return metrics;
    }

    private static IReadOnlyList<ulong> HistogramBucketCounts(HistogramSnapshot snapshot)
    {
        var counts = new List<ulong>(snapshot.Counts.Count + 1);
        long previous = 0;
        foreach (var count in snapshot.Counts)
        {
            var delta = Math.Max(0, count - previous);
            counts.Add((ulong)delta);
            previous = count;
        }
        counts.Add((ulong)Math.Max(0, snapshot.Total - previous));
        return counts;
    }

    private static IReadOnlyList<global::Holons.V1.KeyValue> MetricAttributes(
        ObsConfig config,
        IReadOnlyDictionary<string, string> labels)
    {
        var attrs = new List<global::Holons.V1.KeyValue>
        {
            Wire.KeyValue(AttributeNames.HolonsSlug, config.Slug),
            Wire.KeyValue(AttributeNames.ServiceName, config.Slug),
            Wire.KeyValue(AttributeNames.HolonsInstanceUid, config.InstanceUid),
            Wire.KeyValue(AttributeNames.ServiceInstanceId, config.InstanceUid),
        };
        foreach (var (key, value) in labels.OrderBy(pair => pair.Key, StringComparer.Ordinal))
            attrs.Add(Wire.KeyValue(key, value));
        return attrs;
    }

    internal static LogRecord FromProtoLogRecord(global::Holons.V1.LogRecord record) =>
        new() { Record = record.Clone() };
}

internal static class PrometheusText
{
    public static string Render(Observability obs)
    {
        var registry = obs.Registry;
        if (registry is null)
            return "";

        var sb = new StringBuilder();
        foreach (var counter in registry.Counters)
        {
            AppendHelpType(sb, counter.Name, counter.Help, "counter");
            sb.Append(counter.Name).Append(FormatLabels(WithIdentityLabels(obs, counter.Labels))).Append(' ')
                .Append(counter.Value.ToString(CultureInfo.InvariantCulture)).Append('\n');
        }
        foreach (var gauge in registry.Gauges)
        {
            AppendHelpType(sb, gauge.Name, gauge.Help, "gauge");
            sb.Append(gauge.Name).Append(FormatLabels(WithIdentityLabels(obs, gauge.Labels))).Append(' ')
                .Append(gauge.Value.ToString("G17", CultureInfo.InvariantCulture)).Append('\n');
        }
        foreach (var histogram in registry.Histograms)
        {
            AppendHelpType(sb, histogram.Name, histogram.Help, "histogram");
            var snapshot = histogram.Snapshot();
            var labels = WithIdentityLabels(obs, histogram.Labels);
            for (var i = 0; i < snapshot.Bounds.Count; i++)
            {
                var bucketLabels = new Dictionary<string, string>(labels)
                {
                    ["le"] = snapshot.Bounds[i].ToString("G17", CultureInfo.InvariantCulture),
                };
                sb.Append(histogram.Name).Append("_bucket").Append(FormatLabels(bucketLabels)).Append(' ')
                    .Append(snapshot.Counts[i].ToString(CultureInfo.InvariantCulture)).Append('\n');
            }
            var infLabels = new Dictionary<string, string>(labels) { ["le"] = "+Inf" };
            sb.Append(histogram.Name).Append("_bucket").Append(FormatLabels(infLabels)).Append(' ')
                .Append(snapshot.Total.ToString(CultureInfo.InvariantCulture)).Append('\n');
            sb.Append(histogram.Name).Append("_sum").Append(FormatLabels(labels)).Append(' ')
                .Append(snapshot.Sum.ToString("G17", CultureInfo.InvariantCulture)).Append('\n');
            sb.Append(histogram.Name).Append("_count").Append(FormatLabels(labels)).Append(' ')
                .Append(snapshot.Total.ToString(CultureInfo.InvariantCulture)).Append('\n');
        }
        return sb.ToString();
    }

    private static void AppendHelpType(StringBuilder sb, string name, string help, string type)
    {
        if (!string.IsNullOrEmpty(help))
            sb.Append("# HELP ").Append(name).Append(' ').Append(EscapeHelp(help)).Append('\n');
        sb.Append("# TYPE ").Append(name).Append(' ').Append(type).Append('\n');
    }

    private static IReadOnlyDictionary<string, string> WithIdentityLabels(
        Observability obs,
        IReadOnlyDictionary<string, string> labels)
    {
        var result = new Dictionary<string, string>(labels);
        if (!string.IsNullOrEmpty(obs.Config.Slug))
            result.TryAdd("slug", obs.Config.Slug);
        if (!string.IsNullOrEmpty(obs.Config.InstanceUid))
            result.TryAdd("instance_uid", obs.Config.InstanceUid);
        return result;
    }

    private static string FormatLabels(IReadOnlyDictionary<string, string> labels)
    {
        if (labels.Count == 0)
            return "";

        var parts = labels
            .OrderBy(pair => pair.Key, StringComparer.Ordinal)
            .Select(pair => $"{pair.Key}=\"{EscapeLabelValue(pair.Value)}\"");
        return "{" + string.Join(",", parts) + "}";
    }

    private static string EscapeHelp(string value) =>
        value.Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\n", "\\n", StringComparison.Ordinal);

    private static string EscapeLabelValue(string value) =>
        value.Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\n", "\\n", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal);
}

internal sealed class PrometheusServer : IDisposable
{
    private readonly Observability _obs;
    private readonly TcpListener _listener;
    private readonly CancellationTokenSource _cts = new();
    private readonly Task _task;

    private PrometheusServer(Observability obs, TcpListener listener, string address)
    {
        _obs = obs;
        _listener = listener;
        Address = address;
        _task = Task.Run(RunAsync);
    }

    public string Address { get; }

    public static PrometheusServer Start(Observability obs, string bind)
    {
        var (ip, advertisedHost, port) = ParseBind(bind);
        var listener = new TcpListener(ip, port);
        listener.Start();
        var actualPort = ((IPEndPoint)listener.LocalEndpoint).Port;
        return new PrometheusServer(obs, listener, $"http://{advertisedHost}:{actualPort}/metrics");
    }

    public void Dispose()
    {
        _cts.Cancel();
        try { _listener.Stop(); } catch { }
        try { _task.Wait(TimeSpan.FromSeconds(2)); } catch { }
        _cts.Dispose();
    }

    private async Task RunAsync()
    {
        while (!_cts.IsCancellationRequested)
        {
            TcpClient? client = null;
            try
            {
                client = await _listener.AcceptTcpClientAsync(_cts.Token).ConfigureAwait(false);
                _ = Task.Run(() => HandleAsync(client, _cts.Token));
            }
            catch (OperationCanceledException)
            {
                client?.Dispose();
                break;
            }
            catch
            {
                client?.Dispose();
                if (!_cts.IsCancellationRequested)
                    await Task.Delay(100, _cts.Token).ConfigureAwait(false);
            }
        }
    }

    private async Task HandleAsync(TcpClient client, CancellationToken ct)
    {
        using var ownedClient = client;
        try
        {
            await using var stream = ownedClient.GetStream();
            using var reader = new StreamReader(stream, Encoding.ASCII, leaveOpen: true);
            string? line;
            var requestLine = await reader.ReadLineAsync(ct).ConfigureAwait(false) ?? "";
            while (!string.IsNullOrEmpty(line = await reader.ReadLineAsync(ct).ConfigureAwait(false))) { }

            var ok = requestLine.StartsWith("GET /metrics ", StringComparison.Ordinal)
                || requestLine.StartsWith("GET / ", StringComparison.Ordinal);
            var status = ok ? "200 OK" : "404 Not Found";
            var body = ok ? PrometheusText.Render(_obs) : "not found\n";
            var bodyBytes = Encoding.UTF8.GetBytes(body);
            var header = Encoding.ASCII.GetBytes(
                "HTTP/1.1 " + status + "\r\n" +
                "Content-Type: text/plain; version=0.0.4; charset=utf-8\r\n" +
                "Content-Length: " + bodyBytes.Length.ToString(CultureInfo.InvariantCulture) + "\r\n" +
                "Connection: close\r\n\r\n");
            await stream.WriteAsync(header, ct).ConfigureAwait(false);
            await stream.WriteAsync(bodyBytes, ct).ConfigureAwait(false);
        }
        catch
        {
            // Best effort HTTP endpoint for local diagnostics.
        }
    }

    private static (IPAddress Ip, string AdvertisedHost, int Port) ParseBind(string bind)
    {
        var raw = string.IsNullOrWhiteSpace(bind) ? "127.0.0.1:0" : bind.Trim();
        if (raw.StartsWith("http://", StringComparison.OrdinalIgnoreCase))
        {
            var uri = new Uri(raw);
            raw = $"{uri.Host}:{uri.Port}";
        }

        var split = raw.LastIndexOf(':');
        var host = split > 0 ? raw[..split] : "127.0.0.1";
        var portRaw = split > 0 ? raw[(split + 1)..] : raw;
        var port = int.TryParse(portRaw, NumberStyles.None, CultureInfo.InvariantCulture, out var parsedPort)
            ? parsedPort
            : 0;
        var ip = host switch
        {
            "" or "0.0.0.0" or "*" => IPAddress.Any,
            "::" => IPAddress.IPv6Any,
            _ when IPAddress.TryParse(host, out var parsedIp) => parsedIp,
            _ => Dns.GetHostAddresses(host).First(),
        };
        var advertised = host switch
        {
            "" or "0.0.0.0" or "*" => "127.0.0.1",
            "::" => "::1",
            _ => host,
        };
        return (ip, advertised, port);
    }
}

internal sealed record MemberIdentity(string Slug, string InstanceUid);

public sealed class MemberRelay : IAsyncDisposable, IDisposable
{
    private static readonly TimeSpan RetryDelay = TimeSpan.FromSeconds(2);
    private readonly Observability _obs;
    private readonly string _memberSlug;
    private readonly string _address;
    private readonly GrpcChannel? _channel;
    private readonly MemberIdentity? _identity;
    private readonly Action<string> _logger;
    private readonly CancellationTokenSource _cts = new();
    private readonly List<Task> _tasks = new();

    public MemberRelay(Observability obs, string memberSlug, string address, Action<string>? logger = null)
    {
        _obs = obs;
        _memberSlug = memberSlug;
        _address = address;
        _logger = logger ?? (_ => { });
    }

    public MemberRelay(Observability obs, string memberSlug, string memberUid, GrpcChannel channel, Action<string>? logger = null)
    {
        _obs = obs;
        _memberSlug = memberSlug;
        _address = "";
        _channel = channel;
        _identity = new MemberIdentity(memberSlug, memberUid);
        _logger = logger ?? (_ => { });
    }

    public void Start()
    {
        if (_obs.Enabled(Family.Logs) && _obs.LogRing is not null)
            _tasks.Add(Task.Run(() => PumpLogsAsync(_cts.Token)));
        if (_obs.Enabled(Family.Events) && _obs.EventBus is not null)
            _tasks.Add(Task.Run(() => PumpEventsAsync(_cts.Token)));
    }

    public void Dispose()
    {
        DisposeAsync().AsTask().GetAwaiter().GetResult();
    }

    public async ValueTask DisposeAsync()
    {
        _cts.Cancel();
        try { await Task.WhenAll(_tasks).WaitAsync(TimeSpan.FromSeconds(3)).ConfigureAwait(false); } catch { }
        _cts.Dispose();
    }

    private async Task PumpLogsAsync(CancellationToken ct)
    {
        if (_channel is not null && _identity is not null)
        {
            await PumpLogsFromChannelAsync(_channel, _identity, ct).ConfigureAwait(false);
            return;
        }

        while (!ct.IsCancellationRequested)
        {
            GrpcChannel? channel = null;
            try
            {
                channel = global::Holons.Connect.ConnectTarget(_address, new global::Holons.Connect.ConnectOptions { Timeout = TimeSpan.FromSeconds(5), Start = false });
                var identity = await ResolveIdentityAsync(channel, ct).ConfigureAwait(false);
                await PumpLogsFromChannelAsync(channel, identity, ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                break;
            }
            catch (Exception error)
            {
                _logger($"member relay logs {_memberSlug}: {error.Message}");
            }
            finally
            {
                global::Holons.Connect.Disconnect(channel);
            }

            await RetryAsync(ct).ConfigureAwait(false);
        }
    }

    private async Task PumpEventsAsync(CancellationToken ct)
    {
        if (_channel is not null && _identity is not null)
        {
            await PumpEventsFromChannelAsync(_channel, _identity, ct).ConfigureAwait(false);
            return;
        }

        while (!ct.IsCancellationRequested)
        {
            GrpcChannel? channel = null;
            try
            {
                channel = global::Holons.Connect.ConnectTarget(_address, new global::Holons.Connect.ConnectOptions { Timeout = TimeSpan.FromSeconds(5), Start = false });
                var identity = await ResolveIdentityAsync(channel, ct).ConfigureAwait(false);
                await PumpEventsFromChannelAsync(channel, identity, ct).ConfigureAwait(false);
            }
            catch (OperationCanceledException) when (ct.IsCancellationRequested)
            {
                break;
            }
            catch (Exception error)
            {
                _logger($"member relay events {_memberSlug}: {error.Message}");
            }
            finally
            {
                global::Holons.Connect.Disconnect(channel);
            }

            await RetryAsync(ct).ConfigureAwait(false);
        }
    }

    private async Task<MemberIdentity> ResolveIdentityAsync(GrpcChannel channel, CancellationToken ct)
    {
        var client = new global::Holons.V1.HolonObservability.HolonObservabilityClient(channel);
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(5));
        using var call = client.Events(new global::Holons.V1.EventsRequest(), cancellationToken: timeout.Token);
        MemberIdentity? fallback = null;
        try
        {
            while (await call.ResponseStream.MoveNext(timeout.Token).ConfigureAwait(false))
            {
                var ev = call.ResponseStream.Current;
                var instanceUid = Wire.AttributeString(ev.Attributes, AttributeNames.HolonsInstanceUid);
                if (ev.EventName == EventNames.InstanceReady && !string.IsNullOrEmpty(instanceUid))
                {
                    var slug = Wire.AttributeString(ev.Attributes, AttributeNames.HolonsSlug);
                    var identity = new MemberIdentity(string.IsNullOrEmpty(slug) ? _memberSlug : slug, instanceUid);
                    if (ev.Chain.Count == 0)
                        return identity;
                    fallback ??= identity;
                }
            }
        }
        catch (OperationCanceledException) when (!ct.IsCancellationRequested)
        {
            // Fall back below when the member has no ready event yet.
        }

        return fallback ?? new MemberIdentity(_memberSlug, "");
    }

    private async Task PumpLogsFromChannelAsync(GrpcChannel channel, MemberIdentity identity, CancellationToken ct)
    {
        var client = new global::Holons.V1.HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Logs(
            new global::Holons.V1.LogsRequest
            {
                MinSeverityNumber = global::Holons.V1.SeverityNumber.Info,
                Follow = true,
            },
            cancellationToken: ct);
        while (await call.ResponseStream.MoveNext(ct).ConfigureAwait(false))
        {
            var entry = ObservabilityGrpcService.FromProtoLogRecord(call.ResponseStream.Current);
            _obs.LogRing?.Push(EnrichLog(entry, identity));
        }
    }

    private async Task PumpEventsFromChannelAsync(GrpcChannel channel, MemberIdentity identity, CancellationToken ct)
    {
        var client = new global::Holons.V1.HolonObservability.HolonObservabilityClient(channel);
        using var call = client.Events(
            new global::Holons.V1.EventsRequest { Follow = true },
            cancellationToken: ct);
        while (await call.ResponseStream.MoveNext(ct).ConfigureAwait(false))
        {
            var ev = ObservabilityGrpcService.FromProtoLogRecord(call.ResponseStream.Current);
            _obs.EventBus?.Emit(EnrichEvent(ev, identity));
        }
    }

    private static async Task RetryAsync(CancellationToken ct)
    {
        try { await Task.Delay(RetryDelay, ct).ConfigureAwait(false); }
        catch (OperationCanceledException) { }
    }

    private static LogRecord EnrichLog(LogRecord entry, MemberIdentity identity) => EnrichRecord(entry, identity);

    private static LogRecord EnrichEvent(LogRecord entry, MemberIdentity identity) => EnrichRecord(entry, identity);

    private static LogRecord EnrichRecord(LogRecord entry, MemberIdentity identity)
    {
        var record = entry.Record.Clone();
        record.Chain.Clear();
        record.Chain.Add(Chain.EnrichForMultilog(entry.Record.Chain, identity.Slug, identity.InstanceUid));
        return new LogRecord { Record = record, Private = entry.Private };
    }
}

public static class DiskWriters
{
    public static void Enable(string runDir)
    {
        var obs = ObservabilityRegistry.Current();
        if (obs == null || string.IsNullOrEmpty(runDir)) return;
        Directory.CreateDirectory(runDir);

        if (obs.Enabled(Family.Logs) && obs.LogRing != null)
        {
            var fp = Path.Combine(runDir, "stdout.log");
            obs.LogRing.Subscribe(e =>
            {
                var attrs = UserAttributes(e.Record.Attributes);
                var rec = new Dictionary<string, object?>
                {
                    ["kind"] = "log",
                    ["ts"] = e.Timestamp.ToString("o"),
                    ["level"] = e.Record.SeverityText,
                    ["slug"] = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsSlug),
                    ["instance_uid"] = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsInstanceUid),
                    ["message"] = Wire.AnyValueString(e.Record.Body),
                };
                var sessionId = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsSessionId);
                var rpcMethod = Wire.AttributeString(e.Record.Attributes, AttributeNames.RpcMethod);
                var caller = Wire.AttributeString(e.Record.Attributes, AttributeNames.CodeCaller);
                if (!string.IsNullOrEmpty(sessionId)) rec["session_id"] = sessionId;
                if (!string.IsNullOrEmpty(rpcMethod)) rec["rpc_method"] = rpcMethod;
                if (attrs.Count > 0) rec["fields"] = attrs;
                if (!string.IsNullOrEmpty(caller)) rec["caller"] = caller;
                if (e.Record.Chain.Count > 0) rec["chain"] = e.Record.Chain.ToArray();
                try { File.AppendAllText(fp, JsonSerializer.Serialize(rec) + "\n"); } catch { }
            });
        }

        if (obs.Enabled(Family.Events) && obs.EventBus != null)
        {
            var fp = Path.Combine(runDir, "events.jsonl");
            obs.EventBus.Subscribe(e =>
            {
                var attrs = UserAttributes(e.Record.Attributes);
                var rec = new Dictionary<string, object?>
                {
                    ["kind"] = "event",
                    ["ts"] = e.Timestamp.ToString("o"),
                    ["event_name"] = e.Record.EventName,
                    ["slug"] = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsSlug),
                    ["instance_uid"] = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsInstanceUid),
                };
                var sessionId = Wire.AttributeString(e.Record.Attributes, AttributeNames.HolonsSessionId);
                if (!string.IsNullOrEmpty(sessionId)) rec["session_id"] = sessionId;
                if (attrs.Count > 0) rec["payload"] = attrs;
                if (e.Record.Chain.Count > 0) rec["chain"] = e.Record.Chain.ToArray();
                try { File.AppendAllText(fp, JsonSerializer.Serialize(rec) + "\n"); } catch { }
            });
        }
    }

    private static Dictionary<string, string> UserAttributes(IEnumerable<global::Holons.V1.KeyValue> attributes)
    {
        var result = new Dictionary<string, string>();
        foreach (var attr in attributes)
        {
            if (IsSystemAttribute(attr.Key))
                continue;
            result[attr.Key] = Wire.AnyValueString(attr.Value);
        }
        return result;
    }

    private static bool IsSystemAttribute(string key) => key is
        AttributeNames.HolonsSlug or
        AttributeNames.ServiceName or
        AttributeNames.HolonsInstanceUid or
        AttributeNames.ServiceInstanceId or
        AttributeNames.HolonsSessionId or
        AttributeNames.HolonsTransport or
        AttributeNames.RpcMethod or
        AttributeNames.LoggerName or
        AttributeNames.CodeCaller;
}

public sealed class MetaJson
{
    public string Slug { get; set; } = "";
    public string Uid { get; set; } = "";
    public int Pid { get; set; }
    public DateTime StartedAt { get; set; }
    public string Mode { get; set; } = "persistent";
    public string Transport { get; set; } = "";
    public string Address { get; set; } = "";
    public string MetricsAddr { get; set; } = "";
    public string LogPath { get; set; } = "";
    public long LogBytesRotated { get; set; }
    public string OrganismUid { get; set; } = "";
    public string OrganismSlug { get; set; } = "";
    public bool IsDefault { get; set; }

    public static void Write(string runDir, MetaJson meta)
    {
        Directory.CreateDirectory(runDir);
        var p = Path.Combine(runDir, "meta.json");
        var tmp = p + ".tmp";
        var opts = new JsonSerializerOptions { WriteIndented = true };
        var dict = new Dictionary<string, object?>
        {
            ["slug"] = meta.Slug,
            ["uid"] = meta.Uid,
            ["pid"] = meta.Pid,
            ["started_at"] = meta.StartedAt.ToString("o"),
            ["mode"] = meta.Mode,
            ["transport"] = meta.Transport,
            ["address"] = meta.Address,
        };
        if (!string.IsNullOrEmpty(meta.MetricsAddr)) dict["metrics_addr"] = meta.MetricsAddr;
        if (!string.IsNullOrEmpty(meta.LogPath)) dict["log_path"] = meta.LogPath;
        if (meta.LogBytesRotated > 0) dict["log_bytes_rotated"] = meta.LogBytesRotated;
        if (!string.IsNullOrEmpty(meta.OrganismUid)) dict["organism_uid"] = meta.OrganismUid;
        if (!string.IsNullOrEmpty(meta.OrganismSlug)) dict["organism_slug"] = meta.OrganismSlug;
        if (meta.IsDefault) dict["default"] = true;
        File.WriteAllText(tmp, JsonSerializer.Serialize(dict, opts));
        File.Move(tmp, p, overwrite: true);
    }
}

// C# reference implementation of the cross-SDK observability layer.
// Mirrors sdk/go-holons/pkg/observability.

using System.Collections.Concurrent;
using System.Text;
using System.Text.Json;

namespace Holons.Observability;

public enum Family { Logs, Metrics, Events, Prom, Otel }

public enum Level
{
    Unset = 0, Trace = 1, Debug = 2, Info = 3, Warn = 4, Error = 5, Fatal = 6,
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

public enum EventType
{
    Unspecified = 0,
    InstanceSpawned = 1, InstanceReady = 2, InstanceExited = 3, InstanceCrashed = 4,
    SessionStarted = 5, SessionEnded = 6,
    HandlerPanic = 7, ConfigReloaded = 8,
}

public static class EventTypeExt
{
    public static string Name(this EventType t) => t switch
    {
        EventType.InstanceSpawned => "INSTANCE_SPAWNED",
        EventType.InstanceReady => "INSTANCE_READY",
        EventType.InstanceExited => "INSTANCE_EXITED",
        EventType.InstanceCrashed => "INSTANCE_CRASHED",
        EventType.SessionStarted => "SESSION_STARTED",
        EventType.SessionEnded => "SESSION_ENDED",
        EventType.HandlerPanic => "HANDLER_PANIC",
        EventType.ConfigReloaded => "CONFIG_RELOADED",
        _ => "UNSPECIFIED",
    };
}

public sealed class InvalidTokenException : Exception
{
    public string Token { get; }
    public InvalidTokenException(string token, string reason)
        : base($"OP_OBS: {reason}: {token}") { Token = token; }
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
            if (p == "otel") continue;
            if (!V1Tokens.Contains(p)) continue;
            if (p == "all")
            {
                families.Add(Family.Logs); families.Add(Family.Metrics);
                families.Add(Family.Events); families.Add(Family.Prom);
            }
            else if (Enum.TryParse<Family>(char.ToUpper(p[0]) + p[1..], out var f))
            {
                families.Add(f);
            }
        }
        return families;
    }

    public static void CheckEnv(IDictionary<string, string>? env = null)
    {
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
    public static List<Hop> AppendDirectChild(IEnumerable<Hop> src, string childSlug, string childUid)
    {
        var l = new List<Hop>(src);
        l.Add(new Hop(childSlug, childUid));
        return l;
    }

    public static List<Hop> EnrichForMultilog(IEnumerable<Hop> wire, string srcSlug, string srcUid)
        => AppendDirectChild(wire, srcSlug, srcUid);
}

public sealed class LogEntry
{
    public DateTime Timestamp { get; init; }
    public Level Level { get; init; }
    public string Slug { get; init; } = "";
    public string InstanceUid { get; init; } = "";
    public string SessionId { get; init; } = "";
    public string RpcMethod { get; init; } = "";
    public string Message { get; init; } = "";
    public Dictionary<string, string> Fields { get; init; } = new();
    public string Caller { get; init; } = "";
    public List<Hop> Chain { get; init; } = new();
}

public sealed class LogRing
{
    private readonly int _capacity;
    private readonly Queue<LogEntry> _buf;
    private readonly List<Action<LogEntry>> _subs = new();
    private readonly object _lock = new();

    public LogRing(int capacity = 1024)
    {
        _capacity = Math.Max(1, capacity);
        _buf = new Queue<LogEntry>(_capacity);
    }

    public void Push(LogEntry e)
    {
        List<Action<LogEntry>> copy;
        lock (_lock)
        {
            if (_buf.Count == _capacity) _buf.Dequeue();
            _buf.Enqueue(e);
            copy = new List<Action<LogEntry>>(_subs);
        }
        foreach (var s in copy) try { s(e); } catch { }
    }

    public List<LogEntry> Drain()
    {
        lock (_lock) return new List<LogEntry>(_buf);
    }

    public List<LogEntry> DrainSince(DateTime cutoff)
    {
        lock (_lock) return _buf.Where(e => e.Timestamp >= cutoff).ToList();
    }

    public IDisposable Subscribe(Action<LogEntry> fn)
    {
        lock (_lock) _subs.Add(fn);
        return new Sub(() => { lock (_lock) _subs.Remove(fn); });
    }

    public int Count { get { lock (_lock) return _buf.Count; } }

    private sealed class Sub : IDisposable
    {
        private readonly Action _dispose;
        public Sub(Action d) { _dispose = d; }
        public void Dispose() => _dispose();
    }
}

public sealed class Event
{
    public DateTime Timestamp { get; init; }
    public EventType Type { get; init; }
    public string Slug { get; init; } = "";
    public string InstanceUid { get; init; } = "";
    public string SessionId { get; init; } = "";
    public Dictionary<string, string> Payload { get; init; } = new();
    public List<Hop> Chain { get; init; } = new();
}

public sealed class EventBus
{
    private readonly int _capacity;
    private readonly Queue<Event> _buf;
    private readonly List<Action<Event>> _subs = new();
    private readonly object _lock = new();
    private bool _closed;

    public EventBus(int capacity = 256)
    {
        _capacity = Math.Max(1, capacity);
        _buf = new Queue<Event>(_capacity);
    }

    public void Emit(Event e)
    {
        List<Action<Event>> copy;
        lock (_lock)
        {
            if (_closed) return;
            if (_buf.Count == _capacity) _buf.Dequeue();
            _buf.Enqueue(e);
            copy = new List<Action<Event>>(_subs);
        }
        foreach (var s in copy) try { s(e); } catch { }
    }

    public List<Event> Drain() { lock (_lock) return new List<Event>(_buf); }
    public List<Event> DrainSince(DateTime cutoff) { lock (_lock) return _buf.Where(e => e.Timestamp >= cutoff).ToList(); }

    public IDisposable Subscribe(Action<Event> fn)
    {
        lock (_lock) _subs.Add(fn);
        return new Sub(() => { lock (_lock) _subs.Remove(fn); });
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
    public string PromAddr { get; init; } = "";
    public string[] RedactedFields { get; init; } = Array.Empty<string>();
    public int LogsRingSize { get; init; } = 1024;
    public int EventsRingSize { get; init; } = 256;
    public string RunDir { get; init; } = "";
    public string InstanceUid { get; init; } = "";
    public string OrganismUid { get; init; } = "";
    public string OrganismSlug { get; init; } = "";
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

    public void Log(Level lvl, string message, IDictionary<string, object?>? fields = null)
    {
        if (!Enabled(lvl)) return;
        var redact = new HashSet<string>(_obs.Config.RedactedFields);
        var outFields = new Dictionary<string, string>();
        if (fields != null)
        {
            foreach (var (k, v) in fields)
            {
                if (string.IsNullOrEmpty(k)) continue;
                outFields[k] = redact.Contains(k) ? "<redacted>" : (v?.ToString() ?? "");
            }
        }
        _obs.LogRing?.Push(new LogEntry
        {
            Timestamp = DateTime.UtcNow,
            Level = lvl,
            Slug = _obs.Config.Slug,
            InstanceUid = _obs.Config.InstanceUid,
            Message = message,
            Fields = outFields,
        });
    }

    public void Trace(string m, IDictionary<string, object?>? f = null) => Log(Level.Trace, m, f);
    public void Debug(string m, IDictionary<string, object?>? f = null) => Log(Level.Debug, m, f);
    public void Info(string m, IDictionary<string, object?>? f = null) => Log(Level.Info, m, f);
    public void Warn(string m, IDictionary<string, object?>? f = null) => Log(Level.Warn, m, f);
    public void Error(string m, IDictionary<string, object?>? f = null) => Log(Level.Error, m, f);
    public void Fatal(string m, IDictionary<string, object?>? f = null) => Log(Level.Fatal, m, f);
}

public sealed class Observability
{
    public ObsConfig Config { get; }
    public HashSet<Family> Families { get; }
    public LogRing? LogRing { get; }
    public EventBus? EventBus { get; }
    public Registry? Registry { get; }
    private readonly ConcurrentDictionary<string, Logger> _loggers = new();

    internal Observability(ObsConfig cfg, HashSet<Family> families)
    {
        Config = cfg;
        Families = families;
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

    public void Emit(EventType type, Dictionary<string, string>? payload = null)
    {
        if (EventBus is null) return;
        var redact = new HashSet<string>(Config.RedactedFields);
        var p = new Dictionary<string, string>();
        if (payload != null)
        {
            foreach (var (k, v) in payload)
                p[k] = redact.Contains(k) ? "<redacted>" : v;
        }
        EventBus.Emit(new Event
        {
            Timestamp = DateTime.UtcNow,
            Type = type,
            Slug = Config.Slug,
            InstanceUid = Config.InstanceUid,
            Payload = p,
        });
    }

    public void Close() => EventBus?.Close();

    internal static readonly Logger DisabledLogger = new(new Observability(new ObsConfig { DefaultLogLevel = Level.Fatal }, new HashSet<Family>()), "");
}

public static class ObservabilityRegistry
{
    private static Observability? _current;
    private static readonly object _lock = new();

    public static Observability Configure(ObsConfig cfg)
    {
        var families = Env.ParseOpObs(Environment.GetEnvironmentVariable("OP_OBS"));
        var normalized = cfg with { };
        if (string.IsNullOrEmpty(normalized.Slug))
            normalized = normalized with { Slug = AppDomain.CurrentDomain.FriendlyName };
        var obs = new Observability(normalized, families);
        lock (_lock) _current = obs;
        return obs;
    }

    public static Observability FromEnv(ObsConfig? baseCfg = null)
    {
        var b = baseCfg ?? new ObsConfig();
        static string Read(string k) => Environment.GetEnvironmentVariable(k) ?? "";
        var cfg = b with
        {
            InstanceUid = string.IsNullOrEmpty(b.InstanceUid) ? Read("OP_INSTANCE_UID") : b.InstanceUid,
            OrganismUid = string.IsNullOrEmpty(b.OrganismUid) ? Read("OP_ORGANISM_UID") : b.OrganismUid,
            OrganismSlug = string.IsNullOrEmpty(b.OrganismSlug) ? Read("OP_ORGANISM_SLUG") : b.OrganismSlug,
            PromAddr = string.IsNullOrEmpty(b.PromAddr) ? Read("OP_PROM_ADDR") : b.PromAddr,
            RunDir = string.IsNullOrEmpty(b.RunDir) ? Read("OP_RUN_DIR") : b.RunDir,
        };
        return Configure(cfg);
    }

    public static Observability Current()
    {
        lock (_lock) { return _current ?? new Observability(new ObsConfig { DefaultLogLevel = Level.Fatal }, new HashSet<Family>()); }
    }

    public static void Reset()
    {
        lock (_lock) { _current?.Close(); _current = null; }
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
                var rec = new Dictionary<string, object?>
                {
                    ["kind"] = "log",
                    ["ts"] = e.Timestamp.ToString("o"),
                    ["level"] = e.Level.Name(),
                    ["slug"] = e.Slug,
                    ["instance_uid"] = e.InstanceUid,
                    ["message"] = e.Message,
                };
                if (!string.IsNullOrEmpty(e.SessionId)) rec["session_id"] = e.SessionId;
                if (!string.IsNullOrEmpty(e.RpcMethod)) rec["rpc_method"] = e.RpcMethod;
                if (e.Fields.Count > 0) rec["fields"] = e.Fields;
                if (!string.IsNullOrEmpty(e.Caller)) rec["caller"] = e.Caller;
                if (e.Chain.Count > 0)
                    rec["chain"] = e.Chain.Select(h => new { slug = h.Slug, instance_uid = h.InstanceUid }).ToArray();
                try { File.AppendAllText(fp, JsonSerializer.Serialize(rec) + "\n"); } catch { }
            });
        }

        if (obs.Enabled(Family.Events) && obs.EventBus != null)
        {
            var fp = Path.Combine(runDir, "events.jsonl");
            obs.EventBus.Subscribe(e =>
            {
                var rec = new Dictionary<string, object?>
                {
                    ["kind"] = "event",
                    ["ts"] = e.Timestamp.ToString("o"),
                    ["type"] = e.Type.Name(),
                    ["slug"] = e.Slug,
                    ["instance_uid"] = e.InstanceUid,
                };
                if (!string.IsNullOrEmpty(e.SessionId)) rec["session_id"] = e.SessionId;
                if (e.Payload.Count > 0) rec["payload"] = e.Payload;
                if (e.Chain.Count > 0)
                    rec["chain"] = e.Chain.Select(h => new { slug = h.Slug, instance_uid = h.InstanceUid }).ToArray();
                try { File.AppendAllText(fp, JsonSerializer.Serialize(rec) + "\n"); } catch { }
            });
        }
    }
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

using System.Collections.Concurrent;
using System.Diagnostics;
using System.Runtime.InteropServices;
using Grpc.Net.Client;
using Holons;

namespace Greeting.Godotnet.Services;

internal sealed record GreetingDaemonIdentity(
    string Slug,
    string FamilyName,
    string BinaryName,
    string BuildRunner,
    string BinaryPath)
{
    public const string BinaryPrefix = "gudule-daemon-greeting-";

    public static GreetingDaemonIdentity? FromBinaryPath(string path) =>
        FromBinaryName(Path.GetFileName(path), path);

    public static GreetingDaemonIdentity? FromBinaryName(string binaryName, string binaryPath)
    {
        var normalized = binaryName.EndsWith(".exe", StringComparison.OrdinalIgnoreCase)
            ? binaryName[..^4]
            : binaryName;
        if (!normalized.StartsWith(BinaryPrefix, StringComparison.Ordinal))
            return null;

        var variant = normalized[BinaryPrefix.Length..];
        return new GreetingDaemonIdentity(
            Slug: $"gudule-greeting-daemon-{variant}",
            FamilyName: $"Greeting-Daemon-{DisplayVariant(variant)}",
            BinaryName: normalized,
            BuildRunner: BuildRunnerFor(variant),
            BinaryPath: Path.GetFullPath(binaryPath));
    }

    private static string DisplayVariant(string variant)
    {
        return string.Join(
            "-",
            variant.Split('-', StringSplitOptions.RemoveEmptyEntries)
                .Select(token => token switch
                {
                    "cpp" => "CPP",
                    "js" => "JS",
                    "qt" => "Qt",
                    _ => char.ToUpperInvariant(token[0]) + token[1..],
                }));
    }

    private static string BuildRunnerFor(string variant) => variant switch
    {
        "go" => "go-module",
        "rust" => "cargo",
        "swift" => "swift-package",
        "kotlin" => "gradle",
        "dart" => "dart",
        "python" => "python",
        "csharp" => "dotnet",
        "node" => "npm",
        _ => "go-module",
    };
}

internal sealed class BundledDaemonSession : IAsyncDisposable
{
    private readonly Process _process;

    public BundledDaemonSession(Process process)
    {
        _process = process;
    }

    public async ValueTask DisposeAsync()
    {
        if (_process.HasExited)
        {
            _process.Dispose();
            return;
        }

        try
        {
            _process.Kill(entireProcessTree: true);
        }
        catch (InvalidOperationException)
        {
        }

        await _process.WaitForExitAsync();
        _process.Dispose();
    }
}

public sealed class DaemonProcess : IAsyncDisposable
{
    private GrpcChannel? _channel;
    private GreetingClient? _client;
    private GreetingDaemonIdentity? _daemon;
    private string? _stageRoot;
    private BundledDaemonSession? _daemonSession;

    public string HolonSlug => _daemon?.Slug ?? string.Empty;

    public GreetingClient Client =>
        _client ?? throw new InvalidOperationException("Daemon is not connected.");

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        if (_client is not null)
        {
            return;
        }

        AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);

        var daemon = await ResolveDaemonAsync(cancellationToken);
        var stageRoot = await StageHolonRootAsync(daemon, cancellationToken);
        var portFile = PortFilePath(stageRoot, daemon.Slug);
        var session = await StartBundledDaemonAsync(daemon, portFile, cancellationToken);

        _daemon = daemon;
        _stageRoot = stageRoot;
        _daemonSession = session;

        var previousDirectory = Directory.GetCurrentDirectory();
        try
        {
            cancellationToken.ThrowIfCancellationRequested();
            Directory.SetCurrentDirectory(stageRoot);
            _channel = Connect.ConnectTarget(
                daemon.Slug,
                new Connect.ConnectOptions
                {
                    Transport = "tcp",
                    Start = false,
                    PortFile = portFile,
                });
            _client = new GreetingClient(_channel);
        }
        catch
        {
            if (_channel is not null)
            {
                Connect.Disconnect(_channel);
                _channel = null;
            }

            _client = null;
            _daemon = null;
            _stageRoot = null;
            if (_daemonSession is not null)
            {
                await _daemonSession.DisposeAsync();
                _daemonSession = null;
            }
            await DeleteStageRootAsync(stageRoot);
            throw;
        }
        finally
        {
            Directory.SetCurrentDirectory(previousDirectory);
        }
    }

    public async ValueTask DisposeAsync()
    {
        _client = null;
        _daemon = null;
        var root = _stageRoot;
        _stageRoot = null;
        var session = _daemonSession;
        _daemonSession = null;

        try
        {
            if (_channel is not null)
            {
                Connect.Disconnect(_channel);
                _channel = null;
            }
        }
        finally
        {
            if (session is not null)
            {
                await session.DisposeAsync();
            }

            if (root is not null)
            {
                await DeleteStageRootAsync(root);
            }
        }
    }

    private async Task<GreetingDaemonIdentity> ResolveDaemonAsync(CancellationToken cancellationToken)
    {
        foreach (var candidate in DaemonCandidates())
        {
            cancellationToken.ThrowIfCancellationRequested();
            var daemon = GreetingDaemonIdentity.FromBinaryPath(candidate);
            if (daemon is null || !File.Exists(daemon.BinaryPath))
                continue;

            await EnsureExecutableAsync(daemon.BinaryPath, cancellationToken);
            return daemon;
        }

        throw new FileNotFoundException($"Unable to locate any {GreetingDaemonIdentity.BinaryPrefix}* binary.");
    }

    private IEnumerable<string> DaemonCandidates()
    {
        var seen = new HashSet<string>(StringComparer.Ordinal);

        foreach (var directory in CandidateDirectories())
        {
            if (!Directory.Exists(directory))
                continue;

            if (Path.GetFileName(directory).Equals("daemons", StringComparison.Ordinal))
            {
                foreach (var candidate in SourceTreeDaemonCandidates(directory))
                {
                    var normalized = Path.GetFullPath(candidate);
                    if (seen.Add(normalized))
                        yield return normalized;
                }
                continue;
            }

            foreach (var candidate in Directory.EnumerateFiles(directory, $"{GreetingDaemonIdentity.BinaryPrefix}*"))
            {
                var normalized = Path.GetFullPath(candidate);
                if (seen.Add(normalized))
                    yield return normalized;
            }
        }
    }

    private IEnumerable<string> CandidateDirectories()
    {
        yield return Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), "build"));
        yield return Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), "..", "build"));
        yield return Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "daemons"));

        var baseDirectory = Path.GetFullPath(AppContext.BaseDirectory);
        yield return baseDirectory;
        yield return Path.Combine(baseDirectory, "daemon");
        yield return Path.GetFullPath(Path.Combine(baseDirectory, "..", "Resources"));
        yield return Path.GetFullPath(Path.Combine(baseDirectory, "..", "Resources", "daemon"));
    }

    private IEnumerable<string> SourceTreeDaemonCandidates(string daemonsDir)
    {
        foreach (var daemonDir in Directory.EnumerateDirectories(daemonsDir, $"{GreetingDaemonIdentity.BinaryPrefix}*"))
        {
            var binaryName = Path.GetFileName(daemonDir);
            yield return Path.Combine(daemonDir, ".op", "build", "bin", binaryName);
            yield return Path.Combine(daemonDir, binaryName);
        }
    }

    private static async Task EnsureExecutableAsync(string path, CancellationToken cancellationToken)
    {
        if (RuntimeInformation.IsOSPlatform(OSPlatform.Windows))
        {
            return;
        }

        var chmod = new ProcessStartInfo
        {
            FileName = "/bin/chmod",
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        chmod.ArgumentList.Add("+x");
        chmod.ArgumentList.Add(path);

        using var process = Process.Start(chmod) ?? throw new InvalidOperationException("Unable to invoke chmod.");
        await process.WaitForExitAsync(cancellationToken);
    }

    private async Task<string> StageHolonRootAsync(GreetingDaemonIdentity daemon, CancellationToken cancellationToken)
    {
        var root = Path.Combine(
            FileSystem.Current.CacheDirectory,
            "holons-runtime",
            daemon.Slug,
            Guid.NewGuid().ToString("N"));
        var holonDir = Path.Combine(root, "holons", daemon.Slug);
        Directory.CreateDirectory(holonDir);
        await File.WriteAllTextAsync(
            Path.Combine(holonDir, "holon.yaml"),
            BuildManifest(daemon),
            cancellationToken);
        return root;
    }

    private static string PortFilePath(string root, string daemonSlug) =>
        Path.Combine(root, ".op", "run", $"{daemonSlug}.port");

    private async Task<BundledDaemonSession> StartBundledDaemonAsync(
        GreetingDaemonIdentity daemon,
        string portFile,
        CancellationToken cancellationToken)
    {
        var recentLines = new ConcurrentQueue<string>();
        var listenUri = new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = daemon.BinaryPath,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
            },
            EnableRaisingEvents = true,
        };
        process.StartInfo.ArgumentList.Add("serve");
        process.StartInfo.ArgumentList.Add("--listen");
        process.StartInfo.ArgumentList.Add("tcp://127.0.0.1:0");

        void OnLine(string? line)
        {
            if (string.IsNullOrWhiteSpace(line))
                return;

            recentLines.Enqueue(line);
            while (recentLines.Count > 8 && recentLines.TryDequeue(out _))
            {
            }

            var uri = FirstUri(line);
            if (!string.IsNullOrWhiteSpace(uri))
            {
                listenUri.TrySetResult(uri);
            }
        }

        process.OutputDataReceived += (_, args) => OnLine(args.Data);
        process.ErrorDataReceived += (_, args) => OnLine(args.Data);
        process.Exited += (_, _) =>
        {
            if (listenUri.Task.IsCompleted)
                return;

            var details = recentLines.IsEmpty
                ? string.Empty
                : $": {string.Join(" | ", recentLines)}";
            listenUri.TrySetException(new InvalidOperationException(
                $"Bundled daemon exited before advertising an address{details}"));
        };

        if (!process.Start())
            throw new InvalidOperationException($"Unable to launch {daemon.BinaryName}.");

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(5));
        using var registration = timeout.Token.Register(() =>
            listenUri.TrySetException(new TimeoutException("Bundled daemon did not advertise a tcp:// address.")));

        var uri = await listenUri.Task;
        Directory.CreateDirectory(Path.GetDirectoryName(portFile)!);
        await File.WriteAllTextAsync(portFile, $"{uri.Trim()}{Environment.NewLine}", cancellationToken);
        return new BundledDaemonSession(process);
    }

    private string BuildManifest(GreetingDaemonIdentity daemon)
    {
        var escapedPath = daemon.BinaryPath.Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal);
        return $"""
schema: holon/v0
uuid: "e2d57b4e-2360-4d3d-b080-ac6dd73184dd"
given_name: gudule
family_name: "{daemon.FamilyName}"
motto: Greets users in 56 languages through the bundled daemon.
composer: Codex
clade: deterministic/pure
status: draft
born: "2026-03-12"
generated_by: manual
kind: native
build:
  runner: {daemon.BuildRunner}
artifacts:
  binary: "{escapedPath}"
""" + Environment.NewLine;
    }

    private static string? FirstUri(string line)
    {
        var marker = "tcp://";
        var index = line.IndexOf(marker, StringComparison.Ordinal);
        if (index < 0)
            return null;

        var tail = line[index..];
        var end = tail.IndexOfAny([' ', '\t', '\r', '\n']);
        return end >= 0 ? tail[..end] : tail;
    }

    private static Task DeleteStageRootAsync(string root)
    {
        if (!Directory.Exists(root))
        {
            return Task.CompletedTask;
        }

        return Task.Run(() => Directory.Delete(root, recursive: true));
    }
}

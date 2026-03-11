using System.Diagnostics;
using System.Runtime.InteropServices;
using Grpc.Net.Client;
using Holons;

namespace Greeting.Godotnet.Services;

public sealed class DaemonProcess : IAsyncDisposable
{
    private const string PackagedHolonSlug = "greeting-daemon";
    private const string PackagedBinaryName = "gudule-greeting-daemon";
    private const string DevBinaryName = "gudule-daemon-greeting-go";
    private GrpcChannel? _channel;
    private GreetingClient? _client;
    private string? _stageRoot;

    public string HolonSlug => DaemonTargetOverride() ?? PackagedHolonSlug;

    public GreetingClient Client =>
        _client ?? throw new InvalidOperationException("Daemon is not connected.");

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        if (_client is not null)
        {
            return;
        }

        AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);

        var overrideTarget = DaemonTargetOverride();
        if (overrideTarget is not null)
        {
            _channel = Connect.ConnectTarget(overrideTarget);
            _client = new GreetingClient(_channel);
            return;
        }

        var binaryPath = await ResolveBinaryPathAsync(cancellationToken);
        var stageRoot = await StageHolonRootAsync(binaryPath, cancellationToken);
        _stageRoot = stageRoot;

        var previousDirectory = Directory.GetCurrentDirectory();
        try
        {
            cancellationToken.ThrowIfCancellationRequested();
            Directory.SetCurrentDirectory(stageRoot);
            _channel = Connect.ConnectTarget(PackagedHolonSlug);
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
            _stageRoot = null;
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
        var root = _stageRoot;
        _stageRoot = null;

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
            if (root is not null)
            {
                await DeleteStageRootAsync(root);
            }
        }
    }

    private async Task<string> ResolveBinaryPathAsync(CancellationToken cancellationToken)
    {
        var packagedPath = await TryResolvePackagedBinaryAsync(cancellationToken);
        if (packagedPath is not null)
        {
            return packagedPath;
        }

        var candidates = new[]
        {
            Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), PackagedBinaryNameWithHostSuffix())),
            Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "daemons", "greeting-daemon-go", ".op", "build", "bin", DevBinaryNameWithHostSuffix())),
            Path.GetFullPath(Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "daemons", "greeting-daemon-go", DevBinaryNameWithHostSuffix())),
            Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, PackagedBinaryNameWithHostSuffix())),
        };
        foreach (var candidate in candidates)
        {
            if (File.Exists(candidate))
            {
                return candidate;
            }
        }

        throw new FileNotFoundException($"Unable to locate {PackagedBinaryNameWithHostSuffix()} in the app bundle or source tree.");
    }

    private async Task<string?> TryResolvePackagedBinaryAsync(CancellationToken cancellationToken)
    {
        var binaryName = PackagedBinaryNameWithHostSuffix();
        var cachePath = Path.Combine(FileSystem.Current.CacheDirectory, binaryName);
        if (File.Exists(cachePath))
        {
            return cachePath;
        }

        try
        {
            await using var packageStream = await FileSystem.Current.OpenAppPackageFileAsync(binaryName);
            await using var output = File.Create(cachePath);
            await packageStream.CopyToAsync(output, cancellationToken);
            await output.FlushAsync(cancellationToken);
            await MakeExecutableAsync(cachePath, cancellationToken);
            return cachePath;
        }
        catch (FileNotFoundException)
        {
            return null;
        }
    }

    private static async Task MakeExecutableAsync(string path, CancellationToken cancellationToken)
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

    private async Task<string> StageHolonRootAsync(string binaryPath, CancellationToken cancellationToken)
    {
        var root = Path.Combine(
            FileSystem.Current.CacheDirectory,
            "holons-runtime",
            PackagedHolonSlug);
        var holonDir = Path.Combine(root, "holons", PackagedHolonSlug);
        Directory.CreateDirectory(holonDir);
        await File.WriteAllTextAsync(
            Path.Combine(holonDir, "holon.yaml"),
            BuildManifest(binaryPath),
            cancellationToken);
        return root;
    }

    private string BuildManifest(string binaryPath)
    {
        var escapedPath = binaryPath.Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal);
        return $"""
schema: holon/v0
uuid: "6492d55a-55b8-4ecb-a406-2a2a401f7c01"
given_name: greeting
family_name: "daemon"
motto: Packaged greeting daemon fallback.
composer: Codex
clade: deterministic/pure
status: draft
born: "2026-03-11"
generated_by: codex
kind: native
build:
  runner: recipe
artifacts:
  binary: "{escapedPath}"
""" + Environment.NewLine;
    }

    private static string? DaemonTargetOverride()
    {
        var value = Environment.GetEnvironmentVariable("GUDULE_DAEMON_TARGET");
        return string.IsNullOrWhiteSpace(value) ? null : value.Trim();
    }

    private static string PackagedBinaryNameWithHostSuffix() =>
        OperatingSystem.IsWindows() ? $"{PackagedBinaryName}.exe" : PackagedBinaryName;

    private static string DevBinaryNameWithHostSuffix() =>
        OperatingSystem.IsWindows() ? $"{DevBinaryName}.exe" : DevBinaryName;

    private static Task DeleteStageRootAsync(string root)
    {
        if (!Directory.Exists(root))
        {
            return Task.CompletedTask;
        }

        return Task.Run(() => Directory.Delete(root, recursive: true));
    }
}



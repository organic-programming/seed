using System.Diagnostics;
using System.Runtime.InteropServices;
using Grpc.Net.Client;
using Holons;

namespace Greeting.Godotnet.Services;

public sealed class DaemonProcess : IAsyncDisposable
{
    private readonly string _binaryName;
    private GrpcChannel? _channel;
    private GreetingClient? _client;
    private string? _stageRoot;

    public DaemonProcess(string binaryName)
    {
        _binaryName = binaryName;
    }

    public string HolonSlug => "greeting-daemon-greeting-godotnet";

    public GreetingClient Client =>
        _client ?? throw new InvalidOperationException("Daemon is not connected.");

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        if (_client is not null)
        {
            return;
        }

        AppContext.SetSwitch("System.Net.Http.SocketsHttpHandler.Http2UnencryptedSupport", true);
        var binaryPath = await ResolveBinaryPathAsync(cancellationToken);
        var stageRoot = await StageHolonRootAsync(binaryPath, cancellationToken);
        _stageRoot = stageRoot;

        var previousDirectory = Directory.GetCurrentDirectory();
        try
        {
            cancellationToken.ThrowIfCancellationRequested();
            Directory.SetCurrentDirectory(stageRoot);
            _channel = Connect.ConnectTarget(HolonSlug);
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

        var devPath = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "greeting-daemon", _binaryName));
        if (File.Exists(devPath))
        {
            return devPath;
        }

        throw new FileNotFoundException($"Unable to locate {_binaryName} in the app bundle or source tree.");
    }

    private async Task<string?> TryResolvePackagedBinaryAsync(CancellationToken cancellationToken)
    {
        var cachePath = Path.Combine(FileSystem.Current.CacheDirectory, _binaryName);
        if (File.Exists(cachePath))
        {
            return cachePath;
        }

        try
        {
            await using var packageStream = await FileSystem.Current.OpenAppPackageFileAsync(_binaryName);
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
            HolonSlug);
        var holonDir = Path.Combine(root, "holons", HolonSlug);
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
uuid: "1a409a1e-69e3-4846-9f9b-47b0a6f98f84"
given_name: greeting-daemon
family_name: "Greeting-Godotnet"
motto: Greets users in 56 languages — a Go + .NET recipe example.
composer: Codex
clade: deterministic/pure
status: draft
born: "2026-03-06"
generated_by: manual
kind: native
build:
  runner: go-module
artifacts:
  binary: "{escapedPath}"
""" + Environment.NewLine;
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





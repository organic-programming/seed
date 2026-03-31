using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Text;
using Grpc.Net.Client;
using Holons.V1;

namespace Holons;

/// <summary>Resolve holons to ready-to-use gRPC channels.</summary>
public static class Connector
{
    public static async Task<ConnectResult> Connect(
        int scope,
        string expression,
        string? root,
        int specifiers,
        int timeout)
    {
        if (scope != Discovery.LOCAL)
            return new ConnectResult(null, string.Empty, null, $"scope {scope} not supported");

        var target = (expression ?? string.Empty).Trim();
        if (target.Length == 0)
            return new ConnectResult(null, string.Empty, null, "expression is required");

        var resolved = Discovery.Resolve(scope, target, root, specifiers, timeout);
        if (resolved.Error is not null)
            return new ConnectResult(null, string.Empty, resolved.Ref, resolved.Error);
        if (resolved.Ref is null)
            return new ConnectResult(null, string.Empty, null, $"holon \"{target}\" not found");
        if (resolved.Ref.Error is not null)
            return new ConnectResult(null, string.Empty, resolved.Ref, resolved.Ref.Error);

        try
        {
            return await ConnectResolvedAsync(resolved.Ref, timeout).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            return new ConnectResult(null, string.Empty, resolved.Ref, ex.Message.Length == 0 ? "target unreachable" : ex.Message);
        }
    }

    public static void Disconnect(ConnectResult result)
    {
        if (result.Channel is GrpcChannel channel)
            ConnectionInternals.DisconnectChannel(channel);
    }

    private static async Task<ConnectResult> ConnectResolvedAsync(HolonRef reference, int timeout)
    {
        var scheme = ConnectionInternals.UrlScheme(reference.Url);
        if (scheme is "tcp" or "unix")
        {
            var dialed = await ConnectionInternals.DialReadyAsync(reference.Url, ConnectionInternals.ConnectTimeout(timeout))
                .ConfigureAwait(false);
            ConnectionInternals.RememberChannel(dialed.Channel, null, dialed.Resource, ephemeral: false);
            return new ConnectResult(dialed.Channel, string.Empty, reference, null);
        }

        if (scheme != "file")
            throw new InvalidOperationException($"unsupported target URL \"{reference.Url}\"");

        var launchTarget = ConnectionInternals.LaunchTargetFromRef(reference);
        var started = await ConnectionInternals.StartTcpHolonAsync(launchTarget, ConnectionInternals.ConnectTimeout(timeout))
            .ConfigureAwait(false);
        ConnectionInternals.RememberChannel(started.Channel, started.Process, started.Resource, ephemeral: true);
        return new ConnectResult(started.Channel, string.Empty, reference, null);
    }
}

/// <summary>Compatibility surface for the legacy ConnectTarget API.</summary>
public static class Connect
{
    public sealed record ConnectOptions
    {
        public TimeSpan Timeout { get; init; } = TimeSpan.FromSeconds(5);
        public string Transport { get; init; } = "stdio";
        public bool Start { get; init; } = true;
        public string PortFile { get; init; } = "";
        public string? Root { get; init; }
        public int Scope { get; init; } = Discovery.LOCAL;
        public int Specifiers { get; init; } = Discovery.ALL;
    }

    public static GrpcChannel ConnectTarget(string target) =>
        ConnectTarget(target, new ConnectOptions());

    public static GrpcChannel ConnectTarget(string target, ConnectOptions? options)
    {
        var resolvedOptions = options ?? new ConnectOptions();
        var timeout = resolvedOptions.Timeout > TimeSpan.Zero
            ? resolvedOptions.Timeout
            : TimeSpan.FromSeconds(5);
        var trimmed = (target ?? string.Empty).Trim();
        if (trimmed.Length == 0)
            throw new ArgumentException("target is required", nameof(target));

        if (ConnectionInternals.IsDirectTarget(trimmed))
        {
            var dialed = ConnectionInternals.DialReadyAsync(trimmed, timeout).GetAwaiter().GetResult();
            ConnectionInternals.RememberChannel(dialed.Channel, null, dialed.Resource, ephemeral: false);
            return dialed.Channel;
        }

        var transport = (resolvedOptions.Transport ?? "stdio").Trim().ToLowerInvariant();
        if (transport is not "stdio" and not "tcp" and not "unix")
            throw new ArgumentException($"unsupported transport \"{resolvedOptions.Transport}\"", nameof(options));

        var resolveResult = Discovery.Resolve(
            resolvedOptions.Scope,
            trimmed,
            resolvedOptions.Root,
            resolvedOptions.Specifiers,
            ConnectionInternals.TimeoutMilliseconds(timeout));
        if (resolveResult.Error is not null)
            throw new InvalidOperationException(resolveResult.Error);
        if (resolveResult.Ref is null)
            throw new InvalidOperationException($"holon \"{trimmed}\" not found");

        var launchTarget = ConnectionInternals.LaunchTargetFromRef(resolveResult.Ref);
        if (transport == "stdio")
        {
            var started = ConnectionInternals.StartStdioHolonAsync(launchTarget, timeout).GetAwaiter().GetResult();
            ConnectionInternals.RememberChannel(started.Channel, started.Process, started.Resource, ephemeral: true);
            return started.Channel;
        }

        var portFile = string.IsNullOrWhiteSpace(resolvedOptions.PortFile)
            ? ConnectionInternals.DefaultPortFilePath(resolveResult.Ref.Info?.Slug ?? trimmed)
            : resolvedOptions.PortFile.Trim();
        var reusable = ConnectionInternals.UsablePortFileAsync(portFile, timeout < TimeSpan.FromSeconds(1) ? timeout : TimeSpan.FromSeconds(1))
            .GetAwaiter().GetResult();
        if (reusable is not null)
        {
            ConnectionInternals.RememberChannel(reusable.Channel, null, reusable.Resource, ephemeral: false);
            return reusable.Channel;
        }

        if (!resolvedOptions.Start)
            throw new InvalidOperationException($"holon \"{trimmed}\" is not running");

        if (transport == "unix")
        {
            var started = ConnectionInternals.StartUnixHolonAsync(
                launchTarget,
                resolveResult.Ref.Info?.Slug ?? trimmed,
                portFile,
                timeout).GetAwaiter().GetResult();
            ConnectionInternals.WritePortFile(portFile, started.PublicUri);
            ConnectionInternals.RememberChannel(started.Channel, started.Process, started.Resource, ephemeral: false);
            return started.Channel;
        }

        {
            var started = ConnectionInternals.StartTcpHolonAsync(launchTarget, timeout).GetAwaiter().GetResult();
            ConnectionInternals.WritePortFile(portFile, started.PublicUri);
            ConnectionInternals.RememberChannel(started.Channel, started.Process, started.Resource, ephemeral: false);
            return started.Channel;
        }
    }

    public static void Disconnect(GrpcChannel? channel)
    {
        if (channel is not null)
            ConnectionInternals.DisconnectChannel(channel);
    }
}

internal sealed record LaunchTarget(string CommandPath, IReadOnlyList<string> Arguments, string? WorkingDirectory);

internal sealed record DialedChannel(GrpcChannel Channel, IDisposable? Resource = null);

internal sealed record StartedConnection(GrpcChannel Channel, Process Process, IDisposable? Resource = null, string PublicUri = "");

internal sealed record StartedHandle(Process? Process, IDisposable? Resource, bool Ephemeral);

internal static class ConnectionInternals
{
    private static readonly Dictionary<GrpcChannel, StartedHandle> Started =
        new(ReferenceEqualityComparer.Instance);

    internal static TimeSpan ConnectTimeout(int timeout) =>
        timeout <= 0 ? TimeSpan.FromSeconds(5) : TimeSpan.FromMilliseconds(Math.Max(timeout, 1));

    internal static int TimeoutMilliseconds(TimeSpan timeout) =>
        timeout == Timeout.InfiniteTimeSpan
            ? Discovery.NO_TIMEOUT
            : timeout <= TimeSpan.Zero
            ? 1
            : (int)Math.Clamp(timeout.TotalMilliseconds, 1, int.MaxValue);

    internal static bool IsDirectTarget(string target) =>
        target.Contains("://", StringComparison.Ordinal) || IsHostPortTarget(target);

    internal static string UrlScheme(string rawUrl)
    {
        if (!rawUrl.Contains("://", StringComparison.Ordinal))
            return string.Empty;
        return new Uri(rawUrl, UriKind.Absolute).Scheme.ToLowerInvariant();
    }

    internal static async Task<DialedChannel> DialReadyAsync(string target, TimeSpan timeout)
    {
        var normalized = NormalizeDialTarget(target);
        if (normalized.StartsWith("unix://", StringComparison.Ordinal))
        {
            using var cts = new CancellationTokenSource(timeout);
            await WaitForUnixReadyAsync(normalized, timeout, cts.Token).ConfigureAwait(false);
            var bridge = new UnixBridge(normalized);
            var address = BuildChannelAddress(NormalizeDialTarget(bridge.Uri));
            return new DialedChannel(GrpcChannel.ForAddress(address), bridge);
        }

        if (normalized.StartsWith("tcp://", StringComparison.Ordinal))
            normalized = NormalizeDialTarget(normalized);

        if (normalized.Contains("://", StringComparison.Ordinal))
            return new DialedChannel(GrpcChannel.ForAddress(normalized));

        using (var cts = new CancellationTokenSource(timeout))
        {
            await WaitForTcpReadyAsync(normalized, timeout, cts.Token).ConfigureAwait(false);
        }

        return new DialedChannel(GrpcChannel.ForAddress(BuildChannelAddress(normalized)));
    }

    internal static async Task<DialedChannel?> UsablePortFileAsync(string portFile, TimeSpan timeout)
    {
        try
        {
            var raw = (await File.ReadAllTextAsync(portFile).ConfigureAwait(false)).Trim();
            if (raw.Length == 0)
            {
                File.Delete(portFile);
                return null;
            }

            return await DialReadyAsync(raw, timeout).ConfigureAwait(false);
        }
        catch
        {
            try
            {
                if (File.Exists(portFile))
                    File.Delete(portFile);
            }
            catch
            {
                // Best effort.
            }
            return null;
        }
    }

    internal static void RememberChannel(GrpcChannel channel, Process? process, IDisposable? resource, bool ephemeral)
    {
        lock (Started)
        {
            Started[channel] = new StartedHandle(process, resource, ephemeral);
        }
    }

    internal static void DisconnectChannel(GrpcChannel channel)
    {
        StartedHandle? handle = null;
        lock (Started)
        {
            if (Started.TryGetValue(channel, out var found))
            {
                handle = found;
                Started.Remove(channel);
            }
        }

        channel.Dispose();
        try
        {
            handle?.Resource?.Dispose();
        }
        catch
        {
            // Best effort.
        }

        if (handle is not null && handle.Ephemeral)
            StopProcess(handle.Process);
    }

    internal static HolonInfo DescribePackageDirectory(string packageDir, int timeout) =>
        DescribeBinaryTarget(PackageBinaryPath(packageDir), timeout);

    internal static HolonInfo DescribeBinaryTarget(string binaryPath, int timeout)
    {
        var launchTarget = new LaunchTarget(binaryPath, Array.Empty<string>(), Path.GetDirectoryName(binaryPath));
        try
        {
            var started = StartTcpHolonAsync(launchTarget, ConnectTimeout(timeout)).GetAwaiter().GetResult();
            try
            {
                return DescribeChannel(started.Channel, timeout);
            }
            finally
            {
                started.Resource?.Dispose();
                started.Channel.Dispose();
                StopProcess(started.Process);
            }
        }
        catch
        {
            // Fall back to stdio probing below.
        }

        foreach (var arguments in new[]
        {
            new[] { "serve", "--listen", "stdio://" },
            Array.Empty<string>(),
        })
        {
            try
            {
                return DescribeBinaryTargetOnce(binaryPath, arguments, timeout);
            }
            catch
            {
                // Try the fallback launch form next.
            }
        }

        return DescribeBinaryTargetOnce(binaryPath, Array.Empty<string>(), timeout);
    }

    internal static LaunchTarget LaunchTargetFromRef(HolonRef reference)
    {
        var path = Discovery.PathFromFileUrl(reference.Url);
        var info = reference.Info ?? throw new InvalidOperationException("holon metadata unavailable");

        if (File.Exists(path))
            return new LaunchTarget(path, Array.Empty<string>(), Path.GetDirectoryName(path));

        if (!Directory.Exists(path))
            throw new InvalidOperationException($"target path \"{path}\" is not launchable");

        return path.EndsWith(".holon", StringComparison.OrdinalIgnoreCase)
            || File.Exists(Path.Combine(path, ".holon.json"))
            ? PackageLaunchTarget(path, info)
            : SourceLaunchTarget(path, info);
    }

    internal static string DefaultPortFilePath(string slug) =>
        Path.Combine(Directory.GetCurrentDirectory(), ".op", "run", $"{slug}.port");

    internal static void WritePortFile(string portFile, string uri)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(portFile)!);
        File.WriteAllText(portFile, uri.Trim() + Environment.NewLine);
    }

    internal static string NormalizeDialTarget(string target)
    {
        if (IsHostPortTarget(target))
            return target;

        if (!target.Contains("://", StringComparison.Ordinal))
            return target;

        var parsed = Transport.ParseUri(target);
        if (parsed.Scheme == "tcp")
        {
            var host = string.IsNullOrWhiteSpace(parsed.Host) || parsed.Host == "0.0.0.0"
                ? "127.0.0.1"
                : parsed.Host;
            return $"{host}:{parsed.Port}";
        }

        return target;
    }

    private static HolonInfo DescribeBinaryTargetOnce(string binaryPath, IReadOnlyList<string> arguments, int timeout)
    {
        using var process = new Process
        {
            StartInfo = CreateProcessStartInfo(binaryPath, Array.Empty<string>(), null, arguments),
        };
        if (!process.Start())
            throw new IOException($"failed to start {binaryPath}");

        using var bridge = new StdioBridge(process);
        EnsureStdioStartup(process, bridge, ConnectTimeout(timeout));

        var dialed = DialReadyAsync(bridge.Uri, ConnectTimeout(timeout)).GetAwaiter().GetResult();
        try
        {
            return DescribeChannel(dialed.Channel, timeout);
        }
        finally
        {
            dialed.Resource?.Dispose();
            dialed.Channel.Dispose();
            StopProcess(process);
        }
    }

    private static HolonInfo DescribeChannel(GrpcChannel channel, int timeout)
    {
        var client = new HolonMeta.HolonMetaClient(channel.CreateCallInvoker());
        var deadline = DateTime.UtcNow + ConnectTimeout(timeout);
        var response = client.Describe(new DescribeRequest(), deadline: deadline);
        var manifest = response?.Manifest ?? throw new InvalidOperationException("Describe returned no manifest");
        var identityPayload = manifest.Identity ?? throw new InvalidOperationException("Describe returned no identity");
        var identity = new IdentityInfo(
            identityPayload.GivenName ?? string.Empty,
            identityPayload.FamilyName ?? string.Empty,
            identityPayload.Motto ?? string.Empty,
            identityPayload.Aliases.ToArray());

        return new HolonInfo(
            SlugFor(identity.GivenName, identity.FamilyName),
            identityPayload.Uuid ?? string.Empty,
            identity,
            manifest.Lang ?? string.Empty,
            manifest.Build?.Runner ?? string.Empty,
            identityPayload.Status ?? string.Empty,
            manifest.Kind ?? string.Empty,
            manifest.Transport ?? string.Empty,
            manifest.Artifacts?.Binary ?? string.Empty,
            manifest.Platforms.ToArray(),
            false,
            false);
    }

    internal static async Task<StartedConnection> StartStdioHolonAsync(LaunchTarget target, TimeSpan timeout)
    {
        var process = new Process
        {
            StartInfo = CreateProcessStartInfo(
                target.CommandPath,
                target.Arguments,
                target.WorkingDirectory,
                new[] { "serve", "--listen", "stdio://" }),
        };
        if (!process.Start())
            throw new IOException($"failed to start {target.CommandPath}");

        var bridge = new StdioBridge(process);
        try
        {
            EnsureStdioStartup(process, bridge, timeout);
            var dialed = await DialReadyAsync(bridge.Uri, timeout).ConfigureAwait(false);
            return new StartedConnection(dialed.Channel, process, CombineResources(bridge, dialed.Resource), "stdio://");
        }
        catch
        {
            bridge.Dispose();
            StopProcess(process);
            throw;
        }
    }

    internal static async Task<StartedConnection> StartUnixHolonAsync(
        LaunchTarget target,
        string slug,
        string portFile,
        TimeSpan timeout)
    {
        var uri = DefaultUnixSocketUri(slug, portFile);
        var socketPath = uri["unix://".Length..];
        var process = new Process
        {
            StartInfo = CreateProcessStartInfo(
                target.CommandPath,
                target.Arguments,
                target.WorkingDirectory,
                new[] { "serve", "--listen", uri }),
        };
        var stderr = new StringBuilder();
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrEmpty(args.Data))
                stderr.AppendLine(args.Data);
        };

        if (!process.Start())
            throw new IOException($"failed to start {target.CommandPath}");
        process.BeginErrorReadLine();

        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            if (File.Exists(socketPath))
            {
                var dialed = await DialReadyAsync(uri, timeout).ConfigureAwait(false);
                return new StartedConnection(dialed.Channel, process, dialed.Resource, uri);
            }
            if (process.HasExited)
                throw new IOException($"holon exited before binding unix socket{FormatStderr(stderr)}");
            await Task.Delay(20).ConfigureAwait(false);
        }

        StopProcess(process);
        throw new IOException($"timed out waiting for unix holon startup{FormatStderr(stderr)}");
    }

    internal static async Task<StartedConnection> StartTcpHolonAsync(LaunchTarget target, TimeSpan timeout)
    {
        var process = new Process
        {
            StartInfo = CreateProcessStartInfo(
                target.CommandPath,
                target.Arguments,
                target.WorkingDirectory,
                new[] { "serve", "--listen", "tcp://127.0.0.1:0" }),
        };
        var advertised = new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
        var stderr = new List<string>();
        process.OutputDataReceived += (_, args) =>
        {
            var uri = FirstUri(args.Data);
            if (uri.Length > 0)
                advertised.TrySetResult(uri);
        };
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrEmpty(args.Data))
                stderr.Add(args.Data);
            var uri = FirstUri(args.Data);
            if (uri.Length > 0)
                advertised.TrySetResult(uri);
        };
        process.Exited += (_, _) =>
        {
            advertised.TrySetException(new IOException(
                $"holon exited before advertising an address ({process.ExitCode}): {string.Join(Environment.NewLine, stderr)}"));
        };

        if (!process.Start())
            throw new IOException($"failed to start {target.CommandPath}");
        process.BeginOutputReadLine();
        process.BeginErrorReadLine();

        var completed = await Task.WhenAny(advertised.Task, Task.Delay(timeout)).ConfigureAwait(false);
        if (completed != advertised.Task)
        {
            StopProcess(process);
            throw new IOException("timed out waiting for holon startup");
        }

        var publicUri = await advertised.Task.ConfigureAwait(false);
        var dialed = await DialReadyAsync(publicUri, timeout).ConfigureAwait(false);
        return new StartedConnection(dialed.Channel, process, dialed.Resource, publicUri);
    }

    private static ProcessStartInfo CreateProcessStartInfo(
        string commandPath,
        IReadOnlyList<string> arguments,
        string? workingDirectory,
        IReadOnlyList<string> tailArguments)
    {
        var startInfo = new ProcessStartInfo(commandPath)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        if (!string.IsNullOrWhiteSpace(workingDirectory))
            startInfo.WorkingDirectory = workingDirectory;

        foreach (var argument in arguments)
            startInfo.ArgumentList.Add(argument);
        foreach (var argument in tailArguments)
            startInfo.ArgumentList.Add(argument);
        return startInfo;
    }

    private static void EnsureStdioStartup(Process process, StdioBridge bridge, TimeSpan timeout)
    {
        var startupWindow = timeout < TimeSpan.FromMilliseconds(200)
            ? timeout
            : TimeSpan.FromMilliseconds(200);
        if (startupWindow <= TimeSpan.Zero)
            startupWindow = TimeSpan.FromMilliseconds(1);

        var deadline = DateTime.UtcNow + startupWindow;
        while (DateTime.UtcNow < deadline)
        {
            if (process.HasExited)
                throw new IOException($"holon exited before stdio startup{FormatStderr(bridge.StderrText)}");
            Thread.Sleep(10);
        }
    }

    private static LaunchTarget PackageLaunchTarget(string packageDir, HolonInfo info)
    {
        var entrypoint = string.IsNullOrWhiteSpace(info.Entrypoint) ? info.Slug : info.Entrypoint.Trim();
        if (entrypoint.Length == 0)
            throw new InvalidOperationException("target unreachable");

        var binaryPath = Path.Combine(packageDir, "bin", Discovery.PackageArchDirectory(), Path.GetFileName(entrypoint));
        if (File.Exists(binaryPath))
            return new LaunchTarget(binaryPath, Array.Empty<string>(), packageDir);

        var distEntry = Path.Combine(packageDir, "dist", entrypoint);
        if (File.Exists(distEntry))
        {
            var runnerTarget = LaunchTargetForRunner(info.Runner, distEntry, packageDir);
            if (runnerTarget is not null)
                return runnerTarget;
        }

        var gitRoot = Path.Combine(packageDir, "git");
        if (Directory.Exists(gitRoot))
            return SourceLaunchTarget(gitRoot, info);

        throw new InvalidOperationException("target unreachable");
    }

    private static LaunchTarget SourceLaunchTarget(string sourceDir, HolonInfo info)
    {
        var entrypoint = string.IsNullOrWhiteSpace(info.Entrypoint) ? info.Slug : info.Entrypoint.Trim();
        if (entrypoint.Length > 0)
        {
            if (Path.IsPathRooted(entrypoint) && File.Exists(entrypoint))
                return new LaunchTarget(entrypoint, Array.Empty<string>(), sourceDir);

            var sourcePackageBinary = Path.Combine(
                sourceDir,
                ".op",
                "build",
                $"{info.Slug}.holon",
                "bin",
                Discovery.PackageArchDirectory(),
                Path.GetFileName(entrypoint));
            if (File.Exists(sourcePackageBinary))
                return new LaunchTarget(sourcePackageBinary, Array.Empty<string>(), sourceDir);

            var sourceBinary = Path.Combine(sourceDir, ".op", "build", "bin", Path.GetFileName(entrypoint));
            if (File.Exists(sourceBinary))
                return new LaunchTarget(sourceBinary, Array.Empty<string>(), sourceDir);

            var directEntry = Path.Combine(sourceDir, entrypoint);
            if (File.Exists(directEntry))
            {
                var runnerTarget = LaunchTargetForRunner(info.Runner, directEntry, sourceDir);
                if (runnerTarget is not null)
                    return runnerTarget;
                return new LaunchTarget(directEntry, Array.Empty<string>(), sourceDir);
            }
        }

        throw new InvalidOperationException("target unreachable");
    }

    private static LaunchTarget? LaunchTargetForRunner(string runner, string entrypoint, string workingDirectory)
    {
        var name = (runner ?? string.Empty).Trim().ToLowerInvariant();
        return name switch
        {
            "go" or "go-module" => new LaunchTarget("go", new[] { "run", entrypoint }, workingDirectory),
            "python" => new LaunchTarget("python3", new[] { entrypoint }, workingDirectory),
            "node" or "typescript" or "npm" => new LaunchTarget("node", new[] { entrypoint }, workingDirectory),
            "ruby" => new LaunchTarget("ruby", new[] { entrypoint }, workingDirectory),
            "dart" => new LaunchTarget("dart", new[] { "run", entrypoint }, workingDirectory),
            "shell" or "sh" or "bash" => new LaunchTarget(entrypoint, Array.Empty<string>(), workingDirectory),
            _ => null,
        };
    }

    private static string PackageBinaryPath(string packageDir)
    {
        var archRoot = Path.Combine(packageDir, "bin", Discovery.PackageArchDirectory());
        if (!Directory.Exists(archRoot))
            throw new FileNotFoundException($"no package binary for arch {Discovery.PackageArchDirectory()}");

        return Directory.EnumerateFiles(archRoot)
            .OrderBy(path => path, StringComparer.Ordinal)
            .FirstOrDefault()
            ?? throw new FileNotFoundException($"no package binary for arch {Discovery.PackageArchDirectory()}");
    }

    private static async Task WaitForTcpReadyAsync(string target, TimeSpan timeout, CancellationToken cancellationToken)
    {
        var (host, port) = ParseHostPort(target);
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;

        while (DateTime.UtcNow < deadline)
        {
            try
            {
                using var client = new TcpClient();
                await client.ConnectAsync(host, port, cancellationToken).ConfigureAwait(false);
                return;
            }
            catch (Exception ex) when (ex is SocketException or TaskCanceledException or OperationCanceledException)
            {
                last = ex;
                await Task.Delay(50, CancellationToken.None).ConfigureAwait(false);
            }
        }

        throw new IOException("timed out waiting for gRPC readiness", last);
    }

    private static async Task WaitForUnixReadyAsync(string target, TimeSpan timeout, CancellationToken cancellationToken)
    {
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;

        while (DateTime.UtcNow < deadline)
        {
            cancellationToken.ThrowIfCancellationRequested();
            try
            {
                using var socket = Transport.DialUnix(target);
                return;
            }
            catch (Exception ex) when (ex is SocketException or IOException or OperationCanceledException)
            {
                last = ex;
                await Task.Delay(50, CancellationToken.None).ConfigureAwait(false);
            }
        }

        throw new IOException("timed out waiting for unix gRPC readiness", last);
    }

    private static string BuildChannelAddress(string hostPort)
    {
        var (host, port) = ParseHostPort(hostPort);
        return $"http://{host}:{port}";
    }

    private static (string Host, int Port) ParseHostPort(string target)
    {
        var index = target.LastIndexOf(':');
        if (index <= 0 || index >= target.Length - 1)
            throw new ArgumentException($"invalid host:port target \"{target}\"", nameof(target));
        return (target[..index], int.Parse(target[(index + 1)..]));
    }

    private static bool IsHostPortTarget(string target)
    {
        var index = target.LastIndexOf(':');
        return index > 0
            && index < target.Length - 1
            && !target.Contains("://", StringComparison.Ordinal)
            && int.TryParse(target[(index + 1)..], out _);
    }

    private static string DefaultUnixSocketUri(string slug, string portFile)
    {
        var label = SocketLabel(slug);
        var hash = Fnv1a64(portFile) & 0xFFFFFFFFFFFFUL;
        return $"unix:///tmp/holons-{label}-{hash:x12}.sock";
    }

    private static string SocketLabel(string slug)
    {
        var builder = new StringBuilder();
        var lastDash = false;
        foreach (var ch in (slug ?? string.Empty).Trim().ToLowerInvariant())
        {
            if (char.IsAsciiLetterOrDigit(ch))
            {
                builder.Append(ch);
                lastDash = false;
            }
            else if ((ch == '-' || ch == '_') && builder.Length > 0 && !lastDash)
            {
                builder.Append('-');
                lastDash = true;
            }

            if (builder.Length >= 24)
                break;
        }

        return builder.ToString().Trim('-') is { Length: > 0 } label ? label : "socket";
    }

    private static ulong Fnv1a64(string text)
    {
        ulong hash = 0xcbf29ce484222325UL;
        foreach (var b in Encoding.UTF8.GetBytes(text ?? string.Empty))
        {
            hash ^= b;
            hash *= 0x100000001b3UL;
        }
        return hash;
    }

    private static string FirstUri(string? line)
    {
        if (string.IsNullOrWhiteSpace(line))
            return string.Empty;

        foreach (var field in line.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries))
        {
            var trimmed = field.Trim().Trim('"', '\'', '(', ')', '[', ']', '{', '}', '.', ',');
            if (trimmed.StartsWith("tcp://", StringComparison.Ordinal)
                || trimmed.StartsWith("unix://", StringComparison.Ordinal)
                || trimmed.StartsWith("stdio://", StringComparison.Ordinal)
                || trimmed.StartsWith("ws://", StringComparison.Ordinal)
                || trimmed.StartsWith("wss://", StringComparison.Ordinal))
            {
                return trimmed;
            }
        }

        return string.Empty;
    }

    private static string SlugFor(string givenName, string familyName)
    {
        var given = (givenName ?? string.Empty).Trim();
        var family = (familyName ?? string.Empty).Trim().TrimEnd('?');
        if (given.Length == 0 && family.Length == 0)
            return string.Empty;

        return $"{given}-{family}"
            .Trim()
            .ToLowerInvariant()
            .Replace(" ", "-", StringComparison.Ordinal)
            .Trim('-');
    }

    private static string FormatStderr(StringBuilder stderr)
    {
        var text = stderr.ToString().Trim();
        return text.Length == 0 ? string.Empty : $": {text}";
    }

    private static string FormatStderr(string stderr) =>
        string.IsNullOrWhiteSpace(stderr) ? string.Empty : $": {stderr.Trim()}";

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
            Exception? error = null;
            try { first.Dispose(); } catch (Exception ex) { error = ex; }
            try { second.Dispose(); } catch (Exception ex) { error ??= ex; }
            if (error is not null)
                throw error;
        }
    }

    internal static void StopProcess(Process? process)
    {
        if (process is null || process.HasExited)
            return;

        try
        {
            process.Kill(entireProcessTree: true);
            process.WaitForExit(2000);
        }
        catch
        {
            // Best effort.
        }
    }
}

internal sealed class StdioBridge : IDisposable
{
    private readonly Process _process;
    private readonly TcpListener _listener;
    private readonly StringBuilder _stderr = new();
    private readonly Thread _acceptThread;
    private volatile bool _closed;
    private TcpClient? _client;

    public StdioBridge(Process process)
    {
        _process = process;
        _listener = new TcpListener(IPAddress.Loopback, 0);
        _listener.Start();
        StartDrainThread(process.StandardError.BaseStream, _stderr, "holons-stdio-bridge-stderr");
        _acceptThread = new Thread(AcceptLoop) { IsBackground = true, Name = "holons-stdio-bridge-accept" };
        _acceptThread.Start();
    }

    public string Uri => $"tcp://127.0.0.1:{((_listener.LocalEndpoint as IPEndPoint)?.Port ?? 0)}";

    public string StderrText
    {
        get
        {
            lock (_stderr)
                return _stderr.ToString().Trim();
        }
    }

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
                var networkStream = accepted.GetStream();
                var firstChunk = new byte[16 * 1024];
                var firstRead = networkStream.Read(firstChunk, 0, firstChunk.Length);
                if (firstRead <= 0)
                {
                    accepted.Close();
                    _client = null;
                    continue;
                }

                _process.StandardInput.BaseStream.Write(firstChunk, 0, firstRead);
                _process.StandardInput.BaseStream.Flush();

                var upstream = StartPump(
                    networkStream,
                    _process.StandardInput.BaseStream,
                    closeOutput: false,
                    "holons-stdio-bridge-up");
                var downstream = StartPump(
                    _process.StandardOutput.BaseStream,
                    networkStream,
                    closeOutput: true,
                    "holons-stdio-bridge-down");
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
        finally
        {
            try { _client?.Close(); } catch { }
            _client = null;
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
            catch
            {
                // Closed during shutdown.
            }
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

    private static void StartDrainThread(Stream stream, StringBuilder capture, string name)
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
                    {
                        capture.Append(Encoding.UTF8.GetString(buffer, 0, read));
                    }
                }
            }
            catch
            {
                // Closed during shutdown.
            }
        })
        {
            IsBackground = true,
            Name = name,
        };
        thread.Start();
    }

    private static void TryDispose(IDisposable? disposable)
    {
        try
        {
            disposable?.Dispose();
        }
        catch
        {
            // ignored
        }
    }
}

internal sealed class UnixBridge : IDisposable
{
    private readonly TcpListener _listener;
    private readonly Thread _acceptThread;
    private volatile bool _closed;
    private readonly string _target;
    private readonly object _connectionsGate = new();
    private readonly HashSet<IDisposable> _connections = new();

    public UnixBridge(string target)
    {
        _target = target;
        _listener = new TcpListener(IPAddress.Loopback, 0);
        _listener.Start();
        _acceptThread = new Thread(AcceptLoop) { IsBackground = true, Name = "holons-unix-bridge-accept" };
        _acceptThread.Start();
    }

    public string Uri => $"tcp://127.0.0.1:{((_listener.LocalEndpoint as IPEndPoint)?.Port ?? 0)}";

    public void Dispose()
    {
        _closed = true;
        try { _listener.Stop(); } catch { }
        List<IDisposable> active;
        lock (_connectionsGate)
        {
            active = _connections.ToList();
            _connections.Clear();
        }
        foreach (var connection in active)
        {
            try { connection.Dispose(); } catch { }
        }
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

                var handler = new Thread(() => HandleClient(accepted))
                {
                    IsBackground = true,
                    Name = "holons-unix-bridge-client",
                };
                handler.Start();
            }
        }
        catch
        {
            // Closed during shutdown.
        }
    }

    private void HandleClient(TcpClient accepted)
    {
        Socket? upstream = null;
        try
        {
            upstream = Transport.DialUnix(_target);
            RegisterConnection(accepted);
            RegisterConnection(upstream);

            var clientStream = accepted.GetStream();
            var upstreamStream = new NetworkStream(upstream, ownsSocket: false);
            var up = StartPump(clientStream, upstreamStream, closeOutput: true, "holons-unix-bridge-up");
            var down = StartPump(upstreamStream, clientStream, closeOutput: true, "holons-unix-bridge-down");
            up.Join();
            down.Join();
        }
        catch
        {
            // Closed during shutdown.
        }
        finally
        {
            UnregisterAndDispose(accepted);
            if (upstream is not null)
                UnregisterAndDispose(upstream);
        }
    }

    private void RegisterConnection(IDisposable connection)
    {
        lock (_connectionsGate)
        {
            if (_closed)
            {
                connection.Dispose();
                return;
            }
            _connections.Add(connection);
        }
    }

    private void UnregisterAndDispose(IDisposable connection)
    {
        lock (_connectionsGate)
        {
            _connections.Remove(connection);
        }
        try { connection.Dispose(); } catch { }
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
            catch
            {
                // Closed during shutdown.
            }
            finally
            {
                if (closeOutput)
                {
                    try { output.Close(); } catch { }
                }
            }
        })
        {
            IsBackground = true,
            Name = name,
        };
        thread.Start();
        return thread;
    }
}

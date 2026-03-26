using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Net.Client;

namespace Holons;

/// <summary>Resolve holons to ready gRPC channels.</summary>
public static class Connect
{
    public sealed record ConnectOptions
    {
        public TimeSpan Timeout { get; init; } = TimeSpan.FromSeconds(5);
        public string Transport { get; init; } = "stdio";
        public bool Start { get; init; } = true;
        public string PortFile { get; init; } = "";
    }

    private sealed record StartedHandle(Process? Process, IDisposable? Resource, bool Ephemeral);

    private sealed record StartedProcess(string Uri, Process Process, IDisposable? Resource = null);

    private sealed record DialedChannel(GrpcChannel Channel, IDisposable? Resource = null);

    private static readonly Dictionary<GrpcChannel, StartedHandle> Started =
        new(ReferenceEqualityComparer.Instance);

    public static GrpcChannel ConnectTarget(string target) =>
        ConnectInternal(target, new ConnectOptions(), defaultEphemeral: true).GetAwaiter().GetResult();

    public static GrpcChannel ConnectTarget(string target, ConnectOptions options) =>
        ConnectInternal(target, options ?? new ConnectOptions(), defaultEphemeral: false).GetAwaiter().GetResult();

    public static void Disconnect(GrpcChannel? channel)
    {
        if (channel is null)
            return;

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
        handle?.Resource?.Dispose();
        if (handle is not null && handle.Ephemeral)
            StopProcess(handle.Process);
    }

    private static async Task<GrpcChannel> ConnectInternal(string target, ConnectOptions options, bool defaultEphemeral)
    {
        var trimmed = (target ?? "").Trim();
        if (trimmed.Length == 0)
            throw new ArgumentException("target is required", nameof(target));

        var timeout = options.Timeout > TimeSpan.Zero ? options.Timeout : TimeSpan.FromSeconds(5);

        if (IsDirectTarget(trimmed))
        {
            var direct = await DialReadyAsync(NormalizeDialTarget(trimmed), timeout);
            RememberDialedChannel(direct, ephemeral: false);
            return direct.Channel;
        }

        var transport = (options.Transport ?? "stdio").Trim().ToLowerInvariant();
        if (transport is not "stdio" and not "tcp" and not "unix")
            throw new ArgumentException($"unsupported transport \"{options.Transport}\"", nameof(options));
        var ephemeral = defaultEphemeral || transport == "stdio";

        var entry = Discover.FindBySlug(trimmed) ??
            throw new InvalidOperationException($"holon \"{trimmed}\" not found");

        var portFile = string.IsNullOrWhiteSpace(options.PortFile)
            ? DefaultPortFilePath(entry.Slug)
            : options.PortFile.Trim();

        var reusable = await UsablePortFileAsync(portFile, timeout);
        if (reusable is not null)
        {
            RememberDialedChannel(reusable, ephemeral: false);
            return reusable.Channel;
        }
        if (!options.Start)
            throw new InvalidOperationException($"holon \"{trimmed}\" is not running");

        var binaryPath = ResolveBinaryPath(entry);
        var started = transport == "stdio"
            ? await StartStdioHolonAsync(binaryPath, timeout)
            : transport == "unix"
            ? await StartUnixHolonAsync(binaryPath, entry.Slug, portFile, timeout)
            : await StartTcpHolonAsync(binaryPath, timeout);

        DialedChannel dialed;
        try
        {
            dialed = await DialReadyAsync(NormalizeDialTarget(started.Uri), timeout);
        }
        catch
        {
            started.Resource?.Dispose();
            StopProcess(started.Process);
            throw;
        }
        var channel = dialed.Channel;

        if (!ephemeral && (transport == "tcp" || transport == "unix"))
        {
            try
            {
                WritePortFile(portFile, started.Uri);
            }
            catch
            {
                channel.Dispose();
                StopProcess(started.Process);
                throw;
            }
        }

        lock (Started)
        {
            Started[channel] = new StartedHandle(started.Process, CombineResources(started.Resource, dialed.Resource), ephemeral);
        }

        return channel;
    }

    private static async Task<DialedChannel> DialReadyAsync(string target, TimeSpan timeout)
    {
        IDisposable? resource = null;
        string address;
        if (target.StartsWith("unix://", StringComparison.Ordinal))
        {
            using var cts = new CancellationTokenSource(timeout);
            await WaitForUnixReadyAsync(target, timeout, cts.Token);
            var bridge = new UnixBridge(target);
            resource = bridge;
            address = BuildChannelAddress(NormalizeDialTarget(bridge.Uri));
        }
        else
        {
            using var cts = new CancellationTokenSource(timeout);
            await WaitForTcpReadyAsync(target, timeout, cts.Token);
            address = BuildChannelAddress(target);
        }
        return new DialedChannel(GrpcChannel.ForAddress(address), resource);
    }

    private static async Task<DialedChannel?> UsablePortFileAsync(string portFile, TimeSpan timeout)
    {
        try
        {
            var raw = (await File.ReadAllTextAsync(portFile)).Trim();
            if (raw.Length == 0)
            {
                File.Delete(portFile);
                return null;
            }

            return await DialReadyAsync(NormalizeDialTarget(raw), timeout < TimeSpan.FromSeconds(1)
                ? timeout
                : TimeSpan.FromSeconds(1));
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
                // Best-effort stale port file cleanup.
            }
            return null;
        }
    }

    private static async Task WaitForTcpReadyAsync(string target, TimeSpan timeout, CancellationToken cancellationToken)
    {
        var hostPort = ParseHostPort(target);
        var deadline = DateTime.UtcNow + timeout;
        Exception? last = null;

        while (DateTime.UtcNow < deadline)
        {
            try
            {
                using var client = new TcpClient();
                await client.ConnectAsync(hostPort.Host, hostPort.Port, cancellationToken);
                return;
            }
            catch (Exception ex) when (ex is SocketException || ex is TaskCanceledException || ex is OperationCanceledException)
            {
                last = ex;
                await Task.Delay(50, CancellationToken.None);
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
            try
            {
                using var socket = Transport.DialUnix(target);
                return;
            }
            catch (Exception ex) when (ex is SocketException || ex is IOException || ex is OperationCanceledException)
            {
                last = ex;
                await Task.Delay(50, CancellationToken.None);
            }
        }

        throw new IOException("timed out waiting for unix gRPC readiness", last);
    }

    private static async Task<StartedProcess> StartTcpHolonAsync(string binaryPath, TimeSpan timeout)
    {
        var startInfo = new ProcessStartInfo(binaryPath)
        {
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        startInfo.ArgumentList.Add("serve");
        startInfo.ArgumentList.Add("--listen");
        startInfo.ArgumentList.Add("tcp://127.0.0.1:0");

        var process = new Process { StartInfo = startInfo, EnableRaisingEvents = true };
        var tcs = new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
        var stderr = new List<string>();

        process.OutputDataReceived += (_, args) =>
        {
            var uri = FirstUri(args.Data);
            if (uri.Length > 0)
                tcs.TrySetResult(uri);
        };
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrEmpty(args.Data))
                stderr.Add(args.Data);
            var uri = FirstUri(args.Data);
            if (uri.Length > 0)
                tcs.TrySetResult(uri);
        };
        process.Exited += (_, _) =>
        {
            tcs.TrySetException(new IOException(
                $"holon exited before advertising an address ({process.ExitCode}): {string.Join(Environment.NewLine, stderr)}"));
        };

        if (!process.Start())
            throw new IOException($"failed to start {binaryPath}");

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();

        var completed = await Task.WhenAny(tcs.Task, Task.Delay(timeout));
        if (completed == tcs.Task)
            return new StartedProcess(await tcs.Task, process);

        StopProcess(process);
        throw new IOException("timed out waiting for holon startup");
    }

    private static async Task<StartedProcess> StartUnixHolonAsync(string binaryPath, string slug, string portFile, TimeSpan timeout)
    {
        var uri = DefaultUnixSocketUri(slug, portFile);
        var socketPath = uri["unix://".Length..];
        var startInfo = new ProcessStartInfo(binaryPath)
        {
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        startInfo.ArgumentList.Add("serve");
        startInfo.ArgumentList.Add("--listen");
        startInfo.ArgumentList.Add(uri);

        var process = new Process { StartInfo = startInfo };
        var stderr = new StringBuilder();
        process.ErrorDataReceived += (_, args) =>
        {
            if (!string.IsNullOrEmpty(args.Data))
                stderr.AppendLine(args.Data);
        };

        if (!process.Start())
            throw new IOException($"failed to start {binaryPath}");

        process.BeginErrorReadLine();

        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            if (File.Exists(socketPath))
                return new StartedProcess(uri, process);
            if (process.HasExited)
                throw new IOException($"holon exited before binding unix socket{(stderr.Length == 0 ? "" : $": {stderr.ToString().Trim()}")}");
            await Task.Delay(20).ConfigureAwait(false);
        }

        StopProcess(process);
        throw new IOException($"timed out waiting for unix holon startup{(stderr.Length == 0 ? "" : $": {stderr.ToString().Trim()}")}");
    }

    private static async Task<StartedProcess> StartStdioHolonAsync(string binaryPath, TimeSpan timeout)
    {
        var startInfo = new ProcessStartInfo(binaryPath)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
        };
        startInfo.ArgumentList.Add("serve");
        startInfo.ArgumentList.Add("--listen");
        startInfo.ArgumentList.Add("stdio://");

        var process = new Process { StartInfo = startInfo };
        if (!process.Start())
            throw new IOException($"failed to start {binaryPath}");

        var bridge = new StdioBridge(process);
        var startupWindow = timeout < TimeSpan.FromMilliseconds(200)
            ? timeout
            : TimeSpan.FromMilliseconds(200);
        if (startupWindow <= TimeSpan.Zero)
            startupWindow = TimeSpan.FromMilliseconds(1);

        var deadline = DateTime.UtcNow + startupWindow;
        while (DateTime.UtcNow < deadline)
        {
            if (process.HasExited)
            {
                var stderr = bridge.StderrText;
                bridge.Dispose();
                throw new IOException($"holon exited before stdio startup{(stderr.Length == 0 ? "" : $": {stderr}")}");
            }
            await Task.Delay(10).ConfigureAwait(false);
        }

        return new StartedProcess(bridge.Uri, process, bridge);
    }

    private static string ResolveBinaryPath(Discover.HolonEntry entry)
    {
        if (entry.Manifest is null)
            throw new InvalidOperationException($"holon \"{entry.Slug}\" has no manifest");

        var binary = (entry.Manifest.Artifacts.Binary ?? "").Trim();
        if (binary.Length == 0)
            throw new InvalidOperationException($"holon \"{entry.Slug}\" has no artifacts.binary");

        if (Path.IsPathRooted(binary) && File.Exists(binary))
            return binary;

        var candidate = Path.Combine(entry.Dir, ".op", "build", "bin", Path.GetFileName(binary));
        if (File.Exists(candidate))
            return candidate;

        var pathEnv = Environment.GetEnvironmentVariable("PATH") ?? "";
        foreach (var dir in pathEnv.Split(Path.PathSeparator, StringSplitOptions.RemoveEmptyEntries))
        {
            var resolved = Path.Combine(dir, Path.GetFileName(binary));
            if (File.Exists(resolved))
                return resolved;
        }

        throw new InvalidOperationException($"built binary not found for holon \"{entry.Slug}\"");
    }

    private static string DefaultPortFilePath(string slug) =>
        Path.Combine(Directory.GetCurrentDirectory(), ".op", "run", $"{slug}.port");

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
        foreach (var ch in (slug ?? "").Trim().ToLowerInvariant())
        {
            if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9'))
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
        foreach (var b in Encoding.UTF8.GetBytes(text ?? ""))
        {
            hash ^= b;
            hash *= 0x100000001b3UL;
        }
        return hash;
    }

    private static void WritePortFile(string portFile, string uri)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(portFile)!);
        File.WriteAllText(portFile, uri.Trim() + Environment.NewLine);
    }

    private static void StopProcess(Process? process)
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
            // Best-effort shutdown.
        }
    }

    private static bool IsDirectTarget(string target) =>
        target.Contains("://", StringComparison.Ordinal) || target.Contains(':', StringComparison.Ordinal);

    private static string NormalizeDialTarget(string target)
    {
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

    private static string BuildChannelAddress(string target)
    {
        var hostPort = ParseHostPort(target);
        return $"http://{hostPort.Host}:{hostPort.Port}";
    }

    private static (string Host, int Port) ParseHostPort(string target)
    {
        var index = target.LastIndexOf(':');
        if (index <= 0 || index >= target.Length - 1)
            throw new ArgumentException($"invalid host:port target \"{target}\"", nameof(target));
        return (target[..index], int.Parse(target[(index + 1)..]));
    }

    private static string FirstUri(string? line)
    {
        if (string.IsNullOrWhiteSpace(line))
            return "";

        foreach (var field in line.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries))
        {
            var trimmed = field.Trim().Trim('"', '\'', '(', ')', '[', ']', '{', '}', '.', ',');
            if (trimmed.StartsWith("tcp://", StringComparison.Ordinal) ||
                trimmed.StartsWith("unix://", StringComparison.Ordinal) ||
                trimmed.StartsWith("stdio://", StringComparison.Ordinal) ||
                trimmed.StartsWith("ws://", StringComparison.Ordinal) ||
                trimmed.StartsWith("wss://", StringComparison.Ordinal))
            {
                return trimmed;
            }
        }
        return "";
    }

    private static void RememberDialedChannel(DialedChannel dialed, bool ephemeral)
    {
        if (dialed.Resource is null)
            return;
        lock (Started)
        {
            Started[dialed.Channel] = new StartedHandle(null, dialed.Resource, ephemeral);
        }
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
            Exception? error = null;
            try { first.Dispose(); } catch (Exception ex) { error = ex; }
            try { second.Dispose(); } catch (Exception ex) { error ??= ex; }
            if (error is not null)
                throw error;
        }
    }

    private sealed class StdioBridge : IDisposable
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
            try
            {
                _listener.Stop();
            }
            catch
            {
                // ignored
            }

            try
            {
                _client?.Close();
            }
            catch
            {
                // ignored
            }

            TryDispose(_process.StandardInput);
            TryDispose(_process.StandardOutput);
            TryDispose(_process.StandardError);

            try
            {
                _acceptThread.Join(200);
            }
            catch
            {
                // ignored
            }
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

                    try
                    {
                        accepted.Close();
                    }
                    catch
                    {
                        // ignored
                    }
                    _client = null;
                }
            }
            catch
            {
                // Listener/socket closed during shutdown.
            }
            finally
            {
                try
                {
                    _client?.Close();
                }
                catch
                {
                    // ignored
                }
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
                    // Pipe/socket closed during shutdown.
                }
                finally
                {
                    if (closeOutput)
                    {
                        TryDispose(output);
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
                    // Stream closed during shutdown.
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

    private sealed class UnixBridge : IDisposable
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
                active = [.. _connections];
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
                // Bridge closed during shutdown.
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
                // Bridge closed during shutdown.
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
                    // Bridge closed during shutdown.
                }
                finally
                {
                    if (closeOutput)
                    {
                        try { output.Close(); } catch { }
                    }
                }
            })
            { IsBackground = true, Name = name };
            thread.Start();
            return thread;
        }
    }
}

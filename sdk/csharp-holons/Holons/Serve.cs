using System.Collections.Generic;
using System.Net;
using System.Net.Sockets;
using Grpc.Core;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace Holons;

/// <summary>Standard gRPC server runner utilities.</summary>
public static class Serve
{
    public sealed record ServeOptions
    {
        public bool Describe { get; init; } = true;
        public bool Reflect { get; init; } = false;
        public Action<string> Logger { get; init; } = message => Console.Error.WriteLine(message);
        public Action<string>? OnListen { get; init; }
        public int ShutdownGracePeriodSeconds { get; init; } = 10;
    }

    public sealed record ParsedFlags(string ListenUri, bool Reflect);

    public sealed record GrpcServiceRegistration(
        Action<IServiceCollection> ConfigureServices,
        Action<WebApplication> MapEndpoints);

    private sealed record StartedApplication(WebApplication App, string PublicUri);

    public sealed class RunningServer : IAsyncDisposable, IDisposable
    {
        private readonly WebApplication _app;
        private readonly Action<string> _logger;
        private readonly Func<Task>? _auxiliaryStop;
        private int _stopped;

        internal RunningServer(
            WebApplication app,
            string publicUri,
            Action<string> logger,
            Func<Task>? auxiliaryStop = null)
        {
            _app = app;
            _logger = logger;
            _auxiliaryStop = auxiliaryStop;
            PublicUri = publicUri;
        }

        public string PublicUri { get; }

        public Task AwaitAsync() => _app.WaitForShutdownAsync();

        public async Task StopAsync(int shutdownGracePeriodSeconds = 10)
        {
            if (Interlocked.Exchange(ref _stopped, 1) != 0)
            {
                await _app.WaitForShutdownAsync().ConfigureAwait(false);
                return;
            }

            if (_auxiliaryStop is not null)
                await _auxiliaryStop().ConfigureAwait(false);

            using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(shutdownGracePeriodSeconds));
            try
            {
                await _app.StopAsync(timeout.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                _logger($"graceful stop timed out after {shutdownGracePeriodSeconds}s");
            }
            await _app.DisposeAsync().ConfigureAwait(false);
        }

        public void Dispose()
        {
            StopAsync().GetAwaiter().GetResult();
        }

        public async ValueTask DisposeAsync()
        {
            await StopAsync().ConfigureAwait(false);
        }
    }

    /// <summary>Parse --listen or --port from command-line args.</summary>
    public static string ParseFlags(string[] args)
    {
        return ParseOptions(args).ListenUri;
    }

    public static ParsedFlags ParseOptions(string[] args)
    {
        var listenUri = Transport.DefaultUri;
        var reflect = false;
        for (int i = 0; i < args.Length; i++)
        {
            if (args[i] == "--listen" && i + 1 < args.Length)
                listenUri = args[i + 1];
            if (args[i] == "--port" && i + 1 < args.Length)
                listenUri = $"tcp://:{args[i + 1]}";
            if (args[i] == "--reflect")
                reflect = true;
        }
        return new ParsedFlags(listenUri, reflect);
    }

    public static GrpcServiceRegistration Service<TService>()
        where TService : class =>
        new(
            services => services.AddSingleton<TService>(),
            app => app.MapGrpcService<TService>());

    public static GrpcServiceRegistration Service<TService>(TService instance)
        where TService : class =>
        new(
            services => services.AddSingleton(instance),
            app => app.MapGrpcService<TService>());

    public static void Run(string listenUri, params GrpcServiceRegistration[] services) =>
        RunWithOptions(listenUri, services, null);

    public static void RunWithOptions(
        string listenUri,
        IEnumerable<GrpcServiceRegistration> services,
        ServeOptions? options = null)
    {
        options ??= new ServeOptions();
        var running = StartWithOptions(listenUri, services, options);

        ConsoleCancelEventHandler? cancelHandler = null;
        EventHandler? exitHandler = null;

        cancelHandler = (_, args) =>
        {
            args.Cancel = true;
            options.Logger("shutting down gRPC server");
            running.StopAsync(options.ShutdownGracePeriodSeconds).GetAwaiter().GetResult();
        };
        exitHandler = (_, _) =>
        {
            running.StopAsync(options.ShutdownGracePeriodSeconds).GetAwaiter().GetResult();
        };

        Console.CancelKeyPress += cancelHandler;
        AppDomain.CurrentDomain.ProcessExit += exitHandler;

        try
        {
            running.AwaitAsync().GetAwaiter().GetResult();
        }
        finally
        {
            Console.CancelKeyPress -= cancelHandler;
            AppDomain.CurrentDomain.ProcessExit -= exitHandler;
        }
    }

    public static RunningServer StartWithOptions(
        string listenUri,
        IEnumerable<GrpcServiceRegistration> services,
        ServeOptions? options = null)
    {
        options ??= new ServeOptions();
        var parsed = Transport.ParseUri(string.IsNullOrWhiteSpace(listenUri) ? Transport.DefaultUri : listenUri);
        var registrations = services.ToList();
        var describeEnabled = MaybeAddDescribe(registrations, options);
        var reflectionEnabled = MaybeAddReflection(registrations, options);

        return parsed.Scheme switch
        {
            "tcp" => StartTcpServer(
                host: string.IsNullOrWhiteSpace(parsed.Host) ? "0.0.0.0" : parsed.Host,
                port: parsed.Port ?? 9090,
                registrations: registrations,
                describeEnabled: describeEnabled,
                reflectionEnabled: reflectionEnabled,
                options: options,
                publicUriOverride: null,
                suppressAnnouncement: false),
            "stdio" => StartStdioServer(registrations, describeEnabled, reflectionEnabled, options),
            "unix" => StartUnixServer(parsed.Path ?? throw new ArgumentException("unix path missing"), registrations, describeEnabled, reflectionEnabled, options),
            _ => throw new ArgumentException(
                $"Serve.Run(...) currently supports tcp://, unix://, and stdio:// only: {listenUri}"),
        };
    }

    private static RunningServer StartStdioServer(
        IReadOnlyList<GrpcServiceRegistration> registrations,
        bool describeEnabled,
        bool reflectionEnabled,
        ServeOptions options)
    {
        var backing = StartApplication(
            host: "127.0.0.1",
            port: 0,
            registrations: registrations,
            publicUriOverride: null);

        var (host, port) = ParseTarget(backing.PublicUri);
        RunningServer? running = null;
        var bridge = new StdioServerBridge(host, port, () =>
        {
            running?.StopAsync(options.ShutdownGracePeriodSeconds).GetAwaiter().GetResult();
        });

        running = new RunningServer(
            backing.App,
            "stdio://",
            options.Logger,
            auxiliaryStop: async () => await bridge.DisposeAsync().ConfigureAwait(false));

        bridge.Start();
        var mode = FormatMode(describeEnabled, reflectionEnabled);
        options.OnListen?.Invoke("stdio://");
        options.Logger($"gRPC server listening on stdio:// ({mode})");
        return running;
    }

    private static RunningServer StartUnixServer(
        string path,
        IReadOnlyList<GrpcServiceRegistration> registrations,
        bool describeEnabled,
        bool reflectionEnabled,
        ServeOptions options)
    {
        var backing = StartApplication(
            host: "127.0.0.1",
            port: 0,
            registrations: registrations,
            publicUriOverride: null);

        var (host, port) = ParseTarget(backing.PublicUri);
        var bridge = new UnixServerBridge(path, host, port);
        bridge.Start();

        var publicUri = $"unix://{path}";
        var mode = FormatMode(describeEnabled, reflectionEnabled);
        options.OnListen?.Invoke(publicUri);
        options.Logger($"gRPC server listening on {publicUri} ({mode})");
        return new RunningServer(
            backing.App,
            publicUri,
            options.Logger,
            auxiliaryStop: async () => await bridge.DisposeAsync().ConfigureAwait(false));
    }

    private static RunningServer StartTcpServer(
        string host,
        int port,
        IReadOnlyList<GrpcServiceRegistration> registrations,
        bool describeEnabled,
        bool reflectionEnabled,
        ServeOptions options,
        string? publicUriOverride,
        bool suppressAnnouncement)
    {
        var started = StartApplication(host, port, registrations, publicUriOverride);
        if (!suppressAnnouncement)
        {
            var mode = FormatMode(describeEnabled, reflectionEnabled);
            options.OnListen?.Invoke(started.PublicUri);
            options.Logger($"gRPC server listening on {started.PublicUri} ({mode})");
        }

        return new RunningServer(started.App, started.PublicUri, options.Logger);
    }

    private static StartedApplication StartApplication(
        string host,
        int port,
        IReadOnlyList<GrpcServiceRegistration> registrations,
        string? publicUriOverride)
    {
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.ConfigureKestrel(serverOptions =>
        {
            serverOptions.Listen(ResolveAddress(host), port, listenOptions =>
            {
                listenOptions.Protocols = HttpProtocols.Http2;
            });
        });

        builder.Services.AddGrpc();
        foreach (var registration in registrations)
            registration.ConfigureServices(builder.Services);

        var app = builder.Build();
        foreach (var registration in registrations)
            registration.MapEndpoints(app);
        app.Start();

        var publicUri = publicUriOverride ?? ResolvePublicUri(app, host);
        return new StartedApplication(app, publicUri);
    }

    private static bool MaybeAddDescribe(
        List<GrpcServiceRegistration> registrations,
        ServeOptions options)
    {
        if (!options.Describe)
            return false;

        try
        {
            registrations.Add(Describe.Registration());
            return true;
        }
        catch (Exception error)
        {
            options.Logger($"HolonMeta registration failed: {error.Message}");
            throw;
        }
    }

    private static bool MaybeAddReflection(
        List<GrpcServiceRegistration> registrations,
        ServeOptions options)
    {
        if (!options.Reflect)
            return false;

        registrations.Add(new GrpcServiceRegistration(
            services => services.AddGrpcReflection(),
            app => app.MapGrpcReflectionService()));
        return true;
    }

    private static string FormatMode(bool describeEnabled, bool reflectionEnabled) =>
        $"{(describeEnabled ? "Describe ON" : "Describe OFF")}, {(reflectionEnabled ? "reflection ON" : "reflection OFF")}";

    private static string ResolvePublicUri(WebApplication app, string requestedHost)
    {
        var addresses = app.Services
            .GetRequiredService<IServer>()
            .Features
            .Get<IServerAddressesFeature>()?
            .Addresses;
        var address = addresses?.FirstOrDefault() ?? throw new InvalidOperationException("Kestrel did not report a bound address");
        var uri = new Uri(address);
        var host = AdvertisedHost(requestedHost);
        return $"tcp://{host}:{uri.Port}";
    }

    private static string AdvertisedHost(string host) =>
        host switch
        {
            "" or "0.0.0.0" => "127.0.0.1",
            "::" => "::1",
            _ => host
        };

    private static IPAddress ResolveAddress(string host)
    {
        if (host == "0.0.0.0")
            return IPAddress.Any;
        if (host == "::")
            return IPAddress.IPv6Any;
        if (IPAddress.TryParse(host, out var ip))
            return ip;
        return Dns.GetHostAddresses(host).First();
    }

    private static (string Host, int Port) ParseTarget(string uri)
    {
        var parsed = Transport.ParseUri(uri);
        if (parsed.Scheme != "tcp")
            throw new ArgumentException($"unexpected listen uri {uri}", nameof(uri));
        var host = string.IsNullOrWhiteSpace(parsed.Host) ? "127.0.0.1" : parsed.Host;
        return (host!, parsed.Port ?? 9090);
    }

    private sealed class StdioServerBridge : IAsyncDisposable
    {
        private readonly TcpClient _client;
        private readonly NetworkStream _network;
        private readonly Action _onDisconnect;
        private readonly CancellationTokenSource _cts = new();
        private readonly Task[] _pumps;
        private int _completed;

        public StdioServerBridge(string host, int port, Action onDisconnect)
        {
            _client = new TcpClient();
            _client.Connect(host, port);
            _network = _client.GetStream();
            _onDisconnect = onDisconnect;
            _pumps =
            [
                Task.Run(PumpStdInAsync),
                Task.Run(PumpStdOutAsync),
            ];
        }

        public void Start()
        {
            foreach (var pump in _pumps)
            {
                pump.ContinueWith(
                    _ =>
                    {
                        if (Interlocked.Increment(ref _completed) == _pumps.Length)
                            _onDisconnect();
                    },
                    TaskScheduler.Default);
            }
        }

        public async ValueTask DisposeAsync()
        {
            _cts.Cancel();
            try
            {
                _client.Close();
            }
            catch
            {
                // ignored
            }

            try
            {
                await Task.WhenAll(_pumps).ConfigureAwait(false);
            }
            catch
            {
                // ignored
            }

            _cts.Dispose();
        }

        private async Task PumpStdInAsync()
        {
            var input = Console.OpenStandardInput();
            var buffer = new byte[16 * 1024];

            try
            {
                while (!_cts.IsCancellationRequested)
                {
                    var read = await input.ReadAsync(buffer.AsMemory(0, buffer.Length), _cts.Token).ConfigureAwait(false);
                    if (read <= 0)
                    {
                        _client.Client.Shutdown(SocketShutdown.Send);
                        break;
                    }

                    await _network.WriteAsync(buffer.AsMemory(0, read), _cts.Token).ConfigureAwait(false);
                    await _network.FlushAsync(_cts.Token).ConfigureAwait(false);
                }
            }
            catch
            {
                // Stream closed during shutdown.
            }
        }

        private async Task PumpStdOutAsync()
        {
            var output = Console.OpenStandardOutput();
            var buffer = new byte[16 * 1024];

            try
            {
                while (!_cts.IsCancellationRequested)
                {
                    var read = await _network.ReadAsync(buffer.AsMemory(0, buffer.Length), _cts.Token).ConfigureAwait(false);
                    if (read <= 0)
                        break;

                    await output.WriteAsync(buffer.AsMemory(0, read), _cts.Token).ConfigureAwait(false);
                    await output.FlushAsync(_cts.Token).ConfigureAwait(false);
                }
            }
            catch
            {
                // Stream closed during shutdown.
            }
        }
    }

    private sealed class UnixServerBridge : IAsyncDisposable
    {
        private readonly Socket _listener;
        private readonly string _path;
        private readonly string _host;
        private readonly int _port;
        private readonly CancellationTokenSource _cts = new();
        private readonly object _connectionsGate = new();
        private readonly HashSet<IDisposable> _connections = [];
        private Task? _acceptLoop;

        public UnixServerBridge(string path, string host, int port)
        {
            var listener = Transport.Listen($"unix://{path}");
            _listener = listener switch
            {
                Transport.TransportListener.Unix unix => unix.Socket,
                _ => throw new ArgumentException($"expected unix listener for {path}", nameof(path)),
            };
            _path = path;
            _host = host;
            _port = port;
        }

        public void Start()
        {
            _acceptLoop = Task.Run(AcceptLoopAsync);
        }

        public async ValueTask DisposeAsync()
        {
            _cts.Cancel();
            try
            {
                _listener.Close();
            }
            catch
            {
                // ignored
            }

            IDisposable[] active;
            lock (_connectionsGate)
            {
                active = _connections.ToArray();
                _connections.Clear();
            }

            foreach (var connection in active)
            {
                try { connection.Dispose(); } catch { }
            }

            if (_acceptLoop is not null)
            {
                try
                {
                    await _acceptLoop.ConfigureAwait(false);
                }
                catch
                {
                    // ignored during shutdown
                }
            }

            try
            {
                if (File.Exists(_path))
                    File.Delete(_path);
            }
            catch
            {
                // ignored
            }

            _cts.Dispose();
        }

        private async Task AcceptLoopAsync()
        {
            while (!_cts.IsCancellationRequested)
            {
                Socket? client = null;
                try
                {
                    client = await _listener.AcceptAsync(_cts.Token).ConfigureAwait(false);
                    if (_cts.IsCancellationRequested)
                    {
                        client.Dispose();
                        return;
                    }

                    _ = Task.Run(() => HandleClientAsync(client), _cts.Token);
                }
                catch (OperationCanceledException)
                {
                    client?.Dispose();
                    return;
                }
                catch
                {
                    client?.Dispose();
                    if (_cts.IsCancellationRequested)
                        return;
                }
            }
        }

        private async Task HandleClientAsync(Socket client)
        {
            TcpClient? upstream = null;
            try
            {
                upstream = new TcpClient();
                await upstream.ConnectAsync(_host, _port, _cts.Token).ConfigureAwait(false);

                Register(client);
                Register(upstream);

                using var clientStream = new NetworkStream(client, ownsSocket: false);
                using var upstreamStream = upstream.GetStream();

                var up = PumpAsync(clientStream, upstreamStream, closeOutput: true);
                var down = PumpAsync(upstreamStream, clientStream, closeOutput: true);
                await Task.WhenAll(up, down).ConfigureAwait(false);
            }
            catch
            {
                // Closed during shutdown or client disconnect.
            }
            finally
            {
                Unregister(client);
                try { client.Dispose(); } catch { }
                if (upstream is not null)
                {
                    Unregister(upstream);
                    try { upstream.Dispose(); } catch { }
                }
            }
        }

        private void Register(IDisposable connection)
        {
            lock (_connectionsGate)
            {
                if (_cts.IsCancellationRequested)
                {
                    connection.Dispose();
                    return;
                }
                _connections.Add(connection);
            }
        }

        private void Unregister(IDisposable connection)
        {
            lock (_connectionsGate)
            {
                _connections.Remove(connection);
            }
        }

        private async Task PumpAsync(Stream input, Stream output, bool closeOutput)
        {
            var buffer = new byte[16 * 1024];
            try
            {
                while (!_cts.IsCancellationRequested)
                {
                    var read = await input.ReadAsync(buffer.AsMemory(0, buffer.Length), _cts.Token).ConfigureAwait(false);
                    if (read <= 0)
                    {
                        if (closeOutput)
                        {
                            try { output.Close(); } catch { }
                        }
                        return;
                    }

                    await output.WriteAsync(buffer.AsMemory(0, read), _cts.Token).ConfigureAwait(false);
                    await output.FlushAsync(_cts.Token).ConfigureAwait(false);
                }
            }
            catch
            {
                // Closed during shutdown.
            }
        }
    }
}

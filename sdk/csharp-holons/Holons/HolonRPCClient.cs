using System.Collections.Concurrent;
using System.Globalization;
using System.Net.Http;
using System.Net.Security;
using System.Net.WebSockets;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace Holons;

public sealed class HolonRPCResponseException : Exception
{
    public HolonRPCResponseException(int code, string message, JsonNode? data = null)
        : base($"rpc error {code}: {message}")
    {
        Code = code;
        ErrorData = data;
    }

    public int Code { get; }
    public JsonNode? ErrorData { get; }
}

public sealed record HolonRPCSseEvent(
    string Event,
    string Id,
    JsonObject? Result = null,
    HolonRPCResponseException? Error = null);

public sealed class HolonRPCClient : IAsyncDisposable
{
    private readonly ConcurrentDictionary<string, Func<JsonObject, Task<JsonObject>>> _handlers = new();
    private readonly ConcurrentDictionary<string, TaskCompletionSource<JsonObject>> _pending = new();
    private readonly SemaphoreSlim _sendLock = new(1, 1);
    private readonly object _stateLock = new();
    private readonly RemoteCertificateValidationCallback? _serverCertificateValidation;

    private ClientWebSocket? _socket;
    private HttpClient? _httpClient;
    private Uri? _httpBaseUri;
    private Uri? _endpoint;
    private CancellationTokenSource? _receiveLoopCts;
    private Task? _receiveLoopTask;
    private CancellationTokenSource? _heartbeatCts;
    private Task? _heartbeatTask;
    private TaskCompletionSource<bool> _connectedTcs = NewConnectionTcs();
    private TransportMode _transportMode = TransportMode.WebSocket;

    private volatile bool _closed = true;
    private volatile bool _reconnectLoopRunning;
    private long _nextID;
    private int _reconnectAttempt;

    public HolonRPCClient(
        int heartbeatIntervalMs = 15000,
        int heartbeatTimeoutMs = 5000,
        int reconnectMinDelayMs = 500,
        int reconnectMaxDelayMs = 30000,
        double reconnectFactor = 2.0,
        double reconnectJitter = 0.1,
        int connectTimeoutMs = 10000,
        int requestTimeoutMs = 10000,
        RemoteCertificateValidationCallback? serverCertificateValidation = null)
    {
        HeartbeatIntervalMs = heartbeatIntervalMs;
        HeartbeatTimeoutMs = heartbeatTimeoutMs;
        ReconnectMinDelayMs = reconnectMinDelayMs;
        ReconnectMaxDelayMs = reconnectMaxDelayMs;
        ReconnectFactor = reconnectFactor;
        ReconnectJitter = reconnectJitter;
        ConnectTimeoutMs = connectTimeoutMs;
        RequestTimeoutMs = requestTimeoutMs;
        _serverCertificateValidation = serverCertificateValidation;
    }

    public int HeartbeatIntervalMs { get; }
    public int HeartbeatTimeoutMs { get; }
    public int ReconnectMinDelayMs { get; }
    public int ReconnectMaxDelayMs { get; }
    public double ReconnectFactor { get; }
    public double ReconnectJitter { get; }
    public int ConnectTimeoutMs { get; }
    public int RequestTimeoutMs { get; }

    public async Task ConnectAsync(string url, CancellationToken cancellationToken = default)
    {
        if (string.IsNullOrWhiteSpace(url))
            throw new ArgumentException("url is required", nameof(url));

        await CloseAsync().ConfigureAwait(false);
        _closed = false;
        _endpoint = new Uri(url);
        _connectedTcs = NewConnectionTcs();
        _transportMode = ResolveTransportMode(_endpoint);

        if (_transportMode == TransportMode.Http)
        {
            _httpBaseUri = NormalizeHttpBaseUri(_endpoint);
            _httpClient = CreateHttpClient();
            _connectedTcs.TrySetResult(true);
        }
        else
        {
            await OpenSocketAsync(initial: true, cancellationToken).ConfigureAwait(false);
        }
        await WaitConnectedAsync(ConnectTimeoutMs, cancellationToken).ConfigureAwait(false);
    }

    public void Register(string method, Func<JsonObject, Task<JsonObject>> handler)
    {
        if (string.IsNullOrWhiteSpace(method))
            throw new ArgumentException("method is required", nameof(method));
        ArgumentNullException.ThrowIfNull(handler);
        _handlers[method] = handler;
    }

    public async Task<JsonObject> InvokeAsync(
        string method,
        JsonObject? @params = null,
        int? timeoutMs = null,
        CancellationToken cancellationToken = default)
    {
        if (string.IsNullOrWhiteSpace(method))
            throw new ArgumentException("method is required", nameof(method));

        await WaitConnectedAsync(ConnectTimeoutMs, cancellationToken).ConfigureAwait(false);

        if (_transportMode == TransportMode.Http)
        {
            return await InvokeHttpAsync(
                method,
                @params,
                timeoutMs ?? RequestTimeoutMs,
                cancellationToken).ConfigureAwait(false);
        }

        var id = $"c{Interlocked.Increment(ref _nextID)}";
        var tcs = new TaskCompletionSource<JsonObject>(TaskCreationOptions.RunContinuationsAsynchronously);
        if (!_pending.TryAdd(id, tcs))
            throw new InvalidOperationException($"pending request already exists for id {id}");

        var payload = new JsonObject
        {
            ["jsonrpc"] = "2.0",
            ["id"] = id,
            ["method"] = method,
            ["params"] = (@params ?? new JsonObject()).DeepClone()
        };

        try
        {
            await SendAsync(payload, cancellationToken).ConfigureAwait(false);
        }
        catch
        {
            _pending.TryRemove(id, out _);
            throw;
        }

        var timeout = TimeSpan.FromMilliseconds(timeoutMs ?? RequestTimeoutMs);

        try
        {
            return await tcs.Task.WaitAsync(timeout, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            _pending.TryRemove(id, out _);
        }
    }

    public async Task<IReadOnlyList<HolonRPCSseEvent>> StreamAsync(
        string method,
        JsonObject? @params = null,
        CancellationToken cancellationToken = default)
    {
        if (_transportMode != TransportMode.Http)
            throw new InvalidOperationException("HTTP+SSE streaming requires an http:// or https:// endpoint");

        await WaitConnectedAsync(ConnectTimeoutMs, cancellationToken).ConfigureAwait(false);
        return await StreamHttpAsync(
            method,
            httpMethod: HttpMethod.Post,
            body: @params ?? new JsonObject(),
            query: null,
            cancellationToken).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<HolonRPCSseEvent>> StreamQueryAsync(
        string method,
        IReadOnlyDictionary<string, string>? query = null,
        CancellationToken cancellationToken = default)
    {
        if (_transportMode != TransportMode.Http)
            throw new InvalidOperationException("HTTP+SSE streaming requires an http:// or https:// endpoint");

        await WaitConnectedAsync(ConnectTimeoutMs, cancellationToken).ConfigureAwait(false);
        return await StreamHttpAsync(
            method,
            httpMethod: HttpMethod.Get,
            body: null,
            query: query,
            cancellationToken).ConfigureAwait(false);
    }

    public async Task CloseAsync()
    {
        _closed = true;
        CancelHeartbeat();

        ClientWebSocket? socket;
        HttpClient? httpClient;
        CancellationTokenSource? receiveCts;
        Task? receiveTask;

        lock (_stateLock)
        {
            socket = _socket;
            _socket = null;
            httpClient = _httpClient;
            _httpClient = null;
            _httpBaseUri = null;
            receiveCts = _receiveLoopCts;
            _receiveLoopCts = null;
            receiveTask = _receiveLoopTask;
            _receiveLoopTask = null;
            _connectedTcs = NewConnectionTcs();
        }

        if (receiveCts is not null)
        {
            try
            {
                receiveCts.Cancel();
            }
            catch
            {
                // ignored
            }
        }

        if (socket is not null)
        {
            try
            {
                if (socket.State == WebSocketState.Open || socket.State == WebSocketState.CloseReceived)
                {
                    await socket.CloseAsync(
                        WebSocketCloseStatus.NormalClosure,
                        "client close",
                        CancellationToken.None).ConfigureAwait(false);
                }
            }
            catch
            {
                // ignored
            }
            finally
            {
                socket.Dispose();
            }
        }

        if (receiveTask is not null)
        {
            try
            {
                await receiveTask.ConfigureAwait(false);
            }
            catch
            {
                // ignored
            }
        }

        receiveCts?.Dispose();
        httpClient?.Dispose();
        FailAllPending(new InvalidOperationException("holon-rpc client closed"));
    }

    public async ValueTask DisposeAsync()
    {
        await CloseAsync().ConfigureAwait(false);
        _sendLock.Dispose();
    }

    private async Task OpenSocketAsync(bool initial, CancellationToken cancellationToken)
    {
        if (_closed)
            return;

        if (_endpoint is null)
            throw new InvalidOperationException("endpoint is not set");

        var ws = new ClientWebSocket();
        ws.Options.AddSubProtocol("holon-rpc");
        if (_serverCertificateValidation is not null)
            ws.Options.RemoteCertificateValidationCallback = _serverCertificateValidation;

        using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeoutCts.CancelAfter(ConnectTimeoutMs);

        try
        {
            await ws.ConnectAsync(_endpoint, timeoutCts.Token).ConfigureAwait(false);

            if (!string.Equals(ws.SubProtocol, "holon-rpc", StringComparison.Ordinal))
            {
                await ws.CloseAsync(
                    WebSocketCloseStatus.ProtocolError,
                    "missing holon-rpc subprotocol",
                    CancellationToken.None).ConfigureAwait(false);
                throw new InvalidOperationException("server did not negotiate holon-rpc subprotocol");
            }

            var loopCts = new CancellationTokenSource();

            lock (_stateLock)
            {
                _socket = ws;
                _receiveLoopCts = loopCts;
                _receiveLoopTask = Task.Run(() => ReceiveLoopAsync(ws, loopCts.Token));
                _connectedTcs.TrySetResult(true);
            }

            _reconnectAttempt = 0;
            StartHeartbeat();
        }
        catch
        {
            ws.Dispose();
            if (initial)
                throw;

            StartReconnectLoop();
        }
    }

    private async Task ReceiveLoopAsync(ClientWebSocket ws, CancellationToken cancellationToken)
    {
        var buffer = new byte[16 * 1024];
        using var messageBuffer = new MemoryStream();

        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                var result = await ws.ReceiveAsync(buffer, cancellationToken).ConfigureAwait(false);

                if (result.MessageType == WebSocketMessageType.Close)
                    break;

                if (result.MessageType != WebSocketMessageType.Text)
                    continue;

                messageBuffer.Write(buffer, 0, result.Count);
                if (!result.EndOfMessage)
                    continue;

                var text = Encoding.UTF8.GetString(messageBuffer.ToArray());
                messageBuffer.SetLength(0);
                await HandleIncomingAsync(text, cancellationToken).ConfigureAwait(false);
            }
        }
        catch (OperationCanceledException)
        {
            // expected on shutdown
        }
        catch
        {
            // transport error -> reconnect path
        }
        finally
        {
            await HandleDisconnectAsync().ConfigureAwait(false);
        }
    }

    private async Task HandleIncomingAsync(string text, CancellationToken cancellationToken)
    {
        JsonNode? node;
        try
        {
            node = JsonNode.Parse(text);
        }
        catch
        {
            return;
        }

        if (node is not JsonObject obj)
            return;

        if (obj["method"] is not null)
        {
            await HandleRequestAsync(obj, cancellationToken).ConfigureAwait(false);
            return;
        }

        if (obj["result"] is not null || obj["error"] is not null)
            HandleResponse(obj);
    }

    private async Task HandleRequestAsync(JsonObject msg, CancellationToken cancellationToken)
    {
        var idNode = msg["id"];
        var jsonrpc = msg["jsonrpc"]?.GetValue<string>();
        var method = msg["method"]?.GetValue<string>();

        if (!string.Equals(jsonrpc, "2.0", StringComparison.Ordinal) || string.IsNullOrWhiteSpace(method))
        {
            if (idNode is not null)
                await SendErrorAsync(idNode, -32600, "invalid request", null, cancellationToken).ConfigureAwait(false);
            return;
        }

        if (method == "rpc.heartbeat")
        {
            if (idNode is not null)
                await SendResultAsync(idNode, new JsonObject(), cancellationToken).ConfigureAwait(false);
            return;
        }

        if (idNode is not null)
        {
            var sid = IdNodeToKey(idNode);
            if (sid is null || !sid.StartsWith("s", StringComparison.Ordinal))
            {
                await SendErrorAsync(
                    idNode,
                    -32600,
                    "server request id must start with 's'",
                    null,
                    cancellationToken).ConfigureAwait(false);
                return;
            }
        }

        if (!_handlers.TryGetValue(method, out var handler))
        {
            if (idNode is not null)
                await SendErrorAsync(idNode, -32601, $"method \"{method}\" not found", null, cancellationToken)
                    .ConfigureAwait(false);
            return;
        }

        var paramsObject = msg["params"] as JsonObject ?? new JsonObject();

        try
        {
            var result = await handler((JsonObject)paramsObject.DeepClone()).ConfigureAwait(false);
            if (idNode is not null)
                await SendResultAsync(idNode, result ?? new JsonObject(), cancellationToken).ConfigureAwait(false);
        }
        catch (HolonRPCResponseException rpcError)
        {
            if (idNode is not null)
            {
                await SendErrorAsync(idNode, rpcError.Code, rpcError.Message, rpcError.ErrorData, cancellationToken)
                    .ConfigureAwait(false);
            }
        }
        catch (Exception error)
        {
            if (idNode is not null)
                await SendErrorAsync(idNode, 13, error.Message, null, cancellationToken).ConfigureAwait(false);
        }
    }

    private void HandleResponse(JsonObject msg)
    {
        var id = IdNodeToKey(msg["id"]);
        if (id is null)
            return;

        if (!_pending.TryRemove(id, out var tcs))
            return;

        if (msg["error"] is JsonObject errorObj)
        {
            var code = errorObj["code"]?.GetValue<int>() ?? -32603;
            var message = errorObj["message"]?.GetValue<string>() ?? "internal error";
            tcs.TrySetException(new HolonRPCResponseException(code, message, errorObj["data"]?.DeepClone()));
            return;
        }

        var result = msg["result"] as JsonObject ?? new JsonObject();
        tcs.TrySetResult((JsonObject)result.DeepClone());
    }

    private async Task HandleDisconnectAsync()
    {
        CancelHeartbeat();
        FailAllPending(new InvalidOperationException("holon-rpc connection closed"));

        ClientWebSocket? socketToDispose = null;
        CancellationTokenSource? receiveCtsToDispose = null;

        lock (_stateLock)
        {
            if (_socket is null && _closed)
                return;

            socketToDispose = _socket;
            receiveCtsToDispose = _receiveLoopCts;
            _socket = null;
            _receiveLoopCts = null;
            _receiveLoopTask = null;
            _connectedTcs = NewConnectionTcs();
        }

        if (receiveCtsToDispose is not null)
        {
            try
            {
                receiveCtsToDispose.Cancel();
            }
            catch
            {
                // ignored
            }
            receiveCtsToDispose.Dispose();
        }

        socketToDispose?.Dispose();

        if (!_closed)
            StartReconnectLoop();

        await Task.CompletedTask;
    }

    private void StartReconnectLoop()
    {
        if (_reconnectLoopRunning || _closed)
            return;

        _reconnectLoopRunning = true;

        _ = Task.Run(async () =>
        {
            try
            {
                while (!_closed)
                {
                    var delayMs = ComputeReconnectDelayMs(_reconnectAttempt++);
                    await Task.Delay(delayMs).ConfigureAwait(false);
                    if (_closed)
                        return;

                    try
                    {
                        await OpenSocketAsync(initial: false, CancellationToken.None).ConfigureAwait(false);
                        if (_socket is not null && _socket.State == WebSocketState.Open)
                            return;
                    }
                    catch
                    {
                        // continue loop
                    }
                }
            }
            finally
            {
                _reconnectLoopRunning = false;
            }
        });
    }

    private int ComputeReconnectDelayMs(int attempt)
    {
        var baseDelay = Math.Min(
            ReconnectMinDelayMs * Math.Pow(ReconnectFactor, attempt),
            ReconnectMaxDelayMs);
        var jitter = baseDelay * ReconnectJitter * Random.Shared.NextDouble();
        return Math.Max(1, (int)Math.Round(baseDelay + jitter));
    }

    private void StartHeartbeat()
    {
        CancelHeartbeat();
        var heartbeatCts = new CancellationTokenSource();
        var ct = heartbeatCts.Token;
        _heartbeatCts = heartbeatCts;

        _heartbeatTask = Task.Run(async () =>
        {
            var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(HeartbeatIntervalMs));
            try
            {
                while (await timer.WaitForNextTickAsync(ct).ConfigureAwait(false))
                {
                    if (_closed || _socket is null)
                        return;

                    try
                    {
                        await InvokeAsync(
                            "rpc.heartbeat",
                            new JsonObject(),
                            HeartbeatTimeoutMs,
                            ct).ConfigureAwait(false);
                    }
                    catch
                    {
                        try
                        {
                            _socket?.Abort();
                        }
                        catch
                        {
                            // ignored
                        }
                        return;
                    }
                }
            }
            catch (OperationCanceledException)
            {
                // normal shutdown path
            }
            finally
            {
                timer.Dispose();
            }
        }, ct);
    }

    private void CancelHeartbeat()
    {
        var heartbeatCts = Interlocked.Exchange(ref _heartbeatCts, null);
        if (heartbeatCts is not null)
        {
            try
            {
                heartbeatCts.Cancel();
            }
            catch
            {
                // ignored
            }
            heartbeatCts.Dispose();
        }

        Interlocked.Exchange(ref _heartbeatTask, null);
    }

    private async Task WaitConnectedAsync(int timeoutMs, CancellationToken cancellationToken)
    {
        if (_transportMode == TransportMode.Http && _httpClient is not null && _httpBaseUri is not null)
            return;
        if (_socket is not null && _socket.State == WebSocketState.Open)
            return;
        if (_closed)
            throw new InvalidOperationException("holon-rpc client closed");

        await _connectedTcs.Task
            .WaitAsync(TimeSpan.FromMilliseconds(timeoutMs), cancellationToken)
            .ConfigureAwait(false);
    }

    private async Task<JsonObject> InvokeHttpAsync(
        string method,
        JsonObject? @params,
        int timeoutMs,
        CancellationToken cancellationToken)
    {
        using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeoutCts.CancelAfter(timeoutMs);

        using var request = new HttpRequestMessage(HttpMethod.Post, MethodUri(method))
        {
            Content = JsonContent(@params ?? new JsonObject())
        };
        request.Headers.Accept.ParseAdd("application/json");

        using var response = await RequireHttpClient()
            .SendAsync(request, HttpCompletionOption.ResponseHeadersRead, timeoutCts.Token)
            .ConfigureAwait(false);

        return await ParseHttpRPCResponseAsync(response, timeoutCts.Token).ConfigureAwait(false);
    }

    private async Task<IReadOnlyList<HolonRPCSseEvent>> StreamHttpAsync(
        string method,
        HttpMethod httpMethod,
        JsonObject? body,
        IReadOnlyDictionary<string, string>? query,
        CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(httpMethod, MethodUri(method, query));
        request.Headers.Accept.ParseAdd("text/event-stream");
        if (httpMethod == HttpMethod.Post)
            request.Content = JsonContent(body ?? new JsonObject());

        using var response = await RequireHttpClient()
            .SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken)
            .ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
            await ParseHttpRPCResponseAsync(response, cancellationToken).ConfigureAwait(false);

        var contentType = response.Content.Headers.ContentType?.MediaType ?? "";
        if (!string.Equals(contentType, "text/event-stream", StringComparison.OrdinalIgnoreCase))
            throw new InvalidOperationException($"unexpected SSE content-type \"{contentType}\"");

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        using var reader = new StreamReader(stream, Encoding.UTF8, detectEncodingFromByteOrderMarks: true);

        var events = new List<HolonRPCSseEvent>();
        var eventType = "";
        var eventID = "";
        var dataLines = new List<string>();

        while (!reader.EndOfStream)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var line = await reader.ReadLineAsync(cancellationToken).ConfigureAwait(false);
            if (line is null)
                break;

            if (line.Length == 0)
            {
                var parsed = ParseSseEvent(eventType, eventID, dataLines);
                if (parsed is not null)
                    events.Add(parsed);
                eventType = "";
                eventID = "";
                dataLines.Clear();
                continue;
            }

            if (line.StartsWith(':'))
                continue;

            var separator = line.IndexOf(':');
            var field = separator >= 0 ? line[..separator] : line;
            var value = separator >= 0 ? line[(separator + 1)..].TrimStart(' ') : "";

            switch (field)
            {
                case "event":
                    eventType = value;
                    break;
                case "id":
                    eventID = value;
                    break;
                case "data":
                    dataLines.Add(value);
                    break;
            }
        }

        var finalEvent = ParseSseEvent(eventType, eventID, dataLines);
        if (finalEvent is not null)
            events.Add(finalEvent);

        return events;
    }

    private async Task SendAsync(JsonObject payload, CancellationToken cancellationToken)
    {
        var socket = _socket;
        if (socket is null || socket.State != WebSocketState.Open)
            throw new InvalidOperationException("websocket is not connected");

        var data = Encoding.UTF8.GetBytes(payload.ToJsonString());

        await _sendLock.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            await socket.SendAsync(
                data,
                WebSocketMessageType.Text,
                endOfMessage: true,
                cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            _sendLock.Release();
        }
    }

    private Task SendResultAsync(
        JsonNode idNode,
        JsonObject result,
        CancellationToken cancellationToken)
    {
        var payload = new JsonObject
        {
            ["jsonrpc"] = "2.0",
            ["id"] = idNode.DeepClone(),
            ["result"] = (result ?? new JsonObject()).DeepClone()
        };
        return SendAsync(payload, cancellationToken);
    }

    private Task SendErrorAsync(
        JsonNode idNode,
        int code,
        string message,
        JsonNode? data,
        CancellationToken cancellationToken)
    {
        var errorObject = new JsonObject
        {
            ["code"] = code,
            ["message"] = message
        };
        if (data is not null)
            errorObject["data"] = data.DeepClone();

        var payload = new JsonObject
        {
            ["jsonrpc"] = "2.0",
            ["id"] = idNode.DeepClone(),
            ["error"] = errorObject
        };
        return SendAsync(payload, cancellationToken);
    }

    private void FailAllPending(Exception error)
    {
        if (_pending.IsEmpty)
            return;

        foreach (var (_, tcs) in _pending)
            tcs.TrySetException(error);

        _pending.Clear();
    }

    private static string? IdNodeToKey(JsonNode? idNode)
    {
        if (idNode is null)
            return null;

        if (idNode is JsonValue value)
        {
            if (value.TryGetValue<string>(out var str))
                return str;
            if (value.TryGetValue<int>(out var i))
                return i.ToString(CultureInfo.InvariantCulture);
            if (value.TryGetValue<long>(out var l))
                return l.ToString(CultureInfo.InvariantCulture);
            if (value.TryGetValue<double>(out var d))
                return d.ToString(CultureInfo.InvariantCulture);
            if (value.TryGetValue<decimal>(out var m))
                return m.ToString(CultureInfo.InvariantCulture);
        }

        return null;
    }

    private static TaskCompletionSource<bool> NewConnectionTcs() =>
        new(TaskCreationOptions.RunContinuationsAsynchronously);

    private enum TransportMode
    {
        WebSocket,
        Http,
    }

    private static TransportMode ResolveTransportMode(Uri endpoint)
    {
        return endpoint.Scheme.ToLowerInvariant() switch
        {
            "ws" or "wss" => TransportMode.WebSocket,
            "http" or "https" => TransportMode.Http,
            _ => throw new ArgumentException($"unsupported holon-rpc URL scheme \"{endpoint.Scheme}\"", nameof(endpoint)),
        };
    }

    private HttpClient RequireHttpClient()
    {
        return _httpClient ?? throw new InvalidOperationException("HTTP client is not connected");
    }

    private HttpClient CreateHttpClient()
    {
        if (_serverCertificateValidation is null)
            return new HttpClient();

        var handler = new HttpClientHandler
        {
            ServerCertificateCustomValidationCallback = (_, certificate, chain, errors) =>
                _serverCertificateValidation(this, certificate, chain, errors)
        };
        return new HttpClient(handler, disposeHandler: true);
    }

    private static Uri NormalizeHttpBaseUri(Uri endpoint)
    {
        var builder = new UriBuilder(endpoint);
        if (string.IsNullOrWhiteSpace(builder.Path) || builder.Path == "/")
            builder.Path = "/api/v1/rpc";
        builder.Path = builder.Path.TrimEnd('/');
        return builder.Uri;
    }

    private Uri MethodUri(string method, IReadOnlyDictionary<string, string>? query = null)
    {
        if (_httpBaseUri is null)
            throw new InvalidOperationException("HTTP endpoint is not connected");
        if (string.IsNullOrWhiteSpace(method))
            throw new ArgumentException("method is required", nameof(method));

        var uri = new Uri(_httpBaseUri.AbsoluteUri.TrimEnd('/') + "/" + method.TrimStart('/'));
        if (query is null || query.Count == 0)
            return uri;

        var builder = new UriBuilder(uri)
        {
            Query = string.Join("&", query.Select(pair =>
                $"{Uri.EscapeDataString(pair.Key)}={Uri.EscapeDataString(pair.Value)}"))
        };
        return builder.Uri;
    }

    private static StringContent JsonContent(JsonObject payload)
    {
        return new StringContent(payload.ToJsonString(), Encoding.UTF8, "application/json");
    }

    private static async Task<JsonObject> ParseHttpRPCResponseAsync(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        var node = await JsonNode.ParseAsync(stream, cancellationToken: cancellationToken).ConfigureAwait(false);
        if (node is not JsonObject payload)
            throw new InvalidOperationException("invalid JSON-RPC response payload");

        if (payload["error"] is JsonObject errorObject)
        {
            var code = errorObject["code"]?.GetValue<int>() ?? 13;
            var message = errorObject["message"]?.GetValue<string>() ?? "internal error";
            throw new HolonRPCResponseException(code, message, errorObject["data"]?.DeepClone());
        }

        return payload["result"] as JsonObject ?? new JsonObject();
    }

    private static HolonRPCSseEvent? ParseSseEvent(string eventType, string eventID, IReadOnlyList<string> dataLines)
    {
        if (string.IsNullOrWhiteSpace(eventType) && string.IsNullOrWhiteSpace(eventID) && dataLines.Count == 0)
            return null;

        var resolvedEvent = string.IsNullOrWhiteSpace(eventType) ? "message" : eventType;
        var data = string.Join("\n", dataLines);
        if (string.IsNullOrWhiteSpace(data))
            return new HolonRPCSseEvent(resolvedEvent, eventID);

        var payload = JsonNode.Parse(data) as JsonObject
            ?? throw new InvalidOperationException("invalid SSE JSON-RPC payload");

        if (payload["error"] is JsonObject errorObject)
        {
            var code = errorObject["code"]?.GetValue<int>() ?? 13;
            var message = errorObject["message"]?.GetValue<string>() ?? "internal error";
            return new HolonRPCSseEvent(
                resolvedEvent,
                eventID,
                Error: new HolonRPCResponseException(code, message, errorObject["data"]?.DeepClone()));
        }

        return new HolonRPCSseEvent(
            resolvedEvent,
            eventID,
            Result: payload["result"] as JsonObject ?? new JsonObject());
    }
}

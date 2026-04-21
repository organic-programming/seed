package org.organicprogramming.holons;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.reflect.TypeToken;

import java.lang.reflect.Type;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.WebSocket;
import java.time.Duration;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.ThreadLocalRandom;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Holon-RPC JSON-RPC 2.0 client over WebSocket using the {@code holon-rpc}
 * subprotocol.
 */
public final class HolonRPCClient {
    /**
     * Handler for server-initiated calls.
     */
    @FunctionalInterface
    public interface Handler {
        CompletionStage<Map<String, Object>> handle(Map<String, Object> params);
    }

    /**
     * JSON-RPC error response exception.
     */
    public static final class HolonRPCResponseException extends RuntimeException {
        private final int code;
        private final Object data;

        public HolonRPCResponseException(int code, String message, Object data) {
            super("rpc error " + code + ": " + message);
            this.code = code;
            this.data = data;
        }

        public int code() {
            return code;
        }

        public Object data() {
            return data;
        }
    }

    private static final Type MAP_TYPE = new TypeToken<Map<String, Object>>() {
    }.getType();

    private final HttpClient httpClient;
    private final Gson gson;

    private final long heartbeatIntervalMs;
    private final long heartbeatTimeoutMs;
    private final long reconnectMinDelayMs;
    private final long reconnectMaxDelayMs;
    private final double reconnectFactor;
    private final double reconnectJitter;
    private final long connectTimeoutMs;
    private final long requestTimeoutMs;

    private final ScheduledExecutorService scheduler;
    private final Map<String, Handler> handlers = new ConcurrentHashMap<>();
    private final Map<String, CompletableFuture<Map<String, Object>>> pending = new ConcurrentHashMap<>();
    private final AtomicLong nextID = new AtomicLong(0);

    private volatile String endpoint;
    private volatile WebSocket socket;
    private volatile ScheduledFuture<?> heartbeatTask;
    private volatile ScheduledFuture<?> reconnectTask;
    private volatile CompletableFuture<Void> connectedFuture = new CompletableFuture<>();
    private volatile boolean closed = true;
    private volatile int reconnectAttempt = 0;

    public HolonRPCClient() {
        this(15_000, 5_000, 500, 30_000, 2.0, 0.1, 10_000, 10_000);
    }

    public HolonRPCClient(
            long heartbeatIntervalMs,
            long heartbeatTimeoutMs,
            long reconnectMinDelayMs,
            long reconnectMaxDelayMs,
            double reconnectFactor,
            double reconnectJitter,
            long connectTimeoutMs,
            long requestTimeoutMs) {
        this.heartbeatIntervalMs = heartbeatIntervalMs;
        this.heartbeatTimeoutMs = heartbeatTimeoutMs;
        this.reconnectMinDelayMs = reconnectMinDelayMs;
        this.reconnectMaxDelayMs = reconnectMaxDelayMs;
        this.reconnectFactor = reconnectFactor;
        this.reconnectJitter = reconnectJitter;
        this.connectTimeoutMs = connectTimeoutMs;
        this.requestTimeoutMs = requestTimeoutMs;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofMillis(connectTimeoutMs))
                .build();
        this.gson = new Gson();
        this.scheduler = Executors.newSingleThreadScheduledExecutor(runnable -> {
            Thread thread = new Thread(runnable, "holon-rpc-client");
            thread.setDaemon(true);
            return thread;
        });
    }

    public void connect(String url) throws Exception {
        Objects.requireNonNull(url, "url");
        if (url.isBlank()) {
            throw new IllegalArgumentException("url is required");
        }

        close();
        closed = false;
        endpoint = url;
        connectedFuture = new CompletableFuture<>();

        openSocket(true);
        awaitConnected(connectTimeoutMs);
    }

    public void register(String method, Handler handler) {
        Objects.requireNonNull(method, "method");
        Objects.requireNonNull(handler, "handler");
        if (method.isBlank()) {
            throw new IllegalArgumentException("method is required");
        }
        handlers.put(method, handler);
    }

    public Map<String, Object> invoke(String method, Map<String, Object> params) throws Exception {
        return invoke(method, params, requestTimeoutMs);
    }

    public Map<String, Object> invoke(String method, Map<String, Object> params, long timeoutMs) throws Exception {
        Objects.requireNonNull(method, "method");
        if (method.isBlank()) {
            throw new IllegalArgumentException("method is required");
        }

        awaitConnected(connectTimeoutMs);
        String id = "c" + nextID.incrementAndGet();
        CompletableFuture<Map<String, Object>> responseFuture = new CompletableFuture<>();
        pending.put(id, responseFuture);

        JsonObject payload = new JsonObject();
        payload.addProperty("jsonrpc", "2.0");
        payload.addProperty("id", id);
        payload.addProperty("method", method);
        payload.add("params", gson.toJsonTree(params != null ? params : Map.of()));

        try {
            send(payload);
        } catch (RuntimeException error) {
            pending.remove(id);
            throw error;
        }

        try {
            return responseFuture.get(timeoutMs, TimeUnit.MILLISECONDS);
        } finally {
            pending.remove(id);
        }
    }

    public synchronized void close() {
        closed = true;
        cancelHeartbeat();
        cancelReconnect();
        failAllPending(new IllegalStateException("holon-rpc client closed"));

        WebSocket ws = socket;
        socket = null;
        if (ws != null) {
            ws.sendClose(1000, "client close");
        }
    }

    private void awaitConnected(long timeoutMs) throws Exception {
        CompletableFuture<Void> cf = connectedFuture;
        try {
            cf.get(timeoutMs, TimeUnit.MILLISECONDS);
        } catch (ExecutionException ee) {
            Throwable cause = ee.getCause();
            if (cause instanceof Exception ex) {
                throw ex;
            }
            throw new RuntimeException(cause);
        }
    }

    private void openSocket(boolean initial) {
        if (closed) {
            return;
        }

        URI uri = URI.create(endpoint);
        httpClient.newWebSocketBuilder()
                .subprotocols("holon-rpc")
                .connectTimeout(Duration.ofMillis(connectTimeoutMs))
                .buildAsync(uri, new ListenerImpl())
                .whenComplete((ws, error) -> {
                    if (error != null) {
                        if (initial && !connectedFuture.isDone()) {
                            connectedFuture.completeExceptionally(error);
                        }
                        if (!closed) {
                            scheduleReconnect();
                        }
                    }
                });
    }

    private void handleIncoming(String text) {
        JsonElement parsed;
        try {
            parsed = JsonParser.parseString(text);
        } catch (Exception ignored) {
            return;
        }

        if (!parsed.isJsonObject()) {
            return;
        }

        JsonObject msg = parsed.getAsJsonObject();
        if (msg.has("method")) {
            handleRequest(msg);
            return;
        }
        if (msg.has("result") || msg.has("error")) {
            handleResponse(msg);
        }
    }

    private void handleRequest(JsonObject msg) {
        JsonElement id = msg.get("id");
        String jsonrpc = asString(msg.get("jsonrpc"));
        String method = asString(msg.get("method"));

        if (!"2.0".equals(jsonrpc) || method == null || method.isBlank()) {
            if (id != null && !id.isJsonNull()) {
                sendError(id, -32600, "invalid request", null);
            }
            return;
        }

        if ("rpc.heartbeat".equals(method)) {
            if (id != null && !id.isJsonNull()) {
                sendResult(id, Map.of());
            }
            return;
        }

        if (id != null && !id.isJsonNull()) {
            String sid = asID(id);
            if (sid == null || !sid.startsWith("s")) {
                sendError(id, -32600, "server request id must start with 's'", null);
                return;
            }
        }

        Handler handler = handlers.get(method);
        if (handler == null) {
            if (id != null && !id.isJsonNull()) {
                sendError(id, -32601, "method \"" + method + "\" not found", null);
            }
            return;
        }

        Map<String, Object> params = toMap(msg.get("params"));
        CompletionStage<Map<String, Object>> stage;
        try {
            stage = handler.handle(params);
        } catch (Exception error) {
            stage = CompletableFuture.failedFuture(error);
        }

        if (id == null || id.isJsonNull()) {
            return;
        }

        stage.whenComplete((result, error) -> {
            if (error != null) {
                Throwable cause = unwrap(error);
                if (cause instanceof HolonRPCResponseException rpcError) {
                    sendError(id, rpcError.code(), rpcError.getMessage(), rpcError.data());
                } else {
                    sendError(id, 13, cause.getMessage() != null ? cause.getMessage() : "internal error", null);
                }
                return;
            }
            sendResult(id, result != null ? result : Map.of());
        });
    }

    private void handleResponse(JsonObject msg) {
        String id = asID(msg.get("id"));
        if (id == null) {
            return;
        }

        CompletableFuture<Map<String, Object>> cf = pending.remove(id);
        if (cf == null || cf.isDone()) {
            return;
        }

        if (msg.has("error") && msg.get("error").isJsonObject()) {
            JsonObject errorObj = msg.getAsJsonObject("error");
            int code = errorObj.has("code") ? errorObj.get("code").getAsInt() : -32603;
            String message = errorObj.has("message") ? asString(errorObj.get("message")) : "internal error";
            Object data = errorObj.has("data") ? gson.fromJson(errorObj.get("data"), Object.class) : null;
            cf.completeExceptionally(new HolonRPCResponseException(code, message, data));
            return;
        }

        cf.complete(toMap(msg.get("result")));
    }

    private synchronized void handleDisconnect() {
        socket = null;
        cancelHeartbeat();
        failAllPending(new IllegalStateException("holon-rpc connection closed"));
        connectedFuture = new CompletableFuture<>();

        if (!closed) {
            scheduleReconnect();
        }
    }

    private synchronized void scheduleReconnect() {
        if (closed || reconnectTask != null) {
            return;
        }

        double base = Math.min(
                reconnectMinDelayMs * Math.pow(reconnectFactor, reconnectAttempt),
                reconnectMaxDelayMs);
        double jitter = base * reconnectJitter * ThreadLocalRandom.current().nextDouble();
        long delayMs = (long) (base + jitter);
        reconnectAttempt++;

        reconnectTask = scheduler.schedule(() -> {
            reconnectTask = null;
            openSocket(false);
        }, delayMs, TimeUnit.MILLISECONDS);
    }

    private synchronized void startHeartbeat() {
        cancelHeartbeat();
        heartbeatTask = scheduler.scheduleAtFixedRate(() -> {
            if (closed || socket == null) {
                return;
            }

            try {
                invoke("rpc.heartbeat", Map.of(), heartbeatTimeoutMs);
            } catch (Exception ignored) {
                WebSocket ws = socket;
                if (ws != null) {
                    ws.abort();
                }
            }
        }, heartbeatIntervalMs, heartbeatIntervalMs, TimeUnit.MILLISECONDS);
    }

    private synchronized void cancelHeartbeat() {
        if (heartbeatTask != null) {
            heartbeatTask.cancel(true);
            heartbeatTask = null;
        }
    }

    private synchronized void cancelReconnect() {
        if (reconnectTask != null) {
            reconnectTask.cancel(true);
            reconnectTask = null;
        }
        reconnectAttempt = 0;
    }

    private void send(JsonObject payload) {
        WebSocket ws = socket;
        if (ws == null) {
            throw new IllegalStateException("websocket is not connected");
        }
        ws.sendText(gson.toJson(payload), true).join();
    }

    private void sendResult(JsonElement id, Map<String, Object> result) {
        JsonObject payload = new JsonObject();
        payload.addProperty("jsonrpc", "2.0");
        payload.add("id", id);
        payload.add("result", gson.toJsonTree(result != null ? result : Map.of()));
        send(payload);
    }

    private void sendError(JsonElement id, int code, String message, Object data) {
        JsonObject payload = new JsonObject();
        payload.addProperty("jsonrpc", "2.0");
        payload.add("id", id);
        JsonObject error = new JsonObject();
        error.addProperty("code", code);
        error.addProperty("message", message);
        if (data != null) {
            error.add("data", gson.toJsonTree(data));
        }
        payload.add("error", error);
        send(payload);
    }

    private void failAllPending(Throwable error) {
        if (pending.isEmpty()) {
            return;
        }
        Map<String, CompletableFuture<Map<String, Object>>> snapshot = Map.copyOf(pending);
        pending.clear();
        snapshot.values().forEach(cf -> cf.completeExceptionally(error));
    }

    private Map<String, Object> toMap(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return Map.of();
        }
        if (!element.isJsonObject()) {
            return Map.of();
        }
        Map<String, Object> result = gson.fromJson(element, MAP_TYPE);
        return result != null ? result : Map.of();
    }

    private static String asString(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return null;
        }
        return element.getAsString();
    }

    private static String asID(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return null;
        }
        if (element.isJsonPrimitive()) {
            return element.getAsString();
        }
        return null;
    }

    private static Throwable unwrap(Throwable throwable) {
        if (throwable instanceof CompletionException ce && ce.getCause() != null) {
            return ce.getCause();
        }
        return throwable;
    }

    private final class ListenerImpl implements WebSocket.Listener {
        private final StringBuilder textBuffer = new StringBuilder();

        @Override
        public void onOpen(WebSocket webSocket) {
            String protocol = webSocket.getSubprotocol();
            if (!"holon-rpc".equals(protocol)) {
                if (!connectedFuture.isDone()) {
                    connectedFuture.completeExceptionally(
                            new IllegalStateException("server did not negotiate holon-rpc subprotocol"));
                }
                webSocket.sendClose(1002, "missing holon-rpc subprotocol");
                return;
            }

            socket = webSocket;
            reconnectAttempt = 0;
            cancelReconnect();
            startHeartbeat();
            if (!connectedFuture.isDone()) {
                connectedFuture.complete(null);
            }
            webSocket.request(1);
        }

        @Override
        public CompletionStage<?> onText(WebSocket webSocket, CharSequence data, boolean last) {
            textBuffer.append(data);
            if (last) {
                handleIncoming(textBuffer.toString());
                textBuffer.setLength(0);
            }
            webSocket.request(1);
            return CompletableFuture.completedFuture(null);
        }

        @Override
        public CompletionStage<?> onClose(WebSocket webSocket, int statusCode, String reason) {
            handleDisconnect();
            return CompletableFuture.completedFuture(null);
        }

        @Override
        public void onError(WebSocket webSocket, Throwable error) {
            handleDisconnect();
        }
    }
}

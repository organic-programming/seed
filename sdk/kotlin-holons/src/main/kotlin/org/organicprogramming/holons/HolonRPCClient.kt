package org.organicprogramming.holons

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import kotlin.math.min
import kotlin.random.Random

class HolonRPCResponseException(
    val code: Int,
    override val message: String,
    val data: JsonElement? = null,
) : RuntimeException("rpc error $code: $message")

class HolonRPCClient(
    private val heartbeatIntervalMs: Long = 15_000,
    private val heartbeatTimeoutMs: Long = 5_000,
    private val reconnectMinDelayMs: Long = 500,
    private val reconnectMaxDelayMs: Long = 30_000,
    private val reconnectFactor: Double = 2.0,
    private val reconnectJitter: Double = 0.1,
    private val connectTimeoutMs: Long = 10_000,
    private val okHttpClient: OkHttpClient = OkHttpClient.Builder().readTimeout(0, java.util.concurrent.TimeUnit.MILLISECONDS).build(),
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
) {
    private val json = Json { ignoreUnknownKeys = false }

    private val handlers = ConcurrentHashMap<String, suspend (JsonObject) -> JsonObject>()
    private val pending = ConcurrentHashMap<String, CompletableDeferred<JsonObject>>()
    private val nextID = AtomicLong(0)

    private val connectionState = MutableStateFlow(false)
    private val reconnecting = AtomicBoolean(false)
    private val closed = AtomicBoolean(false)

    @Volatile
    private var endpoint: String? = null

    @Volatile
    private var webSocket: WebSocket? = null

    @Volatile
    private var heartbeatJob: Job? = null

    @Volatile
    private var reconnectJob: Job? = null

    suspend fun connect(url: String) {
        endpoint = url
        closed.set(false)
        openWebSocket(url)
        awaitConnected(connectTimeoutMs)
    }

    fun register(method: String, handler: suspend (JsonObject) -> JsonObject) {
        require(method.isNotBlank()) { "method is required" }
        handlers[method] = handler
    }

    suspend fun invoke(
        method: String,
        params: JsonObject = buildJsonObject { },
        timeoutMs: Long = 10_000,
    ): JsonObject {
        require(method.isNotBlank()) { "method is required" }
        awaitConnected(connectTimeoutMs)

        val id = "c${nextID.incrementAndGet()}"
        val deferred = CompletableDeferred<JsonObject>()
        pending[id] = deferred

        val payload = buildJsonObject {
            put("jsonrpc", JsonPrimitive("2.0"))
            put("id", JsonPrimitive(id))
            put("method", JsonPrimitive(method))
            put("params", params)
        }

        val sent = webSocket?.send(payload.toString()) ?: false
        if (!sent) {
            pending.remove(id)
            throw IllegalStateException("websocket send failed")
        }

        return try {
            withTimeout(timeoutMs) {
                deferred.await()
            }
        } finally {
            pending.remove(id)
        }
    }

    suspend fun close() {
        closed.set(true)
        reconnectJob?.cancel()
        reconnectJob = null

        heartbeatJob?.cancel()
        heartbeatJob = null

        webSocket?.close(1000, "client close")
        webSocket = null
        connectionState.value = false

        failAllPending(IllegalStateException("holon-rpc client closed"))
        scope.cancel()

        okHttpClient.dispatcher.cancelAll()
        okHttpClient.dispatcher.executorService.shutdownNow()
        okHttpClient.connectionPool.evictAll()
        okHttpClient.cache?.close()
    }

    private suspend fun awaitConnected(timeoutMs: Long) {
        if (connectionState.value) {
            return
        }

        withTimeout(timeoutMs) {
            connectionState.filter { it }.first()
        }
    }

    private fun openWebSocket(url: String) {
        val request = Request.Builder()
            .url(url)
            .header("Sec-WebSocket-Protocol", "holon-rpc")
            .build()

        okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                val protocol = response.header("Sec-WebSocket-Protocol")
                if (protocol != "holon-rpc") {
                    webSocket.close(1002, "missing holon-rpc subprotocol")
                    return
                }

                this@HolonRPCClient.webSocket = webSocket
                connectionState.value = true
                startHeartbeat()
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                scope.launch {
                    handleIncomingMessage(text)
                }
            }

            override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                webSocket.close(code, reason)
                scope.launch { handleDisconnect() }
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                scope.launch { handleDisconnect() }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                scope.launch { handleDisconnect() }
            }
        })
    }

    private suspend fun handleIncomingMessage(text: String) {
        val payload = json.parseToJsonElement(text).jsonObject

        if (payload["method"] != null) {
            handleIncomingRequest(payload)
        } else if (payload["result"] != null || payload["error"] != null) {
            handleIncomingResponse(payload)
        }
    }

    private suspend fun handleIncomingRequest(payload: JsonObject) {
        val method = payload["method"]?.jsonPrimitive?.content
        val id = payload["id"]
        val jsonrpc = payload["jsonrpc"]?.jsonPrimitive?.content

        if (jsonrpc != "2.0" || method == null) {
            if (id != null) {
                sendError(id, -32600, "invalid request")
            }
            return
        }

        if (method == "rpc.heartbeat") {
            if (id != null) {
                sendResult(id, buildJsonObject { })
            }
            return
        }

        if (id != null) {
            val sid = id.jsonPrimitive.content
            if (!sid.startsWith("s")) {
                sendError(id, -32600, "server request id must start with 's'")
                return
            }
        }

        val handler = handlers[method]
        if (handler == null) {
            if (id != null) {
                sendError(id, -32601, "method \"$method\" not found")
            }
            return
        }

        val params = payload["params"]?.jsonObject ?: buildJsonObject { }

        try {
            val result = handler(params)
            if (id != null) {
                sendResult(id, result)
            }
        } catch (rpc: HolonRPCResponseException) {
            if (id != null) {
                sendError(id, rpc.code, rpc.message, rpc.data)
            }
        } catch (t: Throwable) {
            if (id != null) {
                sendError(id, 13, t.message ?: "internal error")
            }
        }
    }

    private fun handleIncomingResponse(payload: JsonObject) {
        val id = payload["id"]?.jsonPrimitive?.content ?: return
        val deferred = pending.remove(id) ?: return

        val errorObj = payload["error"]?.jsonObject
        if (errorObj != null) {
            val code = errorObj["code"]?.jsonPrimitive?.int ?: -32603
            val message = errorObj["message"]?.jsonPrimitive?.content ?: "internal error"
            deferred.completeExceptionally(HolonRPCResponseException(code, message, errorObj["data"]))
            return
        }

        val result = payload["result"]?.jsonObject ?: buildJsonObject { }
        deferred.complete(result)
    }

    private fun sendResult(id: JsonElement, result: JsonObject) {
        val payload = buildJsonObject {
            put("jsonrpc", JsonPrimitive("2.0"))
            put("id", id)
            put("result", result)
        }
        if (webSocket?.send(payload.toString()) != true) {
            throw IllegalStateException("websocket send failed")
        }
    }

    private fun sendError(id: JsonElement, code: Int, message: String, data: JsonElement? = null) {
        val payload = buildJsonObject {
            put("jsonrpc", JsonPrimitive("2.0"))
            put("id", id)
            put("error", buildJsonObject {
                put("code", JsonPrimitive(code))
                put("message", JsonPrimitive(message))
                if (data != null) {
                    put("data", data)
                }
            })
        }
        if (webSocket?.send(payload.toString()) != true) {
            throw IllegalStateException("websocket send failed")
        }
    }

    private fun startHeartbeat() {
        if (heartbeatJob?.isActive == true) {
            return
        }

        heartbeatJob = scope.launch {
            while (true) {
                delay(heartbeatIntervalMs)
                if (closed.get()) {
                    return@launch
                }

                try {
                    withTimeout(heartbeatTimeoutMs) {
                        invoke("rpc.heartbeat", buildJsonObject { })
                    }
                } catch (_: Throwable) {
                    webSocket?.cancel()
                    return@launch
                }
            }
        }
    }

    private suspend fun handleDisconnect() {
        webSocket = null
        connectionState.value = false
        failAllPending(IllegalStateException("holon-rpc connection closed"))

        if (!closed.get()) {
            startReconnectLoop()
        }
    }

    private fun startReconnectLoop() {
        if (!reconnecting.compareAndSet(false, true)) {
            return
        }

        reconnectJob?.cancel()
        reconnectJob = scope.launch {
            var attempt = 0

            while (!closed.get()) {
                val target = endpoint ?: break
                openWebSocket(target)

                try {
                    awaitConnected(connectTimeoutMs)
                    reconnecting.set(false)
                    return@launch
                } catch (_: Throwable) {
                    val base = min(
                        reconnectMinDelayMs * Math.pow(reconnectFactor, attempt.toDouble()),
                        reconnectMaxDelayMs.toDouble(),
                    )
                    val jitter = base * reconnectJitter * Random.nextDouble(0.0, 1.0)
                    delay((base + jitter).toLong())
                    attempt += 1
                }
            }

            reconnecting.set(false)
        }
    }

    private fun failAllPending(error: Throwable) {
        val values = pending.values.toList()
        pending.clear()
        values.forEach { it.completeExceptionally(error) }
    }
}

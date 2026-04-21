package org.organicprogramming.holons

import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets
import java.time.Duration
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

/** Holon-RPC JSON-RPC 2.0 client over HTTP+SSE. */
class HolonRPCHTTPClient(
    baseURL: String,
    private val httpClient: HttpClient = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build(),
) {
    data class SSEEvent(
        val event: String,
        val id: String,
        val result: JsonObject = buildJsonObject { },
        val error: HolonRPCResponseException? = null,
    )

    private val json = Json { ignoreUnknownKeys = false }
    private val normalizedBaseURL = baseURL.trim().trimEnd('/')

    init {
        require(normalizedBaseURL.isNotBlank()) { "baseURL is required" }
    }

    suspend fun invoke(
        method: String,
        params: JsonObject = buildJsonObject { },
    ): JsonObject = withContext(Dispatchers.IO) {
        val request = HttpRequest.newBuilder(URI.create(methodURL(method)))
            .timeout(Duration.ofSeconds(10))
            .header("Content-Type", "application/json")
            .header("Accept", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(params.toString()))
            .build()

        val response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8))
        decodeRPCResponse(response.statusCode(), response.body())
    }

    suspend fun stream(
        method: String,
        params: JsonObject = buildJsonObject { },
    ): List<SSEEvent> = withContext(Dispatchers.IO) {
        val request = HttpRequest.newBuilder(URI.create(methodURL(method)))
            .timeout(Duration.ofSeconds(10))
            .header("Content-Type", "application/json")
            .header("Accept", "text/event-stream")
            .POST(HttpRequest.BodyPublishers.ofString(params.toString()))
            .build()

        val response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8))
        readSSEEvents(response.statusCode(), response.body())
    }

    suspend fun streamQuery(
        method: String,
        params: Map<String, String>,
    ): List<SSEEvent> = withContext(Dispatchers.IO) {
        val query = params.entries.joinToString("&") { (key, value) ->
            "${URLEncoder.encode(key, StandardCharsets.UTF_8)}=${URLEncoder.encode(value, StandardCharsets.UTF_8)}"
        }
        val url = buildString {
            append(methodURL(method))
            if (query.isNotEmpty()) {
                append('?')
                append(query)
            }
        }

        val request = HttpRequest.newBuilder(URI.create(url))
            .timeout(Duration.ofSeconds(10))
            .header("Accept", "text/event-stream")
            .GET()
            .build()

        val response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8))
        readSSEEvents(response.statusCode(), response.body())
    }

    private fun methodURL(method: String): String {
        val trimmed = method.trim().trim('/')
        require(trimmed.isNotBlank()) { "method is required" }
        return "$normalizedBaseURL/$trimmed"
    }

    private fun decodeRPCResponse(statusCode: Int, body: String): JsonObject {
        val message = parseObject(body)
        if (message != null) {
            message["error"]?.jsonObject?.let { throw rpcException(it) }
            message["result"]?.jsonObject?.let { return it }
            if (statusCode < 400) {
                return message
            }
        }
        check(statusCode < 400) { "http status $statusCode" }
        return buildJsonObject { }
    }

    private fun readSSEEvents(statusCode: Int, body: String): List<SSEEvent> {
        if (statusCode >= 400) {
            decodeRPCResponse(statusCode, body)
        }

        val events = mutableListOf<SSEEvent>()
        var event = ""
        var id = ""
        var data = ""

        body.lineSequence().forEach { line ->
            if (line.isEmpty()) {
                if (event.isNotEmpty() || id.isNotEmpty() || data.isNotEmpty()) {
                    val decoded = decodeSSEEvent(event, id, data)
                    events += decoded
                    if (decoded.event == "done") {
                        return events
                    }
                }
                event = ""
                id = ""
                data = ""
                return@forEach
            }

            when {
                line.startsWith("event:") -> event = line.removePrefix("event:").trim()
                line.startsWith("id:") -> id = line.removePrefix("id:").trim()
                line.startsWith("data:") -> data = line.removePrefix("data:").trim()
            }
        }

        if (event.isNotEmpty() || id.isNotEmpty() || data.isNotEmpty()) {
            events += decodeSSEEvent(event, id, data)
        }
        return events
    }

    private fun decodeSSEEvent(event: String, id: String, data: String): SSEEvent {
        if (event == "done") {
            return SSEEvent(event = "done", id = id)
        }

        val message = parseObject(data) ?: return SSEEvent(event = event, id = id)
        val error = message["error"]?.jsonObject
        if (error != null) {
            return SSEEvent(
                event = event,
                id = id,
                error = rpcException(error),
            )
        }

        return SSEEvent(
            event = event,
            id = id,
            result = message["result"]?.jsonObject ?: buildJsonObject { },
        )
    }

    private fun parseObject(body: String): JsonObject? {
        if (body.isBlank()) {
            return null
        }
        return runCatching { json.parseToJsonElement(body).jsonObject }.getOrNull()
    }

    private fun rpcException(error: JsonObject): HolonRPCResponseException =
        HolonRPCResponseException(
            code = error["code"]?.jsonPrimitive?.int ?: 13,
            message = error["message"]?.jsonPrimitive?.content ?: "internal error",
            data = error["data"],
        )
}

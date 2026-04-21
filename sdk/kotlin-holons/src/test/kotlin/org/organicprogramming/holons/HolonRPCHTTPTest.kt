package org.organicprogramming.holons

import com.sun.net.httpserver.HttpExchange
import com.sun.net.httpserver.HttpServer
import java.net.InetSocketAddress
import java.nio.charset.StandardCharsets
import java.time.Duration
import java.util.concurrent.Executors
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class HolonRPCHTTPTest {
    @Test
    fun invokePostsJsonRpcOverHttp() = runBlocking {
        startServer(
            methodPath = "echo.v1.Echo/Ping",
            handler = { exchange ->
                assertEquals("POST", exchange.requestMethod)
                assertEquals("application/json", exchange.requestHeaders.getFirst("Content-Type"))
                assertEquals("/api/v1/rpc/echo.v1.Echo/Ping", exchange.requestURI.path)
                writeJSON(
                    exchange,
                    200,
                    """
                    {"jsonrpc":"2.0","id":"h1","result":{"message":"hola"}}
                    """.trimIndent(),
                )
            },
        ).use { fixture ->
            val client = HolonRPCHTTPClient(fixture.baseURL)
            val result = client.invoke(
                "echo.v1.Echo/Ping",
                buildJsonObject { put("message", JsonPrimitive("hello")) },
            )
            assertEquals("hola", result["message"]?.jsonPrimitive?.content)
        }
    }

    @Test
    fun streamPostsServerSentEvents() = runBlocking {
        startServer(
            methodPath = "build.v1.Build/Watch",
            handler = { exchange ->
                assertEquals("POST", exchange.requestMethod)
                assertEquals("text/event-stream", exchange.requestHeaders.getFirst("Accept"))
                writeSSE(
                    exchange,
                    """
                    event: message
                    id: 1
                    data: {"jsonrpc":"2.0","id":"h1","result":{"status":"building","progress":42}}

                    event: message
                    id: 2
                    data: {"jsonrpc":"2.0","id":"h1","result":{"status":"done","progress":100}}

                    event: done
                    data:

                    """.trimIndent(),
                )
            },
        ).use { fixture ->
            val client = HolonRPCHTTPClient(fixture.baseURL)
            val events = client.stream(
                "build.v1.Build/Watch",
                buildJsonObject { put("project", JsonPrimitive("myapp")) },
            )

            assertEquals(3, events.size)
            assertEquals("message", events[0].event)
            assertEquals("1", events[0].id)
            assertEquals("building", events[0].result["status"]?.jsonPrimitive?.content)
            assertEquals("done", events[1].result["status"]?.jsonPrimitive?.content)
            assertEquals("done", events[2].event)
        }
    }

    @Test
    fun streamQueryUsesGetAndParsesDoneEvent() = runBlocking {
        startServer(
            methodPath = "build.v1.Build/Watch",
            handler = { exchange ->
                assertEquals("GET", exchange.requestMethod)
                assertEquals("project=myapp", exchange.requestURI.query)
                writeSSE(
                    exchange,
                    """
                    event: message
                    id: 1
                    data: {"jsonrpc":"2.0","id":"h2","result":{"status":"watching"}}

                    event: done
                    data:

                    """.trimIndent(),
                )
            },
        ).use { fixture ->
            val client = HolonRPCHTTPClient(fixture.baseURL)
            val events = client.streamQuery("build.v1.Build/Watch", mapOf("project" to "myapp"))

            assertEquals(2, events.size)
            assertEquals("watching", events[0].result["status"]?.jsonPrimitive?.content)
            assertEquals("done", events[1].event)
        }
    }

    @Test
    fun invokeSurfacesJsonRpcErrors() = runBlocking {
        startServer(
            methodPath = "missing.v1.Service/Method",
            handler = { exchange ->
                writeJSON(
                    exchange,
                    404,
                    """
                    {"jsonrpc":"2.0","id":"h3","error":{"code":5,"message":"method not found"}}
                    """.trimIndent(),
                )
            },
        ).use { fixture ->
            val client = HolonRPCHTTPClient(fixture.baseURL)
            val error = assertFailsWith<HolonRPCResponseException> {
                client.invoke("missing.v1.Service/Method")
            }

            assertEquals(5, error.code)
            assertTrue(error.message.contains("method not found"))
        }
    }

    private fun startServer(
        methodPath: String,
        handler: (HttpExchange) -> Unit,
    ): HTTPFixture {
        val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0)
        server.createContext("/api/v1/rpc/$methodPath") { exchange ->
            try {
                handler(exchange)
            } finally {
                exchange.close()
            }
        }
        server.executor = Executors.newCachedThreadPool()
        server.start()
        return HTTPFixture(server, "http://127.0.0.1:${server.address.port}/api/v1/rpc")
    }

    private fun writeJSON(exchange: HttpExchange, status: Int, body: String) {
        val payload = body.toByteArray(StandardCharsets.UTF_8)
        exchange.responseHeaders.set("Content-Type", "application/json")
        exchange.sendResponseHeaders(status, payload.size.toLong())
        exchange.responseBody.use { output ->
            output.write(payload)
        }
    }

    private fun writeSSE(exchange: HttpExchange, body: String) {
        val payload = body.toByteArray(StandardCharsets.UTF_8)
        exchange.responseHeaders.set("Content-Type", "text/event-stream")
        exchange.sendResponseHeaders(200, payload.size.toLong())
        exchange.responseBody.use { output ->
            output.write(payload)
            output.flush()
        }
    }

    private data class HTTPFixture(
        val server: HttpServer,
        val baseURL: String,
    ) : AutoCloseable {
        override fun close() {
            server.stop(Duration.ofSeconds(1).toSeconds().toInt())
        }
    }
}

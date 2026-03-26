package org.organicprogramming.holons;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executors;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertTrue;

class HolonRPCHttpTest {

    @Test
    void invokePostsJsonRpcOverHttp() throws Exception {
        try (HTTPFixture fixture = startServer(exchange -> {
            assertEquals("POST", exchange.getRequestMethod());
            assertEquals("application/json", exchange.getRequestHeaders().getFirst("Content-Type"));
            assertEquals("/api/v1/rpc/echo.v1.Echo/Ping", exchange.getRequestURI().getPath());
            writeJSON(exchange, 200, """
                    {"jsonrpc":"2.0","id":"h1","result":{"message":"hola"}}
                    """);
        }, "echo.v1.Echo/Ping")) {
            HolonRPCHttpClient client = new HolonRPCHttpClient(fixture.baseURL());
            Map<String, Object> result = client.invoke("echo.v1.Echo/Ping", Map.of("message", "hello"));
            assertEquals("hola", result.get("message"));
        }
    }

    @Test
    void streamPostsServerSentEvents() throws Exception {
        try (HTTPFixture fixture = startServer(exchange -> {
            assertEquals("POST", exchange.getRequestMethod());
            assertEquals("text/event-stream", exchange.getRequestHeaders().getFirst("Accept"));
            writeSSE(exchange, """
                    event: message
                    id: 1
                    data: {"jsonrpc":"2.0","id":"h1","result":{"status":"building","progress":42}}

                    event: message
                    id: 2
                    data: {"jsonrpc":"2.0","id":"h1","result":{"status":"done","progress":100}}

                    event: done
                    data:

                    """);
        }, "build.v1.Build/Watch")) {
            HolonRPCHttpClient client = new HolonRPCHttpClient(fixture.baseURL());
            List<HolonRPCHttpClient.SSEEvent> events = client.stream("build.v1.Build/Watch", Map.of("project", "myapp"));

            assertEquals(3, events.size());
            assertEquals("message", events.get(0).event());
            assertEquals("1", events.get(0).id());
            assertEquals("building", events.get(0).result().get("status"));
            assertEquals("done", events.get(1).result().get("status"));
            assertEquals("done", events.get(2).event());
        }
    }

    @Test
    void streamQueryUsesGetAndParsesDoneEvent() throws Exception {
        try (HTTPFixture fixture = startServer(exchange -> {
            assertEquals("GET", exchange.getRequestMethod());
            assertEquals("project=myapp", exchange.getRequestURI().getQuery());
            writeSSE(exchange, """
                    event: message
                    id: 1
                    data: {"jsonrpc":"2.0","id":"h2","result":{"status":"watching"}}

                    event: done
                    data:

                    """);
        }, "build.v1.Build/Watch")) {
            HolonRPCHttpClient client = new HolonRPCHttpClient(fixture.baseURL());
            List<HolonRPCHttpClient.SSEEvent> events = client.streamQuery("build.v1.Build/Watch", Map.of("project", "myapp"));

            assertEquals(2, events.size());
            assertEquals("watching", events.get(0).result().get("status"));
            assertEquals("done", events.get(1).event());
        }
    }

    @Test
    void invokeSurfacesJsonRpcErrors() throws Exception {
        try (HTTPFixture fixture = startServer(exchange -> writeJSON(exchange, 404, """
                {"jsonrpc":"2.0","id":"h3","error":{"code":5,"message":"method not found"}}
                """), "missing.v1.Service/Method")) {
            HolonRPCHttpClient client = new HolonRPCHttpClient(fixture.baseURL());
            HolonRPCClient.HolonRPCResponseException error = assertInstanceOf(
                    HolonRPCClient.HolonRPCResponseException.class,
                    org.junit.jupiter.api.Assertions.assertThrows(RuntimeException.class, () ->
                            client.invoke("missing.v1.Service/Method", Map.of())));
            assertEquals(5, error.code());
            assertTrue(error.getMessage().contains("method not found"));
        }
    }

    private static HTTPFixture startServer(ExchangeHandler handler, String methodPath) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/api/v1/rpc/" + methodPath, exchange -> {
            try {
                handler.handle(exchange);
            } finally {
                exchange.close();
            }
        });
        server.setExecutor(Executors.newCachedThreadPool());
        server.start();
        return new HTTPFixture(server, "http://127.0.0.1:" + server.getAddress().getPort() + "/api/v1/rpc");
    }

    private static void writeJSON(HttpExchange exchange, int status, String body) throws IOException {
        byte[] payload = body.strip().getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, payload.length);
        try (OutputStream output = exchange.getResponseBody()) {
            output.write(payload);
        }
    }

    private static void writeSSE(HttpExchange exchange, String body) throws IOException {
        byte[] payload = body.stripIndent().getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
        exchange.sendResponseHeaders(200, payload.length);
        try (OutputStream output = exchange.getResponseBody()) {
            output.write(payload);
            output.flush();
        }
    }

    private interface ExchangeHandler {
        void handle(HttpExchange exchange) throws IOException;
    }

    private record HTTPFixture(HttpServer server, String baseURL) implements AutoCloseable {
        @Override
        public void close() {
            server.stop((int) Duration.ofSeconds(1).toSeconds());
        }
    }
}

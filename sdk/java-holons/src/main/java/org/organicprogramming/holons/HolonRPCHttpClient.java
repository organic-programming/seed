package org.organicprogramming.holons;

import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import com.google.gson.reflect.TypeToken;

import java.io.BufferedReader;
import java.io.StringReader;
import java.lang.reflect.Type;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/** Holon-RPC JSON-RPC 2.0 client over HTTP+SSE. */
public final class HolonRPCHttpClient {
    private static final Type MAP_TYPE = new TypeToken<Map<String, Object>>() {
    }.getType();

    public record RPCError(int code, String message, Object data) {
    }

    public record SSEEvent(String event, String id, Map<String, Object> result, RPCError error) {
    }

    private final String baseURL;
    private final HttpClient httpClient;
    private final Gson gson;

    public HolonRPCHttpClient(String baseURL) {
        this(baseURL, HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build());
    }

    public HolonRPCHttpClient(String baseURL, HttpClient httpClient) {
        String trimmed = Objects.requireNonNull(baseURL, "baseURL").trim();
        if (trimmed.isBlank()) {
            throw new IllegalArgumentException("baseURL is required");
        }
        this.baseURL = trimmed.replaceAll("/+$", "");
        this.httpClient = Objects.requireNonNull(httpClient, "httpClient");
        this.gson = new Gson();
    }

    public Map<String, Object> invoke(String method, Map<String, Object> params) throws Exception {
        HttpRequest request = HttpRequest.newBuilder(URI.create(methodURL(method)))
                .timeout(Duration.ofSeconds(10))
                .header("Content-Type", "application/json")
                .header("Accept", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(gson.toJson(params != null ? params : Map.of())))
                .build();

        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        return decodeRPCResponse(response.statusCode(), response.body());
    }

    public List<SSEEvent> stream(String method, Map<String, Object> params) throws Exception {
        HttpRequest request = HttpRequest.newBuilder(URI.create(methodURL(method)))
                .timeout(Duration.ofSeconds(10))
                .header("Content-Type", "application/json")
                .header("Accept", "text/event-stream")
                .POST(HttpRequest.BodyPublishers.ofString(gson.toJson(params != null ? params : Map.of())))
                .build();

        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        return readSSEEvents(response.statusCode(), response.body());
    }

    public List<SSEEvent> streamQuery(String method, Map<String, String> params) throws Exception {
        StringBuilder url = new StringBuilder(methodURL(method));
        Map<String, String> query = params != null ? params : Map.of();
        if (!query.isEmpty()) {
            url.append('?');
            boolean first = true;
            for (Map.Entry<String, String> entry : query.entrySet()) {
                if (!first) {
                    url.append('&');
                }
                first = false;
                url.append(URLEncoder.encode(entry.getKey(), StandardCharsets.UTF_8));
                url.append('=');
                url.append(URLEncoder.encode(entry.getValue(), StandardCharsets.UTF_8));
            }
        }

        HttpRequest request = HttpRequest.newBuilder(URI.create(url.toString()))
                .timeout(Duration.ofSeconds(10))
                .header("Accept", "text/event-stream")
                .GET()
                .build();

        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        return readSSEEvents(response.statusCode(), response.body());
    }

    private String methodURL(String method) {
        String trimmed = Objects.requireNonNull(method, "method").trim();
        if (trimmed.isBlank()) {
            throw new IllegalArgumentException("method is required");
        }
        return baseURL + "/" + trimmed.replaceAll("^/+|/+$", "");
    }

    private Map<String, Object> decodeRPCResponse(int statusCode, String body) {
        JsonObject message = parseObject(body);
        if (message != null && message.has("error") && message.get("error").isJsonObject()) {
            throw rpcException(message.getAsJsonObject("error"));
        }
        if (message != null && message.has("result")) {
            return toMap(message.get("result"));
        }
        if (statusCode >= 400) {
            throw new IllegalStateException("http status " + statusCode);
        }
        return message != null ? toMap(message) : Map.of();
    }

    private List<SSEEvent> readSSEEvents(int statusCode, String body) {
        if (statusCode >= 400) {
            decodeRPCResponse(statusCode, body);
        }

        List<SSEEvent> events = new ArrayList<>();
        try (BufferedReader reader = new BufferedReader(new StringReader(body))) {
            String event = "";
            String id = "";
            String data = "";

            String line;
            while ((line = reader.readLine()) != null) {
                if (line.isEmpty()) {
                    if (!event.isEmpty() || !id.isEmpty() || !data.isEmpty()) {
                        SSEEvent decoded = decodeSSEEvent(event, id, data);
                        events.add(decoded);
                        if ("done".equals(decoded.event())) {
                            return events;
                        }
                    }
                    event = "";
                    id = "";
                    data = "";
                    continue;
                }

                if (line.startsWith("event:")) {
                    event = line.substring("event:".length()).trim();
                } else if (line.startsWith("id:")) {
                    id = line.substring("id:".length()).trim();
                } else if (line.startsWith("data:")) {
                    data = line.substring("data:".length()).trim();
                }
            }

            if (!event.isEmpty() || !id.isEmpty() || !data.isEmpty()) {
                events.add(decodeSSEEvent(event, id, data));
            }
            return events;
        } catch (Exception e) {
            throw new IllegalStateException("read SSE stream failed", e);
        }
    }

    private SSEEvent decodeSSEEvent(String event, String id, String data) {
        if ("done".equals(event)) {
            return new SSEEvent("done", id, Map.of(), null);
        }

        JsonObject message = parseObject(data);
        if (message == null) {
            return new SSEEvent(event, id, Map.of(), null);
        }
        if (message.has("error") && message.get("error").isJsonObject()) {
            JsonObject error = message.getAsJsonObject("error");
            return new SSEEvent(event, id, Map.of(), new RPCError(
                    error.has("code") ? error.get("code").getAsInt() : 13,
                    error.has("message") ? error.get("message").getAsString() : "internal error",
                    fromJson(error.get("data"))));
        }
        return new SSEEvent(event, id, toMap(message.get("result")), null);
    }

    private Map<String, Object> toMap(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return Map.of();
        }
        if (!element.isJsonObject()) {
            return Map.of();
        }
        return gson.fromJson(element, MAP_TYPE);
    }

    private Object fromJson(JsonElement element) {
        if (element == null || element.isJsonNull()) {
            return null;
        }
        return gson.fromJson(element, Object.class);
    }

    private JsonObject parseObject(String body) {
        if (body == null || body.isBlank()) {
            return null;
        }
        try {
            JsonElement parsed = JsonParser.parseString(body);
            return parsed.isJsonObject() ? parsed.getAsJsonObject() : null;
        } catch (Exception ignored) {
            return null;
        }
    }

    private HolonRPCClient.HolonRPCResponseException rpcException(JsonObject error) {
        int code = error.has("code") ? error.get("code").getAsInt() : 13;
        String message = error.has("message") ? error.get("message").getAsString() : "internal error";
        return new HolonRPCClient.HolonRPCResponseException(code, message, fromJson(error.get("data")));
    }
}

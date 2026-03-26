package org.organicprogramming.holons

import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.tls.HandshakeCertificates
import okhttp3.tls.HeldCertificate
import java.io.File
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class HolonRPCTest {
    @Test
    fun holonRPCGoEchoRoundTrip() = runBlocking {
        withGoHolonRPCServer("echo") { url ->
            val client = HolonRPCClient(
                heartbeatIntervalMs = 250,
                heartbeatTimeoutMs = 250,
                reconnectMinDelayMs = 100,
                reconnectMaxDelayMs = 400,
            )

            client.connect(url)
            val out = client.invoke("echo.v1.Echo/Ping", buildJsonObject { put("message", JsonPrimitive("hello")) })
            assertEquals("hello", out["message"]?.jsonPrimitive?.content)
            client.close()
        }
    }

    @Test
    fun holonRPCRegisterHandlesServerCalls() = runBlocking {
        withGoHolonRPCServer("echo") { url ->
            val client = HolonRPCClient(
                heartbeatIntervalMs = 250,
                heartbeatTimeoutMs = 250,
                reconnectMinDelayMs = 100,
                reconnectMaxDelayMs = 400,
            )

            client.register("client.v1.Client/Hello") { params ->
                buildJsonObject {
                    put("message", JsonPrimitive("hello ${params["name"]?.jsonPrimitive?.content ?: ""}"))
                }
            }

            client.connect(url)
            val out = client.invoke("echo.v1.Echo/CallClient", buildJsonObject { })
            assertEquals("hello go", out["message"]?.jsonPrimitive?.content)
            client.close()
        }
    }

    @Test
    fun holonRPCReconnectAndHeartbeat() = runBlocking {
        withGoHolonRPCServer("drop-once") { url ->
            val client = HolonRPCClient(
                heartbeatIntervalMs = 200,
                heartbeatTimeoutMs = 200,
                reconnectMinDelayMs = 100,
                reconnectMaxDelayMs = 400,
            )

            client.connect(url)
            val first = client.invoke("echo.v1.Echo/Ping", buildJsonObject { put("message", JsonPrimitive("first")) })
            assertEquals("first", first["message"]?.jsonPrimitive?.content)

            delay(700)

            val second = invokeEventually(client, "echo.v1.Echo/Ping", buildJsonObject { put("message", JsonPrimitive("second")) })
            assertEquals("second", second["message"]?.jsonPrimitive?.content)

            val hb = invokeEventually(client, "echo.v1.Echo/HeartbeatCount", buildJsonObject { })
            val count = hb["count"]?.jsonPrimitive?.int ?: 0
            assertTrue(count >= 1)

            client.close()
        }
    }

    @Test
    fun holonRPCWssEchoRoundTrip() = runBlocking {
        val serverCert = HeldCertificate.Builder()
            .commonName("localhost")
            .addSubjectAlternativeName("localhost")
            .build()
        val serverCertificates = HandshakeCertificates.Builder()
            .heldCertificate(serverCert)
            .build()
        val clientCertificates = HandshakeCertificates.Builder()
            .addTrustedCertificate(serverCert.certificate)
            .build()

        MockWebServer().use { server ->
            server.useHttps(serverCertificates.sslSocketFactory(), false)
            server.enqueue(
                MockResponse()
                    .setHeader("Sec-WebSocket-Protocol", "holon-rpc")
                    .withWebSocketUpgrade(
                        object : WebSocketListener() {
                            private val json = Json

                            override fun onMessage(webSocket: WebSocket, text: String) {
                                val payload = json.parseToJsonElement(text).jsonObject
                                val method = payload["method"]?.jsonPrimitive?.content.orEmpty()
                                val id = payload["id"]?.jsonPrimitive?.content.orEmpty()
                                val result =
                                    if (method == "rpc.heartbeat") {
                                        buildJsonObject { }
                                    } else {
                                        payload["params"]?.jsonObject ?: buildJsonObject { }
                                    }

                                webSocket.send(
                                    buildJsonObject {
                                        put("jsonrpc", JsonPrimitive("2.0"))
                                        put("id", JsonPrimitive(id))
                                        put("result", result)
                                    }.toString(),
                                )
                            }
                        },
                    ),
            )
            server.start()

            val client = HolonRPCClient(
                heartbeatIntervalMs = 60_000,
                heartbeatTimeoutMs = 5_000,
                reconnectMinDelayMs = 100,
                reconnectMaxDelayMs = 400,
                okHttpClient = OkHttpClient.Builder()
                    .sslSocketFactory(clientCertificates.sslSocketFactory(), clientCertificates.trustManager)
                    .hostnameVerifier { _, _ -> true }
                    .readTimeout(0, TimeUnit.MILLISECONDS)
                    .build(),
            )

            val url = server.url("/rpc").toString().replaceFirst("https://", "wss://")
            client.connect(url)
            val out = client.invoke("echo.v1.Echo/Ping", buildJsonObject { put("message", JsonPrimitive("hello")) })
            assertEquals("hello", out["message"]?.jsonPrimitive?.content)
            client.close()
        }
    }

    private suspend fun invokeEventually(client: HolonRPCClient, method: String, params: JsonObject): JsonObject {
        var lastError: Throwable? = null

        repeat(40) {
            try {
                return client.invoke(method, params)
            } catch (t: Throwable) {
                lastError = t
                delay(120)
            }
        }

        throw lastError ?: IllegalStateException("invoke eventually failed")
    }

    private suspend fun withGoHolonRPCServer(mode: String, block: suspend (String) -> Unit) {
        val sdkDir = File(System.getProperty("user.dir")).parentFile ?: error("sdk dir not found")
        val goHolonsDir = File(sdkDir, "go-holons")

        val helperFile = File(goHolonsDir, "tmp-holonrpc-${UUID.randomUUID()}.go")
        helperFile.writeText(goHolonRPCServerSource)

        val process = ProcessBuilder(resolveGoBinary(), "run", helperFile.absolutePath, mode)
            .directory(goHolonsDir)
            .redirectErrorStream(false)
            .start()

        val stdout = process.inputStream.bufferedReader(StandardCharsets.UTF_8)
        val stderr = process.errorStream.bufferedReader(StandardCharsets.UTF_8)

        try {
            val url = stdout.readLine()
                ?: error("Go holon-rpc helper did not output URL: ${stderr.readText()}")

            block(url)
        } finally {
            process.destroy()
            if (!process.waitFor(5, TimeUnit.SECONDS)) {
                process.destroyForcibly()
                process.waitFor(5, TimeUnit.SECONDS)
            }
            helperFile.delete()
        }
    }

    private fun resolveGoBinary(): String {
        val preferred = File("/Users/bpds/go/go1.25.1/bin/go")
        if (preferred.canExecute()) {
            return preferred.absolutePath
        }
        return "go"
    }
}

private val goHolonRPCServerSource = """
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"nhooyr.io/websocket"
)

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	mode := "echo"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	var heartbeatCount int64
	var dropped int32

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		defer c.CloseNow()

		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}

			var msg rpcMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				_ = writeError(ctx, c, nil, -32700, "parse error")
				continue
			}
			if msg.JSONRPC != "2.0" {
				_ = writeError(ctx, c, msg.ID, -32600, "invalid request")
				continue
			}
			if msg.Method == "" {
				continue
			}

			switch msg.Method {
			case "rpc.heartbeat":
				atomic.AddInt64(&heartbeatCount, 1)
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{})
			case "echo.v1.Echo/Ping":
				var params map[string]interface{}
				_ = json.Unmarshal(msg.Params, &params)
				if params == nil {
					params = map[string]interface{}{}
				}
				_ = writeResult(ctx, c, msg.ID, params)
				if mode == "drop-once" && atomic.CompareAndSwapInt32(&dropped, 0, 1) {
					time.Sleep(100 * time.Millisecond)
					_ = c.Close(websocket.StatusNormalClosure, "drop once")
					return
				}
			case "echo.v1.Echo/HeartbeatCount":
				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{"count": atomic.LoadInt64(&heartbeatCount)})
			case "echo.v1.Echo/CallClient":
				callID := "s1"
				if err := writeRequest(ctx, c, callID, "client.v1.Client/Hello", map[string]interface{}{"name": "go"}); err != nil {
					_ = writeError(ctx, c, msg.ID, 13, err.Error())
					continue
				}

				innerResult, callErr := waitForResponse(ctx, c, callID)
				if callErr != nil {
					_ = writeError(ctx, c, msg.ID, 13, callErr.Error())
					continue
				}
				_ = writeResult(ctx, c, msg.ID, innerResult)
			default:
				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
			}
		}
	})

	srv := &http.Server{Handler: h}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	fmt.Printf("ws://%s/rpc\n", ln.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func writeRequest(ctx context.Context, c *websocket.Conn, id interface{}, method string, params map[string]interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  mustRaw(params),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeResult(ctx context.Context, c *websocket.Conn, id interface{}, result interface{}) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  mustRaw(result),
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func writeError(ctx context.Context, c *websocket.Conn, id interface{}, code int, message string) error {
	payload, err := json.Marshal(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, payload)
}

func waitForResponse(ctx context.Context, c *websocket.Conn, expectedID string) (map[string]interface{}, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		_, data, err := c.Read(deadlineCtx)
		if err != nil {
			return nil, err
		}

		var msg rpcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		id, _ := msg.ID.(string)
		if id != expectedID {
			continue
		}
		if msg.Error != nil {
			return nil, fmt.Errorf("client error: %d %s", msg.Error.Code, msg.Error.Message)
		}
		var out map[string]interface{}
		if err := json.Unmarshal(msg.Result, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func mustRaw(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}
""".trimIndent()

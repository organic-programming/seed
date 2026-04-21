package org.organicprogramming.holons;

import org.junit.jupiter.api.Test;

import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.cert.CertificateFactory;
import java.security.cert.X509Certificate;
import java.time.Duration;
import java.util.Base64;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class HolonRPCTest {

    @Test
    void holonRPCEchoRoundTrip() throws Exception {
        withGoHolonRPCServer("echo", url -> {
            HolonRPCClient client = new HolonRPCClient(
                    250, 250, 100, 400,
                    2.0, 0.1, 10_000, 10_000);

            client.connect(url);
            Map<String, Object> out = client.invoke(
                    "echo.v1.Echo/Ping",
                    Map.of("message", "hello"));
            assertEquals("hello", out.get("message"));
            client.close();
        });
    }

    @Test
    void holonRPCRegisterHandlesServerCalls() throws Exception {
        withGoHolonRPCServer("echo", url -> {
            HolonRPCClient client = new HolonRPCClient(
                    250, 250, 100, 400,
                    2.0, 0.1, 10_000, 10_000);

            client.register("client.v1.Client/Hello", params -> {
                String name = params.getOrDefault("name", "").toString();
                return CompletableFuture.completedFuture(Map.of("message", "hello " + name));
            });

            client.connect(url);
            Map<String, Object> out = client.invoke("echo.v1.Echo/CallClient", Map.of());
            assertEquals("hello go", out.get("message"));
            client.close();
        });
    }

    @Test
    void holonRPCReconnectAndHeartbeat() throws Exception {
        withGoHolonRPCServer("drop-once", url -> {
            HolonRPCClient client = new HolonRPCClient(
                    200, 200, 100, 400,
                    2.0, 0.1, 10_000, 10_000);

            client.connect(url);
            Map<String, Object> first = client.invoke(
                    "echo.v1.Echo/Ping",
                    Map.of("message", "first"));
            assertEquals("first", first.get("message"));

            Thread.sleep(700);

            Map<String, Object> second = invokeEventually(
                    client,
                    "echo.v1.Echo/Ping",
                    Map.of("message", "second"));
            assertEquals("second", second.get("message"));

            Map<String, Object> hb = invokeEventually(
                    client,
                    "echo.v1.Echo/HeartbeatCount",
                    Map.of());
            Number count = (Number) hb.getOrDefault("count", 0);
            assertTrue(count.intValue() >= 1);

            client.close();
        });
    }

    @Test
    void holonRPCEchoRoundTripOverSecureWebSocket() throws Exception {
        withGoHolonRPCTLSServer((url, certBase64) -> {
            SSLContext previous = SSLContext.getDefault();
            SSLContext.setDefault(trustingContext(certBase64));

            HolonRPCClient client = new HolonRPCClient(
                    250, 250, 100, 400,
                    2.0, 0.1, 10_000, 10_000);
            try {
                client.connect(url);
                Map<String, Object> out = client.invoke(
                        "echo.v1.Echo/Ping",
                        Map.of("message", "hello"));
                assertEquals("hello", out.get("message"));
            } finally {
                client.close();
                SSLContext.setDefault(previous);
            }
        });
    }

    private static Map<String, Object> invokeEventually(
            HolonRPCClient client,
            String method,
            Map<String, Object> params) throws Exception {
        Exception last = null;

        for (int i = 0; i < 40; i++) {
            try {
                return client.invoke(method, params);
            } catch (Exception error) {
                last = error;
                Thread.sleep(120);
            }
        }

        throw last != null ? last : new IllegalStateException("invoke eventually failed");
    }

    private static void withGoHolonRPCServer(
            String mode,
            ThrowingConsumer<String> body) throws Exception {
        withGoHolonRPCServerSource(GO_HOLON_RPC_SERVER_SOURCE, mode, (url, ignored) -> body.accept(url));
    }

    private static void withGoHolonRPCTLSServer(
            ThrowingBiConsumer<String, String> body) throws Exception {
        withGoHolonRPCServerSource(GO_HOLON_RPC_TLS_SERVER_SOURCE, "tls-echo", body);
    }

    private static void withGoHolonRPCServerSource(
            String source,
            String mode,
            ThrowingBiConsumer<String, String> body) throws Exception {
        Path sdkDir = Path.of(System.getProperty("user.dir")).getParent();
        if (sdkDir == null) {
            throw new IllegalStateException("sdk dir not found");
        }
        Path goHolonsDir = sdkDir.resolve("go-holons");
        Path helperFile = goHolonsDir.resolve("tmp-holonrpc-" + UUID.randomUUID() + ".go");
        Files.writeString(helperFile, source, StandardCharsets.UTF_8);

        Process process = new ProcessBuilder(resolveGoBinary(), "run", helperFile.toString(), mode)
                .directory(goHolonsDir.toFile())
                .redirectErrorStream(false)
                .start();

        try (BufferedReader stdout = new BufferedReader(
                new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
            String url = readLineWithTimeout(stdout, Duration.ofSeconds(20));
            if (url == null) {
                String stderr = new String(process.getErrorStream().readAllBytes(), StandardCharsets.UTF_8);
                throw new IllegalStateException("Go holon-rpc helper did not output URL: " + stderr);
            }

            String certBase64 = "";
            if ("tls-echo".equals(mode)) {
                certBase64 = readLineWithTimeout(stdout, Duration.ofSeconds(20));
                if (certBase64 == null || certBase64.isBlank()) {
                    String stderr = new String(process.getErrorStream().readAllBytes(), StandardCharsets.UTF_8);
                    throw new IllegalStateException("Go holon-rpc TLS helper did not output cert: " + stderr);
                }
            }

            body.accept(url, certBase64);
        } finally {
            process.destroy();
            if (!process.waitFor(5, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                process.waitFor(5, TimeUnit.SECONDS);
            }
            Files.deleteIfExists(helperFile);
        }
    }

    private static String readLineWithTimeout(BufferedReader reader, Duration timeout)
            throws InterruptedException, ExecutionException, TimeoutException {
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try {
            Future<String> lineFuture = executor.submit(reader::readLine);
            return lineFuture.get(timeout.toMillis(), TimeUnit.MILLISECONDS);
        } finally {
            executor.shutdownNow();
        }
    }

    private static String resolveGoBinary() {
        Path preferred = Path.of("/Users/bpds/go/go1.25.1/bin/go");
        if (Files.isExecutable(preferred)) {
            return preferred.toString();
        }
        return "go";
    }

    private static SSLContext trustingContext(String certBase64) throws Exception {
        byte[] der = Base64.getDecoder().decode(certBase64);
        X509Certificate certificate = (X509Certificate) CertificateFactory.getInstance("X.509")
                .generateCertificate(new ByteArrayInputStream(der));

        KeyStore trustStore = KeyStore.getInstance(KeyStore.getDefaultType());
        trustStore.load(null, null);
        trustStore.setCertificateEntry("holon-rpc-test", certificate);

        TrustManagerFactory trustManagerFactory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        trustManagerFactory.init(trustStore);

        SSLContext context = SSLContext.getInstance("TLS");
        context.init(null, trustManagerFactory.getTrustManagers(), null);
        return context;
    }

    @FunctionalInterface
    private interface ThrowingConsumer<T> {
        void accept(T value) throws Exception;
    }

    @FunctionalInterface
    private interface ThrowingBiConsumer<T, U> {
        void accept(T first, U second) throws Exception;
    }

    private static final String GO_HOLON_RPC_SERVER_SOURCE = """
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

            	fmt.Printf("ws://%s/rpc\\n", ln.Addr().String())

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
            """;

    private static final String GO_HOLON_RPC_TLS_SERVER_SOURCE = """
            package main

            import (
            	"context"
            	"encoding/base64"
            	"encoding/json"
            	"fmt"
            	"net/http"
            	"net/http/httptest"
            	"os"
            	"os/signal"
            	"sync/atomic"
            	"syscall"

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
            	var heartbeatCount int64

            	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
            			Subprotocols: []string{"holon-rpc"},
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
            			case "echo.v1.Echo/HeartbeatCount":
            				_ = writeResult(ctx, c, msg.ID, map[string]interface{}{"count": atomic.LoadInt64(&heartbeatCount)})
            			default:
            				_ = writeError(ctx, c, msg.ID, -32601, fmt.Sprintf("method %q not found", msg.Method))
            			}
            		}
            	})

            	srv := httptest.NewUnstartedServer(h)
            	srv.StartTLS()
            	defer srv.Close()

            	certDER := srv.TLS.Certificates[0].Certificate[0]
            	fmt.Printf("wss://%s/rpc\\n", srv.Listener.Addr().String())
            	fmt.Printf("%s\\n", base64.StdEncoding.EncodeToString(certDER))

            	sigCh := make(chan os.Signal, 1)
            	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
            	<-sigCh
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

            func mustRaw(v interface{}) json.RawMessage {
            	b, _ := json.Marshal(v)
            	return json.RawMessage(b)
            }
            """;
}

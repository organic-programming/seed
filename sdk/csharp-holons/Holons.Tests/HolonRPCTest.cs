using System.Diagnostics;
using System.Text;
using System.Text.Json.Nodes;
using Holons;

namespace Holons.Tests;

public class HolonRPCTest
{
    [Fact]
    public async Task HolonRpcEchoRoundTrip()
    {
        await WithGoHolonRpcServer("echo", async url =>
        {
            await using var client = new HolonRPCClient(
                heartbeatIntervalMs: 250,
                heartbeatTimeoutMs: 250,
                reconnectMinDelayMs: 100,
                reconnectMaxDelayMs: 400);

            await client.ConnectAsync(url);
            var result = await client.InvokeAsync(
                "echo.v1.Echo/Ping",
                new JsonObject { ["message"] = "hello" });

            Assert.Equal("hello", result["message"]?.GetValue<string>());
            await client.CloseAsync();
        });
    }

    [Fact]
    public async Task HolonRpcRegisterHandlesServerCalls()
    {
        await WithGoHolonRpcServer("echo", async url =>
        {
            await using var client = new HolonRPCClient(
                heartbeatIntervalMs: 250,
                heartbeatTimeoutMs: 250,
                reconnectMinDelayMs: 100,
                reconnectMaxDelayMs: 400);

            client.Register("client.v1.Client/Hello", @params =>
            {
                var name = @params["name"]?.GetValue<string>() ?? string.Empty;
                return Task.FromResult<JsonObject>(new JsonObject { ["message"] = $"hello {name}" });
            });

            await client.ConnectAsync(url);
            var result = await client.InvokeAsync("echo.v1.Echo/CallClient");

            Assert.Equal("hello go", result["message"]?.GetValue<string>());
            await client.CloseAsync();
        });
    }

    [Fact]
    public async Task HolonRpcReconnectAndHeartbeat()
    {
        await WithGoHolonRpcServer("drop-once", async url =>
        {
            await using var client = new HolonRPCClient(
                heartbeatIntervalMs: 200,
                heartbeatTimeoutMs: 200,
                reconnectMinDelayMs: 100,
                reconnectMaxDelayMs: 400);

            await client.ConnectAsync(url);

            var first = await client.InvokeAsync(
                "echo.v1.Echo/Ping",
                new JsonObject { ["message"] = "first" });
            Assert.Equal("first", first["message"]?.GetValue<string>());

            await Task.Delay(700);

            var second = await InvokeEventually(
                client,
                "echo.v1.Echo/Ping",
                new JsonObject { ["message"] = "second" });
            Assert.Equal("second", second["message"]?.GetValue<string>());

            var heartbeat = await InvokeEventually(
                client,
                "echo.v1.Echo/HeartbeatCount",
                new JsonObject());
            var count = heartbeat["count"]?.GetValue<int>() ?? 0;
            Assert.True(count >= 1);

            await client.CloseAsync();
        });
    }

    [Fact]
    public async Task HolonRpcCloseAsyncIsSafeWhenCalledConcurrently()
    {
        await WithGoHolonRpcServer("echo", async url =>
        {
            await using var client = new HolonRPCClient(
                heartbeatIntervalMs: 200,
                heartbeatTimeoutMs: 200,
                reconnectMinDelayMs: 100,
                reconnectMaxDelayMs: 400);

            await client.ConnectAsync(url);

            var result = await client.InvokeAsync(
                "echo.v1.Echo/Ping",
                new JsonObject { ["message"] = "close-race" });
            Assert.Equal("close-race", result["message"]?.GetValue<string>());

            using var release = new ManualResetEventSlim(false);
            var closeTasks = Enumerable.Range(0, 16)
                .Select(_ => Task.Run(async () =>
                {
                    release.Wait();
                    await client.CloseAsync();
                }))
                .ToArray();

            release.Set();
            await Task.WhenAll(closeTasks);
        });
    }

    private static async Task<JsonObject> InvokeEventually(
        HolonRPCClient client,
        string method,
        JsonObject @params)
    {
        Exception? last = null;

        for (var i = 0; i < 40; i++)
        {
            try
            {
                return await client.InvokeAsync(method, @params);
            }
            catch (Exception ex)
            {
                last = ex;
                await Task.Delay(120);
            }
        }

        throw last ?? new InvalidOperationException("invoke eventually failed");
    }

    private static async Task WithGoHolonRpcServer(string mode, Func<string, Task> body)
    {
        var sdkDir = FindSdkDirectory();
        var goHolonsDir = Path.Combine(sdkDir, "go-holons");
        var helperPath = Path.Combine(goHolonsDir, $"tmp-holonrpc-{Guid.NewGuid()}.go");
        await File.WriteAllTextAsync(helperPath, GoHolonRpcServerSource, Encoding.UTF8);

        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = ResolveGoBinary(),
                WorkingDirectory = goHolonsDir,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
            }
        };
        process.StartInfo.ArgumentList.Add("run");
        process.StartInfo.ArgumentList.Add(helperPath);
        process.StartInfo.ArgumentList.Add(mode);
        process.Start();

        try
        {
            var lineTask = process.StandardOutput.ReadLineAsync();
            var timeoutTask = Task.Delay(TimeSpan.FromSeconds(20));
            var completed = await Task.WhenAny(lineTask, timeoutTask);
            if (completed == timeoutTask)
            {
                var stderr = await process.StandardError.ReadToEndAsync();
                throw new TimeoutException($"Go holon-rpc helper timed out: {stderr}");
            }

            var url = await lineTask;
            if (string.IsNullOrWhiteSpace(url))
            {
                var stderr = await process.StandardError.ReadToEndAsync();
                throw new InvalidOperationException($"Go holon-rpc helper did not output URL: {stderr}");
            }

            await body(url);
        }
        finally
        {
            try
            {
                if (!process.HasExited)
                {
                    process.Kill(entireProcessTree: true);
                }
            }
            catch
            {
                // ignored
            }

            await process.WaitForExitAsync();

            if (File.Exists(helperPath))
                File.Delete(helperPath);
        }
    }

    private static string FindSdkDirectory()
    {
        var dir = new DirectoryInfo(Directory.GetCurrentDirectory());

        while (dir is not null)
        {
            var candidate = Path.Combine(dir.FullName, "go-holons");
            if (Directory.Exists(candidate))
                return dir.FullName;
            dir = dir.Parent;
        }

        throw new DirectoryNotFoundException("Unable to locate sdk directory containing go-holons");
    }

    private static string ResolveGoBinary()
    {
        var preferred = "/Users/bpds/go/go1.25.1/bin/go";
        return File.Exists(preferred) ? preferred : "go";
    }

    private const string GoHolonRpcServerSource = """
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
        """;
}

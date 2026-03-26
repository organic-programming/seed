package transport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/organic-programming/go-holons/pkg/transport"
)

// helper: dial a test server and return the WebSocket conn
func dialTestBridge(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	ctx := context.Background()
	wsURL := "ws" + srv.URL[4:]
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"holon-rpc"},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

// helper: send a request and read the response
func roundTrip(t *testing.T, c *websocket.Conn, req string) map[string]interface{} {
	t.Helper()
	ctx := context.Background()
	if err := c.Write(ctx, websocket.MessageText, []byte(req)); err != nil {
		t.Fatal(err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp["jsonrpc"]; got != "2.0" {
		t.Fatalf("jsonrpc = %v, want 2.0", got)
	}
	return resp
}

// --- Browser → Go ---

func TestWebBridgeRoundTrip(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("hello.v1.HelloService/Greet", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		var req struct {
			Name string `json:"name"`
		}
		json.Unmarshal(payload, &req)
		name := req.Name
		if name == "" {
			name = "World"
		}
		return json.Marshal(map[string]string{"message": fmt.Sprintf("Hello, %s!", name)})
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"1","method":"hello.v1.HelloService/Greet","params":{"name":"Alice"}}`)
	if resp["id"] != "1" {
		t.Errorf("id = %v", resp["id"])
	}
	result := resp["result"].(map[string]interface{})
	if result["message"] != "Hello, Alice!" {
		t.Errorf("message = %v", result["message"])
	}
}

func TestWebBridgeDefaultName(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("hello.v1.HelloService/Greet", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		var req struct{ Name string }
		json.Unmarshal(payload, &req)
		name := req.Name
		if name == "" {
			name = "World"
		}
		return json.Marshal(map[string]string{"message": fmt.Sprintf("Hello, %s!", name)})
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"2","method":"hello.v1.HelloService/Greet","params":{}}`)
	result := resp["result"].(map[string]interface{})
	if result["message"] != "Hello, World!" {
		t.Errorf("message = %v", result["message"])
	}
}

func TestWebBridgeMethodNotFound(t *testing.T) {
	bridge := transport.NewWebBridge()
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"3","method":"no.Such/Method"}`)
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"].(float64) != -32601 {
		t.Errorf("code = %v", errObj["code"])
	}
}

func TestWebBridgeMethods(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("a.B/C", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) { return nil, nil })
	bridge.Register("d.E/F", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) { return nil, nil })

	if len(bridge.Methods()) != 2 {
		t.Errorf("got %d methods, want 2", len(bridge.Methods()))
	}
}

// --- Go → Browser (bidirectional) ---

func TestWebBridgeGoCallsBrowser(t *testing.T) {
	bridge := transport.NewWebBridge()

	// Capture the WebConn when browser connects
	var conn *transport.WebConn
	var connReady sync.WaitGroup
	connReady.Add(1)
	bridge.OnConnect(func(c *transport.WebConn) {
		conn = c
		connReady.Done()
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	// Connect a "browser" that handles incoming requests
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	// Browser-side handler: reads a request, sends a response
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg map[string]interface{}
			json.Unmarshal(data, &msg)

			if method, ok := msg["method"].(string); ok && method == "ui.v1.UIService/GetViewport" {
				resp, _ := json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      msg["id"],
					"result":  map[string]int{"width": 1920, "height": 1080},
				})
				c.Write(ctx, websocket.MessageText, resp)
			}
		}
	}()

	connReady.Wait()

	// Go→Browser invocation
	payload, _ := json.Marshal(map[string]string{})
	result, err := conn.InvokeWithTimeout("ui.v1.UIService/GetViewport", payload, 2*time.Second)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}

	var viewport map[string]float64
	json.Unmarshal(result, &viewport)
	if viewport["width"] != 1920 {
		t.Errorf("width = %v", viewport["width"])
	}
	if viewport["height"] != 1080 {
		t.Errorf("height = %v", viewport["height"])
	}
}

func TestWebBridgeGoCallsBrowserError(t *testing.T) {
	bridge := transport.NewWebBridge()

	var conn *transport.WebConn
	var connReady sync.WaitGroup
	connReady.Add(1)
	bridge.OnConnect(func(c *transport.WebConn) {
		conn = c
		connReady.Done()
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	// Browser always responds with an error
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg map[string]interface{}
			json.Unmarshal(data, &msg)

			if _, ok := msg["method"]; ok {
				resp, _ := json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      msg["id"],
					"error":   map[string]interface{}{"code": -32700, "message": "not supported"},
				})
				c.Write(ctx, websocket.MessageText, resp)
			}
		}
	}()

	connReady.Wait()

	_, err := conn.InvokeWithTimeout("any.Method/Here", nil, 2*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}

	webErr, ok := err.(*transport.WebError)
	if !ok {
		t.Fatalf("expected WebError, got %T: %v", err, err)
	}
	if webErr.Code != -32700 {
		t.Errorf("code = %d, want -32700", webErr.Code)
	}
}

func TestWebBridgeInvalidJSON(t *testing.T) {
	bridge := transport.NewWebBridge()
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	ctx := context.Background()
	if err := c.Write(ctx, websocket.MessageText, []byte(`{bad-json`)); err != nil {
		t.Fatal(err)
	}

	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp["jsonrpc"]; got != "2.0" {
		t.Fatalf("jsonrpc = %v, want 2.0", got)
	}

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got: %v", resp)
	}
	if errObj["code"].(float64) != -32700 {
		t.Fatalf("expected code=-32700 for invalid JSON, got: %v", errObj["code"])
	}
}

func TestWebBridgeInvalidRequestMissingMethod(t *testing.T) {
	bridge := transport.NewWebBridge()
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"bad-req","params":{"x":1}}`)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error response, got: %v", resp)
	}
	if errObj["code"].(float64) != -32600 {
		t.Fatalf("code = %v, want -32600", errObj["code"])
	}
}

func TestWebBridgeHeartbeatCompatibility(t *testing.T) {
	bridge := transport.NewWebBridge()
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"h1","method":"rpc.heartbeat","params":{}}`)
	if resp["id"] != "h1" {
		t.Fatalf("id = %v, want h1", resp["id"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected success envelope for rpc.heartbeat: %v", resp)
	}
	if len(result) != 0 {
		t.Fatalf("heartbeat result = %v, want {}", result)
	}
}

func TestWebBridgeHeartbeatSuccess(t *testing.T) {
	bridge := transport.NewWebBridge()
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"hb-1","method":"rpc.heartbeat","params":{}}`)
	if resp["id"] != "hb-1" {
		t.Fatalf("id = %v, want hb-1", resp["id"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected heartbeat success response, got: %v", resp)
	}
	if len(result) != 0 {
		t.Fatalf("heartbeat result = %v, want {}", result)
	}
}

func TestWebBridgeNotification(t *testing.T) {
	bridge := transport.NewWebBridge()
	called := make(chan struct{}, 1)
	bridge.Register("notify.v1.Notify/Send", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		var body map[string]interface{}
		_ = json.Unmarshal(payload, &body)
		called <- struct{}{}
		return json.Marshal(map[string]bool{"ok": true})
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := `{"jsonrpc":"2.0","method":"notify.v1.Notify/Send","params":{"value":"x"}}`
	if err := c.Write(ctx, websocket.MessageText, []byte(req)); err != nil {
		t.Fatalf("write notification: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("notification handler was not called")
	}

	readCtx, cancelRead := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancelRead()
	_, _, err := c.Read(readCtx)
	if err == nil {
		t.Fatal("expected no response for notification")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) && !strings.Contains(err.Error(), "closed network connection") {
		t.Fatalf("unexpected notification read error: %v", err)
	}
}

func TestWebBridgeInvalidJSONRPCVersion(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("hello.v1.HelloService/Greet", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]string{"message": "ok"})
	})
	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"1.0","id":"v1","method":"hello.v1.HelloService/Greet","params":{}}`)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got: %v", resp)
	}
	if errObj["code"].(float64) != -32600 {
		t.Fatalf("code = %v, want -32600", errObj["code"])
	}
}

func TestWebBridgeIgnoresUnknownAndDuplicateResponseIDs(t *testing.T) {
	bridge := transport.NewWebBridge()

	var conn *transport.WebConn
	var connReady sync.WaitGroup
	connReady.Add(1)
	bridge.OnConnect(func(c *transport.WebConn) {
		conn = c
		connReady.Done()
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	// Browser behavior:
	// 1) send an unknown response id (must be ignored by bridge)
	// 2) send the expected response
	// 3) send a duplicate response with the same id (must be ignored)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}

			var req map[string]interface{}
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}

			id, ok := req["id"].(string)
			if !ok || id == "" {
				continue
			}
			if _, ok := req["method"].(string); !ok {
				continue
			}

			unknownResp, _ := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      "unknown-" + id,
				"result":  map[string]bool{"ignored": true},
			})
			c.Write(ctx, websocket.MessageText, unknownResp)

			mainResp, _ := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]string{"echoId": id},
			})
			c.Write(ctx, websocket.MessageText, mainResp)
			c.Write(ctx, websocket.MessageText, mainResp)
		}
	}()

	connReady.Wait()

	for i := 0; i < 2; i++ {
		payload, _ := json.Marshal(map[string]string{})
		result, err := conn.InvokeWithTimeout("ui.v1.UIService/GetViewport", payload, 2*time.Second)
		if err != nil {
			t.Fatalf("invoke %d failed: %v", i+1, err)
		}

		var out map[string]string
		if err := json.Unmarshal(result, &out); err != nil {
			t.Fatalf("unmarshal invoke %d result: %v", i+1, err)
		}
		if out["echoId"] == "" {
			t.Fatalf("invoke %d returned empty echoId: %v", i+1, out)
		}
	}
}

func TestWebBridgeConcurrentInvokeMultipleClients(t *testing.T) {
	bridge := transport.NewWebBridge()

	const clientCount = 4
	connCh := make(chan *transport.WebConn, clientCount)
	bridge.OnConnect(func(c *transport.WebConn) {
		connCh <- c
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	browserConns := make([]*websocket.Conn, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		c := dialTestBridge(t, srv)
		browserConns = append(browserConns, c)

		clientID := i
		go func(conn *websocket.Conn, id int) {
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}

				var req map[string]interface{}
				if err := json.Unmarshal(data, &req); err != nil {
					continue
				}

				reqID, _ := req["id"].(string)
				if reqID == "" {
					continue
				}
				if _, ok := req["method"].(string); !ok {
					continue
				}

				resp, _ := json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      reqID,
					"result":  map[string]int{"client": id},
				})
				_ = conn.Write(ctx, websocket.MessageText, resp)
			}
		}(c, clientID)
	}
	defer func() {
		for _, c := range browserConns {
			c.CloseNow()
		}
	}()

	webConns := make([]*transport.WebConn, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		select {
		case wc := <-connCh:
			webConns = append(webConns, wc)
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for connection %d", i+1)
		}
	}

	type invokeResult struct {
		client int
		got    int
		err    error
	}
	results := make(chan invokeResult, clientCount)

	var wg sync.WaitGroup
	for i, wc := range webConns {
		wg.Add(1)
		go func(client int, conn *transport.WebConn) {
			defer wg.Done()

			payload, _ := json.Marshal(map[string]int{"requestClient": client})
			out, err := conn.InvokeWithTimeout("ui.v1.UIService/GetViewport", payload, 2*time.Second)
			if err != nil {
				results <- invokeResult{client: client, err: err}
				return
			}

			var body map[string]int
			if err := json.Unmarshal(out, &body); err != nil {
				results <- invokeResult{client: client, err: err}
				return
			}
			results <- invokeResult{client: client, got: body["client"]}
		}(i, wc)
	}
	wg.Wait()
	close(results)

	seen := make(map[int]bool, clientCount)
	for r := range results {
		if r.err != nil {
			t.Fatalf("invoke for client %d failed: %v", r.client, r.err)
		}
		seen[r.got] = true
	}

	if len(seen) != clientCount {
		t.Fatalf("expected responses from %d distinct clients, got %d", clientCount, len(seen))
	}
}

func TestWebBridgeInvokeConnectionDropMidCall(t *testing.T) {
	bridge := transport.NewWebBridge()

	connCh := make(chan *transport.WebConn, 1)
	bridge.OnConnect(func(c *transport.WebConn) {
		connCh <- c
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	go func() {
		_, _, err := c.Read(ctx)
		if err != nil {
			return
		}
		_ = c.Close(websocket.StatusNormalClosure, "disconnect mid-invoke")
	}()

	var conn *transport.WebConn
	select {
	case conn = <-connCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server-side connection")
	}

	_, err := conn.InvokeWithTimeout("ui.v1.UIService/GetViewport", nil, 2*time.Second)
	if err == nil {
		t.Fatal("expected invoke to fail after client disconnect")
	}
	if !strings.Contains(err.Error(), "connection closed") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("unexpected error on disconnect: %v", err)
	}
}

func TestWebBridgeConcurrentLoad(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("echo.v1.Echo/Ping", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return payload, nil
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	const n = 50
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()

			conn := dialTestBridge(t, srv)
			defer conn.CloseNow()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			req, err := json.Marshal(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      fmt.Sprintf("c-%d", i),
				"method":  "echo.v1.Echo/Ping",
				"params":  map[string]int{"n": i},
			})
			if err != nil {
				errCh <- err
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, req); err != nil {
				errCh <- err
				return
			}

			_, data, err := conn.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(data, &resp); err != nil {
				errCh <- err
				return
			}
			result, ok := resp["result"].(map[string]interface{})
			if !ok {
				errCh <- fmt.Errorf("missing result: %v", resp)
				return
			}
			got, _ := result["n"].(float64)
			if int(got) != i {
				errCh <- fmt.Errorf("roundtrip mismatch: got %v want %d", got, i)
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWebBridgeResourceCleanup(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("rpc.heartbeat", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	before := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		c := dialTestBridge(t, srv)
		_ = roundTrip(t, c, `{"jsonrpc":"2.0","id":"r1","method":"rpc.heartbeat","params":{}}`)
		c.CloseNow()
	}

	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	after := runtime.NumGoroutine()

	delta := after - before
	if delta < 0 {
		delta = -delta
	}
	if delta > 5 {
		t.Fatalf("goroutine delta = %d (before=%d after=%d), want <= 5", delta, before, after)
	}
}

func TestWebBridgeMalformedJSONPayloads(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("hello.v1.HelloService/Greet", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, &transport.WebError{Code: -32700, Message: "invalid payload"}
		}
		return json.Marshal(map[string]string{"message": fmt.Sprintf("Hello, %s!", req.Name)})
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	testCases := []struct {
		name     string
		message  string
		wantCode float64
	}{
		{
			name:     "invalid-json-document",
			message:  `{bad-json`,
			wantCode: -32700,
		},
		{
			name:     "invalid-envelope-field-type",
			message:  `{"jsonrpc":"2.0","id":"e1","method":123,"params":{}}`,
			wantCode: -32600,
		},
		{
			name:     "malformed-handler-payload-shape",
			message:  `{"jsonrpc":"2.0","id":"e2","method":"hello.v1.HelloService/Greet","params":"not-an-object"}`,
			wantCode: -32700,
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := c.Write(ctx, websocket.MessageText, []byte(tc.message)); err != nil {
				t.Fatalf("write malformed payload: %v", err)
			}

			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("read malformed payload response: %v", err)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got := resp["jsonrpc"]; got != "2.0" {
				t.Fatalf("jsonrpc = %v, want 2.0", got)
			}

			errObj, ok := resp["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected error response, got: %v", resp)
			}
			if errObj["code"].(float64) != tc.wantCode {
				t.Fatalf("error code = %v, want %v", errObj["code"], tc.wantCode)
			}
		})
	}
}

func TestMessageSizeRejection(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("echo.v1.Echo/Ping", func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		return payload, nil
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	oversizeConn := dialTestBridge(t, srv)
	defer oversizeConn.CloseNow()

	oversized := strings.Repeat("a", 2*1024*1024)
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "big-1",
		"method":  "echo.v1.Echo/Ping",
		"params":  map[string]string{"message": oversized},
	})
	if err != nil {
		t.Fatalf("marshal oversized request: %v", err)
	}

	readErrCh := make(chan error, 1)
	go func() {
		readCtx, readCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer readCancel()
		_, _, readErr := oversizeConn.Read(readCtx)
		readErrCh <- readErr
	}()

	writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	writer, err := oversizeConn.Writer(writeCtx, websocket.MessageText)
	if err != nil {
		writeCancel()
		t.Fatalf("open oversized writer: %v", err)
	}

	const chunk = 16 * 1024
	for off := 0; off < len(req); off += chunk {
		end := off + chunk
		if end > len(req) {
			end = len(req)
		}
		if _, err := writer.Write(req[off:end]); err != nil {
			break
		}
	}
	_ = writer.Close()
	writeCancel()

	var readErr error
	select {
	case readErr = <-readErrCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for oversize close frame")
	}
	if status := websocket.CloseStatus(readErr); status != websocket.StatusMessageTooBig {
		t.Fatalf("close status = %v, want %v (err=%v)", status, websocket.StatusMessageTooBig, readErr)
	}

	// The server must stay healthy after rejecting the oversized frame.
	probeConn := dialTestBridge(t, srv)
	defer probeConn.CloseNow()
	resp := roundTrip(t, probeConn, `{"jsonrpc":"2.0","id":"ok-1","method":"echo.v1.Echo/Ping","params":{"message":"ok"}}`)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected success response after oversize rejection, got: %v", resp)
	}
	if got, _ := result["message"].(string); got != "ok" {
		t.Fatalf("post-rejection echo mismatch: got %q want %q", got, "ok")
	}
}

func TestWebBridgeAllowOrigins(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.AllowOrigins("allowed.example")

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	ctx := context.Background()
	wsURL := "ws" + srv.URL[4:]

	_, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"holon-rpc"},
		HTTPHeader:   http.Header{"Origin": []string{"blocked.example"}},
	})
	if err == nil {
		t.Fatal("expected origin check failure")
	}

	bridge.AllowOrigins()
	openConn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		Subprotocols: []string{"holon-rpc"},
	})
	if err != nil {
		t.Fatalf("dial with unrestricted origins: %v", err)
	}
	openConn.CloseNow()
}

func TestWebBridgeMarshalResponseFailure(t *testing.T) {
	bridge := transport.NewWebBridge()
	bridge.Register("bad.v1.Service/Method", func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		// Invalid RawMessage bytes trigger marshalWsResp fallback envelope.
		return json.RawMessage(`{bad-json`), nil
	})

	srv := httptest.NewServer(bridge)
	defer srv.Close()

	c := dialTestBridge(t, srv)
	defer c.CloseNow()

	resp := roundTrip(t, c, `{"jsonrpc":"2.0","id":"m1","method":"bad.v1.Service/Method","params":{}}`)
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error response, got: %v", resp)
	}
	if errObj["code"].(float64) != -32603 {
		t.Fatalf("error code = %v, want -32603", errObj["code"])
	}
}

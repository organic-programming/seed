package holonrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

func startHolonRPCServer(t *testing.T, register func(*holonrpc.Server)) (*holonrpc.Server, string) {
	t.Helper()

	server := holonrpc.NewServer("ws://127.0.0.1:0/rpc")
	if register != nil {
		register(server)
	}

	addr, err := server.Start()
	if err != nil {
		t.Fatalf("start server: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})

	return server, addr
}

func dialRawHolonRPC(t *testing.T, addr string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, addr, &websocket.DialOptions{
		Subprotocols: []string{"holon-rpc"},
	})
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return ws
}

func connectHolonRPCClient(t *testing.T, addr string) *holonrpc.Client {
	t.Helper()

	client := holonrpc.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, addr); err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func readWSJSONMap(t *testing.T, ws *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return out
}

func requireRPCErrorCode(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected RPC error code %d, got nil", want)
	}

	var rpcErr *holonrpc.ResponseError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected ResponseError, got %T: %v", err, err)
	}
	if rpcErr.Code != want {
		t.Fatalf("error code = %d, want %d", rpcErr.Code, want)
	}
}

func requireWireErrorCode(t *testing.T, msg map[string]any, want int) {
	t.Helper()
	errObj, ok := msg["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got: %#v", msg)
	}
	got, ok := errObj["code"].(float64)
	if !ok {
		t.Fatalf("expected numeric error code, got: %#v", errObj["code"])
	}
	if int(got) != want {
		t.Fatalf("error code = %v, want %d", got, want)
	}
}

func TestHolonRPCGoAgainstGoDirectional(t *testing.T) {
	server, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	client := holonrpc.NewClient()
	client.Register("client.v1.Client/Hello", func(_ context.Context, params map[string]any) (map[string]any, error) {
		name, _ := params["name"].(string)
		return map[string]any{"message": fmt.Sprintf("hello %s", name)}, nil
	})
	t.Cleanup(func() {
		_ = client.Close()
	})

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelConnect()
	if err := client.Connect(connectCtx, addr); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	clientIDCtx, cancelClientID := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelClientID()
	clientID, err := server.WaitForClient(clientIDCtx)
	if err != nil {
		t.Fatalf("wait for client: %v", err)
	}

	invokeCtx, cancelInvoke := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelInvoke()
	echo, err := client.Invoke(invokeCtx, "echo.v1.Echo/Ping", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("client invoke echo: %v", err)
	}
	if got, _ := echo["message"].(string); got != "hello" {
		t.Fatalf("echo message = %q, want %q", got, "hello")
	}

	reply, err := server.Invoke(invokeCtx, clientID, "client.v1.Client/Hello", map[string]any{"name": "go"})
	if err != nil {
		t.Fatalf("server invoke client: %v", err)
	}
	if got, _ := reply["message"].(string); got != "hello go" {
		t.Fatalf("client reply message = %q, want %q", got, "hello go")
	}
}

func TestHolonRPCClientRejectsNonDirectionalID(t *testing.T) {
	respCh := make(chan map[string]any, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer c.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		req := map[string]any{
			"jsonrpc": "2.0",
			"id":      "c99",
			"method":  "client.v1.Client/Hello",
			"params":  map[string]any{"name": "go"},
		}
		reqData, _ := json.Marshal(req)
		if err := c.Write(ctx, websocket.MessageText, reqData); err != nil {
			return
		}

		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		respCh <- resp
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	client := holonrpc.NewClient()
	client.Register("client.v1.Client/Hello", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		return map[string]any{"message": "unexpected"}, nil
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, wsURL); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	select {
	case resp := <-respCh:
		requireWireErrorCode(t, resp, -32600)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for rejection response")
	}
}

func TestHolonRPCHeartbeat(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	client := connectHolonRPCClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := client.Invoke(ctx, "rpc.heartbeat", nil)
	if err != nil {
		t.Fatalf("heartbeat invoke: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("heartbeat result = %#v, want empty object", result)
	}
}

func TestHolonRPCPeers(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)
	clientA := connectHolonRPCClient(t, addr)
	_ = connectHolonRPCClient(t, addr)

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelWait()

	wantIDs := make(map[string]struct{}, 2)
	for i := 0; i < 2; i++ {
		id, err := server.WaitForClient(waitCtx)
		if err != nil {
			t.Fatalf("wait for client %d: %v", i+1, err)
		}
		wantIDs[id] = struct{}{}
	}

	callCtx, cancelCall := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelCall()
	result, err := clientA.Invoke(callCtx, "rpc.peers", map[string]any{})
	if err != nil {
		t.Fatalf("rpc.peers invoke: %v", err)
	}

	rawPeers, ok := result["peers"].([]any)
	if !ok {
		t.Fatalf("rpc.peers result peers has type %T, want []any", result["peers"])
	}
	if len(rawPeers) != 2 {
		t.Fatalf("rpc.peers count = %d, want 2", len(rawPeers))
	}

	gotIDs := make(map[string]struct{}, len(rawPeers))
	for _, rawPeer := range rawPeers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			t.Fatalf("peer entry has type %T, want map[string]any", rawPeer)
		}

		id, ok := peer["id"].(string)
		if !ok || id == "" {
			t.Fatalf("peer id = %#v, want non-empty string", peer["id"])
		}
		gotIDs[id] = struct{}{}

		methods, ok := peer["methods"].([]any)
		if !ok {
			t.Fatalf("peer methods has type %T, want []any", peer["methods"])
		}
		if len(methods) != 0 {
			t.Fatalf("peer methods = %#v, want empty list", methods)
		}
	}

	for id := range wantIDs {
		if _, ok := gotIDs[id]; !ok {
			t.Fatalf("rpc.peers missing connected client id %q in %#v", id, gotIDs)
		}
	}
}

func TestHolonRPCNotification(t *testing.T) {
	called := make(chan struct{}, 1)
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("notify.v1.Notify/Send", func(_ context.Context, params map[string]any) (map[string]any, error) {
			if params["value"] == "x" {
				called <- struct{}{}
			}
			return map[string]any{"ok": true}, nil
		})
	})

	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := []byte(`{"jsonrpc":"2.0","method":"notify.v1.Notify/Send","params":{"value":"x"}}`)
	if err := ws.Write(ctx, websocket.MessageText, req); err != nil {
		t.Fatalf("write notification: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("notification handler was not called")
	}

	readCtx, cancelRead := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancelRead()
	_, _, err := ws.Read(readCtx)
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "closed network connection") {
		t.Fatalf("expected no response for notification, got: %v", err)
	}
}

func TestHolonRPCUnknownMethod(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	client := connectHolonRPCClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Invoke(ctx, "does.not.Exist/Method", map[string]any{})
	requireRPCErrorCode(t, err, -32601)
}

func TestHolonRPCInvalidParams(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := []byte(`{"jsonrpc":"2.0","id":"badparams","method":"echo.v1.Echo/Ping","params":"a-string"}`)
	if err := ws.Write(ctx, websocket.MessageText, req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	resp := readWSJSONMap(t, ws, 2*time.Second)
	requireWireErrorCode(t, resp, -32602)
}

func TestHolonRPCParseError(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{bad-json`)); err != nil {
		t.Fatalf("write malformed json: %v", err)
	}

	resp := readWSJSONMap(t, ws, 2*time.Second)
	requireWireErrorCode(t, resp, -32700)
}

func TestHolonRPCInvalidRequestEnvelopeShape(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":"bad-shape","method":123,"params":{}}`)); err != nil {
		t.Fatalf("write invalid request envelope: %v", err)
	}

	resp := readWSJSONMap(t, ws, 2*time.Second)
	requireWireErrorCode(t, resp, -32600)
}

func TestHolonRPCInvalidRequestMissingMethod(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":"missing-method","params":{"x":1}}`)); err != nil {
		t.Fatalf("write invalid request envelope: %v", err)
	}

	resp := readWSJSONMap(t, ws, 2*time.Second)
	requireWireErrorCode(t, resp, -32600)
}

func TestHolonRPCBatchRequestUnsupported(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageText, []byte(`[{"jsonrpc":"2.0","id":"1","method":"rpc.heartbeat","params":{}}]`)); err != nil {
		t.Fatalf("write batch request: %v", err)
	}

	resp := readWSJSONMap(t, ws, 2*time.Second)
	requireWireErrorCode(t, resp, -32600)
}

func TestHolonRPCInternalErrorCode(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("boom.v1.Service/Crash", func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New("boom")
		})
	})
	client := connectHolonRPCClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Invoke(ctx, "boom.v1.Service/Crash", map[string]any{})
	requireRPCErrorCode(t, err, -32603)
}

func TestHolonRPCMethodNotFoundErrorContainsMethodName(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	client := connectHolonRPCClient(t, addr)

	method := "does.not.Exist/Method"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Invoke(ctx, method, map[string]any{})

	var rpcErr *holonrpc.ResponseError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected ResponseError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32601 {
		t.Fatalf("error code = %d, want -32601", rpcErr.Code)
	}
	if !strings.Contains(rpcErr.Message, method) {
		t.Fatalf("error message = %q, want to contain %q", rpcErr.Message, method)
	}
}

func TestHolonRPCServerUnregister(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
		s.Unregister("echo.v1.Echo/Ping")
	})

	client := connectHolonRPCClient(t, addr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Invoke(ctx, "echo.v1.Echo/Ping", map[string]any{"message": "x"})
	requireRPCErrorCode(t, err, -32601)
}

func TestHolonRPCClientConnected(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	client := holonrpc.NewClient()

	if client.Connected() {
		t.Fatal("Connected() = true before Connect, want false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, addr); err != nil {
		t.Fatalf("connect client: %v", err)
	}
	if !client.Connected() {
		t.Fatal("Connected() = false after Connect, want true")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if client.Connected() {
		t.Fatal("Connected() = true after Close, want false")
	}
}

func TestHolonRPCDoubleClose(t *testing.T) {
	_, addr := startHolonRPCServer(t, nil)
	client := holonrpc.NewClient()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, addr); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

func TestHolonRPCConnectGuards(t *testing.T) {
	client := holonrpc.NewClient()

	if err := client.Connect(context.Background(), " "); err == nil {
		t.Fatal("expected empty-url connect error")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if err := client.Connect(context.Background(), "ws://127.0.0.1:1/rpc"); err == nil {
		t.Fatal("expected closed-client connect error")
	}

	_, addr := startHolonRPCServer(t, nil)
	client2 := holonrpc.NewClient()
	t.Cleanup(func() { _ = client2.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := client2.Connect(ctx, addr); err != nil {
		cancel()
		t.Fatalf("initial connect: %v", err)
	}
	cancel()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	err := client2.Connect(ctx2, addr)
	cancel2()
	if err == nil {
		t.Fatal("expected already-connected error")
	}
}

func TestHolonRPCConnectWithReconnectInitialDialFailure(t *testing.T) {
	client := holonrpc.NewClient()
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	err := client.ConnectWithReconnect(ctx, "ws://127.0.0.1:1/rpc")
	cancel()
	if err == nil {
		t.Fatal("expected reconnect-enabled connect dial failure")
	}
	if client.Connected() {
		t.Fatal("Connected() = true after failed reconnect dial, want false")
	}

	invokeCtx, invokeCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer invokeCancel()
	_, invokeErr := client.Invoke(invokeCtx, "rpc.heartbeat", nil)
	if invokeErr == nil {
		t.Fatal("expected invoke error while client is disconnected")
	}
	if strings.Contains(invokeErr.Error(), "connection closed") {
		t.Fatalf("expected no reconnect loop after initial failure, got: %v", invokeErr)
	}
}

func TestHolonRPCClientRejectsMissingSubprotocolNegotiation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer c.CloseNow()

		<-r.Context().Done()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client := holonrpc.NewClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := client.Connect(ctx, wsURL)
	if err == nil {
		t.Fatal("expected connect failure when holon-rpc subprotocol is not negotiated")
	}
	if !strings.Contains(err.Error(), "did not negotiate holon-rpc") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHolonRPCNullIDNotification(t *testing.T) {
	called := make(chan struct{}, 1)
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("notify.v1.Notify/Send", func(_ context.Context, params map[string]any) (map[string]any, error) {
			if params["value"] == "null-id" {
				called <- struct{}{}
			}
			return map[string]any{"ok": true}, nil
		})
	})

	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := []byte(`{"jsonrpc":"2.0","id":null,"method":"notify.v1.Notify/Send","params":{"value":"null-id"}}`)
	if err := ws.Write(ctx, websocket.MessageText, req); err != nil {
		t.Fatalf("write null-id notification: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("null-id notification handler was not called")
	}

	readCtx, readCancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer readCancel()
	_, _, err := ws.Read(readCtx)
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "closed network connection") {
		t.Fatalf("expected no response for null-id notification, got: %v", err)
	}
}

func TestValenceMultiConcurrent(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			// Add small work so the test fails if requests are serialized.
			time.Sleep(300 * time.Millisecond)
			return params, nil
		})
	})

	const clients = 5
	start := make(chan struct{})
	errCh := make(chan error, clients)
	var wg sync.WaitGroup
	wg.Add(clients)

	for i := 0; i < clients; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start

			client := holonrpc.NewClient()
			defer client.Close()

			connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := client.Connect(connectCtx, addr); err != nil {
				connectCancel()
				errCh <- fmt.Errorf("client %d connect: %w", i, err)
				return
			}
			connectCancel()

			callCtx, callCancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
			resp, err := client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": fmt.Sprintf("c-%d", i)})
			callCancel()
			if err != nil {
				errCh <- fmt.Errorf("client %d invoke: %w", i, err)
				return
			}

			want := fmt.Sprintf("c-%d", i)
			got, _ := resp["message"].(string)
			if got != want {
				errCh <- fmt.Errorf("client %d mismatch: got %q want %q", i, got, want)
				return
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

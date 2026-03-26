package holonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestReconnectDelayBounds(t *testing.T) {
	for attempt := 0; attempt < 10; attempt++ {
		got := reconnectDelay(attempt)

		base := float64(reconnectMinDelay) * math.Pow(reconnectFactor, float64(attempt))
		if base > float64(reconnectMaxDelay) {
			base = float64(reconnectMaxDelay)
		}
		min := time.Duration(base)
		max := time.Duration(base * (1 + reconnectJitter))

		if got < min || got > max {
			t.Fatalf("reconnectDelay(%d) = %v, want in [%v, %v]", attempt, got, min, max)
		}
	}
}

func TestDisableReconnectMismatchedDoneIsNoop(t *testing.T) {
	client := NewClient()

	client.stateMu.Lock()
	client.reconnectDone = make(chan struct{})
	client.stateMu.Unlock()

	done := make(chan struct{})
	go func() {
		client.disableReconnect(make(chan struct{}))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("disableReconnect blocked on mismatched done channel")
	}
}

func TestServerStartDefaultsIdempotentAndClosed(t *testing.T) {
	server := NewServer("ws://:0")

	addr1, err := server.Start()
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	if !strings.HasPrefix(addr1, "ws://127.0.0.1:") || !strings.HasSuffix(addr1, "/rpc") {
		t.Fatalf("server address = %q, want ws://127.0.0.1:<port>/rpc", addr1)
	}

	addr2, err := server.Start()
	if err != nil {
		t.Fatalf("start server second call: %v", err)
	}
	if addr2 != addr1 {
		t.Fatalf("idempotent start address mismatch: got %q want %q", addr2, addr1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		t.Fatalf("close server: %v", err)
	}

	if _, err := server.Start(); err == nil {
		t.Fatal("expected start to fail after close")
	}
}

func TestServerInvokeUnknownClient(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	if _, err := server.Invoke(context.Background(), "missing", "echo.v1.Echo/Ping", map[string]any{}); err == nil {
		t.Fatal("expected invoke error for unknown client")
	}
}

func TestClientHandleResponseInvalidVersion(t *testing.T) {
	client := NewClient()

	id := makeID("c1")
	key, ok := idKey(id)
	if !ok {
		t.Fatal("idKey failed")
	}

	ch := make(chan rpcMessage, 1)
	client.pending[key] = ch

	client.handleResponse(rpcMessage{
		JSONRPC: "1.0",
		ID:      id,
		Result:  json.RawMessage(`{"ok":true}`),
	})

	select {
	case resp := <-ch:
		if resp.Error == nil || resp.Error.Code != codeInvalidRequest {
			t.Fatalf("expected invalid response error, got: %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for pending response")
	}
}

func TestServerHandlePeerResponseInvalidVersion(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")

	id := makeID("s1")
	key, ok := idKey(id)
	if !ok {
		t.Fatal("idKey failed")
	}

	ch := make(chan rpcMessage, 1)
	peer := &serverPeer{
		pending: map[string]chan rpcMessage{
			key: ch,
		},
	}

	server.handlePeerResponse(peer, rpcMessage{
		JSONRPC: "1.0",
		ID:      id,
		Result:  json.RawMessage(`{"ok":true}`),
	})

	select {
	case resp := <-ch:
		if resp.Error == nil || resp.Error.Code != codeInvalidRequest {
			t.Fatalf("expected invalid response error, got: %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for peer pending response")
	}
}

func TestClientReadLoopInvalidEnvelopeWithID(t *testing.T) {
	respCh := make(chan rpcMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer ws.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := ws.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":"bad-1","params":{"x":1}}`)); err != nil {
			return
		}

		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var resp rpcMessage
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		respCh <- resp
	}))
	defer srv.Close()

	client := NewClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, wsURL(srv.URL)); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error == nil || resp.Error.Code != codeInvalidRequest {
			t.Fatalf("expected invalid-request error response, got: %#v", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for invalid-request response")
	}
}

func TestClientReadLoopIgnoresBinaryFrames(t *testing.T) {
	respCh := make(chan rpcMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer ws.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		_ = ws.Write(ctx, websocket.MessageBinary, []byte("binary"))
		if err := ws.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":"s1","method":"rpc.heartbeat","params":{}}`)); err != nil {
			return
		}

		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var resp rpcMessage
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		respCh <- resp
	}))
	defer srv.Close()

	client := NewClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, wsURL(srv.URL)); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error != nil {
			t.Fatalf("expected heartbeat success after binary frame, got error: %v", resp.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for heartbeat response")
	}
}

func TestClientSendResultMarshalFailureReturnsInternalError(t *testing.T) {
	respCh := make(chan rpcMessage, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:       []string{"holon-rpc"},
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer ws.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := ws.Write(ctx, websocket.MessageText, []byte(`{"jsonrpc":"2.0","id":"s9","method":"client.v1.Client/Marshal","params":{}}`)); err != nil {
			return
		}

		_, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		var resp rpcMessage
		if err := json.Unmarshal(data, &resp); err != nil {
			return
		}
		respCh <- resp
	}))
	defer srv.Close()

	client := NewClient()
	client.Register("client.v1.Client/Marshal", func(context.Context, map[string]any) (map[string]any, error) {
		return map[string]any{"bad": make(chan int)}, nil
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, wsURL(srv.URL)); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	select {
	case resp := <-respCh:
		if resp.Error == nil || resp.Error.Code != codeInternalError {
			t.Fatalf("expected internal error response, got: %#v", resp)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for marshal failure response")
	}
}

func TestServerSendPeerResultMarshalFailureReturnsInternalError(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	server.Register("echo.v1.Echo/Ping", func(context.Context, map[string]any) (map[string]any, error) {
		return map[string]any{"bad": make(chan int)}, nil
	})

	addr, err := server.Start()
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	}()

	client := NewClient()
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx, addr); err != nil {
		t.Fatalf("connect client: %v", err)
	}

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()
	_, err = client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": "x"})
	if err == nil {
		t.Fatal("expected internal error from marshal failure")
	}

	var rpcErr *ResponseError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected ResponseError, got %T: %v", err, err)
	}
	if rpcErr.Code != codeInternalError {
		t.Fatalf("error code = %d, want %d", rpcErr.Code, codeInternalError)
	}
}

func TestSendPeerErrorMarshalFailure(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	err := server.sendPeerError(nil, json.RawMessage("{"), codeInternalError, "bad", nil)
	if err == nil {
		t.Fatal("expected marshal error for invalid raw id")
	}
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

package holonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDecodeParamsAndResultHelpers(t *testing.T) {
	tests := []struct {
		name    string
		raw     json.RawMessage
		wantErr bool
	}{
		{name: "empty", raw: nil},
		{name: "null", raw: json.RawMessage("null")},
		{name: "object", raw: json.RawMessage(`{"a":1}`)},
		{name: "invalid-type", raw: json.RawMessage(`"bad"`), wantErr: true},
		{name: "invalid-json", raw: json.RawMessage(`{bad`), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeParams(tc.raw)
			if tc.wantErr && err == nil {
				t.Fatal("expected decodeParams error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected decodeParams error: %v", err)
			}
		})
	}

	resultTests := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "empty", raw: nil},
		{name: "null", raw: json.RawMessage("null")},
		{name: "object", raw: json.RawMessage(`{"ok":true}`)},
		{name: "scalar", raw: json.RawMessage(`"x"`)},
	}
	for _, tc := range resultTests {
		t.Run("result-"+tc.name, func(t *testing.T) {
			got, err := decodeResult(tc.raw)
			if err != nil {
				t.Fatalf("decodeResult error: %v", err)
			}
			if got == nil {
				t.Fatal("decodeResult returned nil map")
			}
		})
	}

	if _, err := decodeResult(json.RawMessage(`{bad`)); err == nil {
		t.Fatal("expected decodeResult error for malformed JSON")
	}
}

func TestIDHelpersAndMarshalHelpers(t *testing.T) {
	id := makeID("s1")
	if !hasID(id) {
		t.Fatal("hasID returned false for valid id")
	}
	if hasID(json.RawMessage("null")) {
		t.Fatal("hasID returned true for null id")
	}
	if key, ok := idKey(id); !ok || key == "" {
		t.Fatalf("idKey failed: ok=%v key=%q", ok, key)
	}
	if _, ok := idKey(json.RawMessage("null")); ok {
		t.Fatal("idKey should reject null ids")
	}

	if _, err := decodeStringID(json.RawMessage(`123`)); err == nil {
		t.Fatal("decodeStringID should fail on non-string ids")
	}
	if out, err := decodeStringID(makeID("abc")); err != nil || out != "abc" {
		t.Fatalf("decodeStringID mismatch: out=%q err=%v", out, err)
	}

	if _, err := marshalObject(map[string]any{"ch": make(chan int)}); err == nil {
		t.Fatal("marshalObject should fail for non-serializable values")
	}
	if out, err := marshalObject(nil); err != nil || string(out) != "{}" {
		t.Fatalf("marshalObject(nil) = %q, err=%v", string(out), err)
	}
	if _, err := marshalMessage(rpcMessage{
		JSONRPC: jsonRPCVersion,
		ID:      makeID("x"),
		Result:  json.RawMessage("{}"),
	}); err != nil {
		t.Fatalf("marshalMessage error: %v", err)
	}
}

func TestResponseErrorErrorFormatting(t *testing.T) {
	var nilErr *ResponseError
	if got := nilErr.Error(); got == "" {
		t.Fatal("nil ResponseError.Error() returned empty string")
	}

	errNoData := (&ResponseError{Code: -32600, Message: "invalid request"}).Error()
	if errNoData == "" || errNoData == "invalid request" {
		t.Fatalf("unexpected error string: %q", errNoData)
	}

	errWithData := (&ResponseError{Code: -32602, Message: "invalid params", Data: map[string]any{"field": "x"}}).Error()
	if errWithData == "" || !strings.Contains(errWithData, "data:") {
		t.Fatalf("expected data in error string, got: %q", errWithData)
	}
}

func TestClientCurrentConnAndUnregister(t *testing.T) {
	client := NewClient()
	client.Register("x", func(context.Context, map[string]any) (map[string]any, error) { return nil, nil })
	client.Unregister("x")

	if _, _, err := client.currentConn(); err == nil {
		t.Fatal("expected not-connected error")
	}

	client.stateMu.Lock()
	client.closed = true
	client.stateMu.Unlock()
	if _, _, err := client.currentConn(); err == nil {
		t.Fatal("expected closed error")
	}
}

func TestClientHandleRequestNotificationPaths(t *testing.T) {
	client := NewClient()
	called := 0
	client.Register("ok.v1/Test", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		called++
		return map[string]any{"ok": true}, nil
	})
	client.Register("err.v1/Test", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		return nil, errors.New("boom")
	})

	ctx := context.Background()
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: "1.0", Method: "ok.v1/Test"})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "   "})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "rpc.heartbeat"})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "ok.v1/Test", Params: json.RawMessage(`"bad"`)})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "missing.v1/Test"})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "err.v1/Test", Params: json.RawMessage(`{}`)})
	client.handleRequest(nil, ctx, rpcMessage{JSONRPC: jsonRPCVersion, Method: "ok.v1/Test", Params: json.RawMessage(`{}`)})

	if called != 1 {
		t.Fatalf("handler call count = %d, want 1", called)
	}
}

func TestServerAddressClientIDsAndStartErrors(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	if got := server.Address(); got != "ws://127.0.0.1:0/rpc" {
		t.Fatalf("Address() = %q", got)
	}

	server.peersMu.Lock()
	server.peers["a"] = &serverPeer{id: "a"}
	server.peers["b"] = &serverPeer{id: "b"}
	server.peersMu.Unlock()

	ids := server.ClientIDs()
	if len(ids) != 2 {
		t.Fatalf("ClientIDs len = %d, want 2", len(ids))
	}

	bad := NewServer("://bad")
	if _, err := bad.Start(); err == nil {
		t.Fatal("expected invalid URL error")
	}

	unsupported := NewServer("http://127.0.0.1:0/rpc")
	if _, err := unsupported.Start(); err == nil {
		t.Fatal("expected unsupported-scheme error")
	}
}

func TestServerWaitForClientTimeout(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := server.WaitForClient(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got: %v", err)
	}
}

func TestServerHandlePeerRequestNotificationPaths(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	called := 0
	server.Register("ok.v1/Test", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		called++
		return map[string]any{"ok": true}, nil
	})
	server.Register("err.v1/Test", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		return nil, errors.New("boom")
	})

	peerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	peer := &serverPeer{
		ctx:     peerCtx,
		cancel:  cancel,
		pending: make(map[string]chan rpcMessage),
	}

	server.handlePeerRequest(peer, rpcMessage{JSONRPC: "1.0", Method: "ok.v1/Test"})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "   "})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "rpc.heartbeat"})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "ok.v1/Test", Params: json.RawMessage(`"bad"`)})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "missing.v1/Test"})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "err.v1/Test", Params: json.RawMessage(`{}`)})
	server.handlePeerRequest(peer, rpcMessage{JSONRPC: jsonRPCVersion, Method: "ok.v1/Test", Params: json.RawMessage(`{}`)})

	if called != 1 {
		t.Fatalf("handler call count = %d, want 1", called)
	}
}

func TestCallPeerHandlerContextCanceled(t *testing.T) {
	server := NewServer("ws://127.0.0.1:0/rpc")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := server.callPeerHandler(ctx, func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}, map[string]any{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got: %v", err)
	}
}

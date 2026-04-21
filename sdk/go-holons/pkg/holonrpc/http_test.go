package holonrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

func startHTTPHolonRPCServer(t *testing.T, register func(*holonrpc.HTTPServer)) (*holonrpc.HTTPServer, string) {
	t.Helper()

	server := holonrpc.NewHTTPServer("http://127.0.0.1:0/api/v1/rpc")
	if register != nil {
		register(server)
	}

	addr, err := server.Start()
	if err != nil {
		t.Fatalf("start http server: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})

	return server, addr
}

func TestHTTPHolonRPCInvoke(t *testing.T) {
	_, addr := startHTTPHolonRPCServer(t, func(s *holonrpc.HTTPServer) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	reqBody := bytes.NewBufferString(`{"message":"hello"}`)
	req, err := http.NewRequest(http.MethodPost, addr+"/echo.v1.Echo/Ping", reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("raw invoke: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", payload["result"])
	}
	if result["message"] != "hello" {
		t.Fatalf("result.message = %#v, want hello", result["message"])
	}

	client := holonrpc.NewHTTPClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := client.Invoke(ctx, "echo.v1.Echo/Ping", map[string]any{"message": "hola"})
	if err != nil {
		t.Fatalf("client invoke: %v", err)
	}
	if out["message"] != "hola" {
		t.Fatalf("client result.message = %#v, want hola", out["message"])
	}
}

func TestHTTPHolonRPCStreamPOST(t *testing.T) {
	_, addr := startHTTPHolonRPCServer(t, func(s *holonrpc.HTTPServer) {
		s.RegisterStream("build.v1.Build/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
			if got := params["project"]; got != "myapp" {
				t.Fatalf("project = %#v, want myapp", got)
			}
			if err := send(map[string]any{"status": "building", "progress": 42.0}); err != nil {
				return err
			}
			return send(map[string]any{"status": "done", "progress": 100.0})
		})
	})

	req, err := http.NewRequest(http.MethodPost, addr+"/build.v1.Build/Watch", bytes.NewBufferString(`{"project":"myapp"}`))
	if err != nil {
		t.Fatalf("new stream request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("raw stream request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
	_ = resp.Body.Close()

	client := holonrpc.NewHTTPClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, err := client.Stream(ctx, "build.v1.Build/Watch", map[string]any{"project": "myapp"})
	if err != nil {
		t.Fatalf("stream post: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3 (message, message, done)", len(events))
	}
	if events[0].Event != "message" || events[0].ID != "1" {
		t.Fatalf("first event = %+v, want message id=1", events[0])
	}
	if events[0].Result["status"] != "building" {
		t.Fatalf("first result.status = %#v, want building", events[0].Result["status"])
	}
	if events[1].Result["status"] != "done" {
		t.Fatalf("second result.status = %#v, want done", events[1].Result["status"])
	}
	if events[2].Event != "done" {
		t.Fatalf("last event = %+v, want done", events[2])
	}
}

func TestHTTPHolonRPCStreamGET(t *testing.T) {
	_, addr := startHTTPHolonRPCServer(t, func(s *holonrpc.HTTPServer) {
		s.RegisterStream("build.v1.Build/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
			if got := params["project"]; got != "myapp" {
				t.Fatalf("project = %#v, want myapp", got)
			}
			return send(map[string]any{"status": "watching"})
		})
	})

	client := holonrpc.NewHTTPClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, err := client.StreamQuery(ctx, "build.v1.Build/Watch", map[string]string{"project": "myapp"})
	if err != nil {
		t.Fatalf("stream query: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (message, done)", len(events))
	}
	if events[0].Result["status"] != "watching" {
		t.Fatalf("result.status = %#v, want watching", events[0].Result["status"])
	}
	if events[1].Event != "done" {
		t.Fatalf("last event = %+v, want done", events[1])
	}
}

func TestHTTPHolonRPCCORSPreflight(t *testing.T) {
	_, addr := startHTTPHolonRPCServer(t, nil)

	req, err := http.NewRequest(http.MethodOptions, addr+"/echo.v1.Echo/Ping", nil)
	if err != nil {
		t.Fatalf("new options request: %v", err)
	}
	req.Header.Set("Origin", "https://example.test")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.test" {
		t.Fatalf("allow-origin = %q, want request origin", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("allow-methods = %q", got)
	}
}

func TestHTTPHolonRPCMethodNotFound(t *testing.T) {
	_, addr := startHTTPHolonRPCServer(t, nil)

	client := holonrpc.NewHTTPClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Invoke(ctx, "missing.v1.Service/Method", nil)
	if err == nil {
		t.Fatal("expected not-found error")
	}

	var rpcErr *holonrpc.ResponseError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected ResponseError, got %T: %v", err, err)
	}
	if rpcErr.Code != 5 {
		t.Fatalf("error code = %d, want 5", rpcErr.Code)
	}
}

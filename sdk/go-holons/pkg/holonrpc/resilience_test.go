package holonrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

func TestHolonRPCConcurrentLoad(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	client := connectHolonRPCClient(t, addr)

	const n = 50
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			want := fmt.Sprintf("msg-%d", i)
			resp, err := client.Invoke(ctx, "echo.v1.Echo/Ping", map[string]any{"message": want})
			if err != nil {
				errCh <- fmt.Errorf("invoke %d failed: %w", i, err)
				return
			}

			got, _ := resp["message"].(string)
			if got != want {
				errCh <- fmt.Errorf("invoke %d response mismatch: got %q want %q", i, got, want)
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

func TestHolonRPCResourceCleanup(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	before := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		client := holonrpc.NewClient()

		connectCtx, cancelConnect := context.WithTimeout(context.Background(), 2*time.Second)
		if err := client.Connect(connectCtx, addr); err != nil {
			cancelConnect()
			t.Fatalf("connect cycle %d: %v", i, err)
		}
		cancelConnect()

		invokeCtx, cancelInvoke := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Invoke(invokeCtx, "echo.v1.Echo/Ping", map[string]any{"i": i})
		cancelInvoke()
		if err != nil {
			_ = client.Close()
			t.Fatalf("invoke cycle %d: %v", i, err)
		}

		if err := client.Close(); err != nil {
			t.Fatalf("close cycle %d: %v", i, err)
		}
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

func TestHolonRPCTimeoutPropagation(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("slow.v1.Sleep/Wait", func(_ context.Context, _ map[string]any) (map[string]any, error) {
			time.Sleep(5 * time.Second)
			return map[string]any{"ok": true}, nil
		})
	})

	client := connectHolonRPCClient(t, addr)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Invoke(ctx, "slow.v1.Sleep/Wait", map[string]any{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout propagation too slow: elapsed=%v", elapsed)
	}
}

func TestHolonRPCChaos(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			time.Sleep(80 * time.Millisecond)
			return params, nil
		})
	})

	const clients = 20
	wsConns := make([]*websocket.Conn, 0, clients)
	for i := 0; i < clients; i++ {
		wsConns = append(wsConns, dialRawHolonRPC(t, addr))
	}
	t.Cleanup(func() {
		for _, ws := range wsConns {
			if ws != nil {
				ws.CloseNow()
			}
		}
	})

	for i, ws := range wsConns {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req := map[string]any{
			"jsonrpc": "2.0",
			"id":      fmt.Sprintf("req-%d", i),
			"method":  "echo.v1.Echo/Ping",
			"params":  map[string]any{"client": i},
		}
		payload, _ := json.Marshal(req)
		err := ws.Write(ctx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			t.Fatalf("write request %d: %v", i, err)
		}
	}

	for i := 0; i < 2; i++ {
		if err := wsConns[i].Close(websocket.StatusNormalClosure, "chaos drop"); err != nil {
			t.Fatalf("close dropped client %d: %v", i, err)
		}
		wsConns[i] = nil
	}

	for i := 2; i < clients; i++ {
		resp := readWSJSONMap(t, wsConns[i], 2*time.Second)
		if _, ok := resp["result"].(map[string]any); !ok {
			t.Fatalf("client %d missing result after chaos: %#v", i, resp)
		}
	}

	probeClient := connectHolonRPCClient(t, addr)
	probeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := probeClient.Invoke(probeCtx, "rpc.heartbeat", nil); err != nil {
		t.Fatalf("server was not alive after chaos: %v", err)
	}
}

func TestHolonRPCGracefulShutdown(t *testing.T) {
	started := make(chan struct{}, 1)
	server, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("slow.v1.Work/Do", func(_ context.Context, _ map[string]any) (map[string]any, error) {
			started <- struct{}{}
			time.Sleep(500 * time.Millisecond)
			return map[string]any{"ok": true}, nil
		})
	})

	client := holonrpc.NewClient()
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := client.Connect(connectCtx, addr); err != nil {
		connectCancel()
		t.Fatalf("connect client: %v", err)
	}
	connectCancel()
	defer client.Close()

	type invokeResult struct {
		resp map[string]any
		err  error
	}
	invokeCh := make(chan invokeResult, 1)
	go func() {
		callCtx, callCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer callCancel()
		resp, err := client.Invoke(callCtx, "slow.v1.Work/Do", map[string]any{})
		invokeCh <- invokeResult{resp: resp, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	closeErr := server.Close(closeCtx)
	closeCancel()
	if closeErr != nil {
		t.Fatalf("server close failed: %v", closeErr)
	}

	select {
	case out := <-invokeCh:
		if out.err == nil {
			if got, _ := out.resp["ok"].(bool); !got {
				t.Fatalf("successful response mismatch: %#v", out.resp)
			}
			return
		}

		var rpcErr *holonrpc.ResponseError
		if errors.As(out.err, &rpcErr) && rpcErr.Code == 14 {
			return
		}
		if strings.Contains(out.err.Error(), "connection closed") {
			return
		}
		t.Fatalf("unexpected invoke error during shutdown: %v", out.err)
	case <-time.After(3 * time.Second):
		t.Fatal("invoke did not finish during shutdown")
	}
}

func TestHolonRPCReconnect(t *testing.T) {
	bindURL := reserveHolonRPCBindURL(t)

	startEchoServer := func() *holonrpc.Server {
		server := holonrpc.NewServer(bindURL)
		server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})

		if _, err := server.Start(); err != nil {
			t.Fatalf("start server: %v", err)
		}
		return server
	}

	server := startEchoServer()
	t.Cleanup(func() {
		if server == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Close(ctx)
	})

	client := holonrpc.NewClient()
	t.Cleanup(func() {
		_ = client.Close()
	})

	reconnectCtx, reconnectCancel := context.WithCancel(context.Background())
	t.Cleanup(reconnectCancel)

	if err := client.ConnectWithReconnect(reconnectCtx, server.Address()); err != nil {
		t.Fatalf("connect with reconnect: %v", err)
	}

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	resp, err := client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": "before"})
	callCancel()
	if err != nil {
		t.Fatalf("initial echo invoke: %v", err)
	}
	if got, _ := resp["message"].(string); got != "before" {
		t.Fatalf("initial echo mismatch: got %q want %q", got, "before")
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := server.Close(closeCtx); err != nil {
		closeCancel()
		t.Fatalf("close server: %v", err)
	}
	closeCancel()

	disconnectDeadline := time.Now().Add(2 * time.Second)
	for client.Connected() && time.Now().Before(disconnectDeadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if client.Connected() {
		t.Fatal("client did not detect disconnect")
	}

	downCtx, downCancel := context.WithTimeout(context.Background(), time.Second)
	_, err = client.Invoke(downCtx, "echo.v1.Echo/Ping", map[string]any{"message": "down"})
	downCancel()
	requireRPCErrorCode(t, err, 14)

	server = startEchoServer()

	reconnectDeadline := time.Now().Add(5 * time.Second)
	for !client.Connected() && time.Now().Before(reconnectDeadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !client.Connected() {
		t.Fatal("client did not reconnect within 5s")
	}

	retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
	resp, err = client.Invoke(retryCtx, "echo.v1.Echo/Ping", map[string]any{"message": "after"})
	retryCancel()
	if err != nil {
		t.Fatalf("echo after reconnect: %v", err)
	}
	if got, _ := resp["message"].(string); got != "after" {
		t.Fatalf("post-reconnect echo mismatch: got %q want %q", got, "after")
	}
}

func TestHolonRPC_MessageOrderingByRequestID(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			if rawDelay, ok := params["delay_ms"].(float64); ok && rawDelay > 0 {
				time.Sleep(time.Duration(rawDelay) * time.Millisecond)
			}
			return map[string]any{"id": params["id"]}, nil
		})
	})

	client := connectHolonRPCClient(t, addr)

	const calls = 20
	errCh := make(chan error, calls)
	var wg sync.WaitGroup
	wg.Add(calls)

	for i := 0; i < calls; i++ {
		i := i
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := client.Invoke(ctx, "echo.v1.Echo/Ping", map[string]any{
				"id":       i,
				"delay_ms": float64((calls - i) * 5),
			})
			if err != nil {
				errCh <- err
				return
			}
			got, ok := resp["id"].(float64)
			if !ok {
				errCh <- fmt.Errorf("missing id field in response: %#v", resp)
				return
			}
			if int(got) != i {
				errCh <- fmt.Errorf("response id mismatch: got %d want %d", int(got), i)
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

func TestHolonRPC_LargePayloadNearLimit(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	client := connectHolonRPCClient(t, addr)
	nearLimit := strings.Repeat("x", 900*1024)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := client.Invoke(ctx, "echo.v1.Echo/Ping", map[string]any{"message": nearLimit})
	if err != nil {
		t.Fatalf("invoke near-limit payload: %v", err)
	}
	got, _ := resp["message"].(string)
	if got != nearLimit {
		t.Fatalf("near-limit payload mismatch: got len %d want len %d", len(got), len(nearLimit))
	}
}

func TestHolonRPC_OversizedMessageRejection(t *testing.T) {
	_, addr := startHolonRPCServer(t, func(s *holonrpc.Server) {
		s.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
			return params, nil
		})
	})

	ws := dialRawHolonRPC(t, addr)
	defer ws.CloseNow()

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
		_, _, readErr := ws.Read(readCtx)
		readErrCh <- readErr
	}()

	writeCtx, writeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	writer, err := ws.Writer(writeCtx, websocket.MessageText)
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

	// Server should remain healthy after rejecting oversize payloads.
	probeClient := connectHolonRPCClient(t, addr)
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer probeCancel()
	if _, err := probeClient.Invoke(probeCtx, "rpc.heartbeat", nil); err != nil {
		t.Fatalf("heartbeat after oversize rejection failed: %v", err)
	}
}

func reserveHolonRPCBindURL(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := lis.Addr().(*net.TCPAddr)
	if err := lis.Close(); err != nil {
		t.Fatalf("release reserved port: %v", err)
	}

	return fmt.Sprintf("ws://127.0.0.1:%d/rpc", addr.Port)
}

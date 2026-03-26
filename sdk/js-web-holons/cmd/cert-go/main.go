package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

const (
	defaultMode        = "echo"
	defaultTimeoutMS   = 5000
	defaultMessage     = "cert"
	defaultClients     = 50
	defaultCycles      = 100
	defaultDropCount   = 2
	defaultPayloadSize = 2 * 1024 * 1024
)

type options struct {
	mode         string
	uri          string
	message      string
	timeoutMS    int
	clients      int
	cycles       int
	dropCount    int
	payloadBytes int
}

type outcome struct {
	Status string         `json:"status"`
	Mode   string         `json:"mode"`
	Data   map[string]any `json:"data,omitempty"`
}

func main() {
	opts, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.timeoutMS)*time.Millisecond)
	defer cancel()

	var data map[string]any
	switch opts.mode {
	case "echo":
		data, err = runEcho(ctx, opts)
	case "bidirectional":
		data, err = runBidirectional(ctx, opts)
	case "concurrent-load":
		data, err = runConcurrentLoad(ctx, opts)
	case "resource-cleanup":
		data, err = runResourceCleanup(ctx, opts)
	case "timeout":
		data, err = runTimeout(ctx, opts)
	case "chaos":
		data, err = runChaos(ctx, opts)
	case "oversize":
		data, err = runOversize(ctx, opts)
	default:
		err = fmt.Errorf("unsupported --mode %q", opts.mode)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(outcome{Status: "pass", Mode: opts.mode, Data: data}); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() (options, error) {
	mode := flag.String("mode", defaultMode, "check mode: echo|bidirectional|concurrent-load|resource-cleanup|timeout|chaos|oversize")
	message := flag.String("message", defaultMessage, "message payload")
	timeoutMS := flag.Int("timeout-ms", defaultTimeoutMS, "per-check timeout in milliseconds")
	clients := flag.Int("clients", defaultClients, "number of concurrent clients")
	cycles := flag.Int("cycles", defaultCycles, "number of connect/disconnect cycles")
	dropCount := flag.Int("drop-count", defaultDropCount, "number of abruptly dropped clients")
	payloadBytes := flag.Int("payload-bytes", defaultPayloadSize, "payload size in bytes for oversize checks")
	flag.Parse()

	if flag.NArg() != 1 {
		return options{}, fmt.Errorf("usage: cert-go --mode <mode> [flags] ws://host:port/rpc")
	}
	if *timeoutMS <= 0 {
		return options{}, fmt.Errorf("--timeout-ms must be a positive integer")
	}
	if *clients <= 0 {
		return options{}, fmt.Errorf("--clients must be a positive integer")
	}
	if *cycles <= 0 {
		return options{}, fmt.Errorf("--cycles must be a positive integer")
	}
	if *dropCount < 0 {
		return options{}, fmt.Errorf("--drop-count must be >= 0")
	}
	if *payloadBytes <= 0 {
		return options{}, fmt.Errorf("--payload-bytes must be a positive integer")
	}

	return options{
		mode:         *mode,
		uri:          flag.Arg(0),
		message:      *message,
		timeoutMS:    *timeoutMS,
		clients:      *clients,
		cycles:       *cycles,
		dropCount:    *dropCount,
		payloadBytes: *payloadBytes,
	}, nil
}

func runEcho(ctx context.Context, opts options) (map[string]any, error) {
	client, err := connectClient(ctx, opts.uri)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
	defer cancel()

	resp, err := client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": opts.message})
	if err != nil {
		return nil, fmt.Errorf("echo invoke failed: %w", err)
	}
	if got, _ := resp["message"].(string); got != opts.message {
		return nil, fmt.Errorf("echo mismatch: got %q want %q", got, opts.message)
	}
	return map[string]any{"message": opts.message}, nil
}

func runBidirectional(ctx context.Context, opts options) (map[string]any, error) {
	called := make(chan string, 1)

	client := holonrpc.NewClient()
	client.Register("test.v1.Client/Pong", func(_ context.Context, params map[string]any) (map[string]any, error) {
		message, _ := params["message"].(string)
		select {
		case called <- message:
		default:
		}
		return map[string]any{"message": fmt.Sprintf("pong:%s", message)}, nil
	})
	defer client.Close()

	connectCtx, connectCancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
	defer connectCancel()
	if err := client.Connect(connectCtx, opts.uri); err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}

	callCtx, callCancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
	defer callCancel()
	resp, err := client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": opts.message})
	if err != nil {
		return nil, fmt.Errorf("echo invoke failed: %w", err)
	}
	if got, _ := resp["message"].(string); got != opts.message {
		return nil, fmt.Errorf("echo mismatch: got %q want %q", got, opts.message)
	}

	select {
	case got := <-called:
		return map[string]any{"server_call": got}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for server-initiated call")
	}
}

func runConcurrentLoad(ctx context.Context, opts options) (map[string]any, error) {
	const method = "echo.v1.Echo/Ping"

	var wg sync.WaitGroup
	errCh := make(chan error, opts.clients)

	for i := 0; i < opts.clients; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()

			clientCtx, cancelClient := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
			defer cancelClient()

			client, err := connectClient(clientCtx, opts.uri)
			if err != nil {
				errCh <- fmt.Errorf("client %d connect failed: %w", i, err)
				return
			}
			defer client.Close()

			message := fmt.Sprintf("%s-%d", opts.message, i)
			invokeCtx, cancelInvoke := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
			defer cancelInvoke()

			resp, err := client.Invoke(invokeCtx, method, map[string]any{"message": message})
			if err != nil {
				errCh <- fmt.Errorf("client %d invoke failed: %w", i, err)
				return
			}
			if got, _ := resp["message"].(string); got != message {
				errCh <- fmt.Errorf("client %d mismatch: got %q want %q", i, got, message)
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return map[string]any{"clients": opts.clients}, nil
}

func runResourceCleanup(ctx context.Context, opts options) (map[string]any, error) {
	runtime.GC()
	time.Sleep(150 * time.Millisecond)
	before := runtime.NumGoroutine()

	for i := 0; i < opts.cycles; i++ {
		clientCtx, cancelClient := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
		client, err := connectClient(clientCtx, opts.uri)
		cancelClient()
		if err != nil {
			return nil, fmt.Errorf("cycle %d connect failed: %w", i, err)
		}

		invokeCtx, cancelInvoke := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
		_, err = client.Invoke(invokeCtx, "echo.v1.Echo/Ping", map[string]any{"message": fmt.Sprintf("c-%d", i)})
		cancelInvoke()
		closeErr := client.Close()
		if err != nil {
			return nil, fmt.Errorf("cycle %d invoke failed: %w", i, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("cycle %d close failed: %w", i, closeErr)
		}
	}

	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	after := runtime.NumGoroutine()
	delta := after - before
	if delta < 0 {
		delta = -delta
	}
	if delta > 5 {
		return nil, fmt.Errorf("goroutine delta %d exceeds threshold 5 (before=%d after=%d)", delta, before, after)
	}

	return map[string]any{
		"cycles":           opts.cycles,
		"goroutine_before": before,
		"goroutine_after":  after,
		"goroutine_delta":  delta,
	}, nil
}

func runTimeout(ctx context.Context, opts options) (map[string]any, error) {
	client, err := connectClient(ctx, opts.uri)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
	defer cancel()
	_, err = client.Invoke(callCtx, "echo.v1.Echo/Ping", map[string]any{"message": opts.message})
	if err == nil {
		return nil, errors.New("expected timeout error, got nil")
	}

	var rpcErr *holonrpc.ResponseError
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &rpcErr) && rpcErr.Code == 4) {
		return map[string]any{"error": err.Error()}, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return map[string]any{"error": err.Error()}, nil
	}
	return nil, fmt.Errorf("expected timeout-related error, got: %v", err)
}

func runChaos(ctx context.Context, opts options) (map[string]any, error) {
	if opts.dropCount > opts.clients {
		return nil, fmt.Errorf("--drop-count (%d) cannot exceed --clients (%d)", opts.dropCount, opts.clients)
	}

	conns := make([]*websocket.Conn, 0, opts.clients)
	for i := 0; i < opts.clients; i++ {
		ws, _, err := websocket.Dial(ctx, opts.uri, &websocket.DialOptions{Subprotocols: []string{"holon-rpc"}})
		if err != nil {
			for _, c := range conns {
				_ = c.Close(websocket.StatusInternalError, "cleanup")
			}
			return nil, fmt.Errorf("dial client %d failed: %w", i, err)
		}
		conns = append(conns, ws)
	}
	defer func() {
		for _, ws := range conns {
			if ws != nil {
				ws.CloseNow()
			}
		}
	}()

	for i, ws := range conns {
		req := map[string]any{
			"jsonrpc": "2.0",
			"id":      fmt.Sprintf("req-%d", i),
			"method":  "echo.v1.Echo/Ping",
			"params":  map[string]any{"message": fmt.Sprintf("m-%d", i)},
		}
		data, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		writeCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
		err = ws.Write(writeCtx, websocket.MessageText, data)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("write request %d failed: %w", i, err)
		}
	}

	for i := 0; i < opts.dropCount; i++ {
		conns[i].CloseNow()
		conns[i] = nil
	}

	for i := opts.dropCount; i < opts.clients; i++ {
		resp, err := readJSONMap(ctx, conns[i], time.Duration(opts.timeoutMS)*time.Millisecond)
		if err != nil {
			return nil, fmt.Errorf("read response %d failed: %w", i, err)
		}
		if _, ok := resp["result"].(map[string]any); !ok {
			return nil, fmt.Errorf("response %d missing result: %#v", i, resp)
		}
	}

	if err := probeHeartbeat(ctx, opts.uri, time.Duration(opts.timeoutMS)*time.Millisecond); err != nil {
		return nil, fmt.Errorf("heartbeat probe failed after chaos: %w", err)
	}

	return map[string]any{"clients": opts.clients, "dropped": opts.dropCount}, nil
}

func runOversize(ctx context.Context, opts options) (map[string]any, error) {
	ws, _, err := websocket.Dial(ctx, opts.uri, &websocket.DialOptions{Subprotocols: []string{"holon-rpc"}})
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}
	defer ws.CloseNow()

	oversized := strings.Repeat("a", opts.payloadBytes)
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "big-1",
		"method":  "echo.v1.Echo/Ping",
		"params":  map[string]string{"message": oversized},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal oversized request: %w", err)
	}

	readErrCh := make(chan error, 1)
	go func() {
		readCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
		defer cancel()
		_, _, readErr := ws.Read(readCtx)
		readErrCh <- readErr
	}()

	writeCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.timeoutMS)*time.Millisecond)
	writer, err := ws.Writer(writeCtx, websocket.MessageText)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open writer: %w", err)
	}

	const chunkSize = 16 * 1024
	for off := 0; off < len(req); off += chunkSize {
		end := off + chunkSize
		if end > len(req) {
			end = len(req)
		}
		if _, err := writer.Write(req[off:end]); err != nil {
			break
		}
	}
	_ = writer.Close()
	cancel()

	var readErr error
	select {
	case readErr = <-readErrCh:
	case <-ctx.Done():
		return nil, errors.New("timeout waiting for oversize close frame")
	}

	status := websocket.CloseStatus(readErr)
	if status != websocket.StatusMessageTooBig {
		return nil, fmt.Errorf("close status = %v, want %v (err=%v)", status, websocket.StatusMessageTooBig, readErr)
	}

	if err := probeHeartbeat(ctx, opts.uri, time.Duration(opts.timeoutMS)*time.Millisecond); err != nil {
		return nil, fmt.Errorf("heartbeat probe failed after oversize rejection: %w", err)
	}

	return map[string]any{"close_status": int(status), "payload_bytes": opts.payloadBytes}, nil
}

func connectClient(ctx context.Context, uri string) (*holonrpc.Client, error) {
	client := holonrpc.NewClient()
	if err := client.Connect(ctx, uri); err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	return client, nil
}

func probeHeartbeat(parent context.Context, uri string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	client, err := connectClient(ctx, uri)
	if err != nil {
		return err
	}
	defer client.Close()

	callCtx, callCancel := context.WithTimeout(parent, timeout)
	defer callCancel()
	_, err = client.Invoke(callCtx, "rpc.heartbeat", nil)
	if err != nil {
		return err
	}
	return nil
}

func readJSONMap(parent context.Context, ws *websocket.Conn, timeout time.Duration) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	_, data, err := ws.Read(ctx)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

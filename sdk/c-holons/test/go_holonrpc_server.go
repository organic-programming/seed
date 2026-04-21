package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

const (
	defaultBindURL = "ws://127.0.0.1:0/rpc"
	defaultSDK     = "c-holons"
	defaultVersion = "0.1.0"
	fanoutMethod   = "*.Echo/Ping"
	peerPingMethod = "echo.v1.Echo/Ping"
)

type options struct {
	bindURL string
	sdk     string
	version string
	once    bool
}

type serverMode int

const (
	serverModeWebSocket serverMode = iota + 1
	serverModeHTTP
)

func main() {
	opts, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	mode, bindURL, err := normalizeBindURL(opts.bindURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	opts.bindURL = bindURL

	switch mode {
	case serverModeHTTP:
		runHTTPServer(opts)
	default:
		runWebSocketServer(opts)
	}
}

func runWebSocketServer(opts options) {
	server := holonrpc.NewServer(opts.bindURL)

	handled := make(chan struct{})
	var handledOnce sync.Once
	markHandled := func() {
		handledOnce.Do(func() {
			close(handled)
		})
	}

	var latestClientMu sync.RWMutex
	latestClientID := ""

	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		markHandled()
		out := make(map[string]any, len(params)+2)
		for k, v := range params {
			out[k] = v
		}
		out["sdk"] = opts.sdk
		out["version"] = opts.version
		return out, nil
	})

	server.Register("echo.v1.Echo/CallClient", func(ctx context.Context, params map[string]any) (map[string]any, error) {
		markHandled()
		latestClientMu.RLock()
		clientID := latestClientID
		latestClientMu.RUnlock()
		if clientID == "" {
			return nil, &holonrpc.ResponseError{
				Code:    14,
				Message: "no connected client",
			}
		}

		name := "c"
		if rawName, ok := params["name"].(string); ok {
			trimmed := strings.TrimSpace(rawName)
			if trimmed != "" {
				name = trimmed
			}
		}

		return server.Invoke(ctx, clientID, "client.v1.Client/Hello", map[string]any{"name": name})
	})

	server.Register(fanoutMethod, func(ctx context.Context, params map[string]any) (map[string]any, error) {
		markHandled()
		clientIDs := server.ClientIDs()
		if len(clientIDs) == 0 {
			return nil, &holonrpc.ResponseError{
				Code:    5,
				Message: "no connected peers",
			}
		}

		results := make([]any, 0, len(clientIDs))
		for _, clientID := range clientIDs {
			reply, invokeErr := server.Invoke(ctx, clientID, peerPingMethod, params)
			if invokeErr != nil {
				if errors.Is(invokeErr, context.Canceled) || errors.Is(invokeErr, context.DeadlineExceeded) {
					return nil, invokeErr
				}

				var rpcErr *holonrpc.ResponseError
				if errors.As(invokeErr, &rpcErr) {
					// Method-not-found means this peer does not implement the target method.
					if rpcErr.Code == -32601 || rpcErr.Code == 5 {
						continue
					}
					results = append(results, map[string]any{
						"peer": clientID,
						"error": map[string]any{
							"code":    rpcErr.Code,
							"message": rpcErr.Message,
						},
					})
					continue
				}

				results = append(results, map[string]any{
					"peer": clientID,
					"error": map[string]any{
						"code":    13,
						"message": invokeErr.Error(),
					},
				})
				continue
			}

			results = append(results, map[string]any{
				"peer":   clientID,
				"result": reply,
			})
		}

		if len(results) == 0 {
			return nil, &holonrpc.ResponseError{
				Code:    5,
				Message: "no peer implements echo.v1.Echo/Ping",
			}
		}

		// `value` aligns with go-holons decodeResult wrapping for non-object JSON results.
		return map[string]any{
			"value": results,
		}, nil
	})

	addr, err := server.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(addr)

	if opts.once {
		select {
		case <-handled:
		case <-time.After(10 * time.Second):
		}

		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()
		if err := server.Close(closeCtx); err != nil {
			fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	waitCtx, waitCancel := context.WithCancel(context.Background())
	defer waitCancel()

	go func() {
		for {
			id, waitErr := server.WaitForClient(waitCtx)
			if waitErr != nil {
				return
			}
			latestClientMu.Lock()
			latestClientID = id
			latestClientMu.Unlock()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	signal.Stop(sigCh)
	waitCancel()

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()
	if err := server.Close(closeCtx); err != nil {
		fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
		os.Exit(1)
	}
}

func runHTTPServer(opts options) {
	server := holonrpc.NewHTTPServer(opts.bindURL)

	handled := make(chan struct{})
	var handledOnce sync.Once
	markHandled := func() {
		handledOnce.Do(func() {
			close(handled)
		})
	}

	server.Register("rpc.heartbeat", func(_ context.Context, _ map[string]any) (map[string]any, error) {
		markHandled()
		return map[string]any{}, nil
	})

	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		markHandled()
		out := make(map[string]any, len(params)+2)
		for k, v := range params {
			out[k] = v
		}
		out["sdk"] = opts.sdk
		out["version"] = opts.version
		return out, nil
	})

	server.RegisterStream("build.v1.Build/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
		markHandled()

		project := "unknown"
		if rawProject, ok := params["project"].(string); ok {
			trimmed := strings.TrimSpace(rawProject)
			if trimmed != "" {
				project = trimmed
			}
		}

		if err := send(map[string]any{"status": "building", "project": project, "progress": 42.0}); err != nil {
			return err
		}
		return send(map[string]any{"status": "done", "project": project, "progress": 100.0})
	})

	addr, err := server.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(addr)

	if opts.once {
		select {
		case <-handled:
		case <-time.After(10 * time.Second):
		}

		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()
		if err := server.Close(closeCtx); err != nil {
			fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	signal.Stop(sigCh)

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()
	if err := server.Close(closeCtx); err != nil {
		fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() (options, error) {
	sdk := flag.String("sdk", defaultSDK, "sdk name returned in echo responses")
	version := flag.String("version", defaultVersion, "sdk version returned in echo responses")
	once := flag.Bool("once", false, "exit after handling first client request")
	flag.Parse()

	if flag.NArg() > 1 {
		return options{}, fmt.Errorf("usage: go_holonrpc_server.go [ws://host:port/rpc|http://host:port/api/v1/rpc|rest+sse://host:port/api/v1/rpc] [--sdk <name>] [--version <version>]")
	}

	bindURL := defaultBindURL
	if flag.NArg() == 1 {
		bindURL = flag.Arg(0)
	}

	return options{
		bindURL: bindURL,
		sdk:     *sdk,
		version: *version,
		once:    *once,
	}, nil
}

func normalizeBindURL(raw string) (serverMode, string, error) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "ws://"):
		return serverModeWebSocket, trimmed, nil
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		return serverModeHTTP, trimmed, nil
	case strings.HasPrefix(trimmed, "rest+sse://"):
		return serverModeHTTP, "http://" + strings.TrimPrefix(trimmed, "rest+sse://"), nil
	default:
		return 0, "", fmt.Errorf("unsupported Holon-RPC server transport: %s", raw)
	}
}

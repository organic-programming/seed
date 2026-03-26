package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

const (
	defaultListen  = "ws://127.0.0.1:0/rpc"
	defaultSDK     = "go-holons"
	defaultVersion = "0.1.0"
)

type options struct {
	listen  string
	sdk     string
	version string
}

func main() {
	opts, err := parseFlags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	server := holonrpc.NewServer(opts.listen)
	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		out := make(map[string]any, len(params)+2)
		for k, v := range params {
			out[k] = v
		}
		out["sdk"] = opts.sdk
		out["version"] = opts.version
		return out, nil
	})

	addr, err := server.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	signal.Stop(sigCh)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "close failed: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() (options, error) {
	listen := flag.String("listen", defaultListen, "WebSocket listen URL")
	sdk := flag.String("sdk", defaultSDK, "sdk name returned in echo responses")
	version := flag.String("version", defaultVersion, "sdk version returned in echo responses")
	flag.Parse()

	if flag.NArg() > 0 {
		return options{}, fmt.Errorf("usage: holonrpc-server-go [--listen ws://127.0.0.1:0/rpc] [--sdk name] [--version value]")
	}

	return options{
		listen:  *listen,
		sdk:     *sdk,
		version: *version,
	}, nil
}

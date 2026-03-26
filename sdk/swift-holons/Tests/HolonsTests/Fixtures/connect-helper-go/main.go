package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
)

type helperOptions struct {
	listen string
	slug   string
	motto  string
}

type holonMetaServer struct {
	holonsv1.UnimplementedHolonMetaServer
	slug  string
	motto string
}

func (s holonMetaServer) Describe(_ context.Context, _ *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				GivenName: s.slug,
				Motto:     s.motto,
			},
		},
	}, nil
}

func main() {
	os.Args = stripServeCommand(os.Args)

	opts := parseFlags()

	listener, err := transport.Listen(opts.listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	server := grpc.NewServer()
	holonsv1.RegisterHolonMetaServer(server, holonMetaServer{
		slug:  opts.slug,
		motto: opts.motto,
	})

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	if !isStdio(opts.listen) {
		fmt.Println(publicURI(opts.listen, listener.Addr().String()))
	}

	if isStdio(opts.listen) {
		if err := <-serveDone; err != nil && !isBenignServeError(err) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)

	select {
	case <-signals:
		shutdown(server)
	case err := <-serveDone:
		if err != nil && !isBenignServeError(err) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := <-serveDone; err != nil && !isBenignServeError(err) {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func stripServeCommand(args []string) []string {
	if len(args) <= 1 {
		return args
	}

	normalized := make([]string, 0, len(args))
	normalized = append(normalized, args[0])

	removed := false
	for _, arg := range args[1:] {
		if !removed && arg == "serve" {
			removed = true
			continue
		}
		normalized = append(normalized, arg)
	}

	return normalized
}

func parseFlags() helperOptions {
	listen := flag.String("listen", "tcp://127.0.0.1:0", "listen URI")
	slug := flag.String("slug", "connect-helper", "holon slug")
	motto := flag.String("motto", "swift connect tests", "holon motto")
	flag.Parse()

	return helperOptions{
		listen: *listen,
		slug:   *slug,
		motto:  *motto,
	}
}

func shutdown(server *grpc.Server) {
	done := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		server.Stop()
	}
}

func publicURI(listenURI, addr string) string {
	if !strings.HasPrefix(listenURI, "tcp://") {
		return listenURI
	}

	hostPort := strings.TrimPrefix(listenURI, "tcp://")
	host := "127.0.0.1"
	if idx := strings.LastIndex(hostPort, ":"); idx >= 0 {
		candidate := hostPort[:idx]
		switch candidate {
		case "", "0.0.0.0", "::", "[::]":
			host = "127.0.0.1"
		default:
			host = strings.Trim(candidate, "[]")
		}
	}

	port := addr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		port = addr[idx+1:]
	}

	return fmt.Sprintf("tcp://%s:%s", host, port)
}

func isStdio(uri string) bool {
	return uri == "stdio://" || uri == "stdio"
}

func isBenignServeError(err error) bool {
	if err == nil || err == grpc.ErrServerStopped {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection")
}

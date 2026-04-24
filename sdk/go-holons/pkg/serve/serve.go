// Package serve provides a standard implementation of the `serve` command
// for Go holons (Constitution Article 11).
//
// Usage in a holon's main.go:
//
//	case "serve":
//	    options := serve.ParseOptions(os.Args[2:])
//	    serve.RunCLIOptions(options, func(s *grpc.Server) {
//	        pb.RegisterMyServiceServer(s, &myServer{})
//	    })
package serve

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/holonrpc"
	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RegisterFunc is called to register gRPC services on the server.
type RegisterFunc func(s *grpc.Server)

// CLIOptions contains standard serve-command flags.
type CLIOptions struct {
	ListenURI  string
	ListenURIs []string
	Reflect    bool
}

// ParseFlags extracts --listen and --port from command-line args.
// Returns a transport URI. If neither flag is present, returns the default.
func ParseFlags(args []string) string {
	return ParseOptions(args).ListenURI
}

// ParseOptions extracts standard serve-command flags.
// Reflection is disabled by default and enabled with --reflect.
func ParseOptions(args []string) CLIOptions {
	listenURIs := make([]string, 0, 1)
	reflectEnabled := false

	for i, arg := range args {
		if arg == "--listen" && i+1 < len(args) {
			listenURIs = append(listenURIs, args[i+1])
		}
		// Backward compatibility: --port N → tcp://:N
		if arg == "--port" && i+1 < len(args) {
			listenURIs = []string{"tcp://:" + args[i+1]}
		}
		if arg == "--reflect" {
			reflectEnabled = true
		}
	}

	if len(listenURIs) == 0 {
		listenURIs = []string{transport.DefaultURI}
	}

	options := CLIOptions{
		ListenURI:  listenURIs[0],
		ListenURIs: append([]string(nil), listenURIs...),
		Reflect:    reflectEnabled,
	}
	return options
}

// RunCLIOptions starts a holon using the parsed standard serve flags,
// including repeated --listen values when present.
func RunCLIOptions(options CLIOptions, register RegisterFunc) error {
	listenURIs := options.ListenURIs
	if len(listenURIs) == 0 {
		listenURIs = []string{options.ListenURI}
	}
	if len(listenURIs) == 0 || strings.TrimSpace(listenURIs[0]) == "" {
		listenURIs = []string{transport.DefaultURI}
	}
	return RunWithOptions(listenURIs[0], register, options.Reflect, listenURIs[1:]...)
}

// Run starts gRPC and/or HTTP+SSE servers on the supplied transport URIs with
// reflection disabled by default. It blocks until SIGTERM/SIGINT is received,
// then shuts down gracefully. If in-flight RPC draining exceeds 10 seconds, the
// gRPC server is force-stopped to satisfy the operational shutdown deadline.
func Run(listenURI string, register RegisterFunc, moreListenURIs ...string) error {
	return RunWithOptions(listenURI, register, false, moreListenURIs...)
}

// RunWithOptions is like Run but lets you control reflection. Repeated listen
// URIs allow one holon process to expose multiple transports at once.
func RunWithOptions(listenURI string, register RegisterFunc, reflect bool, moreListenURIs ...string) (runErr error) {
	listenURIs := normalizeListenURIs(listenURI, moreListenURIs)

	// Observability: fail-fast on unknown OP_OBS tokens, then install
	// the singleton (safe no-op when OP_OBS is empty).
	if err := observability.CheckEnv(); err != nil {
		return fmt.Errorf("observability env: %w", err)
	}
	obs := observability.FromEnv(observability.Config{})

	grpcOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(observability.UnaryServerInterceptor()),
	}
	s := grpc.NewServer(grpcOptions...)
	if register != nil {
		register(s)
	}
	if err := autoRegisterHolonMeta(s); err != nil {
		log.Printf("HolonMeta registration failed: %v", err)
		return fmt.Errorf("register HolonMeta: %w", err)
	}
	observability.Register(s)
	if reflect {
		reflection.Register(s)
	}

	// Prometheus /metrics HTTP endpoint when OP_OBS=prom. Bound on an
	// ephemeral port; per the binding rule in OBSERVABILITY.md
	// §Transport Constraints, HTTP-capable transports would reuse the
	// gRPC listener — a refinement we can add later without changing
	// the public surface.
	var promServer *observability.PromServer
	var metricsAddr string
	if obs.Enabled(observability.FamilyProm) {
		addr := os.Getenv("OP_PROM_ADDR")
		if addr == "" {
			addr = ":0"
		}
		ps := observability.NewPromServer(addr)
		if bound, err := ps.Start(); err != nil {
			log.Printf("warning: prom HTTP bind failed: %v", err)
		} else {
			log.Printf("Prometheus /metrics listening on %s", bound)
			promServer = ps
			metricsAddr = bound
		}
	}

	// Disk writers under <OP_RUN_DIR>/<slug>/<uid>/ when OP_RUN_DIR is
	// set (typically by `op run`). OP_RUN_DIR itself is the registry
	// root; observability.FromEnv derives the per-instance path.
	runDir := obs.RunDir()
	if runDir != "" {
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			log.Printf("warning: observability run directory: %v", err)
		}
		if err := observability.EnableDiskWriters(runDir); err != nil {
			log.Printf("warning: observability disk writers: %v", err)
		}
	}

	type grpcEndpoint struct {
		uri string
		lis net.Listener
	}

	grpcEndpoints := make([]grpcEndpoint, 0, len(listenURIs))
	httpServers := make([]*holonrpc.HTTPServer, 0, len(listenURIs))
	var bridgeConn *grpc.ClientConn
	var internalBridge net.Listener

	defer func() {
		if runErr == nil {
			return
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, server := range httpServers {
			_ = server.Close(shutdownCtx)
		}
		if bridgeConn != nil {
			_ = bridgeConn.Close()
		}
		if internalBridge != nil {
			_ = internalBridge.Close()
		}
		s.Stop()
		for _, endpoint := range grpcEndpoints {
			_ = endpoint.lis.Close()
		}
	}()

	for _, uri := range listenURIs {
		if isHTTPListenURI(uri) {
			httpServers = append(httpServers, holonrpc.NewHTTPServer(uri))
			continue
		}

		lis, err := transport.Listen(uri)
		if err != nil {
			return err
		}
		grpcEndpoints = append(grpcEndpoints, grpcEndpoint{uri: uri, lis: lis})
	}

	serveErrCh := make(chan error, len(grpcEndpoints)+1)
	var serveWG sync.WaitGroup

	if len(httpServers) > 0 {
		internalBridge, runErr = transport.Listen("tcp://127.0.0.1:0")
		if runErr != nil {
			return fmt.Errorf("listen internal HTTP bridge: %w", runErr)
		}
		serveWG.Add(1)
		go func() {
			defer serveWG.Done()
			if err := s.Serve(internalBridge); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				select {
				case serveErrCh <- fmt.Errorf("serve internal HTTP bridge: %w", err):
				default:
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		bridgeConn, runErr = grpcclient.Dial(ctx, internalBridge.Addr().String())
		cancel()
		if runErr != nil {
			_ = internalBridge.Close()
			return fmt.Errorf("dial internal HTTP bridge: %w", runErr)
		}

		bridge, err := newHTTPBridge(s, bridgeConn, describe.StaticResponse())
		if err != nil {
			return err
		}
		for _, server := range httpServers {
			bridge.apply(server)
			address, err := server.Start()
			if err != nil {
				return fmt.Errorf("start HTTP+SSE server %s: %w", server.Address(), err)
			}
			log.Printf("HTTP+SSE server listening on %s", address)
		}
	}

	mode := "reflection ON"
	if !reflect {
		mode = "reflection OFF"
	}
	for _, endpoint := range grpcEndpoints {
		endpoint := endpoint
		serveWG.Add(1)
		go func() {
			defer serveWG.Done()
			if err := s.Serve(endpoint.lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
				select {
				case serveErrCh <- fmt.Errorf("serve %s: %w", endpoint.uri, err):
				default:
				}
			}
		}()
		log.Printf("gRPC server listening on %s (%s)", advertisedURI(endpoint.uri, endpoint.lis.Addr()), mode)
	}

	// Announce INSTANCE_READY as soon as the first listener is bound,
	// even when events are disabled the call is a no-op.
	if len(grpcEndpoints) > 0 {
		firstURI := advertisedURI(grpcEndpoints[0].uri, grpcEndpoints[0].lis.Addr())
		observability.EmitReady(context.Background(), firstURI)
		// Write meta.json for `op ps` / HolonInstance.List when we
		// have a run directory. Skips silently when OP_RUN_DIR is
		// empty (manual launch without `op run`).
		if runDir != "" {
			meta := observability.MetaJSON{
				Slug:         obs.Slug(),
				UID:          obs.InstanceUID(),
				PID:          os.Getpid(),
				StartedAt:    time.Now(),
				Mode:         "persistent",
				Transport:    transport.Scheme(grpcEndpoints[0].uri),
				Address:      firstURI,
				MetricsAddr:  metricsAddr,
				OrganismUID:  obs.OrganismUID(),
				OrganismSlug: obs.OrganismSlug(),
			}
			if obs.Enabled(observability.FamilyLogs) {
				meta.LogPath = filepath.Join(runDir, "stdout.log")
			}
			if err := observability.WriteMetaJSON(runDir, meta); err != nil {
				log.Printf("warning: meta.json write failed: %v", err)
			}
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	shutdown := func(reason string) {
		log.Println(reason)

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, server := range httpServers {
			if err := server.Close(shutdownCtx); err != nil {
				log.Printf("HTTP+SSE shutdown error: %v", err)
			}
		}

		observability.EmitExited(shutdownCtx, reason)

		done := make(chan struct{})
		go func() {
			defer close(done)
			s.GracefulStop()
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Println("graceful stop timed out after 10s; forcing hard stop")
			s.Stop()
		}
		if bridgeConn != nil {
			_ = bridgeConn.Close()
		}
		if promServer != nil {
			_ = promServer.Close(shutdownCtx)
		}
	}

	select {
	case <-sigCh:
		shutdown("shutting down servers...")
		serveWG.Wait()
		return nil
	case err := <-serveErrCh:
		shutdown("shutting down servers after serve error...")
		serveWG.Wait()
		return err
	}
}

func advertisedURI(listenURI string, addr net.Addr) string {
	if addr == nil {
		return listenURI
	}

	raw := strings.TrimSpace(addr.String())
	if raw == "" {
		return listenURI
	}
	if strings.Contains(raw, "://") {
		return raw
	}

	switch addr.Network() {
	case "tcp", "tcp4", "tcp6":
		return "tcp://" + raw
	case "unix", "unixgram", "unixpacket":
		return "unix://" + raw
	default:
		switch transport.Scheme(listenURI) {
		case "tcp":
			return "tcp://" + raw
		case "unix":
			return "unix://" + raw
		default:
			return raw
		}
	}
}

func autoRegisterHolonMeta(s *grpc.Server) error {
	return describe.Register(s)
}

func normalizeListenURIs(listenURI string, moreListenURIs []string) []string {
	listenURIs := make([]string, 0, 1+len(moreListenURIs))
	for _, uri := range append([]string{listenURI}, moreListenURIs...) {
		if trimmed := strings.TrimSpace(uri); trimmed != "" {
			listenURIs = append(listenURIs, trimmed)
		}
	}
	if len(listenURIs) == 0 {
		return []string{transport.DefaultURI}
	}
	return listenURIs
}

func isHTTPListenURI(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

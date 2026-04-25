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
	"net/url"
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

// MemberRef identifies a direct organism child whose observability
// stream should be relayed into this process when the corresponding
// families are enabled.
type MemberRef struct {
	Slug    string
	UID     string
	Address string
}

// ServeOptions contains non-CLI serve runner settings.
type ServeOptions struct {
	Reflect         bool
	MemberEndpoints []MemberRef
}

type grpcEndpoint struct {
	uri string
	lis net.Listener
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
	return RunWithServeOptions(listenURI, register, ServeOptions{Reflect: reflect}, moreListenURIs...)
}

// RunWithServeOptions is like RunWithOptions and also accepts direct
// organism member endpoints for observability relay.
func RunWithServeOptions(listenURI string, register RegisterFunc, options ServeOptions, moreListenURIs ...string) (runErr error) {
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
	if options.Reflect {
		reflection.Register(s)
	}

	// Prometheus /metrics endpoint when OP_OBS=prom. HTTP-capable
	// transports share their existing listener; non-HTTP transports keep
	// the dedicated ephemeral sidecar.
	var promServer *observability.PromServer
	var metricsAddr string
	promEnabled := obs.Enabled(observability.FamilyProm)
	promSharesHTTP := promEnabled && hasHTTPListenURI(listenURIs)
	if promEnabled && !promSharesHTTP {
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

	var multilogWriter *observability.MultilogWriter
	if mw, err := observability.StartOrganismMultilog(); err != nil {
		log.Printf("warning: observability multilog: %v", err)
	} else {
		multilogWriter = mw
	}

	relayCtx, cancelRelays := context.WithCancel(context.Background())
	relays, relayConns := startMemberRelays(relayCtx, options.MemberEndpoints)
	cleanupObservability := func() {
		cancelRelays()
		for _, relay := range relays {
			relay.Stop()
		}
		for _, conn := range relayConns {
			_ = conn.Close()
		}
		if multilogWriter != nil {
			if err := multilogWriter.Stop(); err != nil {
				log.Printf("warning: observability multilog close: %v", err)
			}
		}
	}

	grpcEndpoints := make([]grpcEndpoint, 0, len(listenURIs))
	httpServers := make([]*holonrpc.HTTPServer, 0, len(listenURIs))
	var bridgeConn *grpc.ClientConn
	var internalBridge net.Listener
	httpAddresses := make([]string, 0, len(listenURIs))

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
		if promServer != nil {
			_ = promServer.Close(shutdownCtx)
		}
		cleanupObservability()
		s.Stop()
		for _, endpoint := range grpcEndpoints {
			_ = endpoint.lis.Close()
		}
	}()

	for _, uri := range listenURIs {
		if isHTTPListenURI(uri) {
			server := holonrpc.NewHTTPServer(uri)
			if promSharesHTTP {
				server.Handle("/metrics", observability.PromHandler())
			}
			httpServers = append(httpServers, server)
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
			httpAddresses = append(httpAddresses, address)
			log.Printf("HTTP+SSE server listening on %s", address)
			if promSharesHTTP {
				sharedMetricsAddr := metricsURLForHTTPAddress(address)
				log.Printf("Prometheus /metrics sharing HTTP listener on %s", sharedMetricsAddr)
				if metricsAddr == "" {
					metricsAddr = sharedMetricsAddr
				}
			}
		}
	}

	mode := "reflection ON"
	if !options.Reflect {
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
	readyAddress, readyTransport := firstReadyEndpoint(grpcEndpoints, httpAddresses)
	if readyAddress != "" {
		observability.EmitReady(context.Background(), readyAddress)
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
				Transport:    readyTransport,
				Address:      readyAddress,
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
		cleanupObservability()

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

func hasHTTPListenURI(uris []string) bool {
	for _, uri := range uris {
		if isHTTPListenURI(uri) {
			return true
		}
	}
	return false
}

func metricsURLForHTTPAddress(address string) string {
	parsed, err := url.Parse(address)
	if err != nil {
		return strings.TrimRight(address, "/") + "/metrics"
	}
	parsed.Path = "/metrics"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func firstReadyEndpoint(grpcEndpoints []grpcEndpoint, httpAddresses []string) (address, scheme string) {
	if len(grpcEndpoints) > 0 {
		endpoint := grpcEndpoints[0]
		return advertisedURI(endpoint.uri, endpoint.lis.Addr()), transport.Scheme(endpoint.uri)
	}
	if len(httpAddresses) > 0 {
		address := httpAddresses[0]
		return address, transport.Scheme(address)
	}
	return "", ""
}

func startMemberRelays(ctx context.Context, members []MemberRef) ([]*observability.Relay, []*grpc.ClientConn) {
	obs := observability.Current()
	if obs == nil || (!obs.Enabled(observability.FamilyLogs) && !obs.Enabled(observability.FamilyEvents)) {
		return nil, nil
	}
	relays := make([]*observability.Relay, 0, len(members))
	conns := make([]*grpc.ClientConn, 0, len(members))
	for _, member := range members {
		member.Slug = strings.TrimSpace(member.Slug)
		member.UID = strings.TrimSpace(member.UID)
		member.Address = strings.TrimSpace(member.Address)
		if member.Slug == "" || member.UID == "" || member.Address == "" {
			log.Printf("warning: observability relay skipped incomplete member ref: slug=%q uid=%q address=%q", member.Slug, member.UID, member.Address)
			continue
		}
		conn, err := grpcclient.Dial(ctx, normalizeRelayDialTarget(member.Address))
		if err != nil {
			log.Printf("warning: observability relay dial %s/%s: %v", member.Slug, member.UID, err)
			continue
		}
		relay := observability.NewRelay(member.Slug, member.UID, conn)
		if err := relay.Start(ctx); err != nil {
			_ = conn.Close()
			log.Printf("warning: observability relay start %s/%s: %v", member.Slug, member.UID, err)
			continue
		}
		relays = append(relays, relay)
		conns = append(conns, conn)
	}
	return relays, conns
}

func normalizeRelayDialTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	switch parsed.Scheme {
	case "tcp":
		host := parsed.Hostname()
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		port := parsed.Port()
		if port == "" {
			return trimmed
		}
		return net.JoinHostPort(host, port)
	case "unix", "ws", "wss":
		return trimmed
	default:
		return trimmed
	}
}

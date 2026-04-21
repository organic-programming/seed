package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
)

const (
	defaultListenURI = "tcp://127.0.0.1:0"
	defaultSDK       = "go-holons"
	defaultVersion   = "0.3.0"
)

type options struct {
	listenURI string
	sdk       string
	version   string
}

type PingRequest struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type echoService interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
}

type server struct {
	sdk     string
	version string
}

func (s server) Ping(_ context.Context, in *PingRequest) (*PingResponse, error) {
	return &PingResponse{
		Message: in.Message,
		SDK:     s.sdk,
		Version: s.version,
	}, nil
}

var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "echo.v1.Echo",
	HandlerType: (*echoService)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(
				srv interface{},
				ctx context.Context,
				dec func(interface{}) error,
				interceptor grpc.UnaryServerInterceptor,
			) (interface{}, error) {
				in := new(PingRequest)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(echoService).Ping(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/echo.v1.Echo/Ping",
				}
				handler := func(ctx context.Context, req interface{}) (interface{}, error) {
					return srv.(echoService).Ping(ctx, req.(*PingRequest))
				}
				return interceptor(ctx, in, info, handler)
			},
		},
	},
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		// Compatibility with grpcclient.DialStdio helper processes.
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	opts := parseFlags()
	describe.UseStaticResponse(echoDescribeResponse(opts))

	lis, err := transport.Listen(opts.listenURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	grpcServer.RegisterService(&echoServiceDesc, server{
		sdk:     opts.sdk,
		version: opts.version,
	})

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- grpcServer.Serve(lis)
	}()

	if !isStdioURI(opts.listenURI) {
		fmt.Println(publicURI(opts.listenURI, lis.Addr()))
	}

	if isStdioURI(opts.listenURI) {
		if err := <-serveErrCh; err != nil && !isBenignServeError(err) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
		shutdown(grpcServer)
	case err := <-serveErrCh:
		if err != nil && !isBenignServeError(err) {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := <-serveErrCh; err != nil && !isBenignServeError(err) {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	listen := flag.String("listen", defaultListenURI, "transport URI to listen on")
	port := flag.String("port", "", "tcp port shortcut (equivalent to --listen tcp://127.0.0.1:<port>)")
	sdk := flag.String("sdk", defaultSDK, "sdk identifier in Ping response")
	version := flag.String("version", defaultVersion, "sdk version in Ping response")
	flag.Parse()

	listenURI := *listen
	if *port != "" {
		listenURI = fmt.Sprintf("tcp://127.0.0.1:%s", *port)
	}

	return options{
		listenURI: listenURI,
		sdk:       *sdk,
		version:   *version,
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

func publicURI(listenURI string, addr net.Addr) string {
	if strings.HasPrefix(listenURI, "tcp://") {
		host := extractTCPHost(listenURI)
		if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
			host = "127.0.0.1"
		}

		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			return fmt.Sprintf("tcp://%s", addr.String())
		}
		return fmt.Sprintf("tcp://%s:%s", host, port)
	}
	return listenURI
}

func extractTCPHost(uri string) string {
	rest := strings.TrimPrefix(uri, "tcp://")
	host, _, err := net.SplitHostPort(rest)
	if err != nil {
		return ""
	}
	return host
}

func isStdioURI(uri string) bool {
	return uri == "stdio://" || uri == "stdio"
}

func isBenignServeError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, grpc.ErrServerStopped) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection")
}

func echoDescribeResponse(opts options) *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "echo-server-sdk-0000",
				GivenName:  "Echo",
				FamilyName: "Server",
				Motto:      "Echoes payloads for SDK transport tests.",
				Composer:   "go-holons",
				Status:     "draft",
				Born:       "2026-03-23",
				Version:    opts.version,
			},
			Lang: "go",
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        echoServiceDesc.ServiceName,
			Description: "Echo test server used by go-holons integration tests.",
			Methods: []*holonsv1.MethodDoc{{
				Name:        "Ping",
				Description: "Returns the inbound message with SDK metadata.",
				InputType:   "echo.v1.PingRequest",
				OutputType:  "echo.v1.PingResponse",
			}},
		}},
	}
}

package main

import (
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

const defaultListenURI = "tcp://127.0.0.1:0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	listen := flag.String("listen", defaultListenURI, "tcp URI to listen on")
	flag.Parse()

	listener, err := transport.Listen(*listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	describe.UseStaticResponse(describeOnlyResponse())
	if err := describe.Register(grpcServer); err != nil {
		fmt.Fprintf(os.Stderr, "register describe failed: %v\n", err)
		os.Exit(1)
	}

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- grpcServer.Serve(listener)
	}()

	if !isStdioURI(*listen) {
		fmt.Println(publicURI(*listen, listener.Addr()))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
		shutdown(grpcServer)
	case err := <-serveErrCh:
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
			fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := <-serveErrCh; err != nil && !strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func publicURI(listenURI string, addr net.Addr) string {
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

func isStdioURI(uri string) bool {
	return strings.TrimSpace(uri) == "stdio://"
}

func describeOnlyResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "mcp-test-rob-go",
				GivenName:  "Rob",
				FamilyName: "Go",
				Motto:      "Build what you mean.",
				Composer:   "test",
				Status:     "draft",
				Born:       "2026-03-08",
				Aliases:    []string{"rob-go", "rob"},
			},
			Lang: "go",
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        "rob_go.v1.RobGoService",
			Description: "Wraps the go command for gRPC access.",
			Methods: []*holonsv1.MethodDoc{{
				Name:        "Build",
				Description: "Compile Go packages.",
				InputType:   "rob_go.v1.BuildRequest",
				OutputType:  "rob_go.v1.BuildResponse",
				InputFields: []*holonsv1.FieldDoc{{
					Name:        "package",
					Type:        "string",
					Number:      1,
					Description: "The Go package to build.",
					Label:       holonsv1.FieldLabel_FIELD_LABEL_REQUIRED,
					Required:    true,
				}},
				OutputFields: []*holonsv1.FieldDoc{{
					Name:        "output",
					Type:        "string",
					Number:      1,
					Description: "Compiler output.",
				}},
			}},
		}},
	}
}

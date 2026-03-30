package main

import (
	"context"
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
	echov1 "github.com/organic-programming/grace-op/internal/cli/testsupport/echoholon/protos/echo/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const defaultListenURI = "tcp://127.0.0.1:0"

type server struct {
	echov1.UnimplementedEchoServiceServer
}

func (server) Ping(_ context.Context, request *echov1.PingRequest) (*echov1.PingResponse, error) {
	message := request.GetMessage()
	switch request.GetMode() {
	case echov1.EchoMode_ECHO_MODE_UPPER:
		message = strings.ToUpper(message)
	case echov1.EchoMode_ECHO_MODE_LOWER:
		message = strings.ToLower(message)
	}

	return &echov1.PingResponse{
		Message: message,
		Count:   int32(len(request.GetTags())),
		Mode:    request.GetMode(),
	}, nil
}

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
	echov1.RegisterEchoServiceServer(grpcServer, server{})
	describe.UseStaticResponse(echoDescribeResponse())
	if err := describe.Register(grpcServer); err != nil {
		fmt.Fprintf(os.Stderr, "register describe failed: %v\n", err)
		os.Exit(1)
	}
	reflection.Register(grpcServer)

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

func echoDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "grace-op-echoholon-0000",
				GivenName:  "Echo",
				FamilyName: "Server",
				Motto:      "Echo what you send.",
				Composer:   "test",
				Status:     "draft",
				Born:       "2026-03-08",
				Aliases:    []string{"echo", "echo-server"},
			},
			Lang: "go",
			Skills: []*holonsv1.HolonManifest_Skill{{
				Name:        "repeat-back",
				Description: "Echo user text back with optional tags.",
				When:        "User wants a safe connectivity smoke test.",
				Steps: []string{
					"Call EchoService.Ping with the text to echo.",
					"Inspect the returned message, tag count, and mode.",
				},
			}},
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        "echo.v1.EchoService",
			Description: "Echoes request payloads for MCP integration tests.",
			Methods: []*holonsv1.MethodDoc{{
				Name:         "Ping",
				Description:  "Echo the request payload.",
				InputType:    "echo.v1.PingRequest",
				OutputType:   "echo.v1.PingResponse",
				ExampleInput: `{"message":"hello","tags":["one","two"],"mode":"ECHO_MODE_UPPER"}`,
				InputFields: []*holonsv1.FieldDoc{
					{
						Name:        "message",
						Type:        "string",
						Number:      1,
						Description: "Message to echo back.",
						Label:       holonsv1.FieldLabel_FIELD_LABEL_REQUIRED,
						Required:    true,
						Example:     `"hello"`,
					},
					{
						Name:        "tags",
						Type:        "string",
						Number:      2,
						Description: "Tags to include in the response count.",
						Label:       holonsv1.FieldLabel_FIELD_LABEL_REPEATED,
						Example:     `["one","two"]`,
					},
					{
						Name:        "mode",
						Type:        "echo.v1.EchoMode",
						Number:      3,
						Description: "Output mode.",
						EnumValues: []*holonsv1.EnumValueDoc{
							{Name: "ECHO_MODE_UNSPECIFIED", Number: 0},
							{Name: "ECHO_MODE_UPPER", Number: 1},
							{Name: "ECHO_MODE_LOWER", Number: 2},
						},
					},
				},
				OutputFields: []*holonsv1.FieldDoc{
					{
						Name:        "message",
						Type:        "string",
						Number:      1,
						Description: "Echoed text.",
					},
					{
						Name:        "count",
						Type:        "int32",
						Number:      2,
						Description: "Number of tags seen.",
					},
					{
						Name:        "mode",
						Type:        "echo.v1.EchoMode",
						Number:      3,
						Description: "Mode chosen by the caller.",
						EnumValues: []*holonsv1.EnumValueDoc{
							{Name: "ECHO_MODE_UNSPECIFIED", Number: 0},
							{Name: "ECHO_MODE_UPPER", Number: 1},
							{Name: "ECHO_MODE_LOWER", Number: 2},
						},
					},
				},
			}},
		}},
	}
}

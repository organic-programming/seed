package grpcclient

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

type envServer struct {
	opv1.UnimplementedOPServiceServer
}

func (envServer) Env(_ context.Context, req *opv1.EnvRequest) (*opv1.EnvResponse, error) {
	return &opv1.EnvResponse{
		Oppath:      "/tmp/op",
		Opbin:       "/tmp/bin",
		Root:        "/tmp/root",
		Initialized: true,
		Shell:       map[bool]string{true: "shell=true", false: "shell=false"}[req.GetShell()],
		CacheDir:    "/tmp/cache",
	}, nil
}

type staticDescribeServer struct {
	holonsv1.UnimplementedHolonMetaServer
	response *holonsv1.DescribeResponse
}

func (s staticDescribeServer) Describe(context.Context, *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	return s.response, nil
}

func TestInvokeConnUsesDescribeWithoutReflection(t *testing.T) {
	address, cleanup := startTestGRPCServer(t, true, false)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	result, err := InvokeConn(ctx, conn, "Env", `{"shell":true}`)
	if err != nil {
		t.Fatalf("InvokeConn: %v", err)
	}
	if result.Service != "op.v1.OPService" {
		t.Fatalf("Service = %q, want %q", result.Service, "op.v1.OPService")
	}
	if result.Method != "Env" {
		t.Fatalf("Method = %q, want %q", result.Method, "Env")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Output), &payload); err != nil {
		t.Fatalf("json.Unmarshal(output): %v\noutput=%s", err, result.Output)
	}
	if got := payload["shell"]; got != "shell=true" {
		t.Fatalf("shell = %v, want %q", got, "shell=true")
	}
}

func TestListMethodsUsesDescribeWithoutReflection(t *testing.T) {
	address, cleanup := startTestGRPCServer(t, true, false)
	defer cleanup()

	methods, err := ListMethods(address)
	if err != nil {
		t.Fatalf("ListMethods: %v", err)
	}
	if len(methods) != 1 || methods[0] != "op.v1.OPService/Env" {
		t.Fatalf("methods = %v, want [op.v1.OPService/Env]", methods)
	}
}

func TestInvokeConnFallsBackToReflection(t *testing.T) {
	address, cleanup := startTestGRPCServer(t, false, true)
	defer cleanup()

	result, err := Dial(address, "Env", `{"shell":true}`)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Output), &payload); err != nil {
		t.Fatalf("json.Unmarshal(output): %v\noutput=%s", err, result.Output)
	}
	if got := payload["shell"]; got != "shell=true" {
		t.Fatalf("shell = %v, want %q", got, "shell=true")
	}
}

func startTestGRPCServer(t *testing.T, withDescribe bool, withReflection bool) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	server := grpc.NewServer()
	opv1.RegisterOPServiceServer(server, envServer{})
	if withDescribe {
		holonsv1.RegisterHolonMetaServer(server, staticDescribeServer{
			response: &holonsv1.DescribeResponse{
				Manifest: &holonsv1.HolonManifest{
					Identity: &holonsv1.HolonManifest_Identity{
						GivenName: "Op",
					},
				},
				Services: []*holonsv1.ServiceDoc{
					{
						Name: "op.v1.OPService",
						Methods: []*holonsv1.MethodDoc{
							{
								Name:       "Env",
								InputType:  "op.v1.EnvRequest",
								OutputType: "op.v1.EnvResponse",
								InputFields: []*holonsv1.FieldDoc{
									{Name: "init", Type: "bool", Number: 1},
									{Name: "shell", Type: "bool", Number: 2},
								},
								OutputFields: []*holonsv1.FieldDoc{
									{Name: "oppath", Type: "string", Number: 1},
									{Name: "opbin", Type: "string", Number: 2},
									{Name: "root", Type: "string", Number: 3},
									{Name: "initialized", Type: "bool", Number: 4},
									{Name: "shell", Type: "string", Number: 5},
									{Name: "cache_dir", Type: "string", Number: 6},
								},
							},
						},
					},
				},
			},
		})
	}
	if withReflection {
		reflection.Register(server)
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
	}
}

package server_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	holonserve "github.com/organic-programming/go-holons/pkg/serve"
	"github.com/organic-programming/go-holons/pkg/transport"
	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/identity"
	"github.com/organic-programming/grace-op/internal/server"
	"github.com/organic-programming/grace-op/internal/testutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// startTestServer launches an in-memory gRPC server.
func startTestServer(t *testing.T, root string) (opv1.OPServiceClient, func()) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	server.Register(s, api.RPCHandler{})

	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		conn.Close()
		s.Stop()
		os.Chdir(original) //nolint:errcheck
	}

	return opv1.NewOPServiceClient(conn), cleanup
}

// seedHolon creates a holon.proto in a temp subdirectory.
func seedHolon(t *testing.T, root, uuid, givenName string) {
	t.Helper()
	dir := filepath.Join(root, givenName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	id := identity.Identity{
		UUID:        uuid,
		GivenName:   givenName,
		FamilyName:  "Test",
		Motto:       "Testing.",
		Composer:    "Test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-01-01",
		GeneratedBy: "test",
		Lang:        "go",
	}
	if err := testutil.WriteIdentityFile(id, filepath.Join(dir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}
}

// --- Discover tests ---

func TestDiscoverEmpty(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.Discover(context.Background(), &opv1.DiscoverRequest{})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("Discover returned %d entries, want 0", len(resp.Entries))
	}
}

func TestDiscoverWithHolons(t *testing.T) {
	root := t.TempDir()
	seedHolon(t, root, "disc-1", "Alpha")
	seedHolon(t, root, "disc-2", "Beta")

	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.Discover(context.Background(), &opv1.DiscoverRequest{})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("Discover returned %d entries, want 2", len(resp.Entries))
	}
	for _, e := range resp.Entries {
		if e.Origin != "local" {
			t.Errorf("Origin = %q, want %q", e.Origin, "local")
		}
	}
}

// --- Invoke tests ---

func TestInvokeUnknown(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.Invoke(context.Background(), &opv1.InvokeRequest{
		Holon: "nonexistent-holon",
		Args:  []string{"hello"},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if resp.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", resp.ExitCode)
	}
}

func TestInvokeEcho(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	// "echo" is in $PATH on all platforms
	resp, err := client.Invoke(context.Background(), &opv1.InvokeRequest{
		Holon: "echo",
		Args:  []string{"hello", "from", "op"},
	})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", resp.ExitCode)
	}
	expected := "hello from op\n"
	if resp.Stdout != expected {
		t.Errorf("Stdout = %q, want %q", resp.Stdout, expected)
	}
}

// --- Identity RPCs (promoted from Sophia) ---

func TestCreateIdentity(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.CreateIdentity(context.Background(), &opv1.CreateIdentityRequest{
		GivenName:  "NewHolon",
		FamilyName: "RPC",
		Motto:      "Born by gRPC.",
		Composer:   "Test Suite",
		Clade:      opv1.Clade_PROBABILISTIC_GENERATIVE,
		OutputDir:  filepath.Join(root, "new-holon"),
	})
	if err != nil {
		t.Fatalf("CreateIdentity failed: %v", err)
	}
	if resp.Identity.Uuid == "" {
		t.Error("UUID must not be empty")
	}
	if resp.Identity.GivenName != "NewHolon" {
		t.Errorf("GivenName = %q, want %q", resp.Identity.GivenName, "NewHolon")
	}
	if _, err := os.Stat(resp.FilePath); err != nil {
		t.Errorf("holon.proto not created: %v", err)
	}
}

func TestCreateIdentityValidation(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	_, err := client.CreateIdentity(context.Background(), &opv1.CreateIdentityRequest{
		GivenName: "OnlyName",
	})
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestListIdentities(t *testing.T) {
	root := t.TempDir()
	seedHolon(t, root, "list-1", "Gamma")
	seedHolon(t, root, "list-2", "Delta")

	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.ListIdentities(context.Background(), &opv1.ListIdentitiesRequest{})
	if err != nil {
		t.Fatalf("ListIdentities failed: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("ListIdentities returned %d entries, want 2", len(resp.Entries))
	}
}

func TestListIdentitiesEmpty(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.ListIdentities(context.Background(), &opv1.ListIdentitiesRequest{})
	if err != nil {
		t.Fatalf("ListIdentities failed: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("ListIdentities returned %d entries, want 0", len(resp.Entries))
	}
}

func TestShowIdentity(t *testing.T) {
	root := t.TempDir()
	seedHolon(t, root, "show-uuid-42", "Epsilon")

	client, cleanup := startTestServer(t, root)
	defer cleanup()

	resp, err := client.ShowIdentity(context.Background(), &opv1.ShowIdentityRequest{
		Uuid: "show-uuid-42",
	})
	if err != nil {
		t.Fatalf("ShowIdentity failed: %v", err)
	}
	if resp.Identity.Uuid != "show-uuid-42" {
		t.Errorf("UUID = %q, want %q", resp.Identity.Uuid, "show-uuid-42")
	}
	if resp.RawContent == "" {
		t.Error("RawContent must not be empty")
	}
}

func TestShowIdentityNotFound(t *testing.T) {
	root := t.TempDir()
	client, cleanup := startTestServer(t, root)
	defer cleanup()

	_, err := client.ShowIdentity(context.Background(), &opv1.ShowIdentityRequest{
		Uuid: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent UUID")
	}
}

// --- Standard Go serve integration ---

func TestRunWithOptionsPortConflict(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	port := lis.Addr().(*net.TCPAddr).Port
	err = holonserve.RunWithOptions(fmt.Sprintf("tcp://:%d", port), func(s *grpc.Server) {
		server.Register(s, api.RPCHandler{})
	}, false)
	if err == nil {
		t.Fatal("expected error for port conflict")
	}
}

// --- ws:// transport test ---

func TestWSTransport(t *testing.T) {
	root := t.TempDir()
	seedHolon(t, root, "ws-1", "WSAlpha")

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original) //nolint:errcheck

	wsLis, err := transport.Listen("ws://127.0.0.1:0")
	if err != nil {
		t.Fatalf("ws listen: %v", err)
	}
	defer wsLis.Close()

	s := grpc.NewServer()
	server.Register(s, api.RPCHandler{})
	reflection.Register(s)
	go func() { _ = s.Serve(wsLis) }()
	defer s.Stop()

	// Get the actual port from the WS listener's addr
	wsAddr := wsLis.Addr().String()

	// Use OP's DialWebSocket to call Discover
	result, err := grpcclient.DialWebSocket(wsAddr, "Discover", "{}")
	if err != nil {
		t.Fatalf("DialWebSocket Discover: %v", err)
	}
	if result.Output == "" {
		t.Error("expected non-empty output from Discover")
	}
}

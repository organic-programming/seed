// These tests verify the RPC facet of clem-ader over an in-memory gRPC transport with an explicit config-dir.
package api

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	"github.com/organic-programming/clem-ader/internal/testrepo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestRPCSurface(t *testing.T) {
	root := testrepo.Create(t)
	configDir := filepath.Join(root, "integration")
	withWorkingDir(t, root, func() {
		client := startRPCServer(t)

		testResponse, err := client.Test(context.Background(), &aderv1.TestRequest{
			ConfigDir:     configDir,
			Suite:         "fixture",
			Profile:       "integration",
			Source:        "workspace",
			ArchivePolicy: "never",
		})
		if err != nil {
			t.Fatalf("RPC Test() error = %v", err)
		}
		historyID := testResponse.GetManifest().GetHistoryId()
		if historyID == "" {
			t.Fatal("expected history id from RPC Test")
		}

		historyResponse, err := client.History(context.Background(), &aderv1.HistoryRequest{ConfigDir: configDir})
		if err != nil {
			t.Fatalf("RPC History() error = %v", err)
		}
		if len(historyResponse.GetEntries()) == 0 {
			t.Fatal("expected at least one history entry from RPC History")
		}

		showResponse, err := client.ShowHistory(context.Background(), &aderv1.ShowHistoryRequest{
			ConfigDir: configDir,
			HistoryId: historyID,
		})
		if err != nil {
			t.Fatalf("RPC ShowHistory() error = %v", err)
		}
		if showResponse.GetManifest().GetHistoryId() != historyID {
			t.Fatalf("RPC ShowHistory() history id = %q, want %q", showResponse.GetManifest().GetHistoryId(), historyID)
		}

		archiveResponse, err := client.Archive(context.Background(), &aderv1.ArchiveRequest{
			ConfigDir: configDir,
			Latest:    true,
		})
		if err != nil {
			t.Fatalf("RPC Archive() error = %v", err)
		}
		if archiveResponse.GetArchivePath() == "" {
			t.Fatal("expected archive path from RPC Archive")
		}

		stale := filepath.Join(root, "integration", ".artifacts", "local-suite", "rpc-stale")
		if err := os.MkdirAll(stale, 0o755); err != nil {
			t.Fatalf("mkdir stale: %v", err)
		}
		cleanupResponse, err := client.Cleanup(context.Background(), &aderv1.CleanupRequest{ConfigDir: configDir})
		if err != nil {
			t.Fatalf("RPC Cleanup() error = %v", err)
		}
		if cleanupResponse.GetRemovedLocalSuiteDirs() == 0 {
			t.Fatal("expected Cleanup RPC to remove at least one directory")
		}

		downgradeResponse, err := client.Downgrade(context.Background(), &aderv1.DowngradeRequest{
			ConfigDir: configDir,
			Suite:     "fixture",
			Profile:   "unit",
			StepIds:   []string{"sdk-go-unit"},
		})
		if err != nil {
			t.Fatalf("RPC Downgrade() error = %v", err)
		}
		if len(downgradeResponse.GetProfileChanges()) != 1 {
			t.Fatalf("RPC Downgrade() profile changes = %d, want 1", len(downgradeResponse.GetProfileChanges()))
		}
	})
}

func startRPCServer(t *testing.T) aderv1.AderServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	aderv1.RegisterAderServiceServer(server, RPCHandler{})
	go func() {
		if err := server.Serve(lis); err != nil {
			t.Logf("rpc server exited: %v", err)
		}
	}()
	t.Cleanup(server.GracefulStop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return aderv1.NewAderServiceClient(conn)
}

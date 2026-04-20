//go:build e2e

package uninstall_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestUninstall_RPC_RemovesInstalledArtifact(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Install(ctx, &opv1.InstallRequest{Target: "gabriel-greeting-go", Build: true})
	if err != nil {
		t.Fatalf("rpc Install: %v", err)
	}
	resp, err := client.Uninstall(ctx, &opv1.UninstallRequest{Target: "gabriel-greeting-go.holon"})
	if err != nil {
		t.Fatalf("rpc Uninstall: %v", err)
	}
	integration.RequirePathMissing(t, resp.GetReport().GetInstalled())
}

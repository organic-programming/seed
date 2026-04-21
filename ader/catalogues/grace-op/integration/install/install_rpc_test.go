//go:build e2e

package install_test

import (
	"context"
	"testing"
	"time"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInstall_RPC_BuildAndInstall(t *testing.T) {
	sb := integration.NewSandbox(t)
	client, cleanup := integration.SetupSandboxStdioOPClient(t, sb)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Install(ctx, &opv1.InstallRequest{Target: "gabriel-greeting-go", Build: true})
	if err != nil {
		t.Fatalf("rpc Install: %v", err)
	}
	integration.RequirePathExists(t, resp.GetReport().GetInstalled())
}

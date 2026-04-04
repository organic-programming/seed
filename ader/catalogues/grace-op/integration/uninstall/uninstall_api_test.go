package uninstall_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestUninstall_API_RemovesInstalledArtifact(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		installed, err := api.Install(&opv1.InstallRequest{Target: "gabriel-greeting-go", Build: true})
		if err != nil {
			t.Fatalf("api.Install: %v", err)
		}
		resp, err := api.Uninstall(&opv1.UninstallRequest{Target: "gabriel-greeting-go.holon"})
		if err != nil {
			t.Fatalf("api.Uninstall: %v", err)
		}
		integration.RequirePathMissing(t, resp.GetReport().GetInstalled())
		integration.RequirePathExists(t, installed.GetReport().GetArtifact())
	})
}

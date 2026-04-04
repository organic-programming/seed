package install_test

import (
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestInstall_API_BuildAndInstall(t *testing.T) {
	sb := integration.NewSandbox(t)
	integration.WithSandboxEnv(t, sb, func() {
		resp, err := api.Install(&opv1.InstallRequest{Target: "gabriel-greeting-go", Build: true})
		if err != nil {
			t.Fatalf("api.Install: %v", err)
		}
		integration.RequirePathExists(t, resp.GetReport().GetInstalled())
	})
}

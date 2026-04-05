package composite_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuild_07_Composite(t *testing.T) {
	rootPath := integration.DefaultWorkspaceDir(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	for _, spec := range integration.CompositeTestHolons(t) {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			t.Logf("Building composite app %s from a clean state...", spec.Slug)
			cmd := exec.Command(opBin, "build", spec.Slug, "--root", rootPath)
			cmd.Env = envVars
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to build composite %s: %v\nOutput: %s", spec.Slug, err, string(out))
			}

			if _, err := os.Stat(integration.CompositeArtifactPath(rootPath, spec.Slug)); err != nil {
				t.Fatalf("expected built composite artifact for %s: %v", spec.Slug, err)
			}
		})
	}
}

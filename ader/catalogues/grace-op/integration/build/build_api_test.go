package build_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuildAPI_BuildDryRun_GabrielGreetingGo(t *testing.T) {
	env := newBuildTestEnv(t)

	report := buildViaAPI(t, env, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertSourceReport(t, "API build dry-run", report, "gabriel-greeting-go")
	if report.Operation != "build" {
		t.Fatalf("operation = %q, want build", report.Operation)
	}
}

func TestBuildAPI_BuildDryRunNoSign_GabrielGreetingAppSwiftUI(t *testing.T) {
	env := newBuildTestEnv(t)

	report := buildViaAPI(t, env, "gabriel-greeting-app-swiftui", &opv1.BuildOptions{
		DryRun: true,
		NoSign: true,
	})

	if report == nil {
		t.Fatal("expected lifecycle report")
	}
	if report.Holon != "gabriel-greeting-app-swiftui" {
		t.Fatalf("holon = %q, want gabriel-greeting-app-swiftui", report.Holon)
	}
}

func TestBuildAPI_BuildPrefersSourceOverOPBIN_GabrielGreetingGo(t *testing.T) {
	env := newBuildTestEnv(t)

	runOP(t, env.OpBin, env.EnvVars, "--root", env.AbsRoot, "build", "gabriel-greeting-go", "--install")
	report := buildViaAPI(t, env, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertSourceReport(t, "API source over OPBIN", report, "gabriel-greeting-go")
}

func TestBuildAPI_BuildPrefersSourceOverPATH_GabrielGreetingGo(t *testing.T) {
	env := newBuildTestEnv(t)

	runOP(t, env.OpBin, env.EnvVars, "--root", env.AbsRoot, "build", "gabriel-greeting-go")
	sourceBinary := buildArtifactPath(t, "gabriel-greeting-go")
	pathDir := t.TempDir()
	copyExecutable(t, sourceBinary, filepath.Join(pathDir, "gabriel-greeting-go"))

	env.EnvVars = withEnvEntry(env.EnvVars, "PATH", pathDir+string(os.PathListSeparator)+envValue(env.EnvVars, "PATH"))
	integration.TeardownHolons(t, env.AbsRoot)

	report := buildViaAPI(t, env, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertSourceReport(t, "API source over PATH", report, "gabriel-greeting-go")
}

func TestBuildAPI_BuildThenInstall_GabrielGreetingGo(t *testing.T) {
	env := newBuildTestEnv(t)

	buildReport := buildViaAPI(t, env, "gabriel-greeting-go", nil)
	installReport := installViaAPI(t, env, "gabriel-greeting-go")

	assertSourceReport(t, "API build", buildReport, "gabriel-greeting-go")
	if installReport == nil {
		t.Fatal("expected install report")
	}
	if strings.TrimSpace(installReport.Installed) == "" {
		t.Fatal("expected installed path in install report")
	}
}

func TestBuildAPI_CleanThenBuild_GabrielGreetingGo(t *testing.T) {
	env := newBuildTestEnv(t)

	buildViaAPI(t, env, "gabriel-greeting-go", nil)
	markerPath := filepath.Join(holonOPDir(t, "gabriel-greeting-go"), "stale-marker.txt")
	if err := os.WriteFile(markerPath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale marker: %v", err)
	}

	cleanReport := cleanViaAPI(t, env, "gabriel-greeting-go")
	buildReport := buildViaAPI(t, env, "gabriel-greeting-go", nil)

	if cleanReport == nil {
		t.Fatal("expected clean report")
	}
	assertSourceReport(t, "API build after clean", buildReport, "gabriel-greeting-go")
	assertMarkerRemoved(t, "gabriel-greeting-go")
	assertArtifactExists(t, "gabriel-greeting-go")
}

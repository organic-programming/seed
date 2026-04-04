package build_test

import (
	"os"
	"path/filepath"
	"testing"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuildRPC_BuildDryRunMatchesAPIAndCLI_GabrielGreetingGo(t *testing.T) {
	apiEnv := newBuildTestEnv(t)
	apiReport := buildViaAPI(t, apiEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	cliEnv := newBuildTestEnv(t)
	cliReport := buildViaCLIJSON(t, cliEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true}, false)

	rpcEnv := newBuildTestEnv(t)
	rpcReport := buildViaRPC(t, rpcEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertLifecycleEqual(t, "CLI dry-run", cliReport, "API dry-run", apiReport)
	assertLifecycleEqual(t, "RPC dry-run", rpcReport, "API dry-run", apiReport)
}

func TestBuildRPC_BuildDryRunNoSignMatchesAPIAndCLI_GabrielGreetingAppSwiftUI(t *testing.T) {
	opts := &opv1.BuildOptions{DryRun: true, NoSign: true}

	apiEnv := newBuildTestEnv(t)
	apiReport := buildViaAPI(t, apiEnv, "gabriel-greeting-app-swiftui", opts)

	cliEnv := newBuildTestEnv(t)
	cliReport := buildViaCLIJSON(t, cliEnv, "gabriel-greeting-app-swiftui", opts, false)

	rpcEnv := newBuildTestEnv(t)
	rpcReport := buildViaRPC(t, rpcEnv, "gabriel-greeting-app-swiftui", opts)

	assertLifecycleEqual(t, "CLI no-sign", cliReport, "API no-sign", apiReport)
	assertLifecycleEqual(t, "RPC no-sign", rpcReport, "API no-sign", apiReport)
}

func TestBuildRPC_BuildPrefersSourceOverOPBINMatchesAPIAndCLI_GabrielGreetingGo(t *testing.T) {
	apiEnv := newBuildTestEnv(t)
	runOP(t, apiEnv.OpBin, apiEnv.EnvVars, "--root", apiEnv.AbsRoot, "build", "gabriel-greeting-go", "--install")
	apiReport := buildViaAPI(t, apiEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	cliEnv := newBuildTestEnv(t)
	runOP(t, cliEnv.OpBin, cliEnv.EnvVars, "--root", cliEnv.AbsRoot, "build", "gabriel-greeting-go", "--install")
	cliReport := buildViaCLIJSON(t, cliEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true}, false)

	rpcEnv := newBuildTestEnv(t)
	runOP(t, rpcEnv.OpBin, rpcEnv.EnvVars, "--root", rpcEnv.AbsRoot, "build", "gabriel-greeting-go", "--install")
	rpcReport := buildViaRPC(t, rpcEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertSourceReport(t, "API source over OPBIN", apiReport, "gabriel-greeting-go")
	assertSourceReport(t, "CLI source over OPBIN", cliReport, "gabriel-greeting-go")
	assertSourceReport(t, "RPC source over OPBIN", rpcReport, "gabriel-greeting-go")
	assertLifecycleEqual(t, "CLI source over OPBIN", cliReport, "API source over OPBIN", apiReport)
	assertLifecycleEqual(t, "RPC source over OPBIN", rpcReport, "API source over OPBIN", apiReport)
}

func TestBuildRPC_BuildPrefersSourceOverPATHMatchesAPIAndCLI_GabrielGreetingGo(t *testing.T) {
	apiEnv := newBuildTestEnv(t)
	runOP(t, apiEnv.OpBin, apiEnv.EnvVars, "--root", apiEnv.AbsRoot, "build", "gabriel-greeting-go")
	apiPathDir := t.TempDir()
	copyExecutable(t, buildArtifactPath(t, "gabriel-greeting-go"), filepath.Join(apiPathDir, "gabriel-greeting-go"))
	apiEnv.EnvVars = withEnvEntry(apiEnv.EnvVars, "PATH", apiPathDir+string(os.PathListSeparator)+envValue(apiEnv.EnvVars, "PATH"))
	integration.TeardownHolons(t, apiEnv.AbsRoot)
	apiReport := buildViaAPI(t, apiEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	cliEnv := newBuildTestEnv(t)
	runOP(t, cliEnv.OpBin, cliEnv.EnvVars, "--root", cliEnv.AbsRoot, "build", "gabriel-greeting-go")
	cliPathDir := t.TempDir()
	copyExecutable(t, buildArtifactPath(t, "gabriel-greeting-go"), filepath.Join(cliPathDir, "gabriel-greeting-go"))
	cliEnv.EnvVars = withEnvEntry(cliEnv.EnvVars, "PATH", cliPathDir+string(os.PathListSeparator)+envValue(cliEnv.EnvVars, "PATH"))
	integration.TeardownHolons(t, cliEnv.AbsRoot)
	cliReport := buildViaCLIJSON(t, cliEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true}, false)

	rpcEnv := newBuildTestEnv(t)
	runOP(t, rpcEnv.OpBin, rpcEnv.EnvVars, "--root", rpcEnv.AbsRoot, "build", "gabriel-greeting-go")
	rpcPathDir := t.TempDir()
	copyExecutable(t, buildArtifactPath(t, "gabriel-greeting-go"), filepath.Join(rpcPathDir, "gabriel-greeting-go"))
	rpcEnv.EnvVars = withEnvEntry(rpcEnv.EnvVars, "PATH", rpcPathDir+string(os.PathListSeparator)+envValue(rpcEnv.EnvVars, "PATH"))
	integration.TeardownHolons(t, rpcEnv.AbsRoot)
	rpcReport := buildViaRPC(t, rpcEnv, "gabriel-greeting-go", &opv1.BuildOptions{DryRun: true})

	assertSourceReport(t, "API source over PATH", apiReport, "gabriel-greeting-go")
	assertSourceReport(t, "CLI source over PATH", cliReport, "gabriel-greeting-go")
	assertSourceReport(t, "RPC source over PATH", rpcReport, "gabriel-greeting-go")
	assertLifecycleEqual(t, "CLI source over PATH", cliReport, "API source over PATH", apiReport)
	assertLifecycleEqual(t, "RPC source over PATH", rpcReport, "API source over PATH", apiReport)
}

func TestBuildRPC_BuildThenInstallMatchesAPIAndCLI_GabrielGreetingGo(t *testing.T) {
	apiEnv := newBuildTestEnv(t)
	apiBuild := buildViaAPI(t, apiEnv, "gabriel-greeting-go", nil)
	apiInstall := installViaAPI(t, apiEnv, "gabriel-greeting-go")

	cliEnv := newBuildTestEnv(t)
	cliBuild, cliInstall := buildAndInstallViaCLIJSON(t, cliEnv, "gabriel-greeting-go", nil)

	rpcEnv := newBuildTestEnv(t)
	rpcBuild := buildViaRPC(t, rpcEnv, "gabriel-greeting-go", nil)
	rpcInstall := installViaRPC(t, rpcEnv, "gabriel-greeting-go")

	assertLifecycleEqual(t, "CLI build+install build", cliBuild, "API build+install build", apiBuild)
	assertLifecycleEqual(t, "RPC build+install build", rpcBuild, "API build+install build", apiBuild)
	assertInstallEqual(t, "RPC build+install install", rpcInstall, "API build+install install", apiInstall)
	if cliInstall == nil || apiInstall == nil {
		t.Fatal("expected install reports")
	}
}

func TestBuildRPC_CleanThenBuildMatchesAPIAndCLI_GabrielGreetingGo(t *testing.T) {
	apiEnv := newBuildTestEnv(t)
	buildViaAPI(t, apiEnv, "gabriel-greeting-go", nil)
	apiMarker := filepath.Join(holonOPDir(t, "gabriel-greeting-go"), "stale-marker.txt")
	if err := os.WriteFile(apiMarker, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write API stale marker: %v", err)
	}
	apiClean := cleanViaAPI(t, apiEnv, "gabriel-greeting-go")
	apiBuild := buildViaAPI(t, apiEnv, "gabriel-greeting-go", nil)
	assertMarkerRemoved(t, "gabriel-greeting-go")

	cliEnv := newBuildTestEnv(t)
	buildViaCLIJSON(t, cliEnv, "gabriel-greeting-go", nil, false)
	cliMarker := filepath.Join(holonOPDir(t, "gabriel-greeting-go"), "stale-marker.txt")
	if err := os.WriteFile(cliMarker, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write CLI stale marker: %v", err)
	}
	cliBuild := buildViaCLIJSON(t, cliEnv, "gabriel-greeting-go", nil, true)
	assertMarkerRemoved(t, "gabriel-greeting-go")

	rpcEnv := newBuildTestEnv(t)
	buildViaRPC(t, rpcEnv, "gabriel-greeting-go", nil)
	rpcMarker := filepath.Join(holonOPDir(t, "gabriel-greeting-go"), "stale-marker.txt")
	if err := os.WriteFile(rpcMarker, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write RPC stale marker: %v", err)
	}
	rpcClean := cleanViaRPC(t, rpcEnv, "gabriel-greeting-go")
	rpcBuild := buildViaRPC(t, rpcEnv, "gabriel-greeting-go", nil)
	assertMarkerRemoved(t, "gabriel-greeting-go")

	if apiClean == nil || rpcClean == nil {
		t.Fatal("expected clean reports")
	}
	assertLifecycleEqual(t, "CLI clean+build build", cliBuild, "API clean+build build", apiBuild)
	assertLifecycleEqual(t, "RPC clean+build build", rpcBuild, "API clean+build build", apiBuild)
}

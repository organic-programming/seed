package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/organic-programming/grace-op/api"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

type commandSurface struct {
	Command string
	RPCs    []string
}

func TestCommandSurfaceSymmetry(t *testing.T) {
	symmetric := []commandSurface{
		{Command: "build", RPCs: []string{"Build"}},
		{Command: "check", RPCs: []string{"Check"}},
		{Command: "clean", RPCs: []string{"Clean"}},
		{Command: "discover", RPCs: []string{"Discover"}},
		{Command: "do", RPCs: []string{"RunSequence"}},
		{Command: "env", RPCs: []string{"Env"}},
		{Command: "inspect", RPCs: []string{"Inspect"}},
		{Command: "install", RPCs: []string{"Install"}},
		{Command: "invoke", RPCs: []string{"Invoke"}},
		{Command: "list", RPCs: []string{"ListIdentities"}},
		{Command: "mod", RPCs: []string{"ModAdd", "ModGraph", "ModInit", "ModList", "ModPull", "ModRemove", "ModTidy", "ModUpdate"}},
		{Command: "new", RPCs: []string{"CreateIdentity", "GenerateTemplate", "ListTemplates"}},
		{Command: "run", RPCs: []string{"Run"}},
		{Command: "show", RPCs: []string{"ShowIdentity"}},
		{Command: "test", RPCs: []string{"Test"}},
		{Command: "tools", RPCs: []string{"Tools"}},
		{Command: "uninstall", RPCs: []string{"Uninstall"}},
		{Command: "version", RPCs: []string{"Version"}},
	}
	cliOnly := []string{"completion", "mcp", "serve"}

	wantCLI := append([]string{}, cliOnly...)
	wantRPC := make([]string, 0, len(symmetric))
	for _, surface := range symmetric {
		wantCLI = append(wantCLI, surface.Command)
		wantRPC = append(wantRPC, surface.RPCs...)
	}
	wantCLI = append(wantCLI, "help")
	sort.Strings(wantCLI)
	sort.Strings(wantRPC)

	root := newRootCmd("0.1.0-test")
	root.InitDefaultHelpCmd()
	gotCLI := make([]string, 0, len(root.Commands()))
	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}
		gotCLI = append(gotCLI, cmd.Name())
	}
	sort.Strings(gotCLI)
	if !reflect.DeepEqual(gotCLI, wantCLI) {
		t.Fatalf("CLI commands = %v, want %v", gotCLI, wantCLI)
	}

	gotRPC := make([]string, 0, len(opv1.OPService_ServiceDesc.Methods))
	for _, method := range opv1.OPService_ServiceDesc.Methods {
		gotRPC = append(gotRPC, method.MethodName)
	}
	sort.Strings(gotRPC)
	if !reflect.DeepEqual(gotRPC, wantRPC) {
		t.Fatalf("service RPCs = %v, want %v", gotRPC, wantRPC)
	}

	gotAPI := publicAPINames()
	sort.Strings(gotAPI)
	if !reflect.DeepEqual(gotAPI, wantRPC) {
		t.Fatalf("public API RPCs = %v, want %v", gotAPI, wantRPC)
	}

	protoPath := filepath.Join("..", "..", "api", "v1", "holon.proto")
	data, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("read proto: %v", err)
	}
	block := regexp.MustCompile(`rpcs:\s*\[(?s)(.*?)\]`).FindSubmatch(data)
	if len(block) != 2 {
		t.Fatal("failed to find contract.rpcs block in holon.proto")
	}
	re := regexp.MustCompile(`"([A-Za-z]+)"`)
	matches := re.FindAllSubmatch(block[1], -1)
	gotContract := make([]string, 0, len(matches))
	for _, match := range matches {
		gotContract = append(gotContract, string(match[1]))
	}
	sort.Strings(gotContract)
	if !reflect.DeepEqual(gotContract, wantRPC) {
		t.Fatalf("contract.rpcs = %v, want %v", gotContract, wantRPC)
	}
}

func publicAPINames() []string {
	var _ func(*opv1.DiscoverRequest) (*opv1.DiscoverResponse, error) = api.Discover
	var _ func(*opv1.InvokeRequest) (*opv1.InvokeResponse, error) = api.Invoke
	var _ func(*opv1.CreateIdentityRequest) (*opv1.CreateIdentityResponse, error) = api.CreateIdentity
	var _ func(*opv1.ListIdentitiesRequest) (*opv1.ListIdentitiesResponse, error) = api.ListIdentities
	var _ func(*opv1.ShowIdentityRequest) (*opv1.ShowIdentityResponse, error) = api.ShowIdentity
	var _ func(*opv1.ListTemplatesRequest) (*opv1.ListTemplatesResponse, error) = api.ListTemplates
	var _ func(*opv1.GenerateTemplateRequest) (*opv1.GenerateTemplateResponse, error) = api.GenerateTemplate
	var _ func(*opv1.VersionRequest) (*opv1.VersionResponse, error) = api.Version
	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Check
	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Build
	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Test
	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Clean
	var _ func(*opv1.InstallRequest) (*opv1.InstallResponse, error) = api.Install
	var _ func(*opv1.UninstallRequest) (*opv1.InstallResponse, error) = api.Uninstall
	var _ func(*opv1.RunRequest) (*opv1.RunResponse, error) = api.Run
	var _ func(*opv1.InspectRequest) (*opv1.InspectResponse, error) = api.Inspect
	var _ func(*opv1.RunSequenceRequest) (*opv1.RunSequenceResponse, error) = api.RunSequence
	var _ func(*opv1.ModInitRequest) (*opv1.ModInitResponse, error) = api.ModInit
	var _ func(*opv1.ModAddRequest) (*opv1.ModAddResponse, error) = api.ModAdd
	var _ func(*opv1.ModRemoveRequest) (*opv1.ModRemoveResponse, error) = api.ModRemove
	var _ func(*opv1.ModTidyRequest) (*opv1.ModTidyResponse, error) = api.ModTidy
	var _ func(*opv1.ModPullRequest) (*opv1.ModPullResponse, error) = api.ModPull
	var _ func(*opv1.ModUpdateRequest) (*opv1.ModUpdateResponse, error) = api.ModUpdate
	var _ func(*opv1.ModListRequest) (*opv1.ModListResponse, error) = api.ModList
	var _ func(*opv1.ModGraphRequest) (*opv1.ModGraphResponse, error) = api.ModGraph
	var _ func(*opv1.ToolsRequest) (*opv1.ToolsResponse, error) = api.Tools
	var _ func(*opv1.EnvRequest) (*opv1.EnvResponse, error) = api.Env

	return []string{
		"Build",
		"Check",
		"Clean",
		"CreateIdentity",
		"Discover",
		"Env",
		"GenerateTemplate",
		"Inspect",
		"Install",
		"Invoke",
		"ListIdentities",
		"ListTemplates",
		"ModAdd",
		"ModGraph",
		"ModInit",
		"ModList",
		"ModPull",
		"ModRemove",
		"ModTidy",
		"ModUpdate",
		"Run",
		"RunSequence",
		"ShowIdentity",
		"Test",
		"Tools",
		"Uninstall",
		"Version",
	}
}

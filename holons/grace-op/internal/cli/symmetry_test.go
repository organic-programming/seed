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

func TestBuildVersionSurfaceSymmetry(t *testing.T) {
	wantRPC := []string{"Build", "Clean", "Install", "Version"}

	gotRPC := make([]string, 0, len(wantRPC))
	for _, method := range opv1.OPService_ServiceDesc.Methods {
		switch method.MethodName {
		case "Build", "Clean", "Install", "Version":
			gotRPC = append(gotRPC, method.MethodName)
		}
	}
	sort.Strings(gotRPC)
	if !reflect.DeepEqual(gotRPC, wantRPC) {
		t.Fatalf("service RPC subset = %v, want %v", gotRPC, wantRPC)
	}

	root := newRootCmd("0.1.0-test")
	gotCLI := make([]string, 0, len(wantRPC))
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "build", "clean", "install", "version":
			gotCLI = append(gotCLI, cmd.Name())
		}
	}
	sort.Strings(gotCLI)
	wantCLI := []string{"build", "clean", "install", "version"}
	if !reflect.DeepEqual(gotCLI, wantCLI) {
		t.Fatalf("CLI command subset = %v, want %v", gotCLI, wantCLI)
	}

	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Build
	var _ func(*opv1.LifecycleRequest) (*opv1.LifecycleResponse, error) = api.Clean
	var _ func(*opv1.InstallRequest) (*opv1.InstallResponse, error) = api.Install
	var _ func(*opv1.VersionRequest) (*opv1.VersionResponse, error) = api.Version

	gotAPI := []string{"Build", "Clean", "Install", "Version"}
	sort.Strings(gotAPI)
	if !reflect.DeepEqual(gotAPI, wantRPC) {
		t.Fatalf("public API subset = %v, want %v", gotAPI, wantRPC)
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
	var gotContract []string
	for _, match := range matches {
		switch string(match[1]) {
		case "Build", "Clean", "Install", "Version":
			gotContract = append(gotContract, string(match[1]))
		}
	}
	sort.Strings(gotContract)
	if !reflect.DeepEqual(gotContract, wantRPC) {
		t.Fatalf("contract.rpcs subset = %v, want %v", gotContract, wantRPC)
	}
}

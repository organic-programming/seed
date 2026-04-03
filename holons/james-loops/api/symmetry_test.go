// These tests verify that the declared contract, CLI commands, RPC service, and public API stay aligned.
package api

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	jamesloopsv1 "github.com/organic-programming/james-loops/gen/go/v1"
)

func TestSurfaceSymmetry(t *testing.T) {
	wantRPC := []string{"Abort", "Drop", "Enqueue", "List", "Log", "ReEnqueue", "Resume", "Run", "Skip", "Status"}

	gotRPC := make([]string, 0, len(jamesloopsv1.JamesLoopsService_ServiceDesc.Methods))
	for _, method := range jamesloopsv1.JamesLoopsService_ServiceDesc.Methods {
		gotRPC = append(gotRPC, method.MethodName)
	}
	sort.Strings(gotRPC)
	if !reflect.DeepEqual(gotRPC, wantRPC) {
		t.Fatalf("service RPCs = %v, want %v", gotRPC, wantRPC)
	}

	root := newRootCommand(io.Discard, io.Discard)
	var gotCLI []string
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "serve", "help", "completion", "version", "profile":
			continue
		default:
			gotCLI = append(gotCLI, cmd.Name())
		}
	}
	sort.Strings(gotCLI)
	wantCLI := []string{"abort", "drop", "enqueue", "list", "log", "re-enqueue", "resume", "run", "skip", "status"}
	if !reflect.DeepEqual(gotCLI, wantCLI) {
		t.Fatalf("CLI commands = %v, want %v", gotCLI, wantCLI)
	}

	gotAPI := []string{"Abort", "Drop", "Enqueue", "List", "Log", "ReEnqueue", "Resume", "Run", "Skip", "Status"}
	sort.Strings(gotAPI)
	if !reflect.DeepEqual(gotAPI, wantRPC) {
		t.Fatalf("public API set = %v, want %v", gotAPI, wantRPC)
	}

	protoPath := filepath.Join("v1", "holon.proto")
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
		gotContract = append(gotContract, string(match[1]))
	}
	sort.Strings(gotContract)
	if !reflect.DeepEqual(gotContract, wantRPC) {
		t.Fatalf("contract.rpcs = %v, want %v", gotContract, wantRPC)
	}
}

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

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
)

func TestSurfaceSymmetry(t *testing.T) {
	wantRPC := []string{"Archive", "ArchiveBouquet", "BouquetHistory", "Cleanup", "Downgrade", "History", "Promote", "ShowBouquetHistory", "ShowHistory", "Test", "TestBouquet"}

	gotRPC := make([]string, 0, len(aderv1.AderService_ServiceDesc.Methods))
	for _, method := range aderv1.AderService_ServiceDesc.Methods {
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
		case "serve", "help", "completion", "version":
			continue
		default:
			gotCLI = append(gotCLI, cmd.Name())
		}
	}
	sort.Strings(gotCLI)
	wantCLI := []string{"archive", "archive-bouquet", "cleanup", "downgrade", "history", "history-bouquet", "promote", "show", "show-bouquet", "test", "test-bouquet"}
	if !reflect.DeepEqual(gotCLI, wantCLI) {
		t.Fatalf("CLI commands = %v, want %v", gotCLI, wantCLI)
	}

	gotAPI := []string{"Archive", "ArchiveBouquet", "BouquetHistory", "Cleanup", "Downgrade", "History", "Promote", "ShowBouquetHistory", "ShowHistory", "Test", "TestBouquet"}
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

func TestHistoryCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	history := root.Commands()[0]
	for _, cmd := range root.Commands() {
		if cmd.Name() == "history" {
			history = cmd
			break
		}
	}
	if history.Name() != "history" {
		t.Fatalf("history command missing from CLI")
	}
	if history.Use != "history <catalogue-dir>" {
		t.Fatalf("history use = %q, want %q", history.Use, "history <catalogue-dir>")
	}
}

func TestDowngradeCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	downgrade := root.Commands()[0]
	for _, cmd := range root.Commands() {
		if cmd.Name() == "downgrade" {
			downgrade = cmd
			break
		}
	}
	if downgrade.Name() != "downgrade" {
		t.Fatalf("downgrade command missing from CLI")
	}
	if downgrade.Use != "downgrade <catalogue-dir>" {
		t.Fatalf("downgrade use = %q, want %q", downgrade.Use, "downgrade <catalogue-dir>")
	}
	if downgrade.Flags().Lookup("profile") != nil {
		t.Fatal("downgrade should not expose --profile")
	}
}

func TestPromoteCommandShape(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard)
	promote := root.Commands()[0]
	for _, cmd := range root.Commands() {
		if cmd.Name() == "promote" {
			promote = cmd
			break
		}
	}
	if promote.Name() != "promote" {
		t.Fatalf("promote command missing from CLI")
	}
	if promote.Use != "promote <catalogue-dir>" {
		t.Fatalf("promote use = %q, want %q", promote.Use, "promote <catalogue-dir>")
	}
}

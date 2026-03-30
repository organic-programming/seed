package api

import (
	"strings"
	"testing"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/runpolicy"
)

func TestParseRunArgsDefaultsToLoopbackTCP(t *testing.T) {
	holon, opts, err := parseRunArgs([]string{"gabriel-greeting-go"})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if holon != "gabriel-greeting-go" {
		t.Fatalf("holon = %q, want %q", holon, "gabriel-greeting-go")
	}
	if opts.ListenURI != runpolicy.DefaultRunListenURI {
		t.Fatalf("ListenURI = %q, want %q", opts.ListenURI, runpolicy.DefaultRunListenURI)
	}
	if opts.ListenExplicit {
		t.Fatal("ListenExplicit = true, want false")
	}
}

func TestParseRunArgsRejectsExplicitStdioListen(t *testing.T) {
	_, _, err := parseRunArgs([]string{"gabriel-greeting-go", "--listen", "stdio://"})
	if err == nil {
		t.Fatal("parseRunArgs() error = nil, want rejection")
	}
	if !strings.Contains(err.Error(), "--listen stdio:// is not supported for op run") {
		t.Fatalf("error = %q, want stdio rejection", err)
	}
}

func TestParseRunArgsSupportsPortShorthand(t *testing.T) {
	holon, opts, err := parseRunArgs([]string{"gabriel-greeting-go:9097"})
	if err != nil {
		t.Fatalf("parseRunArgs() error = %v", err)
	}
	if holon != "gabriel-greeting-go" {
		t.Fatalf("holon = %q, want %q", holon, "gabriel-greeting-go")
	}
	if opts.ListenURI != "tcp://:9097" {
		t.Fatalf("ListenURI = %q, want %q", opts.ListenURI, "tcp://:9097")
	}
	if !opts.ListenExplicit {
		t.Fatal("ListenExplicit = false, want true")
	}
}

func TestRunWithIODefaultsToLoopbackTCP(t *testing.T) {
	resp, err := runWithIO(&opv1.RunRequest{Holon: "missing-holon"}, runIO{})
	if err == nil {
		t.Fatal("runWithIO() error = nil, want resolution failure")
	}
	if resp == nil {
		t.Fatal("runWithIO() response = nil, want response")
	}
	if resp.GetListenUri() != runpolicy.DefaultRunListenURI {
		t.Fatalf("ListenUri = %q, want %q", resp.GetListenUri(), runpolicy.DefaultRunListenURI)
	}
}

func TestRunWithIORejectsExplicitStdioListen(t *testing.T) {
	resp, err := runWithIO(&opv1.RunRequest{
		Holon:     "missing-holon",
		ListenUri: "stdio://",
	}, runIO{})
	if err == nil {
		t.Fatal("runWithIO() error = nil, want rejection")
	}
	if resp == nil {
		t.Fatal("runWithIO() response = nil, want response")
	}
	if !strings.Contains(err.Error(), "--listen stdio:// is not supported for op run") {
		t.Fatalf("error = %q, want stdio rejection", err)
	}
	if resp.GetListenUri() != "stdio://" {
		t.Fatalf("ListenUri = %q, want %q", resp.GetListenUri(), "stdio://")
	}
}

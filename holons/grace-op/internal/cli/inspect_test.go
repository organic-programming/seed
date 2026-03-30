package cli

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/grace-op/internal/identity"
	"google.golang.org/grpc"
)

func TestInspectCommandOfflineText(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedInspectableHolon(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"inspect", "rob-go"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("inspect returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "rob-go - Build what you mean.") {
		t.Fatalf("inspect output missing header: %q", output)
	}
	if !strings.Contains(output, "rob_go.v1.RobGoService") {
		t.Fatalf("inspect output missing service: %q", output)
	}
	if !strings.Contains(output, "Build(BuildRequest) -> BuildResponse") {
		t.Fatalf("inspect output missing method signature: %q", output)
	}
	if !strings.Contains(output, "@example \"./cmd/rob\"") {
		t.Fatalf("inspect output missing field example: %q", output)
	}
	if !strings.Contains(output, "Skills:") || !strings.Contains(output, "prepare-release") {
		t.Fatalf("inspect output missing skills: %q", output)
	}
}

func TestInspectCommandJSON(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedInspectableHolon(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"inspect", "rob-go", "--json"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("inspect --json returned %d, want 0", code)
		}
	})

	var payload struct {
		Slug     string `json:"slug"`
		Motto    string `json:"motto"`
		Services []struct {
			Name    string `json:"name"`
			Methods []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputFields []struct {
					Name     string `json:"name"`
					Required bool   `json:"required"`
					Example  string `json:"example"`
				} `json:"input_fields"`
			} `json:"methods"`
		} `json:"services"`
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("inspect json output is invalid: %v\noutput=%s", err, output)
	}
	if payload.Slug != "rob-go" {
		t.Fatalf("slug = %q, want %q", payload.Slug, "rob-go")
	}
	if payload.Motto != "Build what you mean." {
		t.Fatalf("motto = %q", payload.Motto)
	}
	if len(payload.Services) != 1 || payload.Services[0].Name != "rob_go.v1.RobGoService" {
		t.Fatalf("unexpected services payload: %+v", payload.Services)
	}
	if len(payload.Services[0].Methods) != 1 {
		t.Fatalf("methods = %d, want 1", len(payload.Services[0].Methods))
	}
	if len(payload.Services[0].Methods[0].InputFields) == 0 || !payload.Services[0].Methods[0].InputFields[0].Required {
		t.Fatalf("first input field missing required marker: %+v", payload.Services[0].Methods[0].InputFields)
	}
	if len(payload.Skills) != 1 || payload.Skills[0].Name != "prepare-release" {
		t.Fatalf("unexpected skills payload: %+v", payload.Skills)
	}
}

func TestInspectCommandProtoBackedHolonJSON(t *testing.T) {
	repoRoot := inspectRepoRoot(t)
	chdirForTest(t, repoRoot)

	output := captureStdout(t, func() {
		code := Run([]string{"inspect", "gabriel-greeting-go", "--json"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("inspect proto holon returned %d, want 0", code)
		}
	})

	var payload struct {
		Slug   string `json:"slug"`
		Motto  string `json:"motto"`
		Skills []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Sequences []struct {
			Name string `json:"name"`
		} `json:"sequences"`
		Services []struct {
			Name    string `json:"name"`
			Methods []struct {
				Name string `json:"name"`
			} `json:"methods"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("inspect proto holon json output is invalid: %v\noutput=%s", err, output)
	}
	if payload.Slug != "gabriel-greeting-go" {
		t.Fatalf("slug = %q, want %q", payload.Slug, "gabriel-greeting-go")
	}
	if payload.Motto == "" {
		t.Fatal("motto should not be empty")
	}
	if len(payload.Services) != 1 || payload.Services[0].Name != "greeting.v1.GreetingService" {
		t.Fatalf("unexpected services payload: %+v", payload.Services)
	}
	if got := len(payload.Services[0].Methods); got != 2 {
		t.Fatalf("methods = %d, want 2", got)
	}
	if len(payload.Skills) != 1 || payload.Skills[0].Name != "multilingual-greeter" {
		t.Fatalf("unexpected skills payload: %+v", payload.Skills)
	}
	if len(payload.Sequences) != 2 || payload.Sequences[0].Name != "multilingual-greeting" || payload.Sequences[1].Name != "greeting-fr-ja-ru-en" {
		t.Fatalf("unexpected sequences payload: %+v", payload.Sequences)
	}
}

func TestInspectCommandHostPortFallback(t *testing.T) {
	address := startDescribeServer(t, &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				GivenName:  "Echo",
				FamilyName: "Server",
				Motto:      "Echo what you send.",
			},
		},
		Services: []*holonsv1.ServiceDoc{
			{
				Name:        "echo.v1.EchoService",
				Description: "Echoes request payloads.",
				Methods: []*holonsv1.MethodDoc{
					{
						Name:        "Ping",
						Description: "Reply with the same text.",
						InputType:   "echo.v1.PingRequest",
						OutputType:  "echo.v1.PingResponse",
						InputFields: []*holonsv1.FieldDoc{
							{
								Name:        "text",
								Type:        "string",
								Number:      1,
								Description: "Text to echo back.",
								Label:       holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
								Required:    true,
								Example:     `"hello"`,
							},
						},
					},
				},
			},
		},
	})

	output := captureStdout(t, func() {
		code := Run([]string{"inspect", address, "--json"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("inspect host:port returned %d, want 0", code)
		}
	})

	var payload struct {
		Slug     string `json:"slug"`
		Services []struct {
			Name string `json:"name"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("inspect host:port json output is invalid: %v\noutput=%s", err, output)
	}
	if payload.Slug != "echo-server" {
		t.Fatalf("slug = %q, want %q", payload.Slug, "echo-server")
	}
	if len(payload.Services) != 1 || payload.Services[0].Name != "echo.v1.EchoService" {
		t.Fatalf("unexpected services: %+v", payload.Services)
	}
}

func TestInspectCommandMissingSlug(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	stderr := captureStderr(t, func() {
		code := Run([]string{"inspect", "missing"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("inspect missing returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, `op inspect: holon "missing" not found`) {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func seedInspectableHolon(t *testing.T, root string) {
	t.Helper()

	dir := filepath.Join(root, "holons", "rob-go")
	if err := os.MkdirAll(filepath.Join(dir, "protos", "rob_go", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := `schema: holon/v0
uuid: "inspect-test-rob-go"
given_name: "rob"
family_name: "go"
motto: "Build what you mean."
composer: "test"
clade: "deterministic/io_bound"
status: draft
born: "2026-03-08"
parents: []
reproduction: manual
aliases: ["rob-go", "rob"]
generated_by: "test"
lang: "go"
proto_status: draft
description: |
  Wraps the go command for gRPC access.
skills:
  - name: prepare-release
    description: Prepare a Go package for production release.
    when: User wants clean, tested, optimized code ready to ship.
    steps:
      - Fmt - format all source files
      - Vet - run static analysis
      - Test - run the full test suite
      - Build with mode=release
kind: native
build:
  runner: go-module
artifacts:
  binary: rob-go
`
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}

	proto := `syntax = "proto3";

package rob_go.v1;

// Wraps the go command for gRPC access.
service RobGoService {
  // Compile Go packages.
  // @example {"package":"./cmd/rob"}
  rpc Build(BuildRequest) returns (BuildResponse);
}

message BuildRequest {
  // The Go package to build.
  // @required
  // @example "./cmd/rob"
  string package = 1;
}

message BuildResponse {
  // Compiler output.
  string output = 1;

  // Whether the build succeeded.
  bool success = 2;
}
`
	if err := os.WriteFile(filepath.Join(dir, "protos", "rob_go", "v1", "rob_go.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
}

type staticDescribeServer struct {
	holonsv1.UnimplementedHolonMetaServer
	response *holonsv1.DescribeResponse
}

func (s staticDescribeServer) Describe(context.Context, *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	return s.response, nil
}

func startDescribeServer(t *testing.T, response *holonsv1.DescribeResponse) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	server := grpc.NewServer()
	holonsv1.RegisterHolonMetaServer(server, staticDescribeServer{response: response})
	go func() {
		_ = server.Serve(lis)
	}()

	t.Cleanup(func() {
		server.Stop()
		_ = lis.Close()
	})

	return lis.Addr().String()
}

func inspectRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

package inspect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProtoDirExtractsDocsAndTags(t *testing.T) {
	protoDir := t.TempDir()
	writeProtoFixture(t, protoDir)

	doc, err := ParseProtoDir(protoDir)
	if err != nil {
		t.Fatalf("ParseProtoDir returned error: %v", err)
	}

	if len(doc.Services) != 1 {
		t.Fatalf("services = %d, want 1", len(doc.Services))
	}
	service := doc.Services[0]
	if service.Name != "rob_go.v1.RobGoService" {
		t.Fatalf("service name = %q, want %q", service.Name, "rob_go.v1.RobGoService")
	}
	if service.Description != "Wraps the go command for gRPC access." {
		t.Fatalf("service description = %q", service.Description)
	}
	if len(service.Methods) != 1 {
		t.Fatalf("methods = %d, want 1", len(service.Methods))
	}

	method := service.Methods[0]
	if method.Description != "Compile Go packages." {
		t.Fatalf("method description = %q", method.Description)
	}
	if method.ExampleInput != `{"package":"./cmd/rob"}` {
		t.Fatalf("method example = %q", method.ExampleInput)
	}
	if got := len(method.InputFields); got != 2 {
		t.Fatalf("input fields = %d, want 2", got)
	}

	packageField := method.InputFields[0]
	if packageField.Name != "package" {
		t.Fatalf("first field name = %q, want %q", packageField.Name, "package")
	}
	if !packageField.Required {
		t.Fatalf("package field required = false, want true")
	}
	if packageField.Example != `"./cmd/rob"` {
		t.Fatalf("package field example = %q", packageField.Example)
	}
	if packageField.Description != "The Go package to build." {
		t.Fatalf("package field description = %q", packageField.Description)
	}

	optionsField := method.InputFields[1]
	if optionsField.Type != "rob_go.v1.Options" {
		t.Fatalf("options field type = %q", optionsField.Type)
	}
	if len(optionsField.NestedFields) != 1 {
		t.Fatalf("nested fields = %d, want 1", len(optionsField.NestedFields))
	}
	if optionsField.NestedFields[0].Name != "release" {
		t.Fatalf("nested field name = %q, want %q", optionsField.NestedFields[0].Name, "release")
	}
}

func TestRenderTextIncludesSkills(t *testing.T) {
	doc := &Document{
		Slug:  "rob-go",
		Motto: "Build what you mean.",
		Services: []Service{
			{
				Name:        "rob_go.v1.RobGoService",
				Description: "Wraps the go command for gRPC access.",
				Methods: []Method{
					{
						Name:        "Build",
						Description: "Compile Go packages.",
						InputType:   "rob_go.v1.BuildRequest",
						OutputType:  "rob_go.v1.BuildResponse",
						InputFields: []Field{
							{
								Name:        "package",
								Type:        "string",
								Description: "The Go package to build.",
								Required:    true,
								Example:     `"./cmd/rob"`,
							},
						},
					},
				},
			},
		},
		Skills: []Skill{
			{
				Name:        "prepare-release",
				Description: "Prepare a Go package for production release.",
				When:        "User wants clean, tested, optimized code ready to ship.",
				Steps: []string{
					"Fmt - format all source files",
					"Vet - run static analysis",
				},
			},
		},
	}

	output := RenderText(doc)
	assertContains(t, output, "rob-go - Build what you mean.")
	assertContains(t, output, "Build(BuildRequest) -> BuildResponse")
	assertContains(t, output, "@example \"./cmd/rob\"")
	assertContains(t, output, "Skills:")
	assertContains(t, output, "prepare-release - Prepare a Go package for production release.")
}

func writeProtoFixture(t *testing.T, protoDir string) {
	t.Helper()

	file := filepath.Join(protoDir, "rob_go", "v1", "rob_go.proto")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	content := `syntax = "proto3";

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

  // Build configuration.
  Options options = 2;
}

message Options {
  // Use release optimizations.
  bool release = 1;
}

message BuildResponse {
  // Compiler output.
  string output = 1;

  // Whether the build succeeded.
  bool success = 2;
}
`

	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output missing %q:\n%s", want, output)
	}
}

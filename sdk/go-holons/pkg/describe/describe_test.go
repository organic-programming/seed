package describe_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
)

func TestBuildResponseFromEchoProto(t *testing.T) {
	root := echoHolonDir(t)

	response, err := describe.BuildResponse(filepath.Join(root, "protos"), filepath.Join(root, "protos", "echo", "v1", "holon.proto"))
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}

	if got := describeResponseSlug(response); got != "echo-server" {
		t.Fatalf("slug = %q, want %q", got, "echo-server")
	}
	if got := response.GetManifest().GetIdentity().GetMotto(); got != "Reply precisely." {
		t.Fatalf("motto = %q, want %q", got, "Reply precisely.")
	}

	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}

	service := response.GetServices()[0]
	if service.GetName() != "echo.v1.Echo" {
		t.Fatalf("service name = %q, want %q", service.GetName(), "echo.v1.Echo")
	}
	if service.GetDescription() != "Echo echoes request payloads for documentation tests." {
		t.Fatalf("service description = %q", service.GetDescription())
	}

	if len(service.GetMethods()) != 1 {
		t.Fatalf("methods len = %d, want 1", len(service.GetMethods()))
	}

	method := service.GetMethods()[0]
	if method.GetName() != "Ping" {
		t.Fatalf("method name = %q, want %q", method.GetName(), "Ping")
	}
	if method.GetDescription() != "Ping echoes the inbound message." {
		t.Fatalf("method description = %q", method.GetDescription())
	}
	if method.GetExampleInput() != `{"message":"hello","sdk":"go-holons"}` {
		t.Fatalf("example_input = %q", method.GetExampleInput())
	}

	assertField(t, method.GetInputFields(), &holonsv1.FieldDoc{
		Name:        "message",
		Type:        "string",
		Number:      1,
		Description: "Message to echo back.",
		Label:       holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
		Required:    true,
		Example:     `"hello"`,
	})
}

func TestBuildResponseFromWrapperProtoWithSharedRecipesRoot(t *testing.T) {
	root := wrappedHolonDir(t)

	response, err := describe.BuildResponse(filepath.Join(root, "protos"), filepath.Join(root, "protos", "greeting", "v1", "holon.proto"))
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}

	if got := describeResponseSlug(response); got != "wrapped-greeter" {
		t.Fatalf("slug = %q, want %q", got, "wrapped-greeter")
	}
	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}

	service := response.GetServices()[0]
	if service.GetName() != "greeting.v1.GreetingService" {
		t.Fatalf("service name = %q, want %q", service.GetName(), "greeting.v1.GreetingService")
	}
	if len(service.GetMethods()) != 1 {
		t.Fatalf("methods len = %d, want 1", len(service.GetMethods()))
	}

	method := service.GetMethods()[0]
	if method.GetName() != "SayHello" {
		t.Fatalf("method name = %q, want %q", method.GetName(), "SayHello")
	}
	assertField(t, method.GetInputFields(), &holonsv1.FieldDoc{
		Name:        "name",
		Type:        "string",
		Number:      1,
		Description: "Name to greet.",
		Label:       holonsv1.FieldLabel_FIELD_LABEL_OPTIONAL,
		Required:    false,
		Example:     `"Bob"`,
	})
}

func TestBuildResponseFromProtoManifestSource(t *testing.T) {
	root := repoGabrielHolonDir(t)

	response, err := describe.BuildResponse(root, filepath.Join(root, "api", "v1", "holon.proto"))
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}

	if got := describeResponseSlug(response); got != "gabriel-greeting-go" {
		t.Fatalf("slug = %q, want %q", got, "gabriel-greeting-go")
	}
	if got := response.GetManifest().GetIdentity().GetMotto(); got != "Greets users in 56 languages — a Go daemon recipe example." {
		t.Fatalf("motto = %q", got)
	}

	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}
	service := response.GetServices()[0]
	if service.GetName() != "greeting.v1.GreetingService" {
		t.Fatalf("service name = %q, want %q", service.GetName(), "greeting.v1.GreetingService")
	}
	if len(service.GetMethods()) != 2 {
		t.Fatalf("methods len = %d, want 2", len(service.GetMethods()))
	}
}

func TestBuildResponseFromProtoDirWithLocalSharedProtos(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "api", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir api/v1: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "_protos", "holons", "v1"), 0o755); err != nil {
		t.Fatalf("mkdir _protos/holons/v1: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "api", "v1", "holon.proto"), []byte(`
syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "test-uuid"
    given_name: "Test"
    family_name: "Echo"
    motto: "Echoes staged imports."
    composer: "test"
    status: "draft"
    born: "2026-03-16"
  }
  lang: "go"
};

service Echo {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  string message = 1;
}

message PingResponse {
  string message = 1;
}
`), 0o644); err != nil {
		t.Fatalf("write holon.proto: %v", err)
	}

	manifestProto, err := os.ReadFile(repoManifestProtoPath(t))
	if err != nil {
		t.Fatalf("read manifest proto: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "_protos", "holons", "v1", "manifest.proto"), manifestProto, 0o644); err != nil {
		t.Fatalf("write shared manifest proto: %v", err)
	}

	response, err := describe.BuildResponse(root, filepath.Join(root, "api", "v1", "holon.proto"))
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}

	if got := describeResponseSlug(response); got != "test-echo" {
		t.Fatalf("slug = %q, want %q", got, "test-echo")
	}
	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}
	if got := response.GetServices()[0].GetName(); got != "test.v1.Echo" {
		t.Fatalf("service name = %q, want %q", got, "test.v1.Echo")
	}
}

func assertField(t *testing.T, fields []*holonsv1.FieldDoc, want *holonsv1.FieldDoc) {
	t.Helper()

	for _, field := range fields {
		if field.GetName() != want.GetName() {
			continue
		}
		if field.GetType() != want.GetType() {
			t.Fatalf("field %s type = %q, want %q", want.GetName(), field.GetType(), want.GetType())
		}
		if field.GetNumber() != want.GetNumber() {
			t.Fatalf("field %s number = %d, want %d", want.GetName(), field.GetNumber(), want.GetNumber())
		}
		if field.GetDescription() != want.GetDescription() {
			t.Fatalf("field %s description = %q, want %q", want.GetName(), field.GetDescription(), want.GetDescription())
		}
		if field.GetLabel() != want.GetLabel() {
			t.Fatalf("field %s label = %v, want %v", want.GetName(), field.GetLabel(), want.GetLabel())
		}
		if field.GetRequired() != want.GetRequired() {
			t.Fatalf("field %s required = %v, want %v", want.GetName(), field.GetRequired(), want.GetRequired())
		}
		if field.GetExample() != want.GetExample() {
			t.Fatalf("field %s example = %q, want %q", want.GetName(), field.GetExample(), want.GetExample())
		}
		return
	}

	t.Fatalf("field %q not found", want.GetName())
}

func describeResponseSlug(response *holonsv1.DescribeResponse) string {
	if response == nil {
		return ""
	}
	identity := response.GetManifest().GetIdentity()
	given := identity.GetGivenName()
	family := identity.GetFamilyName()
	if given == "" && family == "" {
		return ""
	}
	return strings.ToLower(strings.Trim(strings.ReplaceAll(given+"-"+strings.TrimSuffix(family, "?"), " ", "-"), "-"))
}

func echoHolonDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	copyTree(t, filepath.Join(describeTestdataRoot(t), "echoholon", "protos"), filepath.Join(root, "protos"))
	writeSharedManifestProto(t, root)

	manifest := fmt.Sprintf(`syntax = "proto3";

package echo.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: %q
    composer: "describe-test"
    status: "draft"
    born: "2026-03-17"
  }
  lang: "go"
};
`, "echo-server-0000", "Echo", "Server", "Reply precisely.")

	writeTestFile(t, filepath.Join(root, "protos", "echo", "v1", "holon.proto"), manifest)
	return root
}

func wrappedHolonDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	copyTree(t, filepath.Join(describeTestdataRoot(t), "wrappedholon", "protos"), filepath.Join(root, "protos"))
	copyTree(t, filepath.Join(describeTestdataRoot(t), "recipes"), filepath.Join(root, "recipes"))
	writeSharedManifestProto(t, root)

	manifest := fmt.Sprintf(`syntax = "proto3";

package greeting.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: %q
    given_name: %q
    family_name: %q
    motto: %q
    composer: "describe-test"
    status: "draft"
    born: "2026-03-17"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "go-module"
  }
  artifacts: {
    binary: "wrapped-greeter"
  }
};
`, "wrapped-greeter-0000-0000-0000-000000000001", "wrapped", "Greeter", "Wraps a shared proto.")

	writeTestFile(t, filepath.Join(root, "protos", "greeting", "v1", "holon.proto"), manifest)
	return root
}

func describeTestdataRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func repoGabrielHolonDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "examples", "hello-world", "gabriel-greeting-go")
}

func repoManifestProtoPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "identity", "testdata", "_protos", "holons", "v1", "manifest.proto")
}

func writeSharedManifestProto(t *testing.T, root string) {
	t.Helper()

	data, err := os.ReadFile(repoManifestProtoPath(t))
	if err != nil {
		t.Fatalf("read manifest proto: %v", err)
	}
	writeTestFile(t, filepath.Join(root, "_protos", "holons", "v1", "manifest.proto"), string(data))
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()

	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}

func writeTestFile(t *testing.T, path string, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

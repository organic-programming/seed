package identity_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/organic-programming/go-holons/pkg/identity"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestResolveManifest_ProtoFirst(t *testing.T) {
	dir := filepath.Join(testdataDir(), "protoholon")

	id, source, err := identity.ResolveManifest(dir)
	if err != nil {
		t.Fatalf("ResolveManifest returned error: %v", err)
	}

	if id.UUID != "test-uuid-1234" {
		t.Errorf("UUID = %q, want %q", id.UUID, "test-uuid-1234")
	}
	if id.GivenName != "gabriel" {
		t.Errorf("GivenName = %q, want %q", id.GivenName, "gabriel")
	}
	if id.FamilyName != "Greeting-Go" {
		t.Errorf("FamilyName = %q, want %q", id.FamilyName, "Greeting-Go")
	}
	if id.Motto != "Test greeting holon." {
		t.Errorf("Motto = %q, want %q", id.Motto, "Test greeting holon.")
	}
	if id.Lang != "go" {
		t.Errorf("Lang = %q, want %q", id.Lang, "go")
	}
	if id.Reproduction != "assisted" {
		t.Errorf("Reproduction = %q, want %q", id.Reproduction, "assisted")
	}
	if id.GeneratedBy != "op" {
		t.Errorf("GeneratedBy = %q, want %q", id.GeneratedBy, "op")
	}
	if len(id.Parents) != 1 || id.Parents[0] != "parent-a" {
		t.Errorf("Parents = %v, want [parent-a]", id.Parents)
	}
	if got := id.Slug(); got != "gabriel-greeting-go" {
		t.Errorf("Slug() = %q, want %q", got, "gabriel-greeting-go")
	}
	if !filepath.IsAbs(source) {
		t.Errorf("source should be absolute, got %q", source)
	}
	t.Logf("resolved from: %s", source)
}

func TestResolveManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()

	_, _, err := identity.ResolveManifest(dir)
	if err == nil {
		t.Fatal("ResolveManifest should return error for empty directory")
	}
}

func TestResolveManifest_SourceProtosBeatStaleStagedProtos(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	protoDir := filepath.Join(appDir, "api", "v1")
	sourceProtoDir := filepath.Join(root, "_protos", "holons", "v1")
	stagedProtoDir := filepath.Join(appDir, ".op", "protos", "holons", "v1")

	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceProtoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagedProtoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sourceProtoDir, "manifest.proto"), []byte(testManifestProto(true)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedProtoDir, "manifest.proto"), []byte(testManifestProto(false)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(protoDir, "holon.proto"), []byte(`syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    uuid: "test-uuid-staged"
    given_name: "Staged"
    family_name: "Proto"
  }
  build: {
    targets: {
      key: "default"
      value: {
        steps: {
          copy_all_holons: {
            to: "app/Holons"
          }
        }
      }
    }
  }
};
`), 0o644); err != nil {
		t.Fatal(err)
	}

	id, _, err := identity.ResolveManifest(appDir)
	if err != nil {
		t.Fatalf("ResolveManifest returned error: %v", err)
	}
	if got := id.Slug(); got != "staged-proto" {
		t.Fatalf("Slug() = %q, want staged-proto", got)
	}
}

func testManifestProto(withCopyAllHolons bool) string {
	copyAllHolonsField := ""
	copyAllHolonsMessage := ""
	if withCopyAllHolons {
		copyAllHolonsField = "      CopyAllHolons copy_all_holons = 6;\n"
		copyAllHolonsMessage = "    message CopyAllHolons { string to = 1; }\n"
	}
	return `syntax = "proto3";

package holons.v1;

import "google/protobuf/descriptor.proto";

message HolonManifest {
  message Identity {
    string uuid = 2;
    string given_name = 3;
    string family_name = 4;
  }
  message Build {
    map<string, Target> targets = 5;
    message Target { repeated Step steps = 1; }
  }
  message Step {
    oneof action {
` + copyAllHolonsField + `    }
` + copyAllHolonsMessage + `  }
  Identity identity = 1;
  Build build = 10;
}

extend google.protobuf.FileOptions {
  HolonManifest manifest = 50000;
}
`
}

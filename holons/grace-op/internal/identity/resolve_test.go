package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFromProtoFileParsesCopyArtifactStep(t *testing.T) {
	root := t.TempDir()
	writeSharedManifestProto(t, root)

	protoDir := filepath.Join(root, "app", "api", "v1")
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	protoPath := filepath.Join(protoDir, ProtoManifestFileName)
	proto := `syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "11111111-2222-3333-4444-555555555555"
    given_name: "Proto"
    family_name: "Artifact"
    motto: "Parses copy_artifact steps."
    composer: "test"
    status: "draft"
    born: "2026-03-16"
  }
  kind: "composite"
  build: {
    runner: "recipe"
    members: { id: "holon" path: "../holon" type: "holon" }
    members: { id: "app" path: "." type: "component" }
    targets: {
      key: "macos"
      value: {
        steps: { build_member: "holon" parallel: true }
        steps: {
          copy_artifact: {
            from: "holon"
            to: "build/MyApp.app/Contents/Resources/Holons/holon.holon"
          }
        }
      }
    }
  }
  artifacts: {
    primary: "build/MyApp.app"
  }
};
`
	if err := os.WriteFile(protoPath, []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveFromProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ResolveFromProtoFile failed: %v", err)
	}

	target := resolved.BuildTargets["macos"]
	if len(target.Steps) != 2 {
		t.Fatalf("len(target.Steps) = %d, want 2", len(target.Steps))
	}
	if !target.Steps[0].Parallel {
		t.Fatal("expected build_member parallel flag to be resolved")
	}
	if target.Steps[1].CopyArtifact == nil {
		t.Fatal("expected copy_artifact step to be resolved")
	}
	if got := target.Steps[1].CopyArtifact.From; got != "holon" {
		t.Fatalf("CopyArtifact.From = %q, want holon", got)
	}
	if got := target.Steps[1].CopyArtifact.To; got != "build/MyApp.app/Contents/Resources/Holons/holon.holon" {
		t.Fatalf("CopyArtifact.To = %q", got)
	}
}

func TestWriteHolonProtoRoundTripsSupportedIdentityFields(t *testing.T) {
	root := t.TempDir()
	writeSharedManifestProto(t, root)

	protoPath := filepath.Join(root, "alpha", ManifestFileName)
	id := Identity{
		Schema:       "holon/v0",
		UUID:         "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		GivenName:    "Alpha",
		FamilyName:   "Writer",
		Motto:        "Writes proto manifests.",
		Composer:     "test",
		Clade:        "deterministic/pure",
		Status:       "draft",
		Born:         "2026-03-17",
		Parents:      []string{"parent-a"},
		Reproduction: "manual",
		Aliases:      []string{"alpha", "writer"},
		GeneratedBy:  "op",
		Lang:         "go",
		ProtoStatus:  "draft",
		Description:  "Proto-backed identity.",
	}

	if err := WriteHolonProto(id, protoPath); err != nil {
		t.Fatalf("WriteHolonProto failed: %v", err)
	}

	data, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", protoPath, err)
	}
	text := string(data)
	if !strings.Contains(text, `syntax = "proto3";`) {
		t.Fatalf("proto file missing syntax header: %s", text)
	}
	if !strings.Contains(text, `import "holons/v1/manifest.proto";`) {
		t.Fatalf("proto file missing manifest import: %s", text)
	}
	if !strings.Contains(text, `schema: "holon/v1"`) {
		t.Fatalf("proto file missing normalized schema: %s", text)
	}
	for _, legacyField := range []string{"clade:", "parents:", "reproduction:", "generated_by:", "proto_status:"} {
		if strings.Contains(text, legacyField) {
			t.Fatalf("proto file unexpectedly contains %q: %s", legacyField, text)
		}
	}

	resolved, err := ResolveFromProtoFile(protoPath)
	if err != nil {
		t.Fatalf("ResolveFromProtoFile failed: %v", err)
	}

	if got := resolved.Identity.Schema; got != "holon/v1" {
		t.Fatalf("Schema = %q, want %q", got, "holon/v1")
	}
	if got := resolved.Identity.UUID; got != id.UUID {
		t.Fatalf("UUID = %q, want %q", got, id.UUID)
	}
	if got := resolved.Identity.GivenName; got != id.GivenName {
		t.Fatalf("GivenName = %q, want %q", got, id.GivenName)
	}
	if got := resolved.Identity.FamilyName; got != id.FamilyName {
		t.Fatalf("FamilyName = %q, want %q", got, id.FamilyName)
	}
	if got := resolved.Identity.Motto; got != id.Motto {
		t.Fatalf("Motto = %q, want %q", got, id.Motto)
	}
	if got := resolved.Identity.Composer; got != id.Composer {
		t.Fatalf("Composer = %q, want %q", got, id.Composer)
	}
	if got := resolved.Identity.Status; got != id.Status {
		t.Fatalf("Status = %q, want %q", got, id.Status)
	}
	if got := resolved.Identity.Born; got != id.Born {
		t.Fatalf("Born = %q, want %q", got, id.Born)
	}
	if got := strings.Join(resolved.Identity.Aliases, ","); got != strings.Join(id.Aliases, ",") {
		t.Fatalf("Aliases = %q, want %q", got, strings.Join(id.Aliases, ","))
	}
	if got := resolved.Identity.Lang; got != id.Lang {
		t.Fatalf("Lang = %q, want %q", got, id.Lang)
	}
	if got := resolved.Description; got != id.Description {
		t.Fatalf("Description = %q, want %q", got, id.Description)
	}
	if resolved.Identity.Clade != "" || len(resolved.Identity.Parents) != 0 || resolved.Identity.Reproduction != "" || resolved.Identity.GeneratedBy != "" || resolved.Identity.ProtoStatus != "" {
		t.Fatalf("legacy identity fields should not round-trip through holon.proto: %+v", resolved.Identity)
	}
}

func TestRegistryScansProtoFilesAndFindsByUUID(t *testing.T) {
	root := t.TempDir()
	writeSharedManifestProto(t, root)

	alphaPath := filepath.Join(root, "alpha", ManifestFileName)
	betaPath := filepath.Join(root, "beta", ManifestFileName)
	if err := WriteHolonProto(Identity{
		UUID:       "11111111-2222-3333-4444-555555555555",
		GivenName:  "Alpha",
		FamilyName: "Holon",
		Motto:      "Alpha holon.",
		Composer:   "test",
		Status:     "draft",
		Born:       "2026-03-17",
	}, alphaPath); err != nil {
		t.Fatalf("WriteHolonProto(alpha) failed: %v", err)
	}
	if err := WriteHolonProto(Identity{
		UUID:       "99999999-8888-7777-6666-555555555555",
		GivenName:  "Beta",
		FamilyName: "Holon",
		Motto:      "Beta holon.",
		Composer:   "test",
		Status:     "draft",
		Born:       "2026-03-17",
	}, betaPath); err != nil {
		t.Fatalf("WriteHolonProto(beta) failed: %v", err)
	}

	identities, err := FindAll(root)
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}
	if len(identities) != 2 {
		t.Fatalf("FindAll returned %d identities, want 2", len(identities))
	}

	located, err := FindAllWithPaths(root)
	if err != nil {
		t.Fatalf("FindAllWithPaths failed: %v", err)
	}
	if len(located) != 2 {
		t.Fatalf("FindAllWithPaths returned %d identities, want 2", len(located))
	}
	foundPaths := map[string]bool{}
	for _, entry := range located {
		foundPaths[entry.Path] = true
	}
	if !foundPaths[alphaPath] || !foundPaths[betaPath] {
		t.Fatalf("FindAllWithPaths returned unexpected paths: %+v", foundPaths)
	}

	streamed := make([]string, 0, 2)
	progressEvents := make([]ScanProgress, 0, 1)
	if err := ScanAllWithPaths(root, 1, func(h LocatedIdentity) {
		streamed = append(streamed, h.Identity.UUID)
	}, func(progress ScanProgress) {
		progressEvents = append(progressEvents, progress)
	}); err != nil {
		t.Fatalf("ScanAllWithPaths failed: %v", err)
	}
	if len(streamed) != 2 {
		t.Fatalf("ScanAllWithPaths streamed %d identities, want 2", len(streamed))
	}
	if len(progressEvents) == 0 {
		t.Fatal("ScanAllWithPaths did not report progress")
	}
	lastProgress := progressEvents[len(progressEvents)-1]
	if lastProgress.HolonsFound != 2 || lastProgress.ScannedFiles == 0 {
		t.Fatalf("final progress = %+v, want HolonsFound=2 and ScannedFiles>0", lastProgress)
	}

	found, err := FindByUUID(root, "11111111-2222")
	if err != nil {
		t.Fatalf("FindByUUID failed: %v", err)
	}
	if found != alphaPath {
		t.Fatalf("FindByUUID returned %q, want %q", found, alphaPath)
	}
}

func TestResolveUsesHolonProtoOnly(t *testing.T) {
	root := t.TempDir()
	otherPath := filepath.Join(root, "manifest.txt")
	if err := os.WriteFile(otherPath, []byte("schema: holon/v0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", otherPath, err)
	}

	_, err := Resolve(root)
	if err == nil {
		t.Fatal("Resolve succeeded without holon.proto")
	}
	if !strings.Contains(err.Error(), ProtoManifestFileName) {
		t.Fatalf("Resolve error = %q, want mention of %q", err.Error(), ProtoManifestFileName)
	}
}

func writeSharedManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join("..", "..", "..", "..", "holons", "grace-op", "_protos", "holons", "v1", "manifest.proto")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", source, err)
	}

	targetDir := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", targetDir, err)
	}
	target := filepath.Join(targetDir, "manifest.proto")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", target, err)
	}
}

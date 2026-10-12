package holons

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func cargoTargetFixture(t *testing.T, root string) *LoadedManifest {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunnerManifest(t, root, "schema: holon/v0\nkind: native\nbuild:\n  runner: cargo\nartifacts:\n  binary: cargo-demo\n")
	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestWindowsCargoTargetSeparatesWorkspaceAndSnapshot(t *testing.T) {
	t.Setenv("OPPATH", t.TempDir())
	live := cargoTargetFixture(t, filepath.Join(t.TempDir(), "live"))
	snapshot := cargoTargetFixture(t, filepath.Join(t.TempDir(), "snapshot"))
	if cargoTargetDir(live) == cargoTargetDir(snapshot) {
		t.Fatal("snapshot reused live Cargo target")
	}
	if cargoTargetDir(live) != cargoTargetDir(live) {
		t.Fatal("unstable Cargo target")
	}
	args := []string{"cargo", "run", "--target-dir", ".op/build/cargo", "--bin", "package-runtime"}
	got := cargoHookArgs(live, live.Dir, args)
	if got[3] != cargoTargetDir(live) || args[3] != ".op/build/cargo" {
		t.Fatalf("hook normalization/mutation: %v / %v", got, args)
	}
	equals := []string{"cargo", "run", "--target-dir=.op/build/cargo"}
	if got := cargoHookArgs(live, live.Dir, equals)[2]; got != "--target-dir="+cargoTargetDir(live) {
		t.Fatal(got)
	}
	other := []string{"cargo", "run", "--target-dir", "custom"}
	if !reflect.DeepEqual(cargoHookArgs(live, live.Dir, other), other) {
		t.Fatal("custom target changed")
	}
	if got := cargoTargetEnv(live); !reflect.DeepEqual(got, []string{"CARGO_TARGET_DIR=" + cargoTargetDir(live), "OP_CARGO_TARGET_DIR=" + cargoTargetDir(live)}) {
		t.Fatal(got)
	}
}

func TestWindowsCargoCleanRequiresMatchingOwner(t *testing.T) {
	t.Setenv("OPPATH", t.TempDir())
	manifest := cargoTargetFixture(t, t.TempDir())
	if err := prepareCargoTargetDir(manifest); err != nil {
		t.Fatal(err)
	}
	root := cargoTargetDir(manifest)
	sentinel := filepath.Join(root, "preserve")
	if err := os.WriteFile(sentinel, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".op-source"), []byte("another source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanCargoTargetDir(manifest); err == nil {
		t.Fatal("clean accepted the wrong owner")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("wrong-owner clean removed data")
	}
	if err := os.WriteFile(filepath.Join(root, ".op-source"), []byte(filepath.Clean(manifest.Dir)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cleanCargoTargetDir(manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("owned target remains after clean")
	}
}

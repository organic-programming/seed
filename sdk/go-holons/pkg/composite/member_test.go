package composite

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMemberResolvesEmbeddedBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "observability-cascade-go.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH)
	self := filepath.Join(binDir, "observability-cascade-go")
	memberDir := filepath.Join(binDir, "holons", "node-a")
	member := filepath.Join(memberDir, "observability-cascade-go-node-a")

	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(self, []byte("composite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "README.txt"), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(member, []byte("member"), 0o755); err != nil {
		t.Fatal(err)
	}

	restore := stubExecutablePath(t, self)
	defer restore()

	got, err := Member("node-a")
	if err != nil {
		t.Fatalf("Member returned error: %v", err)
	}
	if got != member {
		t.Fatalf("Member = %q, want %q", got, member)
	}
}

func TestMemberRejectsEmptyID(t *testing.T) {
	if _, err := Member(" "); err == nil {
		t.Fatal("Member should reject an empty id")
	}
}

func TestMemberReportsMissingExecutable(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "composite.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH)
	self := filepath.Join(binDir, "composite")
	memberDir := filepath.Join(binDir, "holons", "node-a")

	if err := os.MkdirAll(memberDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(self, []byte("composite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberDir, "member.txt"), []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := stubExecutablePath(t, self)
	defer restore()

	if _, err := Member("node-a"); err == nil {
		t.Fatal("Member should report missing executable")
	}
}

func TestMemberPropagatesExecutableError(t *testing.T) {
	want := errors.New("boom")
	old := executablePath
	executablePath = func() (string, error) {
		return "", want
	}
	t.Cleanup(func() {
		executablePath = old
	})

	_, err := Member("node-a")
	if !errors.Is(err, want) {
		t.Fatalf("Member error = %v, want %v", err, want)
	}
}

func stubExecutablePath(t *testing.T, path string) func() {
	t.Helper()
	old := executablePath
	executablePath = func() (string, error) {
		return path, nil
	}
	return func() {
		executablePath = old
	}
}

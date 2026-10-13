//go:build windows && go1.25

package holons

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func mapDescribeOutput(t *testing.T, path string, size int) []byte {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(file) })
	mapping, err := windows.CreateFileMapping(file, nil, windows.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(mapping) })
	address, err := windows.MapViewOfFile(mapping, windows.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.UnmapViewOfFile(address) })
	return unsafe.Slice((*byte)(unsafe.Pointer(address)), size)
}

func TestWindowsDescribeOutputMappedSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "describe_generated.rs")
	original := []byte("// original generated source\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	oldView := mapDescribeOutput(t, path, len(original))
	// Reproduce the real failure, rather than merely simulating an error.
	if err := os.WriteFile(path, original, 0o644); !errors.Is(err, windows.ERROR_USER_MAPPED_FILE) {
		t.Fatalf("mapped truncation = %v, want ERROR_USER_MAPPED_FILE", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeDescribeOutput(path, original, 0o644); err != nil {
		t.Fatalf("identical mapped content: %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("identical generation changed file identity or time: %v", err)
	}
	changed := []byte("// changed source is deliberately a different length\n")
	if err := writeDescribeOutput(path, changed, 0o644); err != nil {
		t.Fatalf("replace mapped content: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, changed) || !bytes.Equal(oldView, original) {
		t.Fatalf("replacement did not separate new bytes and existing mapping: %v", err)
	}
	newView := mapDescribeOutput(t, path, len(changed))
	if err := writeDescribeOutput(path, original, 0o644); err != nil {
		t.Fatalf("restore while generated output is mapped: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) || !bytes.Equal(newView, changed) {
		t.Fatalf("restoration changed a reader's mapping: %v", err)
	}
	assertNoDescribeStagingFiles(t, root)
}

func TestWindowsDescribeOutputDeniedReplacePreservesSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "describe_generated.rs")
	original := []byte("// previous source\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(locked) })
	if err := writeDescribeOutput(path, original, 0o644); err != nil {
		t.Fatalf("identical locked content should need no write: %v", err)
	}
	if err := writeDescribeOutput(path, []byte("different"), 0o644); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("replacement without delete sharing = %v, want sharing/access error", err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("failed replacement damaged source: %v", err)
	}
	assertNoDescribeStagingFiles(t, root)
}

func TestWindowsDescribeOutputGenerationRestoresMappedSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "describe_generated.rs")
	templatePath := filepath.Join(root, "describe.rs.tmpl")
	original, generated := []byte("// original\n"), []byte("// generated\n")
	for path, data := range map[string][]byte{path: original, templatePath: generated} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldView := mapDescribeOutput(t, path, len(original))
	restore, err := writeDescribeSource(templatePath, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, generated) {
		t.Fatalf("generation: %q, %v", got, err)
	}
	generatedView := mapDescribeOutput(t, path, len(generated))
	restore()
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) || !bytes.Equal(oldView, original) || !bytes.Equal(generatedView, generated) {
		t.Fatalf("generation rollback: %q, %v", got, err)
	}
	assertNoDescribeStagingFiles(t, root)
}

func assertNoDescribeStagingFiles(t *testing.T, root string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(root, ".op-describe-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("staging files remain: %v, %v", leftovers, err)
	}
}

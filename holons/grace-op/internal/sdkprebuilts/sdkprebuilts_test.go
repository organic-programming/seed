package sdkprebuilts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testTarget = "x86_64-unknown-linux-gnu"

func TestInstallPathUsesOPPATH(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	got := InstallPath("cpp", "1.80.0", testTarget)
	want := filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget)
	if got != want {
		t.Fatalf("InstallPath() = %q, want %q", got, want)
	}
}

func TestInstallLocalTarballVerifiesAndLocates(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	source := filepath.Join(t.TempDir(), "cpp-1.80.0-"+testTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/grpc/grpc.h": {Mode: 0o644, Body: []byte("/* grpc */\n")},
		"lib/libgrpc.a":       {Mode: 0o644, Body: []byte("archive\n")},
		"bin/protoc":          {Mode: 0o755, Body: []byte("#!/bin/sh\n")},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "cpp",
		Target: testTarget,
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Install() notes = %#v, want none", notes)
	}
	if prebuilt.Version != "1.80.0" {
		t.Fatalf("installed version = %q, want 1.80.0", prebuilt.Version)
	}
	if prebuilt.Path != filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget) {
		t.Fatalf("installed path = %q", prebuilt.Path)
	}
	for _, rel := range []string{"include/grpc/grpc.h", "lib/libgrpc.a", "bin/protoc", metadataFile} {
		if _, err := os.Stat(filepath.Join(prebuilt.Path, rel)); err != nil {
			t.Fatalf("installed file %s missing: %v", rel, err)
		}
	}

	verified, ok, err := Verify(QueryOptions{Lang: "cpp", Target: testTarget})
	if err != nil {
		t.Fatalf("Verify() returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Verify() ok = false, want true")
	}
	if verified.Path != prebuilt.Path {
		t.Fatalf("Verify() path = %q, want %q", verified.Path, prebuilt.Path)
	}

	located, err := Locate(QueryOptions{Lang: "cpp", Target: testTarget})
	if err != nil {
		t.Fatalf("Locate() returned error: %v", err)
	}
	if located.Path != prebuilt.Path {
		t.Fatalf("Locate() path = %q, want %q", located.Path, prebuilt.Path)
	}
}

func TestListInstalledIteratesRuntimeTree(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	writeInstalledMetadata(t, Prebuilt{
		Lang:          "ruby",
		Version:       "1.58.3",
		Target:        testTarget,
		Path:          filepath.Join(runtimeHome, "sdk", "ruby", "1.58.3", testTarget),
		ArchiveSHA256: "bbbb",
		Installed:     true,
	})
	writeInstalledMetadata(t, Prebuilt{
		Lang:          "cpp",
		Version:       "1.80.0",
		Target:        testTarget,
		Path:          filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", testTarget),
		ArchiveSHA256: "aaaa",
		Installed:     true,
	})
	if err := os.MkdirAll(filepath.Join(runtimeHome, "sdk", "cpp", "1.80.0", "ios-arm64"), 0o755); err != nil {
		t.Fatalf("create unsupported target dir: %v", err)
	}

	entries, err := ListInstalled("")
	if err != nil {
		t.Fatalf("ListInstalled() returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListInstalled() returned %d entries, want 2: %#v", len(entries), entries)
	}
	if entries[0].Lang != "cpp" || entries[1].Lang != "ruby" {
		t.Fatalf("entries sorted by lang = %#v", entries)
	}
	if entries[0].ArchiveSHA256 != "aaaa" || entries[1].ArchiveSHA256 != "bbbb" {
		t.Fatalf("metadata not loaded: %#v", entries)
	}

	filtered, err := ListInstalled("ruby")
	if err != nil {
		t.Fatalf("ListInstalled(ruby) returned error: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Lang != "ruby" {
		t.Fatalf("ListInstalled(ruby) = %#v", filtered)
	}
}

func TestNormalizeVersionRejectsPathTraversal(t *testing.T) {
	for _, version := range []string{"../1.80.0", `..\1.80.0`, ".", "1:80:0"} {
		if _, err := NormalizeVersion(version); err == nil {
			t.Fatalf("NormalizeVersion(%q) returned nil error", version)
		}
	}
}

type testTarEntry struct {
	Mode int64
	Body []byte
}

func writeTestTarGz(t *testing.T, path string, entries map[string]testTarEntry) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tarball: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, entry := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: entry.Mode,
			Size: int64(len(entry.Body)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(entry.Body); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close tarball: %v", err)
	}
}

func writeSHA256Sidecar(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tarball for sha256: %v", err)
	}
	sum := sha256.Sum256(data)
	if err := os.WriteFile(path+".sha256", []byte(hex.EncodeToString(sum[:])+"  "+filepath.Base(path)+"\n"), 0o644); err != nil {
		t.Fatalf("write sha256 sidecar: %v", err)
	}
}

func writeInstalledMetadata(t *testing.T, prebuilt Prebuilt) {
	t.Helper()

	if err := os.MkdirAll(prebuilt.Path, 0o755); err != nil {
		t.Fatalf("create install dir: %v", err)
	}
	data, err := json.Marshal(prebuilt)
	if err != nil {
		t.Fatalf("marshal prebuilt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prebuilt.Path, metadataFile), data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}

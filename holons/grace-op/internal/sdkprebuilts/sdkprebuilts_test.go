package sdkprebuilts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestInstallLocalHolonsReleaseTarballInfersVersion(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	source := filepath.Join(t.TempDir(), "zig-holons-v0.1.0-"+testTarget+".tar.gz")
	writeTestTarGz(t, source, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_zig.a":  {Mode: 0o644, Body: []byte("archive\n")},
	})
	writeSHA256Sidecar(t, source)

	prebuilt, _, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Target: testTarget,
		Source: source,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if prebuilt.Version != "0.1.0" {
		t.Fatalf("installed version = %q, want 0.1.0", prebuilt.Version)
	}
	if prebuilt.Path != filepath.Join(runtimeHome, "sdk", "zig", "0.1.0", testTarget) {
		t.Fatalf("installed path = %q", prebuilt.Path)
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

func TestListAvailableReadsGitHubReleases(t *testing.T) {
	server := newReleaseServer(t, map[string][]byte{})
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	entries, notes, err := ListAvailable(" zig ")
	if err != nil {
		t.Fatalf("ListAvailable() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %#v, want three zig artifacts", entries)
	}
	if entries[0].Lang != "zig" || entries[0].Version != "0.1.1" || entries[0].Target != "aarch64-apple-darwin" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[0].Source == "" {
		t.Fatalf("first entry has empty source")
	}
}

func TestInstallWithoutSourceResolvesLatestRelease(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	assets := map[string][]byte{}
	archiveName := "zig-holons-v0.1.1-" + testTarget + ".tar.gz"
	archivePath := filepath.Join(t.TempDir(), archiveName)
	writeTestTarGz(t, archivePath, map[string]testTarEntry{
		"include/holons_sdk.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_zig.a":  {Mode: 0o644, Body: []byte("archive\n")},
	})
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	assets["/assets/"+archiveName] = archiveData
	assets["/assets/"+archiveName+".sha256"] = []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n")

	server := newReleaseServer(t, assets)
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "zig",
		Target: testTarget,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if prebuilt.Version != "0.1.1" {
		t.Fatalf("version = %q, want 0.1.1", prebuilt.Version)
	}
	if prebuilt.Source != server.URL+"/assets/"+archiveName {
		t.Fatalf("source = %q", prebuilt.Source)
	}
	if _, err := os.Stat(filepath.Join(prebuilt.Path, "lib/libholons_zig.a")); err != nil {
		t.Fatalf("installed archive missing: %v", err)
	}
}

func TestListAvailableReadsReleaseManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "release-manifest.json", "browser_download_url": "%s/assets/release-manifest.json"},
      {"name": "cpp-holons-v1.80.0-%s.tar.gz", "browser_download_url": "%s/assets/legacy.tar.gz"}
    ]
  }
]`, serverURL(r), testTarget, serverURL(r))
		case "/assets/release-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
  "schema": "sdk-prebuilts-release-manifest/v1",
  "sdk": "cpp",
  "version": "1.80.0",
  "tag": "cpp-holons-v1.80.0",
  "artifacts": [
    {
      "target": "%s",
      "archive": {
        "name": "cpp-holons-v1.80.0-%s.tar.gz",
        "url": "%s/assets/from-manifest.tar.gz",
        "sha256": "abc123"
      },
      "debug": {
        "name": "cpp-holons-v1.80.0-%s-debug.tar.gz",
        "url": "%s/assets/from-manifest-debug.tar.gz",
        "sha256": "def456"
      },
      "sbom": {
        "name": "cpp-holons-v1.80.0-%s.tar.gz.spdx.json",
        "url": "%s/assets/from-manifest.tar.gz.spdx.json",
        "sha256": "789abc"
      }
    }
  ]
}`, testTarget, testTarget, serverURL(r), testTarget, serverURL(r), testTarget, serverURL(r))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	entries, notes, err := ListAvailable("cpp")
	if err != nil {
		t.Fatalf("ListAvailable() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %#v, want one manifest artifact", entries)
	}
	if entries[0].Source != server.URL+"/assets/from-manifest.tar.gz" {
		t.Fatalf("source = %q, want manifest URL", entries[0].Source)
	}
	if entries[0].ArchiveSHA256 != "abc123" {
		t.Fatalf("archive sha = %q, want abc123", entries[0].ArchiveSHA256)
	}
}

func TestInstallWithoutSourceUsesReleaseManifestSHA(t *testing.T) {
	runtimeHome := t.TempDir()
	t.Setenv("OPPATH", runtimeHome)

	archiveName := "cpp-holons-v1.80.0-" + testTarget + ".tar.gz"
	archivePath := filepath.Join(t.TempDir(), archiveName)
	writeTestTarGz(t, archivePath, map[string]testTarEntry{
		"include/holons.grpc.pb.h": {Mode: 0o644, Body: []byte("/* holons */\n")},
		"lib/libholons_cpp.a":      {Mode: 0o644, Body: []byte("archive\n")},
		"bin/protoc":               {Mode: 0o755, Body: []byte("#!/bin/sh\n")},
	})
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	sum := sha256.Sum256(archiveData)
	expectedSHA := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `[
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "release-manifest.json", "browser_download_url": "%s/assets/release-manifest.json"}
    ]
  }
]`, serverURL(r))
		case "/assets/release-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
  "schema": "sdk-prebuilts-release-manifest/v1",
  "sdk": "cpp",
  "version": "1.80.0",
  "tag": "cpp-holons-v1.80.0",
  "artifacts": [
    {
      "target": "%s",
      "archive": {
        "name": "%s",
        "url": "%s/assets/%s",
        "sha256": "%s"
      }
    }
  ]
}`, testTarget, archiveName, serverURL(r), archiveName, expectedSHA)
		case "/assets/" + archiveName:
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(releasesAPIEnv, server.URL+"/releases")

	prebuilt, notes, err := Install(context.Background(), InstallOptions{
		Lang:   "cpp",
		Target: testTarget,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("notes = %#v, want none", notes)
	}
	if prebuilt.Source != server.URL+"/assets/"+archiveName {
		t.Fatalf("source = %q", prebuilt.Source)
	}
	if prebuilt.ArchiveSHA256 != expectedSHA {
		t.Fatalf("archive sha = %q, want %q", prebuilt.ArchiveSHA256, expectedSHA)
	}
	if _, err := os.Stat(filepath.Join(prebuilt.Path, "bin/protoc")); err != nil {
		t.Fatalf("installed archive missing: %v", err)
	}
}

func TestNormalizeVersionRejectsPathTraversal(t *testing.T) {
	for _, version := range []string{"../1.80.0", `..\1.80.0`, ".", "1:80:0"} {
		if _, err := NormalizeVersion(version); err == nil {
			t.Fatalf("NormalizeVersion(%q) returned nil error", version)
		}
	}
}

func TestHostTripletForWindowsUsesTransitionalGNU(t *testing.T) {
	got, err := HostTripletFor("windows", "amd64", false)
	if err != nil {
		t.Fatalf("HostTripletFor() returned error: %v", err)
	}
	if got != "x86_64-windows-gnu" {
		t.Fatalf("HostTripletFor(windows, amd64) = %q, want x86_64-windows-gnu", got)
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

func newReleaseServer(t *testing.T, assets map[string][]byte) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := assets[r.URL.Path]; ok {
			_, _ = w.Write(data)
			return
		}
		if r.URL.Path != "/releases" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[
  {
    "tag_name": "zig-holons-v0.1.0",
    "draft": false,
    "assets": [
      {"name": "zig-holons-v0.1.0-%s.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s.tar.gz"},
      {"name": "zig-holons-v0.1.0-%s.tar.gz.sha256", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s.tar.gz.sha256"},
      {"name": "zig-holons-v0.1.0-%s-debug.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.0-%s-debug.tar.gz"}
    ]
  },
  {
    "tag_name": "zig-holons-v0.1.1",
    "draft": false,
    "assets": [
      {"name": "zig-holons-v0.1.1-%s.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.1-%s.tar.gz"},
      {"name": "zig-holons-v0.1.1-aarch64-apple-darwin.tar.gz", "browser_download_url": "%s/assets/zig-holons-v0.1.1-aarch64-apple-darwin.tar.gz"}
    ]
  },
  {
    "tag_name": "cpp-holons-v1.80.0",
    "draft": false,
    "assets": [
      {"name": "cpp-holons-v1.80.0-%s.tar.gz", "browser_download_url": "%s/assets/cpp-holons-v1.80.0-%s.tar.gz"}
    ]
  }
]`, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, testTarget, serverURL(r), testTarget, serverURL(r), testTarget, serverURL(r), testTarget)
	}))
	return server
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

package holons

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsCRTRedistUsesSelectedVisualStudioOrExplicitOverride(t *testing.T) {
	instance := t.TempDir()
	versionPath := filepath.Join(instance, "VC", "Auxiliary", "Build", "Microsoft.VCRedistVersion.default.txt")
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte("14.44.35208\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(instance, "VC", "Redist", "MSVC", "14.44.35208", "x64", "Microsoft.VC143.CRT")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"VSINSTALLDIR", "VCINSTALLDIR", "vswhere"} {
		t.Run(selection, func(t *testing.T) {
			getenv := func(name string) string {
				if name == selection {
					if name == "VCINSTALLDIR" {
						return filepath.Join(instance, "VC") + string(filepath.Separator)
					}
					return instance
				}
				return ""
			}
			got, err := windowsCRTRedistDirectory(getenv, func() (string, error) {
				if selection != "vswhere" {
					t.Fatal("overrode the selected Visual Studio instance")
				}
				return instance, nil
			}, "amd64")
			if err != nil || got != want {
				t.Fatalf("redist=%q error=%v, want %q", got, err, want)
			}
		})
	}
	explicit := t.TempDir()
	got, err := windowsCRTRedistDirectory(func(name string) string {
		if name == "OP_WINDOWS_CRT_DIR" {
			return explicit
		}
		return instance
	}, func() (string, error) { t.Fatal("discovered VS despite explicit override"); return "", nil }, "amd64")
	if err != nil || got != explicit {
		t.Fatalf("explicit redist=%q error=%v", got, err)
	}
}

func TestWindowsCRTRedistUsesNewToolsetFolderAndRejectsAmbiguity(t *testing.T) {
	instance := t.TempDir()
	versionPath := filepath.Join(instance, "VC", "Auxiliary", "Build", "Microsoft.VCRedistVersion.default.txt")
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte("14.50.35717\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(instance, "VC", "Redist", "MSVC", "14.50.35717", "x64")
	want := filepath.Join(parent, "Microsoft.VC145.CRT")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	getenv := func(string) string { return "" }
	findVS := func() (string, error) { return instance, nil }
	got, err := windowsCRTRedistDirectory(getenv, findVS, "amd64")
	if err != nil || got != want {
		t.Fatalf("new toolset redist=%q error=%v, want %q", got, err, want)
	}
	if err := os.MkdirAll(filepath.Join(parent, "Microsoft.VC143.CRT"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := windowsCRTRedistDirectory(getenv, findVS, "amd64"); err == nil {
		t.Fatal("selected an arbitrary CRT from an ambiguous redist layout")
	}
}

func TestWindowsCRTCopyValidatesArchitectureAndRejectsDebug(t *testing.T) {
	for _, scenario := range []string{"valid", "debug", "wrong-architecture", "missing", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			redist, destination := t.TempDir(), t.TempDir()
			for _, name := range []string{"msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll"} {
				if scenario == "missing" && name == "vcruntime140.dll" {
					continue
				}
				machine := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
				if scenario == "wrong-architecture" {
					machine = pe.IMAGE_FILE_MACHINE_I386
				}
				writeWindowsCRTPEFixture(t, filepath.Join(redist, name), machine)
			}
			if scenario == "debug" {
				writeWindowsCRTPEFixture(t, filepath.Join(redist, "vcruntime140d.dll"), pe.IMAGE_FILE_MACHINE_AMD64)
			}
			if scenario == "malformed" {
				if err := os.WriteFile(filepath.Join(redist, "other.dll"), []byte("not PE"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			err := copyWindowsCRT(redist, destination, "amd64")
			if scenario != "valid" {
				if err == nil {
					t.Fatal("accepted invalid CRT source")
				}
				entries, readErr := os.ReadDir(destination)
				if readErr != nil || len(entries) != 0 {
					t.Fatalf("published files before validating CRT: %v %v", entries, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"msvcp140.dll", "vcruntime140.dll", "vcruntime140_1.dll"} {
				got, err := os.ReadFile(filepath.Join(destination, name))
				want, _ := os.ReadFile(filepath.Join(redist, name))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("CRT %s not copied exactly: %v", name, err)
				}
			}
		})
	}
}

func TestWindowsCRTRejectsInvalidRedistVersion(t *testing.T) {
	instance := t.TempDir()
	path := filepath.Join(instance, "VC", "Auxiliary", "Build", "Microsoft.VCRedistVersion.default.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("../../developer-debug-dlls"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := windowsCRTRedistDirectory(func(string) string { return "" }, func() (string, error) { return instance, nil }, "amd64")
	if err == nil || !strings.Contains(err.Error(), "invalid Visual Studio CRT version") {
		t.Fatalf("accepted unsafe version: %v", err)
	}
}

func writeWindowsCRTPEFixture(t *testing.T, path string, machine uint16) {
	t.Helper()
	data := make([]byte, 0x100)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:], 0x40)
	copy(data[0x40:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[0x44:], machine)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

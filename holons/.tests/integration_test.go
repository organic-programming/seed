package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type whoListJSON struct {
	Entries []struct {
		Identity struct {
			UUID       string `json:"uuid"`
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
			Composer   string `json:"composer"`
			Lang       string `json:"lang"`
		} `json:"identity"`
		Origin string `json:"origin"`
	} `json:"entries"`
}

type whoShowJSON struct {
	Identity struct {
		UUID      string `json:"uuid"`
		GivenName string `json:"givenName"`
		Composer  string `json:"composer"`
	} `json:"identity"`
}

type identityRow struct {
	UUID       string
	GivenName  string
	FamilyName string
	Composer   string
	Lang       string
	Origin     string
}

func TestGraceOpCallsSophiaViaMem(t *testing.T) {
	workspace, opBinary := setupWorkspaceWithBinaries(t)

	createInput := `{"givenName":"MemAlpha","familyName":"RoundTrip","motto":"mem","composer":"integration","clade":"DETERMINISTIC_PURE","lang":"go","outputDir":"holons/mem-alpha"}`
	if _, _, err := runCommand(workspace, opBinary, "who", "new", createInput); err != nil {
		t.Fatalf("op who new via mem failed: %v", err)
	}

	listResp := runWhoListJSON(t, workspace, opBinary)
	entry := findEntryByGivenName(t, listResp, "MemAlpha")

	showResp := runWhoShowJSON(t, workspace, opBinary, entry.Identity.UUID)
	if showResp.Identity.GivenName != "MemAlpha" {
		t.Fatalf("show givenName = %q, want %q", showResp.Identity.GivenName, "MemAlpha")
	}
	if showResp.Identity.Composer != "integration" {
		t.Fatalf("show composer = %q, want %q", showResp.Identity.Composer, "integration")
	}
}

func TestGraceOpCallsSophiaViaStdio(t *testing.T) {
	workspace, opBinary := setupWorkspaceWithBinaries(t)
	writeTransportOverride(t, workspace, "who", "stdio://")

	createInput := `{"givenName":"StdioAlpha","familyName":"RoundTrip","motto":"stdio","composer":"integration","clade":"DETERMINISTIC_PURE","lang":"go","outputDir":"holons/stdio-alpha"}`
	if _, _, err := runCommand(workspace, opBinary, "who", "new", createInput); err != nil {
		t.Fatalf("op who new via stdio failed: %v", err)
	}

	listResp := runWhoListJSON(t, workspace, opBinary)
	entry := findEntryByGivenName(t, listResp, "StdioAlpha")

	showResp := runWhoShowJSON(t, workspace, opBinary, entry.Identity.UUID)
	if showResp.Identity.GivenName != "StdioAlpha" {
		t.Fatalf("show givenName = %q, want %q", showResp.Identity.GivenName, "StdioAlpha")
	}
	if showResp.Identity.Composer != "integration" {
		t.Fatalf("show composer = %q, want %q", showResp.Identity.Composer, "integration")
	}
}

func TestMemAndStdioIdentityResultsMatch(t *testing.T) {
	workspace, opBinary := setupWorkspaceWithBinaries(t)

	createInput := `{"givenName":"ParityAlpha","familyName":"RoundTrip","motto":"parity","composer":"integration","clade":"DETERMINISTIC_PURE","lang":"go","outputDir":"holons/parity-alpha"}`
	if _, _, err := runCommand(workspace, opBinary, "who", "new", createInput); err != nil {
		t.Fatalf("op who new via mem failed: %v", err)
	}

	memList := runWhoListJSON(t, workspace, opBinary)
	memRows := normalizeRows(memList)

	writeTransportOverride(t, workspace, "who", "stdio://")
	stdioList := runWhoListJSON(t, workspace, opBinary)
	stdioRows := normalizeRows(stdioList)

	if !reflect.DeepEqual(memRows, stdioRows) {
		t.Fatalf("mem and stdio rows differ\nmem=%+v\nstdio=%+v", memRows, stdioRows)
	}
}

func setupWorkspaceWithBinaries(t *testing.T) (workspace string, opBinary string) {
	t.Helper()

	repoRoot := repositoryRoot(t)
	workspace = t.TempDir()

	mustMkdirAll(t, filepath.Join(workspace, "holons", "grace-op"))
	mustMkdirAll(t, filepath.Join(workspace, "holons", "sophia-who"))

	opBinary = filepath.Join(workspace, "holons", "grace-op", "op")
	whoBinary := filepath.Join(workspace, "holons", "sophia-who", "who")

	buildBinary(t, filepath.Join(repoRoot, "holons", "grace-op"), opBinary, "./cmd/op")
	buildBinary(t, filepath.Join(repoRoot, "holons", "sophia-who"), whoBinary, "./cmd/who")

	copyFile(t,
		filepath.Join(repoRoot, "holons", "sophia-who", "HOLON.md"),
		filepath.Join(workspace, "holons", "sophia-who", "HOLON.md"),
	)
	copyFile(t,
		filepath.Join(repoRoot, "holons", "grace-op", "HOLON.md"),
		filepath.Join(workspace, "holons", "grace-op", "HOLON.md"),
	)

	return workspace, opBinary
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func runWhoListJSON(t *testing.T, workspace, opBinary string) whoListJSON {
	t.Helper()
	stdout, stderr, err := runCommand(workspace, opBinary, "--format", "json", "who", "list", "holons")
	if err != nil {
		t.Fatalf("op --format json who list failed: %v\nstderr: %s", err, stderr)
	}

	var resp whoListJSON
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid list json: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if len(resp.Entries) == 0 {
		t.Fatalf("expected non-empty list output\nstdout: %s", stdout)
	}
	return resp
}

func runWhoShowJSON(t *testing.T, workspace, opBinary, uuid string) whoShowJSON {
	t.Helper()
	stdout, stderr, err := runCommand(workspace, opBinary, "--format", "json", "who", "show", uuid)
	if err != nil {
		t.Fatalf("op --format json who show failed: %v\nstderr: %s", err, stderr)
	}

	var resp whoShowJSON
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid show json: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	return resp
}

func findEntryByGivenName(t *testing.T, resp whoListJSON, givenName string) struct {
	Identity struct {
		UUID       string `json:"uuid"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
		Composer   string `json:"composer"`
		Lang       string `json:"lang"`
	} `json:"identity"`
	Origin string `json:"origin"`
} {
	t.Helper()
	for _, entry := range resp.Entries {
		if entry.Identity.GivenName == givenName {
			return entry
		}
	}
	t.Fatalf("entry with givenName=%q not found in %+v", givenName, resp.Entries)
	return struct {
		Identity struct {
			UUID       string `json:"uuid"`
			GivenName  string `json:"givenName"`
			FamilyName string `json:"familyName"`
			Composer   string `json:"composer"`
			Lang       string `json:"lang"`
		} `json:"identity"`
		Origin string `json:"origin"`
	}{}
}

func normalizeRows(resp whoListJSON) []identityRow {
	rows := make([]identityRow, 0, len(resp.Entries))
	for _, entry := range resp.Entries {
		rows = append(rows, identityRow{
			UUID:       entry.Identity.UUID,
			GivenName:  entry.Identity.GivenName,
			FamilyName: entry.Identity.FamilyName,
			Composer:   entry.Identity.Composer,
			Lang:       entry.Identity.Lang,
			Origin:     entry.Origin,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].UUID == rows[j].UUID {
			return rows[i].GivenName < rows[j].GivenName
		}
		return rows[i].UUID < rows[j].UUID
	})
	return rows
}

func writeTransportOverride(t *testing.T, workspace, holon, transportURI string) {
	t.Helper()
	content := fmt.Sprintf("transport:\n  %s: %s\n", holon, transportURI)
	path := filepath.Join(workspace, ".holonconfig")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func buildBinary(t *testing.T, moduleDir, outPath, pkg string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(outPath))

	cmd := exec.Command("go", "build", "-o", outPath, pkg)
	cmd.Dir = moduleDir
	stdout, stderr, err := runRaw(cmd)
	if err != nil {
		t.Fatalf("build failed in %s: %v\nstdout: %s\nstderr: %s", moduleDir, err, stdout, stderr)
	}
}

func runCommand(dir, binary string, args ...string) (stdout string, stderr string, err error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	return runRaw(cmd)
}

func runRaw(cmd *exec.Cmd) (stdout string, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	mustMkdirAll(t, filepath.Dir(dst))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

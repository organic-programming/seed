package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type compositeHolonPackage struct {
	Entrypoint string `json:"entrypoint"`
}

type CompositeCOAXSession struct {
	ListenURI string
	HomeDir   string

	process *ProcessHandle
	sb      *Sandbox
}

func StartBuiltCompositeCOAX(t *testing.T, sb *Sandbox, slug string, extraArgs ...string) *CompositeCOAXSession {
	t.Helper()

	report := BuildDryRunReportFor(t, sb, slug, extraArgs...)
	artifactPath := ReportPath(t, report.Artifact)
	RequirePathExists(t, artifactPath)

	launchPath, workDir, bundleID := compositeLaunchSpec(t, artifactPath)
	tmpHome := shortCompositeRuntimeHome(t)
	tmpDir := filepath.Join(tmpHome, "t")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", tmpDir, err)
	}

	listenURI := fmt.Sprintf("tcp://127.0.0.1:%d", AvailablePort(t))
	env := []string{
		"HOME=" + tmpHome,
		"TMPDIR=" + tmpDir,
		"TMP=" + tmpDir,
		"TEMP=" + tmpDir,
		"XDG_CONFIG_HOME=" + filepath.Join(tmpHome, ".config"),
		"APPDATA=" + filepath.Join(tmpHome, "AppData", "Roaming"),
		"USERPROFILE=" + tmpHome,
		"OP_COAX_SERVER_ENABLED=1",
		"OP_COAX_SERVER_LISTEN_URI=" + listenURI,
	}
	if runtime.GOOS == "darwin" {
		env = append(env, "CFFIXED_USER_HOME="+tmpHome)
	}
	if strings.TrimSpace(bundleID) != "" {
		env = append(env, "__CFBundleIdentifier="+bundleID)
	}

	process := sb.StartProcess(t, RunOptions{
		BinaryPath:       launchPath,
		Env:              env,
		SkipDiscoverRoot: true,
		WorkDir:          workDir,
	})
	address := process.WaitForCOAXListenAddress(t, ProcessStartTimeout)

	return &CompositeCOAXSession{
		ListenURI: address,
		HomeDir:   tmpHome,
		process:   process,
		sb:        sb,
	}
}

func shortCompositeRuntimeHome(t *testing.T) string {
	t.Helper()

	rt := mustRuntime(t)
	base := rt.tempAliasRoot
	if strings.TrimSpace(base) == "" {
		base = rt.tempBaseRoot
	}
	if strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", base, err)
	}

	dir, err := os.MkdirTemp(base, "cx-")
	if err != nil {
		t.Fatalf("mkdtemp %s: %v", base, err)
	}
	return dir
}

func (s *CompositeCOAXSession) InvokeResult(t *testing.T, method string, payload string) CmdResult {
	t.Helper()
	if s == nil || s.sb == nil {
		t.Fatal("composite COAX session is not initialized")
	}

	args := []string{"--format", "json", s.ListenURI, method}
	if payload != "" {
		args = append(args, payload)
	}
	return s.sb.RunOPWithOptions(t, RunOptions{
		SkipDiscoverRoot: true,
		Timeout:          60 * time.Second,
	}, args...)
}

func (s *CompositeCOAXSession) InvokeJSON(t *testing.T, method string, payload string) map[string]any {
	t.Helper()
	result := s.InvokeResult(t, method, payload)
	RequireSuccess(t, result)
	return DecodeJSON[map[string]any](t, result.Stdout)
}

func (s *CompositeCOAXSession) Stop(t *testing.T) {
	t.Helper()
	if s == nil {
		return
	}

	if s.sb != nil && strings.TrimSpace(s.ListenURI) != "" {
		_ = s.sb.RunOPWithOptions(t, RunOptions{
			SkipDiscoverRoot: true,
			Timeout:          10 * time.Second,
		}, "--format", "json", s.ListenURI, "TurnOffCoax", "{}")
		if s.process != nil {
			_ = s.process.Wait(3 * time.Second)
		}
	}

	if s.process != nil {
		s.process.Stop(t)
	}
}

func (s *CompositeCOAXSession) Wait(timeout time.Duration) error {
	if s == nil || s.process == nil {
		return nil
	}
	return s.process.Wait(timeout)
}

func (s *CompositeCOAXSession) CombinedOutput() string {
	if s == nil || s.process == nil {
		return ""
	}
	return s.process.Combined()
}

func compositeLaunchSpec(t *testing.T, artifactPath string) (string, string, string) {
	t.Helper()

	trimmed := strings.TrimSpace(artifactPath)
	switch {
	case strings.HasSuffix(strings.ToLower(trimmed), ".app"):
		launchPath, bundleID := appBundleLaunchSpec(t, trimmed)
		return launchPath, filepath.Dir(launchPath), bundleID
	case strings.HasSuffix(strings.ToLower(trimmed), ".holon"):
		entrypoint := readCompositePackageEntrypoint(t, trimmed)
		runtimeDir := filepath.Join(trimmed, "bin", runtime.GOOS+"_"+runtime.GOARCH)
		candidate := filepath.Join(runtimeDir, filepath.FromSlash(entrypoint))
		if strings.HasSuffix(strings.ToLower(entrypoint), ".app") {
			launchPath, bundleID := appBundleLaunchSpec(t, candidate)
			return launchPath, filepath.Dir(launchPath), bundleID
		}
		RequirePathExists(t, candidate)
		return candidate, filepath.Dir(candidate), ""
	default:
		t.Fatalf("unsupported composite artifact: %s", artifactPath)
		return "", "", ""
	}
}

func readCompositePackageEntrypoint(t *testing.T, packagePath string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(packagePath, ".holon.json"))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(packagePath, ".holon.json"), err)
	}
	var pkg compositeHolonPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("decode .holon.json: %v", err)
	}
	if strings.TrimSpace(pkg.Entrypoint) == "" {
		t.Fatalf("entrypoint missing from %s", filepath.Join(packagePath, ".holon.json"))
	}
	return pkg.Entrypoint
}

func appBundleLaunchSpec(t *testing.T, bundlePath string) (string, string) {
	t.Helper()

	plistPath := filepath.Join(bundlePath, "Contents", "Info.plist")
	executable := plistStringValue(t, plistPath, "CFBundleExecutable")
	bundleID := plistStringValue(t, plistPath, "CFBundleIdentifier")
	launchPath := filepath.Join(bundlePath, "Contents", "MacOS", executable)
	RequirePathExists(t, launchPath)
	return launchPath, bundleID
}

func plistStringValue(t *testing.T, plistPath string, key string) string {
	t.Helper()

	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("read %s: %v", plistPath, err)
	}

	needle := "<key>" + key + "</key>"
	content := string(data)
	index := strings.Index(content, needle)
	if index < 0 {
		t.Fatalf("%s missing from %s", key, plistPath)
	}
	rest := content[index+len(needle):]
	start := strings.Index(rest, "<string>")
	end := strings.Index(rest, "</string>")
	if start < 0 || end < 0 || end <= start+len("<string>") {
		t.Fatalf("%s has no string value in %s", key, plistPath)
	}
	return strings.TrimSpace(rest[start+len("<string>") : end])
}

package composite_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestBuild_07_Composite(t *testing.T) {
	rootPath := integration.DefaultWorkspaceDir(t)
	integration.TeardownHolons(t, rootPath)
	envVars, opBin := integration.SetupIsolatedOP(t, rootPath)
	t.Run("hardened-dry-run-flutter", func(t *testing.T) {
		cmd := exec.Command(opBin, "--format", "json", "build", "gabriel-greeting-app-flutter", "--dry-run", "--hardened", "--root", rootPath)
		cmd.Env = envVars
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed hardened dry-run build: %v\nOutput: %s", err, string(out))
		}

		var report struct {
			Notes    []string `json:"notes"`
			Children []struct {
				Holon string `json:"holon"`
			} `json:"children"`
		}
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("parse hardened dry-run JSON: %v\nOutput: %s", err, string(out))
		}

		for _, child := range report.Children {
			if child.Holon == "gabriel-greeting-python" {
				t.Fatalf("hardened dry-run should exclude python child report:\n%s", string(out))
			}
		}

		foundPythonSkip := false
		for _, note := range report.Notes {
			if note == `hardened: skipped build_member "greeting-python" (runner "python" not standalone)` {
				foundPythonSkip = true
				break
			}
		}
		if !foundPythonSkip {
			t.Fatalf("hardened dry-run missing skip note:\n%s", string(out))
		}
	})

	for _, spec := range integration.CompositeTestHolons(t) {
		spec := spec
		t.Run(spec.Slug, func(t *testing.T) {
			t.Logf("Building composite app %s from a clean state...", spec.Slug)
			cmd := exec.Command(opBin, "build", spec.Slug, "--root", rootPath)
			cmd.Env = envVars
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to build composite %s: %v\nOutput: %s", spec.Slug, err, string(out))
			}

			if _, err := os.Stat(integration.CompositeArtifactPath(rootPath, spec.Slug)); err != nil {
				t.Fatalf("expected built composite artifact for %s: %v", spec.Slug, err)
			}

			verifySandboxedCompositeApp(t, rootPath, spec.Slug)
		})
	}
}

func verifySandboxedCompositeApp(t *testing.T, rootPath string, slug string) {
	t.Helper()

	bundlePath, err := compositeAppBundlePath(rootPath, slug)
	if err != nil {
		t.Fatalf("locate app bundle for %s: %v", slug, err)
	}

	verifyCodeSignature(t, bundlePath)
	verifyEntitlementEnabled(t, bundlePath, "com.apple.security.app-sandbox")
	verifyEntitlementEnabled(t, bundlePath, "com.apple.security.network.client")
	verifyEntitlementEnabled(t, bundlePath, "com.apple.security.network.server")
}

func compositeAppBundlePath(rootPath string, slug string) (string, error) {
	switch slug {
	case "gabriel-greeting-app-swiftui":
		return integration.CompositeArtifactPath(rootPath, slug), nil
	case "gabriel-greeting-app-flutter":
		return firstAppBundleUnder(integration.CompositeArtifactPath(rootPath, slug))
	default:
		return "", fmt.Errorf("unsupported composite app %q", slug)
	}
}

func firstAppBundleUnder(root string) (string, error) {
	var match string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".app" {
			match = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if match == "" {
		return "", fmt.Errorf("missing .app bundle under %s", root)
	}
	return match, nil
}

func verifyCodeSignature(t *testing.T, bundlePath string) {
	t.Helper()

	cmd := exec.Command("codesign", "--verify", "--deep", bundlePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codesign verify %s: %v\nOutput: %s", bundlePath, err, string(out))
	}
}

func verifyEntitlementEnabled(t *testing.T, bundlePath string, key string) {
	t.Helper()

	cmd := exec.Command("codesign", "-d", "--entitlements=-", bundlePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("codesign entitlements %s: %v\nOutput: %s", bundlePath, err, string(out))
	}

	pattern := regexp.MustCompile(fmt.Sprintf(`(?s)\[Key\]\s+%s\s+\[Value\]\s+\[Bool\]\s+true`, regexp.QuoteMeta(key)))
	matches := pattern.FindStringSubmatch(string(out))
	if len(matches) == 0 {
		t.Fatalf("expected %s enabled in %s entitlements:\n%s", key, bundlePath, string(out))
	}
}

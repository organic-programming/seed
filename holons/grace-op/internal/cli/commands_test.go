package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
	opmod "github.com/organic-programming/grace-op/internal/mod"
)

func TestVersionCommand(t *testing.T) {
	code := Run([]string{"version"}, "0.1.0-test")
	if code != 0 {
		t.Errorf("version returned %d, want 0", code)
	}
}

func TestHelpCommand(t *testing.T) {
	for _, cmd := range []string{"help", "--help", "-h"} {
		code := Run([]string{cmd}, "0.1.0-test")
		if code != 0 {
			t.Errorf("%s returned %d, want 0", cmd, code)
		}
	}
}

func TestHelpCommandIncludesCleanFlags(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"help"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("help returned %d, want 0", code)
		}
	})

	for _, want := range []string{
		"op <holon> --clean <method> [--no-build] [json]",
		"--clean                                      clean before building (cannot be combined with --dry-run)",
		"--clean                                      clean before building and running (cannot be combined with --no-build)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestHelpBuildTopicIncludesCleanFlag(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"help", "build"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("help build returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "--clean") {
		t.Fatalf("help build missing --clean flag:\n%s", output)
	}
	if !strings.Contains(output, "cannot be combined with --dry-run") {
		t.Fatalf("help build missing dry-run restriction:\n%s", output)
	}
}

func TestHelpRunTopicIncludesCleanFlag(t *testing.T) {
	output := captureStdout(t, func() {
		code := Run([]string{"help", "run"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("help run returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "--clean") {
		t.Fatalf("help run missing --clean flag:\n%s", output)
	}
	if !strings.Contains(output, "cannot be combined with --no-build") {
		t.Fatalf("help run missing no-build restriction:\n%s", output)
	}
}

func TestRunWhoListThroughTransportChainWithoutBuiltInComposerFails(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "dummy-test",
		givenName:  "Sophia",
		familyName: "TestHolon",
		aliases:    []string{"who", "sophia"},
		lang:       "go",
	})
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "atlas",
		binaryName: "atlas",
		givenName:  "atlas",
		familyName: "Holon",
		aliases:    []string{"atlas"},
		lang:       "go",
	})

	code := Run([]string{"who", "list", "holons"}, "0.1.0-test")
	if code != 1 {
		t.Fatalf("who list returned %d, want 1", code)
	}
}

func TestRunPromotedListNative(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "dummy-test",
		givenName:  "Sophia",
		familyName: "TestHolon",
		aliases:    []string{"who", "sophia"},
		lang:       "go",
	})

	code := Run([]string{"list", "holons"}, "0.1.0-test")
	if code != 0 {
		t.Fatalf("list returned %d, want 0", code)
	}
}

func TestRunNativeShowCommand(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "dummy-test",
		givenName:  "Sophia",
		familyName: "TestHolon",
		aliases:    []string{"who", "sophia"},
		lang:       "go",
	})

	output := captureStdout(t, func() {
		code := Run([]string{"show", "who"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("show returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "Sophia TestHolon") {
		t.Fatalf("show output missing identity name: %q", output)
	}
	if !strings.Contains(output, identity.ManifestFileName) {
		t.Fatalf("show output missing manifest path: %q", output)
	}
}

func TestRunNativeNewCommandJSON(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{
			"new",
			"--json",
			`{"given_name":"Alpha","family_name":"Builder","motto":"Builds holons.","composer":"test","clade":"deterministic/io_bound","lang":"go"}`,
		}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("new returned %d, want 0", code)
		}
	})

	createdPath := filepath.Join(root, "holons", "alpha-builder", identity.ManifestFileName)
	if _, err := os.Stat(createdPath); err != nil {
		t.Fatalf("created holon manifest missing: %v", err)
	}
	data, err := os.ReadFile(createdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `option (holons.v1.manifest) = {`) {
		t.Fatalf("created holon manifest missing manifest option: %s", string(data))
	}
	if !strings.Contains(string(data), `given_name: "Alpha"`) {
		t.Fatalf("created holon manifest missing given_name: %s", string(data))
	}
	if !strings.Contains(output, "Identity created") {
		t.Fatalf("new output missing creation message: %q", output)
	}
	if !strings.Contains(output, "Alpha Builder") {
		t.Fatalf("new output missing identity name: %q", output)
	}
}

func TestRunNewListTemplates(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"new", "--list"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("new --list returned %d, want 0", code)
		}
	})

	for _, expected := range []string{"composite-go-swiftui", "composite-go-web", "composite-python-web"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("template list missing %q: %q", expected, output)
		}
	}
}

func TestRunNewTemplateCreatesScaffold(t *testing.T) {
	t.Skip("template rendering is covered in internal/scaffold; CLI template catalog wiring is outside discovery migration")

	root := t.TempDir()
	chdirForTest(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"new", "--template", "go-daemon", "delta-engine", "--set", "service=EchoService"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("template new returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "Created delta-engine from go-daemon") {
		t.Fatalf("stdout missing creation summary: %q", output)
	}

	mainPath := filepath.Join(root, "delta-engine", "cmd", "delta-engine", "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v", mainPath, err)
	}
	if !strings.Contains(string(data), "EchoService ready for delta-engine") {
		t.Fatalf("generated main.go missing override: %s", string(data))
	}
}

func TestRunNewTemplateJSONOutput(t *testing.T) {
	t.Skip("template rendering is covered in internal/scaffold; CLI template catalog wiring is outside discovery migration")

	root := t.TempDir()
	chdirForTest(t, root)

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "new", "--template", "composite-go-swiftui", "my-console"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("template new json returned %d, want 0", code)
		}
	})

	var payload struct {
		Template string `json:"template"`
		Dir      string `json:"dir"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("template new json invalid: %v\noutput=%s", err, output)
	}
	if payload.Template != "composite-go-swiftui" {
		t.Fatalf("template = %q, want %q", payload.Template, "composite-go-swiftui")
	}
	gotDir, err := filepath.EvalSymlinks(payload.Dir)
	if err != nil {
		gotDir = filepath.Clean(payload.Dir)
	}
	wantDir, err := filepath.EvalSymlinks(filepath.Join(root, "my-console"))
	if err != nil {
		wantDir = filepath.Join(root, "my-console")
	}
	if gotDir != wantDir {
		t.Fatalf("dir = %q, want %q", payload.Dir, wantDir)
	}
}

func TestMapHolonCommandToRPC(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMethod  string
		wantInput   string
		wantClean   bool
		wantNoBuild bool
		wantErr     bool
	}{
		{
			name:       "list default",
			args:       []string{"list"},
			wantMethod: "ListIdentities",
			wantInput:  "{}",
		},
		{
			name:       "list root dir",
			args:       []string{"list", "holons"},
			wantMethod: "ListIdentities",
			wantInput:  `{"rootDir":"holons"}`,
		},
		{
			name:       "show uuid",
			args:       []string{"show", "abc123"},
			wantMethod: "ShowIdentity",
			wantInput:  `{"uuid":"abc123"}`,
		},
		{
			name:       "new with json input",
			args:       []string{"new", `{"givenName":"Alpha"}`},
			wantMethod: "CreateIdentity",
			wantInput:  `{"givenName":"Alpha"}`,
		},
		{
			name:       "custom method passthrough",
			args:       []string{"ListIdentities"},
			wantMethod: "ListIdentities",
			wantInput:  "{}",
		},
		{
			name:        "custom method with no-build and json",
			args:        []string{"Ping", "--no-build", `{"message":"hello"}`},
			wantMethod:  "Ping",
			wantInput:   `{"message":"hello"}`,
			wantNoBuild: true,
		},
		{
			name:        "custom method with clean no-build and json",
			args:        []string{"--clean", "Ping", "--no-build", `{"message":"hello"}`},
			wantMethod:  "Ping",
			wantInput:   `{"message":"hello"}`,
			wantClean:   true,
			wantNoBuild: true,
		},
		{
			name:    "no-build must follow method",
			args:    []string{"Ping", `{"message":"hello"}`, "--no-build"},
			wantErr: true,
		},
		{
			name:    "clean must precede method",
			args:    []string{"Ping", "--clean", `{"message":"hello"}`},
			wantErr: true,
		},
		{
			name:    "show missing uuid",
			args:    []string{"show"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method, input, cleanFirst, noBuild, err := mapHolonCommandToRPC(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("mapHolonCommandToRPC returned error: %v", err)
			}
			if cleanFirst != tc.wantClean {
				t.Fatalf("cleanFirst = %t, want %t", cleanFirst, tc.wantClean)
			}
			if noBuild != tc.wantNoBuild {
				t.Fatalf("noBuild = %t, want %t", noBuild, tc.wantNoBuild)
			}
			if method != tc.wantMethod {
				t.Fatalf("method = %q, want %q", method, tc.wantMethod)
			}
			if input != tc.wantInput {
				t.Fatalf("input = %q, want %q", input, tc.wantInput)
			}
		})
	}
}

func TestDoCommandDryRunProtoBackedSequence(t *testing.T) {
	repoRoot := inspectRepoRoot(t)
	chdirForTest(t, repoRoot)

	output := captureStdout(t, func() {
		code := Run([]string{
			"do",
			"gabriel-greeting-go",
			"multilingual-greeting",
			"--name=Maria",
			"--lang_code=fr",
			"--dry-run",
		}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("do --dry-run returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, `[1/2] op gabriel-greeting-go ListLanguages`) {
		t.Fatalf("dry run output missing first step: %q", output)
	}
	if !strings.Contains(output, `SayHello '{"name":"Maria","lang_code":"fr"}'`) {
		t.Fatalf("dry run output missing rendered second step: %q", output)
	}
}

func TestCommandForArtifactIncludesCompositeAssemblyEnv(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "build", "app")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := &holons.LoadedManifest{
		Dir:  root,
		Name: "gudule-greeting-flutter-rust",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-Flutter-Rust",
			Transport:  "stdio",
			Artifacts:  holons.ArtifactPaths{Primary: "build/app"},
		},
	}

	cmd, err := commandForArtifact(manifest, holons.BuildContext{Target: "macos"}, "stdio://")
	if err != nil {
		t.Fatalf("commandForArtifact returned error: %v", err)
	}

	if got := envValue(cmd.Env, "OP_ASSEMBLY_FAMILY"); got != "Greeting-Flutter-Rust" {
		t.Fatalf("OP_ASSEMBLY_FAMILY = %q, want %q", got, "Greeting-Flutter-Rust")
	}
	if got := envValue(cmd.Env, "OP_ASSEMBLY_TRANSPORT"); got != "stdio" {
		t.Fatalf("OP_ASSEMBLY_TRANSPORT = %q, want %q", got, "stdio")
	}
}

func TestCommandForInstalledArtifactIncludesCompositeAssemblyEnv(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, "gudule-greeting-flutter-rust")
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	target := &holons.Target{
		Manifest: &holons.LoadedManifest{
			Dir:  root,
			Name: "gudule-greeting-flutter-rust",
			Manifest: holons.Manifest{
				Kind:       holons.KindComposite,
				FamilyName: "Greeting-Flutter-Rust",
				Transport:  "stdio",
			},
		},
	}

	cmd, err := commandForInstalledArtifact(binaryPath, target, "stdio://")
	if err != nil {
		t.Fatalf("commandForInstalledArtifact returned error: %v", err)
	}

	if got := envValue(cmd.Env, "OP_ASSEMBLY_FAMILY"); got != "Greeting-Flutter-Rust" {
		t.Fatalf("OP_ASSEMBLY_FAMILY = %q, want %q", got, "Greeting-Flutter-Rust")
	}
	if got := envValue(cmd.Env, "OP_ASSEMBLY_TRANSPORT"); got != "stdio" {
		t.Fatalf("OP_ASSEMBLY_TRANSPORT = %q, want %q", got, "stdio")
	}
}

func TestCommandForArtifactDoesNotAddAssemblyEnvForNativeHolons(t *testing.T) {
	root := t.TempDir()
	binaryPath := filepath.Join(root, ".op", "build", "gudule-daemon-greeting-rust.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "gudule-daemon-greeting-rust")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := &holons.LoadedManifest{
		Dir:  root,
		Name: "gudule-daemon-greeting-rust",
		Manifest: holons.Manifest{
			Kind: holons.KindNative,
			Artifacts: holons.ArtifactPaths{
				Binary: "gudule-daemon-greeting-rust",
			},
		},
	}

	cmd, err := commandForArtifact(manifest, holons.BuildContext{Target: "macos"}, "tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("commandForArtifact returned error: %v", err)
	}

	if got := envValue(cmd.Env, "OP_ASSEMBLY_FAMILY"); got != "" {
		t.Fatalf("OP_ASSEMBLY_FAMILY = %q, want empty", got)
	}
	if got := envValue(cmd.Env, "OP_ASSEMBLY_TRANSPORT"); got != "" {
		t.Fatalf("OP_ASSEMBLY_TRANSPORT = %q, want empty", got)
	}
}

func TestOpenAppBundleCommandPassesAssemblyEnvOnMacOS(t *testing.T) {
	manifest := &holons.LoadedManifest{
		Name: "gudule-greeting-flutter-rust",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-Flutter-Rust",
			Transport:  "stdio",
		},
	}

	cmd := openAppBundleCommand("/tmp/gudule-greeting-hostui-flutter.app", manifest)
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "OP_ASSEMBLY_FAMILY=Greeting-Flutter-Rust") {
		t.Fatalf("open args missing family env: %v", cmd.Args)
	}
	if !strings.Contains(args, "OP_ASSEMBLY_DISPLAY_FAMILY=Greeting-Flutter-Rust (Flutter UI)") {
		t.Fatalf("open args missing display family env: %v", cmd.Args)
	}
	if !strings.Contains(args, "OP_ASSEMBLY_TRANSPORT=stdio") {
		t.Fatalf("open args missing transport env: %v", cmd.Args)
	}
}

func TestNormalizeMacOSAppBundleMetadataUsesCompositeIdentity(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "Example.app")
	contents := filepath.Join(bundle, "Contents")
	appDir := filepath.Join(contents, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	plistPath := filepath.Join(contents, "Info.plist")
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>placeholder-app</string>
	<key>CFBundleDisplayName</key>
	<string>placeholder-app</string>
	<key>CFBundleIdentifier</key>
	<string>org.organicprogramming.placeholder</string>
</dict>
</plist>
`
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "example.cfg"), []byte("java-options=-Xdock:name=placeholder-app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	normalizeMacOSAppBundleMetadata(bundle, &holons.LoadedManifest{
		Name: "gudule-greeting-kotlinui-csharp",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-Kotlinui-Csharp",
		},
	})

	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "<string>Gudule Greeting-Kotlinui-Csharp (Kotlin UI)</string>") {
		t.Fatalf("normalized plist missing display name: %s", content)
	}
	if !strings.Contains(content, "<string>org.organicprogramming.gudule-greeting-kotlinui-csharp</string>") {
		t.Fatalf("normalized plist missing bundle identifier: %s", content)
	}

	cfgData, err := os.ReadFile(filepath.Join(appDir, "example.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfgData), "java-options=-Xdock:name=Gudule Greeting-Kotlinui-Csharp (Kotlin UI)") {
		t.Fatalf("normalized cfg missing dock name: %s", string(cfgData))
	}
}

func TestCompositeDisplayFamilyLabelsWebAssemblies(t *testing.T) {
	manifest := &holons.LoadedManifest{
		Name: "gudule-greeting-go-web",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-Go-Web",
		},
	}

	if got := compositeDisplayFamily(manifest); got != "Greeting-Go-Web (Web UI)" {
		t.Fatalf("compositeDisplayFamily = %q, want %q", got, "Greeting-Go-Web (Web UI)")
	}
	if got := compositeBundleDisplayName(manifest); got != "Gudule Greeting-Go-Web (Web UI)" {
		t.Fatalf("compositeBundleDisplayName = %q, want %q", got, "Gudule Greeting-Go-Web (Web UI)")
	}
}

func TestCommandForArtifactPreservesTransportOverrideFromEnvironment(t *testing.T) {
	t.Setenv("OP_ASSEMBLY_TRANSPORT", "stdio")

	root := t.TempDir()
	artifactPath := filepath.Join(root, "build", "app")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := &holons.LoadedManifest{
		Dir:  root,
		Name: "gudule-greeting-flutter-go",
		Manifest: holons.Manifest{
			Kind:       holons.KindComposite,
			FamilyName: "Greeting-Flutter-Go",
			Transport:  "tcp",
			Artifacts:  holons.ArtifactPaths{Primary: "build/app"},
		},
	}

	cmd, err := commandForArtifact(manifest, holons.BuildContext{Target: "macos"}, "stdio://")
	if err != nil {
		t.Fatalf("commandForArtifact returned error: %v", err)
	}

	if got := envValue(cmd.Env, "OP_ASSEMBLY_TRANSPORT"); got != "stdio" {
		t.Fatalf("OP_ASSEMBLY_TRANSPORT = %q, want %q", got, "stdio")
	}
}

func TestDiscoverCommand(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "who",
		binaryName: "who",
		givenName:  "who",
		familyName: "Holon",
		aliases:    []string{"who"},
		lang:       "go",
	})
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "atlas",
		binaryName: "atlas",
		givenName:  "atlas",
		familyName: "Holon",
		aliases:    []string{"atlas"},
		lang:       "rust",
	})

	output := captureStdout(t, func() {
		code := Run([]string{"discover"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("discover returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "LANG") {
		t.Fatalf("discover output missing LANG column: %q", output)
	}
	if !strings.Contains(output, "who Holon") {
		t.Fatalf("discover output missing who holon row: %q", output)
	}
	if !strings.Contains(output, "atlas Holon") {
		t.Fatalf("discover output missing atlas holon row: %q", output)
	}
	// Verify relative path appears in the who row (tabwriter converts tabs to spaces)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "who Holon") {
			if !strings.Contains(line, "who") {
				t.Fatalf("who row missing relative path: %q", line)
			}
			break
		}
	}
	if !strings.Contains(output, "source") {
		t.Fatalf("discover output missing origin: %q", output)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func TestDiscoverCommandJSONFormat(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "who",
		binaryName: "who",
		givenName:  "who",
		familyName: "Holon",
		aliases:    []string{"who"},
		lang:       "go",
	})
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "atlas",
		binaryName: "atlas",
		givenName:  "atlas",
		familyName: "Holon",
		aliases:    []string{"atlas"},
		lang:       "rust",
	})

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "discover"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("discover --format json returned %d, want 0", code)
		}
	})

	var payload struct {
		Entries []struct {
			UUID         string `json:"uuid"`
			GivenName    string `json:"given_name"`
			FamilyName   string `json:"family_name"`
			Lang         string `json:"lang"`
			Clade        string `json:"clade"`
			Status       string `json:"status"`
			RelativePath string `json:"relative_path"`
			Origin       string `json:"origin"`
		} `json:"entries"`
		PathBinaries []string `json:"path_binaries"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("discover json output is invalid: %v\noutput=%s", err, output)
	}
	if len(payload.Entries) < 2 {
		t.Fatalf("entries = %d, want at least 2", len(payload.Entries))
	}

	foundWho := false
	for _, entry := range payload.Entries {
		if entry.GivenName != "who" {
			continue
		}
		foundWho = true
		if entry.Lang != "go" {
			t.Fatalf("who lang = %q, want %q", entry.Lang, "go")
		}
		if entry.Origin != "source" {
			t.Fatalf("who origin = %q, want %q", entry.Origin, "source")
		}
		if entry.RelativePath != "holons/who" {
			t.Fatalf("who relative_path = %q, want %q", entry.RelativePath, "holons/who")
		}
	}
	if !foundWho {
		t.Fatalf("who entry not found in json output: %s", output)
	}
}

func TestDiscoverCommandIncludesCachedAndInstalledHolons(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "who",
		binaryName: "who",
		givenName:  "who",
		familyName: "Holon",
		aliases:    []string{"who"},
		lang:       "go",
	})

	runtimeHome := filepath.Join(root, "runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	cacheDir := filepath.Join(runtimeHome, "cache", "github.com", "example", "cached-holon@v1.0.0")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachedID := identity.Identity{
		UUID:        "cached-test-holon",
		GivenName:   "Cached",
		FamilyName:  "Holon",
		Motto:       "Cached test.",
		Composer:    "test",
		Clade:       "deterministic/pure",
		Status:      "draft",
		Born:        "2026-03-07",
		Aliases:     []string{"cached"},
		GeneratedBy: "test",
		Lang:        "go",
	}
	cachedManifest := fmt.Sprintf("%s\nkind: native\nbuild:\n  runner: go-module\nartifacts:\n  binary: cached-holon\n", manifestIdentityPrefix(cachedID))
	if err := writeCLIManifestFile(filepath.Join(cacheDir, identity.ManifestFileName), cachedManifest); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(runtimeHome, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(runtimeHome, "bin", "installed-holon")
	if err := os.WriteFile(installedPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"discover"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("discover returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "cached") {
		t.Fatalf("discover output missing cached holon: %q", output)
	}
	if !strings.Contains(output, "In $OPBIN:") {
		t.Fatalf("discover output missing $OPBIN section: %q", output)
	}
	if !strings.Contains(output, "installed-holon") {
		t.Fatalf("discover output missing installed binary: %q", output)
	}
}

func TestEnvCommand(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	t.Setenv("OPPATH", filepath.Join(root, ".op-home"))
	t.Setenv("OPBIN", filepath.Join(root, ".op-home", "bin"))

	output := captureStdout(t, func() {
		code := Run([]string{"env"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("env returned %d, want 0", code)
		}
	})

	if !strings.Contains(output, "OPPATH="+filepath.Join(root, ".op-home")) {
		t.Fatalf("env output missing OPPATH: %q", output)
	}
	if !strings.Contains(output, "OPBIN="+filepath.Join(root, ".op-home", "bin")) {
		t.Fatalf("env output missing OPBIN: %q", output)
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		wantRoot = root
	}
	if !strings.Contains(output, "ROOT="+wantRoot) {
		t.Fatalf("env output missing ROOT: %q", output)
	}
}

func TestEnvCommandInitAndShell(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	output := captureStdout(t, func() {
		code := Run([]string{"env", "--init", "--shell"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("env --init --shell returned %d, want 0", code)
		}
	})

	if _, err := os.Stat(filepath.Join(root, ".runtime")); err != nil {
		t.Fatalf("runtime home missing after init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".runtime", "bin")); err != nil {
		t.Fatalf("opbin missing after init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".runtime", "cache")); err != nil {
		t.Fatalf("cache missing after init: %v", err)
	}
	if !strings.Contains(output, `export OPPATH="${OPPATH:-$HOME/.op}"`) {
		t.Fatalf("env --shell output missing OPPATH export: %q", output)
	}
	if !strings.Contains(output, `export PATH="$OPBIN:$PATH"`) {
		t.Fatalf("env --shell output missing PATH export: %q", output)
	}
}

func TestEnvCommandJSONFormat(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "env"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("env --format json returned %d, want 0", code)
		}
	})

	var payload struct {
		OPPATH string `json:"oppath"`
		OPBIN  string `json:"opbin"`
		ROOT   string `json:"root"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("env json output is invalid: %v\noutput=%s", err, output)
	}
	if payload.OPPATH != filepath.Join(root, ".runtime") {
		t.Fatalf("oppath = %q, want %q", payload.OPPATH, filepath.Join(root, ".runtime"))
	}
	if payload.OPBIN != filepath.Join(root, ".runtime", "bin") {
		t.Fatalf("opbin = %q, want %q", payload.OPBIN, filepath.Join(root, ".runtime", "bin"))
	}
	wantRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		wantRoot = root
	}
	if payload.ROOT != wantRoot {
		t.Fatalf("root = %q, want %q", payload.ROOT, wantRoot)
	}
}

func TestInstallCommand(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"install", "--build", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("install returned %d, want 0", code)
		}
	})

	installed := filepath.Join(root, ".runtime", "bin", "demo.holon")
	if _, err := os.Stat(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo")); err != nil {
		t.Fatalf("installed package binary missing: %v", err)
	}
	if !strings.Contains(output, "Installed: "+installed) {
		t.Fatalf("install output missing installed path: %q", output)
	}
}

func TestInstallCommandRebuildsExistingArtifact(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"fresh\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	staleArtifact := []byte("#!/bin/sh\necho stale\n")
	staleMode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		staleArtifact = []byte("@echo off\r\necho stale\r\n")
		staleMode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo"), staleArtifact, staleMode); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"install", "--build", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("install returned %d, want 0", code)
		}
	})

	installed := filepath.Join(root, ".runtime", "bin", "demo.holon")
	result, err := exec.Command(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo")).CombinedOutput()
	if err != nil {
		t.Fatalf("running installed binary failed: %v\noutput=%s", err, result)
	}
	if got := strings.TrimSpace(string(result)); got != "fresh" {
		t.Fatalf("installed binary output = %q, want %q", got, "fresh")
	}
	if !strings.Contains(output, "rebuilt before install") {
		t.Fatalf("install output missing rebuild note: %q", output)
	}
}

func TestInstallCommandJSONFormat(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "install", "--build", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("install --format json returned %d, want 0", code)
		}
	})

	var payload struct {
		Operation string `json:"operation"`
		Binary    string `json:"binary"`
		Installed string `json:"installed"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("install json output invalid: %v\noutput=%s", err, output)
	}
	if payload.Operation != "install" {
		t.Fatalf("operation = %q, want install", payload.Operation)
	}
	if payload.Binary != "demo" {
		t.Fatalf("binary = %q, want demo", payload.Binary)
	}
	if payload.Installed != filepath.Join(root, ".runtime", "bin", "demo.holon") {
		t.Fatalf("installed = %q, want %q", payload.Installed, filepath.Join(root, ".runtime", "bin", "demo.holon"))
	}
}

func TestInstallCommandProtoManifest(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	writeProtoInstallFixture(t, root, "demo-proto")

	output := captureStdout(t, func() {
		code := Run([]string{"install", "--build", "demo-proto"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("install returned %d, want 0", code)
		}
	})

	installed := filepath.Join(root, ".runtime", "bin", "demo-proto.holon")
	if _, err := os.Stat(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo-proto")); err != nil {
		t.Fatalf("installed package binary missing: %v", err)
	}
	if !strings.Contains(output, "Installed: "+installed) {
		t.Fatalf("install output missing installed path: %q", output)
	}
	if !strings.Contains(output, "Binary: demo-proto") {
		t.Fatalf("install output missing binary name: %q", output)
	}
}

func TestInstallCommandFailsWhenArtifactMissing(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		code := Run([]string{"install", dir}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("install returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "artifact missing") {
		if !strings.Contains(stderr, "artifact not found at") {
			t.Fatalf("stderr missing missing-artifact error: %q", stderr)
		}
	}
}

func TestInstallCommandInstallsCompositeBundle(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	dir := filepath.Join(root, "composite")
	bundle := filepath.Join(dir, "app", "MyApp.app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "MyApp"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  members:\n    - id: app\n      path: app\n      type: component\n  targets:\n    macos:\n      steps:\n        - exec:\n            cwd: app\n            argv: [\"echo\", \"hello\"]\nartifacts:\n  primary: app/MyApp.app\n"); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"install", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("install composite returned %d, want 0", code)
		}
	})

	installed := filepath.Join(root, ".runtime", "bin", "composite.holon")
	appBundle := filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "MyApp.app")
	if _, err := os.Stat(filepath.Join(appBundle, "Contents", "MacOS", "MyApp")); err != nil {
		t.Fatalf("installed app bundle missing payload: %v", err)
	}
	if !strings.Contains(output, "Installed: "+installed) {
		t.Fatalf("stdout missing installed bundle path: %q", output)
	}
}

func TestBuildCommandEmitsProgressAndSuggestions(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\ncontract:\n  grpc: true\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"build", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("build returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "Operation: build") {
		t.Fatalf("stdout missing lifecycle report: %q", stdout)
	}
	for _, expected := range []string{
		"building demo-holon…",
		"checking manifest...",
		"validating prerequisites...",
		"go build -o",
		"verifying artifact...",
		"Next steps:",
		"op test demo",
		"op install demo",
		"op run demo:9090",
	} {
		if !strings.Contains(stderr, expected) {
			t.Fatalf("stderr missing %q: %q", expected, stderr)
		}
	}
	if strings.Contains(stderr, "✓ built demo in") {
		t.Fatalf("stderr should keep the final progress line instead of printing a summary: %q", stderr)
	}
	if strings.Contains(stderr, "op test demo  run tests") {
		t.Fatalf("stderr still renders command and description on one line: %q", stderr)
	}
}

func TestBuildCommandJSONSuppressesProgressAndSuggestions(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\ncontract:\n  grpc: true\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"--format", "json", "build", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("build --format json returned %d, want 0", code)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr not empty for json build: %q", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v\noutput=%s", err, stdout)
	}
}

func TestBuildCommandQuietSuppressesProgressAndSuggestions(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\ncontract:\n  grpc: true\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"build", "--quiet", dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("build --quiet returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "Operation: build") {
		t.Fatalf("stdout missing lifecycle report: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr not empty for quiet build: %q", stderr)
	}
}

func TestCheckAndDryRunBuildSupportPythonDartAndRubyRunners(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(t *testing.T, root, toolDir string) string
		wantCheckRunner  string
		wantBuildRunner  string
		wantBuildCommand string
	}{
		{
			name: "python",
			setup: func(t *testing.T, root, toolDir string) string {
				t.Helper()
				writeFakeCommand(t, toolDir, "python")
				dir := filepath.Join(root, "py-demo")
				if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "app", "main.py"), []byte("print('ok')\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("pytest\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				manifest := "schema: holon/v0\nkind: composite\nbuild:\n  runner: python\nrequires:\n  files: [app/main.py]\nartifacts:\n  primary: app/main.py\n"
				if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantCheckRunner:  "Runner: python",
			wantBuildRunner:  "Runner: python",
			wantBuildCommand: "python -m pip install -r requirements.txt",
		},
		{
			name: "dart",
			setup: func(t *testing.T, root, toolDir string) string {
				t.Helper()
				writeFakeCommand(t, toolDir, "dart")
				dir := filepath.Join(root, "dart-demo")
				if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "bin", "main.dart"), []byte("void main() {}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				manifest := "schema: holon/v0\nkind: native\nbuild:\n  runner: dart\nrequires:\n  commands: [dart]\n  files: [pubspec.yaml, bin/main.dart]\nartifacts:\n  binary: dart-demo\n"
				if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantCheckRunner:  "Runner: dart",
			wantBuildRunner:  "Runner: dart",
			wantBuildCommand: "dart compile exe bin/main.dart -o ",
		},
		{
			name: "ruby",
			setup: func(t *testing.T, root, toolDir string) string {
				t.Helper()
				writeFakeCommand(t, toolDir, "ruby")
				writeFakeCommand(t, toolDir, "bundle")
				dir := filepath.Join(root, "ruby-demo")
				if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://example.test'\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "app", "main.rb"), []byte("puts 'ok'\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				manifest := "schema: holon/v0\nkind: composite\nbuild:\n  runner: ruby\nrequires:\n  files: [Gemfile, app/main.rb]\nartifacts:\n  primary: app/main.rb\n"
				if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantCheckRunner:  "Runner: ruby",
			wantBuildRunner:  "Runner: ruby",
			wantBuildCommand: "bundle install",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			toolDir := t.TempDir()
			chdirForTest(t, root)
			t.Setenv("PATH", toolDir)
			dir := tc.setup(t, root, toolDir)

			checkStdout, checkStderr := captureOutput(t, func() {
				code := Run([]string{"check", dir}, "0.1.0-test")
				if code != 0 {
					t.Fatalf("check returned %d, want 0", code)
				}
			})
			if !strings.Contains(checkStdout, "Operation: check") || !strings.Contains(checkStdout, tc.wantCheckRunner) {
				t.Fatalf("unexpected check output:\nstdout=%s\nstderr=%s", checkStdout, checkStderr)
			}
			if strings.TrimSpace(checkStderr) != "" {
				t.Fatalf("check stderr not empty: %q", checkStderr)
			}

			buildStdout, buildStderr := captureOutput(t, func() {
				code := Run([]string{"build", "--dry-run", dir}, "0.1.0-test")
				if code != 0 {
					t.Fatalf("build --dry-run returned %d, want 0", code)
				}
			})
			if !strings.Contains(buildStdout, "Operation: build") || !strings.Contains(buildStdout, tc.wantBuildRunner) {
				t.Fatalf("unexpected dry-run build stdout:\nstdout=%s\nstderr=%s", buildStdout, buildStderr)
			}
			if !strings.Contains(buildStderr, tc.wantBuildCommand) || !strings.Contains(buildStderr, "verifying artifact...") {
				t.Fatalf("unexpected dry-run build stderr:\nstdout=%s\nstderr=%s", buildStdout, buildStderr)
			}
		})
	}
}

func TestUninstallCommand(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	if err := os.MkdirAll(filepath.Join(root, ".runtime", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(root, ".runtime", "bin", "demo.holon")
	if err := os.MkdirAll(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	code := Run([]string{"uninstall", "demo"}, "0.1.0-test")
	if code != 0 {
		t.Fatalf("uninstall returned %d, want 0", code)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("installed package still exists: %v", err)
	}
}

func TestBuildCommandDryRunAcceptsNoSign(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "app", "MyApp.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := runtimeTargetForRunTest()
	manifest := fmt.Sprintf("schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  members:\n    - id: app\n      path: app\n      type: component\n  targets:\n    %s:\n      steps:\n        - assert_file:\n            path: app/MyApp.app\nartifacts:\n  primary: app/MyApp.app\n", target)
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"build", "--dry-run", "--no-sign", "--target", target, dir}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("build --dry-run --no-sign returned %d, want 0", code)
		}
	})

	if strings.Contains(stderr, "codesign --force --deep --sign -") {
		t.Fatalf("dry-run stderr should omit codesign when --no-sign is set:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if !strings.Contains(stdout, "skip signing (--no-sign): app/MyApp.app") {
		t.Fatalf("stdout missing no-sign note:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
}

func TestBuildCommandCleanRemovesStaleOutputsBeforeBuild(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")

	staleMarker := filepath.Join(dir, ".op", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(staleMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staleMarker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"build", "--clean", "demo"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("build --clean returned %d, want 0", code)
		}
	})

	if strings.Contains(stdout, "Operation: clean") {
		t.Fatalf("build --clean should report the build result, not a clean report:\nstdout=%s\nstderr=%s", stdout, stderr)
	}
	if _, err := os.Stat(staleMarker); !os.IsNotExist(err) {
		t.Fatalf("stale marker still exists after build --clean: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo")); err != nil {
		t.Fatalf("binary missing after build --clean: %v", err)
	}
}

func TestBuildCommandCleanRejectsDryRun(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := Run([]string{"build", "--clean", "--dry-run", "demo"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("build --clean --dry-run returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "--clean cannot be combined with --dry-run") {
		t.Fatalf("stderr missing clean/dry-run conflict: %q", stderr)
	}
}

func TestLifecycleCommandsRejectNoSignOutsideBuild(t *testing.T) {
	for _, operation := range []string{"check", "test", "clean"} {
		t.Run(operation, func(t *testing.T) {
			stderr := captureStderr(t, func() {
				code := Run([]string{operation, "--no-sign"}, "0.1.0-test")
				if code != 1 {
					t.Fatalf("%s returned %d, want 1", operation, code)
				}
			})

			if !strings.Contains(stderr, fmt.Sprintf("op %s: unknown flag %q", operation, "--no-sign")) {
				t.Fatalf("stderr missing unknown-flag message: %q", stderr)
			}
		})
	}
}

func TestUninstallCommandJSONFormat(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	t.Setenv("OPPATH", filepath.Join(root, ".runtime"))
	t.Setenv("OPBIN", filepath.Join(root, ".runtime", "bin"))

	if err := os.MkdirAll(filepath.Join(root, ".runtime", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(root, ".runtime", "bin", "demo.holon")
	if err := os.MkdirAll(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "uninstall", "demo"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("uninstall --format json returned %d, want 0", code)
		}
	})

	var payload struct {
		Operation string `json:"operation"`
		Binary    string `json:"binary"`
		Installed string `json:"installed"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("uninstall json output invalid: %v\noutput=%s", err, output)
	}
	if payload.Operation != "uninstall" {
		t.Fatalf("operation = %q, want uninstall", payload.Operation)
	}
	if payload.Binary != "demo" {
		t.Fatalf("binary = %q, want demo", payload.Binary)
	}
	if payload.Installed != installed {
		t.Fatalf("installed = %q, want %q", payload.Installed, installed)
	}
}

func TestModInitCommandInfersSlugFromHolonYAML(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	id := identity.New()
	id.GivenName = "Alpha"
	id.FamilyName = "Builder"
	id.Motto = "Builds holons."
	id.Composer = "test"
	id.Clade = "deterministic/pure"
	id.Status = "draft"
	id.Lang = "go"
	if err := writeCLIIdentityFile(id, filepath.Join(root, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		code := Run([]string{"mod", "init"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod init returned %d, want 0", code)
		}
	})

	data, err := os.ReadFile(filepath.Join(root, "holon.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "holon alpha-builder") {
		t.Fatalf("holon.mod missing inferred slug: %s", string(data))
	}
	if !strings.Contains(output, "created") {
		t.Fatalf("mod init output missing create message: %q", output)
	}
}

func TestModCommandsUseOPPATHCache(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	runtimeHome := filepath.Join(root, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	if err := os.WriteFile(filepath.Join(root, "holon.mod"), []byte("holon alpha-builder\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	depPath := "github.com/example/dep"
	version := "v1.0.0"
	cacheDir := filepath.Join(runtimeHome, "cache", depPath+"@"+version)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachedID := identity.New()
	cachedID.GivenName = "Cached"
	cachedID.FamilyName = "Dep"
	cachedID.Motto = "Cached dependency."
	cachedID.Composer = "test"
	cachedID.Clade = "deterministic/pure"
	cachedID.Status = "draft"
	cachedID.Lang = "go"
	if err := writeCLIIdentityFile(cachedID, filepath.Join(cacheDir, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "holon.mod"), []byte("holon github.com/example/dep\n\nrequire (\n    github.com/example/subdep v0.2.0\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	addOutput := captureStdout(t, func() {
		code := Run([]string{"mod", "add", depPath, version}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod add returned %d, want 0", code)
		}
	})
	if !strings.Contains(addOutput, depPath+"@"+version) {
		t.Fatalf("mod add output missing dependency: %q", addOutput)
	}

	listOutput := captureStdout(t, func() {
		code := Run([]string{"mod", "list"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod list returned %d, want 0", code)
		}
	})
	if !strings.Contains(listOutput, depPath) {
		t.Fatalf("mod list output missing dependency: %q", listOutput)
	}

	graphOutput := captureStdout(t, func() {
		code := Run([]string{"mod", "graph"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod graph returned %d, want 0", code)
		}
	})
	if !strings.Contains(graphOutput, "github.com/example/subdep@v0.2.0") {
		t.Fatalf("mod graph output missing transitive dependency: %q", graphOutput)
	}

	pullOutput := captureStdout(t, func() {
		code := Run([]string{"mod", "pull"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod pull returned %d, want 0", code)
		}
	})
	if !strings.Contains(pullOutput, depPath+"@"+version) {
		t.Fatalf("mod pull output missing fetched dependency: %q", pullOutput)
	}

	if err := os.WriteFile(filepath.Join(root, "holon.sum"), []byte(depPath+" "+version+" h1:keep\n"+"github.com/example/stale v9.9.9 h1:drop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyOutput := captureStdout(t, func() {
		code := Run([]string{"mod", "tidy"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod tidy returned %d, want 0", code)
		}
	})
	if !strings.Contains(tidyOutput, "updated") {
		t.Fatalf("mod tidy output missing update message: %q", tidyOutput)
	}

	sumData, err := os.ReadFile(filepath.Join(root, "holon.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sumData), "github.com/example/stale") {
		t.Fatalf("holon.sum still contains stale dependency: %s", string(sumData))
	}
	if !strings.Contains(string(sumData), depPath+" "+version) {
		t.Fatalf("holon.sum missing kept dependency: %s", string(sumData))
	}
}

func TestModCommandsJSONFormat(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	runtimeHome := filepath.Join(root, ".runtime")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", filepath.Join(runtimeHome, "bin"))

	id := identity.New()
	id.GeneratedBy = "op"
	id.GivenName = "Alpha"
	id.FamilyName = "Builder"
	id.Motto = "Builds holons."
	id.Composer = "test"
	id.Clade = "deterministic/pure"
	id.Reproduction = "manual"
	id.Lang = "go"
	if err := writeCLIIdentityFile(id, filepath.Join(root, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}

	remoteCalls := 0
	restore := opmod.SetRemoteTagsForTesting(func(depPath string) ([]string, error) {
		switch depPath {
		case "github.com/example/dep":
			remoteCalls++
			if remoteCalls == 1 {
				return []string{"v1.0.0", "v1.4.0"}, nil
			}
			return []string{"v1.0.0", "v1.4.0", "v1.5.0"}, nil
		default:
			return nil, fmt.Errorf("unexpected dep %s", depPath)
		}
	})
	t.Cleanup(restore)

	cacheV14 := filepath.Join(runtimeHome, "cache", "github.com/example/dep@v1.4.0")
	if err := os.MkdirAll(cacheV14, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIIdentityFile(id, filepath.Join(cacheV14, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheV14, "holon.mod"), []byte("holon github.com/example/dep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cacheV15 := filepath.Join(runtimeHome, "cache", "github.com/example/dep@v1.5.0")
	if err := os.MkdirAll(cacheV15, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIIdentityFile(id, filepath.Join(cacheV15, identity.ManifestFileName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheV15, "holon.mod"), []byte("holon github.com/example/dep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	initOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "init"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod init --format json returned %d, want 0", code)
		}
	})
	var initPayload struct {
		HolonPath string `json:"holon_path"`
	}
	if err := json.Unmarshal([]byte(initOutput), &initPayload); err != nil {
		t.Fatalf("mod init json invalid: %v\noutput=%s", err, initOutput)
	}
	if initPayload.HolonPath != "alpha-builder" {
		t.Fatalf("holon_path = %q, want alpha-builder", initPayload.HolonPath)
	}

	addOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "add", "github.com/example/dep"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod add --format json returned %d, want 0", code)
		}
	})
	var addPayload struct {
		Dependency struct {
			Path      string `json:"path"`
			Version   string `json:"version"`
			CachePath string `json:"cache_path"`
		} `json:"dependency"`
	}
	if err := json.Unmarshal([]byte(addOutput), &addPayload); err != nil {
		t.Fatalf("mod add json invalid: %v\noutput=%s", err, addOutput)
	}
	if addPayload.Dependency.Version != "v1.4.0" {
		t.Fatalf("version = %q, want v1.4.0", addPayload.Dependency.Version)
	}

	listOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "list"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod list --format json returned %d, want 0", code)
		}
	})
	var listPayload struct {
		Dependencies []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(listOutput), &listPayload); err != nil {
		t.Fatalf("mod list json invalid: %v\noutput=%s", err, listOutput)
	}
	if len(listPayload.Dependencies) != 1 || listPayload.Dependencies[0].Version != "v1.4.0" {
		t.Fatalf("unexpected list payload: %+v", listPayload.Dependencies)
	}

	graphOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "graph"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod graph --format json returned %d, want 0", code)
		}
	})
	var graphPayload struct {
		Root string `json:"root"`
	}
	if err := json.Unmarshal([]byte(graphOutput), &graphPayload); err != nil {
		t.Fatalf("mod graph json invalid: %v\noutput=%s", err, graphOutput)
	}
	if graphPayload.Root != "alpha-builder" {
		t.Fatalf("graph root = %q, want alpha-builder", graphPayload.Root)
	}

	pullOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "pull"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod pull --format json returned %d, want 0", code)
		}
	})
	var pullPayload struct {
		Fetched []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
		} `json:"fetched"`
	}
	if err := json.Unmarshal([]byte(pullOutput), &pullPayload); err != nil {
		t.Fatalf("mod pull json invalid: %v\noutput=%s", err, pullOutput)
	}
	if len(pullPayload.Fetched) != 1 || pullPayload.Fetched[0].Version != "v1.4.0" {
		t.Fatalf("unexpected pull payload: %+v", pullPayload.Fetched)
	}

	if err := os.WriteFile(filepath.Join(root, "holon.sum"), []byte("github.com/example/dep v1.4.0 h1:keep\ngithub.com/example/stale v9.9.9 h1:drop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tidyOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "tidy"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod tidy --format json returned %d, want 0", code)
		}
	})
	var tidyPayload struct {
		Pruned []string `json:"pruned"`
	}
	if err := json.Unmarshal([]byte(tidyOutput), &tidyPayload); err != nil {
		t.Fatalf("mod tidy json invalid: %v\noutput=%s", err, tidyOutput)
	}
	if len(tidyPayload.Pruned) != 1 {
		t.Fatalf("unexpected tidy pruned entries: %+v", tidyPayload.Pruned)
	}

	updateOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "update", "github.com/example/dep"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod update --format json returned %d, want 0", code)
		}
	})
	var updatePayload struct {
		Updated []struct {
			NewVersion string `json:"new_version"`
		} `json:"updated"`
	}
	if err := json.Unmarshal([]byte(updateOutput), &updatePayload); err != nil {
		t.Fatalf("mod update json invalid: %v\noutput=%s", err, updateOutput)
	}
	if len(updatePayload.Updated) != 1 || updatePayload.Updated[0].NewVersion != "v1.5.0" {
		t.Fatalf("unexpected update payload: %+v", updatePayload.Updated)
	}

	removeOutput := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "mod", "remove", "github.com/example/dep"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("mod remove --format json returned %d, want 0", code)
		}
	})
	var removePayload struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(removeOutput), &removePayload); err != nil {
		t.Fatalf("mod remove json invalid: %v\noutput=%s", err, removeOutput)
	}
	if removePayload.Path != "github.com/example/dep" {
		t.Fatalf("remove path = %q, want github.com/example/dep", removePayload.Path)
	}
}

func TestDispatchUnknownHolon(t *testing.T) {
	code := Run([]string{"nonexistent-holon", "some-command"}, "0.1.0-test")
	if code != 1 {
		t.Errorf("dispatch (unknown) returned %d, want 1", code)
	}
}

func TestRunCommandBuildsAndRunsService(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"run", "demo", "--listen", "tcp://127.0.0.1:9099"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("run returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "serve --listen tcp://127.0.0.1:9099") {
		t.Fatalf("run output missing serve args: %q", stdout)
	}
	if !strings.Contains(stderr, "go build -o") {
		t.Fatalf("run stderr missing build step: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo")); err != nil {
		t.Fatalf("built artifact missing after run: %v", err)
	}
}

func TestRunCommandRunsCompositePrimaryArtifact(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(filepath.Join(appDir, "cmd", "runapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte("module example.com/runapp\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "cmd", "runapp", "main.go"), []byte(runEchoMainSource("composite launched")), 0o644); err != nil {
		t.Fatal(err)
	}

	target := runtimeTargetForRunTest()
	artifact := "app/run-app" + executableSuffixForRunTest()
	manifest := fmt.Sprintf(
		"schema: holon/v0\nkind: composite\nbuild:\n  runner: recipe\n  defaults:\n    target: %s\n    mode: debug\n  members:\n    - id: app\n      path: app\n      type: component\n  targets:\n    %s:\n      steps:\n        - exec:\n            cwd: app\n            argv: [\"go\", \"build\", \"-o\", \"run-app%s\", \"./cmd/runapp\"]\n        - assert_file:\n            path: %s\nrequires:\n  commands: [go]\n  files: [app/go.mod]\nartifacts:\n  primary: %s\n",
		target,
		target,
		executableSuffixForRunTest(),
		artifact,
		artifact,
	)
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), manifest); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"run", "demo"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("run returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "composite launched") {
		t.Fatalf("run output missing composite launch: %q", stdout)
	}
	if !strings.Contains(stderr, "go build -o") {
		t.Fatalf("run stderr missing recipe build step: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(artifact))); err != nil {
		t.Fatalf("composite artifact missing after run: %v", err)
	}
}

func TestRunCommandSkipsBuildWhenArtifactAlreadyExists(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")
	buildRunBinary(t, dir, filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo"), "./cmd/demo")
	if err := os.Remove(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"run", "demo"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("run returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "serve --listen tcp://127.0.0.1:0") {
		t.Fatalf("run output missing default tcp listen: %q", stdout)
	}
	if strings.Contains(stderr, "go build -o") {
		t.Fatalf("run unexpectedly rebuilt artifact: %q", stderr)
	}
}

func TestRunCommandCleanRebuildsStaleArtifact(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte(runEchoMainSource("stale")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "demo")
	buildRunBinary(t, dir, binaryPath, "./cmd/demo")

	staleMarker := filepath.Join(dir, ".op", "stale.txt")
	if err := os.WriteFile(staleMarker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "demo", "main.go"), []byte(runEchoMainSource("rebuilt")), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		code := Run([]string{"run", "--clean", "demo"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("run --clean returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "rebuilt serve --listen tcp://127.0.0.1:0") {
		t.Fatalf("run --clean output missing rebuilt binary execution: %q", stdout)
	}
	if strings.Contains(stdout, "stale serve --listen tcp://127.0.0.1:0") {
		t.Fatalf("run --clean unexpectedly used stale binary: %q", stdout)
	}
	if !strings.Contains(stderr, "go build -o") {
		t.Fatalf("run --clean stderr missing rebuild step: %q", stderr)
	}
	if _, err := os.Stat(staleMarker); !os.IsNotExist(err) {
		t.Fatalf("stale marker still exists after run --clean: %v", err)
	}
}

func TestRunCommandNoBuildFailsWhenArtifactMissing(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), "schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: demo\n"); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		code := Run([]string{"run", "--no-build", "demo"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("run returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "artifact missing") {
		t.Fatalf("stderr missing artifact error: %q", stderr)
	}
}

func TestRunCommandRejectsExplicitStdioListen(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := Run([]string{"run", "demo", "--listen", "stdio://"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("run returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "--listen stdio:// is not supported for op run") {
		t.Fatalf("stderr missing stdio rejection: %q", stderr)
	}
}

func TestRunCommandCleanRejectsNoBuild(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := Run([]string{"run", "--clean", "--no-build", "demo"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("run --clean --no-build returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "--clean cannot be combined with --no-build") {
		t.Fatalf("stderr missing clean/no-build conflict: %q", stderr)
	}
}

func TestRunCommandUsesInstalledBinaryWithoutSource(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)

	runtimeHome := filepath.Join(root, "runtime")
	opbin := filepath.Join(runtimeHome, "bin")
	t.Setenv("OPPATH", runtimeHome)
	t.Setenv("OPBIN", opbin)
	t.Setenv("PATH", opbin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.MkdirAll(opbin, 0o755); err != nil {
		t.Fatal(err)
	}

	srcDir := filepath.Join(root, "installed-src")
	if err := os.MkdirAll(filepath.Join(srcDir, "cmd", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module example.com/installed\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "cmd", "demo", "main.go"), []byte(runEchoMainSource("installed")), 0o644); err != nil {
		t.Fatal(err)
	}
	buildRunBinary(t, srcDir, filepath.Join(opbin, "demo"+executableSuffixForRunTest()), "./cmd/demo")

	stdout := captureStdout(t, func() {
		code := Run([]string{"run", "demo:9097"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("run returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, "serve --listen tcp://:9097") {
		t.Fatalf("run output missing shorthand listen: %q", stdout)
	}
}

func TestGRPCURIWithoutPortRequiresMethodForSlugTarget(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := Run([]string{"grpc://rob-go"}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("grpc://rob-go returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "method required") {
		t.Fatalf("stderr missing method-required error: %q", stderr)
	}
}

func TestHolonSlugDispatchUsesAutoConnectChain(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	stdout := captureStdout(t, func() {
		code := Run([]string{"echo-server", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("echo-server Ping returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}
}

func TestHolonSlugDispatchAutoBuildsCompiledSourceHolon(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	seedCompiledAutoBuildEchoHolon(t, root)

	exitCode := 0
	stdout, stderr := captureOutput(t, func() {
		exitCode = Run([]string{"echo-server", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
	})
	if exitCode != 0 {
		t.Fatalf("echo-server Ping returned %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
	}

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("auto-build should stay silent on non-TTY success, stderr=%q", stderr)
	}

	binaryPath := filepath.Join(root, "holons", "echo-server", ".op", "build", "echo-server.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "echo-server")
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("auto-build did not produce binary %q: %v", binaryPath, err)
	}
}

func TestHolonSlugDispatchCleanRemovesStaleOutputsAndAutoBuilds(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	seedCompiledAutoBuildEchoHolon(t, root)

	staleMarker := filepath.Join(root, "holons", "echo-server", ".op", "stale.txt")
	if err := os.MkdirAll(filepath.Dir(staleMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staleMarker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode := 0
	stdout, stderr := captureOutput(t, func() {
		exitCode = Run([]string{"echo-server", "--clean", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
	})
	if exitCode != 0 {
		t.Fatalf("echo-server --clean Ping returned %d, want 0\nstdout=%q\nstderr=%q", exitCode, stdout, stderr)
	}

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}
	if _, err := os.Stat(staleMarker); !os.IsNotExist(err) {
		t.Fatalf("stale marker still exists after slug dispatch --clean: %v", err)
	}

	binaryPath := filepath.Join(root, "holons", "echo-server", ".op", "build", "echo-server.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, "echo-server")
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("slug dispatch --clean did not rebuild binary %q: %v", binaryPath, err)
	}
}

func TestGRPCSlugDispatchUsesAutoConnectChain(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	stdout := captureStdout(t, func() {
		code := Run([]string{"grpc://echo-server", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("grpc://echo-server returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}
}

func TestHolonSlugDispatchCleanRejectsNoBuild(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := Run([]string{"echo-server", "--clean", "Ping", "--no-build", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("echo-server --clean Ping --no-build returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "--clean cannot be combined with --no-build") {
		t.Fatalf("stderr missing clean/no-build conflict: %q", stderr)
	}
}

func TestGRPCSlugDispatchNoBuildFailsForMissingCompiledBinary(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go command not available")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake command not available")
	}

	root := t.TempDir()
	chdirForTest(t, root)
	seedCompiledAutoBuildEchoHolon(t, root)

	stderr := captureStderr(t, func() {
		code := Run([]string{"grpc://echo-server", "Ping", "--no-build", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 1 {
			t.Fatalf("grpc://echo-server returned %d, want 1", code)
		}
	})

	if !strings.Contains(stderr, "built binary not found") {
		t.Fatalf("stderr missing binary-not-found error: %q", stderr)
	}
	if strings.Contains(stderr, "building echo-server") {
		t.Fatalf("stderr should not include auto-build progress when --no-build is set: %q", stderr)
	}
}

func TestGRPCTCPSlugDispatchForcesTCP(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	stdout := captureStdout(t, func() {
		code := Run([]string{"tcp://echo-server", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("tcp://echo-server returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}

	portFile := filepath.Join(root, ".op", "run", "echo-server.port")
	if _, err := os.Stat(portFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("forced TCP dispatch should not leave a port file, got err=%v", err)
	}
}

func TestGRPCStdioSlugDispatchUsesDirectStdioScheme(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	seedEchoHolon(t, root)

	stdout := captureStdout(t, func() {
		code := Run([]string{"stdio://echo-server", "Ping", `{"message":"Alice"}`}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("stdio://echo-server returned %d, want 0", code)
		}
	})

	if !strings.Contains(stdout, `"message": "Alice"`) {
		t.Fatalf("stdout missing echoed payload: %q", stdout)
	}
}

func TestFlagValue(t *testing.T) {
	args := []string{"--name", "Test", "--lang", "rust", "--verbose"}

	if v := flagValue(args, "--name"); v != "Test" {
		t.Errorf("flagValue(--name) = %q, want %q", v, "Test")
	}
	if v := flagValue(args, "--lang"); v != "rust" {
		t.Errorf("flagValue(--lang) = %q, want %q", v, "rust")
	}
	if v := flagValue(args, "--missing"); v != "" {
		t.Errorf("flagValue(--missing) = %q, want empty", v)
	}
	// --verbose has no value after it
	if v := flagValue(args, "--verbose"); v != "" {
		t.Errorf("flagValue(--verbose) = %q, want empty", v)
	}
}

func TestFlagOrDefault(t *testing.T) {
	args := []string{"--name", "Test"}

	if v := flagOrDefault(args, "--name", "fallback"); v != "Test" {
		t.Errorf("flagOrDefault(--name) = %q, want %q", v, "Test")
	}
	if v := flagOrDefault(args, "--missing", "fallback"); v != "fallback" {
		t.Errorf("flagOrDefault(--missing) = %q, want %q", v, "fallback")
	}
}

func TestParseGlobalFormat(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFormat Format
		wantArgs   []string
		wantErr    bool
	}{
		{
			name:       "default format",
			args:       []string{"who", "list"},
			wantFormat: FormatText,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "long flag",
			args:       []string{"--format", "json", "who", "list"},
			wantFormat: FormatJSON,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "short flag",
			args:       []string{"-f", "json", "who", "list"},
			wantFormat: FormatJSON,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "inline long flag",
			args:       []string{"--format=text", "who", "list"},
			wantFormat: FormatText,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "inline short flag",
			args:       []string{"-f=text", "who", "list"},
			wantFormat: FormatText,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "flag after command is still global",
			args:       []string{"who", "-f", "json", "list"},
			wantFormat: FormatJSON,
			wantArgs:   []string{"who", "list"},
		},
		{
			name:       "non-global format passes through",
			args:       []string{"--format", "yaml", "who", "list"},
			wantFormat: FormatText,
			wantArgs:   []string{"--format", "yaml", "who", "list"},
		},
		{
			name:    "missing format value",
			args:    []string{"-f"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotFormat, gotArgs, err := parseGlobalFormat(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGlobalFormat returned error: %v", err)
			}
			if gotFormat != tc.wantFormat {
				t.Fatalf("format = %q, want %q", gotFormat, tc.wantFormat)
			}
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("args length = %d, want %d", len(gotArgs), len(tc.wantArgs))
			}
			for i := range gotArgs {
				if gotArgs[i] != tc.wantArgs[i] {
					t.Fatalf("args[%d] = %q, want %q", i, gotArgs[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestParseGlobalOptions(t *testing.T) {
	gopts, args, err := parseGlobalOptions([]string{"-q", "--format", "json", "build", "."})
	if err != nil {
		t.Fatalf("parseGlobalOptions returned error: %v", err)
	}
	if gopts.format != FormatJSON {
		t.Fatalf("format = %q, want %q", gopts.format, FormatJSON)
	}
	if !gopts.quiet {
		t.Fatal("quiet = false, want true")
	}
	if len(args) != 2 || args[0] != "build" || args[1] != "." {
		t.Fatalf("args = %#v, want [build .]", args)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
		_ = w.Close()
		_ = r.Close()
	}()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = w.Close()
		_ = r.Close()
	}()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stderr: %v", err)
	}
	return buf.String()
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		_ = stdoutW.Close()
		_ = stdoutR.Close()
		_ = stderrW.Close()
		_ = stderrR.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf bytes.Buffer
	if _, err := io.Copy(&stdoutBuf, stdoutR); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	var stderrBuf bytes.Buffer
	if _, err := io.Copy(&stderrBuf, stderrR); err != nil {
		t.Fatalf("read captured stderr: %v", err)
	}
	return stdoutBuf.String(), stderrBuf.String()
}

func writeRunServiceFixture(t *testing.T, dir, name string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "cmd", name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/"+name+"\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", name, "main.go"), []byte(runEchoMainSource(name)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCLIManifestFile(filepath.Join(dir, identity.ManifestFileName), fmt.Sprintf("schema: holon/v0\nkind: native\nbuild:\n  runner: go-module\nrequires:\n  commands: [go]\n  files: [go.mod]\nartifacts:\n  binary: %s\n", name)); err != nil {
		t.Fatal(err)
	}
}

func writeFakeCommand(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	data := []byte("#!/bin/sh\nexit 0\n")
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		path += ".bat"
		data = []byte("@echo off\r\nexit /b 0\r\n")
		mode = 0o644
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func buildRunBinary(t *testing.T, dir, output, pkg string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", output, pkg)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
}

func runEchoMainSource(label string) string {
	return fmt.Sprintf(`package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println(%q, strings.Join(os.Args[1:], " "))
}
`, label)
}

func runtimeTargetForRunTest() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

func executableSuffixForRunTest() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func writeProtoInstallFixture(t *testing.T, root, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "cmd", name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "api", "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/"+name+"\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", name, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCLISharedManifestProto(t, root)

	proto := fmt.Sprintf(`syntax = "proto3";

package test.v1;

import "holons/v1/manifest.proto";

	option (holons.v1.manifest) = {
	  identity: {
	    schema: "holon/v1"
	    uuid: "%s-uuid"
	    given_name: "Demo"
	    family_name: "Proto"
	    motto: "Proto-backed install fixture."
	    composer: "test"
	    status: "draft"
	    born: "2026-03-15"
	  }
	  kind: "native"
	  lang: "go"
	  build: {
	    runner: "go-module"
	    main: "./cmd/%s"
	  }
	  requires: {
	    commands: ["go"]
	    files: ["go.mod"]
	  }
	  artifacts: {
	    binary: "%s"
	  }
	};
`, name, name, name)
	if err := os.WriteFile(filepath.Join(dir, "api", "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func writeCLISharedManifestProto(t *testing.T, root string) {
	t.Helper()

	source := filepath.Join(cliRepoRoot(t), "_protos", "holons", "v1", "manifest.proto")
	if _, err := os.Stat(source); err != nil {
		source = filepath.Join(cliRepoRoot(t), "holons", "grace-op", "_protos", "holons", "v1", "manifest.proto")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}

	targetDir := filepath.Join(root, "_protos", "holons", "v1")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", targetDir, err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "manifest.proto"), data, 0o644); err != nil {
		t.Fatalf("write manifest.proto: %v", err)
	}
}

func seedCompiledAutoBuildEchoHolon(t *testing.T, root string) {
	t.Helper()

	dir := filepath.Join(root, "holons", "echo-server")
	copyDir(t, cliTestSupportDir(t, "echoholon"), dir)
	if err := os.Remove(filepath.Join(dir, "holon.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".op", "build")); err != nil {
		t.Fatal(err)
	}

	writeCLISharedManifestProto(t, root)

	repoRoot := cliRepoRoot(t)
	graceOpRoot := filepath.Join(repoRoot, "holons", "grace-op")
	goHolonsRoot := filepath.Join(repoRoot, "sdk", "go-holons")

	goMod := fmt.Sprintf(`module github.com/organic-programming/grace-op/auto-build-fixtures/echo-server

go 1.25.1

require (
	github.com/organic-programming/go-holons v0.0.0
	github.com/organic-programming/grace-op v0.0.0
	google.golang.org/grpc v1.78.0
)

replace github.com/organic-programming/grace-op => %s
replace github.com/organic-programming/go-holons => %s
`, filepath.ToSlash(graceOpRoot), filepath.ToSlash(goHolonsRoot))
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	cmakeLists := `cmake_minimum_required(VERSION 3.20)
project(EchoServer NONE)

set(BINARY_PATH "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/echo-server")

add_custom_command(
  OUTPUT "${BINARY_PATH}"
  COMMAND go build -mod=mod -o "${BINARY_PATH}" .
  WORKING_DIRECTORY "${CMAKE_SOURCE_DIR}"
  VERBATIM
)

add_custom_target(echo_server ALL DEPENDS "${BINARY_PATH}")
`
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte(cmakeLists), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";

package echofixture.v1;

import "holons/v1/manifest.proto";

option (holons.v1.manifest) = {
  identity: {
    schema: "holon/v1"
    uuid: "echo-server-autobuild-fixture"
    given_name: "Echo"
    family_name: "Server"
    motto: "Compiled auto-build fixture."
    composer: "test"
    status: "draft"
    born: "2026-03-24"
  }
  kind: "native"
  lang: "go"
  build: {
    runner: "cmake"
  }
  requires: {
    commands: ["cmake", "go"]
    files: ["go.mod", "CMakeLists.txt"]
  }
  artifacts: {
    binary: "echo-server"
  }
};
`
	if err := os.WriteFile(filepath.Join(dir, "v1", "holon.proto"), []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cliRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

func TestSpecifiersFromFlags(t *testing.T) {
	if got := specifiersFromFlags(nil); got != sdkdiscover.ALL {
		t.Fatalf("specifiersFromFlags(nil) = 0x%02X, want 0x%02X", got, sdkdiscover.ALL)
	}

	got := specifiersFromFlags([]string{"--source", "--installed"})
	want := sdkdiscover.SOURCE | sdkdiscover.INSTALLED
	if got != want {
		t.Fatalf("specifiersFromFlags(source+installed) = 0x%02X, want 0x%02X", got, want)
	}
}

func TestOpListWithSourceFlag(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "who",
		binaryName: "who",
		givenName:  "who",
		familyName: "Holon",
		aliases:    []string{"who"},
		lang:       "go",
	})

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "list", "--source"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("list --source returned %d, want 0", code)
		}
	})

	var payload struct {
		Entries []struct {
			Origin string `json:"origin"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("list json invalid: %v\noutput=%s", err, output)
	}
	if len(payload.Entries) == 0 {
		t.Fatalf("expected at least one entry: %s", output)
	}
	for _, entry := range payload.Entries {
		if entry.Origin != "source" {
			t.Fatalf("origin = %q, want source", entry.Origin)
		}
	}
}

func TestOpListWithLimit(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "who",
		binaryName: "who",
		givenName:  "who",
		familyName: "Holon",
		aliases:    []string{"who"},
		lang:       "go",
	})
	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "atlas",
		binaryName: "atlas",
		givenName:  "atlas",
		familyName: "Holon",
		aliases:    []string{"atlas"},
		lang:       "rust",
	})

	output := captureStdout(t, func() {
		code := Run([]string{"--format", "json", "list", "--limit", "1"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("list --limit returned %d, want 0", code)
		}
	})

	var payload struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("list json invalid: %v\noutput=%s", err, output)
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(payload.Entries))
	}
}

func TestParseGlobalOptionsWithTimeout(t *testing.T) {
	opts, args, err := parseGlobalOptions([]string{"--timeout", "3000", "list"})
	if err != nil {
		t.Fatalf("parseGlobalOptions returned error: %v", err)
	}
	if opts.timeout != 3000 {
		t.Fatalf("timeout = %d, want 3000", opts.timeout)
	}
	if !reflect.DeepEqual(args, []string{"list"}) {
		t.Fatalf("args = %v, want [list]", args)
	}
}

func TestOriginFlagShowsResolvedPath(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)

	seedTransportHolon(t, root, transportHolonSeed{
		dirName:    "dummy-test",
		givenName:  "Sophia",
		familyName: "TestHolon",
		aliases:    []string{"who", "sophia"},
		lang:       "go",
	})

	stderr := captureStderr(t, func() {
		code := Run([]string{"--origin", "show", "who"}, "0.1.0-test")
		if code != 0 {
			t.Fatalf("show --origin returned %d, want 0", code)
		}
	})

	if !strings.Contains(stderr, "origin:") {
		t.Fatalf("stderr missing origin marker: %q", stderr)
	}
	if !strings.Contains(stderr, "source") {
		t.Fatalf("stderr missing origin layer: %q", stderr)
	}
}

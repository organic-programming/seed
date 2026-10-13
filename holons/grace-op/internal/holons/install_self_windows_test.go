//go:build windows

package holons

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
)

// The subprocess is a real mapped PE image named op.exe. All install paths and
// PATH entries are temporary; the test never changes the operator's OP or PATH.
func TestWindowsSelfInstallInvokedExecutable(t *testing.T) {
	if child := os.Getenv("OP_SELF_INSTALL_TEST_CHILD"); child != "" {
		if child == "install" {
			if _, err := Install(os.Getenv("OP_SELF_INSTALL_TEST_SOURCE"), InstallOptions{ResolveSpecifiers: sdkdiscover.SOURCE}); err != nil {
				t.Fatal(err)
			}
			fmt.Println("installed")
			_, _ = io.Copy(io.Discard, os.Stdin) // Keep the replaced image mapped.
		} else if child == "probe" {
			fmt.Println("replacement executable started")
		} else {
			t.Fatalf("unknown child mode %q", child)
		}
		return
	}

	root := t.TempDir()
	t.Setenv("OPPATH", filepath.Join(root, "runtime"))
	t.Setenv("OPBIN", filepath.Join(root, "runtime", "bin"))
	dir := writeProtoGoHolonFixture(t, filepath.Join(root, "source"), "op")
	manifest, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.BinaryPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeHolonJSON(manifest, BuildContext{}); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	legacyBin := filepath.Join(root, "go", "bin")
	canonicalBin := os.Getenv("OPBIN")
	bootstrap := filepath.Join(root, "bootstrap", "op.exe")
	legacy := filepath.Join(legacyBin, "op.exe")
	untouched := filepath.Join(root, "unrelated", "op.exe")
	for _, path := range []string{bootstrap, legacy, untouched} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, image, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Preserve the deliberately inconvenient shell order throughout both steps.
	t.Setenv("PATH", strings.Join([]string{legacyBin, canonicalBin, filepath.Dir(untouched), os.Getenv("PATH")}, string(os.PathListSeparator)))
	pathBefore := os.Getenv("PATH")

	for _, step := range []struct {
		name, invoked, opbin string
	}{
		{"bootstrap with explicit OPBIN", bootstrap, legacyBin},
		{"ordinary shell still resolves go bin first", legacy, canonicalBin},
		{"direct installed package invocation", PackageBinaryPath(filepath.Join(canonicalBin, "op.holon"), "op"), canonicalBin},
	} {
		t.Run(step.name, func(t *testing.T) {
			// A legal PE overlay makes old/new bytes observably different without
			// compiling a second program or replacing an actual installed OP.
			newImage := append(bytes.Clone(image), []byte("\nself-install generation: "+step.name)...)
			if err := os.WriteFile(manifest.BinaryPath(), newImage, 0o755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, step.invoked, "-test.run=^TestWindowsSelfInstallInvokedExecutable$")
			command.Dir = root
			command.Env = append(os.Environ(), "OPBIN="+step.opbin, "OP_SELF_INSTALL_TEST_CHILD=install", "OP_SELF_INSTALL_TEST_SOURCE="+dir)
			stdin, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = stdin.Close()
				if err := command.Wait(); err != nil {
					t.Errorf("install subprocess: %v\n%s", err, stderr.String())
				}
			}()
			if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "installed\n" {
				t.Fatalf("self-install result = %q, %v\n%s", line, err, stderr.String())
			}
			for _, path := range []string{step.invoked, filepath.Join(step.opbin, "op.exe"), PackageBinaryPath(filepath.Join(step.opbin, "op.holon"), "op")} {
				got, err := os.ReadFile(path)
				if err != nil || sha256.Sum256(got) != sha256.Sum256(newImage) {
					t.Fatalf("self-install did not publish exact new bytes at %s: %v", path, err)
				}
			}
			// The next command resolves using the same shell PATH while the old
			// process is still alive. The canonical directory need not come first.
			resolved, err := exec.LookPath("op.exe")
			if err != nil || !sameWindowsInstallPath(resolved, legacy) {
				t.Fatalf("shell resolution changed: %q, %v", resolved, err)
			}
			probeExecutable := resolved
			if step.name == "direct installed package invocation" {
				probeExecutable = step.invoked
			}
			probe := exec.CommandContext(ctx, probeExecutable, "-test.run=^TestWindowsSelfInstallInvokedExecutable$")
			probe.Env = append(os.Environ(), "OP_SELF_INSTALL_TEST_CHILD=probe")
			if output, err := probe.CombinedOutput(); err != nil || !bytes.Contains(output, []byte("replacement executable started")) {
				t.Fatalf("new image cannot start while previous image is running: %v\n%s", err, output)
			}
			other, err := os.ReadFile(untouched)
			if err != nil || !bytes.Equal(other, image) || os.Getenv("PATH") != pathBefore {
				t.Fatalf("self-install changed another PATH entry or the environment: %v", err)
			}
		})
	}
}

func TestWindowsSelfInstallMissingBinaryPreservesPackage(t *testing.T) {
	root := t.TempDir()
	artifact, installed := filepath.Join(root, "new.holon"), filepath.Join(root, "op.holon")
	for _, path := range []string{artifact, filepath.Dir(PackageBinaryPath(installed, "op"))} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	previous := PackageBinaryPath(installed, "op")
	if err := os.WriteFile(previous, []byte("previous executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installWindowsSelf(artifact, installed, "op"); err == nil {
		t.Fatal("self-install accepted a missing new executable")
	}
	if got, err := os.ReadFile(previous); err != nil || string(got) != "previous executable" {
		t.Fatalf("invalid artifact changed the previous package: %q, %v", got, err)
	}
}

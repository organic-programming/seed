package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/grace-op/internal/holons"
)

func dialogProcess(pid int, path string) holons.ActiveArtifactProcess {
	return holons.ActiveArtifactProcess{PID: pid, Path: filepath.FromSlash(path), Started: "fixture-start"}
}

func dialogFixture(input string, processes []holons.ActiveArtifactProcess) (artifactDialog, *bytes.Buffer, *[]holons.ActiveArtifactProcess) {
	output := &bytes.Buffer{}
	active := append([]holons.ActiveArtifactProcess(nil), processes...)
	return artifactDialog{
		input:       strings.NewReader(input),
		output:      output,
		interactive: true,
		check: func() error {
			if len(active) == 0 {
				return nil
			}
			return &holons.ActiveArtifactConflict{Operation: holons.OperationBuild, Holon: "fixture", Processes: append([]holons.ActiveArtifactProcess(nil), active...)}
		},
		gracePeriod: time.Millisecond,
		forcePeriod: time.Millisecond,
		pollPeriod:  time.Millisecond,
	}, output, &active
}

func TestArtifactDialogRefusalAndEOFNeverStop(t *testing.T) {
	for _, input := range []string{"n\n", "\n", "", "y", "o\n", "oui\n", "whatever\n"} {
		t.Run(input, func(t *testing.T) {
			dialog, output, _ := dialogFixture(input, []holons.ActiveArtifactProcess{dialogProcess(42, "/fixture/app")})
			dialog.stop = func(context.Context, holons.ActiveArtifactProcess, bool) error {
				t.Fatal("refusal or EOF stopped a process")
				return nil
			}
			if err := dialog.resolve(context.Background()); !errors.Is(err, errArtifactRunCancelled) {
				t.Fatalf("got %v, want cancellation", err)
			}
			for _, want := range []string{"PID 42", filepath.FromSlash("/fixture/app"), "interrupt work in progress", "This application is using files that need to be rebuilt. Close it and continue? [y/N]"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("output %q lacks %q", output.String(), want)
				}
			}
		})
	}
}

type forbiddenDialogInput struct{ t *testing.T }

func (r forbiddenDialogInput) Read([]byte) (int, error) {
	r.t.Fatal("noninteractive invocation attempted to read input")
	return 0, io.EOF
}

func TestArtifactDialogNonInteractiveReturnsStructuredConflict(t *testing.T) {
	dialog, output, _ := dialogFixture("", []holons.ActiveArtifactProcess{dialogProcess(42, "/fixture/app")})
	dialog.interactive = false
	dialog.input = forbiddenDialogInput{t}
	dialog.stop = func(context.Context, holons.ActiveArtifactProcess, bool) error {
		t.Fatal("noninteractive invocation stopped a process")
		return nil
	}
	var conflict *holons.ActiveArtifactConflict
	if err := dialog.resolve(context.Background()); !errors.As(err, &conflict) || len(conflict.Processes) != 1 {
		t.Fatalf("expected structured diagnostic, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("noninteractive invocation displayed a prompt: %q", output.String())
	}
}

func TestArtifactDialogGracefulApplicationClosureWaitsForWorkers(t *testing.T) {
	app := dialogProcess(41, "/fixture/Demo.app/Contents/MacOS/Demo")
	app.BundlePath = filepath.FromSlash("/fixture/Demo.app")
	worker := dialogProcess(42, "/fixture/Demo.app/Contents/Frameworks/worker")
	dialog, output, active := dialogFixture(" YES \n", []holons.ActiveArtifactProcess{worker, app})
	stops := 0
	dialog.stop = func(_ context.Context, process holons.ActiveArtifactProcess, force bool) error {
		stops++
		if force || process.PID != app.PID {
			t.Fatalf("unexpected stop: process=%+v force=%v", process, force)
		}
		*active = nil // App closes and reaps its worker.
		return nil
	}
	if err := dialog.resolve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if stops != 1 || strings.Contains(output.String(), "Force") {
		t.Fatalf("stops=%d output=%q", stops, output.String())
	}
}

func TestArtifactDialogOrphanGetsNormalStop(t *testing.T) {
	worker := dialogProcess(42, "/fixture/Demo.app/Contents/Frameworks/worker")
	dialog, _, active := dialogFixture("y\n", []holons.ActiveArtifactProcess{worker})
	stops := 0
	dialog.stop = func(_ context.Context, process holons.ActiveArtifactProcess, force bool) error {
		stops++
		if force || process != worker {
			t.Fatalf("unexpected orphan stop: process=%+v force=%v", process, force)
		}
		*active = nil
		return nil
	}
	if err := dialog.resolve(context.Background()); err != nil || stops != 1 {
		t.Fatalf("err=%v stops=%d", err, stops)
	}
}

func TestArtifactDialogTimeoutNeedsSeparateConsent(t *testing.T) {
	for _, reply := range []string{"n\n", "\n", "", "y\n"} {
		t.Run(reply, func(t *testing.T) {
			app := dialogProcess(41, "/fixture/Demo.app/Contents/MacOS/Demo")
			app.BundlePath = filepath.FromSlash("/fixture/Demo.app")
			worker := dialogProcess(42, "/fixture/Demo.app/Contents/Frameworks/worker")
			dialog, output, active := dialogFixture("y\n"+reply, []holons.ActiveArtifactProcess{app, worker})
			forced := 0
			dialog.stop = func(_ context.Context, process holons.ActiveArtifactProcess, force bool) error {
				if !force {
					if process.PID != app.PID {
						t.Fatal("worker stopped before waiting for application")
					}
					*active = []holons.ActiveArtifactProcess{worker}
					return nil
				}
				forced++
				if process.PID != worker.PID {
					t.Fatalf("forced stale process: %+v", process)
				}
				*active = nil
				return nil
			}
			err := dialog.resolve(context.Background())
			if reply == "y\n" {
				if err != nil || forced != 1 {
					t.Fatalf("err=%v forced=%d", err, forced)
				}
			} else if !errors.Is(err, errArtifactRunCancelled) || forced != 0 {
				t.Fatalf("err=%v forced=%d", err, forced)
			}
			if !strings.Contains(output.String(), "after the shutdown timeout") || !strings.Contains(output.String(), "Force these processes to stop") {
				t.Fatalf("missing timeout and force question: %q", output.String())
			}
		})
	}
}

func TestArtifactDialogReinventoriesAfterForce(t *testing.T) {
	worker := dialogProcess(42, "/fixture/app/worker")
	restarted := dialogProcess(43, "/fixture/app/worker")
	dialog, _, active := dialogFixture("y\ny\n", []holons.ActiveArtifactProcess{worker})
	forced := 0
	dialog.stop = func(_ context.Context, process holons.ActiveArtifactProcess, force bool) error {
		if force {
			forced++
			*active = []holons.ActiveArtifactProcess{restarted}
		}
		return nil
	}
	var conflict *holons.ActiveArtifactConflict
	if err := dialog.resolve(context.Background()); !errors.As(err, &conflict) || conflict.Processes[0].PID != restarted.PID {
		t.Fatalf("new worker did not block continuation: %v", err)
	}
	if forced != 1 {
		t.Fatalf("automatically forced new worker: %d requests", forced)
	}
}

func TestArtifactDialogIdentityChangeAborts(t *testing.T) {
	dialog, _, _ := dialogFixture("y\ny\n", []holons.ActiveArtifactProcess{dialogProcess(42, "/fixture/app")})
	stops := 0
	dialog.stop = func(context.Context, holons.ActiveArtifactProcess, bool) error {
		stops++
		return holons.ErrActiveArtifactProcessChanged
	}
	if err := dialog.resolve(context.Background()); !errors.Is(err, errArtifactRunCancelled) || stops != 1 {
		t.Fatalf("err=%v stops=%d", err, stops)
	}
}

type onFirstRead struct {
	reader io.Reader
	before func()
}

func (r *onFirstRead) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.reader.Read(p)
}

func TestArtifactDialogPIDReusedDuringForceConsentIsNotStopped(t *testing.T) {
	original := dialogProcess(42, "/fixture/app/worker")
	dialog, _, active := dialogFixture("", []holons.ActiveArtifactProcess{original})
	dialog.input = io.MultiReader(strings.NewReader("y\n"), &onFirstRead{
		reader: strings.NewReader("y\n"),
		before: func() {
			// Same PID and path, but a different process started while the
			// user was considering the force question.
			(*active)[0].Started = "replacement-start"
		},
	})
	attempts, killed := 0, 0
	dialog.stop = func(_ context.Context, process holons.ActiveArtifactProcess, force bool) error {
		if !force {
			return nil
		}
		attempts++
		if process.Started != (*active)[0].Started {
			return holons.ErrActiveArtifactProcessChanged
		}
		killed++
		return nil
	}
	if err := dialog.resolve(context.Background()); !errors.Is(err, errArtifactRunCancelled) || attempts != 1 || killed != 0 {
		t.Fatalf("err=%v force attempts=%d processes killed=%d", err, attempts, killed)
	}
}

func TestArtifactDialogInterruptionDuringPromptAndWait(t *testing.T) {
	for _, during := range []string{"prompt", "wait"} {
		t.Run(during, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dialog, _, _ := dialogFixture("y\n", []holons.ActiveArtifactProcess{dialogProcess(42, "/fixture/app")})
			if during == "prompt" {
				reader, writer := io.Pipe()
				defer reader.Close()
				defer writer.Close()
				dialog.input = reader
				time.AfterFunc(time.Millisecond, cancel)
			}
			dialog.stop = func(context.Context, holons.ActiveArtifactProcess, bool) error {
				if during == "prompt" {
					t.Fatal("interrupted prompt stopped a process")
				}
				cancel()
				return nil
			}
			if err := dialog.resolve(ctx); !errors.Is(err, errArtifactRunCancelled) {
				t.Fatalf("got %v, want cancellation", err)
			}
		})
	}
}

func TestArtifactDialogDoesNotTreatOtherErrorsAsConflicts(t *testing.T) {
	dialog, output, _ := dialogFixture("y\n", nil)
	want := errors.New("compile failed: Refusing to rebuild or replace fixture")
	dialog.check = func() error { return want }
	if err := dialog.resolve(context.Background()); err != want || output.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, output.String())
	}
}

func TestWorkerHasApplicationUsesPathBoundary(t *testing.T) {
	app := dialogProcess(41, "/checkout/Demo.app/Contents/MacOS/Demo")
	app.BundlePath = filepath.FromSlash("/checkout/Demo.app")
	for path, want := range map[string]bool{
		"/checkout/Demo.app/Contents/worker": true,
		"/checkout/Demo.app-other/worker":    false,
		"/other-checkout/Demo.app/worker":    false,
	} {
		if got := workerHasApplication(dialogProcess(42, path), []holons.ActiveArtifactProcess{app}); got != want {
			t.Fatalf("path=%s got=%v want=%v", path, got, want)
		}
	}
}

func TestRunArtifactPreflightPrecedesClean(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")
	marker := filepath.Join(dir, ".op", "preserve.txt")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	preflight := func(_ *holons.LoadedManifest, _ holons.BuildContext, operation holons.Operation) error {
		calls++
		if operation != holons.OperationClean {
			t.Fatalf("operation=%s", operation)
		}
		return errArtifactRunCancelled
	}
	captureOutput(t, func() {
		if code := cmdRunWithArtifactPreflight(FormatText, commandRuntimeOptions{}, []string{"--clean", dir}, preflight); code != 1 {
			t.Fatalf("exit=%d", code)
		}
	})
	if data, err := os.ReadFile(marker); err != nil || string(data) != "unchanged" || calls != 1 {
		t.Fatalf("clean ran before cancellation: data=%q err=%v calls=%d", data, err, calls)
	}
}

func TestRunExistingArtifactSkipsPreflight(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")
	buildRunBinary(t, dir, filepath.Join(dir, ".op", "build", "demo.holon", "bin", runtime.GOOS+"_"+runtime.GOARCH, hostTestExecutableName("demo")), "./cmd/demo")
	captureOutput(t, func() {
		code := cmdRunWithArtifactPreflight(FormatText, commandRuntimeOptions{}, []string{dir}, func(*holons.LoadedManifest, holons.BuildContext, holons.Operation) error {
			t.Fatal("existing artifact should launch without process preflight")
			return nil
		})
		if code != 0 {
			t.Fatalf("exit=%d", code)
		}
	})
}

func TestRunCleanContinuesAfterAcceptedClosure(t *testing.T) {
	root := t.TempDir()
	chdirForTest(t, root)
	dir := filepath.Join(root, "demo")
	writeRunServiceFixture(t, dir, "demo")
	marker := filepath.Join(dir, ".op", "preserve.txt")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	dialog, _, active := dialogFixture("y\n", []holons.ActiveArtifactProcess{dialogProcess(42, "/fixture/worker")})
	dialog.stop = func(context.Context, holons.ActiveArtifactProcess, bool) error {
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("clean started before closure: %v", err)
		}
		*active = nil
		return nil
	}
	stdout, _ := captureOutput(t, func() {
		code := cmdRunWithArtifactPreflight(FormatText, commandRuntimeOptions{}, []string{"--clean", dir}, func(*holons.LoadedManifest, holons.BuildContext, holons.Operation) error {
			return dialog.resolve(context.Background())
		})
		if code != 0 {
			t.Fatalf("exit=%d", code)
		}
	})
	if !strings.Contains(stdout, "demo serve --listen") {
		t.Fatalf("application not launched after closure: %q", stdout)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("clean did not run after closure: %v", err)
	}
}

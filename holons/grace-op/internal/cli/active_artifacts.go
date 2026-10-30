package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/organic-programming/grace-op/internal/holons"
	"golang.org/x/term"
)

const (
	artifactGracePeriod = 10 * time.Second
	artifactForcePeriod = 5 * time.Second
	artifactPollPeriod  = 100 * time.Millisecond
)

var errArtifactRunCancelled = errors.New("operation cancelled; no artifacts were modified")

// All interaction belongs to the CLI. Neither the checker nor the lifecycle
// receives a permission flag: they always take a fresh process inventory.
func prepareRunArtifacts(format Format, manifest *holons.LoadedManifest, buildCtx holons.BuildContext, operation holons.Operation) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	dialog := artifactDialog{
		input:       os.Stdin,
		output:      os.Stderr,
		interactive: format != FormatJSON && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())),
		check:       func() error { return holons.CheckActiveArtifacts(manifest, buildCtx, operation) },
		stop:        holons.RequestActiveArtifactStop,
		gracePeriod: artifactGracePeriod,
		forcePeriod: artifactForcePeriod,
		pollPeriod:  artifactPollPeriod,
	}
	return dialog.resolve(ctx)
}

type artifactDialog struct {
	input       io.Reader
	output      io.Writer
	interactive bool
	check       func() error
	stop        func(context.Context, holons.ActiveArtifactProcess, bool) error
	gracePeriod time.Duration
	forcePeriod time.Duration
	pollPeriod  time.Duration
}

func (d artifactDialog) resolve(ctx context.Context) error {
	err := d.check()
	if err == nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
		}
		return nil
	}
	var conflict *holons.ActiveArtifactConflict
	if !errors.As(err, &conflict) || !d.interactive {
		return err
	}
	d.showProcesses(conflict.Processes)
	fmt.Fprintln(d.output, "Closing the application may interrupt work in progress.")
	if err := d.confirm(ctx, "This application is using files that need to be rebuilt. Close it and continue? [y/N] "); err != nil {
		return err
	}

	// Ask applications first. Workers under their bundle get the entire grace
	// period to exit normally with them. An orphan has no application to ask,
	// so its normal shutdown is a separately identity-checked SIGTERM.
	var apps []holons.ActiveArtifactProcess
	for _, process := range conflict.Processes {
		if process.BundlePath != "" {
			apps = append(apps, process)
		}
	}
	for _, process := range apps {
		if err := d.requestStop(ctx, process, false); err != nil {
			return err
		}
	}
	for _, process := range conflict.Processes {
		if process.BundlePath == "" && !workerHasApplication(process, apps) {
			if err := d.requestStop(ctx, process, false); err != nil {
				return err
			}
		}
	}

	err = d.wait(ctx, d.gracePeriod)
	if err == nil {
		return nil
	}
	if !errors.As(err, &conflict) {
		return err
	}
	fmt.Fprintln(d.output, "Processes are still using the files after the shutdown timeout:")
	d.showProcesses(conflict.Processes)
	fmt.Fprintln(d.output, "Forcing termination may lose work in progress or unsaved data.")
	if err := d.confirm(ctx, "Force these processes to stop and continue? [y/N] "); err != nil {
		return err
	}
	for _, process := range conflict.Processes {
		if err := d.requestStop(ctx, process, true); err != nil {
			return err
		}
	}
	if err := d.wait(ctx, d.forcePeriod); err != nil {
		return fmt.Errorf("files are still in use; rebuild cancelled: %w", err)
	}
	return nil
}

func workerHasApplication(worker holons.ActiveArtifactProcess, apps []holons.ActiveArtifactProcess) bool {
	for _, app := range apps {
		rel, err := filepath.Rel(app.BundlePath, worker.Path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

func (d artifactDialog) showProcesses(processes []holons.ActiveArtifactProcess) {
	for _, process := range processes {
		kind := "worker/process"
		if process.BundlePath != "" {
			kind = "application"
		}
		fmt.Fprintf(d.output, "  %s — PID %d: %s\n", kind, process.PID, process.Path)
	}
}

func (d artifactDialog) requestStop(ctx context.Context, process holons.ActiveArtifactProcess, force bool) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
	}
	// Bound the native quit request itself as well as the later exit wait.
	requestCtx, cancel := context.WithTimeout(ctx, artifactGracePeriod)
	defer cancel()
	if err := d.stop(requestCtx, process, force); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
		}
		if errors.Is(err, holons.ErrActiveArtifactProcessChanged) || force {
			return fmt.Errorf("%w: PID %d: %v", errArtifactRunCancelled, process.PID, err)
		}
		// A normal quit can be declined by the application (for instance an
		// unsaved document). Still wait, then require separate force consent.
		fmt.Fprintf(d.output, "Shutdown not confirmed for PID %d: %v\n", process.PID, err)
	}
	return nil
}

// Poll the complete inventory, not just the PIDs that received a request: a
// worker may have outlived or been spawned by its application during shutdown.
func (d artifactDialog) wait(ctx context.Context, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	period := d.pollPeriod
	if period <= 0 {
		period = artifactPollPeriod
	}
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
		}
		err := d.check()
		var conflict *holons.ActiveArtifactConflict
		if err == nil || !errors.As(err, &conflict) {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
		case <-deadline.C:
			return err
		case <-ticker.C:
		}
	}
}

func (d artifactDialog) confirm(ctx context.Context, prompt string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
	}
	fmt.Fprint(d.output, prompt)
	type result struct {
		line string
		err  error
	}
	answer := make(chan result, 1)
	go func() {
		// Do not buffer ahead: the next prompt or the launched application
		// owns the bytes after this newline. EOF never counts as consent.
		var line strings.Builder
		var b [1]byte
		for line.Len() < 4096 {
			n, err := d.input.Read(b[:])
			if err != nil {
				answer <- result{err: err}
				return
			}
			if n == 0 {
				continue
			}
			if b[0] == '\n' {
				answer <- result{line: line.String()}
				return
			}
			line.WriteByte(b[0])
		}
		answer <- result{err: errors.New("response too long")}
	}()
	select {
	case <-ctx.Done():
		fmt.Fprintln(d.output)
		return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
	case reply := <-answer:
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", errArtifactRunCancelled, ctx.Err())
		}
		if reply.err != nil {
			return fmt.Errorf("%w: reading response: %v", errArtifactRunCancelled, reply.err)
		}
		if text := strings.ToLower(strings.TrimSpace(reply.line)); text != "y" && text != "yes" {
			return errArtifactRunCancelled
		}
		return nil
	}
}

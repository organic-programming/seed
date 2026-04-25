package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/observability"
)

const observeMetaTimeout = 15 * time.Second

func runForegroundObserved(cmd *exec.Cmd, slug, uid, runRoot string, jsonOut bool) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	tailErrCh := make(chan error, 1)
	go func() {
		tailErrCh <- streamObservedRun(ctx, slug, uid, runRoot, jsonOut)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, observeForwardSignals()...)
	defer signal.Stop(sigCh)

	tailDone := false
	var forceKill <-chan time.Time
	shutdownRequested := false
	for {
		select {
		case err := <-waitCh:
			if shutdownRequested {
				killObservedRunProcesses(uid, runRoot)
			}
			if !tailDone {
				select {
				case <-tailErrCh:
				case <-time.After(2 * time.Second):
					cancel()
				}
			}
			cancel()
			return err
		case err := <-tailErrCh:
			tailDone = true
			if err == nil || errors.Is(err, context.Canceled) {
				continue
			}
			cancel()
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			select {
			case <-waitCh:
			case <-time.After(3 * time.Second):
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-waitCh
			}
			return err
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
			signalObservedInstanceProcess(slug, uid, runRoot, sig)
			shutdownRequested = true
			if forceKill == nil {
				forceKill = time.After(5 * time.Second)
			}
		case <-forceKill:
			forceKill = nil
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			killObservedRunProcesses(uid, runRoot)
		}
	}
}

func observeForwardSignals() []os.Signal {
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	return signals
}

func streamObservedRun(ctx context.Context, slug, uid, runRoot string, jsonOut bool) error {
	inst, err := waitForObservedInstance(ctx, slug, uid, runRoot)
	if err != nil {
		return err
	}
	conn, err := dialInstance(ctx, inst)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := v1.NewHolonObservabilityClient(conn)
	streamCtx, cancelStreams := context.WithCancel(ctx)
	defer cancelStreams()

	logs, err := client.Logs(streamCtx, &v1.LogsRequest{Follow: true, MinLevel: v1.LogLevel_TRACE})
	if err != nil {
		return fmt.Errorf("op run --observe logs: %w", err)
	}
	events, err := client.Events(streamCtx, &v1.EventsRequest{Follow: true})
	if err != nil {
		return fmt.Errorf("op run --observe events: %w", err)
	}

	var printMu sync.Mutex
	errCh := make(chan error, 2)
	go func() {
		errCh <- streamObservedLogs(streamCtx, logs, jsonOut, &printMu)
	}()
	go func() {
		errCh <- streamObservedEvents(streamCtx, events, jsonOut, &printMu, cancelStreams)
	}()

	remaining := 2
	for remaining > 0 {
		select {
		case <-ctx.Done():
			cancelStreams()
			return ctx.Err()
		case err := <-errCh:
			remaining--
			if err != nil && ctx.Err() == nil {
				return err
			}
		}
	}
	return nil
}

func streamObservedLogs(ctx context.Context, stream v1.HolonObservability_LogsClient, jsonOut bool, printMu *sync.Mutex) error {
	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		printMu.Lock()
		renderLogEntry(entry, jsonOut)
		printMu.Unlock()
	}
}

func streamObservedEvents(ctx context.Context, stream v1.HolonObservability_EventsClient, jsonOut bool, printMu *sync.Mutex, cancelStreams context.CancelFunc) error {
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		printMu.Lock()
		renderEvent(event, jsonOut)
		printMu.Unlock()
		if event.Type == v1.EventType_INSTANCE_EXITED || event.Type == v1.EventType_INSTANCE_CRASHED {
			cancelStreams()
			return nil
		}
	}
}

func waitForObservedInstance(ctx context.Context, slug, uid, runRoot string) (instanceRow, error) {
	if uid == "" || runRoot == "" {
		return instanceRow{}, fmt.Errorf("op run --observe: missing instance uid or run root")
	}
	ctx, cancel := context.WithTimeout(ctx, observeMetaTimeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		inst, ok := readObservedInstance(slug, uid, runRoot)
		if ok && inst.meta.Address != "" {
			return inst, nil
		}
		select {
		case <-ctx.Done():
			return instanceRow{}, fmt.Errorf("op run --observe: timed out waiting for meta.json for uid %s", uid)
		case <-ticker.C:
		}
	}
}

func readObservedInstance(slug, uid, runRoot string) (instanceRow, bool) {
	if slug != "" {
		if runDir, err := observability.InstanceRunDir(runRoot, slug, uid); err == nil {
			if meta, err := observability.ReadMetaJSON(runDir); err == nil {
				return instanceRow{meta: meta, runDir: runDir, alive: isPidAlive(meta.PID)}, true
			}
		}
		return instanceRow{}, false
	}
	matches, _ := filepath.Glob(filepath.Join(runRoot, "*", uid, "meta.json"))
	for _, match := range matches {
		runDir := filepath.Dir(match)
		meta, err := observability.ReadMetaJSON(runDir)
		if err == nil {
			return instanceRow{meta: meta, runDir: runDir, alive: isPidAlive(meta.PID)}, true
		}
	}
	return instanceRow{}, false
}

func signalObservedInstanceProcess(slug, uid, runRoot string, sig os.Signal) {
	inst, ok := readObservedInstance(slug, uid, runRoot)
	if !ok {
		return
	}
	signalPID(inst.meta.PID, sig)
}

func killObservedRunProcesses(uid, runRoot string) {
	if uid == "" || runRoot == "" {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(runRoot, "*", uid, "meta.json"))
	for _, match := range matches {
		runDir := filepath.Dir(match)
		meta, err := observability.ReadMetaJSON(runDir)
		if err != nil {
			continue
		}
		signalPID(meta.PID, os.Kill)
	}
}

func signalPID(pid int, sig os.Signal) {
	if pid <= 0 || sig == nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(sig)
}

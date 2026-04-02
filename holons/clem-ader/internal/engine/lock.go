package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const catalogueLockName = "ader.lock"

func acquireCatalogueLock(ctx context.Context, artifactsDir string) (func(), error) {
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(artifactsDir, catalogueLockName)
	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(file, "pid=%d\nstarted_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			_ = file.Close()
			return func() {
				_ = os.Remove(lockPath)
			}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if staleCatalogueLock(lockPath) {
			_ = os.Remove(lockPath)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func staleCatalogueLock(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "pid=") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "pid=")))
		if err != nil {
			return false
		}
		return !processRunning(pid)
	}
	return false
}

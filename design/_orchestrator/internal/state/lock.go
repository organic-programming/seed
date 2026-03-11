package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const lockFileName = ".codex_orchestrator.lock"

type Lock struct {
	mu       sync.Mutex
	path     string
	released bool
}

func Acquire(root string) (*Lock, error) {
	root = normalizePath(root)
	if root == "" {
		return nil, fmt.Errorf("root cannot be empty")
	}

	lockPath := filepath.Join(root, lockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}

	for {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
				_ = file.Close()
				_ = os.Remove(lockPath)
				return nil, err
			}
			if err := file.Close(); err != nil {
				_ = os.Remove(lockPath)
				return nil, err
			}
			return &Lock{path: lockPath}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}

		pid, readErr := readLockPID(lockPath)
		if readErr != nil || !processAlive(pid) {
			_ = os.Remove(lockPath)
			continue
		}

		return nil, fmt.Errorf("another orchestrator is running (PID %d)", pid)
	}
}

func (l *Lock) Release() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.released || l.path == "" {
		return nil
	}

	l.released = true
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if pid == os.Getpid() {
		return true
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

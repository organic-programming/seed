//go:build windows

package processowner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestOwnerHelper(t *testing.T) {
	role := os.Getenv("HOLONS_OWNER_TEST_ROLE")
	if role == "" {
		return
	}
	root := os.Getenv("HOLONS_OWNER_TEST_ROOT")
	if role != "leaf" {
		if role != "unowned" {
			if err := Ensure(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if err := Ensure(); err != nil { // Idempotence must not create another job.
				os.Exit(3)
			}
		}
		next := "leaf"
		if role == "owner" {
			next = "branch"
		}
		child := ownerHelperCommand(context.Background(), next, root)
		if err := child.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(4)
		}
	}
	if err := os.WriteFile(filepath.Join(root, role+".pid"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		os.Exit(5)
	}
	for {
		if role == "owner" || role == "unowned" {
			if _, err := os.Stat(filepath.Join(root, "exit")); err == nil {
				os.Exit(0)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestChildJobOwnsUncooperativeGrandchild(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprintf("forced=%v", forced), func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := ownerHelperCommand(ctx, "unowned", root)
			if err := Start(cmd); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = Kill(cmd); _ = Wait(cmd) }()
			var pid int
			for ctx.Err() == nil {
				if data, err := os.ReadFile(filepath.Join(root, "leaf.pid")); err == nil {
					pid, _ = strconv.Atoi(string(data))
					if pid != 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				t.Fatal("unowned grandchild did not start")
			}
			handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(handle)
			if !Running(cmd) {
				t.Fatal("child job reports no active processes")
			}
			if forced {
				if err := Kill(cmd); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(root, "exit"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			_ = Wait(cmd)
			state, err := windows.WaitForSingleObject(handle, 5000)
			if err != nil || state != windows.WAIT_OBJECT_0 {
				t.Fatalf("grandchild survived: state=%d error=%v", state, err)
			}
			if Running(cmd) {
				t.Fatal("reaped child job remains registered")
			}
		})
	}
}

func ownerHelperCommand(ctx context.Context, role, root string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOwnerHelper$")
	cmd.Env = append(os.Environ(), "HOLONS_OWNER_TEST_ROLE="+role, "HOLONS_OWNER_TEST_ROOT="+root)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func TestOwnerExitTerminatesNestedDescendants(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprintf("forced=%v", forced), func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := ownerHelperCommand(ctx, "owner", root)
			var diagnostic strings.Builder
			cmd.Stderr = &diagnostic
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			var descendants []windows.Handle
			for _, role := range []string{"branch", "leaf"} {
				path := filepath.Join(root, role+".pid")
				var pid int
				for ctx.Err() == nil {
					data, err := os.ReadFile(path)
					if err == nil {
						pid, _ = strconv.Atoi(string(data))
						if pid != 0 {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
				}
				if pid == 0 {
					t.Fatalf("%s did not start", role)
				}
				handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
				if err != nil {
					t.Fatal(err)
				}
				defer windows.CloseHandle(handle)
				descendants = append(descendants, handle)
			}
			if forced {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(root, "exit"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, handle := range descendants {
				state, err := windows.WaitForSingleObject(handle, 5000)
				if err != nil || state != windows.WAIT_OBJECT_0 {
					t.Fatalf("descendant survived owner exit: state=%d error=%v", state, err)
				}
			}
		})
	}
}

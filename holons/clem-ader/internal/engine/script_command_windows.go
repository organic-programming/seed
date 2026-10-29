//go:build windows

package engine

import (
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildScriptCommand(step StepSpec, _ os.FileInfo) (*exec.Cmd, error) {
	switch strings.ToLower(filepath.Ext(step.Script)) {
	case ".ps1":
		pwsh, err := exec.LookPath("pwsh")
		if err != nil {
			return nil, fmt.Errorf("PowerShell 7 is required for %s: %w", step.Script, err)
		}
		args := append([]string{"-NoLogo", "-NoProfile", "-NonInteractive", "-File", step.Script}, step.Args...)
		return exec.Command(pwsh, args...), nil
	case ".exe":
		image, err := pe.Open(step.Script)
		if err != nil {
			return nil, fmt.Errorf("script %s is not a PE executable: %w", step.Script, err)
		}
		defer image.Close()
		if image.OptionalHeader == nil || image.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 || image.Characteristics&pe.IMAGE_FILE_DLL != 0 {
			return nil, fmt.Errorf("script %s is not an executable PE image", step.Script)
		}
		// CreateProcess enforces the file's Windows ACLs and application-control
		// policy at launch; POSIX mode bits cannot represent execute permission.
		return exec.Command(step.Script, step.Args...), nil
	default:
		return nil, fmt.Errorf("script %s must be a PowerShell script or native executable on Windows", step.Script)
	}
}

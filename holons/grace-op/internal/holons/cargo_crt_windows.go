package holons

import (
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var windowsDebugCRT = regexp.MustCompile(`(?i)^(?:ucrtbased|(?:msvcp|vcruntime|concrt|vccorlib)140(?:_\d+)?d(?:_.*)?)\.dll$`)
var windowsRedistVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var windowsRedistFolder = regexp.MustCompile(`^Microsoft\.VC\d+\.CRT$`)

// MSVC-linked Cargo artifacts need their redistributable CRT beside the entry
// point. A developer machine's System32 or PATH must not supply package files.
func bundleCargoWindowsCRT(manifest *LoadedManifest, report *Report) error {
	output, err := exec.Command("rustc", "-vV").CombinedOutput()
	if err != nil {
		return fmt.Errorf("identify Cargo CRT toolchain: %w\n%s", err, output)
	}
	host := ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "host: ") {
			host = strings.TrimSpace(strings.TrimPrefix(line, "host: "))
		}
	}
	if host == "" {
		return fmt.Errorf("rustc did not identify its host for Windows CRT packaging")
	}
	if !strings.HasSuffix(host, "-windows-msvc") {
		return nil // GNU uses a different runtime, outside the MSVC CRT package.
	}
	redist, err := windowsCRTRedistDirectory(os.Getenv, findWindowsVisualStudio, runtime.GOARCH)
	if err != nil {
		return err
	}
	if err := copyWindowsCRT(redist, filepath.Dir(manifest.BinaryPath()), runtime.GOARCH); err != nil {
		return fmt.Errorf("bundle Cargo Windows CRT: %w", err)
	}
	report.Notes = append(report.Notes, "bundled app-local Visual C++ redistributable CRT from "+redist)
	return nil
}

func findWindowsVisualStudio() (string, error) {
	installer := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	output, err := exec.Command(installer, "-latest", "-products", "*", "-requires",
		"Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-property", "installationPath").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("locate Visual Studio redistributable: %w\n%s", err, output)
	}
	instance := strings.TrimSpace(string(output))
	if instance == "" {
		return "", fmt.Errorf("no Visual Studio C++ installation; set OP_WINDOWS_CRT_DIR to its redistributable CRT directory")
	}
	return instance, nil
}

func windowsCRTArchitecture(goarch string) (string, uint16, error) {
	switch goarch {
	case "amd64":
		return "x64", pe.IMAGE_FILE_MACHINE_AMD64, nil
	case "arm64":
		return "arm64", pe.IMAGE_FILE_MACHINE_ARM64, nil
	case "386":
		return "x86", pe.IMAGE_FILE_MACHINE_I386, nil
	default:
		return "", 0, fmt.Errorf("unsupported Windows CRT architecture %q", goarch)
	}
}

func windowsCRTRedistDirectory(getenv func(string) string, findVS func() (string, error), goarch string) (string, error) {
	architecture, _, err := windowsCRTArchitecture(goarch)
	if err != nil {
		return "", err
	}
	if override := strings.TrimSpace(getenv("OP_WINDOWS_CRT_DIR")); override != "" {
		return filepath.Abs(override)
	}
	instance := strings.TrimSpace(getenv("VSINSTALLDIR"))
	if instance == "" {
		if vc := strings.TrimSpace(getenv("VCINSTALLDIR")); vc != "" {
			instance = filepath.Dir(filepath.Clean(vc))
		}
	}
	if instance == "" {
		instance, err = findVS()
		if err != nil {
			return "", err
		}
	}
	version, err := os.ReadFile(filepath.Join(instance, "VC", "Auxiliary", "Build", "Microsoft.VCRedistVersion.default.txt"))
	if err != nil {
		return "", fmt.Errorf("read Visual Studio CRT version: %w", err)
	}
	trimmed := strings.TrimSpace(string(version))
	if !windowsRedistVersion.MatchString(trimmed) {
		return "", fmt.Errorf("invalid Visual Studio CRT version %q", trimmed)
	}
	architectureRoot := filepath.Join(instance, "VC", "Redist", "MSVC", trimmed, architecture)
	entries, err := os.ReadDir(architectureRoot)
	if err != nil {
		return "", fmt.Errorf("find Visual Studio CRT directory: %w", err)
	}
	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() && windowsRedistFolder.MatchString(entry.Name()) {
			candidates = append(candidates, filepath.Join(architectureRoot, entry.Name()))
		}
	}
	if len(candidates) != 1 {
		return "", fmt.Errorf("expected one redistributable CRT in %s, found %d; set OP_WINDOWS_CRT_DIR explicitly", architectureRoot, len(candidates))
	}
	return candidates[0], nil
}

func copyWindowsCRT(redist, destination, goarch string) error {
	_, machine, err := windowsCRTArchitecture(goarch)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(redist)
	if err != nil {
		return err
	}
	files := make(map[string]string)
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".dll") {
			continue
		}
		if !entry.Type().IsRegular() || windowsDebugCRT.MatchString(entry.Name()) {
			return fmt.Errorf("invalid or non-redistributable CRT file: %s", filepath.Join(redist, entry.Name()))
		}
		path := filepath.Join(redist, entry.Name())
		module, err := pe.Open(path)
		if err != nil {
			return fmt.Errorf("inspect CRT %s: %w", path, err)
		}
		actual := module.Machine
		closeErr := module.Close()
		if actual != machine {
			return fmt.Errorf("CRT %s has machine %#x, want %#x", path, actual, machine)
		}
		if closeErr != nil {
			return closeErr
		}
		files[strings.ToLower(entry.Name())] = path
	}
	required := []string{"msvcp140.dll", "vcruntime140.dll"}
	if goarch == "amd64" {
		required = append(required, "vcruntime140_1.dll")
	}
	for _, name := range required {
		if files[name] == "" {
			return fmt.Errorf("missing redistributable CRT %s in %s", name, redist)
		}
	}
	// Validate the whole source set before publishing any DLL.
	for name, source := range files {
		target := filepath.Join(destination, name)
		if strings.EqualFold(filepath.Clean(source), filepath.Clean(target)) {
			continue
		}
		if err := copyFile(source, target); err != nil {
			return err
		}
	}
	return nil
}

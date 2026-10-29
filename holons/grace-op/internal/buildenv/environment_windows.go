package buildenv

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"

	openv "github.com/organic-programming/grace-op/internal/env"
	"gopkg.in/yaml.v3"
)

// Project values live in seed-toolchain.yaml, never in OP. The nearest ancestor
// declaring a Windows environment wins; a seed file with SDK pins alone does
// not hide an enclosing application's build environment.
type configuration struct {
	Protoc struct {
		Version string `yaml:"version"`
	} `yaml:"protoc"`
	BuildEnvironments struct {
		Windows *windowsEnvironment `yaml:"windows"`
	} `yaml:"build_environments"`
}
type windowsEnvironment struct {
	VisualStudio  *visualStudio     `yaml:"visual_studio"`
	Path          []string          `yaml:"path"`
	Env           map[string]string `yaml:"env"`
	RequiredFiles []string          `yaml:"required_files"`
}
type visualStudio struct {
	Version          string `yaml:"version"`
	MSVC             string `yaml:"msvc"`
	Architecture     string `yaml:"architecture"`
	HostArchitecture string `yaml:"host_architecture"`
}
type vsInstance struct {
	InstallationPath    string `json:"installationPath"`
	InstallationVersion string `json:"installationVersion"`
	IsComplete          bool   `json:"isComplete"`
}

var developerEnvironmentCache sync.Map

// Prepare returns a private environment for build children. It never modifies
// os.Environ, the caller's shell, registry or installed tools.
func Prepare(dir string) ([]string, error) {
	file, config, err := findConfiguration(dir)
	if err != nil || config == nil {
		return nil, err
	}
	base := Merge(os.Environ(), []string{"OPPATH=" + openv.OPPATH(), "OPBIN=" + openv.OPBIN(), "CONFIG_DIR=" + filepath.Dir(file)})
	version := config.Protoc.Version
	if version == "" {
		data, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "seed", "seed-toolchain.yaml"))
		if readErr == nil {
			var seed configuration
			if err := yaml.Unmarshal(data, &seed); err != nil {
				return nil, fmt.Errorf("read SDK toolchain: %w", err)
			}
			version = seed.Protoc.Version
		} else if !os.IsNotExist(readErr) {
			return nil, readErr
		}
	}
	if version != "" {
		base = Merge(base, []string{"PROTOC_VERSION=" + version})
	}
	settings := config.BuildEnvironments.Windows
	if settings.VisualStudio != nil {
		base, err = prepareVisualStudio(base, *settings.VisualStudio)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	// Resolve references independently of YAML map iteration order. Cycles and
	// misspelled variables fail instead of silently turning into empty paths.
	keys := make([]string, 0, len(settings.Env))
	for key := range settings.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	resolving := map[string]bool{}
	resolved := map[string]bool{}
	var resolve func(string) (string, error)
	resolve = func(key string) (string, error) {
		if key == "" || strings.ContainsAny(key, "=\x00") {
			return "", fmt.Errorf("invalid environment name %q", key)
		}
		if resolved[key] {
			return Value(base, key), nil
		}
		raw, declared := settings.Env[key]
		if !declared {
			value := Value(base, key)
			if value == "" {
				return "", fmt.Errorf("undefined build environment variable %s", key)
			}
			return value, nil
		}
		if strings.EqualFold(key, "PATH") {
			return "", fmt.Errorf("declare PATH entries with path, not env.PATH")
		}
		if resolving[key] {
			return "", fmt.Errorf("cyclic build environment variable %s", key)
		}
		resolving[key] = true
		value, err := expand(raw, resolve)
		if err != nil {
			return "", err
		}
		base = Merge(base, []string{key + "=" + value})
		resolved[key], resolving[key] = true, false
		return value, nil
	}
	for _, key := range keys {
		if _, err := resolve(key); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	}
	var paths []string
	for _, entry := range settings.Path {
		path, err := expand(entry, resolve)
		if err != nil {
			return nil, fmt.Errorf("%s: path: %w", file, err)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(file), path)
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("%s: build PATH directory missing: %s", file, path)
		}
		paths = append(paths, path)
	}
	paths = append(paths, Value(base, "PATH"))
	base = Merge(base, []string{"PATH=" + strings.Join(paths, string(os.PathListSeparator))})
	for _, required := range settings.RequiredFiles {
		path, err := expand(required, resolve)
		if err != nil {
			return nil, fmt.Errorf("%s: required_files: %w", file, err)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(file), path)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: required build tool file missing: %s", file, path)
		}
	}
	return base, nil
}

func findConfiguration(dir string) (string, *configuration, error) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}
	for {
		file := filepath.Join(current, "seed-toolchain.yaml")
		data, err := os.ReadFile(file)
		if err == nil {
			var config configuration
			if err := yaml.Unmarshal(data, &config); err != nil {
				return "", nil, fmt.Errorf("parse %s: %w", file, err)
			}
			if config.BuildEnvironments.Windows != nil {
				return file, &config, nil
			}
		} else if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil, nil
		}
		current = parent
	}
}

func expand(raw string, lookup func(string) (string, error)) (string, error) {
	var firstErr error
	value := os.Expand(raw, func(key string) string {
		value, err := lookup(key)
		if firstErr == nil {
			firstErr = err
		}
		return value
	})
	if strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("NUL in environment value")
	}
	return value, firstErr
}

func prepareVisualStudio(base []string, settings visualStudio) ([]string, error) {
	if settings.Architecture == "" {
		settings.Architecture = "x64"
	}
	if settings.HostArchitecture == "" {
		settings.HostArchitecture = "x64"
	}
	for _, arch := range []string{settings.Architecture, settings.HostArchitecture} {
		if arch != "x64" && arch != "x86" && arch != "arm64" {
			return nil, fmt.Errorf("unsupported Visual Studio architecture %q", arch)
		}
	}
	where := filepath.Join(Value(base, "ProgramFiles(x86)"), "Microsoft Visual Studio", "Installer", "vswhere.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, where, "-products", "*", "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "-format", "json", "-utf8")
	cmd.Env = base
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("discover Visual Studio C++ with vswhere: %w", err)
	}
	var instances []vsInstance
	if err := json.Unmarshal(output, &instances); err != nil {
		return nil, fmt.Errorf("decode vswhere: %w", err)
	}
	var candidates []vsInstance
	for _, instance := range instances {
		if instance.IsComplete && (settings.Version == "" || instance.InstallationVersion == settings.Version) {
			candidates = append(candidates, instance)
		}
	}
	if len(candidates) != 1 {
		return nil, fmt.Errorf("expected one complete Visual Studio C++ installation for version %q, found %d; declare visual_studio.version", settings.Version, len(candidates))
	}
	instance := candidates[0]
	versionFile := filepath.Join(instance.InstallationPath, "VC", "Auxiliary", "Build", "Microsoft.VCToolsVersion.default.txt")
	toolset, err := os.ReadFile(versionFile)
	if err != nil {
		return nil, err
	}
	if settings.MSVC != "" && strings.TrimSpace(string(toolset)) != settings.MSVC {
		return nil, fmt.Errorf("Visual Studio MSVC %s, expected %s", strings.TrimSpace(string(toolset)), settings.MSVC)
	}
	cacheKey := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(base, "\x00")+fmt.Sprint(settings)+instance.InstallationPath+string(toolset))))
	if cached, ok := developerEnvironmentCache.Load(cacheKey); ok {
		return append([]string{}, cached.([]string)...), nil
	}
	devcmd := filepath.Join(instance.InstallationPath, "Common7", "Tools", "VsDevCmd.bat")
	// Only Microsoft's installed provider is executable configuration. Project
	// YAML cannot inject a shell command; architecture values are enumerated.
	if strings.ContainsAny(devcmd, "\"%\r\n") {
		return nil, fmt.Errorf("unsupported Visual Studio installation path %q", devcmd)
	}
	command := `call "` + devcmd + `" -no_logo -arch=` + settings.Architecture + ` -host_arch=` + settings.HostArchitecture + ` >nul && set`
	cmd = exec.CommandContext(ctx, filepath.Join(Value(base, "SystemRoot"), "System32", "cmd.exe"))
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /u /s /c "` + command + `"`, HideWindow: true}
	cmd.Env = base
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("initialize Visual Studio C++ environment: %w", err)
	}
	var imported []string
	if len(output)%2 != 0 {
		return nil, fmt.Errorf("invalid developer environment encoding")
	}
	units := make([]uint16, len(output)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(output[2*i:])
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(utf16.Decode(units)), "\r\n", "\n"), "\n") {
		if at := strings.IndexByte(line, '='); at > 0 {
			imported = append(imported, line)
		}
	}
	result := Merge(base, imported)
	if settings.MSVC != "" && strings.TrimRight(Value(result, "VCToolsVersion"), `/\`) != settings.MSVC {
		return nil, fmt.Errorf("developer environment did not select MSVC %s", settings.MSVC)
	}
	compiler, err := Lookup(result, "cl.exe")
	if err != nil || !strings.HasPrefix(strings.ToLower(compiler), strings.ToLower(instance.InstallationPath+string(os.PathSeparator))) {
		return nil, fmt.Errorf("developer environment compiler is outside selected Visual Studio")
	}
	developerEnvironmentCache.Store(cacheKey, append([]string{}, result...))
	return result, nil
}

func Merge(base, extra []string) []string {
	values := map[string]string{}
	for _, list := range [][]string{base, extra} {
		for _, item := range list {
			if at := strings.IndexByte(item, '='); at > 0 {
				values[strings.ToUpper(item[:at])] = item
			}
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, values[key])
	}
	return out
}

func Value(environment []string, name string) string {
	if environment == nil {
		return os.Getenv(name)
	}
	for i := len(environment) - 1; i >= 0; i-- {
		if at := strings.IndexByte(environment[i], '='); at > 0 && strings.EqualFold(environment[i][:at], name) {
			return environment[i][at+1:]
		}
	}
	return ""
}

// Lookup uses the child's PATH and PATHEXT, including explicit empty values.
// Go's exec.LookPath instead uses the parent process environment.
func Lookup(environment []string, name string) (string, error) {
	if environment == nil {
		return exec.LookPath(name)
	}
	extensions := strings.Split(Value(environment, "PATHEXT"), ";")
	if len(extensions) == 1 && extensions[0] == "" {
		extensions = []string{".COM", ".EXE", ".BAT", ".CMD"}
	}
	if filepath.Ext(name) != "" {
		extensions = append([]string{""}, extensions...)
	}
	paths := filepath.SplitList(Value(environment, "PATH"))
	if strings.ContainsAny(name, `/\:`) {
		paths = []string{""}
	}
	for _, dir := range paths {
		if dir == "" && !strings.ContainsAny(name, `/\:`) {
			continue
		}
		candidate := name
		if dir != "" {
			candidate = filepath.Join(dir, name)
		}
		for _, extension := range extensions {
			path := candidate + extension
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return filepath.Abs(path)
			}
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func Command(dir string, args, extra []string) (*exec.Cmd, error) {
	environment, err := Prepare(dir)
	if err != nil {
		return nil, err
	}
	if environment == nil && (len(extra) == 0 || Value(extra, "CONFIG_DIR") == "") {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if len(extra) != 0 {
			cmd.Env = append(os.Environ(), extra...)
		}
		return cmd, nil
	}
	if environment == nil {
		environment = os.Environ()
	}
	environment = Merge(environment, extra)
	// exec.Cmd.Dir defines the child's working directory. Resolve both explicit
	// relative executables and inherited relative PATH entries there, before
	// Lookup performs filesystem checks in the OP parent process.
	childDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	paths := filepath.SplitList(Value(environment, "PATH"))
	for i, entry := range paths {
		if entry == "" {
			continue
		}
		paths[i], err = relativeToChild(childDir, entry)
		if err != nil {
			return nil, err
		}
	}
	environment = Merge(environment, []string{"PATH=" + strings.Join(paths, string(os.PathListSeparator))})
	program := args[0]
	if strings.ContainsAny(program, `/\:`) {
		program, err = relativeToChild(childDir, program)
		if err != nil {
			return nil, err
		}
	}
	path, err := Lookup(environment, program)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args[1:]...)
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".bat" || ext == ".cmd" {
		var quoted []string
		for _, arg := range append([]string{path}, args[1:]...) {
			if strings.ContainsAny(arg, "\"%\r\n") {
				return nil, fmt.Errorf("unsupported batch argument %q", arg)
			}
			quoted = append(quoted, `"`+arg+`"`)
		}
		cmd = exec.Command(filepath.Join(Value(environment, "SystemRoot"), "System32", "cmd.exe"))
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + strings.Join(quoted, " ") + `"`, HideWindow: true}
	}
	cmd.Dir, cmd.Env = dir, environment
	return cmd, nil
}

func relativeToChild(childDir, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	volume := filepath.VolumeName(path)
	if volume != "" {
		// Drive-relative paths refer to the child's directory on that drive.
		// Another drive has no deterministic current directory in this scope.
		if !strings.EqualFold(volume, filepath.VolumeName(childDir)) {
			return "", fmt.Errorf("drive-relative build path %q is outside child working drive", path)
		}
		path = strings.TrimPrefix(path, volume)
	} else if len(path) != 0 && os.IsPathSeparator(path[0]) {
		return filepath.Clean(filepath.VolumeName(childDir) + path), nil
	}
	return filepath.Join(childDir, path), nil
}

package integration

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkgrpc "github.com/organic-programming/go-holons/pkg/grpcclient"
	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

type isolatedOPCache struct {
	once   sync.Once
	binary string
	err    error
}

var isolatedOPBinaries sync.Map

// SetupIsolatedOP creates an isolated OPPATH and builds the 'op' orchestrator.
// It returns the environment variables to inject and the path to the installed 'op' binary.
func SetupIsolatedOP(t *testing.T, rootDir string) ([]string, string) {
	t.Helper()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		t.Fatalf("resolve root %s: %v", rootDir, err)
	}
	opPath := os.Getenv("ADER_RUN_ARTIFACTS")
	if opPath == "" {
		opPath = t.TempDir()
	}
	opBin := filepath.Join(opPath, "bin")
	if err := os.MkdirAll(opBin, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", opBin, err)
	}
	envVars := isolatedOPEnv(t, opPath, opBin)

	cachedBinary := bootstrapIsolatedOPBinary(t, absRoot)
	gen2Bin := filepath.Join(opBin, "op")
	info, err := os.Stat(cachedBinary)
	if err != nil {
		t.Fatalf("stat cached op binary: %v", err)
	}
	if err := copyFile(cachedBinary, gen2Bin, info.Mode()); err != nil {
		t.Fatalf("copy cached op binary: %v", err)
	}

	if stat, err := os.Stat(gen2Bin); os.IsNotExist(err) || stat.Size() == 0 {
		t.Fatalf("Bootstrap did not produce the expected binary %s", gen2Bin)
	}

	return envVars, gen2Bin
}

func bootstrapIsolatedOPBinary(t *testing.T, absRoot string) string {
	t.Helper()

	cacheKey := absRoot
	entryAny, _ := isolatedOPBinaries.LoadOrStore(cacheKey, &isolatedOPCache{})
	entry := entryAny.(*isolatedOPCache)
	entry.once.Do(func() {
		rt := mustRuntime(t)
		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(absRoot))
		bootstrapRoot := filepath.Join(rt.bootstrapRoot, fmt.Sprintf("%x-%d", hasher.Sum64(), os.Getpid()))
		bootstrapOPPath := filepath.Join(bootstrapRoot, ".op")
		bootstrapOPBin := filepath.Join(bootstrapOPPath, "bin")
		if err := os.MkdirAll(bootstrapOPBin, 0o755); err != nil {
			entry.err = err
			return
		}

		gen1Bin := filepath.Join(bootstrapRoot, "op-gen1")
		cmdGen1 := exec.Command("go", "build", "-o", gen1Bin, filepath.Join(absRoot, "holons", "grace-op", "cmd", "op"))
		cmdGen1.Dir = absRoot
		cmdGen1.Env = withEnvValue(os.Environ(), "GOCACHE", filepath.Join(rt.toolCacheRoot, "go-build"))
		cmdGen1.Env = withEnvValue(cmdGen1.Env, "GOMODCACHE", filepath.Join(rt.toolCacheRoot, "go-mod"))
		if out, err := cmdGen1.CombinedOutput(); err != nil {
			entry.err = fmt.Errorf("native bootstrap build: %w\n%s", err, string(out))
			return
		}

		cmdGen2 := exec.Command(gen1Bin, "build", "op", "--install", "--symlink", "--root", absRoot)
		cmdGen2.Env = isolatedOPEnv(t, bootstrapOPPath, bootstrapOPBin)
		if out, err := cmdGen2.CombinedOutput(); err != nil {
			entry.err = fmt.Errorf("bootstrap op build: %w\n%s", err, string(out))
			return
		}

		entry.binary = filepath.Join(bootstrapOPBin, "op")
	})
	if entry.err != nil {
		t.Fatalf("bootstrap isolated op: %v", entry.err)
	}
	return entry.binary
}

func isolatedOPEnv(t *testing.T, opPath, opBin string) []string {
	t.Helper()

	rt := mustRuntime(t)
	tmpDir := filepath.Join(opPath, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", tmpDir, err)
	}

	envVars := withEnv(os.Environ(), "OPPATH", opPath)
	envVars = withEnv(envVars, "OPBIN", opBin)
	pathValue := integrationToolPath()
	if javaHome := integrationJavaHome(); javaHome != "" {
		envVars = withEnv(envVars, "JAVA_HOME", javaHome)
		pathValue = prependPathEntry(pathValue, filepath.Join(javaHome, "bin"))
	}
	envVars = withEnv(envVars, "PATH", pathValue)
	envVars = withEnv(envVars, "GOCACHE", filepath.Join(rt.toolCacheRoot, "go-build"))
	envVars = withEnv(envVars, "GOMODCACHE", filepath.Join(rt.toolCacheRoot, "go-mod"))
	envVars = withEnv(envVars, "GRACE_OP_SHARED_CACHE_DIR", rt.sharedCacheRoot)
	envVars = withEnv(envVars, "TMPDIR", tmpDir)
	envVars = withEnv(envVars, "TMP", tmpDir)
	envVars = withEnv(envVars, "TEMP", tmpDir)
	return envVars
}

func integrationToolPath() string {
	base := FilterInstalledHolonsPath(os.Getenv("PATH"))
	extras := userGemBinDirs()
	if len(extras) == 0 {
		return base
	}
	paths := make([]string, 0, len(extras)+1)
	paths = append(paths, extras...)
	if strings.TrimSpace(base) != "" {
		paths = append(paths, base)
	}
	return strings.Join(paths, string(os.PathListSeparator))
}

func integrationJavaHome() string {
	candidates := make([]string, 0, 16)
	add := func(home string) {
		home = strings.TrimSpace(home)
		if home == "" {
			return
		}
		candidates = append(candidates, filepath.Clean(home))
	}

	add(os.Getenv("JAVA_HOME"))
	for _, home := range []string{
		"/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home",
		"/usr/local/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home",
		"/opt/homebrew/opt/openjdk/libexec/openjdk.jdk/Contents/Home",
		"/usr/local/opt/openjdk/libexec/openjdk.jdk/Contents/Home",
		"/usr/lib/jvm/java-21-openjdk",
		"/usr/lib/jvm/java-21-openjdk-amd64",
		"/usr/lib/jvm/temurin-21-jdk-amd64",
		"/usr/lib/jvm/jdk-21",
		"/usr/lib/jvm/default-java",
	} {
		add(home)
	}
	for _, pattern := range []string{
		"/Library/Java/JavaVirtualMachines/*/Contents/Home",
		"/usr/lib/jvm/*",
		filepath.Join(os.Getenv("HOME"), ".gradle", "jdks", "*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, match := range matches {
			add(match)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, home := range candidates {
		if _, ok := seen[home]; ok {
			continue
		}
		seen[home] = struct{}{}
		if major, ok := javaHomeMajor(home); ok && major >= 21 {
			return home
		}
	}
	return ""
}

func javaHomeMajor(home string) (int, bool) {
	info, err := os.Stat(filepath.Join(home, "bin", "java"))
	if err != nil || info.IsDir() {
		return 0, false
	}
	data, err := os.ReadFile(filepath.Join(home, "release"))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "JAVA_VERSION" {
			continue
		}
		major, ok := parseJavaMajor(strings.Trim(value, `"`))
		if ok {
			return major, true
		}
	}
	return 0, false
}

func parseJavaMajor(version string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) == 0 {
		return 0, false
	}
	if parts[0] == "1" && len(parts) > 1 {
		major, err := strconv.Atoi(parts[1])
		return major, err == nil
	}
	major, err := strconv.Atoi(parts[0])
	return major, err == nil
}

func prependPathEntry(pathValue, entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return pathValue
	}
	if strings.TrimSpace(pathValue) == "" {
		return entry
	}
	return entry + string(os.PathListSeparator) + pathValue
}

func userGemBinDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(home, ".gem", "ruby", "*", "bin"))
	if err != nil {
		return nil
	}
	sort.Strings(matches)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil && info.IsDir() {
			out = append(out, match)
		}
	}
	return out
}

// SetupStdioOPClient launches the OP binary in stdio gRPC mode and returns a typed client.
func SetupStdioOPClient(t *testing.T, rootDir, opBin string, envVars []string) (opv1.OPServiceClient, func()) {
	t.Helper()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		t.Fatalf("Failed to resolve root %s: %v", rootDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.Command(opBin, "serve", "--listen", "stdio://")
	cmd.Dir = absRoot
	cmd.Env = withEnv(envVars, "OPROOT", absRoot)

	conn, startedCmd, err := sdkgrpc.DialStdioCommand(ctx, cmd)
	if err != nil {
		cancel()
		t.Fatalf("Failed to start stdio RPC client for %s: %v", opBin, err)
	}

	cleanup := func() {
		_ = conn.Close()
		if startedCmd != nil && startedCmd.Process != nil {
			_ = startedCmd.Process.Kill()
			_ = startedCmd.Wait()
		}
		cancel()
	}

	return opv1.NewOPServiceClient(conn), cleanup
}

// SetupSandboxStdioOPClient launches the canonical op binary from the shared integration
// runtime against the mirrored workspace and returns a typed client bound to the sandbox.
func SetupSandboxStdioOPClient(t *testing.T, sb *Sandbox) (opv1.OPServiceClient, func()) {
	t.Helper()

	rootDir := DefaultWorkspaceDir(t)
	envVars := append([]string{}, sb.commandEnv(t, nil)...)
	return SetupStdioOPClient(t, rootDir, CanonicalOPBinary(t), envVars)
}

// SetupSandboxStdioOPClientAt launches the canonical op binary in the given workdir.
func SetupSandboxStdioOPClientAt(t *testing.T, sb *Sandbox, workDir string) (opv1.OPServiceClient, func()) {
	t.Helper()

	envVars := append([]string{}, sb.commandEnv(t, nil)...)
	return SetupStdioOPClient(t, workDir, CanonicalOPBinary(t), envVars)
}

// WithSandboxEnv runs fn with OPPATH, OPBIN, and OPROOT pointing at the sandbox.
func WithSandboxEnv(t *testing.T, sb *Sandbox, fn func()) {
	t.Helper()

	envVars := sb.commandEnv(t, nil)
	for _, entry := range envVars {
		parts := strings.SplitN(entry, "=", 2)
		key := parts[0]
		value := ""
		if len(parts) == 2 {
			value = parts[1]
		}
		t.Setenv(key, value)
	}
	t.Setenv("OPROOT", DefaultWorkspaceDir(t))

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(DefaultWorkspaceDir(t)); err != nil {
		t.Fatalf("Chdir(%s): %v", DefaultWorkspaceDir(t), err)
	}
	defer func() {
		_ = os.Chdir(original)
	}()

	fn()
}

// TeardownHolons vigorously wipes the .op/build specific cache directories across all examples
// to natively guarantee an absolute zero-state environment for the test framework.
func TeardownHolons(t *testing.T, rootDir string) {
	t.Helper()
	examplesDir := filepath.Join(rootDir, "examples/hello-world")
	entries, err := os.ReadDir(examplesDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				opDir := filepath.Join(examplesDir, entry.Name(), ".op")
				_ = os.RemoveAll(opDir)
			}
		}
	}
}

// FilterInstalledHolonsPath removes previously installed holon bins from PATH.
func FilterInstalledHolonsPath(pathValue string) string {
	if strings.TrimSpace(pathValue) == "" {
		return pathValue
	}

	entries := strings.Split(pathValue, string(os.PathListSeparator))
	filtered := make([]string, 0, len(entries))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		lower := strings.ToLower(cleaned)
		if strings.Contains(lower, strings.ToLower(string(filepath.Separator)+".op"+string(filepath.Separator)+"bin")) {
			continue
		}
		if strings.HasSuffix(lower, strings.ToLower(filepath.Join(".op", "bin"))) {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
}

func withEnv(envVars []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(envVars)+1)
	for _, entry := range envVars {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, prefix+value)
}

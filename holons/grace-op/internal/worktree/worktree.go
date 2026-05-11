package worktree

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const BootstrapSchemaVersion = 1

type Mode string

const (
	ModeIsolated Mode = "isolated"
	ModePlain    Mode = "plain"
)

type Command string

const (
	CommandCreate    Command = "create"
	CommandBootstrap Command = "bootstrap"
	CommandDoctor    Command = "doctor"
)

type Activation struct {
	Cwd         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	PathPrepend string            `json:"path_prepend,omitempty"`
}

type Result struct {
	SchemaVersion           int               `json:"schema_version"`
	Command                 string            `json:"command,omitempty"`
	Mode                    string            `json:"mode,omitempty"`
	Status                  string            `json:"status,omitempty"`
	WorktreeStatus          string            `json:"worktree_status,omitempty"`
	Branch                  string            `json:"branch,omitempty"`
	Head                    string            `json:"head,omitempty"`
	Worktree                string            `json:"worktree,omitempty"`
	Oppath                  string            `json:"oppath,omitempty"`
	Opbin                   string            `json:"opbin,omitempty"`
	Activation              Activation        `json:"activation,omitempty"`
	OpSHA256                string            `json:"op_sha256,omitempty"`
	AderSHA256              string            `json:"ader_sha256,omitempty"`
	OpPath                  string            `json:"op_path,omitempty"`
	AderPath                string            `json:"ader_path,omitempty"`
	ConfigChanges           []string          `json:"config_changes,omitempty"`
	ConfigPaths             map[string]string `json:"config_paths,omitempty"`
	BootstrapJSON           string            `json:"bootstrap_json,omitempty"`
	CodexConfigTOML         string            `json:"codex_config_toml,omitempty"`
	ClaudeSettingsLocalJSON string            `json:"claude_settings_local_json,omitempty"`
	GeminiEnv               string            `json:"gemini_env,omitempty"`
	VSCodeSettingsJSON      string            `json:"vscode_settings_json,omitempty"`
	Isolated                bool              `json:"isolated,omitempty"`
	BuiltAt                 string            `json:"built_at,omitempty"`
	Doctor                  *DoctorResult     `json:"doctor,omitempty"`
}

type DoctorResult struct {
	OK              bool            `json:"ok"`
	Cwd             string          `json:"cwd,omitempty"`
	RepoRoot        string          `json:"repo_root,omitempty"`
	ExpectedOPPATH  string          `json:"expected_oppath,omitempty"`
	ExpectedOPBIN   string          `json:"expected_opbin,omitempty"`
	OPPATH          string          `json:"oppath,omitempty"`
	OPBIN           string          `json:"opbin,omitempty"`
	OpPath          string          `json:"op_path,omitempty"`
	AderPath        string          `json:"ader_path,omitempty"`
	Checks          map[string]bool `json:"checks,omitempty"`
	Error           string          `json:"error,omitempty"`
	RecommendedCode int             `json:"recommended_code,omitempty"`
}

var ManagedEnvKeys = []string{"OPPATH", "OPBIN", "PATH"}

var runBuildCommand = func(opCommand string, worktree string, env []string, target string) error {
	cmd := exec.Command(opCommand, "build", target, "--install")
	cmd.Dir = worktree
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("op build %s --install: %w\n%s", target, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func SetBuildCommandForTest(fn func(opCommand string, worktree string, env []string, target string) error) func() {
	old := runBuildCommand
	runBuildCommand = fn
	return func() {
		runBuildCommand = old
	}
}

func Create(branch string, mode Mode) (*Result, error) {
	switch mode {
	case ModeIsolated:
		result, err := Bootstrap(branch)
		if result != nil {
			result.Command = string(CommandCreate)
		}
		return result, err
	case ModePlain:
		return CreatePlain(branch)
	default:
		return nil, errors.New("create requires isolated or plain mode")
	}
}

func CreatePlain(branch string) (*Result, error) {
	_, worktree, status, err := ensureOPWorktree(branch)
	if err != nil {
		return nil, err
	}
	return &Result{
		SchemaVersion: BootstrapSchemaVersion,
		Command:       string(CommandCreate),
		Mode:          string(ModePlain),
		Status:        status,
		Branch:        branch,
		Worktree:      worktree,
		Isolated:      fileExists(markerPath(worktree)),
	}, nil
}

func Bootstrap(branch string) (*Result, error) {
	_, worktree, worktreeStatus, err := ensureOPWorktree(branch)
	if err != nil {
		return nil, err
	}
	marker, _ := loadMarker(worktree)
	values := envValues(worktree, marker)
	if marker != nil && markerValid(marker, branch, worktree) {
		changed, err := writeAgentConfigs(worktree, values)
		if err != nil {
			return nil, err
		}
		result := *marker
		result.Command = string(CommandBootstrap)
		result.Mode = string(ModeIsolated)
		result.Status = "reused"
		result.WorktreeStatus = worktreeStatus
		result.ConfigChanges = changed
		result.ensureConfigPathFields(worktree)
		result.Activation.Env = values
		return &result, nil
	}
	if err := buildBinaries(worktree, values); err != nil {
		return nil, err
	}
	changed, err := writeAgentConfigs(worktree, values)
	if err != nil {
		return nil, err
	}
	opPath := filepath.Join(values["OPBIN"], "op")
	aderPath := filepath.Join(values["OPBIN"], "ader")
	opHash, err := fileSHA256(opPath)
	if err != nil {
		return nil, err
	}
	aderHash, err := fileSHA256(aderPath)
	if err != nil {
		return nil, err
	}
	head, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	paths := configPaths(worktree)
	result := &Result{
		SchemaVersion:  BootstrapSchemaVersion,
		Command:        string(CommandBootstrap),
		Mode:           string(ModeIsolated),
		Status:         "built",
		WorktreeStatus: worktreeStatus,
		Branch:         branch,
		Head:           head,
		Worktree:       worktree,
		Oppath:         values["OPPATH"],
		Opbin:          values["OPBIN"],
		Activation: Activation{
			Cwd:         worktree,
			Env:         values,
			PathPrepend: values["OPBIN"],
		},
		OpSHA256:      opHash,
		AderSHA256:    aderHash,
		OpPath:        opPath,
		AderPath:      aderPath,
		ConfigChanges: changed,
		ConfigPaths:   paths,
		BuiltAt:       time.Now().UTC().Format(time.RFC3339),
	}
	result.ensureConfigPathFields(worktree)
	if err := writeJSONFile(markerPath(worktree), result); err != nil {
		return nil, err
	}
	return result, nil
}

func Doctor() (int, *Result) {
	code, doctor := doctor()
	return code, &Result{
		SchemaVersion: BootstrapSchemaVersion,
		Command:       string(CommandDoctor),
		Status:        doctorStatus(doctor.OK),
		Doctor:        doctor,
	}
}

func ActivationFromResult(result *Result) (string, map[string]string, error) {
	if result == nil {
		return "", nil, errors.New("missing worktree result")
	}
	if strings.TrimSpace(result.Activation.Cwd) == "" {
		return "", nil, errors.New("worktree result has no activation cwd")
	}
	env := map[string]string{}
	for k, v := range result.Activation.Env {
		env[k] = v
	}
	for _, key := range ManagedEnvKeys {
		if strings.TrimSpace(env[key]) == "" {
			return "", nil, fmt.Errorf("worktree result has no activation env %s", key)
		}
	}
	return result.Activation.Cwd, env, nil
}

func MergeEnv(base []string, values map[string]string) []string {
	outMap := map[string]string{}
	for _, kv := range base {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			outMap[k] = v
		}
	}
	for k, v := range values {
		outMap[k] = v
	}
	keys := make([]string, 0, len(outMap))
	for k := range outMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+outMap[k])
	}
	return out
}

func LookPathInEnv(file string, env map[string]string) (string, error) {
	if strings.ContainsAny(file, `/\`) {
		return file, nil
	}
	pathValue := env["PATH"]
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, file)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", exec.ErrNotFound
}

func (r *Result) ensureConfigPathFields(worktree string) {
	if r.ConfigPaths == nil {
		r.ConfigPaths = configPaths(worktree)
	}
	r.BootstrapJSON = markerPath(worktree)
	r.CodexConfigTOML = r.ConfigPaths["codex_config_toml"]
	r.ClaudeSettingsLocalJSON = r.ConfigPaths["claude_settings_local_json"]
	r.GeminiEnv = r.ConfigPaths["gemini_env"]
	r.VSCodeSettingsJSON = r.ConfigPaths["vscode_settings_json"]
}

func ensureOPWorktree(branch string) (string, string, string, error) {
	if strings.TrimSpace(branch) == "" || strings.HasPrefix(branch, "-") {
		return "", "", "", fmt.Errorf("invalid branch name: %q", branch)
	}
	if _, err := gitOutput("", "check-ref-format", "--branch", branch); err != nil {
		return "", "", "", fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	root, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", "", errors.New("not inside a git checkout")
	}
	root, _ = filepath.Abs(root)
	base, err := mainGitWorktree(root)
	if err != nil {
		return "", "", "", err
	}
	target := filepath.Join(filepath.Dir(base), filepath.Base(base)+"-"+slugBranch(branch))
	target, _ = filepath.Abs(target)
	record, err := findGitWorktree(base, target)
	if err != nil {
		return "", "", "", err
	}
	if _, err := os.Stat(target); err == nil {
		if record == nil {
			return "", "", "", fmt.Errorf("target path already exists with conflicting content: %s", target)
		}
		if shortBranch(record["branch"]) != branch {
			return "", "", "", fmt.Errorf("target path is a worktree for %q, not requested branch %q: %s", shortBranch(record["branch"]), branch, target)
		}
		return base, target, "reused", nil
	} else if !os.IsNotExist(err) {
		return "", "", "", err
	}
	if record != nil {
		return "", "", "", fmt.Errorf("git reports an existing worktree at missing path: %s", target)
	}

	args := []string{"worktree", "add"}
	switch {
	case gitQuiet(base, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil:
		args = append(args, target, branch)
	case gitQuiet(base, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch) == nil:
		args = append(args, "-b", branch, target, "origin/"+branch)
	default:
		args = append(args, "-b", branch, target, "HEAD")
	}
	if _, err := gitOutput(base, args...); err != nil {
		return "", "", "", err
	}
	return base, target, "created", nil
}

func mainGitWorktree(root string) (string, error) {
	records, err := gitWorktrees(root)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return root, nil
	}
	return filepath.Abs(records[0]["worktree"])
}

func findGitWorktree(root, target string) (map[string]string, error) {
	records, err := gitWorktrees(root)
	if err != nil {
		return nil, err
	}
	for _, rec := range records {
		p, _ := filepath.Abs(rec["worktree"])
		if p == target {
			return rec, nil
		}
	}
	return nil, nil
}

func gitWorktrees(root string) ([]map[string]string, error) {
	out, err := gitOutput(root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var records []map[string]string
	current := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if len(current) > 0 {
				records = append(records, current)
				current = map[string]string{}
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		current[key] = value
	}
	if len(current) > 0 {
		records = append(records, current)
	}
	return records, nil
}

func gitOutput(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitQuiet(cwd string, args ...string) error {
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	return cmd.Run()
}

func shortBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func slugBranch(branch string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range branch {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "branch"
	}
	return slug
}

func envValues(worktree string, marker *Result) map[string]string {
	oppath := filepath.Join(worktree, ".op")
	opbin := filepath.Join(oppath, "bin")
	pathValue := prependPath(opbin, os.Getenv("PATH"))
	if marker != nil && marker.Activation.Env != nil {
		if p := marker.Activation.Env["PATH"]; p != "" {
			pathValue = p
		}
	}
	return map[string]string{"OPPATH": oppath, "OPBIN": opbin, "PATH": pathValue}
}

func prependPath(first, rest string) string {
	if rest == "" {
		return first
	}
	return first + string(os.PathListSeparator) + rest
}

func configPaths(w string) map[string]string {
	return map[string]string{
		"codex_config_toml":          filepath.Join(w, ".codex", "config.toml"),
		"claude_settings_local_json": filepath.Join(w, ".claude", "settings.local.json"),
		"gemini_env":                 filepath.Join(w, ".gemini", ".env"),
		"vscode_settings_json":       filepath.Join(w, ".vscode", "settings.json"),
	}
}

func buildBinaries(worktree string, values map[string]string) error {
	if err := os.MkdirAll(values["OPBIN"], 0o755); err != nil {
		return err
	}
	if _, err := seedBootstrapSDKPool(values); err != nil {
		return err
	}
	env := MergeEnv(os.Environ(), values)
	opCommand := "op"
	if err := runBuildCommand(opCommand, worktree, env, "op"); err != nil {
		return err
	}
	localOp := filepath.Join(values["OPBIN"], "op")
	if fileExists(localOp) {
		opCommand = localOp
	}
	if err := runBuildCommand(opCommand, worktree, env, "clem-ader"); err != nil {
		return err
	}
	for _, name := range []string{"op", "ader"} {
		if err := ensureDirectBinary(values["OPBIN"], name); err != nil {
			return err
		}
		if !fileExists(filepath.Join(values["OPBIN"], name)) {
			return fmt.Errorf("bootstrap did not produce expected binary: %s", filepath.Join(values["OPBIN"], name))
		}
	}
	return nil
}

func ensureDirectBinary(opbin, name string) error {
	direct := filepath.Join(opbin, name)
	if fileExists(direct) {
		return nil
	}
	pattern := filepath.Join(opbin, "*.holon", "bin", runtimeArchitecture(), name)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}
	rel, err := filepath.Rel(opbin, matches[0])
	if err != nil {
		return err
	}
	if _, err := os.Lstat(direct); err == nil {
		if err := os.Remove(direct); err != nil {
			return err
		}
	}
	if err := os.Symlink(rel, direct); err != nil {
		return fmt.Errorf("symlink %s: %w", direct, err)
	}
	return nil
}

func runtimeArchitecture() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

func seedBootstrapSDKPool(values map[string]string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil, nil
	}
	srcRoot := filepath.Join(home, ".op", "sdk")
	dstRoot := filepath.Join(values["OPPATH"], "sdk")
	if samePath(srcRoot, dstRoot) {
		return nil, nil
	}
	var copied []string
	for _, name := range []string{"go", "shared"} {
		src := filepath.Join(srcRoot, name)
		if !dirExists(src) {
			continue
		}
		dst := filepath.Join(dstRoot, name)
		if fileExists(dst) {
			continue
		}
		if err := copyTree(src, dst); err != nil {
			return copied, fmt.Errorf("seed bootstrap SDK %s: %w", name, err)
		}
		copied = append(copied, dst)
	}
	return copied, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm())
		case entry.Type().IsRegular():
			return copyFile(path, target, info.Mode().Perm())
		default:
			return nil
		}
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func writeAgentConfigs(worktree string, values map[string]string) ([]string, error) {
	paths := configPaths(worktree)
	var changed []string
	fns := []struct {
		path string
		fn   func(string, map[string]string) (bool, error)
	}{
		{paths["codex_config_toml"], mergeCodexConfig},
		{paths["claude_settings_local_json"], mergeClaudeSettings},
		{paths["gemini_env"], mergeGeminiEnv},
		{paths["vscode_settings_json"], mergeVSCodeSettings},
	}
	for _, item := range fns {
		ok, err := item.fn(item.path, values)
		if err != nil {
			return nil, err
		}
		if ok {
			changed = append(changed, item.path)
		}
	}
	return changed, nil
}

func mergeCodexConfig(path string, values map[string]string) (bool, error) {
	section := "[shell_environment_policy.set]"
	lines := readLines(path)
	start, end := -1, len(lines)
	for i, line := range lines {
		if strings.TrimSpace(line) == section {
			start = i
			for j := i + 1; j < len(lines); j++ {
				s := strings.TrimSpace(lines[j])
				if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
					end = j
					break
				}
			}
			break
		}
	}
	if start == -1 {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, section)
		for _, k := range ManagedEnvKeys {
			lines = append(lines, fmt.Sprintf("%s = %q", k, values[k]))
		}
	} else {
		seen := map[string]bool{}
		for i := start + 1; i < end; i++ {
			k, v, ok := parseSimpleAssignment(lines[i])
			if !ok || !managedEnvKey(k) {
				continue
			}
			seen[k] = true
			if v != values[k] {
				return false, fmt.Errorf("%s has conflicting %s: expected %q, found %q", path, k, values[k], v)
			}
		}
		insert := end
		for _, k := range ManagedEnvKeys {
			if !seen[k] {
				lines = append(lines[:insert], append([]string{fmt.Sprintf("%s = %q", k, values[k])}, lines[insert:]...)...)
				insert++
			}
		}
	}
	return writeIfChanged(path, strings.Join(lines, "\n")+"\n")
}

func parseSimpleAssignment(line string) (string, string, bool) {
	k, raw, ok := strings.Cut(strings.TrimSpace(line), "=")
	if !ok {
		return "", "", false
	}
	k = strings.TrimSpace(k)
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		var s string
		if json.Unmarshal([]byte(raw), &s) == nil {
			return k, s, true
		}
	}
	return k, raw, true
}

func managedEnvKey(k string) bool {
	for _, item := range ManagedEnvKeys {
		if item == k {
			return true
		}
	}
	return false
}

func mergeClaudeSettings(path string, values map[string]string) (bool, error) {
	data := map[string]any{}
	if fileExists(path) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return false, err
		}
	}
	envAny, _ := data["env"].(map[string]any)
	if envAny == nil {
		envAny = map[string]any{}
		data["env"] = envAny
	}
	for _, k := range ManagedEnvKeys {
		if current, ok := envAny[k].(string); ok && current != values[k] {
			return false, fmt.Errorf("%s has conflicting env.%s: expected %q, found %q", path, k, values[k], current)
		}
		envAny[k] = values[k]
	}
	return writeJSONFileChanged(path, data)
}

func mergeGeminiEnv(path string, values map[string]string) (bool, error) {
	lines := readLines(path)
	seen := map[string]bool{}
	for _, line := range lines {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !managedEnvKey(k) {
			continue
		}
		seen[k] = true
		if v != values[k] {
			return false, fmt.Errorf("%s has conflicting %s: expected %q, found %q", path, k, values[k], v)
		}
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		for _, k := range ManagedEnvKeys {
			if !seen[k] {
				lines = append(lines, "")
				break
			}
		}
	}
	for _, k := range ManagedEnvKeys {
		if !seen[k] {
			lines = append(lines, k+"="+values[k])
		}
	}
	return writeIfChanged(path, strings.Join(lines, "\n")+"\n")
}

func mergeVSCodeSettings(path string, _ map[string]string) (bool, error) {
	data := map[string]any{}
	if fileExists(path) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if err := json.Unmarshal(stripJSONComments(raw), &data); err != nil {
			return false, err
		}
	}
	key := vscodeEnvKey()
	envAny, _ := data[key].(map[string]any)
	if envAny == nil {
		envAny = map[string]any{}
		data[key] = envAny
	}
	expected := map[string]string{"OPPATH": "${workspaceFolder}/.op", "OPBIN": "${workspaceFolder}/.op/bin", "PATH": "${workspaceFolder}/.op/bin:${env:PATH}"}
	for k, v := range expected {
		if current, ok := envAny[k].(string); ok && current != v {
			return false, fmt.Errorf("%s has conflicting %s.%s: expected %q, found %q", path, key, k, v, current)
		}
		envAny[k] = v
	}
	return writeJSONFileChanged(path, data)
}

func vscodeEnvKey() string {
	switch runtime.GOOS {
	case "darwin":
		return "terminal.integrated.env.osx"
	case "windows":
		return "terminal.integrated.env.windows"
	default:
		return "terminal.integrated.env.linux"
	}
}

func stripJSONComments(raw []byte) []byte {
	var out bytes.Buffer
	inString, esc := false, false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			out.WriteByte(ch)
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(raw) && raw[i+1] == '/' {
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			if i < len(raw) {
				out.WriteByte(raw[i])
			}
			continue
		}
		if ch == '/' && i+1 < len(raw) && raw[i+1] == '*' {
			i += 2
			for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		out.WriteByte(ch)
	}
	return out.Bytes()
}

func markerPath(worktree string) string {
	return filepath.Join(worktree, ".op", ".bootstrap.json")
}

func loadMarker(worktree string) (*Result, error) {
	raw, err := os.ReadFile(markerPath(worktree))
	if err != nil {
		return nil, err
	}
	var data Result
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func markerValid(marker *Result, branch, worktree string) bool {
	if marker.SchemaVersion != BootstrapSchemaVersion || marker.Branch != branch || marker.Worktree != worktree {
		return false
	}
	head, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil || marker.Head != head {
		return false
	}
	for k, v := range configPaths(worktree) {
		if marker.ConfigPaths != nil && marker.ConfigPaths[k] != "" {
			if marker.ConfigPaths[k] != v {
				return false
			}
			continue
		}
		switch k {
		case "codex_config_toml":
			if marker.CodexConfigTOML != v {
				return false
			}
		case "claude_settings_local_json":
			if marker.ClaudeSettingsLocalJSON != v {
				return false
			}
		case "gemini_env":
			if marker.GeminiEnv != v {
				return false
			}
		case "vscode_settings_json":
			if marker.VSCodeSettingsJSON != v {
				return false
			}
		}
	}
	opPath := filepath.Join(worktree, ".op", "bin", "op")
	aderPath := filepath.Join(worktree, ".op", "bin", "ader")
	opHash, err1 := fileSHA256(opPath)
	aderHash, err2 := fileSHA256(aderPath)
	return err1 == nil && err2 == nil && marker.OpSHA256 == opHash && marker.AderSHA256 == aderHash
}

func doctor() (int, *DoctorResult) {
	root, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		return 2, &DoctorResult{OK: false, Error: "not inside a git checkout", RecommendedCode: 2}
	}
	root, _ = filepath.Abs(root)
	expectedOPPATH := filepath.Join(root, ".op")
	expectedOPBIN := filepath.Join(expectedOPPATH, "bin")
	oppath := os.Getenv("OPPATH")
	opbin := os.Getenv("OPBIN")
	pathValue := os.Getenv("PATH")
	opPath, _ := exec.LookPath("op")
	aderPath, _ := exec.LookPath("ader")
	firstPath := ""
	if parts := filepath.SplitList(pathValue); len(parts) > 0 {
		firstPath = parts[0]
	}
	checks := map[string]bool{
		"oppath":     samePath(oppath, expectedOPPATH),
		"opbin":      samePath(opbin, expectedOPBIN),
		"path_first": samePath(firstPath, expectedOPBIN),
		"op_path":    opPath != "" && samePath(filepath.Dir(opPath), expectedOPBIN),
		"ader_path":  aderPath != "" && samePath(filepath.Dir(aderPath), expectedOPBIN),
	}
	ok := true
	for _, v := range checks {
		if !v {
			ok = false
		}
	}
	code := 0
	if !ok {
		code = 1
	}
	return code, &DoctorResult{
		OK:              ok,
		Cwd:             mustAbs("."),
		RepoRoot:        root,
		ExpectedOPPATH:  expectedOPPATH,
		ExpectedOPBIN:   expectedOPBIN,
		OPPATH:          oppath,
		OPBIN:           opbin,
		OpPath:          opPath,
		AderPath:        aderPath,
		Checks:          checks,
		RecommendedCode: code,
	}
}

func doctorStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return aa == bb
}

func mustAbs(p string) string {
	out, _ := filepath.Abs(p)
	return out
}

func writeJSONFile(path string, data any) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func writeJSONFileChanged(path string, data any) (bool, error) {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return false, err
	}
	raw = append(raw, '\n')
	return writeIfChanged(path, string(raw))
}

func writeIfChanged(path, content string) (bool, error) {
	if raw, err := os.ReadFile(path); err == nil && string(raw) == content {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(content), 0o644)
}

func readLines(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.TrimSuffix(string(raw), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func fileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

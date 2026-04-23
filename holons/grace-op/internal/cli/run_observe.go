package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runObserveOptions describes the observability-related flags carried
// by `op run`. They translate to the OP_OBS / OP_PROM_ADDR / OP_OTEL_*
// env vars the SDK consumes.
type runObserveOptions struct {
	// Observe is a comma-separated subset of logs,metrics,events,prom,all.
	// Empty means inherit OP_OBS from the parent env (or no observability).
	Observe string

	// Prom is either an address to bind (":9091") or the magic ":0" for
	// ephemeral. Empty means don't force prom (if OP_OBS already has it
	// in --observe, the SDK picks an ephemeral port itself).
	Prom string

	// OTel endpoint ("host:port"). v2 feature; when set in v1 the SDK
	// rejects it at startup, which op forwards.
	OTel string

	// Session mode. Empty ignores; "1" enables sessions; "metrics"
	// enables session metrics too (see SESSIONS.md).
	Sessions string

	// JSON asks op run to emit a single JSON object for the UID return
	// contract (INSTANCES.md §UID Return Contract).
	JSON bool
}

// extractRunObserveFlags scans args for --observe / --prom / --otel /
// --sessions / --json, removes them, and returns the options plus the
// remaining args.
func extractRunObserveFlags(args []string) ([]string, runObserveOptions) {
	var opts runObserveOptions
	out := args[:0]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--observe":
			opts.Observe = "logs,metrics,events"
		case strings.HasPrefix(a, "--observe="):
			opts.Observe = strings.TrimPrefix(a, "--observe=")
		case a == "--prom":
			opts.Prom = ":0"
			if opts.Observe == "" {
				opts.Observe = "metrics,prom"
			} else if !strings.Contains(opts.Observe, "prom") {
				opts.Observe += ",prom"
			}
		case strings.HasPrefix(a, "--prom="):
			opts.Prom = strings.TrimPrefix(a, "--prom=")
			if !strings.Contains(opts.Observe, "prom") {
				if opts.Observe == "" {
					opts.Observe = "metrics,prom"
				} else {
					opts.Observe += ",prom"
				}
			}
		case strings.HasPrefix(a, "--otel="):
			opts.OTel = strings.TrimPrefix(a, "--otel=")
			if opts.Observe == "" {
				opts.Observe = "metrics,otel"
			} else if !strings.Contains(opts.Observe, "otel") {
				opts.Observe += ",otel"
			}
		case a == "--sessions":
			opts.Sessions = "1"
		case strings.HasPrefix(a, "--sessions="):
			opts.Sessions = strings.TrimPrefix(a, "--sessions=")
		case a == "--json":
			opts.JSON = true
		default:
			out = append(out, a)
		}
	}
	return out, opts
}

// newInstanceUID returns a 16-byte (32-char hex) UID suitable for the
// INSTANCES.md registry. Falls back to time-based digits when the
// crypto random source is unavailable.
func newInstanceUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	// Fallback: not cryptographically strong but still unique per
	// process millisecond.
	return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixMicro())
}

// resolveRunRoot implements the path-resolution priority from
// INSTANCES.md §Path Resolution:
//
//	$OP_RUN_DIR → $OPPATH/run → <OPROOT>/.op/run → ./.op/run
//
// Returns the first writable candidate. The caller can create the
// per-instance subdirectory afterwards.
func resolveRunRoot() (string, error) {
	candidates := []string{
		os.Getenv("OP_RUN_DIR"),
	}
	if pathRoot := os.Getenv("OPPATH"); pathRoot != "" {
		candidates = append(candidates, filepath.Join(pathRoot, "run"))
	}
	if opRoot := os.Getenv("OPROOT"); opRoot != "" {
		candidates = append(candidates, filepath.Join(opRoot, ".op", "run"))
	}
	candidates = append(candidates, filepath.Join(".op", "run"))

	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if err := os.MkdirAll(cand, 0o755); err != nil {
			continue
		}
		abs, err := filepath.Abs(cand)
		if err != nil {
			continue
		}
		return abs, nil
	}
	return "", fmt.Errorf("no writable run root candidate")
}

// prepareInstanceRunDir allocates a UID, creates the per-instance
// directory (<runRoot>/<slug>/<uid>/), and returns the absolute path.
func prepareInstanceRunDir(runRoot, slug, uid string) (string, error) {
	dir := filepath.Join(runRoot, slug, uid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// injectObservabilityEnv sets OP_INSTANCE_UID, OP_RUN_DIR and the
// observability family env vars on the given command. Leaves the
// caller's cmd.Env untouched if empty (inherits parent env).
func injectObservabilityEnv(cmd *exec.Cmd, uid, runDir string, opts runObserveOptions) {
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	set := func(key, value string) {
		prefix := key + "="
		for i, e := range env {
			if strings.HasPrefix(e, prefix) {
				env[i] = prefix + value
				return
			}
		}
		env = append(env, prefix+value)
	}
	set("OP_INSTANCE_UID", uid)
	set("OP_RUN_DIR", runDir)
	if opts.Observe != "" {
		set("OP_OBS", opts.Observe)
	}
	if opts.Prom != "" {
		set("OP_PROM_ADDR", opts.Prom)
	}
	if opts.OTel != "" {
		set("OP_OTEL_ENDPOINT", opts.OTel)
	}
	if opts.Sessions != "" {
		set("OP_SESSIONS", opts.Sessions)
	}
	cmd.Env = env
}

// emitUIDReturn implements INSTANCES.md §UID Return Contract: prints
// either "uid: <uid>\n" (default) or a single JSON object (with --json)
// to stdout before the child's output begins.
func emitUIDReturn(uid, slug, address string, pid int, metricsAddr string, jsonMode bool) {
	if jsonMode {
		payload := map[string]any{
			"uid":     uid,
			"slug":    slug,
			"address": address,
		}
		if pid > 0 {
			payload["pid"] = pid
		}
		if metricsAddr != "" {
			payload["metrics_addr"] = metricsAddr
		}
		_ = json.NewEncoder(os.Stdout).Encode(payload)
		return
	}
	fmt.Printf("uid: %s\n", uid)
}

// applyRunObservability is called between commandForArtifact and
// runForeground. When opts requests observability or the UID-return
// contract, it allocates a UID, creates the per-instance registry
// directory, injects the env vars into cmd, and emits the `uid:` line
// (or JSON). Returns the allocated UID and run directory for later
// use (e.g. meta.json prefill, op ps discovery). Returns ("", "") as
// a no-op signal when observability is not requested.
func applyRunObservability(cmd *exec.Cmd, slug string, opts runObserveOptions) (uid, runDir string, err error) {
	// Fast path: no observability requested and no JSON UID contract
	// needed — leave the command untouched to preserve existing behaviour.
	if opts.Observe == "" && opts.Prom == "" && opts.OTel == "" && opts.Sessions == "" && !opts.JSON {
		return "", "", nil
	}
	runRoot, err := resolveRunRoot()
	if err != nil {
		return "", "", err
	}
	uid = newInstanceUID()
	runDir, err = prepareInstanceRunDir(runRoot, slug, uid)
	if err != nil {
		return "", "", err
	}
	injectObservabilityEnv(cmd, uid, runDir, opts)
	emitUIDReturn(uid, slug, "", 0, "", opts.JSON)
	return uid, runDir, nil
}

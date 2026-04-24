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
// by `op run`. In v1 they translate to the OP_OBS / OP_PROM_ADDR env
// vars the SDK consumes; OTLP is parsed only to reject the reserved flag.
type runObserveOptions struct {
	// Observe is a comma-separated subset of logs,metrics,events,prom,all.
	// Empty means inherit OP_OBS from the parent env (or no observability).
	Observe string

	// Prom is either an address to bind (":9091") or the magic ":0" for
	// ephemeral. Empty means don't force prom (if OP_OBS already has it
	// in --observe, the SDK picks an ephemeral port itself).
	Prom string

	// OTel endpoint ("host:port"). Reserved for v2 and rejected by op
	// before spawning in v1.
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
// Returns the first writable candidate. This is the registry root
// injected into children; SDKs derive <root>/<slug>/<uid>/ themselves.
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

// injectObservabilityEnv sets OP_INSTANCE_UID, OP_RUN_DIR and the
// observability family env vars on the given command. Leaves the
// caller's cmd.Env untouched if empty (inherits parent env).
func injectObservabilityEnv(cmd *exec.Cmd, uid, runRoot string, opts runObserveOptions) {
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
	set("OP_RUN_DIR", runRoot)
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
// contract, it allocates a UID, resolves the registry root, injects
// the env vars into cmd, and emits the `uid:` line (or JSON). Returns
// the allocated UID and registry root for later use. Returns ("", "")
// as a no-op signal when observability is not requested.
func applyRunObservability(cmd *exec.Cmd, slug string, opts runObserveOptions) (uid, runRoot string, err error) {
	// Fast path: no observability requested and no JSON UID contract
	// needed — leave the command untouched to preserve existing behaviour.
	if opts.Observe == "" && opts.Prom == "" && opts.OTel == "" && opts.Sessions == "" && !opts.JSON {
		return "", "", nil
	}
	if err := validateRunObserveV1(opts); err != nil {
		return "", "", err
	}
	runRoot, err = resolveRunRoot()
	if err != nil {
		return "", "", err
	}
	uid = newInstanceUID()
	injectObservabilityEnv(cmd, uid, runRoot, opts)
	emitUIDReturn(uid, slug, "", 0, "", opts.JSON)
	return uid, runRoot, nil
}

func validateRunObserveV1(opts runObserveOptions) error {
	if opts.OTel != "" {
		return fmt.Errorf("--otel is reserved for observability v2; use --prom in v1")
	}
	for _, tok := range strings.Split(opts.Observe, ",") {
		if strings.TrimSpace(tok) == "otel" {
			return fmt.Errorf("OP_OBS token %q is reserved for observability v2; use --prom in v1", tok)
		}
	}
	return nil
}

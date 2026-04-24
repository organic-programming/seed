// Package observability is the Go reference implementation of the
// cross-SDK observability layer described in OBSERVABILITY.md. It
// provides structured logs, metrics, and lifecycle events with a
// zero-cost disabled mode.
//
// Activation follows the two-layer model from the spec:
//
//   - Layer 3 (env) — OP_OBS is a comma-separated list of families
//     (logs, metrics, events, prom) and is the sole source of on/off
//     per family. Parsed by FromEnv at process start.
//   - Layer 1 (code) — Configure supplies knobs (default log level,
//     Prometheus bind address, redacted field names) that apply
//     whenever a family is enabled.
//
// A single package-scope Observability holds the active state. Logger,
// Counter, Gauge, Histogram, and Events are all no-ops when their
// family is disabled.
package observability

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Family identifies a group of signals the layer can emit.
type Family string

const (
	FamilyLogs    Family = "logs"
	FamilyMetrics Family = "metrics"
	FamilyEvents  Family = "events"
	FamilyProm    Family = "prom"
	FamilyOTel    Family = "otel" // reserved v2
)

// OP_OBS tokens that are valid in v1.
var v1Tokens = map[string]bool{
	"logs":    true,
	"metrics": true,
	"events":  true,
	"prom":    true,
	"all":     true,
}

// Config supplies knobs for when a family is enabled. None of these
// fields enables a family on their own — OP_OBS does.
type Config struct {
	// Slug of the holon (used as a well-known field on every signal).
	// Empty means the SDK will fall back to os.Args[0] basename.
	Slug string

	// Default log level applied to every new Logger. Per-logger
	// overrides use Logger.SetLevel. Zero value = INFO.
	DefaultLogLevel Level

	// Prometheus HTTP bind address when OP_OBS contains "prom".
	// ":0" means an ephemeral port. Ignored otherwise.
	PromAddr string

	// Names whose values are replaced by "<redacted>" in log fields
	// and event payloads before emission.
	RedactedFields []string

	// Ring buffer capacity for logs (default 1024) and events
	// (default 256).
	LogsRingSize   int
	EventsRingSize int

	// Run directory where stdout.log / events.jsonl / meta.json are
	// written. FromEnv derives this from OP_RUN_DIR + Slug +
	// InstanceUID because OP_RUN_DIR is the registry root.
	RunDir string

	// Instance identity from the supervisor. FromEnv fills these from
	// OP_INSTANCE_UID / OP_ORGANISM_UID / OP_ORGANISM_SLUG.
	InstanceUID  string
	OrganismUID  string
	OrganismSlug string
}

// Observability is the live state of the layer. The zero value is a
// no-op configuration with every family disabled — calls through it
// are safe and effectively free.
type Observability struct {
	cfg Config

	// Families that Layer 3 enabled. Immutable after Configure.
	families map[Family]bool

	// Lazy-built subsystems; nil when the corresponding family is off.
	registry *Registry
	bus      *EventBus
	ringLogs *LogRing
	loggers  sync.Map // map[string]*Logger

	// Redaction lookup.
	redact map[string]struct{}

	// Monotonic base for entry timestamps; locked once.
	startWall time.Time
}

// current is the package-scope instance returned by the constructors.
// Subsequent Configure / FromEnv calls replace it; the previous value
// is closed. A nil current means every accessor returns a no-op view.
var (
	currentMu sync.RWMutex
	current   *Observability
)

// Configure installs a new Observability with the given configuration
// and returns it. Families are read from OP_OBS at call time. Calling
// Configure again replaces the current instance; old loggers, metrics,
// and the event bus keep working against the previous instance until
// it is closed.
func Configure(cfg Config) *Observability {
	families := parseOPOBS(os.Getenv("OP_OBS"))

	// Defaults.
	if cfg.LogsRingSize <= 0 {
		cfg.LogsRingSize = 1024
	}
	if cfg.EventsRingSize <= 0 {
		cfg.EventsRingSize = 256
	}
	if cfg.DefaultLogLevel == LevelUnset {
		cfg.DefaultLogLevel = LevelInfo
	}
	if cfg.Slug == "" {
		cfg.Slug = defaultSlug()
	}

	obs := &Observability{
		cfg:       cfg,
		families:  families,
		redact:    toSet(cfg.RedactedFields),
		startWall: time.Now(),
	}

	if obs.families[FamilyLogs] {
		obs.ringLogs = NewLogRing(cfg.LogsRingSize)
	}
	if obs.families[FamilyMetrics] {
		obs.registry = NewRegistry()
	}
	if obs.families[FamilyEvents] {
		obs.bus = NewEventBus(cfg.EventsRingSize)
	}

	currentMu.Lock()
	current = obs
	currentMu.Unlock()
	return obs
}

// FromEnv is a convenience wrapper that applies Configure with fields
// populated from the environment: OP_INSTANCE_UID, OP_ORGANISM_UID,
// OP_ORGANISM_SLUG, OP_PROM_ADDR, OP_RUN_DIR. Any caller-provided
// fields in base override the env.
func FromEnv(base Config) *Observability {
	cfg := base
	if cfg.Slug == "" {
		cfg.Slug = defaultSlug()
	}
	if cfg.InstanceUID == "" {
		cfg.InstanceUID = os.Getenv("OP_INSTANCE_UID")
	}
	if cfg.OrganismUID == "" {
		cfg.OrganismUID = os.Getenv("OP_ORGANISM_UID")
	}
	if cfg.OrganismSlug == "" {
		cfg.OrganismSlug = os.Getenv("OP_ORGANISM_SLUG")
	}
	if cfg.PromAddr == "" {
		cfg.PromAddr = os.Getenv("OP_PROM_ADDR")
	}
	if cfg.RunDir == "" {
		if runRoot := os.Getenv("OP_RUN_DIR"); runRoot != "" && cfg.Slug != "" && cfg.InstanceUID != "" {
			if runDir, err := InstanceRunDir(runRoot, cfg.Slug, cfg.InstanceUID); err == nil {
				cfg.RunDir = runDir
			}
		}
	}
	return Configure(cfg)
}

// Current returns the active Observability. When none is configured,
// it returns a zero-value, disabled instance so callers don't need
// nil checks on the hot path.
func Current() *Observability {
	currentMu.RLock()
	defer currentMu.RUnlock()
	if current == nil {
		return disabled
	}
	return current
}

// Reset closes the current instance and installs a fresh disabled one.
// Intended for tests.
func Reset() {
	currentMu.Lock()
	current = nil
	currentMu.Unlock()
}

// disabled is a shared zero-value Observability with every family off.
// Pointers to it are returned by Current() when no instance is set.
var disabled = &Observability{families: map[Family]bool{}}

// Enabled reports whether the given family is active.
func (o *Observability) Enabled(f Family) bool {
	if o == nil {
		return false
	}
	return o.families[f]
}

// Slug returns the holon slug set on this instance (may be empty).
func (o *Observability) Slug() string {
	if o == nil {
		return ""
	}
	return o.cfg.Slug
}

// InstanceUID returns the instance UID set on this instance.
func (o *Observability) InstanceUID() string {
	if o == nil {
		return ""
	}
	return o.cfg.InstanceUID
}

// RunDir returns the per-instance registry directory, when configured.
func (o *Observability) RunDir() string {
	if o == nil {
		return ""
	}
	return o.cfg.RunDir
}

// OrganismUID returns the organism UID set on this instance.
func (o *Observability) OrganismUID() string { return o.cfg.OrganismUID }

// OrganismSlug returns the organism slug set on this instance.
func (o *Observability) OrganismSlug() string { return o.cfg.OrganismSlug }

// IsOrganismRoot reports whether this holon is the organism root.
// Canonical test from INSTANCES.md: OP_ORGANISM_UID == OP_INSTANCE_UID.
func (o *Observability) IsOrganismRoot() bool {
	if o == nil || o.cfg.OrganismUID == "" {
		return false
	}
	return o.cfg.OrganismUID == o.cfg.InstanceUID
}

// Close releases any resources held by the subsystems. Safe to call
// multiple times; subsequent calls are no-ops. Not reversible.
func (o *Observability) Close() error {
	if o == nil || o == disabled {
		return nil
	}
	if o.bus != nil {
		o.bus.Close()
	}
	// Logger ring buffer and registry are GC'd with the instance.
	return nil
}

// parseOPOBS turns the comma-separated env value into a family set.
// "all" expands to logs+metrics+events+prom. Unknown and v2-only
// tokens are dropped here so Configure can stay panic-free; CheckEnv
// applies the fail-fast startup policy.
func parseOPOBS(v string) map[Family]bool {
	out := map[Family]bool{}
	v = strings.TrimSpace(v)
	if v == "" {
		return out
	}
	for _, raw := range strings.Split(v, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		if isV2OnlyToken(tok) {
			continue
		}
		if !v1Tokens[tok] {
			// Unknown token. Same policy as above — the serve runner
			// surfaces this as a startup error via CheckEnv.
			continue
		}
		switch tok {
		case "all":
			out[FamilyLogs] = true
			out[FamilyMetrics] = true
			out[FamilyEvents] = true
			out[FamilyProm] = true
		default:
			out[Family(tok)] = true
		}
	}
	return out
}

// CheckEnv returns a non-nil error if OP_OBS contains an unknown or
// v2-only token, or if OP_SESSIONS is set in v1. Call once at process
// start to satisfy the fail-fast rule from OBSERVABILITY.md §Layer 3.
func CheckEnv() error {
	if v := strings.TrimSpace(os.Getenv("OP_SESSIONS")); v != "" {
		return &InvalidTokenError{
			Var:    "OP_SESSIONS",
			Token:  v,
			Reason: "sessions are reserved for v2; not implemented in v1",
		}
	}
	v := strings.TrimSpace(os.Getenv("OP_OBS"))
	if v == "" {
		return nil
	}
	for _, raw := range strings.Split(v, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		if tok == "otel" {
			return &InvalidTokenError{Token: tok, Reason: "otel export is reserved for v2; not implemented in v1"}
		}
		if tok == "sessions" {
			return &InvalidTokenError{Token: tok, Reason: "sessions are reserved for v2; not implemented in v1"}
		}
		if !v1Tokens[tok] {
			return &InvalidTokenError{Token: tok, Reason: "unknown OP_OBS token"}
		}
	}
	return nil
}

func isV2OnlyToken(tok string) bool {
	return tok == "otel" || tok == "sessions"
}

// InvalidTokenError is returned by CheckEnv when OP_OBS contains an
// invalid token.
type InvalidTokenError struct {
	Var    string
	Token  string
	Reason string
}

func (e *InvalidTokenError) Error() string {
	name := e.Var
	if name == "" {
		name = "OP_OBS"
	}
	return name + ": " + e.Reason + ": " + e.Token
}

func toSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(items))
	for _, s := range items {
		out[s] = struct{}{}
	}
	return out
}

// defaultSlug derives a best-effort slug from os.Args[0]'s basename.
// Used when the caller did not set Config.Slug explicitly.
func defaultSlug() string {
	if len(os.Args) == 0 {
		return ""
	}
	base := os.Args[0]
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return base
}

package observability

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// Level mirrors the proto LogLevel enum. Numeric values are stable.
type Level int32

const (
	LevelUnset Level = 0
	LevelTrace Level = 1
	LevelDebug Level = 2
	LevelInfo  Level = 3
	LevelWarn  Level = 4
	LevelError Level = 5
	LevelFatal Level = 6
)

// String returns the enum name (e.g. "INFO").
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNSPECIFIED"
	}
}

// ParseLevel returns the Level for a case-insensitive name. Unknown
// inputs resolve to LevelInfo.
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "FATAL":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// LogEntry is the in-memory representation of a structured log line.
// Convert to proto with ToProto.
type LogEntry struct {
	Timestamp   time.Time
	Level       Level
	Slug        string
	InstanceUID string
	SessionID   string
	RPCMethod   string
	Message     string
	Fields      map[string]string
	Caller      string // "file:line"
	Chain       []Hop
}

// Logger emits LogEntries into the active Observability. A zero-value
// Logger is safe (all methods are no-ops); acquire one via
// Observability.Logger(name).
type Logger struct {
	obs   *Observability
	name  string
	level atomic.Int32 // Level
}

// Logger returns a named Logger. Loggers are cached per name within
// the Observability instance; repeated calls return the same pointer.
// When logs are disabled, returns a stub that drops every call.
func (o *Observability) Logger(name string) *Logger {
	if o == nil || !o.families[FamilyLogs] {
		return disabledLogger
	}
	if v, ok := o.loggers.Load(name); ok {
		return v.(*Logger)
	}
	l := &Logger{obs: o, name: name}
	l.level.Store(int32(o.cfg.DefaultLogLevel))
	actual, _ := o.loggers.LoadOrStore(name, l)
	return actual.(*Logger)
}

// Name returns the logger's name.
func (l *Logger) Name() string {
	if l == nil {
		return ""
	}
	return l.name
}

// SetLevel overrides the per-logger level. Safe to call from any goroutine.
func (l *Logger) SetLevel(lvl Level) {
	if l == nil || l.obs == nil {
		return
	}
	l.level.Store(int32(lvl))
}

// Enabled reports whether the logger would emit at the given level.
func (l *Logger) Enabled(lvl Level) bool {
	if l == nil || l.obs == nil {
		return false
	}
	return lvl >= Level(l.level.Load())
}

// log is the common emission path. keysAndValues follows the
// slog-style pattern: alternating string keys and any values.
func (l *Logger) log(ctx context.Context, lvl Level, msg string, kv []any) {
	if !l.Enabled(lvl) {
		return
	}
	fields := make(map[string]string, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, _ := kv[i].(string)
		if k == "" {
			continue
		}
		if _, redacted := l.obs.redact[k]; redacted {
			fields[k] = "<redacted>"
			continue
		}
		fields[k] = stringify(kv[i+1])
	}

	// SDK-managed well-known fields (spec §Well-known fields).
	sessionID, rpcMethod := fromContext(ctx)

	entry := LogEntry{
		Timestamp:   time.Now(),
		Level:       lvl,
		Slug:        l.obs.cfg.Slug,
		InstanceUID: l.obs.cfg.InstanceUID,
		SessionID:   sessionID,
		RPCMethod:   rpcMethod,
		Message:     msg,
		Fields:      fields,
		Caller:      callerFrame(3),
	}

	l.obs.ringLogs.Push(entry)
	// Disk write / broadcast is P2's responsibility; the ring is the
	// authoritative replay cache.
}

// Trace emits a TRACE-level log.
func (l *Logger) Trace(msg string, kv ...any) { l.log(nil, LevelTrace, msg, kv) }

// Debug emits a DEBUG-level log.
func (l *Logger) Debug(msg string, kv ...any) { l.log(nil, LevelDebug, msg, kv) }

// Info emits an INFO-level log.
func (l *Logger) Info(msg string, kv ...any) { l.log(nil, LevelInfo, msg, kv) }

// Warn emits a WARN-level log.
func (l *Logger) Warn(msg string, kv ...any) { l.log(nil, LevelWarn, msg, kv) }

// Error emits an ERROR-level log.
func (l *Logger) Error(msg string, kv ...any) { l.log(nil, LevelError, msg, kv) }

// Fatal emits a FATAL-level log. Flush-and-exit is the serve runner's
// responsibility (P2); this method only stamps the entry.
func (l *Logger) Fatal(msg string, kv ...any) { l.log(nil, LevelFatal, msg, kv) }

// InfoContext variants that take a context (for session/method
// correlation, P2 wires them via an interceptor).
func (l *Logger) InfoContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelInfo, msg, kv)
}

// WarnContext is the context-aware counterpart to Warn.
func (l *Logger) WarnContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelWarn, msg, kv)
}

// ErrorContext is the context-aware counterpart to Error.
func (l *Logger) ErrorContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelError, msg, kv)
}

// DebugContext is the context-aware counterpart to Debug.
func (l *Logger) DebugContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelDebug, msg, kv)
}

// TraceContext is the context-aware counterpart to Trace.
func (l *Logger) TraceContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelTrace, msg, kv)
}

// FatalContext is the context-aware counterpart to Fatal.
func (l *Logger) FatalContext(ctx context.Context, msg string, kv ...any) {
	l.log(ctx, LevelFatal, msg, kv)
}

// disabledLogger is the shared no-op instance returned when logs are
// off. Its methods are inlined to essentially a bounds check.
var disabledLogger = &Logger{}

// ctxKey is the package-private context key used by the interceptors
// (P2) to attach session_id + rpc_method correlation data.
type ctxKey struct{}

// CtxValues carries correlation data injected by the serve runner.
type CtxValues struct {
	SessionID string
	RPCMethod string
}

// WithContext returns a new context carrying the correlation values.
func WithContext(ctx context.Context, v CtxValues) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}

// fromContext reads (session_id, rpc_method) from ctx. Returns zero
// strings when ctx is nil or lacks the values.
func fromContext(ctx context.Context) (string, string) {
	if ctx == nil {
		return "", ""
	}
	v, _ := ctx.Value(ctxKey{}).(CtxValues)
	return v.SessionID, v.RPCMethod
}

func callerFrame(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	// Compact to package/file:line.
	short := file
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			if i > 0 {
				// keep two segments
				for j := i - 1; j >= 0; j-- {
					if file[j] == '/' {
						short = file[j+1:]
						break
					}
				}
			}
			break
		}
	}
	return fmt.Sprintf("%s:%d", short, line)
}

// stringify is the SDK's value-to-string conversion. It handles the
// common types without bringing in reflect on the hot path.
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", x)
	case int32:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case uint:
		return fmt.Sprintf("%d", x)
	case uint32:
		return fmt.Sprintf("%d", x)
	case uint64:
		return fmt.Sprintf("%d", x)
	case float32:
		return fmt.Sprintf("%g", x)
	case float64:
		return fmt.Sprintf("%g", x)
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}


package observability

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
)

// Level mirrors the proto SeverityNumber enum for the severities the SDK emits.
type Level int32

const (
	LevelUnset Level = 0
	LevelTrace Level = 1
	LevelDebug Level = 5
	LevelInfo  Level = 9
	LevelWarn  Level = 13
	LevelError Level = 17
	LevelFatal Level = 21
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

type privateMarker struct{}

// Private marks a single log or event emission as local-only. The entry is
// kept in the emitter's local ring and disk writer but is filtered out of
// HolonObservability Logs/Events streams.
func Private() any { return privateMarker{} }

func isPrivateMarker(v any) bool {
	_, ok := v.(privateMarker)
	return ok
}

// Logger emits LogRecords into the active Observability. A zero-value
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
	sessionID, rpcMethod := fromContext(ctx)
	attrs := resourceAttributes(l.obs.cfg.Slug, l.obs.cfg.InstanceUID)
	attrs = append(attrs, keyValue(AttrHolonsSessionID, sessionID))
	if l.name != "" {
		attrs = append(attrs, keyValue(AttrLoggerName, l.name))
	}
	private := false
	for i := 0; i < len(kv); {
		if isPrivateMarker(kv[i]) {
			private = true
			i++
			continue
		}
		if i+1 >= len(kv) {
			break
		}
		k, _ := kv[i].(string)
		if k == "" {
			if isPrivateMarker(kv[i+1]) {
				private = true
			}
			i += 2
			continue
		}
		if _, redacted := l.obs.redact[k]; redacted {
			attrs = append(attrs, keyValue(k, "<redacted>"))
			i += 2
			continue
		}
		attrs = append(attrs, keyValue(k, kv[i+1]))
		i += 2
	}

	// SDK-managed well-known fields (spec §Well-known fields).
	if rpcMethod != "" {
		attrs = append(attrs, keyValue(AttrRPCMethod, rpcMethod))
	}
	if caller := callerFrame(3); caller != "" {
		attrs = append(attrs, keyValue(AttrCodeCaller, caller))
	}

	now := time.Now()
	record := &v1.LogRecord{
		TimeUnixNano:         uint64(now.UnixNano()),
		ObservedTimeUnixNano: uint64(now.UnixNano()),
		SeverityNumber:       v1.SeverityNumber(lvl),
		SeverityText:         lvl.String(),
		Body:                 ToAnyValue(msg),
		Attributes:           attrs,
	}

	l.obs.ringLogs.Push(newLogRecord(record, private))
	// The ring is the authoritative replay cache.
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

// Fatal emits a FATAL-level log. Process shutdown is the serve runner's
// responsibility; this method only stamps the record.
func (l *Logger) Fatal(msg string, kv ...any) { l.log(nil, LevelFatal, msg, kv) }

// InfoContext variants that take a context for session/method correlation.
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

// ctxKey is the package-private context key used by the interceptors to
// attach session_id + rpc_method correlation data.
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

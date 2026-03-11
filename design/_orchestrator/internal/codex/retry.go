package codex

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/logging"
)

type ErrorCategory int

const (
	ErrNetwork ErrorCategory = iota
	ErrQuota
	ErrTaskFailure
	ErrSandboxViolation
)

var (
	sleepFn  = time.Sleep
	nowFn    = time.Now
	jitterFn = func(max int) int {
		if max <= 0 {
			return 0
		}
		return rand.Intn(max)
	}
)

type retryAbortError struct {
	err error
}

func (e *retryAbortError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *retryAbortError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func Classify(exitCode int, stderr string) ErrorCategory {
	text := strings.ToLower(stderr)
	switch {
	case strings.Contains(text, "sandbox"), strings.Contains(text, "permission denied"):
		return ErrSandboxViolation
	case strings.Contains(text, "429"), strings.Contains(text, "rate limit"), strings.Contains(text, "quota"), strings.Contains(text, "capacity"):
		return ErrQuota
	case strings.Contains(text, "connection"), strings.Contains(text, "timeout"), strings.Contains(text, "dns"), strings.Contains(text, "econnrefused"):
		return ErrNetwork
	default:
		_ = exitCode
		return ErrTaskFailure
	}
}

func RetryWithBackoff(category ErrorCategory, fn func() error, logger *logging.TeeWriter) error {
	delays := retryDelays(category)
	if len(delays) == 0 {
		return fn()
	}

	var lastErr error
	for attempt, delay := range delays {
		delay = applyJitter(delay)
		logRetryWait(logger, category, attempt+1, delay)
		waitWithHeartbeat(delay, logger)

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		var abort *retryAbortError
		if errors.As(lastErr, &abort) {
			return abort.err
		}
	}

	return lastErr
}

func retryDelays(category ErrorCategory) []time.Duration {
	switch category {
	case ErrNetwork:
		return []time.Duration{
			5 * time.Second,
			15 * time.Second,
			45 * time.Second,
			2 * time.Minute,
			5 * time.Minute,
		}
	case ErrQuota:
		return []time.Duration{
			15 * time.Minute,
			30 * time.Minute,
			time.Hour,
		}
	default:
		return nil
	}
}

func logRetryWait(logger *logging.TeeWriter, category ErrorCategory, attempt int, delay time.Duration) {
	message := fmt.Sprintf(
		"[retry] category=%s attempt=%d next_retry_at=%s",
		category.String(),
		attempt,
		nowFn().Add(delay).Format(time.RFC3339),
	)
	if logger != nil {
		logger.WriteLine(message)
	}
}

func waitWithHeartbeat(delay time.Duration, logger *logging.TeeWriter) {
	remaining := delay
	const heartbeat = time.Minute
	for remaining > 0 {
		step := remaining
		if step > heartbeat {
			step = heartbeat
		}
		sleepFn(step)
		remaining -= step
		if remaining > 0 && logger != nil {
			logger.WriteLine(fmt.Sprintf("[retry] waiting %s more before retry", remaining.Round(time.Second)))
		}
	}
}

func applyJitter(delay time.Duration) time.Duration {
	delay += time.Duration(jitterFn(int(delay / 10)))
	if delay < 0 {
		return 0
	}
	return delay
}

func (c ErrorCategory) String() string {
	switch c {
	case ErrNetwork:
		return "network"
	case ErrQuota:
		return "quota"
	case ErrSandboxViolation:
		return "sandbox"
	default:
		return "task"
	}
}

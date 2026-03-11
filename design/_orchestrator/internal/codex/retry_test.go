package codex

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/organic-programming/codex-orchestrator/internal/logging"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stderr string
		want   ErrorCategory
	}{
		{name: "network", stderr: "connection timeout", want: ErrNetwork},
		{name: "quota", stderr: "429 rate limit", want: ErrQuota},
		{name: "sandbox", stderr: "permission denied by sandbox", want: ErrSandboxViolation},
		{name: "task", stderr: "task failed", want: ErrTaskFailure},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Classify(1, tc.stderr); got != tc.want {
				t.Fatalf("Classify() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRetryWithBackoffRetriesAndLogsHeartbeat(t *testing.T) {
	originalSleep := sleepFn
	originalNow := nowFn
	originalJitter := jitterFn
	sleepFn = func(time.Duration) {}
	nowFn = func() time.Time { return time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC) }
	jitterFn = func(int) int { return 0 }
	defer func() {
		sleepFn = originalSleep
		nowFn = originalNow
		jitterFn = originalJitter
	}()

	var out bytes.Buffer
	logger := &logging.TeeWriter{Terminal: &out}
	attempts := 0

	err := RetryWithBackoff(ErrNetwork, func() error {
		attempts++
		if attempts == 2 {
			return nil
		}
		return errors.New("temporary failure")
	}, logger)
	if err != nil {
		t.Fatalf("RetryWithBackoff returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if !strings.Contains(out.String(), "[retry] category=network") {
		t.Fatalf("expected retry log output, got %q", out.String())
	}
}
